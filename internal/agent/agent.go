// Package agent is the core orchestration layer for Crush AI agents.
//
// It provides session-based AI agent functionality for managing
// conversations, tool execution, and message handling. It coordinates
// interactions between language models, messages, sessions, and tools while
// handling features like automatic summarization, queuing, and token
// management.
package agent

import (
	"cmp"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"charm.land/fantasy"
	"charm.land/fantasy/providers/anthropic"
	"charm.land/fantasy/providers/bedrock"
	"charm.land/fantasy/providers/google"
	"charm.land/fantasy/providers/openai"
	"charm.land/fantasy/providers/openrouter"
	"github.com/charmbracelet/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/freshness"
	"github.com/charmbracelet/crush/internal/fsext"
	"github.com/charmbracelet/crush/internal/history"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/ratelimit"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/stringext"
	"github.com/charmbracelet/crush/internal/telemetry"
)

//go:embed templates/title.md
var titlePrompt []byte

//go:embed templates/summary.md
var summaryPrompt []byte

const (
	autoCompactThresholdRatio = 0.92
	maxRecoveredFiles         = 5
	maxTokensPerRecoveredFile = 10_000
	maxTotalRecoveredTokens   = 50_000
)

type SessionAgentCall struct {
	SessionID        string
	Prompt           string
	ProviderOptions  fantasy.ProviderOptions
	Attachments      []message.Attachment
	MaxOutputTokens  int64
	Temperature      *float64
	TopP             *float64
	TopK             *int64
	FrequencyPenalty *float64
	PresencePenalty  *float64
}

type SessionAgent interface {
	Run(context.Context, SessionAgentCall) (*fantasy.AgentResult, error)
	SetModels(large Model, small Model)
	SetTools(tools []fantasy.AgentTool)
	Cancel(sessionID string)
	CancelAll()
	IsSessionBusy(sessionID string) bool
	IsBusy() bool
	QueuedPrompts(sessionID string) int
	ClearQueue(sessionID string)
	Summarize(context.Context, string, fantasy.ProviderOptions) error
	Model() Model
}

type Model struct {
	Model      fantasy.LanguageModel
	CatwalkCfg catwalk.Model
	ModelCfg   config.SelectedModel
}

type sessionAgent struct {
	largeModel           Model
	smallModel           Model
	systemPromptPrefix   string
	systemPrompt         string
	tools                []fantasy.AgentTool
	sessions             session.Service
	messages             message.Service
	disableAutoSummarize bool
	isYolo               bool
	history              history.Service
	workingDir           string
	telemetry            *telemetry.Recorder
	freshness            *freshness.Service

	messageQueue   *csync.Map[string, []SessionAgentCall]
	activeRequests *csync.Map[string, context.CancelFunc]
	toolTimers     *csync.Map[string, time.Time]

	// rateLimiter enforces rate limits when configured
	rateLimiter *ratelimit.Limiter
	retryConfig *config.RetryConfig
}

type SessionAgentOptions struct {
	LargeModel           Model
	SmallModel           Model
	SystemPromptPrefix   string
	SystemPrompt         string
	DisableAutoSummarize bool
	IsYolo               bool
	Sessions             session.Service
	Messages             message.Service
	Tools                []fantasy.AgentTool
	History              history.Service
	WorkingDir           string
	Telemetry            *telemetry.Recorder
	Freshness            *freshness.Service
}

func NewSessionAgent(
	opts SessionAgentOptions,
) SessionAgent {
	return &sessionAgent{
		largeModel:           opts.LargeModel,
		smallModel:           opts.SmallModel,
		systemPromptPrefix:   opts.SystemPromptPrefix,
		systemPrompt:         opts.SystemPrompt,
		sessions:             opts.Sessions,
		messages:             opts.Messages,
		disableAutoSummarize: opts.DisableAutoSummarize,
		tools:                opts.Tools,
		isYolo:               opts.IsYolo,
		history:              opts.History,
		workingDir:           opts.WorkingDir,
		telemetry:            opts.Telemetry,
		freshness:            opts.Freshness,
		messageQueue:         csync.NewMap[string, []SessionAgentCall](),
		activeRequests:       csync.NewMap[string, context.CancelFunc](),
		toolTimers:           csync.NewMap[string, time.Time](),
		rateLimiter:          createRateLimiter(opts.LargeModel.ModelCfg),
		retryConfig:          opts.LargeModel.ModelCfg.Retry,
	}
}

// createRateLimiter creates a rate limiter if rate limiting is configured.
func createRateLimiter(modelCfg config.SelectedModel) *ratelimit.Limiter {
	if modelCfg.RateLimiting == nil {
		return nil
	}

	var rpm, tpm int64
	if modelCfg.RateLimiting.RequestsPerMinute != nil {
		rpm = *modelCfg.RateLimiting.RequestsPerMinute
	}
	if modelCfg.RateLimiting.TokensPerMinute != nil {
		tpm = *modelCfg.RateLimiting.TokensPerMinute
	}

	if rpm == 0 && tpm == 0 {
		return nil
	}

	return ratelimit.New(rpm, tpm)
}

// getMaxRetries returns the max retries value from retry config.
// Returns nil if no retry config or max retries is not set (uses fantasy default).
func (a *sessionAgent) getMaxRetries() *int {
	if a.retryConfig == nil || a.retryConfig.MaxRetries == nil {
		return nil
	}
	maxRetries := int(*a.retryConfig.MaxRetries)
	return &maxRetries
}

