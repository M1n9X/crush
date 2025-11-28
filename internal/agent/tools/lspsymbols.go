package tools

import (
	"context"
	_ "embed"
	"fmt"
	"os"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/tools/lspsymbols"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/filepathext"
	"github.com/charmbracelet/crush/internal/fsext"
	"github.com/charmbracelet/crush/internal/history"
	"github.com/charmbracelet/crush/internal/lsp"
	"github.com/charmbracelet/crush/internal/lsp/util"
	"github.com/charmbracelet/crush/internal/permission"
)

const (
	LSPSymbolOverviewToolName           = "lspoverview"
	LSPSymbolFindSymbolToolName         = "lspfind"
	LSPSymbolFindRefsToolName           = "lsprefs"
	LSPSymbolReplaceSymbolBodyToolName  = "lspreplace"
	LSPSymbolInsertBeforeSymbolToolName = "lspinsertbefore"
	LSPSymbolInsertAfterSymbolToolName  = "lspinsertafter"
	LSPSymbolRenameSymbolToolName       = "lsprename"
)

//go:embed lspsymbols/lspoverview.md
var lspOverviewDescription []byte

//go:embed lspsymbols/lspfind.md
var lspFindDescription []byte

//go:embed lspsymbols/lsprefs.md
var lspRefsDescription []byte

//go:embed lspsymbols/lspreplace.md
var lspReplaceDescription []byte

//go:embed lspsymbols/lspinsert.md
var lspInsertDescription []byte

//go:embed lspsymbols/lsprename.md
var lspRenameDescription []byte

type lspSymbolToolDeps struct {
	retriever   *lspsymbols.Retriever
	editor      *lspsymbols.Editor
	permissions permission.Service
	history     history.Service
	lspClients  *csync.Map[string, *lsp.Client]
	workingDir  string
}

func newLSPSymbolDeps(lspClients *csync.Map[string, *lsp.Client], permissions permission.Service, history history.Service, workingDir string) lspSymbolToolDeps {
	r := lspsymbols.NewRetriever(lspClients, workingDir)
	return lspSymbolToolDeps{
		retriever:   r,
		editor:      lspsymbols.NewEditor(r),
		permissions: permissions,
		history:     history,
		lspClients:  lspClients,
		workingDir:  workingDir,
	}
}

type LSPSymbolOverviewParams struct {
	Path           string `json:"path" description:"Relative path to file for overview"`
	MaxAnswerChars int    `json:"max_answer_chars,omitempty" description:"Maximum size of the response in characters (default 150000)"`
}

func NewLSPSymbolOverviewTool(lspClients *csync.Map[string, *lsp.Client], permissions permission.Service, history history.Service, workingDir string) fantasy.AgentTool {
	deps := newLSPSymbolDeps(lspClients, permissions, history, workingDir)
	return fantasy.NewAgentTool(
		LSPSymbolOverviewToolName,
		string(lspOverviewDescription),
		func(ctx context.Context, params LSPSymbolOverviewParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if params.Path == "" {
				return fantasy.NewTextErrorResponse("path is required"), nil
			}
			if params.MaxAnswerChars <= 0 {
				params.MaxAnswerChars = lspsymbols.DefaultMaxAnswerChars
			}
			res, err := deps.retriever.SymbolOverview(ctx, params.Path, params.MaxAnswerChars)
			if err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}
			return fantasy.NewTextResponse(res), nil
		},
	)
}

type LSPSymbolFindSymbolParams struct {
	NamePathPattern string `json:"name_path_pattern" description:"Symbol name or name path pattern to search for"`
	Path            string `json:"path,omitempty" description:"File or directory to scope search"`
	Depth           int    `json:"depth,omitempty" description:"Include children up to this depth in results"`
	IncludeBody     bool   `json:"include_body,omitempty" description:"Attach symbol body to the response"`
	IncludeKinds    []int  `json:"include_kinds,omitempty" description:"LSP symbol kinds to include (ints 1-26)"`
	ExcludeKinds    []int  `json:"exclude_kinds,omitempty" description:"LSP symbol kinds to exclude (ints 1-26)"`
	Substring       bool   `json:"substring,omitempty" description:"Allow substring match on the last path segment (disables regex)"`
	IsRegex         bool   `json:"is_regex,omitempty" description:"Treat name_path_pattern as regex (RE2 syntax). Default: true unless substring=true"`
	MaxResults      int    `json:"max_results,omitempty" description:"Maximum number of symbols to return (0 means no limit)"`
	MaxAnswerChars  int    `json:"max_answer_chars,omitempty" description:"Maximum size of the response in characters (default 150000)"`
}

