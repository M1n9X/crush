package orchestrator

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/humanlayer/humanlayer/claudecode-go"
)

// Color codes
const (
	reset       = "\033[0m"
	bold        = "\033[1m"
	dim         = "\033[2m"
	italic      = "\033[3m"
	underline   = "\033[4m"

	black       = "\033[30m"
	red         = "\033[31m"
	green       = "\033[32m"
	yellow      = "\033[33m"
	blue        = "\033[34m"
	magenta     = "\033[35m"
	cyan        = "\033[36m"
	white       = "\033[37m"

	brightBlack = "\033[90m"
	brightRed   = "\033[91m"
	brightGreen = "\033[92m"
	brightYellow = "\033[93m"
	brightBlue  = "\033[94m"
	brightMagenta = "\033[95m"
	brightCyan  = "\033[96m"
	brightWhite = "\033[97m"
	bgBrightBlack = "\033[100m"
)

// StreamRenderer renders streaming events to terminal
type StreamRenderer struct {
	// State tracking
	mu                 sync.Mutex
	toolUses           []ToolUseInfo
	currentToolUse     *ToolUseInfo
	thinkingInProgress bool
	lastWasThinking    bool
}

// ToolUseInfo tracks tool usage
type ToolUseInfo struct {
	ID        string
	Name      string
	Input     interface{}
	Output    string
	IsError   bool
	Timestamp time.Time
}

// NewStreamRenderer creates a new stream renderer
func NewStreamRenderer() *StreamRenderer {
	return &StreamRenderer{
		toolUses: []ToolUseInfo{},
	}
}

// colorize applies color to text
func colorize(color, text string) string {
	return color + text + reset
}

// RenderEvent renders a single event
func (r *StreamRenderer) RenderEvent(event claudecode.StreamEvent) {
	r.mu.Lock()
	defer r.mu.Unlock()

	switch event.Type {
	case "system":
		r.renderSystemEvent(event)
	case "assistant":
		r.renderAssistantEvent(event)
	case "user":
		r.renderUserEvent(event)
	case "result":
		r.renderResultEvent(event)
	default:
		// Unknown event type
		fmt.Printf("\n%s %s\n",
			colorize(dim, "?"),
			colorize(dim, fmt.Sprintf("Unknown event type: %s", event.Type)))
	}
}

func (r *StreamRenderer) renderSystemEvent(event claudecode.StreamEvent) {
	if event.Subtype == "init" {
		// Model info
		fmt.Printf("\n%s %s %s\n\n",
			colorize(brightCyan, "▲"),
			colorize(brightCyan, "Claude Code"),
			colorize(dim, fmt.Sprintf("(model: %s)", event.Model)))

		// Available tools
		if len(event.Tools) > 0 {
			fmt.Printf("  %s Available tools:\n", colorize(brightYellow, "◆"))
			toolsList := formatToolsList(event.Tools)
			fmt.Printf("    %s\n", colorize(dim, toolsList))
			fmt.Println()
		}
	}
}

func (r *StreamRenderer) renderAssistantEvent(event claudecode.StreamEvent) {
	if event.Message == nil {
		return
	}

	// Parse content
	for _, content := range event.Message.Content {
		switch content.Type {
		case "thinking":
			if !r.thinkingInProgress {
				fmt.Printf("\n%s ", colorize(dim, "💭"))
				r.thinkingInProgress = true
				r.lastWasThinking = true
			}
			// Print thinking on same line
			fmt.Print(colorize(italic+dim, "thinking..."))
			r.lastWasThinking = true

		case "text":
			if r.thinkingInProgress {
				fmt.Println() // End thinking line
				r.thinkingInProgress = false
			}
			if r.lastWasThinking {
				fmt.Println() // Add space after thinking
			}

			// Print text
			text := content.Text
			if len(text) > 0 {
				wrapped := wrapText(text, 80)
				lines := strings.Split(wrapped, "\n")
				for i, line := range lines {
					if i == 0 {
						fmt.Printf("%s %s\n",
							colorize(brightMagenta, "●"),
							colorize(brightMagenta, line))
					} else {
						fmt.Printf("  %s\n",
							colorize(brightMagenta, line))
					}
				}
			}
			r.lastWasThinking = false

		case "tool_use":
			// Tool use notification
			if r.thinkingInProgress {
				fmt.Println() // End thinking line
				r.thinkingInProgress = false
			}
			fmt.Printf("\n  %s Using tool:\n    %s %s\n",
				colorize(brightYellow, "→"),
				colorize(brightYellow, content.Name),
				colorize(dim, fmt.Sprintf("(id: %s)", content.ID)))
			r.lastWasThinking = false

			// Track tool use
			toolUse := ToolUseInfo{
				ID:        content.ID,
				Name:      content.Name,
				Input:     content.Input,
				Timestamp: time.Now(),
			}
			r.toolUses = append(r.toolUses, toolUse)
			r.currentToolUse = &r.toolUses[len(r.toolUses)-1]
		}
	}
}

