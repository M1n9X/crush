package agent

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"charm.land/fantasy"

	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/subagent"
)

//go:embed templates/subagent_tool.md
var subAgentToolDescription []byte

type SubAgentParams struct {
	// Optional short description for logging/UX.
	Description string `json:"description,omitempty" description:"Short description of the task"`
	// Mandatory task prompt for the subagent to execute.
	Prompt string `json:"prompt" description:"The task for the subagent to perform"`
	// Select which subagent profile to use (tools/system prompt).
	SubagentType string `json:"subagent_type,omitempty" description:"The subagent type to use (defined in ~/.claude/.codebreeze/.crush agents or project equivalents). Defaults to general-purpose."`
	// Optional: Specific model name to use for this task. If not provided, uses the default task model or the subagent's configured model.
	ModelName string `json:"model_name,omitempty" description:"Optional: Specific model name to use for this task. If not provided, uses the default task model or the subagent's configured model."`
	// Optional: reuse an existing subagent session (claude) instead of creating a new one.
	SessionID string `json:"session_id,omitempty" description:"Existing subagent session to continue"`
	// Optional: resume token for adapters that require it (e.g. Codex thread id).
	ResumeToken string `json:"resume_token,omitempty" description:"Resume token for adapters that support resuming (e.g. Codex thread id)"`
}

const (
	SubAgentToolName = "subagent"
)

// subAgentTool exposes a configurable subagent powered by user-defined profiles.
func (c *coordinator) subAgentTool(ctx context.Context) (fantasy.AgentTool, error) {
	toolDescription := strings.TrimSpace(string(subAgentToolDescription))
	if defs, err := loadSubAgentDefinitions(c.cfg.WorkingDir()); err == nil && len(defs) > 0 {
		if formatted := formatSubAgentDefinitions(defs); formatted != "" {
			toolDescription = fmt.Sprintf("%s\n\nAvailable subagents:\n%s", toolDescription, formatted)
		}
	}

	return fantasy.NewAgentTool(
		SubAgentToolName,
		toolDescription,
		func(ctx context.Context, params SubAgentParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			// Validate required parameters
			if strings.TrimSpace(params.Prompt) == "" {
				return fantasy.NewTextErrorResponse("prompt is required and cannot be empty"), nil
			}

			parentSessionID := tools.GetSessionFromContext(ctx)
			if parentSessionID == "" {
				return fantasy.ToolResponse{}, errors.New("session id missing from context")
			}

			agentMessageID := tools.GetMessageFromContext(ctx)
			if agentMessageID == "" {
				return fantasy.ToolResponse{}, errors.New("agent message id missing from context")
			}

			registry, err := c.subagentRegistry(ctx)
			if err != nil {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("failed to load subagents: %s", err)), nil
			}
			registration, err := registry.Resolve(params.SubagentType)
			if err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}
			profile := registration.Profile

			agentToolSessionID := c.sessions.CreateAgentToolSessionID(agentMessageID, call.ID)
			sessionTitle := fmt.Sprintf("Sub Agent: %s", profile.Name)
			targetSessionID := strings.TrimSpace(params.SessionID)
			if targetSessionID == "" {
				newSession, err := c.sessions.CreateTaskSession(ctx, agentToolSessionID, parentSessionID, sessionTitle)
				if err != nil {
					return fantasy.ToolResponse{}, fmt.Errorf("error creating session: %s", err)
				}
				targetSessionID = newSession.ID
			} else {
				if _, err := c.sessions.Get(ctx, targetSessionID); err != nil {
					return fantasy.NewTextErrorResponse(fmt.Sprintf("unknown session_id %q", targetSessionID)), nil
				}
			}

			// Subagent sessions are read-only by design; auto-approve to avoid permission prompts.
			c.permissions.AutoApproveSession(targetSessionID)

			// Forward subagent logs to parent session for streaming visibility.
			progressCtx, progressCancel := context.WithCancel(ctx)
			defer progressCancel()
			go c.forwardSubAgentLogs(progressCtx, targetSessionID, parentSessionID, call.ID, profile.Name)

			metadata := map[string]string{
				"parent_session_id": parentSessionID,
				"tool_call_id":      call.ID,
			}
			if profile.MaxSteps > 0 {
				metadata["max_steps"] = strconv.Itoa(profile.MaxSteps)
			}
			if color := strings.TrimSpace(profile.Color); color != "" {
				metadata["color"] = color
			}

			req := subagent.Request{
				Task:        params.Prompt,
				Sandbox:     deriveSandbox(&profile),
				Metadata:    metadata,
				Profile:     &profile,
				SessionID:   targetSessionID,
				ParentID:    parentSessionID,
				ModelName:   params.ModelName,
				Description: params.Description,
			}

			var result *subagent.Result
			if params.ResumeToken != "" && registration.Agent.SupportsResume() {
				result, err = registration.Agent.Resume(ctx, params.ResumeToken, req)
			} else {
				result, err = registration.Agent.Execute(ctx, req)
			}

			// Stop forwarding logs
			progressCancel()

			if err != nil {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("error running subagent %s: %v", profile.Name, err)), nil
			}
			if result == nil {
				return fantasy.NewTextErrorResponse("subagent returned no result"), nil
			}

			if err := c.reconcileSubagentCost(ctx, targetSessionID, parentSessionID, result); err != nil {
				return fantasy.ToolResponse{}, fmt.Errorf("error saving parent session: %s", err)
			}

			return fantasy.NewTextResponse(result.Text), nil
		},
	), nil
}

