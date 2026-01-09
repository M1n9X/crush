package subagent

import (
	"context"
	"errors"
	"fmt"
	"strings"

	codexsdk "github.com/M1n9X/codex-sdk-go"
)

type codexClient interface {
	StartThread(opts ...codexsdk.ThreadOption) codexThread
	ResumeThread(id string, opts ...codexsdk.ThreadOption) codexThread
}

type codexThread interface {
	ID() string
	Run(ctx context.Context, input codexsdk.Input, opts ...codexsdk.TurnOption) (*codexsdk.Turn, error)
	RunStreamed(ctx context.Context, input codexsdk.Input, opts ...codexsdk.TurnOption) (*codexsdk.StreamedTurn, error)
}

type codexFactory func(opts ...codexsdk.Option) (codexClient, error)

// CodexOptions controls how the CodexSubagent is constructed.
type CodexOptions struct {
	CodexPath             string
	BaseURL               string
	APIKey                string
	Env                   map[string]string
	WorkingDirectory      string
	AdditionalDirectories []string
	SkipGitRepoCheck      bool
	DefaultModel          string
	DefaultSandbox        SandboxMode
	DefaultApprovalMode   codexsdk.ApprovalMode
	NetworkAccess         *bool
	WebSearch             *bool
	ModelReasoning        codexsdk.ModelReasoningEffort
	ClientFactory         codexFactory
}

// CodexSubagent is a Subagent implementation backed by codex-sdk-go.
type CodexSubagent struct {
	opts    CodexOptions
	factory codexFactory
}

// NewCodexSubagent builds a Codex-backed subagent.
func NewCodexSubagent(opts CodexOptions) *CodexSubagent {
	factory := opts.ClientFactory
	if factory == nil {
		factory = defaultCodexFactory
	}
	if opts.DefaultSandbox == "" {
		opts.DefaultSandbox = SandboxReadOnly
	}
	return &CodexSubagent{
		opts:    opts,
		factory: factory,
	}
}

func (c *CodexSubagent) Name() string {
	return "codex"
}

func (c *CodexSubagent) Capabilities() []Capability {
	return []Capability{"plan", "code", "review", "docs", "brainstorm"}
}

func (c *CodexSubagent) SupportsResume() bool {
	return true
}

func (c *CodexSubagent) Execute(ctx context.Context, req Request) (*Result, error) {
	return c.run(ctx, "", req)
}

func (c *CodexSubagent) Resume(ctx context.Context, resumeToken string, req Request) (*Result, error) {
	if strings.TrimSpace(resumeToken) == "" {
		return nil, fmt.Errorf("resume token required for codex subagent")
	}
	return c.run(ctx, resumeToken, req)
}

// ExecuteStreamed streams Codex events to the provided handler while running the turn.
func (c *CodexSubagent) ExecuteStreamed(ctx context.Context, req Request, handler func(Event)) (*Result, error) {
	if req.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, req.Timeout)
		defer cancel()
	}

	task := strings.TrimSpace(req.Task)
	if task == "" {
		return nil, fmt.Errorf("task is required for codex subagent")
	}

	client, err := c.factory(c.clientOptions()...)
	if err != nil {
		// Handle structured errors from Codex SDK
		var invalidInputErr *codexsdk.ErrInvalidInput
		if errors.As(err, &invalidInputErr) {
			return nil, fmt.Errorf("invalid codex input for %s: %s", invalidInputErr.Field, invalidInputErr.Reason)
		}
		return nil, fmt.Errorf("build codex client: %w", err)
	}

	thread := client.StartThread(c.threadOptions(req)...)
	streamed, err := thread.RunStreamed(ctx, codexsdk.Text(task))
	if err != nil {
		// Handle execution errors
		var execErr *codexsdk.ErrExecFailed
		if errors.As(err, &execErr) {
			return nil, fmt.Errorf("codex execution failed (exit %d): %s", execErr.ExitCode, execErr.Stderr)
		}
		return nil, err
	}

	var finalResponse string
	var usage *codexsdk.Usage
	for event := range streamed.Events {
		if handler != nil {
			handler(Event{Type: string(event.Type), Payload: event, Provider: "codex"})
		}
		switch event.Type {
		case codexsdk.EventItemCompleted:
			if msg, ok := event.Item.(*codexsdk.AgentMessageItem); ok {
				finalResponse = msg.Text
			}
		case codexsdk.EventTurnCompleted:
			usage = event.Usage
		}
	}

	if err := streamed.Wait(); err != nil {
		return nil, err
	}

	return &Result{
		Text:        finalResponse,
		ResumeToken: thread.ID(),
		Usage:       mapUsage(usage, c.effectiveModel(req)),
	}, nil
}

func (c *CodexSubagent) run(ctx context.Context, resumeToken string, req Request) (*Result, error) {
	if req.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, req.Timeout)
		defer cancel()
	}

	task := strings.TrimSpace(req.Task)
	if task == "" {
		return nil, fmt.Errorf("task is required for codex subagent")
	}

	client, err := c.factory(c.clientOptions()...)
	if err != nil {
		// Handle structured errors from Codex SDK
		var invalidInputErr *codexsdk.ErrInvalidInput
		if errors.As(err, &invalidInputErr) {
			return nil, fmt.Errorf("invalid codex input for %s: %s", invalidInputErr.Field, invalidInputErr.Reason)
		}
		return nil, fmt.Errorf("build codex client: %w", err)
	}

	threadOpts := c.threadOptions(req)
	var thread codexThread
	if resumeToken != "" {
		thread = client.ResumeThread(resumeToken, threadOpts...)
	} else {
		thread = client.StartThread(threadOpts...)
	}

	turn, err := thread.Run(ctx, codexsdk.Text(task))
	if err != nil {
		// Handle execution errors
		var execErr *codexsdk.ErrExecFailed
		if errors.As(err, &execErr) {
			return nil, fmt.Errorf("codex execution failed (exit %d): %s", execErr.ExitCode, execErr.Stderr)
		}
		return nil, err
	}

	return &Result{
		Text:        turn.FinalResponse,
		ResumeToken: thread.ID(),
		Usage:       mapUsage(turn.Usage, c.effectiveModel(req)),
	}, nil
}

