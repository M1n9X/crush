package agent

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	claude "github.com/M1n9X/claude-agent-sdk-go"
	"github.com/M1n9X/claude-agent-sdk-go/types"
	"github.com/charmbracelet/crush/internal/agent/prompt"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/subagent"
)

type claudeClient interface {
	Connect(ctx context.Context) error
	Query(ctx context.Context, prompt string) error
	ReceiveResponse(ctx context.Context) <-chan types.Message
	Close(ctx context.Context) error
}

type claudeClientFactory func(ctx context.Context, opts *types.ClaudeAgentOptions) (claudeClient, error)

type ClaudeCodeSubagent struct {
	coord          *coordinator
	clientFactory  claudeClientFactory
	warningLogger  func(msg string, args ...any)
	usageFromUsage func(map[string]interface{}, string) subagent.Usage
}

func newClaudeCodeSubagent(coord *coordinator) *ClaudeCodeSubagent {
	return &ClaudeCodeSubagent{
		coord:          coord,
		clientFactory:  defaultClaudeClientFactory,
		warningLogger:  slog.Warn,
		usageFromUsage: mapClaudeUsage,
	}
}

func (c *ClaudeCodeSubagent) Name() string {
	return "claude-code"
}

func (c *ClaudeCodeSubagent) Capabilities() []subagent.Capability {
	return []subagent.Capability{"plan", "code", "review", "docs", "brainstorm"}
}

func (c *ClaudeCodeSubagent) SupportsResume() bool {
	return true
}

func (c *ClaudeCodeSubagent) Execute(ctx context.Context, req subagent.Request) (*subagent.Result, error) {
	return c.run(ctx, req)
}

func (c *ClaudeCodeSubagent) Resume(ctx context.Context, _ string, req subagent.Request) (*subagent.Result, error) {
	// SessionAgent handles resuming via the session id, so the resume token is unused here.
	return c.run(ctx, req)
}

func (c *ClaudeCodeSubagent) ExecuteStreamed(ctx context.Context, req subagent.Request, handler func(subagent.Event)) (*subagent.Result, error) {
	if req.Profile == nil {
		return nil, subagent.ErrMissingProfile
	}
	if strings.TrimSpace(req.Task) == "" {
		return nil, fmt.Errorf("task prompt is required")
	}

	agentCfg, allowedTools, err := c.resolveAgentConfig(req.Profile)
	if err != nil {
		return nil, err
	}

	options := c.buildClaudeOptions(ctx, agentCfg, allowedTools, req)
	client, err := c.clientFactory(ctx, options)
	if err != nil {
		return nil, fmt.Errorf("create claude client: %w", err)
	}
	defer client.Close(context.Background())

	if err := client.Connect(ctx); err != nil {
		return nil, err
	}

	fullPrompt := buildSubagentPrompt(req)
	if err := client.Query(ctx, fullPrompt); err != nil {
		return nil, err
	}

	var (
		finalText   string
		resumeToken string
		usage       subagent.Usage
	)

	for msg := range client.ReceiveResponse(ctx) {
		if handler != nil {
			handler(subagent.Event{Type: msg.GetMessageType(), Payload: msg, Provider: "claude"})
		}
		switch m := msg.(type) {
		case *types.AssistantMessage:
			text := assistantText(m)
			if text != "" {
				if finalText == "" {
					finalText = text
				} else {
					finalText = finalText + "\n" + text
				}
			}
		case *types.ResultMessage:
			usage = c.usageFromUsage(m.Usage, c.effectiveModelName(agentCfg, req))
			if m.TotalCostUSD != nil {
				usage.CostUSD = *m.TotalCostUSD
			}
			if m.Result != nil && finalText == "" {
				finalText = *m.Result
			}
			resumeToken = m.SessionID
		}
	}

	return &subagent.Result{
		Text:        strings.TrimSpace(finalText),
		ResumeToken: resumeToken,
		Usage:       usage,
	}, nil
}

