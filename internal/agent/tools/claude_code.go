package tools

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"charm.land/fantasy"
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
			// Validate required parameters
			if strings.TrimSpace(params.Query) == "" {
				return fantasy.NewTextErrorResponse("query is required and cannot be empty"), nil
			}

			// Set defaults
			if params.MaxTurns <= 0 {
				params.MaxTurns = DefaultMaxTurns
			}

			// Determine working directory
			execWorkingDir := workingDir
			if params.WorkingDir != "" {
				execWorkingDir = filepath.Join(workingDir, params.WorkingDir)
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
		},
	)
}
