package orchestrator

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/M1n9X/claude-agent-sdk-go/types"
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

// RenderMessage renders a Claude Agent SDK message
func (r *StreamRenderer) RenderMessage(msg types.Message) {
	r.mu.Lock()
	defer r.mu.Unlock()

	switch m := msg.(type) {
	case *types.SystemMessage:
		r.renderSystemMessage(m)
	case *types.AssistantMessage:
		r.renderAssistantMessage(m)
	case *types.UserMessage:
		r.renderUserMessage(m)
	case *types.ResultMessage:
		r.renderResultMessage(m)
	default:
		// Unknown message type
		fmt.Printf("\n%s %s\n",
			colorize(dim, "?"),
			colorize(dim, fmt.Sprintf("Unknown message type: %T", msg)))
	}
}

func (r *StreamRenderer) renderSystemMessage(msg *types.SystemMessage) {
	if msg.Subtype == types.SystemSubtypeInit {
		// Model info
		fmt.Printf("\n%s %s\n\n",
			colorize(brightCyan, "▲"),
			colorize(brightCyan, "Claude Code"))
	}
}

func (r *StreamRenderer) renderAssistantMessage(msg *types.AssistantMessage) {
	for _, content := range msg.Content {
		switch block := content.(type) {
		case *types.TextBlock:
			if r.thinkingInProgress {
				fmt.Println() // End thinking line
				r.thinkingInProgress = false
			}
			if r.lastWasThinking {
				fmt.Println() // Add space after thinking
			}

			// Print text
			text := block.Text
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

		case *types.ThinkingBlock:
			if !r.thinkingInProgress {
				fmt.Printf("\n%s ", colorize(dim, "💭"))
				r.thinkingInProgress = true
				r.lastWasThinking = true
			}
			// Print thinking
			fmt.Print(colorize(italic+dim, "thinking..."))
			r.lastWasThinking = true

		case *types.ToolUseBlock:
			// Tool use notification
			if r.thinkingInProgress {
				fmt.Println() // End thinking line
				r.thinkingInProgress = false
			}
			fmt.Printf("\n  %s Using tool:\n    %s %s\n",
				colorize(brightYellow, "→"),
				colorize(brightYellow, block.Name),
				colorize(dim, fmt.Sprintf("(id: %s)", block.ID)))
			r.lastWasThinking = false

			// Track tool use
			toolUse := ToolUseInfo{
				ID:        block.ID,
				Name:      block.Name,
				Input:     block.Input,
				Timestamp: time.Now(),
			}
			r.toolUses = append(r.toolUses, toolUse)
			r.currentToolUse = &r.toolUses[len(r.toolUses)-1]

		case *types.ToolResultBlock:
			if r.thinkingInProgress {
				fmt.Println() // End thinking line
				r.thinkingInProgress = false
			}
			fmt.Printf("  %s %s\n",
				colorize(brightGreen, "✓"),
				colorize(dim, "Tool execution complete"))
			r.lastWasThinking = false
		}
	}
}

func (r *StreamRenderer) renderUserMessage(msg *types.UserMessage) {
	// Usually tool results
	// Content can be string or []types.ContentBlock
	if contentStr, ok := msg.Content.(string); ok {
		fmt.Printf("  %s %s\n",
			colorize(brightGreen, "✓"),
			colorize(dim, fmt.Sprintf("User: %s", contentStr)))
	} else if contentBlocks, ok := msg.Content.([]types.ContentBlock); ok {
		for _, block := range contentBlocks {
			if _, ok := block.(*types.ToolResultBlock); ok {
				fmt.Printf("  %s %s\n",
					colorize(brightGreen, "✓"),
					colorize(dim, "Tool execution complete"))
			}
		}
	}
}

func (r *StreamRenderer) renderResultMessage(msg *types.ResultMessage) {
	// End thinking if in progress
	if r.thinkingInProgress {
		fmt.Println()
		r.thinkingInProgress = false
	}

	fmt.Printf("\n%s %s\n",
		colorize(brightBlue, "▼"),
		colorize(brightBlue+bold, "Result"))

	// Cost
	if msg.TotalCostUSD != nil && *msg.TotalCostUSD > 0 {
		fmt.Printf("  %s Cost: %s\n",
			colorize(dim, "•"),
			colorize(brightYellow, fmt.Sprintf("$%.4f", *msg.TotalCostUSD)))
	}

	// Duration
	if msg.DurationMs > 0 {
		duration := time.Duration(msg.DurationMs) * time.Millisecond
		fmt.Printf("  %s Duration: %s\n",
			colorize(dim, "•"),
			colorize(dim, duration.Round(time.Millisecond).String()))
	}

	// Turns
	if msg.NumTurns > 0 {
		fmt.Printf("  %s Turns: %d\n",
			colorize(dim, "•"),
			msg.NumTurns)
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