func (c *ClaudeCodeSubagent) run(ctx context.Context, req subagent.Request) (*subagent.Result, error) {
	if req.Profile == nil {
		return nil, subagent.ErrMissingProfile
	}
	if strings.TrimSpace(req.Task) == "" {
		return nil, fmt.Errorf("task prompt is required")
	}
	if strings.TrimSpace(req.SessionID) == "" {
		return nil, fmt.Errorf("session id is required for claude subagent")
	}

	derivedCfg, _, err := c.resolveAgentConfig(req.Profile)
	if err != nil {
		return nil, err
	}

	effectiveModel := strings.TrimSpace(req.ModelName)
	if effectiveModel == "" {
		effectiveModel = strings.TrimSpace(req.Profile.ModelName)
	}
	if effectiveModel != "" {
		modelType := config.SelectedModelType(effectiveModel)
		if _, ok := c.coord.cfg.Models[modelType]; ok {
			derivedCfg.Model = modelType
		} else {
			c.warningLogger("Subagent requested unknown model_name, falling back to default", "model_name", effectiveModel, "profile", req.Profile.Name)
		}
	}

	sysPrompt, err := c.resolveSystemPrompt(req.Profile)
	if err != nil {
		return nil, err
	}

	agent, err := c.coord.buildAgent(ctx, sysPrompt, derivedCfg)
	if err != nil {
		return nil, err
	}

	tools, err := c.coord.buildTools(ctx, derivedCfg)
	if err != nil {
		return nil, err
	}
	agent.SetTools(tools)

	model := agent.Model()
	maxTokens := model.CatwalkCfg.DefaultMaxTokens
	if model.ModelCfg.MaxTokens != 0 {
		maxTokens = model.ModelCfg.MaxTokens
	}

	providerCfg, ok := c.coord.cfg.Providers.Get(model.ModelCfg.Provider)
	if !ok {
		return nil, errors.New("model provider not configured")
	}

	providerOptions := getProviderOptions(model, providerCfg)

	fullPrompt := buildSubagentPrompt(req)

	if req.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, req.Timeout)
		defer cancel()
	}

	result, err := agent.Run(ctx, SessionAgentCall{
		SessionID:        req.SessionID,
		Prompt:           fullPrompt,
		ProviderOptions:  providerOptions,
		Attachments:      req.Files,
		MaxOutputTokens:  maxTokens,
		Temperature:      model.ModelCfg.Temperature,
		TopP:             model.ModelCfg.TopP,
		TopK:             model.ModelCfg.TopK,
		FrequencyPenalty: model.ModelCfg.FrequencyPenalty,
		PresencePenalty:  model.ModelCfg.PresencePenalty,
	})
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, fmt.Errorf("subagent returned no result")
	}

	text := result.Response.Content.Text()
	usage := subagent.Usage{
		ModelName: model.ModelCfg.Model,
	}
	return &subagent.Result{
		Text:        text,
		ResumeToken: req.SessionID,
		Usage:       usage,
	}, nil
}

func (c *ClaudeCodeSubagent) resolveSystemPrompt(profile *subagent.Profile) (*prompt.Prompt, error) {
	if profile != nil && strings.TrimSpace(profile.SystemPrompt) != "" {
		return prompt.NewPrompt("subagent-"+profile.Name, profile.SystemPrompt, prompt.WithWorkingDir(c.coord.cfg.WorkingDir()))
	}
	return subAgentPrompt(prompt.WithWorkingDir(c.coord.cfg.WorkingDir()))
}

func (c *ClaudeCodeSubagent) resolveAgentConfig(profile *subagent.Profile) (config.Agent, []string, error) {
	agentCfg, ok := c.coord.cfg.Agents[config.AgentCoder]
	if !ok {
		if fallback, ok := c.coord.cfg.Agents[config.AgentTask]; ok {
			agentCfg = fallback
		} else {
			return config.Agent{}, nil, errors.New("coder or task agent not configured")
		}
	}

	baseTools := filterToolNames(agentCfg.AllowedTools, SubAgentToolName, AgentToolName)
	derivedCfg := agentCfg
	derivedCfg.AllowedTools = deriveAllowedTools(baseTools, profile)
	return derivedCfg, derivedCfg.AllowedTools, nil
}

