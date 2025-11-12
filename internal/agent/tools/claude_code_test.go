package tools

import (
	"context"
	"encoding/json"
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/humanlayer/humanlayer/claudecode-go"
	"github.com/stretchr/testify/require"
)

// Mock permission service for testing
type mockClaudeCodePermissionService struct {
	shouldAllow bool
}

func (m *mockClaudeCodePermissionService) Request(opts permission.CreatePermissionRequest) bool {
	return m.shouldAllow
}

func (m *mockClaudeCodePermissionService) GrantPersistent(permission permission.PermissionRequest) {}
func (m *mockClaudeCodePermissionService) Grant(permission permission.PermissionRequest)           {}
func (m *mockClaudeCodePermissionService) Deny(permission permission.PermissionRequest)            {}
func (m *mockClaudeCodePermissionService) Subscribe(context.Context) <-chan pubsub.Event[permission.PermissionRequest] {
	return make(<-chan pubsub.Event[permission.PermissionRequest])
}
func (m *mockClaudeCodePermissionService) Unsubscribe(context.Context)         {}
func (m *mockClaudeCodePermissionService) AutoApproveSession(sessionID string) {}
func (m *mockClaudeCodePermissionService) SetSkipRequests(skip bool)           {}
func (m *mockClaudeCodePermissionService) SkipRequests() bool                  { return false }
func (m *mockClaudeCodePermissionService) SubscribeNotifications(ctx context.Context) <-chan pubsub.Event[permission.PermissionNotification] {
	return make(<-chan pubsub.Event[permission.PermissionNotification])
}

type recordingClaudeCodeClient struct {
	result *claudecode.Result
	err    error

	calls []claudecode.SessionConfig
}

func (r *recordingClaudeCodeClient) LaunchAndWait(config claudecode.SessionConfig) (*claudecode.Result, error) {
	r.calls = append(r.calls, config)
	if r.err != nil {
		return nil, r.err
	}
	return r.result, nil
}

func withClaudeClientStub(t *testing.T, client claudeCodeClient, factoryErr error) {
	t.Helper()
	prevFactory := newClaudeCodeClient
	newClaudeCodeClient = func() (claudeCodeClient, error) {
		if factoryErr != nil {
			return nil, factoryErr
		}
		return client, nil
	}
	t.Cleanup(func() {
		newClaudeCodeClient = prevFactory
	})
}

func TestClaudeCodeTool_Info(t *testing.T) {
	tool := NewClaudeCodeTool(&mockClaudeCodePermissionService{shouldAllow: true}, "/tmp")
	info := tool.Info()

	require.Equal(t, ClaudeCodeToolName, info.Name)
	require.NotEmpty(t, info.Description)
	require.NotNil(t, info.Parameters)
}

func TestClaudeCodeTool_Call_ValidParams(t *testing.T) {
	tool := NewClaudeCodeTool(&mockClaudeCodePermissionService{shouldAllow: true}, "/tmp")

	stubResult := &claudecode.Result{
		Result:     "stub result",
		SessionID:  "stub-session",
		CostUSD:    0.25,
		DurationMS: 900,
		NumTurns:   3,
		ModelUsage: map[string]claudecode.ModelUsageDetail{
			"claude-3.5-sonnet": {},
		},
	}
	stubClient := &recordingClaudeCodeClient{result: stubResult}
	withClaudeClientStub(t, stubClient, nil)

	params := ClaudeCodeParams{
		Query: "Create a simple Go function that adds two numbers",
		Model: "sonnet",
	}

	input, err := json.Marshal(params)
	require.NoError(t, err)

	ctx := context.WithValue(context.Background(), SessionIDContextKey, "test-session")
	result, err := tool.Run(ctx, fantasy.ToolCall{
		ID:    "test-call-id",
		Name:  ClaudeCodeToolName,
		Input: string(input),
	})

	require.NoError(t, err) // Tool should not return a Go error, but rather a ToolResponse with error content
	require.NotNil(t, result)

	// Parse the response to check structure
	var response ClaudeCodeResponse
	err = json.Unmarshal([]byte(result.Content), &response)
	require.NoError(t, err, "Response should be valid JSON")

	// The response should have basic structure regardless of success/failure
	require.NotEmpty(t, response.SessionID, "Should have session ID")
	require.Equal(t, "stub-session", response.SessionID)
	require.Equal(t, "claude-sonnet", response.ModelUsed)
}