func (c *CodexSubagent) clientOptions() []codexsdk.Option {
	var opts []codexsdk.Option
	if c.opts.CodexPath != "" {
		opts = append(opts, codexsdk.WithCodexPath(c.opts.CodexPath))
	}
	if c.opts.BaseURL != "" {
		opts = append(opts, codexsdk.WithBaseURL(c.opts.BaseURL))
	}
	if c.opts.APIKey != "" {
		opts = append(opts, codexsdk.WithAPIKey(c.opts.APIKey))
	}
	if c.opts.Env != nil {
		opts = append(opts, codexsdk.WithEnv(c.opts.Env))
	}
	return opts
}

func (c *CodexSubagent) threadOptions(req Request) []codexsdk.ThreadOption {
	var opts []codexsdk.ThreadOption
	if model := c.effectiveModel(req); model != "" {
		opts = append(opts, codexsdk.WithModel(model))
	}
	opts = append(opts, codexsdk.WithSandboxMode(mapSandbox(req.Sandbox, c.opts.DefaultSandbox)))

	workingDir := c.opts.WorkingDirectory
	if req.Metadata != nil {
		if wd := strings.TrimSpace(req.Metadata["working_dir"]); wd != "" {
			workingDir = wd
		}
	}
	if workingDir != "" {
		opts = append(opts, codexsdk.WithWorkingDirectory(workingDir))
	}
	if c.opts.SkipGitRepoCheck {
		opts = append(opts, codexsdk.WithSkipGitRepoCheck())
	}
	if c.opts.ModelReasoning != "" {
		opts = append(opts, codexsdk.WithModelReasoningEffort(c.opts.ModelReasoning))
	}
	if c.opts.NetworkAccess != nil {
		opts = append(opts, codexsdk.WithNetworkAccess(*c.opts.NetworkAccess))
	}
	if c.opts.WebSearch != nil {
		opts = append(opts, codexsdk.WithWebSearch(*c.opts.WebSearch))
	}
	if c.opts.DefaultApprovalMode != "" {
		opts = append(opts, codexsdk.WithApprovalPolicy(c.opts.DefaultApprovalMode))
	}
	if len(c.opts.AdditionalDirectories) > 0 {
		opts = append(opts, codexsdk.WithAdditionalDirectories(c.opts.AdditionalDirectories...))
	}
	return opts
}

func (c *CodexSubagent) effectiveModel(req Request) string {
	if req.ModelName != "" {
		return req.ModelName
	}
	if req.Profile != nil && req.Profile.ModelName != "" {
		return req.Profile.ModelName
	}
	return c.opts.DefaultModel
}

func defaultCodexFactory(opts ...codexsdk.Option) (codexClient, error) {
	client, err := codexsdk.New(opts...)
	if err != nil {
		return nil, err
	}
	return &sdkCodexClient{client: client}, nil
}

type sdkCodexClient struct {
	client *codexsdk.Codex
}

func (c *sdkCodexClient) StartThread(opts ...codexsdk.ThreadOption) codexThread {
	return &sdkThread{thread: c.client.StartThread(opts...)}
}

func (c *sdkCodexClient) ResumeThread(id string, opts ...codexsdk.ThreadOption) codexThread {
	return &sdkThread{thread: c.client.ResumeThread(id, opts...)}
}

type sdkThread struct {
	thread *codexsdk.Thread
}

func (t *sdkThread) ID() string {
	return t.thread.ID()
}

func (t *sdkThread) Run(ctx context.Context, input codexsdk.Input, opts ...codexsdk.TurnOption) (*codexsdk.Turn, error) {
	return t.thread.Run(ctx, input, opts...)
}

func (t *sdkThread) RunStreamed(ctx context.Context, input codexsdk.Input, opts ...codexsdk.TurnOption) (*codexsdk.StreamedTurn, error) {
	return t.thread.RunStreamed(ctx, input, opts...)
}

func mapSandbox(req SandboxMode, fallback SandboxMode) codexsdk.SandboxMode {
	mode := req
	if mode == "" {
		mode = fallback
	}
	switch mode {
	case SandboxDangerFullAccess:
		return codexsdk.SandboxDangerFullAccess
	case SandboxWorkspaceWrite:
		return codexsdk.SandboxWorkspaceWrite
	default:
		return codexsdk.SandboxReadOnly
	}
}

func mapUsage(usage *codexsdk.Usage, model string) Usage {
	if usage == nil {
		return Usage{}
	}
	total := usage.InputTokens + usage.OutputTokens
	return Usage{
		PromptTokens:     int64(usage.InputTokens),
		CompletionTokens: int64(usage.OutputTokens),
		TotalTokens:      int64(total),
		ModelName:        model,
	}
}
