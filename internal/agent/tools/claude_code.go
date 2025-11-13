package tools

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/orchestrator"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/humanlayer/humanlayer/claudecode-go"
)

type ClaudeCodeParams struct {
	Query              string   `json:"query" description:"The task or question for Claude Code to perform"`
	Model              string   `json:"model,omitempty" description:"Claude model to use (opus, sonnet, haiku). Defaults to sonnet"`
	WorkingDir         string   `json:"working_dir,omitempty" description:"Working directory for Claude Code operations (defaults to current directory)"`
	MaxTurns           int      `json:"max_turns,omitempty" description:"Maximum number of turns for the session. Defaults to 10"`
	AllowedTools       []string `json:"allowed_tools,omitempty" description:"List of tools Claude Code is allowed to use. Defaults to all built-in tools"`
	DisallowedTools    []string `json:"disallowed_tools,omitempty" description:"List of tools Claude Code is not allowed to use"`
	CustomInstructions string   `json:"custom_instructions,omitempty" description:"Custom instructions to prepend to the system prompt"`
	SessionID          string   `json:"session_id,omitempty" description:"Resume an existing session by providing its ID"`
	ForkSession        bool     `json:"fork_session,omitempty" description:"If true with session_id, forks instead of resuming"`
	Verbose            bool     `json:"verbose,omitempty" description:"Enable verbose output"`

	// Multi-turn orchestration options (MVP)
	EnableOrchestrator bool `json:"enable_orchestrator,omitempty" description:"Enable multi-turn orchestration for complex tasks"`
	MaxIterations      int  `json:"max_iterations,omitempty" description:"Maximum orchestrator iterations. Defaults to 3"`
}

type ClaudeCodeResponse struct {
	Result     string  `json:"result"`
	SessionID  string  `json:"session_id"`
	CostUSD    float64 `json:"cost_usd"`
	DurationMS int     `json:"duration_ms"`
	NumTurns   int     `json:"num_turns"`
	IsError    bool    `json:"is_error"`
	Error      string  `json:"error,omitempty"`
	ModelUsed  string  `json:"model_used"`
}

const (
	ClaudeCodeToolName = "claude_code"
	DefaultMaxTurns    = 10
)

type claudeCodeClient interface {
	LaunchAndWait(claudecode.SessionConfig) (*claudecode.Result, error)
}

var newClaudeCodeClient = func() (claudeCodeClient, error) {
	return claudecode.NewClient()
}

//go:embed claude_code.md
var claudeCodeDescription []byte

func NewClaudeCodeTool(permissions permission.Service, workingDir string) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		ClaudeCodeToolName,
		string(claudeCodeDescription),
		func(ctx context.Context, params ClaudeCodeParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			// Check if orchestrator is enabled
			if params.EnableOrchestrator {
				return runWithOrchestrator(ctx, params, call, permissions, workingDir)
			}

			return runSimple(ctx, params, call, permissions, workingDir)
		},
	)
}