func (a *sessionAgent) Run(ctx context.Context, call SessionAgentCall) (*fantasy.AgentResult, error) {
	if call.Prompt == "" {
		return nil, ErrEmptyPrompt
	}
	if call.SessionID == "" {
		return nil, ErrSessionMissing
	}

	// Queue the message if busy
	if a.IsSessionBusy(call.SessionID) {
		existing, ok := a.messageQueue.Get(call.SessionID)
		if !ok {
			existing = []SessionAgentCall{}
		}
		existing = append(existing, call)
		a.messageQueue.Set(call.SessionID, existing)
		return nil, nil
	}

	// Apply rate limiting if configured
	if a.rateLimiter != nil && a.rateLimiter.Enabled() {
		if err := a.rateLimiter.Wait(ctx); err != nil {
			return nil, fmt.Errorf("rate limiter: %w", err)
		}
	}

	if len(a.tools) > 0 {
		// Add Anthropic caching to the last tool.
		a.tools[len(a.tools)-1].SetProviderOptions(a.getCacheControlOptions())
	}

	agent := fantasy.NewAgent(
		a.largeModel.Model,
		fantasy.WithSystemPrompt(a.systemPrompt),
		fantasy.WithTools(a.tools...),
	)

	sessionLock := sync.Mutex{}
	currentSession, err := a.sessions.Get(ctx, call.SessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	msgs, err := a.getSessionMessages(ctx, currentSession)
	if err != nil {
		return nil, fmt.Errorf("failed to get session messages: %w", err)
	}

	var compacted bool
	currentSession, msgs, compacted, err = a.maybeAutoCompact(ctx, currentSession, msgs, call.ProviderOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to compact session: %w", err)
	}
	if compacted {
		slog.Info("Session auto-compacted", "session_id", call.SessionID)
	}

	var wg sync.WaitGroup
	// Generate title if first message.
	if len(msgs) == 0 {
		wg.Go(func() {
			sessionLock.Lock()
			a.generateTitle(ctx, &currentSession, call.Prompt)
			sessionLock.Unlock()
		})
	}

	// Add the user message to the session.
	_, err = a.createUserMessage(ctx, call)
	if err != nil {
		return nil, err
	}

	// Add the session to the context.
	ctx = context.WithValue(ctx, tools.SessionIDContextKey, call.SessionID)

	genCtx, cancel := context.WithCancel(ctx)
	a.activeRequests.Set(call.SessionID, cancel)

	defer cancel()
	defer a.activeRequests.Del(call.SessionID)

	history, files := a.preparePrompt(msgs, call.Attachments...)

	startTime := time.Now()
	a.eventPromptSent(call.SessionID)

	var currentAssistant *message.Message
	var shouldSummarize bool
	result, err := agent.Stream(genCtx, fantasy.AgentStreamCall{
		Prompt:           call.Prompt,
		Files:            files,
		Messages:         history,
		ProviderOptions:  call.ProviderOptions,
		MaxOutputTokens:  &call.MaxOutputTokens,
		TopP:             call.TopP,
		Temperature:      call.Temperature,
		PresencePenalty:  call.PresencePenalty,
		TopK:             call.TopK,
		FrequencyPenalty: call.FrequencyPenalty,
		MaxRetries:       a.getMaxRetries(),
		// Before each step create a new assistant message.
		PrepareStep: func(callContext context.Context, options fantasy.PrepareStepFunctionOptions) (_ context.Context, prepared fantasy.PrepareStepResult, err error) {
			prepared.Messages = options.Messages
			// Reset all cached items.
			for i := range prepared.Messages {
				prepared.Messages[i].ProviderOptions = nil
			}

			queuedCalls, _ := a.messageQueue.Get(call.SessionID)
			a.messageQueue.Del(call.SessionID)
			for _, queued := range queuedCalls {
				userMessage, createErr := a.createUserMessage(callContext, queued)
				if createErr != nil {
					return callContext, prepared, createErr
				}
				prepared.Messages = append(prepared.Messages, userMessage.ToAIMessage()...)
			}

			lastSystemRoleInx := 0
			systemMessageUpdated := false
			for i, msg := range prepared.Messages {
				// Only add cache control to the last message.
				if msg.Role == fantasy.MessageRoleSystem {
					lastSystemRoleInx = i
				} else if !systemMessageUpdated {
					prepared.Messages[lastSystemRoleInx].ProviderOptions = a.getCacheControlOptions()
					systemMessageUpdated = true
				}
				// Than add cache control to the last 2 messages.
				if i > len(prepared.Messages)-3 {
					prepared.Messages[i].ProviderOptions = a.getCacheControlOptions()
				}
			}

			if a.systemPromptPrefix != "" {
				prepared.Messages = append([]fantasy.Message{fantasy.NewSystemMessage(a.systemPromptPrefix)}, prepared.Messages...)
			}

			var assistantMsg message.Message
			assistantMsg, err = a.messages.Create(callContext, call.SessionID, message.CreateMessageParams{
				Role:     message.Assistant,
				Parts:    []message.ContentPart{},
				Model:    a.largeModel.ModelCfg.Model,
				Provider: a.largeModel.ModelCfg.Provider,
			})
			if err != nil {
				return callContext, prepared, err
			}
			callContext = context.WithValue(callContext, tools.MessageIDContextKey, assistantMsg.ID)
			currentAssistant = &assistantMsg
			return callContext, prepared, err
		},
		OnReasoningStart: func(id string, reasoning fantasy.ReasoningContent) error {
			currentAssistant.AppendReasoningContent(reasoning.Text)
			return a.messages.Update(genCtx, *currentAssistant)
		},
		OnReasoningDelta: func(id string, text string) error {
			currentAssistant.AppendReasoningContent(text)
			return a.messages.Update(genCtx, *currentAssistant)
		},
		OnReasoningEnd: func(id string, reasoning fantasy.ReasoningContent) error {
			// handle anthropic signature
			if anthropicData, ok := reasoning.ProviderMetadata[anthropic.Name]; ok {
				if reasoning, ok := anthropicData.(*anthropic.ReasoningOptionMetadata); ok {
					currentAssistant.AppendReasoningSignature(reasoning.Signature)
				}
			}
			if googleData, ok := reasoning.ProviderMetadata[google.Name]; ok {
				if reasoning, ok := googleData.(*google.ReasoningMetadata); ok {
					currentAssistant.AppendThoughtSignature(reasoning.Signature, reasoning.ToolID)
				}
			}
			if openaiData, ok := reasoning.ProviderMetadata[openai.Name]; ok {
				if reasoning, ok := openaiData.(*openai.ResponsesReasoningMetadata); ok {
					currentAssistant.SetReasoningResponsesData(reasoning)
				}
			}
			currentAssistant.FinishThinking()
			return a.messages.Update(genCtx, *currentAssistant)
		},
		OnTextDelta: func(id string, text string) error {
			// Strip leading newline from initial text content. This is is
			// particularly important in non-interactive mode where leading
			// newlines are very visible.
			if len(currentAssistant.Parts) == 0 {
				text = strings.TrimPrefix(text, "\n")
			}

			currentAssistant.AppendContent(text)
			return a.messages.Update(genCtx, *currentAssistant)
		},
		OnToolInputStart: func(id string, toolName string) error {
			toolCall := message.ToolCall{
				ID:               id,
				Name:             toolName,
				ProviderExecuted: false,
				Finished:         false,
			}
			a.toolTimers.Set(id, time.Now())
			currentAssistant.AddToolCall(toolCall)
			return a.messages.Update(genCtx, *currentAssistant)
		},
		OnRetry: func(err *fantasy.ProviderError, delay time.Duration) {
			// Classify the error to determine the category
			category := ClassifyError(err)

			// Get user-friendly error message
			userMsg := GetErrorMessage(err)

			// Update error statistics
			UpdateErrorStats(err)

			// Log the retry attempt
			slog.Info("API request retry",
				"session_id", call.SessionID,
				"error_category", category,
				"error_message", err.Message,
				"user_message", userMsg,
				"retry_delay", delay.String(),
			)
		},
		OnToolCall: func(tc fantasy.ToolCallContent) error {
			toolCall := message.ToolCall{
				ID:               tc.ToolCallID,
				Name:             tc.ToolName,
				Input:            tc.Input,
				ProviderExecuted: false,
				Finished:         true,
			}
			currentAssistant.AddToolCall(toolCall)
			return a.messages.Update(genCtx, *currentAssistant)
		},
		OnToolResult: func(result fantasy.ToolResultContent) error {
			var resultContent string
			isError := false
			start, _ := a.toolTimers.Get(result.ToolCallID)
			duration := time.Since(start)
			a.toolTimers.Del(result.ToolCallID)
			switch result.Result.GetType() {
			case fantasy.ToolResultContentTypeText:
				r, ok := fantasy.AsToolResultOutputType[fantasy.ToolResultOutputContentText](result.Result)
				if ok {
					resultContent = r.Text
				}
			case fantasy.ToolResultContentTypeError:
				r, ok := fantasy.AsToolResultOutputType[fantasy.ToolResultOutputContentError](result.Result)
				if ok {
					isError = true
					resultContent = r.Error.Error()
				}
			case fantasy.ToolResultContentTypeMedia:
				// TODO: handle this message type
			}
			toolResult := message.ToolResult{
				ToolCallID: result.ToolCallID,
				Name:       result.ToolName,
				Content:    resultContent,
				IsError:    isError,
				Metadata:   result.ClientMetadata,
			}
			_, createMsgErr := a.messages.Create(genCtx, currentAssistant.SessionID, message.CreateMessageParams{
				Role: message.Tool,
				Parts: []message.ContentPart{
					toolResult,
				},
			})
			if createMsgErr != nil {
				return createMsgErr
			}
			if a.telemetry != nil {
				tokensIn, tokensOut, toolCost := toolUsage(result.ProviderMetadata, result.ClientMetadata)
				a.telemetry.Publish(telemetry.Event{
					Type:      telemetry.EventToolFinished,
					SessionID: call.SessionID,
					ToolName:  result.ToolName,
					Duration:  duration,
					TokensIn:  tokensIn,
					TokensOut: tokensOut,
					Cost:      toolCost,
					At:        time.Now(),
				})
			}
			return nil
		},
		OnStepFinish: func(stepResult fantasy.StepResult) error {
			finishReason := message.FinishReasonUnknown
			switch stepResult.FinishReason {
			case fantasy.FinishReasonLength:
				finishReason = message.FinishReasonMaxTokens
			case fantasy.FinishReasonStop:
				finishReason = message.FinishReasonEndTurn
			case fantasy.FinishReasonToolCalls:
				finishReason = message.FinishReasonToolUse
			}
			currentAssistant.AddFinish(finishReason, "", "")
			a.updateSessionUsage(a.largeModel, &currentSession, stepResult.Usage, a.openrouterCost(stepResult.ProviderMetadata))
			sessionLock.Lock()
			_, sessionErr := a.sessions.Save(genCtx, currentSession)
			sessionLock.Unlock()
			if sessionErr != nil {
				return sessionErr
			}
			return a.messages.Update(genCtx, *currentAssistant)
		},
		StopWhen: []fantasy.StopCondition{
			func(_ []fantasy.StepResult) bool {
				cw := int64(a.largeModel.CatwalkCfg.ContextWindow)
				tokens := currentSession.CompletionTokens + currentSession.PromptTokens
				remaining := cw - tokens
				var threshold int64
				if cw > 200_000 {
					threshold = 20_000
				} else {
					threshold = int64(float64(cw) * 0.2)
				}
				if (remaining <= threshold) && !a.disableAutoSummarize {
					shouldSummarize = true
					return true
				}
				return false
			},
		},
	})

	a.eventPromptResponded(call.SessionID, time.Since(startTime).Truncate(time.Second))

	if err != nil {
		isCancelErr := errors.Is(err, context.Canceled)
		isPermissionErr := errors.Is(err, permission.ErrorPermissionDenied)
		if currentAssistant == nil {
			return result, err
		}
		// Ensure we finish thinking on error to close the reasoning state.
		currentAssistant.FinishThinking()
		toolCalls := currentAssistant.ToolCalls()
		// INFO: we use the parent context here because the genCtx has been cancelled.
		msgs, createErr := a.messages.List(ctx, currentAssistant.SessionID)
		if createErr != nil {
			return nil, createErr
		}
		for _, tc := range toolCalls {
			if !tc.Finished {
				tc.Finished = true
				tc.Input = "{}"
				currentAssistant.AddToolCall(tc)
				updateErr := a.messages.Update(ctx, *currentAssistant)
				if updateErr != nil {
					return nil, updateErr
				}
			}

			found := false
			for _, msg := range msgs {
				if msg.Role == message.Tool {
					for _, tr := range msg.ToolResults() {
						if tr.ToolCallID == tc.ID {
							found = true
							break
						}
					}
				}
				if found {
					break
				}
			}
			if found {
				continue
			}
			content := "There was an error while executing the tool"
			if isCancelErr {
				content = "Tool execution canceled by user"
			} else if isPermissionErr {
				content = "User denied permission"
			}
			toolResult := message.ToolResult{
				ToolCallID: tc.ID,
				Name:       tc.Name,
				Content:    content,
				IsError:    true,
			}
			_, createErr = a.messages.Create(context.Background(), currentAssistant.SessionID, message.CreateMessageParams{
				Role: message.Tool,
				Parts: []message.ContentPart{
					toolResult,
				},
			})
			if createErr != nil {
				return nil, createErr
			}
		}
		var fantasyErr *fantasy.Error
		var providerErr *fantasy.ProviderError
		const defaultTitle = "Provider Error"
		if isCancelErr {
			currentAssistant.AddFinish(message.FinishReasonCanceled, "User canceled request", "")
		} else if isPermissionErr {
			currentAssistant.AddFinish(message.FinishReasonPermissionDenied, "User denied permission", "")
		} else if errors.As(err, &providerErr) {
			currentAssistant.AddFinish(message.FinishReasonError, cmp.Or(stringext.Capitalize(providerErr.Title), defaultTitle), providerErr.Message)
		} else if errors.As(err, &fantasyErr) {
			currentAssistant.AddFinish(message.FinishReasonError, cmp.Or(stringext.Capitalize(fantasyErr.Title), defaultTitle), fantasyErr.Message)
		} else {
			currentAssistant.AddFinish(message.FinishReasonError, defaultTitle, err.Error())
		}
		// Note: we use the parent context here because the genCtx has been
		// cancelled.
		updateErr := a.messages.Update(ctx, *currentAssistant)
		if updateErr != nil {
			return nil, updateErr
		}
		return nil, err
	}
	wg.Wait()

	if shouldSummarize {
		a.activeRequests.Del(call.SessionID)
		if summarizeErr := a.Summarize(genCtx, call.SessionID, call.ProviderOptions); summarizeErr != nil {
			return nil, summarizeErr
		}
		// If the agent wasn't done...
		if len(currentAssistant.ToolCalls()) > 0 {
			existing, ok := a.messageQueue.Get(call.SessionID)
			if !ok {
				existing = []SessionAgentCall{}
			}
			call.Prompt = fmt.Sprintf("The previous session was interrupted because it got too long, the initial user request was: `%s`", call.Prompt)
			existing = append(existing, call)
			a.messageQueue.Set(call.SessionID, existing)
		}
	}

	// Release active request before processing queued messages.
	a.activeRequests.Del(call.SessionID)
	cancel()

	queuedMessages, ok := a.messageQueue.Get(call.SessionID)
	if !ok || len(queuedMessages) == 0 {
		return result, err
	}
	// There are queued messages restart the loop.
	firstQueuedMessage := queuedMessages[0]
	a.messageQueue.Set(call.SessionID, queuedMessages[1:])
	return a.Run(ctx, firstQueuedMessage)
}

func (a *sessionAgent) maybeAutoCompact(ctx context.Context, sess session.Session, msgs []message.Message, providerOpts fantasy.ProviderOptions) (session.Session, []message.Message, bool, error) {
	if a.disableAutoSummarize {
		return sess, msgs, false, nil
	}
	contextWindow := int64(a.largeModel.CatwalkCfg.ContextWindow)
	if contextWindow == 0 {
		return sess, msgs, false, nil
	}

	usedTokens := sess.PromptTokens + sess.CompletionTokens
	if float64(usedTokens) < float64(contextWindow)*autoCompactThresholdRatio {
		return sess, msgs, false, nil
	}

	compactedSession, compactedMsgs, err := a.autoCompactSession(ctx, sess, msgs, providerOpts)
	if err != nil {
		return sess, msgs, false, err
	}
	return compactedSession, compactedMsgs, true, nil
}

func (a *sessionAgent) autoCompactSession(ctx context.Context, sess session.Session, msgs []message.Message, providerOpts fantasy.ProviderOptions) (session.Session, []message.Message, error) {
	if len(msgs) == 0 {
		return sess, msgs, nil
	}

	if providerOpts == nil {
		providerOpts = fantasy.ProviderOptions{}
	}

	history, _ := a.preparePrompt(msgs)
	compactionNotice := fmt.Sprintf("Context automatically compressed (%.0f%% of window used). Preserved summary and recent files.", autoCompactThresholdRatio*100)

	genCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	summaryMessage, err := a.messages.Create(genCtx, sess.ID, message.CreateMessageParams{
		Role:             message.Assistant,
		Model:            a.largeModel.Model.Model(),
		Provider:         a.largeModel.Model.Provider(),
		IsSummaryMessage: true,
	})
	if err != nil {
		return sess, msgs, err
	}

	agent := fantasy.NewAgent(a.largeModel.Model,
		fantasy.WithSystemPrompt(string(summaryPrompt)),
	)

	resp, err := agent.Stream(genCtx, fantasy.AgentStreamCall{
		Prompt:          "Provide a detailed summary of our conversation above.",
		Messages:        history,
		ProviderOptions: providerOpts,
		MaxRetries:      a.getMaxRetries(),
		PrepareStep: func(callContext context.Context, options fantasy.PrepareStepFunctionOptions) (_ context.Context, prepared fantasy.PrepareStepResult, err error) {
			prepared.Messages = options.Messages
			if a.systemPromptPrefix != "" {
				prepared.Messages = append([]fantasy.Message{fantasy.NewSystemMessage(a.systemPromptPrefix)}, prepared.Messages...)
			}
			return callContext, prepared, nil
		},
		OnReasoningDelta: func(id string, text string) error {
			summaryMessage.AppendReasoningContent(text)
			return a.messages.Update(genCtx, summaryMessage)
		},
		OnReasoningEnd: func(id string, reasoning fantasy.ReasoningContent) error {
			// Handle anthropic signature.
			if anthropicData, ok := reasoning.ProviderMetadata["anthropic"]; ok {
				if signature, ok := anthropicData.(*anthropic.ReasoningOptionMetadata); ok && signature.Signature != "" {
					summaryMessage.AppendReasoningSignature(signature.Signature)
				}
			}
			summaryMessage.FinishThinking()
			return a.messages.Update(genCtx, summaryMessage)
		},
		OnTextDelta: func(id, text string) error {
			summaryMessage.AppendContent(text)
			return a.messages.Update(genCtx, summaryMessage)
		},
	})
	if err != nil {
		isCancelErr := errors.Is(err, context.Canceled)
		if isCancelErr {
			// User cancelled summarize we need to remove the summary message.
			deleteErr := a.messages.Delete(ctx, summaryMessage.ID)
			return sess, msgs, deleteErr
		}
		return sess, msgs, err
	}

	summaryMessage.AddFinish(message.FinishReasonEndTurn, "", "")
	if err := a.messages.Update(genCtx, summaryMessage); err != nil {
		return sess, msgs, err
	}

	var openrouterCost *float64
	for _, step := range resp.Steps {
		stepCost := a.openrouterCost(step.ProviderMetadata)
		if stepCost != nil {
			newCost := *stepCost
			if openrouterCost != nil {
				newCost += *openrouterCost
			}
			openrouterCost = &newCost
		}
	}

	a.updateSessionUsage(a.largeModel, &sess, resp.TotalUsage, openrouterCost)

	// Reset token counters to the summary footprint so we don't double-count
	// tokens after compaction.
	usage := resp.Response.Usage
	sess.SummaryMessageID = summaryMessage.ID
	sess.CompletionTokens = usage.OutputTokens
	sess.PromptTokens = 0
	if _, err := a.sessions.Save(genCtx, sess); err != nil {
		return sess, msgs, err
	}

	_, _ = a.messages.Create(ctx, sess.ID, message.CreateMessageParams{
		Role: message.User,
		Parts: []message.ContentPart{
			message.TextContent{Text: compactionNotice},
		},
	})

	if recovered, err := a.recoverRecentFiles(ctx, sess.ID); err == nil {
		for _, m := range recovered {
			// Already created inside recoverRecentFiles.
			history = append(history, m.ToAIMessage()...)
		}
	} else {
		slog.Debug("failed to recover files after compaction", "err", err)
	}

	updatedSess, err := a.sessions.Get(ctx, sess.ID)
	if err != nil {
		return sess, msgs, err
	}
	updatedMsgs, err := a.getSessionMessages(ctx, updatedSess)
	if err != nil {
		return sess, msgs, err
	}
	return updatedSess, updatedMsgs, nil
}

func (a *sessionAgent) recoverRecentFiles(ctx context.Context, sessionID string) ([]message.Message, error) {
	var paths []string
	if a.freshness != nil {
		top := a.freshness.Top(sessionID, maxRecoveredFiles)
		for _, meta := range top {
			paths = append(paths, meta.Path)
		}
	}
	var historyFiles []history.File
	if a.history != nil {
		if len(paths) == 0 {
			files, err := a.history.ListLatestSessionFiles(ctx, sessionID)
			if err == nil {
				historyFiles = files
			}
		} else {
			for _, p := range paths {
				if f, err := a.history.GetByPathAndSession(ctx, p, sessionID); err == nil {
					historyFiles = append(historyFiles, f)
				}
			}
		}
	}
	// Fallback: use collected paths to read from disk when history is missing.
	pathSet := make(map[string]struct{})
	for _, f := range historyFiles {
		pathSet[f.Path] = struct{}{}
	}
	for _, p := range paths {
		if _, ok := pathSet[p]; ok {
			continue
		}
		if content, err := os.ReadFile(p); err == nil {
			historyFiles = append(historyFiles, history.File{Path: p, Content: string(content)})
		}
	}
	if len(historyFiles) == 0 {
		return nil, nil
	}
	slog.Info("recovering context", "session", sessionID, "files", len(historyFiles))

	var created []message.Message
	var totalTokens int
	for i, f := range historyFiles {
		if i >= maxRecoveredFiles {
			break
		}
		content, tokens, truncated := truncateByTokens(f.Content, maxTokensPerRecoveredFile)
		if totalTokens+tokens > maxTotalRecoveredTokens {
			break
		}
		totalTokens += tokens
		body := fmt.Sprintf(
			"**Recovered File:** %s\n\n```\n%s\n```\n\n(Auto-recovered%s)",
			fsext.PrettyPath(f.Path),
			addLineNumbers(content, 1),
			boolSuffix(truncated, " [truncated]"),
		)
		msg, createErr := a.messages.Create(ctx, sessionID, message.CreateMessageParams{
			Role: message.User,
			Parts: []message.ContentPart{
				message.TextContent{Text: body},
			},
		})
		if createErr != nil {
			return created, createErr
		}
		created = append(created, msg)

	}
	return created, nil
}

func truncateByTokens(content string, maxTokens int) (string, int, bool) {
	estimated := len([]rune(content)) / 4
	if estimated <= maxTokens {
		return content, estimated, false
	}
	// Roughly keep within token budget.
	maxRunes := maxTokens * 4
	runes := []rune(content)
	if maxRunes > len(runes) {
		maxRunes = len(runes)
	}
	return string(runes[:maxRunes]), maxTokens, true
}

func addLineNumbers(content string, startLine int) string {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		lines[i] = fmt.Sprintf("%d: %s", startLine+i, line)
	}
	return strings.Join(lines, "\n")
}

