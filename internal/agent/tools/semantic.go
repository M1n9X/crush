package tools

import (
	"context"
	_ "embed"
	"fmt"
	"os"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/filepathext"
	"github.com/charmbracelet/crush/internal/fsext"
	"github.com/charmbracelet/crush/internal/history"
	"github.com/charmbracelet/crush/internal/lsp"
	"github.com/charmbracelet/crush/internal/lsp/util"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/semantic"
)

const (
	SemanticOverviewToolName           = "semantic_symbol_overview"
	SemanticFindSymbolToolName         = "semantic_find_symbol"
	SemanticFindRefsToolName           = "semantic_find_references"
	SemanticReplaceSymbolBodyToolName  = "semantic_replace_symbol_body"
	SemanticInsertBeforeSymbolToolName = "semantic_insert_before_symbol"
	SemanticInsertAfterSymbolToolName  = "semantic_insert_after_symbol"
	SemanticRenameSymbolToolName       = "semantic_rename_symbol"
)

type semanticToolDeps struct {
	retriever   *semantic.Retriever
	editor      *semantic.Editor
	permissions permission.Service
	history     history.Service
	lspClients  *csync.Map[string, *lsp.Client]
	workingDir  string
}

func newSemanticDeps(lspClients *csync.Map[string, *lsp.Client], permissions permission.Service, history history.Service, workingDir string) semanticToolDeps {
	r := semantic.NewRetriever(lspClients, workingDir)
	return semanticToolDeps{
		retriever:   r,
		editor:      semantic.NewEditor(r),
		permissions: permissions,
		history:     history,
		lspClients:  lspClients,
		workingDir:  workingDir,
	}
}

type SemanticOverviewParams struct {
	Path           string `json:"path" description:"Relative path to file for overview"`
	MaxAnswerChars int    `json:"max_answer_chars,omitempty" description:"Maximum size of the response in characters (default 150000)"`
}

func NewSemanticOverviewTool(lspClients *csync.Map[string, *lsp.Client], permissions permission.Service, history history.Service, workingDir string) fantasy.AgentTool {
	deps := newSemanticDeps(lspClients, permissions, history, workingDir)
	description := "Retrieve top-level symbols for a file using LSP's document symbol. " +
		"Use to quickly understand file structure; includes name_path, kind, and body_location."

	return fantasy.NewAgentTool(
		SemanticOverviewToolName,
		description,
		func(ctx context.Context, params SemanticOverviewParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if params.Path == "" {
				return fantasy.NewTextErrorResponse("path is required"), nil
			}
			if params.MaxAnswerChars <= 0 {
				params.MaxAnswerChars = semantic.DefaultMaxAnswerChars
			}
			res, err := deps.retriever.SymbolOverview(ctx, params.Path, params.MaxAnswerChars)
			if err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}
			return fantasy.NewTextResponse(res), nil
		},
	)
}

type SemanticFindSymbolParams struct {
	NamePathPattern string `json:"name_path_pattern" description:"Symbol name or name path pattern to search for"`
	Path            string `json:"path,omitempty" description:"File or directory to scope search"`
	Depth           int    `json:"depth,omitempty" description:"Include children up to this depth in results"`
	IncludeBody     bool   `json:"include_body,omitempty" description:"Attach symbol body to the response"`
	IncludeKinds    []int  `json:"include_kinds,omitempty" description:"LSP symbol kinds to include (ints 1-26)"`
	ExcludeKinds    []int  `json:"exclude_kinds,omitempty" description:"LSP symbol kinds to exclude (ints 1-26)"`
	Substring       bool   `json:"substring,omitempty" description:"Allow substring match on the last path segment"`
	MaxResults      int    `json:"max_results,omitempty" description:"Maximum number of symbols to return (0 means no limit)"`
	MaxAnswerChars  int    `json:"max_answer_chars,omitempty" description:"Maximum size of the response in characters (default 150000)"`
}

func NewSemanticFindSymbolTool(lspClients *csync.Map[string, *lsp.Client], permissions permission.Service, history history.Service, workingDir string) fantasy.AgentTool {
	deps := newSemanticDeps(lspClients, permissions, history, workingDir)
	description := "Find symbols by name path using LSP document symbols. Supports include/exclude kinds, substring matching, depth for children, and optional bodies."

	return fantasy.NewAgentTool(
		SemanticFindSymbolToolName,
		description,
		func(ctx context.Context, params SemanticFindSymbolParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if params.MaxAnswerChars <= 0 {
				params.MaxAnswerChars = semantic.DefaultMaxAnswerChars
			}
			result, err := deps.retriever.FindSymbols(ctx, params.NamePathPattern, params.Path, params.Depth, params.IncludeBody, params.IncludeKinds, params.ExcludeKinds, params.Substring, params.MaxResults, params.MaxAnswerChars)
			if err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}
			return fantasy.NewTextResponse(result), nil
		},
	)
}

