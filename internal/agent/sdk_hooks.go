package agent

import (
	"context"
	"log/slog"

	"github.com/M1n9X/claude-agent-sdk-go/types"
)

// logToolExecution is a hook that logs tool execution events
func logToolExecution(ctx context.Context, input interface{}, toolUseID *string, hookCtx types.HookContext) (interface{}, error) {
	toolID := "unknown"
	if toolUseID != nil {
		toolID = *toolUseID
	}

	// Extract tool name from input if it's a PreToolUseHookInput
	toolName := "unknown"
	if preToolInput, ok := input.(*types.PreToolUseHookInput); ok {
		toolName = preToolInput.ToolName
	}

	slog.Debug("Tool execution started",
		"tool", toolName,
		"tool_use_id", toolID,
	)
	return nil, nil // Return nil to continue normal execution
}

// logToolCompletion is a hook that logs when a tool completes execution
func logToolCompletion(ctx context.Context, input interface{}, toolUseID *string, hookCtx types.HookContext) (interface{}, error) {
	toolID := "unknown"
	if toolUseID != nil {
		toolID = *toolUseID
	}

	// Extract tool name from input if it's a PostToolUseHookInput
	toolName := "unknown"
	if postToolInput, ok := input.(*types.PostToolUseHookInput); ok {
		toolName = postToolInput.ToolName
	}

	slog.Debug("Tool execution completed",
		"tool", toolName,
		"tool_use_id", toolID,
	)
	return nil, nil
}

// buildDefaultHooks creates the default hook configuration for tool execution logging
func buildDefaultHooks() map[types.HookEvent][]types.HookMatcher {
	return map[types.HookEvent][]types.HookMatcher{
		types.HookEventPreToolUse: {{
			Matcher: nil, // Match all tools
			Hooks:   []types.HookCallbackFunc{logToolExecution},
		}},
		types.HookEventPostToolUse: {{
			Matcher: nil, // Match all tools
			Hooks:   []types.HookCallbackFunc{logToolCompletion},
		}},
	}
}