func boolSuffix(ok bool, suffix string) string {
	if ok {
		return suffix
	}
	return ""
}

func (a *sessionAgent) Summarize(ctx context.Context, sessionID string, opts fantasy.ProviderOptions) error {
	if a.IsSessionBusy(sessionID) {
		return ErrSessionBusy
	}

	currentSession, err := a.sessions.Get(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("failed to get session: %w", err)
	}
	msgs, err := a.getSessionMessages(ctx, currentSession)
	if err != nil {
		return err
	}
	if len(msgs) == 0 {
		// Nothing to summarize.
		return nil
	}

	aiMsgs, _ := a.preparePrompt(msgs)

	genCtx, cancel := context.WithCancel(ctx)
	a.activeRequests.Set(sessionID, cancel)
	defer a.activeRequests.Del(sessionID)
	defer cancel()

	agent := fantasy.NewAgent(a.largeModel.Model,
		fantasy.WithSystemPrompt(string(summaryPrompt)),
	)
	summaryMessage, err := a.messages.Create(ctx, sessionID, message.CreateMessageParams{
		Role:             message.Assistant,
		Model:            a.largeModel.Model.Model(),
		Provider:         a.largeModel.Model.Provider(),
		IsSummaryMessage: true,
	})
	if err != nil {
		return err
	}

	resp, err := agent.Stream(genCtx, fantasy.AgentStreamCall{
		Prompt:          "Provide a detailed summary of our conversation above.",
		Messages:        aiMsgs,
		ProviderOptions: opts,
		MaxRetries:      a.getMaxRetries(),
		PrepareStep: func(callContext context.Context, options fantasy.PrepareStepFunctionOptions) (_ context.Context, prepared fantasy.PrepareStepResult, err error) {
			prepared.Messages = options.Messages
			if a.systemPromptPrefix != "" {
				prepared.Messages = append([]fantasy.Message{fantasy.NewSystemMessage(a.systemPromptPrefix)}, prepared.Messages...)
			}
			return callContext, prepared, nil
		},
		OnReasoningDelta: func(id string, text string) error {
			summaryMessage.AppendReasoningContent(text)
			return a.messages.Update(genCtx, summaryMessage)
		},
		OnReasoningEnd: func(id string, reasoning fantasy.ReasoningContent) error {
			// Handle anthropic signature.
			if anthropicData, ok := reasoning.ProviderMetadata["anthropic"]; ok {
				if signature, ok := anthropicData.(*anthropic.ReasoningOptionMetadata); ok && signature.Signature != "" {
					summaryMessage.AppendReasoningSignature(signature.Signature)
				}
			}
			summaryMessage.FinishThinking()
			return a.messages.Update(genCtx, summaryMessage)
		},
		OnTextDelta: func(id, text string) error {
			summaryMessage.AppendContent(text)
			return a.messages.Update(genCtx, summaryMessage)
		},
	})
	if err != nil {
		isCancelErr := errors.Is(err, context.Canceled)
		if isCancelErr {
			// User cancelled summarize we need to remove the summary message.
			deleteErr := a.messages.Delete(ctx, summaryMessage.ID)
			return deleteErr
		}
		return err
	}

	summaryMessage.AddFinish(message.FinishReasonEndTurn, "", "")
	err = a.messages.Update(genCtx, summaryMessage)
	if err != nil {
		return err
	}

	var openrouterCost *float64
	for _, step := range resp.Steps {
		stepCost := a.openrouterCost(step.ProviderMetadata)
		if stepCost != nil {
			newCost := *stepCost
			if openrouterCost != nil {
				newCost += *openrouterCost
			}
			openrouterCost = &newCost
		}
	}

	a.updateSessionUsage(a.largeModel, &currentSession, resp.TotalUsage, openrouterCost)

	// Just in case, get just the last usage info.
	usage := resp.Response.Usage
	currentSession.SummaryMessageID = summaryMessage.ID
	currentSession.CompletionTokens = usage.OutputTokens
	currentSession.PromptTokens = 0
	_, err = a.sessions.Save(genCtx, currentSession)
	return err
}

