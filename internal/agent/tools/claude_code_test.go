package tools

import (
	"context"
	"encoding/json"
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/pubsub"
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
func (m *mockClaudeCodePermissionService) Persistent() []permission.PermissionRequest { return nil }
func (m *mockClaudeCodePermissionService) ClearPersistent() error                     { return nil }

func TestClaudeCodeTool_Info(t *testing.T) {
	tool := NewClaudeCodeTool(&mockClaudeCodePermissionService{shouldAllow: true}, "/tmp")
	info := tool.Info()

	require.Equal(t, ClaudeCodeToolName, info.Name)
	require.NotEmpty(t, info.Description)
	require.NotNil(t, info.Parameters)
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
