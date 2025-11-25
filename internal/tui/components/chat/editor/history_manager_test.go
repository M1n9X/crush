package editor

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHistoryManager(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "crush-history-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	historyPath := filepath.Join(tmpDir, "history.jsonl")
	manager := &HistoryManager{
		filePath: historyPath,
	}

	// Test saving entries
	projectA := "/path/to/projectA"
	projectB := "/path/to/projectB"

	entries := []struct {
		text    string
		project string
	}{
		{"cmdA1", projectA},
		{"cmdB1", projectB},
		{"cmdA2", projectA},
		{"cmdA2", projectA}, // Duplicate
		{"cmdA3", projectA},
	}

	for _, e := range entries {
		if err := manager.Save(e.text, e.project); err != nil {
			t.Errorf("Failed to save entry %v: %v", e, err)
		}
	}

	// Test retrieving project A history
	histA, err := manager.GetProjectHistory(projectA)
	if err != nil {
		t.Fatalf("Failed to get project A history: %v", err)
	}

	expectedA := []string{"cmdA1", "cmdA2", "cmdA3"}
	if len(histA) != len(expectedA) {
		t.Errorf("Expected %d entries for project A, got %d", len(expectedA), len(histA))
	}
	for i, txt := range histA {
		if txt != expectedA[i] {
			t.Errorf("Expected entry %d to be %s, got %s", i, expectedA[i], txt)
		}
	}

	// Test retrieving project B history
	histB, err := manager.GetProjectHistory(projectB)
	if err != nil {
		t.Fatalf("Failed to get project B history: %v", err)
	}

	expectedB := []string{"cmdB1"}
	if len(histB) != len(expectedB) {
		t.Errorf("Expected %d entries for project B, got %d", len(expectedB), len(histB))
	}
	for i, txt := range histB {
		if txt != expectedB[i] {
			t.Errorf("Expected entry %d to be %s, got %s", i, expectedB[i], txt)
		}
	}
}