func NewLSPSymbolFindSymbolTool(lspClients *csync.Map[string, *lsp.Client], permissions permission.Service, history history.Service, workingDir string) fantasy.AgentTool {
	deps := newLSPSymbolDeps(lspClients, permissions, history, workingDir)
	return fantasy.NewAgentTool(
		LSPSymbolFindSymbolToolName,
		string(lspFindDescription),
		func(ctx context.Context, params LSPSymbolFindSymbolParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			// Default is_regex to true if not explicitly set
			isRegex := true
			if params.Substring {
				// substring mode explicitly disables regex
				isRegex = false
			} else if params.IsRegex {
				isRegex = true
			}

			if params.MaxAnswerChars <= 0 {
				params.MaxAnswerChars = lspsymbols.DefaultMaxAnswerChars
			}
			result, err := deps.retriever.FindSymbols(ctx, params.NamePathPattern, params.Path, params.Depth, params.IncludeBody, params.IncludeKinds, params.ExcludeKinds, params.Substring, isRegex, params.MaxResults, params.MaxAnswerChars)
			if err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}
			return fantasy.NewTextResponse(result), nil
		},
	)
}

type LSPSymbolFindRefsParams struct {
	NamePath       string `json:"name_path" description:"Symbol name path to find references for"`
	Path           string `json:"path" description:"File containing the target symbol"`
	IncludeKinds   []int  `json:"include_kinds,omitempty" description:"LSP symbol kinds to include (ints 1-26)"`
	ExcludeKinds   []int  `json:"exclude_kinds,omitempty" description:"LSP symbol kinds to exclude (ints 1-26)"`
	IncludeImports bool   `json:"include_imports,omitempty" description:"Include import references (default false)"`
	IncludeSelf    bool   `json:"include_self,omitempty" description:"Include the declaration itself (default false)"`
	MaxResults     int    `json:"max_results,omitempty" description:"Maximum number of reference results (0 means no limit)"`
	MaxAnswerChars int    `json:"max_answer_chars,omitempty" description:"Maximum size of the response in characters (default 150000)"`
}

func NewLSPSymbolFindReferencesTool(lspClients *csync.Map[string, *lsp.Client], permissions permission.Service, history history.Service, workingDir string) fantasy.AgentTool {
	deps := newLSPSymbolDeps(lspClients, permissions, history, workingDir)
	return fantasy.NewAgentTool(
		LSPSymbolFindRefsToolName,
		string(lspRefsDescription),
		func(ctx context.Context, params LSPSymbolFindRefsParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if params.NamePath == "" || params.Path == "" {
				return fantasy.NewTextErrorResponse("name_path and path are required"), nil
			}
			if params.MaxAnswerChars <= 0 {
				params.MaxAnswerChars = lspsymbols.DefaultMaxAnswerChars
			}
			result, err := deps.retriever.FindReferences(ctx, params.NamePath, params.Path, params.IncludeKinds, params.ExcludeKinds, params.IncludeImports, params.IncludeSelf, params.MaxResults, params.MaxAnswerChars)
			if err != nil {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("failed to find references: %v", err)), nil
			}
			return fantasy.NewTextResponse(result), nil
		},
	)
}

type LSPSymbolReplaceBodyParams struct {
	NamePath string `json:"name_path" description:"Symbol name path to replace"`
	Path     string `json:"path" description:"File containing the symbol"`
	Body     string `json:"body" description:"Replacement body text"`
}