func (a *sessionAgent) getCacheControlOptions() fantasy.ProviderOptions {
	if t, _ := strconv.ParseBool(os.Getenv("CRUSH_DISABLE_ANTHROPIC_CACHE")); t {
		return fantasy.ProviderOptions{}
	}
	return fantasy.ProviderOptions{
		anthropic.Name: &anthropic.ProviderCacheControlOptions{
			CacheControl: anthropic.CacheControl{Type: "ephemeral"},
		},
		bedrock.Name: &anthropic.ProviderCacheControlOptions{
			CacheControl: anthropic.CacheControl{Type: "ephemeral"},
		},
	}
}

func (a *sessionAgent) createUserMessage(ctx context.Context, call SessionAgentCall) (message.Message, error) {
	var attachmentParts []message.ContentPart
	for _, attachment := range call.Attachments {
		attachmentParts = append(attachmentParts, message.BinaryContent{Path: attachment.FilePath, MIMEType: attachment.MimeType, Data: attachment.Content})
	}
	parts := []message.ContentPart{message.TextContent{Text: call.Prompt}}
	parts = append(parts, attachmentParts...)
	msg, err := a.messages.Create(ctx, call.SessionID, message.CreateMessageParams{
		Role:  message.User,
		Parts: parts,
	})
	if err != nil {
		return message.Message{}, fmt.Errorf("failed to create user message: %w", err)
	}
	return msg, nil
}

