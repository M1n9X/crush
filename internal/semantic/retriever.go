package semantic

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/filepathext"
	"github.com/charmbracelet/crush/internal/fsext"
	"github.com/charmbracelet/crush/internal/lsp"
	"github.com/charmbracelet/x/powernap/pkg/lsp/protocol"
)

const (
	DefaultMaxAnswerChars = 150_000
	// zero means no artificial limit; ignore rules still apply
	maxFilesPerDirectory     = 0
	defaultContextLines      = 1
	workspaceSymbolPathLimit = 2000
	symbolKindFileShortName  = "File"
)

type Symbol struct {
	NamePath     string          `json:"name_path"`
	Kind         string          `json:"kind"`
	RelativePath string          `json:"relative_path,omitempty"`
	BodyLocation *SymbolLocation `json:"body_location,omitempty"`
	Body         string          `json:"body,omitempty"`
	Children     []Symbol        `json:"children,omitempty"`
}

type SymbolLocation struct {
	StartLine int `json:"start_line"`
	EndLine   int `json:"end_line"`
}

type Retriever struct {
	lspClients     *csync.Map[string, *lsp.Client]
	workingDir     string
	maxAnswerChars int

	cacheMu     sync.Mutex
	symbolCache map[string]symbolCacheEntry
}

type symbolCacheEntry struct {
	modTime time.Time
	size    int64
	symbols []protocol.DocumentSymbol
}

type findOptions struct {
	depth         int
	includeBody   bool
	includeKinds  map[protocol.SymbolKind]struct{}
	excludeKinds  map[protocol.SymbolKind]struct{}
	substring     bool
	maxAnswerChar int
}

func NewRetriever(lspClients *csync.Map[string, *lsp.Client], workingDir string) *Retriever {
	return &Retriever{
		lspClients:     lspClients,
		workingDir:     workingDir,
		maxAnswerChars: DefaultMaxAnswerChars,
		symbolCache:    map[string]symbolCacheEntry{},
	}
}

func (r *Retriever) SymbolOverview(ctx context.Context, path string, maxAnswerChars int) (string, error) {
	targets, err := r.resolvePaths(ctx, path)
	if err != nil {
		return "", err
	}

	results := make([]Symbol, 0, len(targets))
	for _, file := range targets {
		client := r.clientForPath(file)
		if client == nil {
			continue
		}

		docSymbols, err := r.documentSymbols(ctx, client, file)
		if err != nil {
			return "", err
		}

		for _, ds := range docSymbols {
			rel := r.relPath(file)
			symbol := r.buildSymbol(ds, []string{ds.Name}, rel, false, 0)
			// overview only needs top-level; children not included
			symbol.Children = nil
			results = append(results, symbol)
		}
	}

	return r.marshalWithLimit(results, maxAnswerChars)
}

func (r *Retriever) FindSymbols(ctx context.Context, namePathPattern string, path string, depth int, includeBody bool, includeKinds, excludeKinds []int, substring bool, maxResults int, maxAnswerChars int) (string, error) {
	if namePathPattern == "" {
		return "", fmt.Errorf("name_path_pattern is required")
	}

	targets, err := r.resolvePaths(ctx, path)
	if err != nil {
		return "", err
	}

	matcher := newNamePathMatcher(namePathPattern, substring)
	opts := r.convertOptions(depth, includeBody, includeKinds, excludeKinds, substring, maxAnswerChars)

	var symbols []Symbol
	truncated := false
	for _, file := range targets {
		client := r.clientForPath(file)
		if client == nil {
			continue
		}

		docSymbols, err := r.documentSymbols(ctx, client, file)
		if err != nil {
			return "", err
		}

		fileMatches := r.findInDocument(ctx, docSymbols, matcher, opts, r.relPath(file), maxResults-len(symbols))
		symbols = append(symbols, fileMatches...)
		if maxResults > 0 && len(symbols) >= maxResults {
			truncated = true
			break
		}
	}

	resp := map[string]any{
		"results":   symbols,
		"truncated": truncated,
	}
	return r.marshalWithLimit(resp, maxAnswerChars)
}