func NewLSPSymbolReplaceBodyTool(lspClients *csync.Map[string, *lsp.Client], permissions permission.Service, history history.Service, workingDir string) fantasy.AgentTool {
	deps := newLSPSymbolDeps(lspClients, permissions, history, workingDir)
	return fantasy.NewAgentTool(
		LSPSymbolReplaceSymbolBodyToolName,
		string(lspReplaceDescription),
		func(ctx context.Context, params LSPSymbolReplaceBodyParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if params.NamePath == "" || params.Path == "" {
				return fantasy.NewTextErrorResponse("name_path and path are required"), nil
			}
			filePath := filepathext.SmartJoin(deps.workingDir, params.Path)
			sessionID := GetSessionFromContext(ctx)
			if sessionID == "" {
				return fantasy.ToolResponse{}, fmt.Errorf("session ID is required for editing")
			}
			oldContent, err := os.ReadFile(filePath)
			if err != nil {
				return fantasy.ToolResponse{}, fmt.Errorf("failed to read file: %w", err)
			}

			if !deps.permissions.Request(permission.CreatePermissionRequest{
				SessionID:   sessionID,
				Path:        fsext.PathOrPrefix(filePath, deps.workingDir),
				ToolCallID:  call.ID,
				ToolName:    LSPSymbolReplaceSymbolBodyToolName,
				Action:      "write",
				Description: fmt.Sprintf("Replace symbol body in %s", filePath),
				Params: map[string]any{
					"file_path": filePath,
				},
			}) {
				return fantasy.ToolResponse{}, permission.ErrorPermissionDenied
			}

			if err := deps.editor.ReplaceSymbolBody(ctx, params.NamePath, params.Path, params.Body); err != nil {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("replace failed: %v", err)), nil
			}

			newContent, err := os.ReadFile(filePath)
			if err == nil {
				upsertHistory(ctx, deps.history, sessionID, filePath, string(oldContent), string(newContent))
			}
			recordFileWrite(filePath)
			recordFileRead(filePath)
			notifyLSPs(ctx, deps.lspClients, filePath)

			return fantasy.NewTextResponse("OK"), nil
		},
	)
}

type LSPSymbolInsertParams struct {
	NamePath string `json:"name_path" description:"Symbol name path as anchor"`
	Path     string `json:"path" description:"File containing the anchor symbol"`
	Body     string `json:"body" description:"Content to insert"`
}

func NewLSPSymbolInsertBeforeTool(lspClients *csync.Map[string, *lsp.Client], permissions permission.Service, history history.Service, workingDir string) fantasy.AgentTool {
	deps := newLSPSymbolDeps(lspClients, permissions, history, workingDir)
	return fantasy.NewAgentTool(
		LSPSymbolInsertBeforeSymbolToolName,
		string(lspInsertDescription),
		func(ctx context.Context, params LSPSymbolInsertParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if params.NamePath == "" || params.Path == "" {
				return fantasy.NewTextErrorResponse("name_path and path are required"), nil
			}
			filePath := filepathext.SmartJoin(deps.workingDir, params.Path)
			sessionID := GetSessionFromContext(ctx)
			if sessionID == "" {
				return fantasy.NewTextErrorResponse("session ID is required for editing"), nil
			}
			oldContent, err := os.ReadFile(filePath)
			if err != nil {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("failed to read file: %v", err)), nil
			}

			if !deps.permissions.Request(permission.CreatePermissionRequest{
				SessionID:   sessionID,
				Path:        fsext.PathOrPrefix(filePath, deps.workingDir),
				ToolCallID:  call.ID,
				ToolName:    LSPSymbolInsertBeforeSymbolToolName,
				Action:      "write",
				Description: fmt.Sprintf("Insert before symbol in %s", filePath),
				Params:      map[string]any{"file_path": filePath},
			}) {
				return fantasy.ToolResponse{}, permission.ErrorPermissionDenied
			}

			if err := deps.editor.InsertBeforeSymbol(ctx, params.NamePath, params.Path, params.Body); err != nil {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("insert_before failed: %v", err)), nil
			}

			newContent, err := os.ReadFile(filePath)
			if err == nil {
				upsertHistory(ctx, deps.history, sessionID, filePath, string(oldContent), string(newContent))
			}
			recordFileWrite(filePath)
			recordFileRead(filePath)
			notifyLSPs(ctx, deps.lspClients, filePath)

			return fantasy.NewTextResponse("OK"), nil
		},
	)
}

func NewLSPSymbolInsertAfterTool(lspClients *csync.Map[string, *lsp.Client], permissions permission.Service, history history.Service, workingDir string) fantasy.AgentTool {
	deps := newLSPSymbolDeps(lspClients, permissions, history, workingDir)
	return fantasy.NewAgentTool(
		LSPSymbolInsertAfterSymbolToolName,
		string(lspInsertDescription),
		func(ctx context.Context, params LSPSymbolInsertParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if params.NamePath == "" || params.Path == "" {
				return fantasy.NewTextErrorResponse("name_path and path are required"), nil
			}
			filePath := filepathext.SmartJoin(deps.workingDir, params.Path)
			sessionID := GetSessionFromContext(ctx)
			if sessionID == "" {
				return fantasy.NewTextErrorResponse("session ID is required for editing"), nil
			}
			oldContent, err := os.ReadFile(filePath)
			if err != nil {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("failed to read file: %v", err)), nil
			}

			if !deps.permissions.Request(permission.CreatePermissionRequest{
				SessionID:   sessionID,
				Path:        fsext.PathOrPrefix(filePath, deps.workingDir),
				ToolCallID:  call.ID,
				ToolName:    LSPSymbolInsertAfterSymbolToolName,
				Action:      "write",
				Description: fmt.Sprintf("Insert after symbol in %s", filePath),
				Params:      map[string]any{"file_path": filePath},
			}) {
				return fantasy.ToolResponse{}, permission.ErrorPermissionDenied
			}

			if err := deps.editor.InsertAfterSymbol(ctx, params.NamePath, params.Path, params.Body); err != nil {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("insert_after failed: %v", err)), nil
			}

			newContent, err := os.ReadFile(filePath)
			if err == nil {
				upsertHistory(ctx, deps.history, sessionID, filePath, string(oldContent), string(newContent))
			}
			recordFileWrite(filePath)
			recordFileRead(filePath)
			notifyLSPs(ctx, deps.lspClients, filePath)

			return fantasy.NewTextResponse("OK"), nil
		},
	)
}

