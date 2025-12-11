package agent

import (
	"testing"

	codexsdk "github.com/M1n9X/codex-sdk-go"
	"github.com/charmbracelet/crush/internal/subagent"
	"github.com/stretchr/testify/require"
)

func TestParseCodexSandbox(t *testing.T) {
	tests := []struct {
		input    string
		expected subagent.SandboxMode
	}{
		{"danger-full-access", subagent.SandboxDangerFullAccess},
		{"DANGER-FULL-ACCESS", subagent.SandboxDangerFullAccess},
		{"workspace-write", subagent.SandboxWorkspaceWrite},
		{"  Workspace-Write  ", subagent.SandboxWorkspaceWrite},
		{"read-only", subagent.SandboxReadOnly},
		{"READ-ONLY", subagent.SandboxReadOnly},
		{"", ""},
		{"unknown", ""},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseCodexSandbox(tt.input)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestParseCodexApprovalMode(t *testing.T) {
	tests := []struct {
		input    string
		expected codexsdk.ApprovalMode
	}{
		{"never", codexsdk.ApprovalNever},
		{"ApprovalNever", codexsdk.ApprovalNever},
		{"on-request", codexsdk.ApprovalOnRequest},
		{"ApprovalOnRequest", codexsdk.ApprovalOnRequest},
		{"request", codexsdk.ApprovalOnRequest},
		{"on-failure", codexsdk.ApprovalOnFailure},
		{"ApprovalOnFailure", codexsdk.ApprovalOnFailure},
		{"failure", codexsdk.ApprovalOnFailure},
		{"untrusted", codexsdk.ApprovalUntrusted},
		{"ApprovalUntrusted", codexsdk.ApprovalUntrusted},
		{"", ""},
		{"unknown", ""},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseCodexApprovalMode(tt.input)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestParseCodexReasoning(t *testing.T) {
	tests := []struct {
		input    string
		expected codexsdk.ModelReasoningEffort
	}{
		{"minimal", codexsdk.ReasoningMinimal},
		{"MINIMAL", codexsdk.ReasoningMinimal},
		{"low", codexsdk.ReasoningLow},
		{"medium", codexsdk.ReasoningMedium},
		{"high", codexsdk.ReasoningHigh},
		{"", ""},
		{"ultra", ""},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseCodexReasoning(tt.input)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestParseBoolPtr(t *testing.T) {
	trueVal := true
	falseVal := false
	tests := []struct {
		input    string
		expected *bool
	}{
		{"1", &trueVal},
		{"t", &trueVal},
		{"true", &trueVal},
		{"TRUE", &trueVal},
		{"yes", &trueVal},
		{"y", &trueVal},
		{"on", &trueVal},
		{"0", &falseVal},
		{"f", &falseVal},
		{"false", &falseVal},
		{"FALSE", &falseVal},
		{"no", &falseVal},
		{"n", &falseVal},
		{"off", &falseVal},
		{"", nil},
		{"maybe", nil},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseBoolPtr(tt.input)
			if tt.expected == nil {
				require.Nil(t, result)
			} else {
				require.NotNil(t, result)
				require.Equal(t, *tt.expected, *result)
			}
		})
	}
}

func TestParseAdditionalDirectories(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{"empty", "", nil},
		{"single", "/foo", []string{"/foo"}},
		{"comma separated", "/foo,/bar", []string{"/foo", "/bar"}},
		{"semicolon separated", "/foo;/bar", []string{"/foo", "/bar"}},
		{"mixed", "/a,/b;/c", []string{"/a", "/b", "/c"}},
		{"with spaces", " /foo , /bar ; /baz ", []string{"/foo", "/bar", "/baz"}},
		{"empty parts", "/foo,,/bar", []string{"/foo", "/bar"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseAdditionalDirectories(tt.input)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestParseEnvMap(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected map[string]string
	}{
		{"empty", "", nil},
		{"single pair", "FOO=bar", map[string]string{"FOO": "bar"}},
		{"multiple comma", "A=1,B=2", map[string]string{"A": "1", "B": "2"}},
		{"multiple semicolon", "A=1;B=2", map[string]string{"A": "1", "B": "2"}},
		{"value with equals", "URL=http://a?b=c", map[string]string{"URL": "http://a?b=c"}},
		{"with spaces", " KEY = val , OTHER = data ", map[string]string{"KEY": "val", "OTHER": "data"}},
		{"missing value", "KEY", nil},
		{"empty key", "=val", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseEnvMap(tt.input)
			if tt.expected == nil {
				if result != nil {
					require.Empty(t, result)
				}
			} else {
				require.Equal(t, tt.expected, result)
			}
		})
	}
}

// Note: TestBuildCodexOptionsFromEnv is tested via integration tests rather than
// unit tests since config.Config.setDefaults is unexported. The individual parsing
// functions above provide comprehensive coverage of the env var parsing logic.