func (r *Retriever) FindReferences(ctx context.Context, namePath string, path string, includeKinds, excludeKinds []int, includeImports, includeSelf bool, maxResults int, maxAnswerChars int) (string, error) {
	if namePath == "" {
		return "", fmt.Errorf("name_path is required")
	}

	absPath := filepathext.SmartJoin(r.workingDir, path)
	symbol, _, client, err := r.findDocumentSymbol(ctx, namePath, absPath)
	if err != nil {
		return r.marshalWithLimit([]Symbol{}, maxAnswerChars)
	}

	refLocations, err := r.referencesForSymbol(ctx, client, absPath, symbol, includeSelf)
	if err != nil {
		return "", err
	}

	includeSet := toKindSet(includeKinds)
	excludeSet := toKindSet(excludeKinds)

	results := make([]map[string]any, 0, len(refLocations))
	for _, ref := range refLocations {
		refPath, err := ref.URI.Path()
		if err != nil {
			continue
		}

		refSymbol, hasSymbol := r.containingSymbolAt(ctx, refPath, ref.Range.Start)
		if !hasSymbol {
			// fabricate a file-level symbol if none found
			start, end := fileLineSpan(refPath)
			refSymbol = Symbol{
				NamePath:     filepath.Base(refPath),
				Kind:         symbolKindFileShortName,
				RelativePath: r.relPath(refPath),
				BodyLocation: &SymbolLocation{StartLine: start, EndLine: end},
			}
		}

		refKind, hasKind := symbolKindFromName(refSymbol.Kind)

		if len(excludeSet) > 0 {
			if hasKind {
				if _, ok := excludeSet[refKind]; ok {
					continue
				}
			}
		}
		if len(includeSet) > 0 {
			if !hasKind {
				continue
			}
			if _, ok := includeSet[refKind]; !ok {
				continue
			}
		}

		snippet := r.contentAroundLine(refPath, int(ref.Range.Start.Line), defaultContextLines)
		if !includeImports && looksLikeImport(snippet, refPath) {
			continue
		}

		results = append(results, map[string]any{
			"name_path":                refSymbol.NamePath,
			"kind":                     refSymbol.Kind,
			"relative_path":            refSymbol.RelativePath,
			"body_location":            refSymbol.BodyLocation,
			"content_around_reference": snippet,
		})
		if maxResults > 0 && len(results) >= maxResults {
			break
		}
	}

	// deterministic order to aid callers
	slices.SortFunc(results, func(a, b map[string]any) int {
		pathA, _ := a["relative_path"].(string)
		pathB, _ := b["relative_path"].(string)
		if pathA == pathB {
			lineA := sortLine(a["body_location"])
			lineB := sortLine(b["body_location"])
			return cmp.Compare(lineA, lineB)
		}
		return cmp.Compare(pathA, pathB)
	})

	resp := map[string]any{
		"results": results,
		"truncated": func() bool {
			return maxResults > 0 && len(results) >= maxResults
		}(),
	}

	return r.marshalWithLimit(resp, maxAnswerChars)
}

func (r *Retriever) referencesForSymbol(ctx context.Context, client *lsp.Client, absPath string, target protocol.DocumentSymbol, includeSelf bool) ([]protocol.Location, error) {
	line := int(target.SelectionRange.Start.Line)
	character := int(target.SelectionRange.Start.Character)
	refs, err := client.FindReferences(ctx, absPath, line+1, character+1, false)
	if err != nil {
		return nil, fmt.Errorf("%s", LSPMethodError("textDocument/references", err))
	}
	if includeSelf {
		return refs, nil
	}
	filtered := make([]protocol.Location, 0, len(refs))
	for _, ref := range refs {
		if ref.Range.Start.Line == target.SelectionRange.Start.Line &&
			ref.Range.Start.Character == target.SelectionRange.Start.Character &&
			ref.Range.End.Line == target.SelectionRange.End.Line &&
			ref.Range.End.Character == target.SelectionRange.End.Character {
			continue
		}
		filtered = append(filtered, ref)
	}
	return filtered, nil
}

