package semantic

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/crush/internal/filepathext"
	"github.com/charmbracelet/crush/internal/lsp"
	"github.com/charmbracelet/x/powernap/pkg/lsp/protocol"
)

// Editor performs lightweight file edits based on LSP symbol ranges.
type Editor struct {
	retriever *Retriever
}

func NewEditor(r *Retriever) *Editor {
	return &Editor{retriever: r}
}

// ReplaceSymbolBody replaces the text covered by the symbol's range.
func (e *Editor) ReplaceSymbolBody(ctx context.Context, namePath, path, body string) error {
	match, absPath, _, err := e.findSymbol(ctx, namePath, path)
	if err != nil {
		return err
	}

	content, err := os.ReadFile(absPath)
	if err != nil {
		return err
	}

	body = strings.TrimSpace(body)

	updated := replaceRange(string(content), match.Range, body)
	return os.WriteFile(absPath, []byte(updated), 0o644)
}

// InsertBeforeSymbol inserts text before the symbol definition.
func (e *Editor) InsertBeforeSymbol(ctx context.Context, namePath, path, body string) error {
	match, absPath, _, err := e.findSymbol(ctx, namePath, path)
	if err != nil {
		return err
	}

	content, err := os.ReadFile(absPath)
	if err != nil {
		return err
	}
	body = normalizeInsertBeforeBody(body, protocol.SymbolKind(match.Kind))

	offset := offsetForPosition(string(content), match.Range.Start.Line, match.Range.Start.Character)
	updated := insertAt(string(content), offset, body)
	return os.WriteFile(absPath, []byte(updated), 0o644)
}

// InsertAfterSymbol inserts text after the symbol definition.
func (e *Editor) InsertAfterSymbol(ctx context.Context, namePath, path, body string) error {
	match, absPath, _, err := e.findSymbol(ctx, namePath, path)
	if err != nil {
		return err
	}

	content, err := os.ReadFile(absPath)
	if err != nil {
		return err
	}
	body = normalizeInsertAfterBody(body, protocol.SymbolKind(match.Kind))

	offset := offsetForPosition(string(content), match.Range.End.Line, match.Range.End.Character)
	updated := insertAt(string(content), offset, body)
	return os.WriteFile(absPath, []byte(updated), 0o644)
}

// RenameSymbol performs an LSP rename and returns the resulting workspace edit without applying it.
func (e *Editor) RenameSymbol(ctx context.Context, namePath, path, newName string) (protocol.WorkspaceEdit, error) {
	match, absPath, client, err := e.findSymbol(ctx, namePath, path)
	if err != nil {
		return protocol.WorkspaceEdit{}, err
	}
	line := int(match.SelectionRange.Start.Line) + 1
	character := int(match.SelectionRange.Start.Character) + 1
	edit, err := client.RenameSymbol(ctx, absPath, line, character, newName)
	if err != nil {
		return protocol.WorkspaceEdit{}, fmt.Errorf("%s", LSPMethodError("textDocument/rename", err))
	}
	return edit, nil
}

type symbolMatch struct {
	protocol.DocumentSymbol
	nameParts []string
}

func (e *Editor) findSymbol(ctx context.Context, namePath, relOrAbsPath string) (symbolMatch, string, *lsp.Client, error) {
	absPath := filepathext.SmartJoin(e.retriever.workingDir, relOrAbsPath)
	sym, parts, client, err := e.retriever.findDocumentSymbol(ctx, namePath, absPath)
	if err != nil {
		return symbolMatch{}, "", nil, err
	}
	return symbolMatch{DocumentSymbol: sym, nameParts: parts}, absPath, client, nil
}

func offsetForPosition(content string, line uint32, character uint32) int {
	lines := strings.Split(content, "\n")
	offset := 0
	for idx, ln := range lines {
		if uint32(idx) == line {
			col := utf16ColumnToByteOffset(ln, character)
			return offset + col
		}
		offset += len(ln)
		if idx < len(lines)-1 {
			offset++
		}
	}
	return len(content)
}

func replaceRange(content string, rng protocol.Range, newText string) string {
	start := offsetForPosition(content, rng.Start.Line, rng.Start.Character)
	end := offsetForPosition(content, rng.End.Line, rng.End.Character)
	if start < 0 {
		start = 0
	}
	if end < start {
		end = start
	}
	if start > len(content) {
		start = len(content)
	}
	if end > len(content) {
		end = len(content)
	}
	return content[:start] + newText + content[end:]
}

func insertAt(content string, offset int, text string) string {
	if offset < 0 {
		offset = 0
	}
	if offset > len(content) {
		offset = len(content)
	}
	return content[:offset] + text + content[offset:]
}

func utf16ColumnToByteOffset(line string, col uint32) int {
	if col == 0 {
		return 0
	}
	var codeUnits uint32
	for i, r := range line {
		if r <= 0xFFFF {
			codeUnits++
		} else {
			codeUnits += 2
		}
		if codeUnits > col {
			return i
		}
		if codeUnits == col {
			return i + len(string(r))
		}
	}
	return len(line)
}

func normalizeInsertAfterBody(body string, kind protocol.SymbolKind) string {
	if !strings.HasSuffix(body, "\n") {
		body += "\n"
	}
	leadingRequested := countLeadingNewlines(body)
	body = strings.TrimLeft(body, "\r\n")
	minLeading := 0
	if symbolRequiresSpacing(kind) {
		minLeading = 1
	}
	body = strings.TrimRight(body, "\r\n") + "\n"
	leading := leadingRequested
	if leading < minLeading {
		leading = minLeading
	}
	if leading > 0 {
		body = strings.Repeat("\n", leading) + body
	}
	return body
}

func normalizeInsertBeforeBody(body string, kind protocol.SymbolKind) string {
	trailingRequested := countTrailingNewlines(body)
	body = strings.TrimRight(body, "\r\n") + "\n"
	minTrailing := 0
	if symbolRequiresSpacing(kind) {
		minTrailing = 1
	}
	if trailingRequested > 0 {
		trailingRequested-- // one newline already appended above
	}
	trailing := trailingRequested
	if trailing < minTrailing {
		trailing = minTrailing
	}
	if trailing > 0 {
		body += strings.Repeat("\n", trailing)
	}
	return body
}

func symbolRequiresSpacing(kind protocol.SymbolKind) bool {
	switch kind {
	case protocol.Function, protocol.Method, protocol.Class, protocol.Struct, protocol.Interface, protocol.Enum, protocol.Constructor:
		return true
	default:
		return false
	}
}

func countLeadingNewlines(text string) int {
	count := 0
	for _, r := range text {
		if r == '\n' {
			count++
			continue
		}
		if r == '\r' {
			continue
		}
		break
	}
	return count
}

func countTrailingNewlines(text string) int {
	count := 0
	for i := len(text) - 1; i >= 0; i-- {
		if text[i] == '\n' {
			count++
			continue
		}
		if text[i] == '\r' {
			continue
		}
		break
	}
	return count
}
