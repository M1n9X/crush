package agent

import (
	"testing"

	"github.com/charmbracelet/crush/internal/subagent"
	"github.com/stretchr/testify/require"
)

func TestDeriveSandbox(t *testing.T) {
	require.Equal(t, subagent.SandboxReadOnly, deriveSandbox(nil))

	allow := subagent.Profile{Permissions: subagent.PermissionMatrix{Edit: "allow"}}
	require.Equal(t, subagent.SandboxWorkspaceWrite, deriveSandbox(&allow))

	ask := subagent.Profile{Permissions: subagent.PermissionMatrix{Edit: "ask"}}
	require.Equal(t, subagent.SandboxWorkspaceWrite, deriveSandbox(&ask))

	danger := subagent.Profile{Permissions: subagent.PermissionMatrix{Edit: "danger"}}
	require.Equal(t, subagent.SandboxDangerFullAccess, deriveSandbox(&danger))

	deny := subagent.Profile{Permissions: subagent.PermissionMatrix{Edit: "deny"}}
	require.Equal(t, subagent.SandboxReadOnly, deriveSandbox(&deny))
}

func TestDeriveAllowedTools(t *testing.T) {
	base := []string{"write", "edit", "read", "view", "grep"}

	t.Run("nil profile returns base", func(t *testing.T) {
		result := deriveAllowedTools(base, nil)
		require.ElementsMatch(t, base, result)
	})

	t.Run("wildcard profile keeps all base tools", func(t *testing.T) {
		profile := &subagent.Profile{Wildcard: true}
		result := deriveAllowedTools(base, profile)
		require.ElementsMatch(t, base, result)
	})

	t.Run("explicit tools list filters to intersection", func(t *testing.T) {
		profile := &subagent.Profile{
			Wildcard: false,
			Tools:    []string{"read", "view", "nonexistent"},
		}
		result := deriveAllowedTools(base, profile)
		require.ElementsMatch(t, []string{"read", "view"}, result)
	})

	t.Run("ToolsEnabled with false removes tools", func(t *testing.T) {
		profile := &subagent.Profile{
			Wildcard:     true,
			ToolsEnabled: map[string]bool{"write": false, "edit": false},
		}
		result := deriveAllowedTools(base, profile)
		require.ElementsMatch(t, []string{"read", "view", "grep"}, result)
		require.NotContains(t, result, "write")
		require.NotContains(t, result, "edit")
	})

	t.Run("ToolsEnabled with true adds tools", func(t *testing.T) {
		profile := &subagent.Profile{
			Wildcard:     true,
			ToolsEnabled: map[string]bool{"bash": true},
		}
		result := deriveAllowedTools(base, profile)
		require.Contains(t, result, "bash")
		require.Contains(t, result, "write")
	})

	t.Run("reviewer profile denies write and edit", func(t *testing.T) {
		// This simulates the built-in reviewer profile
		profile := &subagent.Profile{
			Name:         "reviewer",
			Wildcard:     true,
			ToolsEnabled: map[string]bool{"write": false, "edit": false},
			Permissions:  subagent.PermissionMatrix{Edit: "deny"},
		}
		result := deriveAllowedTools(base, profile)
		require.NotContains(t, result, "write")
		require.NotContains(t, result, "edit")
		require.Contains(t, result, "read")
		require.Contains(t, result, "view")
	})

	t.Run("combined explicit list and toggles", func(t *testing.T) {
		profile := &subagent.Profile{
			Wildcard:     false,
			Tools:        []string{"read", "view", "write"},
			ToolsEnabled: map[string]bool{"write": false, "bash": true},
		}
		result := deriveAllowedTools(base, profile)
		// Only read and view from intersection, plus bash added
		require.Contains(t, result, "read")
		require.Contains(t, result, "view")
		require.Contains(t, result, "bash")
		require.NotContains(t, result, "write") // Explicitly disabled
		require.NotContains(t, result, "edit")  // Not in Tools list
	})
}