func (r *Retriever) findDocumentSymbol(ctx context.Context, namePath string, absPath string) (protocol.DocumentSymbol, []string, *lsp.Client, error) {
	_, client, err := r.ensureFileAndClient(absPath)
	if err != nil {
		return protocol.DocumentSymbol{}, nil, nil, err
	}
	docSymbols, err := r.documentSymbols(ctx, client, absPath)
	if err != nil {
		return protocol.DocumentSymbol{}, nil, nil, err
	}

	matcher := newNamePathMatcher(namePath, false)
	var target protocol.DocumentSymbol
	var targetNameParts []string
	var found bool

	var walk func(symbol protocol.DocumentSymbol, ancestors []string)
	walk = func(symbol protocol.DocumentSymbol, ancestors []string) {
		if found {
			return
		}
		baseName, overload := splitSymbolName(symbol.Name)
		nameParts := append(ancestors, baseName)
		signature := r.signatureForSymbol(ctx, client, absPath, symbol)
		if matcher.matches(nameParts, overload, signature) {
			target = symbol
			targetNameParts = append([]string(nil), append(ancestors, symbol.Name)...)
			found = true
			return
		}
		for _, child := range symbol.Children {
			walk(child, append(ancestors, symbol.Name))
		}
	}

	for _, ds := range docSymbols {
		walk(ds, []string{})
		if found {
			break
		}
	}

	if !found {
		return protocol.DocumentSymbol{}, nil, client, fmt.Errorf("symbol %s not found in %s", namePath, absPath)
	}

	return target, targetNameParts, client, nil
}

func (r *Retriever) documentSymbols(ctx context.Context, client *lsp.Client, absPath string) ([]protocol.DocumentSymbol, error) {
	info, err := os.Stat(absPath)
	if err != nil {
		return nil, fmt.Errorf("path does not exist: %w", err)
	}

	r.cacheMu.Lock()
	if entry, ok := r.symbolCache[absPath]; ok && entry.modTime.Equal(info.ModTime()) && entry.size == info.Size() {
		defer r.cacheMu.Unlock()
		return entry.symbols, nil
	}
	r.cacheMu.Unlock()

	docSymbols, err := client.DocumentSymbols(ctx, absPath)
	if err != nil {
		return nil, fmt.Errorf("%s", LSPMethodError("textDocument/documentSymbol", err))
	}

	r.cacheMu.Lock()
	r.symbolCache[absPath] = symbolCacheEntry{
		modTime: info.ModTime(),
		size:    info.Size(),
		symbols: docSymbols,
	}
	r.cacheMu.Unlock()

	return docSymbols, nil
}

func (r *Retriever) findInDocument(ctx context.Context, docSymbols []protocol.DocumentSymbol, matcher namePathMatcher, opts findOptions, relPath string, remaining int) []Symbol {
	var results []Symbol
	sigCache := make(map[string]string)
	var walk func(symbol protocol.DocumentSymbol, ancestors []string)

	walk = func(symbol protocol.DocumentSymbol, ancestors []string) {
		baseName, overload := splitSymbolName(symbol.Name)
		nameParts := append(ancestors, baseName)
		sKind := protocol.SymbolKind(symbol.Kind)

		if opts.excludeKinds != nil {
			if _, ok := opts.excludeKinds[sKind]; ok {
				for _, child := range symbol.Children {
					walk(child, nameParts)
				}
				return
			}
		}

		if opts.includeKinds != nil {
			if _, ok := opts.includeKinds[sKind]; !ok {
				for _, child := range symbol.Children {
					walk(child, nameParts)
				}
				return
			}
		}

		signature := symbol.Detail
		if matcher.withSignature {
			key := fmt.Sprintf("%s:%d:%d", relPath, symbol.SelectionRange.Start.Line, symbol.SelectionRange.Start.Character)
			if cached, ok := sigCache[key]; ok {
				signature = cached
			} else {
				if sig := r.signatureForSymbolCached(ctx, symbol, relPath, sigCache); sig != "" {
					signature = sig
				}
			}
		}
		if matcher.matches(nameParts, overload, signature) {
			results = append(results, r.buildSymbol(symbol, append(ancestors, symbol.Name), relPath, opts.includeBody, opts.depth))
			if remaining > 0 && len(results) >= remaining {
				return
			}
		}

		for _, child := range symbol.Children {
			walk(child, append(ancestors, symbol.Name))
			if remaining > 0 && len(results) >= remaining {
				return
			}
		}
	}

	for _, ds := range docSymbols {
		walk(ds, []string{})
		if remaining > 0 && len(results) >= remaining {
			break
		}
	}

	return results
}