type SemanticFindRefsParams struct {
	NamePath       string `json:"name_path" description:"Symbol name path to find references for"`
	Path           string `json:"path" description:"File containing the target symbol"`
	IncludeKinds   []int  `json:"include_kinds,omitempty" description:"LSP symbol kinds to include (ints 1-26)"`
	ExcludeKinds   []int  `json:"exclude_kinds,omitempty" description:"LSP symbol kinds to exclude (ints 1-26)"`
	IncludeImports bool   `json:"include_imports,omitempty" description:"Include import references (default false)"`
	IncludeSelf    bool   `json:"include_self,omitempty" description:"Include the declaration itself (default false)"`
	MaxResults     int    `json:"max_results,omitempty" description:"Maximum number of reference results (0 means no limit)"`
	MaxAnswerChars int    `json:"max_answer_chars,omitempty" description:"Maximum size of the response in characters (default 150000)"`
}

func NewSemanticFindReferencesTool(lspClients *csync.Map[string, *lsp.Client], permissions permission.Service, history history.Service, workingDir string) fantasy.AgentTool {
	deps := newSemanticDeps(lspClients, permissions, history, workingDir)
	description := "Find symbols referencing the target symbol (via LSP references) and return locations with context."

	return fantasy.NewAgentTool(
		SemanticFindRefsToolName,
		description,
		func(ctx context.Context, params SemanticFindRefsParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if params.NamePath == "" || params.Path == "" {
				return fantasy.NewTextErrorResponse("name_path and path are required"), nil
			}
			if params.MaxAnswerChars <= 0 {
				params.MaxAnswerChars = semantic.DefaultMaxAnswerChars
			}
			result, err := deps.retriever.FindReferences(ctx, params.NamePath, params.Path, params.IncludeKinds, params.ExcludeKinds, params.IncludeImports, params.IncludeSelf, params.MaxResults, params.MaxAnswerChars)
			if err != nil {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("failed to find references: %v", err)), nil
			}
			return fantasy.NewTextResponse(result), nil
		},
	)
}

type SemanticReplaceBodyParams struct {
	NamePath string `json:"name_path" description:"Symbol name path to replace"`
	Path     string `json:"path" description:"File containing the symbol"`
	Body     string `json:"body" description:"Replacement body text"`
}

func NewSemanticReplaceBodyTool(lspClients *csync.Map[string, *lsp.Client], permissions permission.Service, history history.Service, workingDir string) fantasy.AgentTool {
	deps := newSemanticDeps(lspClients, permissions, history, workingDir)
	description := "Replace the body of a symbol using its LSP range. Use after locating the symbol via semantic_find_symbol."

	return fantasy.NewAgentTool(
		SemanticReplaceSymbolBodyToolName,
		description,
		func(ctx context.Context, params SemanticReplaceBodyParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
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
				ToolName:    SemanticReplaceSymbolBodyToolName,
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

type SemanticInsertParams struct {
	NamePath string `json:"name_path" description:"Symbol name path as anchor"`
	Path     string `json:"path" description:"File containing the anchor symbol"`
	Body     string `json:"body" description:"Content to insert"`
}

func NewSemanticInsertBeforeTool(lspClients *csync.Map[string, *lsp.Client], permissions permission.Service, history history.Service, workingDir string) fantasy.AgentTool {
	deps := newSemanticDeps(lspClients, permissions, history, workingDir)
	description := "Insert content before a symbol definition using LSP ranges."

	return fantasy.NewAgentTool(
		SemanticInsertBeforeSymbolToolName,
		description,
		func(ctx context.Context, params SemanticInsertParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
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
				ToolName:    SemanticInsertBeforeSymbolToolName,
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

func NewSemanticInsertAfterTool(lspClients *csync.Map[string, *lsp.Client], permissions permission.Service, history history.Service, workingDir string) fantasy.AgentTool {
	deps := newSemanticDeps(lspClients, permissions, history, workingDir)
	description := "Insert content after a symbol definition using LSP ranges."

	return fantasy.NewAgentTool(
		SemanticInsertAfterSymbolToolName,
		description,
		func(ctx context.Context, params SemanticInsertParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
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
				ToolName:    SemanticInsertAfterSymbolToolName,
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

type SemanticRenameParams struct {
	NamePath string `json:"name_path" description:"Symbol name path to rename"`
	Path     string `json:"path" description:"File containing the symbol"`
	NewName  string `json:"new_name" description:"New symbol name"`
}

func NewSemanticRenameTool(lspClients *csync.Map[string, *lsp.Client], permissions permission.Service, history history.Service, workingDir string) fantasy.AgentTool {
	deps := newSemanticDeps(lspClients, permissions, history, workingDir)
	description := "Rename a symbol via LSP rename and apply workspace edits."

	return fantasy.NewAgentTool(
		SemanticRenameSymbolToolName,
		description,
		func(ctx context.Context, params SemanticRenameParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
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
					ToolName:    SemanticRenameSymbolToolName,
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
