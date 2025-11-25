package editor

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// HistoryEntry represents a single command history item
type HistoryEntry struct {
	Text      string `json:"text"`
	Timestamp int64  `json:"timestamp"`
	Project   string `json:"project"`
}

// HistoryManager handles persistent storage and retrieval of command history
type HistoryManager struct {
	filePath string
	mu       sync.RWMutex
}

// NewHistoryManager creates a new HistoryManager
func NewHistoryManager() *HistoryManager {
	home, err := os.UserHomeDir()
	if err != nil {
		return &HistoryManager{}
	}
	
	dir := filepath.Join(home, ".crush")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return &HistoryManager{}
	}

	return &HistoryManager{
		filePath: filepath.Join(dir, "history.jsonl"),
	}
}

// Save appends a new entry to the history file
func (m *HistoryManager) Save(text string, project string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.filePath == "" {
		return nil
	}

	entry := HistoryEntry{
		Text:      strings.TrimSpace(text),
		Timestamp: time.Now().Unix(),
		Project:   project,
	}

	f, err := os.OpenFile(m.filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	if _, err := f.Write(data); err != nil {
		return err
	}
	if _, err := f.WriteString("\n"); err != nil {
		return err
	}

	return nil
}

// GetProjectHistory returns the history entries for a specific project
func (m *HistoryManager) GetProjectHistory(project string) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.filePath == "" {
		return nil, nil
	}

	f, err := os.Open(m.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	var entries []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var entry HistoryEntry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			continue
		}

		if entry.Project == project && entry.Text != "" {
			// Deduplicate consecutive entries
			if len(entries) > 0 && entries[len(entries)-1] == entry.Text {
				continue
			}
			entries = append(entries, entry.Text)
		}
	}

	return entries, scanner.Err()
}
