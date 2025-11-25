package memory

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

const (
	DefaultMemoryFile       = "memory.md"
	maxMemoryFiles          = 10
	maxTotalMemoryBytes     = 256_000
	maxSingleMemoryFileSize = 64_000
)

var agentSegmentSanitizer = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

// Entry represents a memory file on disk.
type Entry struct {
	// Path is a user-friendly path (e.g. memory/<agent>/notes.md).
	Path string
	// FullPath is the absolute path on disk.
	FullPath string
	// Content is the file body (possibly truncated to stay within limits).
	Content string
	// Truncated indicates whether Content was shortened to respect limits.
	Truncated bool
}

// Service manages per-agent memory files stored under the data directory.
type Service struct {
	root string
}

// NewService constructs a memory service rooted at dataDir/memory/agents.
func NewService(dataDir string) *Service {
	return &Service{
		root: filepath.Join(dataDir, "memory", "agents"),
	}
}

// List returns memory entries for an agent, bounded by count/size budgets.
func (s *Service) List(agent string) ([]Entry, error) {
	root := s.agentRoot(agent)
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("create memory root: %w", err)
	}

	entries := []Entry{}
	totalBytes := 0

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if len(entries) >= maxMemoryFiles || totalBytes >= maxTotalMemoryBytes {
			return fs.SkipDir
		}
		if strings.HasPrefix(d.Name(), ".") {
			return nil
		}

		info, statErr := d.Info()
		if statErr != nil {
			return statErr
		}
		if info.Size() == 0 {
			entries = append(entries, Entry{
				Path:     s.displayPath(agent, path),
				FullPath: path,
			})
			return nil
		}
		if info.Size() > maxSingleMemoryFileSize {
			data, readErr := os.ReadFile(path)
			if readErr != nil {
				return readErr
			}
			content := string(data[:maxSingleMemoryFileSize])
			entries = append(entries, Entry{
				Path:      s.displayPath(agent, path),
				FullPath:  path,
				Content:   content,
				Truncated: true,
			})
			totalBytes += len(content)
			return nil
		}

		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		content := string(data)
		entries = append(entries, Entry{
			Path:     s.displayPath(agent, path),
			FullPath: path,
			Content:  content,
		})
		totalBytes += len(content)
		return nil
	})

	if err != nil {
		return nil, err
	}

	// Prefer most recently modified files first to mirror "freshest" memory.
	slices.SortFunc(entries, func(a, b Entry) int {
		ainfo, _ := os.Stat(a.FullPath)
		binfo, _ := os.Stat(b.FullPath)
		if ainfo != nil && binfo != nil {
			return int(binfo.ModTime().Unix() - ainfo.ModTime().Unix())
		}
		return strings.Compare(a.Path, b.Path)
	})

	if len(entries) > maxMemoryFiles {
		entries = entries[:maxMemoryFiles]
	}

	return entries, nil
}

// Read loads a single memory entry for an agent.
func (s *Service) Read(agent, filePath string) (Entry, error) {
	fullPath, display, err := s.resolve(agent, filePath)
	if err != nil {
		return Entry{}, err
	}

	data, err := os.ReadFile(fullPath)
	if err != nil {
		return Entry{}, err
	}

	entry := Entry{
		Path:     display,
		FullPath: fullPath,
		Content:  string(data),
	}
	if len(entry.Content) > maxSingleMemoryFileSize {
		entry.Content = entry.Content[:maxSingleMemoryFileSize]
		entry.Truncated = true
	}
	return entry, nil
}

// Write persists a memory entry for an agent.
func (s *Service) Write(agent, filePath, content string) (Entry, error) {
	fullPath, display, err := s.resolve(agent, filePath)
	if err != nil {
		return Entry{}, err
	}

	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		return Entry{}, fmt.Errorf("create memory directory: %w", err)
	}

	if err := os.WriteFile(fullPath, []byte(content), 0o600); err != nil {
		return Entry{}, fmt.Errorf("write memory file: %w", err)
	}

	return Entry{
		Path:     display,
		FullPath: fullPath,
		Content:  content,
	}, nil
}

func (s *Service) resolve(agent, filePath string) (fullPath, display string, err error) {
	if agent == "" {
		return "", "", errors.New("agent is required")
	}
	agent = s.sanitizeAgent(agent)

	rel := filepath.Clean(filePath)
	rel = strings.TrimPrefix(rel, string(filepath.Separator))
	if rel == "" || rel == "." {
		rel = DefaultMemoryFile
	}
	if strings.HasPrefix(rel, "..") {
		return "", "", fmt.Errorf("invalid memory path: %s", rel)
	}

	root := s.agentRoot(agent)
	full := filepath.Join(root, rel)
	if !strings.HasPrefix(full, root) {
		return "", "", fmt.Errorf("memory path escapes agent root: %s", rel)
	}

	return full, s.displayPath(agent, full), nil
}

func (s *Service) agentRoot(agent string) string {
	return filepath.Join(s.root, s.sanitizeAgent(agent))
}

func (s *Service) displayPath(agent, full string) string {
	rel, err := filepath.Rel(s.agentRoot(agent), full)
	if err != nil {
		return filepath.ToSlash(full)
	}
	return filepath.ToSlash(filepath.Join("memory", s.sanitizeAgent(agent), rel))
}

func (s *Service) sanitizeAgent(agent string) string {
	sanitized := agentSegmentSanitizer.ReplaceAllString(agent, "-")
	sanitized = strings.Trim(sanitized, ".-_/ ")
	if sanitized == "" {
		return "default"
	}
	return sanitized
}
