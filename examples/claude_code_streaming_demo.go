// Demo: Claude Code with Real-time Streaming Output
// This demonstrates the streaming output capability of the Claude Code Orchestrator

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/pubsub"
)

// Simple permission service for demo (always grants)
type demoPermissionService struct{}

func (d *demoPermissionService) Request(opts permission.CreatePermissionRequest) bool {
	fmt.Printf("\n%s %s\n\n",
		colorize(brightGreen+dim, "✓"),
		colorize(dim, "Permission granted:"))
	fmt.Printf("  %s %s\n\n",
		colorize(dim, "•"),
		colorize(dim, opts.Description))
	return true
}

func (d *demoPermissionService) GrantPersistent(permission permission.PermissionRequest) {}
func (d *demoPermissionService) Grant(permission permission.PermissionRequest)           {}
func (d *demoPermissionService) Deny(permission permission.PermissionRequest)            {}
func (d *demoPermissionService) AutoApproveSession(sessionID string)                     {}
func (d *demoPermissionService) SetSkipRequests(skip bool)                               {}
func (d *demoPermissionService) SkipRequests() bool                                      { return false }
func (d *demoPermissionService) Subscribe(ctx context.Context) <-chan pubsub.Event[permission.PermissionRequest] {
	return make(<-chan pubsub.Event[permission.PermissionRequest])
}
func (d *demoPermissionService) SubscribeNotifications(ctx context.Context) <-chan pubsub.Event[permission.PermissionNotification] {
	return make(<-chan pubsub.Event[permission.PermissionNotification])
}

// Color codes
const (
	reset     = "\033[0m"
	bold      = "\033[1m"
	dim       = "\033[2m"
	italic    = "\033[3m"
	underline = "\033[4m"

	black   = "\033[30m"
	red     = "\033[31m"
	green   = "\033[32m"
	yellow  = "\033[33m"
	blue    = "\033[34m"
	magenta = "\033[35m"
	cyan    = "\033[36m"
	white   = "\033[37m"

	brightBlack   = "\033[90m"
	brightRed     = "\033[91m"
	brightGreen   = "\033[92m"
	brightYellow  = "\033[93m"
	brightBlue    = "\033[94m"
	brightMagenta = "\033[95m"
	brightCyan    = "\033[96m"
	brightWhite   = "\033[97m"
)

func colorize(color, text string) string {
	return color + text + reset
}

func main() {
	fmt.Println()
	fmt.Printf("%s %s\n", colorize(brightCyan+bold, "🚀"), colorize(brightCyan+bold, "Claude Code Streaming Demo"))
	fmt.Printf("%s\n\n", colorize(dim, "═══════════════════════════════════════════"))

	fmt.Printf("This demo shows real-time streaming of Claude Code execution.\n")
	fmt.Printf("You'll see:\n")
	fmt.Printf("  %s System initialization\n", colorize(brightCyan, "▲"))
	fmt.Printf("  %s Assistant thinking and messages\n", colorize(brightMagenta, "●"))
	fmt.Printf("  %s Tool usage (Bash, Write, Read, etc.)\n", colorize(brightYellow, "→"))
	fmt.Printf("  %s Real-time progress updates\n", colorize(brightGreen, "✓"))
	fmt.Printf("  %s Final results and metrics\n", colorize(brightBlue, "▼"))
	fmt.Println()

	// Wait for user to press enter
	fmt.Printf("%s Press Enter to start...", colorize(dim, "..."))
	var input string
	fmt.Scanln(&input)
	fmt.Println()

	// Create permission service
	permService := &demoPermissionService{}

	// Create workspace directory if it doesn't exist
	workspaceDir := "./claude_code_streaming_workspace"
	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		log.Fatalf("Failed to create workspace directory: %v", err)
	}

	// Create Claude Code tool
	claudeTool := tools.NewClaudeCodeTool(permService, workspaceDir)

	// Example task - something that will use multiple tools
	task := `Create a simple Python script that:
1. Creates a virtual environment
2. Installs the requests library
3. Makes an HTTP GET request to httpbin.org/json
4. Saves the result to a file called data.json
5. Prints the status code and content type

Do this step by step and explain what you're doing.`

	fmt.Printf("\n%s\n", colorize(brightBlue, "📋 Task:"))
	fmt.Printf("%s\n", colorize(white, task))
	fmt.Println()

	params := tools.ClaudeCodeParams{
		Query:              task,
		Model:              "sonnet",
		EnableOrchestrator: true,
		MaxIterations:      1,
		Verbose:            true,
		MaxTurns:           20,
	}

	inputJSON, _ := json.Marshal(params)

	// Create mock context with session ID
	ctx := context.WithValue(context.Background(), tools.SessionIDContextKey, "demo-streaming-session")

	startTime := time.Now()

	// Run the tool
	result, err := claudeTool.Run(ctx, fantasy.ToolCall{
		ID:    "streaming-demo-call-1",
		Name:  tools.ClaudeCodeToolName,
		Input: string(inputJSON),
	})

	duration := time.Since(startTime)

	if err != nil {
		log.Fatalf("\n%s Demo failed: %v\n",
			colorize(brightRed, "✗"),
			colorize(red, err.Error()))
	}

	// Parse response
	var response struct {
		Result                 string  `json:"result"`
		SessionID              string  `json:"session_id"`
		CostUSD                float64 `json:"cost_usd"`
		DurationMS             int     `json:"duration_ms"`
		NumTurns               int     `json:"num_turns"`
		OrchestratorIterations int     `json:"orchestrator_iterations"`
		IsError                bool    `json:"is_error"`
		Error                  string  `json:"error,omitempty"`
	}

	if err := json.Unmarshal([]byte(result.Content), &response); err != nil {
		log.Fatalf("\n%s Failed to parse response: %v\n",
			colorize(brightRed, "✗"),
			colorize(red, err.Error()))
	}

	// Final summary
	fmt.Printf("\n%s\n",
		colorize(brightBlue, "═══════════════════════════════════════════"))
	fmt.Printf("          %s\n",
		colorize(brightBlue+bold, "Demo Complete"))
	fmt.Printf("%s\n\n",
		colorize(brightBlue, "═══════════════════════════════════════════"))

	fmt.Printf("Runtime: %s\n", colorize(dim, duration.Round(time.Millisecond).String()))
	if response.CostUSD > 0 {
		fmt.Printf("Total Cost: %s\n", colorize(brightYellow, fmt.Sprintf("$%.4f", response.CostUSD)))
	}
	fmt.Printf("Total Turns: %d\n", response.NumTurns)
	fmt.Printf("Iterations: %d\n", response.OrchestratorIterations)

	fmt.Printf("\n%s\n\n", colorize(brightGreen, "✓ Streaming output working perfectly!"))
}
