package projectdoc

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/fsext"
	"github.com/charmbracelet/crush/internal/home"
)

// Doc represents a discovered project instruction file.
type Doc struct {
	Path    string
	Content string
}

// Load discovers and reads project instruction files (AGENTS-like docs) based
// on the current working directory and configuration. It respects the maximum
// byte budget and ordering rules inspired by Codex/OpenCode:
//   - Global ~/.crush AGENTS override first
//   - Then from git root (or cwd) down to cwd: AGENTS.override.md, AGENTS.md,
//     InitializeAs, then configured fallbacks (first match per directory)
//
// Files that exceed the remaining budget are truncated with a warning.
func Load(cfg *config.Config) ([]Doc, []string) {
	maxBytes := cfg.Options.ProjectDocMaxBytes
	if maxBytes <= 0 {
		return nil, nil
	}

	var (
		docs      []Doc
		warnings  []string
		seen      = map[string]struct{}{}
		remaining = maxBytes
	)

	candidates := candidateFilenames(cfg)

	addDoc := func(path string) {
		if path == "" {
			return
		}
		path = filepath.Clean(path)
		if _, ok := seen[path]; ok {
			return
		}
		seen[path] = struct{}{}

		content, truncated, err := readWithLimit(path, remaining)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("error reading %s: %v", path, err))
			return
		}
		if strings.TrimSpace(content) == "" {
			return
		}

		docs = append(docs, Doc{
			Path:    filepath.ToSlash(path),
			Content: content,
		})
		if truncated {
			warnings = append(warnings, fmt.Sprintf("project doc %s exceeded byte budget; truncated to %d bytes", path, remaining))
		}
		remaining -= len(content)
	}

	// 1) Global ~/.crush overrides.
	for _, root := range []string{filepath.Join(home.Dir(), ".crush")} {
		if path := firstExisting(root, candidates); path != "" {
			addDoc(path)
		}
		if remaining <= 0 {
			return docs, warnings
		}
	}

	// 2) From git root (or cwd) down to cwd.
	for _, dir := range searchDirs(cfg.WorkingDir()) {
		if path := firstExisting(dir, candidates); path != "" {
			addDoc(path)
		}
		if remaining <= 0 {
			return docs, warnings
		}
	}

	return docs, warnings
}

func candidateFilenames(cfg *config.Config) []string {
	names := []string{
		"AGENTS.override.md",
		"AGENTS.md",
	}
	if cfg.Options.InitializeAs != "" {
		names = append(names, cfg.Options.InitializeAs)
	}
	for _, name := range cfg.Options.ProjectDocFallbackFilenames {
		if name == "" {
			continue
		}
		if !slices.Contains(names, name) {
			names = append(names, name)
		}
	}
	return names
}

// searchDirs returns the ordered list of directories to search from git root
// (if found) down to cwd (inclusive).
func searchDirs(cwd string) []string {
	if cwd == "" {
		return nil
	}

	var chain []string
	gitRoot := fsext.FindGitRoot(cwd)
	if gitRoot == "" {
		return []string{cwd}
	}

	cursor := cwd
	for {
		chain = append(chain, cursor)
		if cursor == gitRoot {
			break
		}
		parent := filepath.Dir(cursor)
		if parent == cursor {
			break
		}
		cursor = parent
	}

	// Reverse so we search from root -> cwd.
	slices.Reverse(chain)
	return chain
}

func firstExisting(dir string, candidates []string) string {
	for _, name := range candidates {
		path := filepath.Join(dir, name)
		info, err := os.Lstat(path)
		if err != nil {
			continue
		}
		if info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			return path
		}
	}
	return ""
}

func readWithLimit(path string, limit int) (string, bool, error) {
	if limit <= 0 {
		return "", false, nil
	}

	f, err := os.Open(path)
	if err != nil {
		return "", false, err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return "", false, err
	}

	reader := io.LimitReader(f, int64(limit))
	data, err := io.ReadAll(reader)
	if err != nil {
		return "", false, err
	}

	truncated := info.Size() > int64(len(data))
	return string(data), truncated, nil
}
