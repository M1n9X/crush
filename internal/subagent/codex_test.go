package subagent

import (
	"context"
	"testing"

	codexsdk "github.com/M1n9X/codex-sdk-go"
	"github.com/stretchr/testify/require"
)

type fakeThread struct {
	id        string
	turn      *codexsdk.Turn
	runCalled bool
	stream    *codexsdk.StreamedTurn
}

func (f *fakeThread) ID() string { return f.id }

func (f *fakeThread) Run(ctx context.Context, input codexsdk.Input, opts ...codexsdk.TurnOption) (*codexsdk.Turn, error) {
	f.runCalled = true
	return f.turn, nil
}

func (f *fakeThread) RunStreamed(ctx context.Context, input codexsdk.Input, opts ...codexsdk.TurnOption) (*codexsdk.StreamedTurn, error) {
	return f.stream, nil
}

type fakeClient struct {
	thread     *fakeThread
	resume     *fakeThread
	startOpts  []codexsdk.ThreadOption
	resumeOpts []codexsdk.ThreadOption
	resumeID   string
	options    []codexsdk.Option
}

func (f *fakeClient) StartThread(opts ...codexsdk.ThreadOption) codexThread {
	f.startOpts = opts
	return f.thread
}

func (f *fakeClient) ResumeThread(id string, opts ...codexsdk.ThreadOption) codexThread {
	f.resumeID = id
	f.resumeOpts = opts
	if f.resume != nil {
		return f.resume
	}
	return f.thread
}

func TestCodexSubagentExecuteUsesThreadOptions(t *testing.T) {
	client := &fakeClient{
		thread: &fakeThread{
			id: "thread-123",
			turn: &codexsdk.Turn{
				FinalResponse: "done",
				Usage: &codexsdk.Usage{
					InputTokens:  10,
					OutputTokens: 5,
				},
			},
		},
	}

	sub := NewCodexSubagent(CodexOptions{
		DefaultModel:          "gpt-4",
		DefaultSandbox:        SandboxWorkspaceWrite,
		WorkingDirectory:      "/workspace",
		AdditionalDirectories: []string{"/tmp/shared"},
		DefaultApprovalMode:   codexsdk.ApprovalOnRequest,
		NetworkAccess:         boolPtr(true),
		WebSearch:             boolPtr(false),
		ModelReasoning:        codexsdk.ReasoningHigh,
		ClientFactory: func(opts ...codexsdk.Option) (codexClient, error) {
			client.options = opts
			return client, nil
		},
	})

	result, err := sub.Execute(context.Background(), Request{Task: "do it"})
	require.NoError(t, err)
	require.Equal(t, "done", result.Text)
	require.Equal(t, "thread-123", result.ResumeToken)
	require.Equal(t, int64(10), result.Usage.PromptTokens)
	require.Equal(t, int64(5), result.Usage.CompletionTokens)
	require.Equal(t, int64(15), result.Usage.TotalTokens)
	require.True(t, client.thread.runCalled)

	threadOpts := codexsdk.ThreadOptions{}
	for _, opt := range client.startOpts {
		opt(&threadOpts)
	}
	require.Equal(t, codexsdk.SandboxWorkspaceWrite, threadOpts.SandboxMode)
	require.Equal(t, "/workspace", threadOpts.WorkingDirectory)
	require.Equal(t, "gpt-4", threadOpts.Model)
	require.Equal(t, codexsdk.ApprovalOnRequest, threadOpts.ApprovalPolicy)
	require.Equal(t, codexsdk.ReasoningHigh, threadOpts.ModelReasoningEffort)
	require.NotNil(t, threadOpts.NetworkAccessEnabled)
	require.True(t, *threadOpts.NetworkAccessEnabled)
	require.NotNil(t, threadOpts.WebSearchEnabled)
	require.False(t, *threadOpts.WebSearchEnabled)
	require.Equal(t, []string{"/tmp/shared"}, threadOpts.AdditionalDirectories)
}

func TestCodexSubagentResumeUsesResumeToken(t *testing.T) {
	client := &fakeClient{
		resume: &fakeThread{
			id: "resume-thread",
			turn: &codexsdk.Turn{
				FinalResponse: "resume",
			},
			stream: &codexsdk.StreamedTurn{
				Events: make(chan codexsdk.ThreadEvent),
			},
		},
	}

	sub := NewCodexSubagent(CodexOptions{
		DefaultModel: "gpt-4",
		ClientFactory: func(opts ...codexsdk.Option) (codexClient, error) {
			return client, nil
		},
	})

	result, err := sub.Resume(context.Background(), "resume-thread", Request{Task: "continue"})
	require.NoError(t, err)
	require.Equal(t, "resume", result.Text)
	require.Equal(t, "resume-thread", client.resumeID)

	threadOpts := codexsdk.ThreadOptions{}
	for _, opt := range client.resumeOpts {
		opt(&threadOpts)
	}
	require.Equal(t, codexsdk.SandboxReadOnly, threadOpts.SandboxMode)
}

func TestCodexSubagentExecuteStreamedEmitsHandler(t *testing.T) {
	evCh := make(chan codexsdk.ThreadEvent, 2)
	evCh <- codexsdk.ThreadEvent{Type: codexsdk.EventItemCompleted, Item: &codexsdk.AgentMessageItem{Text: "hi"}}
	evCh <- codexsdk.ThreadEvent{Type: codexsdk.EventTurnCompleted, Usage: &codexsdk.Usage{InputTokens: 1, OutputTokens: 2}}
	close(evCh)

	client := &fakeClient{
		thread: &fakeThread{
			id: "stream-thread",
			stream: &codexsdk.StreamedTurn{
				Events: evCh,
			},
		},
	}

	sub := NewCodexSubagent(CodexOptions{
		DefaultModel: "gpt-4",
		ClientFactory: func(opts ...codexsdk.Option) (codexClient, error) {
			return client, nil
		},
	})

	var got []Event
	result, err := sub.ExecuteStreamed(context.Background(), Request{Task: "hello"}, func(e Event) { got = append(got, e) })
	require.NoError(t, err)
	require.Equal(t, "stream-thread", result.ResumeToken)
	require.Equal(t, int64(3), result.Usage.TotalTokens)
	require.Len(t, got, 2)
}

func boolPtr(v bool) *bool {
	return &v
}