func (c *ClaudeCodeSubagent) buildClaudeOptions(ctx context.Context, agentCfg config.Agent, allowedTools []string, req subagent.Request) *types.ClaudeAgentOptions {
	modelName := c.effectiveModelName(agentCfg, req)
	modelCandidate := strings.TrimSpace(req.ModelName)
	if modelCandidate == "" && req.Profile != nil {
		modelCandidate = strings.TrimSpace(req.Profile.ModelName)
	}
	if modelName == "" && modelCandidate != "" {
		c.warningLogger("Subagent requested unknown model_name, falling back to default", "model_name", modelCandidate, "profile", req.Profile.Name)
	}

	options := types.NewClaudeAgentOptions().
		WithAllowedTools(allowedTools...).
		WithPermissionMode(types.PermissionModeAcceptEdits).
		WithCWD(c.coord.cfg.WorkingDir())

	options.IncludePartialMessages = true

	if modelName != "" {
		options = options.WithModel(modelName)
	}

	if req.Metadata != nil {
		if resume := strings.TrimSpace(req.Metadata["resume_token"]); resume != "" {
			options = options.WithResume(resume).WithContinueConversation(true)
		}
	}

	if sysPrompt := c.renderSystemPrompt(ctx, req.Profile); sysPrompt != "" {
		options = options.WithSystemPromptString(sysPrompt)
	}

	return options
}

func (c *ClaudeCodeSubagent) renderSystemPrompt(ctx context.Context, profile *subagent.Profile) string {
	if c.coord == nil || c.coord.cfg == nil || c.coord.cfg.WorkingDir() == "" {
		if profile != nil {
			return strings.TrimSpace(profile.SystemPrompt)
		}
		return ""
	}

	p, err := c.resolveSystemPrompt(profile)
	if err != nil {
		c.warningLogger("failed to resolve subagent system prompt", "error", err)
		if profile != nil {
			return strings.TrimSpace(profile.SystemPrompt)
		}
		return ""
	}
	rendered, err := p.Build(ctx, "", "", *c.coord.cfg)
	if err != nil {
		c.warningLogger("failed to render subagent system prompt", "error", err)
		if profile != nil {
			return strings.TrimSpace(profile.SystemPrompt)
		}
		return ""
	}
	return rendered
}

func buildSubagentPrompt(req subagent.Request) string {
	fullPrompt := strings.TrimSpace(req.Task)
	var descParts []string
	if strings.TrimSpace(req.Profile.Description) != "" {
		descParts = append(descParts, strings.TrimSpace(req.Profile.Description))
	}
	if strings.TrimSpace(req.Description) != "" {
		descParts = append(descParts, strings.TrimSpace(req.Description))
	}
	if len(descParts) > 0 {
		fullPrompt = fmt.Sprintf("%s\n\nTask: %s", strings.Join(descParts, "\n"), req.Task)
	}
	return fullPrompt
}

func assistantText(msg *types.AssistantMessage) string {
	if msg == nil {
		return ""
	}
	var parts []string
	for _, block := range msg.Content {
		if tb, ok := block.(*types.TextBlock); ok && strings.TrimSpace(tb.Text) != "" {
			parts = append(parts, strings.TrimSpace(tb.Text))
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

func (c *ClaudeCodeSubagent) effectiveModelName(agentCfg config.Agent, req subagent.Request) string {
	if strings.TrimSpace(req.ModelName) != "" {
		return strings.TrimSpace(req.ModelName)
	}
	if req.Profile != nil && strings.TrimSpace(req.Profile.ModelName) != "" {
		return strings.TrimSpace(req.Profile.ModelName)
	}
	if c.coord == nil || c.coord.cfg == nil {
		return ""
	}
	if model, ok := c.coord.cfg.Models[agentCfg.Model]; ok {
		return model.Model
	}
	return ""
}

func mapClaudeUsage(usage map[string]interface{}, model string) subagent.Usage {
	if usage == nil {
		return subagent.Usage{ModelName: model}
	}
	getInt := func(key string) int64 {
		switch v := usage[key].(type) {
		case float64:
			return int64(v)
		case int:
			return int64(v)
		case int64:
			return v
		}
		return 0
	}
	promptTokens := getInt("input_tokens") + getInt("cache_creation_input_tokens") + getInt("cache_read_input_tokens")
	completionTokens := getInt("output_tokens")
	return subagent.Usage{
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		TotalTokens:      promptTokens + completionTokens,
		ModelName:        model,
	}
}

func defaultClaudeClientFactory(ctx context.Context, opts *types.ClaudeAgentOptions) (claudeClient, error) {
	return claude.NewClient(ctx, opts)
}