type LSPSymbolRenameParams struct {
	NamePath string `json:"name_path" description:"Symbol name path to rename"`
	Path     string `json:"path" description:"File containing the symbol"`
	NewName  string `json:"new_name" description:"New symbol name"`
}

func NewLSPSymbolRenameTool(lspClients *csync.Map[string, *lsp.Client], permissions permission.Service, history history.Service, workingDir string) fantasy.AgentTool {
	deps := newLSPSymbolDeps(lspClients, permissions, history, workingDir)
	return fantasy.NewAgentTool(
		LSPSymbolRenameSymbolToolName,
		string(lspRenameDescription),
		func(ctx context.Context, params LSPSymbolRenameParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if params.NamePath == "" || params.Path == "" || params.NewName == "" {
				return fantasy.NewTextErrorResponse("name_path, path, and new_name are required"), nil
			}
			filePath := filepathext.SmartJoin(deps.workingDir, params.Path)
			sessionID := GetSessionFromContext(ctx)
			if sessionID == "" {
				return fantasy.NewTextErrorResponse("session ID is required for editing"), nil
			}
			edit, err := deps.editor.RenameSymbol(ctx, params.NamePath, params.Path, params.NewName)
			if err != nil {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("rename failed: %v", err)), nil
			}

			affectedPaths := util.WorkspaceEditPaths(edit)
			if len(affectedPaths) == 0 {
				return fantasy.NewTextErrorResponse("rename produced no workspace edits"), nil
			}

			oldContents := make(map[string]string, len(affectedPaths))
			for _, path := range affectedPaths {
				if content, err := os.ReadFile(path); err == nil {
					oldContents[path] = string(content)
				}
			}

			for _, path := range affectedPaths {
				if !deps.permissions.Request(permission.CreatePermissionRequest{
					SessionID:   sessionID,
					Path:        fsext.PathOrPrefix(path, deps.workingDir),
					ToolCallID:  call.ID,
					ToolName:    LSPSymbolRenameSymbolToolName,
					Action:      "write",
					Description: fmt.Sprintf("Rename symbol in %s (edit %s)", filePath, path),
					Params: map[string]any{
						"file_path": path,
						"new_name":  params.NewName,
					},
				}) {
					return fantasy.ToolResponse{}, permission.ErrorPermissionDenied
				}
			}

			if err := util.ApplyWorkspaceEdit(edit); err != nil {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("rename failed: %v", err)), nil
			}

			for _, path := range affectedPaths {
				if newContent, err := os.ReadFile(path); err == nil {
					upsertHistory(ctx, deps.history, sessionID, path, oldContents[path], string(newContent))
				}
				recordFileWrite(path)
				recordFileRead(path)
				notifyLSPs(ctx, deps.lspClients, path)
			}

			return fantasy.NewTextResponse("OK"), nil
		},
	)
}

func upsertHistory(ctx context.Context, files history.Service, sessionID, filePath, oldContent, newContent string) {
	if files == nil || sessionID == "" {
		return
	}

	if _, err := files.GetByPathAndSession(ctx, filePath, sessionID); err != nil {
		if _, err := files.Create(ctx, sessionID, filePath, oldContent); err != nil {
			return
		}
		if _, err := files.CreateVersion(ctx, sessionID, filePath, newContent); err != nil {
			return
		}
		return
	}

	if oldContent != "" && oldContent != newContent {
		if _, err := files.CreateVersion(ctx, sessionID, filePath, oldContent); err != nil {
			return
		}
	}
	_, _ = files.CreateVersion(ctx, sessionID, filePath, newContent)
}