func (a *sessionAgent) preparePrompt(msgs []message.Message, attachments ...message.Attachment) ([]fantasy.Message, []fantasy.FilePart) {
	var history []fantasy.Message
	for _, m := range msgs {
		if len(m.Parts) == 0 {
			continue
		}
		// Skip subagent log plumbing so main agent context stays clean.
		if isSubAgentLog(m) {
			continue
		}
		// Assistant message without content or tool calls (cancelled before it
		// returned anything).
		if m.Role == message.Assistant && len(m.ToolCalls()) == 0 && m.Content().Text == "" && m.ReasoningContent().String() == "" {
			continue
		}
		history = append(history, m.ToAIMessage()...)
	}

	var files []fantasy.FilePart
	for _, attachment := range attachments {
		files = append(files, fantasy.FilePart{
			Filename:  attachment.FileName,
			Data:      attachment.Content,
			MediaType: attachment.MimeType,
		})
	}

	return history, files
}

func isSubAgentLog(m message.Message) bool {
	if m.Role != message.Tool {
		return false
	}
	for _, tr := range m.ToolResults() {
		if tr.Name == SubAgentToolName {
			return true
		}
	}
	return false
}

func (a *sessionAgent) getSessionMessages(ctx context.Context, session session.Session) ([]message.Message, error) {
	msgs, err := a.messages.List(ctx, session.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to list messages: %w", err)
	}

	if session.SummaryMessageID != "" {
		summaryMsgInex := -1
		for i, msg := range msgs {
			if msg.ID == session.SummaryMessageID {
				summaryMsgInex = i
				break
			}
		}
		if summaryMsgInex != -1 {
			msgs = msgs[summaryMsgInex:]
			msgs[0].Role = message.User
		}
	}
	return msgs, nil
}