func (r *Retriever) buildSymbol(symbol protocol.DocumentSymbol, nameParts []string, relPath string, includeBody bool, depth int) Symbol {
	namePath := strings.Join(nameParts, "/")
	bodyLoc := r.toSymbolLocation(symbol.Range)

	kindName := symbolKindNames[protocol.SymbolKind(symbol.Kind)]
	if kindName == "" {
		kindName = symbolKindFileShortName
	}

	result := Symbol{
		NamePath:     namePath,
		Kind:         kindName,
		RelativePath: relPath,
		BodyLocation: &bodyLoc,
	}

	if includeBody {
		if body, err := r.readRange(relPath, symbol.Range); err == nil {
			result.Body = body
		}
	}

	if depth > 0 && len(symbol.Children) > 0 {
		for _, child := range symbol.Children {
			childParts := append(nameParts, child.Name)
			result.Children = append(result.Children, r.buildSymbol(child, childParts, relPath, includeBody, depth-1))
		}
	}

	return result
}

func (r *Retriever) toSymbolLocation(rng protocol.Range) SymbolLocation {
	return SymbolLocation{
		StartLine: int(rng.Start.Line) + 1,
		EndLine:   int(rng.End.Line) + 1,
	}
}

func (r *Retriever) readRange(relPath string, rng protocol.Range) (string, error) {
	absPath := filepathext.SmartJoin(r.workingDir, relPath)
	data, err := os.ReadFile(absPath)
	if err != nil {
		return "", err
	}

	content := string(data)
	start := offsetForPosition(content, rng.Start.Line, rng.Start.Character)
	end := offsetForPosition(content, rng.End.Line, rng.End.Character)
	if start < 0 {
		start = 0
	}
	if end < start {
		end = start
	}
	if end > len(content) {
		end = len(content)
	}
	return content[start:end], nil
}

func (r *Retriever) contentAroundLine(absPath string, line int, contextLines int) string {
	data, err := os.ReadFile(absPath)
	if err != nil {
		return ""
	}
	lines := strings.Split(string(data), "\n")
	start := line - contextLines
	if start < 0 {
		start = 0
	}
	end := line + contextLines
	if end >= len(lines) {
		end = len(lines) - 1
	}
	return strings.Join(lines[start:end+1], "\n")
}

func (r *Retriever) marshalWithLimit(v any, maxAnswerChars int) (string, error) {
	limit := maxAnswerChars
	if limit <= 0 {
		limit = r.maxAnswerChars
	}
	data, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	if len(data) > limit {
		return fmt.Sprintf("The answer is too long (%d characters). Please narrow the query or raise max_answer_chars.", len(data)), nil
	}
	return string(data), nil
}

func (r *Retriever) convertOptions(depth int, includeBody bool, includeKinds, excludeKinds []int, substring bool, maxAnswerChars int) findOptions {
	opts := findOptions{
		depth:         depth,
		includeBody:   includeBody,
		substring:     substring,
		maxAnswerChar: maxAnswerChars,
	}
	if len(includeKinds) > 0 {
		opts.includeKinds = make(map[protocol.SymbolKind]struct{}, len(includeKinds))
		for _, k := range includeKinds {
			opts.includeKinds[protocol.SymbolKind(k)] = struct{}{}
		}
	}
	if len(excludeKinds) > 0 {
		opts.excludeKinds = make(map[protocol.SymbolKind]struct{}, len(excludeKinds))
		for _, k := range excludeKinds {
			opts.excludeKinds[protocol.SymbolKind(k)] = struct{}{}
		}
	}
	return opts
}

func (r *Retriever) resolvePaths(ctx context.Context, path string) ([]string, error) {
	fullPath := filepathext.SmartJoin(r.workingDir, path)
	info, err := os.Stat(fullPath)
	if err != nil {
		return nil, fmt.Errorf("path does not exist: %w", err)
	}

	if !info.IsDir() {
		return []string{fullPath}, nil
	}

	paths, truncated := r.workspaceSymbolPaths(ctx, fullPath)
	if len(paths) > 0 {
		if truncated && maxFilesPerDirectory > 0 && len(paths) > maxFilesPerDirectory {
			paths = paths[:maxFilesPerDirectory]
		}
		return paths, nil
	}

	matches, truncated, err := fsext.GlobWithDoubleStar("**", fullPath, maxFilesPerDirectory)
	if err != nil {
		return nil, err
	}
	if truncated && maxFilesPerDirectory > 0 {
		matches = matches[:maxFilesPerDirectory]
	}

	var files []string
	for _, m := range matches {
		stat, err := os.Stat(m)
		if err != nil || stat.IsDir() {
			continue
		}
		if r.clientForPath(m) != nil {
			files = append(files, m)
		}
	}
	return files, nil
}

