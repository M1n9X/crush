package agent

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"charm.land/fantasy"

	"github.com/charmbracelet/crush/internal/agent/prompt"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/pubsub"
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
}

const (
	SubAgentToolName = "subagent"
)

// subAgentTool exposes a configurable subagent powered by user-defined profiles.
func (c *coordinator) subAgentTool(ctx context.Context) (fantasy.AgentTool, error) {
	agentCfg, ok := c.cfg.Agents[config.AgentCoder]
	if !ok {
		// Fallback to task agent if coder is missing
		var taskOK bool
		agentCfg, taskOK = c.cfg.Agents[config.AgentTask]
		if !taskOK {
			return nil, errors.New("coder or task agent not configured")
		}
	}

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

			sessionID := tools.GetSessionFromContext(ctx)
			if sessionID == "" {
				return fantasy.ToolResponse{}, errors.New("session id missing from context")
			}

			agentMessageID := tools.GetMessageFromContext(ctx)
			if agentMessageID == "" {
				return fantasy.ToolResponse{}, errors.New("agent message id missing from context")
			}

			// Load subagent definitions and pick the requested type.
			defs, err := loadSubAgentDefinitionsWithCache(c.cfg.WorkingDir())
			if err != nil {
				// Log warning but continue with builtin definitions only
				defs = []SubAgentDefinition{
					{
						Name:         "general-purpose",
						Description:  "General-purpose subagent for quick, read-only reconnaissance and lookups.",
						Wildcard:     true,
						SystemPrompt: "You are a general-purpose subagent. Stay concise and use only the tools provided to complete the task.",
						Source:       "builtin",
					},
				}
			}

			// Validate subagent_type if provided
			if params.SubagentType != "" {
				if err := validateSubagentType(params.SubagentType, defs); err != nil {
					return fantasy.NewTextErrorResponse(err.Error()), nil
				}
			}

			definition, err := selectSubAgentDefinition(defs, params.SubagentType)
			if err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}

			agentToolSessionID := c.sessions.CreateAgentToolSessionID(agentMessageID, call.ID)
			sessionTitle := fmt.Sprintf("Sub Agent: %s", definition.Name)
			session, err := c.sessions.CreateTaskSession(ctx, agentToolSessionID, sessionID, sessionTitle)
			if err != nil {
				return fantasy.ToolResponse{}, fmt.Errorf("error creating session: %s", err)
			}
			// Subagent sessions are read-only by design; auto-approve to avoid permission prompts.
			c.permissions.AutoApproveSession(session.ID)

			// Build a derived agent config with a per-subagent tool allowlist.
			baseTools := filterToolNames(agentCfg.AllowedTools, SubAgentToolName, AgentToolName)
			derivedAgentCfg := agentCfg
			if !definition.Wildcard && len(definition.Tools) > 0 {
				derivedAgentCfg.AllowedTools = intersectTools(baseTools, definition.Tools)
			} else {
				derivedAgentCfg.AllowedTools = baseTools
			}

			// Determine effective model: params.ModelName > definition.ModelName > default
			effectiveModel := ""
			if params.ModelName != "" {
				effectiveModel = params.ModelName
			} else if definition.ModelName != "" {
				effectiveModel = definition.ModelName
			}

			// If a specific model is requested, update the derived config
			if effectiveModel != "" {
				modelType := config.SelectedModelType(effectiveModel)
				if _, ok := c.cfg.Models[modelType]; !ok {
					return fantasy.NewTextErrorResponse(fmt.Sprintf("unknown model_name %q; available types: %s", effectiveModel, strings.Join(sortedModelTypes(c.cfg.Models), ", "))), nil
				}
				derivedAgentCfg.Model = modelType
			}

			// Build system prompt (agent-specific if provided, otherwise fallback template).
			var sysPrompt *prompt.Prompt
			if definition.SystemPrompt != "" {
				sysPrompt, err = prompt.NewPrompt("subagent-"+definition.Name, definition.SystemPrompt, prompt.WithWorkingDir(c.cfg.WorkingDir()))
			} else {
				sysPrompt, err = subAgentPrompt(prompt.WithWorkingDir(c.cfg.WorkingDir()))
			}
			if err != nil {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("error preparing system prompt: %s", err)), nil
			}

			// Forward subagent logs to parent session for streaming visibility.
			progressCtx, progressCancel := context.WithCancel(ctx)
			defer progressCancel()
			go c.forwardSubAgentLogs(progressCtx, session.ID, sessionID, call.ID, definition.Name)

			agent, err := c.buildAgent(ctx, sysPrompt, derivedAgentCfg)
			if err != nil {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("error building subagent: %s", err)), nil
			}

			// Build tools synchronously to ensure subagent has its allowlisted tools available immediately.
			subAgentTools, err := c.buildTools(ctx, derivedAgentCfg)
			if err != nil {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("error building subagent tools: %s", err)), nil
			}
			agent.SetTools(subAgentTools)

			model := agent.Model()
			maxTokens := model.CatwalkCfg.DefaultMaxTokens
			if model.ModelCfg.MaxTokens != 0 {
				maxTokens = model.ModelCfg.MaxTokens
			}

			providerCfg, ok := c.cfg.Providers.Get(model.ModelCfg.Provider)
			if !ok {
				return fantasy.ToolResponse{}, errors.New("model provider not configured")
			}

			fullPrompt := params.Prompt
			if params.Description != "" || definition.Description != "" {
				descParts := []string{}
				if definition.Description != "" {
					descParts = append(descParts, definition.Description)
				}
				if params.Description != "" {
					descParts = append(descParts, params.Description)
				}
				fullPrompt = fmt.Sprintf("%s\n\nTask: %s", strings.Join(descParts, "\n"), params.Prompt)
			}

			result, err := agent.Run(ctx, SessionAgentCall{
				SessionID:        session.ID,
				Prompt:           fullPrompt,
				MaxOutputTokens:  maxTokens,
				ProviderOptions:  getProviderOptions(model, providerCfg),
				Temperature:      model.ModelCfg.Temperature,
				TopP:             model.ModelCfg.TopP,
				TopK:             model.ModelCfg.TopK,
				FrequencyPenalty: model.ModelCfg.FrequencyPenalty,
				PresencePenalty:  model.ModelCfg.PresencePenalty,
			})
			if err != nil {
				return fantasy.NewTextErrorResponse("error generating response"), nil
			}

			// Stop forwarding logs
			progressCancel()

			updatedSession, err := c.sessions.Get(ctx, session.ID)
			if err != nil {
				return fantasy.ToolResponse{}, fmt.Errorf("error getting session: %s", err)
			}
			parentSession, err := c.sessions.Get(ctx, sessionID)
			if err != nil {
				return fantasy.ToolResponse{}, fmt.Errorf("error getting parent session: %s", err)
			}

			parentSession.Cost += updatedSession.Cost

			if _, err = c.sessions.Save(ctx, parentSession); err != nil {
				return fantasy.ToolResponse{}, fmt.Errorf("error saving parent session: %s", err)
			}

			return fantasy.NewTextResponse(result.Response.Content.Text()), nil
		},
	), nil
}