func (r *StreamRenderer) renderUserEvent(event claudecode.StreamEvent) {
	if event.Message == nil {
		return
	}
	// Usually tool results
	for _, content := range event.Message.Content {
		if content.Type == "tool_result" {
			fmt.Printf("  %s %s\n",
				colorize(brightGreen, "✓"),
				colorize(dim, "Tool execution complete"))
		}
	}
}

func (r *StreamRenderer) renderResultEvent(event claudecode.StreamEvent) {
	// End thinking if in progress
	if r.thinkingInProgress {
		fmt.Println()
		r.thinkingInProgress = false
	}

	fmt.Printf("\n%s %s\n",
		colorize(brightBlue, "▼"),
		colorize(brightBlue+bold, "Result"))

	// Cost
	if event.CostUSD > 0 {
		fmt.Printf("  %s Cost: %s\n",
			colorize(dim, "•"),
			colorize(brightYellow, fmt.Sprintf("$%.4f", event.CostUSD)))
	}

	// Duration
	if event.DurationMS > 0 {
		duration := time.Duration(event.DurationMS) * time.Millisecond
		fmt.Printf("  %s Duration: %s\n",
			colorize(dim, "•"),
			colorize(dim, duration.Round(time.Millisecond).String()))
	}

	// Turns
	if event.NumTurns > 0 {
		fmt.Printf("  %s Turns: %d\n",
			colorize(dim, "•"),
			event.NumTurns)
	}

	// Model usage
	if len(event.ModelUsage) > 0 {
		fmt.Printf("  %s Model usage:\n", colorize(dim, "•"))
		for model, usage := range event.ModelUsage {
			costStr := ""
			if usage.CostUSD > 0 {
				costStr = fmt.Sprintf(" (%.4f USD)", usage.CostUSD)
			}
			fmt.Printf("    • %s: %d in, %d out%s\n",
				colorize(dim, model),
				usage.InputTokens,
				usage.OutputTokens,
				colorize(dim, costStr))
		}
	}

	// Permission denials
	if event.PermissionDenials != nil && len(event.PermissionDenials.Denials) > 0 {
		fmt.Printf("  %s %s:\n",
			colorize(red, "✗"),
			colorize(red, "Permission denials"))
		for _, denial := range event.PermissionDenials.Denials {
			fmt.Printf("    • %s\n",
				colorize(red, denial.ToolName))
		}
	}

	// Error
	if event.IsError && event.Error != "" {
		fmt.Printf("\n%s %s\n\n%s\n",
			colorize(brightRed, "✗"),
			colorize(brightRed, "Error occurred:"),
			colorize(red, event.Error))
	}

	fmt.Println()
}

// RenderSummary renders final summary
func (r *StreamRenderer) RenderSummary(totalCost float64, totalDuration time.Duration, totalTurns int) {
	fmt.Printf("\n%s\n",
		colorize(brightBlue, "═══════════════════════════════════════════"))
	fmt.Printf("          %s\n",
		colorize(brightBlue+bold, "Session Complete"))
	fmt.Printf("%s\n\n",
		colorize(brightBlue, "═══════════════════════════════════════════"))

	if totalCost > 0 {
		fmt.Printf("Total Cost: %s\n",
			colorize(brightYellow, fmt.Sprintf("$%.4f", totalCost)))
	}
	if totalDuration > 0 {
		fmt.Printf("Duration: %s\n",
			colorize(dim, totalDuration.Round(time.Millisecond).String()))
	}
	if totalTurns > 0 {
		fmt.Printf("Total Turns: %d\n",
			totalTurns)
	}

	if len(r.toolUses) > 0 {
		fmt.Printf("Tools Used: %d\n",
			len(r.toolUses))
	}

	fmt.Println()
}

// Helper functions

func wrapText(text string, width int) string {
	if len(text) <= width {
		return text
	}

	var lines []string
	words := strings.Fields(text)
	if len(words) == 0 {
		return text
	}

	currentLine := words[0]
	for _, word := range words[1:] {
		if len(currentLine)+len(word)+1 <= width {
			currentLine += " " + word
		} else {
			lines = append(lines, currentLine)
			currentLine = word
		}
	}
	lines = append(lines, currentLine)

	return strings.Join(lines, "\n")
}

func formatToolsList(tools []string) string {
	if len(tools) <= 5 {
		return strings.Join(tools, ", ")
	}
	return strings.Join(tools[:5], ", ") + "..."
}