func (r *Retriever) clientForPath(path string) *lsp.Client {
	for client := range r.lspClients.Seq() {
		if client.HandlesFile(path) {
			return client
		}
	}
	return nil
}

func (r *Retriever) workspaceSymbolPaths(ctx context.Context, dir string) ([]string, bool) {
	if ctx == nil {
		return nil, false
	}
	seen := make(map[string]struct{})
	var paths []string
	truncated := false
	for client := range r.lspClients.Seq() {
		// workspace symbols query "" may be heavy; cap results
		syms, err := client.WorkspaceSymbols(ctx, "")
		if err != nil {
			continue
		}
		for _, sym := range syms {
			loc := sym.GetLocation()
			if loc.URI == "" {
				continue
			}
			p, err := loc.URI.Path()
			if err != nil {
				continue
			}
			if !fsext.HasPrefix(p, dir) {
				continue
			}
			if _, ok := seen[p]; ok {
				continue
			}
			seen[p] = struct{}{}
			if r.clientForPath(p) == nil {
				continue
			}
			paths = append(paths, p)
			if workspaceSymbolPathLimit > 0 && len(paths) >= workspaceSymbolPathLimit {
				truncated = true
				return paths, truncated
			}
		}
	}
	return paths, truncated
}

func (r *Retriever) ensureFileAndClient(absPath string) (os.FileInfo, *lsp.Client, error) {
	info, err := os.Stat(absPath)
	if err != nil {
		return nil, nil, fmt.Errorf("path does not exist: %w", err)
	}
	if info.IsDir() {
		return nil, nil, fmt.Errorf("path must be a file, got directory: %s", absPath)
	}

	client := r.clientForPath(absPath)
	if client == nil {
		return nil, nil, fmt.Errorf("no LSP client available for %s", absPath)
	}
	return info, client, nil
}

func (r *Retriever) namePartsForSymbol(symbol protocol.DocumentSymbol) []string {
	return strings.Split(strings.Trim(symbol.Name, "/"), "/")
}

func toKindSet(kinds []int) map[protocol.SymbolKind]struct{} {
	if len(kinds) == 0 {
		return nil
	}
	res := make(map[protocol.SymbolKind]struct{}, len(kinds))
	for _, k := range kinds {
		res[protocol.SymbolKind(k)] = struct{}{}
	}
	return res
}

func (r *Retriever) symbolKindAt(ctx context.Context, absPath string, pos protocol.Position) (protocol.SymbolKind, bool) {
	sym, ok := r.containingSymbolAt(ctx, absPath, pos)
	if !ok {
		return 0, false
	}

	for kind, name := range symbolKindNames {
		if name == sym.Kind {
			return kind, true
		}
	}
	return 0, false
}

func (r *Retriever) containingSymbolAt(ctx context.Context, absPath string, pos protocol.Position) (Symbol, bool) {
	_, client, err := r.ensureFileAndClient(absPath)
	if err != nil {
		return Symbol{}, false
	}
	docSymbols, err := r.documentSymbols(ctx, client, absPath)
	if err != nil {
		return Symbol{}, false
	}

	var best *protocol.DocumentSymbol
	var bestNameParts []string
	var bestSpan int64

	posInRange := func(rng protocol.Range, p protocol.Position) bool {
		beforeStart := p.Line < rng.Start.Line || (p.Line == rng.Start.Line && p.Character < rng.Start.Character)
		afterEnd := p.Line > rng.End.Line || (p.Line == rng.End.Line && p.Character > rng.End.Character)
		return !beforeStart && !afterEnd
	}

	rangeSpan := func(rng protocol.Range) int64 {
		return int64(rng.End.Line-rng.Start.Line)<<32 + int64(rng.End.Character-rng.Start.Character)
	}

	var walk func(protocol.DocumentSymbol, []string)
	walk = func(sym protocol.DocumentSymbol, ancestors []string) {
		if !posInRange(sym.Range, pos) {
			return
		}
		span := rangeSpan(sym.Range)
		if best == nil || span < bestSpan {
			tmp := sym
			best = &tmp
			bestNameParts = append([]string(nil), append(ancestors, sym.Name)...)
			bestSpan = span
		}
		for _, child := range sym.Children {
			walk(child, append(ancestors, sym.Name))
		}
	}

	for _, s := range docSymbols {
		walk(s, []string{})
	}

	if best == nil {
		return Symbol{}, false
	}

	result := r.buildSymbol(*best, bestNameParts, r.relPath(absPath), false, 0)
	return result, true
}

