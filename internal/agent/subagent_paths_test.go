package agent

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadSubAgentDefinitions_ProjectPaths(t *testing.T) {
	// Setup temporary directory structure
	tmpDir, err := os.MkdirTemp("", "crush-test-agents")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create .claude/agents and .codebreeze/agents directories
	claudeDir := filepath.Join(tmpDir, ".claude", "agents")
	codebreezeDir := filepath.Join(tmpDir, ".codebreeze", "agents")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(codebreezeDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a dummy agent in .claude
	claudeAgent := `---
name: claude-agent
description: A test agent from .claude
tools: "*"
---
You are a claude agent.`
	if err := os.WriteFile(filepath.Join(claudeDir, "claude-agent.md"), []byte(claudeAgent), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a dummy agent in .codebreeze
	codebreezeAgent := `---
name: codebreeze-agent
description: A test agent from .codebreeze
tools: "*"
color: "#ff0000"
---
You are a codebreeze agent.`
	if err := os.WriteFile(filepath.Join(codebreezeDir, "codebreeze-agent.md"), []byte(codebreezeAgent), 0644); err != nil {
		t.Fatal(err)
	}

	// Load definitions
	defs, err := loadSubAgentDefinitions(tmpDir)
	if err != nil {
		t.Fatalf("Failed to load definitions: %v", err)
	}

	// Verify agents are loaded
	foundClaude := false
	foundCodebreeze := false
	for _, def := range defs {
		if def.Name == "claude-agent" {
			foundClaude = true
			if def.Source != "project-.claude" {
				t.Errorf("Expected source project-.claude, got %s", def.Source)
			}
		}
		if def.Name == "codebreeze-agent" {
			foundCodebreeze = true
			if def.Source != "project-.codebreeze" {
				t.Errorf("Expected source project-.codebreeze, got %s", def.Source)
			}
			if def.Color != "#ff0000" {
				t.Errorf("Expected color #ff0000, got %s", def.Color)
			}
		}
	}

	if !foundClaude {
		t.Error("Failed to find claude-agent")
	}
	if !foundCodebreeze {
		t.Error("Failed to find codebreeze-agent")
	}
}
