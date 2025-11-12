// Example demonstrating Claude Code integration with Crush
// This file shows how to use the Claude Code tool programmatically

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/pubsub"
)

// Simple permission service for demo
type demoPermissionService struct{}

func (d *demoPermissionService) Request(opts permission.CreatePermissionRequest) bool {
	fmt.Printf("Permission requested for: %s\n", opts.Description)
	return true // Allow all for demo
}

func (d *demoPermissionService) GrantPersistent(permission permission.PermissionRequest) {}
func (d *demoPermissionService) Grant(permission permission.PermissionRequest)           {}
func (d *demoPermissionService) Deny(permission permission.PermissionRequest)            {}
func (d *demoPermissionService) Subscribe(context.Context) <-chan pubsub.Event[permission.PermissionRequest] {
	return make(<-chan pubsub.Event[permission.PermissionRequest])
}
func (d *demoPermissionService) Unsubscribe(context.Context)         {}
func (d *demoPermissionService) AutoApproveSession(sessionID string) {}
func (d *demoPermissionService) SetSkipRequests(skip bool)           {}
func (d *demoPermissionService) SkipRequests() bool                  { return false }
func (d *demoPermissionService) SubscribeNotifications(ctx context.Context) <-chan pubsub.Event[permission.PermissionNotification] {
	return make(<-chan pubsub.Event[permission.PermissionNotification])
}

func main() {
	fmt.Println("🚀 Claude Code Integration Example")
	fmt.Println("==================================")

	// Create permission service
	permService := &demoPermissionService{}

	// Get current working directory
	workingDir, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get working directory: %v", err)
	}

	// Create Claude Code tool
	claudeTool := tools.NewClaudeCodeTool(permService, workingDir)

	// Example 1: Simple code generation
	fmt.Println("\n📋 Example 1: Simple Go HTTP Server")
	fmt.Println("-----------------------------------")

	params1 := tools.ClaudeCodeParams{
		Query:    "Create a simple Go HTTP server with a /hello endpoint that returns JSON response",
		Model:    "sonnet",
		MaxTurns: 5,
	}

	input1, _ := json.Marshal(params1)
	ctx := context.WithValue(context.Background(), tools.SessionIDContextKey, "example-session-1")

	result1, err := claudeTool.Run(ctx, fantasy.ToolCall{
		ID:    "example-call-1",
		Name:  tools.ClaudeCodeToolName,
		Input: string(input1),
	})

	if err != nil {
		log.Printf("Example 1 failed: %v", err)
	} else {
		var response tools.ClaudeCodeResponse
		if err := json.Unmarshal([]byte(result1.Content), &response); err != nil {
			log.Printf("Failed to parse response: %v", err)
		} else {
			fmt.Printf("✅ Session ID: %s\n", response.SessionID)
			fmt.Printf("💰 Cost: $%.4f\n", response.CostUSD)
			fmt.Printf("⏱️  Duration: %dms\n", response.DurationMS)
			fmt.Printf("🔄 Turns: %d\n", response.NumTurns)
			fmt.Printf("📝 Result preview: %.200s...\n", response.Result)

			// Save the full result
			resultFile := filepath.Join(workingDir, "claude_code_example_1.md")
			if err := os.WriteFile(resultFile, []byte(response.Result), 0644); err != nil {
				log.Printf("Failed to save result: %v", err)
			} else {
				fmt.Printf("💾 Full result saved to: %s\n", resultFile)
			}
		}
	}

	// Example 2: Code analysis with restrictions
	fmt.Println("\n📋 Example 2: Code Analysis (Read-only)")
	fmt.Println("---------------------------------------")

	params2 := tools.ClaudeCodeParams{
		Query:           "Analyze the project structure and identify the main components",
		Model:           "sonnet",
		AllowedTools:    []string{"bash", "glob", "view", "ls"},
		DisallowedTools: []string{"edit", "write", "multiedit"},
		MaxTurns:        3,
	}

	input2, _ := json.Marshal(params2)
	ctx2 := context.WithValue(context.Background(), tools.SessionIDContextKey, "example-session-2")

	result2, err := claudeTool.Run(ctx2, fantasy.ToolCall{
		ID:    "example-call-2",
		Name:  tools.ClaudeCodeToolName,
		Input: string(input2),
	})

	if err != nil {
		log.Printf("Example 2 failed: %v", err)
	} else {
		var response tools.ClaudeCodeResponse
		if err := json.Unmarshal([]byte(result2.Content), &response); err != nil {
			log.Printf("Failed to parse response: %v", err)
		} else {
			fmt.Printf("✅ Session ID: %s\n", response.SessionID)
			fmt.Printf("💰 Cost: $%.4f\n", response.CostUSD)
			fmt.Printf("⏱️  Duration: %dms\n", response.DurationMS)
			fmt.Printf("🔄 Turns: %d\n", response.NumTurns)
			fmt.Printf("📊 Analysis: %.300s...\n", response.Result)
		}
	}

	fmt.Println("\n🎉 Examples completed!")
	fmt.Println("======================")
	fmt.Println("Check the output files and session results above.")
	fmt.Println("The Claude Code tool is now integrated into Crush!")
}