func (r *Retriever) relPath(absPath string) string {
	rel, err := filepath.Rel(r.workingDir, absPath)
	if err != nil {
		return absPath
	}
	return filepath.ToSlash(rel)
}

var symbolKindNames = map[protocol.SymbolKind]string{
	protocol.File:          "File",
	protocol.Module:        "Module",
	protocol.Namespace:     "Namespace",
	protocol.Package:       "Package",
	protocol.Class:         "Class",
	protocol.Method:        "Method",
	protocol.Property:      "Property",
	protocol.Field:         "Field",
	protocol.Constructor:   "Constructor",
	protocol.Enum:          "Enum",
	protocol.Interface:     "Interface",
	protocol.Function:      "Function",
	protocol.Variable:      "Variable",
	protocol.Constant:      "Constant",
	protocol.String:        "String",
	protocol.Number:        "Number",
	protocol.Boolean:       "Boolean",
	protocol.Array:         "Array",
	protocol.Object:        "Object",
	protocol.Key:           "Key",
	protocol.Null:          "Null",
	protocol.EnumMember:    "EnumMember",
	protocol.Struct:        "Struct",
	protocol.Event:         "Event",
	protocol.Operator:      "Operator",
	protocol.TypeParameter: "TypeParameter",
}

func symbolKindFromName(name string) (protocol.SymbolKind, bool) {
	for k, v := range symbolKindNames {
		if v == name {
			return k, true
		}
	}
	return 0, false
}

type namePathMatcher struct {
	patternParts  []string
	isAbs         bool
	substring     bool
	overloadIdx   *int
	withSignature bool
}

func newNamePathMatcher(pattern string, substring bool) namePathMatcher {
	raw := pattern
	trimmed := strings.Trim(pattern, "/")
	if trimmed == "" {
		return namePathMatcher{patternParts: []string{}, substring: substring}
	}

	parts := strings.Split(trimmed, "/")
	isAbs := strings.HasPrefix(raw, "/")

	last := parts[len(parts)-1]
	var overload *int
	if idxStart := strings.LastIndex(last, "["); idxStart != -1 && strings.HasSuffix(last, "]") {
		if n, err := parseOverloadIndex(last[idxStart+1 : len(last)-1]); err == nil {
			val := n
			overload = &val
			parts[len(parts)-1] = last[:idxStart]
		}
	}

	withSig := strings.Contains(parts[len(parts)-1], "(")

	return namePathMatcher{
		patternParts:  parts,
		isAbs:         isAbs,
		substring:     substring,
		overloadIdx:   overload,
		withSignature: withSig,
	}
}

func (m namePathMatcher) matches(nameParts []string, overloadIdx *int, signature string) bool {
	if len(m.patternParts) == 0 {
		return false
	}
	if len(m.patternParts) > len(nameParts) {
		return false
	}

	if m.isAbs && len(m.patternParts) != len(nameParts) {
		return false
	}

	if len(m.patternParts) > 1 {
		start := len(nameParts) - len(m.patternParts)
		ancestors := nameParts[start : len(nameParts)-1]
		if !equalStrings(ancestors, m.patternParts[:len(m.patternParts)-1]) {
			return false
		}
	}

	target := m.patternParts[len(m.patternParts)-1]
	candidate := nameParts[len(nameParts)-1]
	if m.withSignature && signature != "" {
		if !signatureMatches(target, signature) && !signatureMatches(candidate+target, signature) {
			return false
		}
	} else {
		if m.substring {
			if !strings.Contains(candidate, target) {
				return false
			}
		} else if candidate != target {
			return false
		}
	}

	if m.overloadIdx != nil && (overloadIdx == nil || *overloadIdx != *m.overloadIdx) {
		return false
	}

	return true
}