// runSimple runs a simple single-turn Claude Code execution
func runSimple(
	ctx context.Context,
	params ClaudeCodeParams,
	call fantasy.ToolCall,
	permissions permission.Service,
	workingDir string,
) (fantasy.ToolResponse, error) {
	// Validate required parameters
	if strings.TrimSpace(params.Query) == "" {
		return fantasy.NewTextErrorResponse("query is required and cannot be empty"), nil
	}

	// Set defaults
	if params.MaxTurns <= 0 {
		params.MaxTurns = DefaultMaxTurns
	}

	// Validate and determine working directory
	execWorkingDir, err := validateWorkingDir(workingDir, params.WorkingDir)
	if err != nil {
		return fantasy.NewTextErrorResponse(fmt.Sprintf("invalid working directory: %v", err)), nil
	}

	// Check permissions
	sessionID := GetSessionFromContext(ctx)
	if sessionID == "" {
		return fantasy.ToolResponse{}, fmt.Errorf("session ID is required for Claude Code operations")
	}

	permReq := permission.CreatePermissionRequest{
		SessionID:   sessionID,
		Path:        execWorkingDir,
		ToolCallID:  call.ID,
		ToolName:    ClaudeCodeToolName,
		Action:      "execute",
		Description: fmt.Sprintf("Claude Code: %s", params.Query),
		Params:      params,
	}

	if !permissions.Request(permReq) {
		return fantasy.ToolResponse{}, permission.ErrorPermissionDenied
	}

	// Configure session
	sessionConfig := claudecode.SessionConfig{
		Query:              params.Query,
		WorkingDir:         execWorkingDir,
		MaxTurns:           params.MaxTurns,
		CustomInstructions: params.CustomInstructions,
		Verbose:            params.Verbose,
	}

	// Set model if specified
	if params.Model != "" {
		switch strings.ToLower(params.Model) {
		case "opus":
			sessionConfig.Model = claudecode.ModelOpus
		case "sonnet":
			sessionConfig.Model = claudecode.ModelSonnet
		case "haiku":
			sessionConfig.Model = claudecode.ModelHaiku
		default:
			return fantasy.NewTextErrorResponse(fmt.Sprintf("invalid model: %s. Must be opus, sonnet, or haiku", params.Model)), nil
		}
	} else {
		sessionConfig.Model = claudecode.ModelSonnet // Default to sonnet
	}

	// Set session management options
	if params.SessionID != "" {
		sessionConfig.SessionID = params.SessionID
		sessionConfig.ForkSession = params.ForkSession
	}

	// Set tool permissions
	if len(params.AllowedTools) > 0 {
		sessionConfig.AllowedTools = params.AllowedTools
	}
	if len(params.DisallowedTools) > 0 {
		sessionConfig.DisallowedTools = params.DisallowedTools
	}

	// Use JSON output format for structured response
	sessionConfig.OutputFormat = claudecode.OutputJSON

	// Create Claude Code client
	client, err := newClaudeCodeClient()
	if err != nil {
		return fantasy.NewTextErrorResponse(fmt.Sprintf("failed to create Claude Code client: %v", err)), nil
	}

	// Launch Claude Code session
	result, err := client.LaunchAndWait(sessionConfig)
	if err != nil {
		return fantasy.NewTextErrorResponse(fmt.Sprintf("Claude Code session failed: %v", err)), nil
	}

	// Build response
	modelUsed := "unknown"
	if _, ok := result.ModelUsage["claude-3.5-sonnet"]; ok {
		modelUsed = "claude-sonnet"
	} else if _, ok := result.ModelUsage["claude-3-sonnet"]; ok {
		modelUsed = "claude-sonnet"
	} else if _, ok := result.ModelUsage["claude-3.5-haiku"]; ok {
		modelUsed = "claude-haiku"
	} else if _, ok := result.ModelUsage["claude-3-haiku"]; ok {
		modelUsed = "claude-haiku"
	} else if _, ok := result.ModelUsage["claude-3-opus"]; ok {
		modelUsed = "claude-opus"
	}

	response := ClaudeCodeResponse{
		Result:     result.Result,
		SessionID:  result.SessionID,
		CostUSD:    result.CostUSD,
		DurationMS: result.DurationMS,
		NumTurns:   result.NumTurns,
		IsError:    result.IsError,
		ModelUsed:  modelUsed,
	}

	if result.Error != "" {
		response.Error = result.Error
	}

	// Return structured response
	responseJSON, err := json.Marshal(response)
	if err != nil {
		return fantasy.NewTextErrorResponse(fmt.Sprintf("failed to marshal response: %v", err)), nil
	}

	return fantasy.ToolResponse{
		Content: string(responseJSON),
	}, nil
}

