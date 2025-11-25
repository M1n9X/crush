package tools

import (
	"context"
	_ "embed"
	"fmt"
	"log/slog"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/history"
	"github.com/charmbracelet/crush/internal/memory"
)

//go:embed memory_write.md
var memoryWriteDescription []byte

type MemoryWriteParams struct {
	FilePath string `json:"file_path" description:"Relative path to the memory file (default: memory.md)"`
	Content  string `json:"content" description:"Content to write to the memory file"`
}

const MemoryWriteToolName = "memory_write"

func NewMemoryWriteTool(agentID string, mem *memory.Service, _ any, files history.Service) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		MemoryWriteToolName,
		string(memoryWriteDescription),
		func(ctx context.Context, params MemoryWriteParams, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if mem == nil {
				return fantasy.NewTextErrorResponse("memory service unavailable"), nil
			}
			if params.Content == "" {
				return fantasy.NewTextErrorResponse("content is required"), nil
			}

			entry, err := mem.Write(agentID, params.FilePath, params.Content)
			if err != nil {
				return fantasy.ToolResponse{}, err
			}

			if files != nil {
				if sessionID := GetSessionFromContext(ctx); sessionID != "" {
					if _, createErr := files.CreateVersion(ctx, sessionID, entry.FullPath, entry.Content); createErr != nil {
						// Not fatal: best-effort history tracking
						slog.Debug("failed to record memory history", "error", createErr, "path", entry.FullPath)
					}
				}
			}

			result := fmt.Sprintf("Saved memory to %s", entry.Path)
			return fantasy.NewTextResponse(result), nil
		},
	)
}
