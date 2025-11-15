// Example demonstrating Claude Code Orchestrator for multi-turn interactions
// This shows how to use the orchestrator to handle complex tasks automatically

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/pubsub"
)

// Simple permission service for demo
type demoPermissionService struct{}

func (d *demoPermissionService) Request(opts permission.CreatePermissionRequest) bool {
	fmt.Printf("✅ Permission granted for: %s\n", opts.Description)
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

func main() {
	fmt.Println("🚀 Claude Code Orchestrator Example")
	fmt.Println("====================================")

	// Create permission service
	permService := &demoPermissionService{}

	// Create workspace directory if it doesn't exist
	workspaceDir := "./claude_code_orchestrator_workspace"
	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		log.Printf("❌ Failed to create workspace directory: %v\n", err)
		return
	}

	// Create Claude Code tool
	claudeTool := tools.NewClaudeCodeTool(permService, workspaceDir)

	// Example: Multi-turn orchestration for complex task
	fmt.Println("📋 Example: Multi-turn Code Generation with Orchestration")
	fmt.Println("---------------------------------------------------------")
	fmt.Println("Task: Create a complete Go web application with router, handlers, and tests")
	fmt.Println()

	params := tools.ClaudeCodeParams{
		Query:              "Create a complete Go web application with a router, multiple handlers, and write tests for them. Start by creating the main server file.",
		Model:              "sonnet",
		EnableOrchestrator: true,
		MaxIterations:      3,
		Verbose:            true,
		MaxTurns:           15,
	}

	input, _ := json.Marshal(params)

	// Create mock context with session ID
	ctx := context.WithValue(context.Background(), tools.SessionIDContextKey, "orchestrator-demo-session")

	result, err := claudeTool.Run(ctx, fantasy.ToolCall{
		ID:    "orchestrator-call-1",
		Name:  tools.ClaudeCodeToolName,
		Input: string(input),
	})

	if err != nil {
		log.Printf("❌ Orchestrator example failed: %v\n", err)
		return
	}

	fmt.Println("\n✅ Orchestration Complete!")
	fmt.Println("==========================")

	// Parse response
	var response struct {
		Result                 string   `json:"result"`
		SessionID              string   `json:"session_id"`
		CostUSD                float64  `json:"cost_usd"`
		DurationMS             int      `json:"duration_ms"`
		NumTurns               int      `json:"num_turns"`
		OrchestratorIterations int      `json:"orchestrator_iterations"`
		IsError                bool     `json:"is_error"`
		Error                  string   `json:"error,omitempty"`
		History                []string `json:"history,omitempty"`
	}

	if err := json.Unmarshal([]byte(result.Content), &response); err != nil {
		log.Printf("❌ Failed to parse response: %v\n", err)
		return
	}

	fmt.Printf("✨ Result: %.200s...\n", response.Result)
	fmt.Printf("💰 Total Cost: $%.4f\n", response.CostUSD)
	fmt.Printf("⏱️  Total Duration: %dms\n", response.DurationMS)
	fmt.Printf("🔄 Total Turns: %d\n", response.NumTurns)
	fmt.Printf("🔄 Orchestrator Iterations: %d\n", response.OrchestratorIterations)
	fmt.Printf("📊 Claude Session ID: %s\n", response.SessionID)

	if len(response.History) > 0 {
		fmt.Println("\n📋 Execution History:")
		for _, step := range response.History {
			fmt.Printf("  - %s\n", step)
		}
	}

	fmt.Println("\n🎉 Multi-turn orchestration completed successfully!")
	fmt.Println("\nDifference from single-turn:")
	fmt.Println("  ✓ Automatically continued when more work was needed")
	fmt.Println("  ✓ Maintained context across multiple Claude Code sessions")
	fmt.Println("  ✓ Returned only after task was fully completed")
	fmt.Println("  ✓ Tracked execution history across all iterations")
}