// runWithOrchestrator runs Claude Code with multi-turn orchestration
func runWithOrchestrator(
	ctx context.Context,
	params ClaudeCodeParams,
	call fantasy.ToolCall,
	permissions permission.Service,
	workingDir string,
) (fantasy.ToolResponse, error) {
	// Validate required parameters
	if strings.TrimSpace(params.Query) == "" {
		return fantasy.NewTextErrorResponse("query is required and cannot be empty"), nil
	}

	// Validate and determine working directory
	execWorkingDir, err := validateWorkingDir(workingDir, params.WorkingDir)
	if err != nil {
		return fantasy.NewTextErrorResponse(fmt.Sprintf("invalid working directory: %v", err)), nil
	}

	// Check permissions for the orchestrator itself
	sessionID := GetSessionFromContext(ctx)
	if sessionID == "" {
		return fantasy.ToolResponse{}, fmt.Errorf("session ID is required for Claude Code operations")
	}

	permReq := permission.CreatePermissionRequest{
		SessionID:   sessionID,
		Path:        execWorkingDir,
		ToolCallID:  call.ID,
		ToolName:    ClaudeCodeToolName,
		Action:      "execute_with_orchestrator",
		Description: fmt.Sprintf("Claude Code with orchestration: %s", params.Query),
		Params:      params,
	}

	if !permissions.Request(permReq) {
		return fantasy.ToolResponse{}, permission.ErrorPermissionDenied
	}

	// Create and run orchestrator
	orch := orchestrator.NewClaudeCodeOrchestrator(permissions, execWorkingDir)

	// Run multi-turn orchestration
	result, err := orch.Run(ctx, sessionID, params.Query)
	if err != nil {
		return fantasy.NewTextErrorResponse(fmt.Sprintf("Orchestrator failed: %v", err)), nil
	}

	// Build enhanced response with orchestration metadata
	response := struct {
		ClaudeCodeResponse
		OrchestratorIterations int      `json:"orchestrator_iterations"`
		History               []string `json:"history,omitempty"`
	}{
		ClaudeCodeResponse: ClaudeCodeResponse{
			Result:     result.FinalResult,
			SessionID:  result.SessionID,
			CostUSD:    result.TotalCost,
			DurationMS: int(result.TotalDuration.Milliseconds()),
			NumTurns:   result.TotalTurns,
			IsError:    result.IsError,
			Error:      result.Error,
			ModelUsed:  "claude-sonnet", // Default for now
		},
		OrchestratorIterations: result.Iterations,
	}

	// For debugging, include step count in history
	if params.Verbose && len(result.History) > 0 {
		response.History = make([]string, len(result.History))
		for i := range result.History {
			response.History[i] = fmt.Sprintf("Step %d complete", i+1)
		}
	}

	// Return structured response
	responseJSON, err := json.Marshal(response)
	if err != nil {
		return fantasy.NewTextErrorResponse(fmt.Sprintf("failed to marshal response: %v", err)), nil
	}

	return fantasy.ToolResponse{
		Content: string(responseJSON),
	}, nil
}

// validateWorkingDir validates and resolves the working directory path
// It prevents directory traversal attacks by ensuring the resolved path
// stays within the base working directory
func validateWorkingDir(baseDir string, subDir string) (string, error) {
	// If no subdirectory specified, return base directory
	if strings.TrimSpace(subDir) == "" {
		return baseDir, nil
	}

	// Resolve the absolute path of base directory
	absBaseDir, err := filepath.Abs(baseDir)
	if err != nil {
		return "", fmt.Errorf("invalid base directory: %v", err)
	}

	// Join and resolve the target path
	targetPath := filepath.Join(absBaseDir, subDir)
	absTargetPath, err := filepath.Abs(targetPath)
	if err != nil {
		return "", fmt.Errorf("invalid target path: %v", err)
	}

	// Check for path traversal (ensure target stays within base directory)
	if !strings.HasPrefix(absTargetPath, absBaseDir) {
		return "", fmt.Errorf("working directory %s is outside allowed base directory %s", subDir, baseDir)
	}

	// Additional check: prevent symlinks from escaping base directory
	// (This is a best-effort check; full protection requires more OS-specific handling)
	info, err := os.Lstat(absTargetPath)
	if err == nil && info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("symlinks are not allowed in working directory path")
	}

	return absTargetPath, nil
}