func selectSubAgentDefinition(defs []SubAgentDefinition, requested string) (SubAgentDefinition, error) {
	name := strings.TrimSpace(requested)
	if name == "" {
		name = "general-purpose"
	}
	for _, d := range defs {
		if d.Name == name {
			return d, nil
		}
	}
	var names []string
	for _, d := range defs {
		names = append(names, d.Name)
	}
	return SubAgentDefinition{}, fmt.Errorf("unknown subagent_type %q; available: %s", name, strings.Join(names, ", "))
}

func intersectTools(base, mask []string) []string {
	if len(mask) == 0 {
		return base
	}
	allowed := map[string]struct{}{}
	for _, m := range mask {
		allowed[m] = struct{}{}
	}
	var out []string
	for _, b := range base {
		if _, ok := allowed[b]; ok {
			out = append(out, b)
		}
	}
	return out
}

// validateSubagentType checks if the requested subagent type is available.
func validateSubagentType(requested string, defs []SubAgentDefinition) error {
	name := strings.TrimSpace(requested)
	if name == "" {
		return nil // Empty is allowed, will use default
	}

	for _, d := range defs {
		if d.Name == name {
			return nil
		}
	}

	var names []string
	for _, d := range defs {
		names = append(names, d.Name)
	}
	return fmt.Errorf("unknown subagent_type %q; available: %s", name, strings.Join(names, ", "))
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

func sortedModelTypes(models map[config.SelectedModelType]config.SelectedModel) []string {
	if len(models) == 0 {
		return []string{}
	}
	keys := make([]string, 0, len(models))
	for k := range models {
		keys = append(keys, string(k))
	}
	slices.Sort(keys)
	return keys
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