func (a *sessionAgent) generateTitle(ctx context.Context, session *session.Session, prompt string) {
	if prompt == "" {
		return
	}

	var maxOutput int64 = 40
	if a.smallModel.CatwalkCfg.CanReason {
		maxOutput = a.smallModel.CatwalkCfg.DefaultMaxTokens
	}

	agent := fantasy.NewAgent(a.smallModel.Model,
		fantasy.WithSystemPrompt(string(titlePrompt)+"\n /no_think"),
		fantasy.WithMaxOutputTokens(maxOutput),
	)

	resp, err := agent.Stream(ctx, fantasy.AgentStreamCall{
		Prompt:     fmt.Sprintf("Generate a concise title for the following content:\n\n%s\n <think>\n\n</think>", prompt),
		MaxRetries: a.getMaxRetries(),
		PrepareStep: func(callContext context.Context, options fantasy.PrepareStepFunctionOptions) (_ context.Context, prepared fantasy.PrepareStepResult, err error) {
			prepared.Messages = options.Messages
			if a.systemPromptPrefix != "" {
				prepared.Messages = append([]fantasy.Message{fantasy.NewSystemMessage(a.systemPromptPrefix)}, prepared.Messages...)
			}
			return callContext, prepared, nil
		},
	})
	if err != nil {
		slog.Error("error generating title", "err", err)
		return
	}

	title := resp.Response.Content.Text()

	title = strings.ReplaceAll(title, "\n", " ")

	// Remove thinking tags if present.
	if idx := strings.Index(title, "</think>"); idx > 0 {
		title = title[idx+len("</think>"):]
	}

	title = strings.TrimSpace(title)
	if title == "" {
		slog.Warn("failed to generate title", "warn", "empty title")
		return
	}

	session.Title = title

	var openrouterCost *float64
	for _, step := range resp.Steps {
		stepCost := a.openrouterCost(step.ProviderMetadata)
		if stepCost != nil {
			newCost := *stepCost
			if openrouterCost != nil {
				newCost += *openrouterCost
			}
			openrouterCost = &newCost
		}
	}

	a.updateSessionUsage(a.smallModel, session, resp.TotalUsage, openrouterCost)
	_, saveErr := a.sessions.Save(ctx, *session)
	if saveErr != nil {
		slog.Error("failed to save session title & usage", "error", saveErr)
		return
	}
}