func filterToolNames(base []string, blocked ...string) []string {
	if len(blocked) == 0 {
		return base
	}
	var filtered []string
	for _, tool := range base {
		if slices.Contains(blocked, tool) {
			continue
		}
		filtered = append(filtered, tool)
	}
	return filtered
}

func (c *coordinator) reconcileSubagentCost(ctx context.Context, subSessionID, parentSessionID string, result *subagent.Result) error {
	if subSessionID == "" || parentSessionID == "" {
		return nil
	}

	subSession, err := c.sessions.Get(ctx, subSessionID)
	if err != nil {
		return err
	}
	parentSession, err := c.sessions.Get(ctx, parentSessionID)
	if err != nil {
		return err
	}

	parentSession.Cost += subSession.Cost
	if result != nil && result.Usage.CostUSD > 0 {
		parentSession.Cost += result.Usage.CostUSD
	}

	_, err = c.sessions.Save(ctx, parentSession)
	return err
}

func (c *coordinator) forwardSubAgentLogs(ctx context.Context, subSessionID, parentSessionID, toolCallID, subagentName string) {
	events := c.messages.Subscribe(ctx)
	lastContent := make(map[string]string)
	metadata := encodeSubAgentLogMetadata(subagentName)

	for event := range events {
		if event.Type != pubsub.CreatedEvent && event.Type != pubsub.UpdatedEvent {
			continue
		}
		msg := event.Payload
		if msg.SessionID != subSessionID {
			continue
		}
		var text string
		switch msg.Role {
		case message.Assistant:
			text = strings.ReplaceAll(msg.Content().Text, "\r\n", "\n")
		case message.Tool:
			var parts []string
			for _, tr := range msg.ToolResults() {
				content := strings.TrimSpace(tr.Content)
				if content == "" {
					continue
				}
				parts = append(parts, content)
			}
			text = strings.Join(parts, "\n\n")
		default:
			continue
		}
		if strings.TrimSpace(text) == "" {
			continue
		}

		prev := lastContent[msg.ID]
		delta := text
		if strings.HasPrefix(text, prev) {
			delta = strings.TrimPrefix(text, prev)
		}
		if delta == "" {
			continue
		}
		lastContent[msg.ID] = text

		_, _ = c.messages.Create(ctx, parentSessionID, message.CreateMessageParams{
			Role: message.Tool,
			Parts: []message.ContentPart{
				message.ToolResult{
					ToolCallID: toolCallID,
					Name:       SubAgentToolName,
					Content:    delta,
					Metadata:   metadata,
				},
			},
		})
	}
}

type subAgentLogMetadata struct {
	SubagentName string `json:"subagent_name"`
}

func encodeSubAgentLogMetadata(name string) string {
	if name == "" {
		return ""
	}
	raw, err := json.Marshal(subAgentLogMetadata{SubagentName: name})
	if err != nil {
		return ""
	}
	return string(raw)
}
