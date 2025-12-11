package agent

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/M1n9X/claude-agent-sdk-go/types"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/subagent"
	"github.com/stretchr/testify/require"
)

func TestClaudeSubagentRequiresSession(t *testing.T) {
	agent := newClaudeCodeSubagent(&coordinator{cfg: &config.Config{}})
	_, err := agent.Execute(context.Background(), subagent.Request{
		Task:      "do work",
		Profile:   &subagent.Profile{Name: "general"},
		SessionID: "",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "session id is required")
}

func TestClaudeSubagentExecuteStreamed(t *testing.T) {
	cfg := &config.Config{
		Agents: map[string]config.Agent{
			config.AgentCoder: {
				AllowedTools: []string{"write", "edit"},
				Model:        config.SelectedModelTypeLarge,
			},
		},
		Models: map[config.SelectedModelType]config.SelectedModel{
			config.SelectedModelTypeLarge: {
				Model: "claude-test",
			},
		},
	}

	messages := make(chan types.Message, 2)
	messages <- &types.AssistantMessage{
		Type: "assistant_message",
		Content: []types.ContentBlock{
			&types.TextBlock{Type: "text", Text: "hello there"},
		},
	}
	cost := 1.25
	messages <- &types.ResultMessage{
		Type:         "result",
		SessionID:    "session-123",
		TotalCostUSD: &cost,
		Usage: map[string]interface{}{
			"input_tokens":  float64(10),
			"output_tokens": float64(4),
		},
	}
	close(messages)

	fake := &fakeClaudeClient{responses: messages}
	coord := &coordinator{cfg: cfg}
	sub := newClaudeCodeSubagent(coord)

	var capturedOpts *types.ClaudeAgentOptions
	sub.clientFactory = func(ctx context.Context, opts *types.ClaudeAgentOptions) (claudeClient, error) {
		capturedOpts = opts
		return fake, nil
	}

	var events []subagent.Event
	result, err := sub.ExecuteStreamed(context.Background(), subagent.Request{
		Task:      "do work",
		Profile:   &subagent.Profile{Name: "general"},
		SessionID: "parent-session",
	}, func(e subagent.Event) { events = append(events, e) })
	require.NoError(t, err)

	require.True(t, fake.connectCalled)
	require.True(t, fake.closeCalled)
	require.Contains(t, fake.queryPrompt, "do work")
	require.Equal(t, "hello there", result.Text)
	require.Equal(t, "session-123", result.ResumeToken)
	require.EqualValues(t, 10, result.Usage.PromptTokens)
	require.EqualValues(t, 4, result.Usage.CompletionTokens)
	require.EqualValues(t, 14, result.Usage.TotalTokens)
	require.InDelta(t, 1.25, result.Usage.CostUSD, 0.0001)

	require.Len(t, events, 2)
	require.Equal(t, "assistant_message", events[0].Type)
	require.Equal(t, "result", events[1].Type)

	require.NotNil(t, capturedOpts)
	require.True(t, capturedOpts.IncludePartialMessages)
	require.NotNil(t, capturedOpts.PermissionMode)
	require.Equal(t, types.PermissionModeAcceptEdits, *capturedOpts.PermissionMode)
	require.ElementsMatch(t, []string{"write", "edit"}, capturedOpts.AllowedTools)
	require.NotNil(t, capturedOpts.Model)
	require.Equal(t, "claude-test", *capturedOpts.Model)
}

func TestBuildSubagentPrompt(t *testing.T) {
	t.Run("task only", func(t *testing.T) {
		req := subagent.Request{Task: "do the thing"}
		result := buildSubagentPrompt(req)
		require.Equal(t, "do the thing", result)
	})

	t.Run("with profile description", func(t *testing.T) {
		req := subagent.Request{
			Task:    "do the thing",
			Profile: &subagent.Profile{Description: "You are a coder."},
		}
		result := buildSubagentPrompt(req)
		require.Contains(t, result, "You are a coder.")
		require.Contains(t, result, "Task: do the thing")
	})

	t.Run("with request description", func(t *testing.T) {
		req := subagent.Request{
			Task:        "do the thing",
			Description: "Extra context here",
		}
		result := buildSubagentPrompt(req)
		require.Contains(t, result, "Extra context here")
		require.Contains(t, result, "Task: do the thing")
	})

	t.Run("with ContextJSON", func(t *testing.T) {
		req := subagent.Request{
			Task:        "review this code",
			ContextJSON: json.RawMessage(`{"files":["a.go","b.go"],"diff":"+ some changes"}`),
		}
		result := buildSubagentPrompt(req)
		require.Contains(t, result, "review this code")
		require.Contains(t, result, "Context:")
		require.Contains(t, result, `"files"`)
		require.Contains(t, result, `"a.go"`)
		require.Contains(t, result, `"diff"`)
	})

	t.Run("with profile description and ContextJSON", func(t *testing.T) {
		req := subagent.Request{
			Task:        "implement feature",
			Profile:     &subagent.Profile{Description: "You are a coder."},
			Description: "High priority",
			ContextJSON: json.RawMessage(`{"plan":"step 1, step 2"}`),
		}
		result := buildSubagentPrompt(req)
		require.Contains(t, result, "You are a coder.")
		require.Contains(t, result, "High priority")
		require.Contains(t, result, "Task: implement feature")
		require.Contains(t, result, "Context:")
		require.Contains(t, result, `"plan"`)
	})

	t.Run("ContextJSON pretty-printed", func(t *testing.T) {
		req := subagent.Request{
			Task:        "review",
			ContextJSON: json.RawMessage(`{"key":"value"}`),
		}
		result := buildSubagentPrompt(req)
		// Should have newlines from pretty-printing
		require.Contains(t, result, "{\n  \"key\": \"value\"\n}")
	})

	t.Run("invalid ContextJSON falls back to raw", func(t *testing.T) {
		req := subagent.Request{
			Task:        "review",
			ContextJSON: json.RawMessage(`not valid json at all`),
		}
		result := buildSubagentPrompt(req)
		// Should still include the raw context since pretty-print fails
		require.Contains(t, result, "Context:")
		require.Contains(t, result, "not valid json at all")
	})
}

type fakeClaudeClient struct {
	connectCalled bool
	queryPrompt   string
	closeCalled   bool
	responses     <-chan types.Message
}

func (f *fakeClaudeClient) Connect(ctx context.Context) error {
	f.connectCalled = true
	return nil
}

func (f *fakeClaudeClient) Query(ctx context.Context, prompt string) error {
	f.queryPrompt = prompt
	return nil
}

func (f *fakeClaudeClient) ReceiveResponse(ctx context.Context) <-chan types.Message {
	return f.responses
}

func (f *fakeClaudeClient) Close(ctx context.Context) error {
	f.closeCalled = true
	return nil
}
