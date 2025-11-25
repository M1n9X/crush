package lsp

import (
	"context"
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/env"
	"github.com/stretchr/testify/require"
)

func TestClient(t *testing.T) {
	ctx := context.Background()

	// Create a simple config for testing
	cfg := config.LSPConfig{
		Command:   "$THE_CMD", // Use echo as a dummy command that won't fail
		Args:      []string{"hello"},
		FileTypes: []string{"go"},
		Env:       map[string]string{},
	}

	// Test creating a powernap client - this will likely fail with echo
	// but we can still test the basic structure
	client, err := New(ctx, "test", cfg, config.NewEnvironmentVariableResolver(env.NewFromMap(map[string]string{
		"THE_CMD": "echo",
	})))
	if err != nil {
		// Expected to fail with echo command, skip the rest
		t.Skipf("Powernap client creation failed as expected with dummy command: %v", err)
		return
	}

	// If we get here, test basic interface methods
	if client.GetName() != "test" {
		t.Errorf("Expected name 'test', got '%s'", client.GetName())
	}

	if !client.HandlesFile("test.go") {
		t.Error("Expected client to handle .go files")
	}

	if client.HandlesFile("test.py") {
		t.Error("Expected client to not handle .py files")
	}

	// Test server state
	client.SetServerState(StateReady)
	if client.GetServerState() != StateReady {
		t.Error("Expected server state to be StateReady")
	}

	// Clean up - expect this to fail with echo command
	if err := client.Close(t.Context()); err != nil {
		// Expected to fail with echo command
		t.Logf("Close failed as expected with dummy command: %v", err)
	}
}

func TestResolveLSPConfig(t *testing.T) {
	resolver := config.NewEnvironmentVariableResolver(env.NewFromMap(map[string]string{
		"CMD":    "echo",
		"ARG":    "world",
		"ENVVAR": "/custom/path",
	}))

	cfg := config.LSPConfig{
		Command: "$CMD",
		Args:    []string{"hello", "$ARG"},
		Env: map[string]string{
			"CUSTOM_PATH": "$ENVVAR",
		},
	}

	resolved, err := resolveLSPConfig(cfg, resolver)
	require.NoError(t, err)
	require.Equal(t, "echo", resolved.command)
	require.Equal(t, []string{"hello", "world"}, resolved.args)
	require.Equal(t, map[string]string{"CUSTOM_PATH": "/custom/path"}, resolved.env)
}

func TestResolveLSPConfigEnvError(t *testing.T) {
	resolver := config.NewEnvironmentVariableResolver(env.NewFromMap(nil))
	cfg := config.LSPConfig{
		Command: "echo",
		Env: map[string]string{
			"CUSTOM_PATH": "$MISSING",
		},
	}

	_, err := resolveLSPConfig(cfg, resolver)
	require.Error(t, err)
}