func (a *sessionAgent) openrouterCost(metadata fantasy.ProviderMetadata) *float64 {
	openrouterMetadata, ok := metadata[openrouter.Name]
	if !ok {
		return nil
	}

	opts, ok := openrouterMetadata.(*openrouter.ProviderMetadata)
	if !ok {
		return nil
	}
	return &opts.Usage.Cost
}

func toolUsage(metadata fantasy.ProviderMetadata, clientMetadata string) (tokensIn, tokensOut int64, cost float64) {
	// Prefer explicit client-supplied metrics when present.
	if tokensIn, tokensOut, cost, ok := parseClientMetadata(clientMetadata); ok {
		return tokensIn, tokensOut, cost
	}
	openrouterMetadata, ok := metadata[openrouter.Name]
	if !ok {
		return 0, 0, 0
	}
	if opts, ok := openrouterMetadata.(*openrouter.ProviderMetadata); ok {
		return opts.Usage.PromptTokens, opts.Usage.CompletionTokens, opts.Usage.Cost
	}
	return 0, 0, 0
}

// parseClientMetadata extracts usage/cost if the tool runner encoded it as JSON
// in the client metadata string (provider-agnostic fallback).
func parseClientMetadata(metadata string) (tokensIn, tokensOut int64, cost float64, ok bool) {
	if metadata == "" {
		return 0, 0, 0, false
	}
	var payload struct {
		TokensIn  int64   `json:"tokens_in"`
		TokensOut int64   `json:"tokens_out"`
		Cost      float64 `json:"cost"`
	}
	if err := json.Unmarshal([]byte(metadata), &payload); err != nil {
		return 0, 0, 0, false
	}
	return payload.TokensIn, payload.TokensOut, payload.Cost, true
}