func parseOverloadIndex(val string) (int, error) {
	var n int
	_, err := fmt.Sscanf(val, "%d", &n)
	return n, err
}

func splitSymbolName(name string) (string, *int) {
	idxStart := strings.LastIndex(name, "[")
	if idxStart != -1 && strings.HasSuffix(name, "]") {
		if n, err := parseOverloadIndex(name[idxStart+1 : len(name)-1]); err == nil {
			val := n
			return name[:idxStart], &val
		}
	}
	return name, nil
}

func looksLikeImport(snippet string, path string) bool {
	trimmed := strings.TrimSpace(snippet)
	if trimmed == "" {
		return false
	}
	lower := strings.ToLower(trimmed)

	ext := strings.ToLower(filepath.Ext(path))

	switch ext {
	case ".go":
		if strings.HasPrefix(lower, "import ") {
			return true
		}
		if strings.HasPrefix(lower, "import(") || strings.HasPrefix(lower, "import (") {
			return true
		}
	case ".py":
		if strings.HasPrefix(lower, "import ") || strings.HasPrefix(lower, "from ") {
			return true
		}
	case ".js", ".ts", ".jsx", ".tsx", ".mjs", ".cjs":
		if strings.HasPrefix(lower, "import ") {
			return true
		}
		if strings.HasPrefix(lower, "const ") && strings.Contains(lower, "require(") {
			return true
		}
		if strings.HasPrefix(lower, "var ") && strings.Contains(lower, "require(") {
			return true
		}
		if strings.HasPrefix(lower, "require(") {
			return true
		}
	case ".rb":
		if strings.HasPrefix(lower, "require ") || strings.HasPrefix(lower, "require_relative ") {
			return true
		}
	case ".rs":
		if strings.HasPrefix(lower, "use ") {
			return true
		}
	case ".cs":
		if strings.HasPrefix(lower, "using ") {
			return true
		}
	case ".cpp", ".cc", ".cxx", ".h", ".hpp", ".c":
		if strings.HasPrefix(lower, "#include") || strings.HasPrefix(lower, "using ") {
			return true
		}
	}

	// generic fallback
	return strings.HasPrefix(lower, "import ") ||
		strings.HasPrefix(lower, "from ") ||
		strings.HasPrefix(lower, "using ") ||
		strings.HasPrefix(lower, "require ") ||
		strings.HasPrefix(lower, "#include")
}

func sortLine(val any) int {
	switch v := val.(type) {
	case *SymbolLocation:
		if v != nil {
			return v.StartLine
		}
	case SymbolLocation:
		return v.StartLine
	}
	return 0
}

func fileLineSpan(absPath string) (int, int) {
	data, err := os.ReadFile(absPath)
	if err != nil {
		return 1, 1
	}
	lines := strings.Count(string(data), "\n") + 1
	return 1, lines
}

func (r *Retriever) signatureForSymbolCached(ctx context.Context, symbol protocol.DocumentSymbol, relPath string, cache map[string]string) string {
	abs := filepathext.SmartJoin(r.workingDir, relPath)
	info, client, err := r.ensureFileAndClient(abs)
	if err != nil || client == nil || info == nil {
		return ""
	}
	sig := r.signatureForSymbol(ctx, client, abs, symbol)
	if sig != "" {
		key := fmt.Sprintf("%s:%d:%d", relPath, symbol.SelectionRange.Start.Line, symbol.SelectionRange.Start.Character)
		cache[key] = sig
	}
	return sig
}

func (r *Retriever) signatureForSymbol(ctx context.Context, client *lsp.Client, absPath string, symbol protocol.DocumentSymbol) string {
	if client == nil {
		return symbol.Detail
	}
	line := int(symbol.SelectionRange.Start.Line) + 1
	character := int(symbol.SelectionRange.Start.Character) + 1
	help, err := client.SignatureHelp(ctx, absPath, line, character)
	if err != nil || help == nil || len(help.Signatures) == 0 {
		return symbol.Detail
	}
	return help.Signatures[0].Label
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
