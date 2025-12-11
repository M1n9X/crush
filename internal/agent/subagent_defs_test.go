package agent

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseSubAgentDefinition_ExtendedFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent.md")
	content := `---
name: explorer
description: test explorer
tools:
  write: false
  read: true
mode: all
model_name: small
permission:
  edit: allow
  bash:
    "*": ask
max_steps: 5
color: "#123456"
---
Custom prompt body`

	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	def, err := parseSubAgentDefinition(path, "test-source")
	require.NoError(t, err)

	require.Equal(t, "explorer", def.Name)
	require.Equal(t, "all", def.Mode)
	require.Equal(t, 5, def.MaxSteps)
	require.Equal(t, "test-source", def.Source)
	require.Equal(t, "#123456", def.Color)
	require.Equal(t, "Custom prompt body", def.SystemPrompt)
	require.False(t, def.Wildcard)
	require.ElementsMatch(t, []string{"read"}, def.Tools)
	require.Equal(t, map[string]bool{"write": false, "read": true}, def.ToolsEnabled)
	require.Equal(t, "allow", def.Permissions.Edit)
	require.Equal(t, "ask", def.Permissions.Bash["*"])
}

func TestLoadSubAgentDefinitions_IncludesBuiltins(t *testing.T) {
	defs, err := loadSubAgentDefinitions(t.TempDir())
	require.NoError(t, err)

	names := make(map[string]bool)
	for _, def := range defs {
		names[def.Name] = true
	}

	require.True(t, names["general"], "missing builtin general")
	require.True(t, names["general-purpose"], "missing builtin general-purpose")
	require.True(t, names["coder"], "missing builtin coder")
	require.True(t, names["reviewer"], "missing builtin reviewer")
	require.True(t, names["explore"], "missing builtin explore")
	require.True(t, names["plan"], "missing builtin plan")
}