func (a *sessionAgent) updateSessionUsage(model Model, session *session.Session, usage fantasy.Usage, overrideCost *float64) {
	modelConfig := model.CatwalkCfg
	cost := modelConfig.CostPer1MInCached/1e6*float64(usage.CacheCreationTokens) +
		modelConfig.CostPer1MOutCached/1e6*float64(usage.CacheReadTokens) +
		modelConfig.CostPer1MIn/1e6*float64(usage.InputTokens) +
		modelConfig.CostPer1MOut/1e6*float64(usage.OutputTokens)

	a.eventTokensUsed(session.ID, model, usage, cost)

	if overrideCost != nil {
		session.Cost += *overrideCost
	} else {
		session.Cost += cost
	}

	session.CompletionTokens = usage.OutputTokens + usage.CacheReadTokens
	session.PromptTokens = usage.InputTokens + usage.CacheCreationTokens

	if a.telemetry != nil {
		a.telemetry.Publish(telemetry.Event{
			Type:      telemetry.EventTokensUsed,
			SessionID: session.ID,
			Cost:      cost,
			TokensIn:  usage.InputTokens + usage.CacheCreationTokens,
			TokensOut: usage.OutputTokens + usage.CacheReadTokens,
			At:        time.Now(),
		})
	}
}

func (a *sessionAgent) Cancel(sessionID string) {
	// Cancel regular requests.
	if cancel, ok := a.activeRequests.Take(sessionID); ok && cancel != nil {
		slog.Info("Request cancellation initiated", "session_id", sessionID)
		cancel()
	}

	// Also check for summarize requests.
	if cancel, ok := a.activeRequests.Take(sessionID + "-summarize"); ok && cancel != nil {
		slog.Info("Summarize cancellation initiated", "session_id", sessionID)
		cancel()
	}

	if a.QueuedPrompts(sessionID) > 0 {
		slog.Info("Clearing queued prompts", "session_id", sessionID)
		a.messageQueue.Del(sessionID)
	}
}

func (a *sessionAgent) ClearQueue(sessionID string) {
	if a.QueuedPrompts(sessionID) > 0 {
		slog.Info("Clearing queued prompts", "session_id", sessionID)
		a.messageQueue.Del(sessionID)
	}
}

func (a *sessionAgent) CancelAll() {
	if !a.IsBusy() {
		return
	}
	for key := range a.activeRequests.Seq2() {
		a.Cancel(key) // key is sessionID
	}

	timeout := time.After(5 * time.Second)
	for a.IsBusy() {
		select {
		case <-timeout:
			return
		default:
			time.Sleep(200 * time.Millisecond)
		}
	}
}

func (a *sessionAgent) IsBusy() bool {
	var busy bool
	for cancelFunc := range a.activeRequests.Seq() {
		if cancelFunc != nil {
			busy = true
			break
		}
	}
	return busy
}

func (a *sessionAgent) IsSessionBusy(sessionID string) bool {
	_, busy := a.activeRequests.Get(sessionID)
	return busy
}

func (a *sessionAgent) QueuedPrompts(sessionID string) int {
	l, ok := a.messageQueue.Get(sessionID)
	if !ok {
		return 0
	}
	return len(l)
}

func (a *sessionAgent) SetModels(large Model, small Model) {
	a.largeModel = large
	a.smallModel = small
}

func (a *sessionAgent) SetTools(tools []fantasy.AgentTool) {
	a.tools = tools
}

func (a *sessionAgent) Model() Model {
	return a.largeModel
}
