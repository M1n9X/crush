package tools

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"os"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/memory"
)

//go:embed memory_read.md
var memoryReadDescription []byte

type MemoryReadParams struct {
	FilePath string `json:"file_path" description:"Relative path to the memory file (default: memory.md)"`
}

const MemoryReadToolName = "memory_read"

func NewMemoryReadTool(agentID string, mem *memory.Service, _ any) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		MemoryReadToolName,
		string(memoryReadDescription),
		func(ctx context.Context, params MemoryReadParams, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if mem == nil {
				return fantasy.NewTextErrorResponse("memory service unavailable"), nil
			}

			entry, err := mem.Read(agentID, params.FilePath)
			if errors.Is(err, os.ErrNotExist) {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("Memory file not found: %s", params.FilePath)), nil
			}
			if err != nil {
				return fantasy.ToolResponse{}, err
			}

			body := fmt.Sprintf("<memory>\n<file path=\"%s\">\n%s\n</file>\n</memory>", entry.Path, entry.Content)
			if entry.Truncated {
				body += "\n\n(Note: memory file truncated for safety)"
			}
			return fantasy.NewTextResponse(body), nil
		},
	)
}