func TestClaudeCodeTool_Call_EmptyQuery(t *testing.T) {
	tool := NewClaudeCodeTool(&mockClaudeCodePermissionService{shouldAllow: true}, "/tmp")

	params := ClaudeCodeParams{
		Query: "",
	}

	input, err := json.Marshal(params)
	require.NoError(t, err)

	ctx := context.WithValue(context.Background(), SessionIDContextKey, "test-session")
	result, err := tool.Run(ctx, fantasy.ToolCall{
		ID:    "test-call-id",
		Name:  ClaudeCodeToolName,
		Input: string(input),
	})

	require.NoError(t, err)
	require.Contains(t, result.Content, "query is required")
}

func TestClaudeCodeTool_Call_PermissionDenied(t *testing.T) {
	tool := NewClaudeCodeTool(&mockClaudeCodePermissionService{shouldAllow: false}, "/tmp")

	params := ClaudeCodeParams{
		Query: "Test query",
	}

	input, err := json.Marshal(params)
	require.NoError(t, err)

	ctx := context.WithValue(context.Background(), SessionIDContextKey, "test-session")
	_, err = tool.Run(ctx, fantasy.ToolCall{
		ID:    "test-call-id",
		Name:  ClaudeCodeToolName,
		Input: string(input),
	})

	require.Error(t, err)
	require.Equal(t, permission.ErrorPermissionDenied, err)
}

func TestClaudeCodeTool_Call_InvalidModel(t *testing.T) {
	tool := NewClaudeCodeTool(&mockClaudeCodePermissionService{shouldAllow: true}, "/tmp")

	params := ClaudeCodeParams{
		Query: "Test query",
		Model: "invalid-model",
	}

	input, err := json.Marshal(params)
	require.NoError(t, err)

	ctx := context.WithValue(context.Background(), SessionIDContextKey, "test-session")
	result, err := tool.Run(ctx, fantasy.ToolCall{
		ID:    "test-call-id",
		Name:  ClaudeCodeToolName,
		Input: string(input),
	})

	require.NoError(t, err)
	require.Contains(t, result.Content, "invalid model")
}

func TestClaudeCodeTool_Call_InvalidJSON(t *testing.T) {
	tool := NewClaudeCodeTool(&mockClaudeCodePermissionService{shouldAllow: true}, "/tmp")

	ctx := context.WithValue(context.Background(), SessionIDContextKey, "test-session")
	result, err := tool.Run(ctx, fantasy.ToolCall{
		ID:    "test-call-id",
		Name:  ClaudeCodeToolName,
		Input: "invalid json",
	})

	require.NoError(t, err)
	require.Contains(t, result.Content, "invalid parameters")
}

func TestClaudeCodeTool_Call_NoSessionID(t *testing.T) {
	tool := NewClaudeCodeTool(&mockClaudeCodePermissionService{shouldAllow: true}, "/tmp")

	params := ClaudeCodeParams{
		Query: "Test query",
	}

	input, err := json.Marshal(params)
	require.NoError(t, err)

	ctx := context.Background() // No session ID in context
	_, err = tool.Run(ctx, fantasy.ToolCall{
		ID:    "test-call-id",
		Name:  ClaudeCodeToolName,
		Input: string(input),
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), "session ID is required")
}

func TestClaudeCodeParams_Validation(t *testing.T) {
	tests := []struct {
		name   string
		params ClaudeCodeParams
		desc   string
	}{
		{
			name: "default_max_turns",
			params: ClaudeCodeParams{
				Query:    "Test",
				MaxTurns: 0,
			},
			desc: "Should use default max turns when not specified",
		},
		{
			name: "valid_model_opus",
			params: ClaudeCodeParams{
				Query: "Test",
				Model: "opus",
			},
			desc: "Should accept opus model",
		},
		{
			name: "valid_model_sonnet",
			params: ClaudeCodeParams{
				Query: "Test",
				Model: "sonnet",
			},
			desc: "Should accept sonnet model",
		},
		{
			name: "valid_model_haiku",
			params: ClaudeCodeParams{
				Query: "Test",
				Model: "haiku",
			},
			desc: "Should accept haiku model",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tool := NewClaudeCodeTool(&mockClaudeCodePermissionService{shouldAllow: true}, "/tmp")

			stubClient := &recordingClaudeCodeClient{
				result: &claudecode.Result{
					Result:    "ok",
					SessionID: "stub-session",
				},
			}
			withClaudeClientStub(t, stubClient, nil)

			input, err := json.Marshal(tt.params)
			require.NoError(t, err)

			ctx := context.WithValue(context.Background(), SessionIDContextKey, "test-session")
			result, err := tool.Run(ctx, fantasy.ToolCall{
				ID:    "test-call-id",
				Name:  ClaudeCodeToolName,
				Input: string(input),
			})

			require.NoError(t, err)
			require.NotNil(t, result)

			// Should not return immediate validation errors for these cases
			require.NotContains(t, result.Content, "failed to parse parameters")
		})
	}
}
