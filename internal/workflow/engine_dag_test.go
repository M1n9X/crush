package workflow

import (
	"context"
	"testing"

	"github.com/charmbracelet/crush/internal/db"
	"github.com/charmbracelet/crush/internal/subagent"
	"github.com/stretchr/testify/require"
)

func TestDAGOnRejectTransitionsBackToNode(t *testing.T) {
	ctx := context.Background()

	conn, err := db.Connect(ctx, t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	q := db.New(conn)

	planAgent := &fakeSubagent{
		name:         "plan-agent",
		capabilities: []subagent.Capability{"plan"},
		result:       &subagent.Result{Text: "plan-ok"},
	}
	reviewAgent := &fakeSubagent{
		name:         "review-agent",
		capabilities: []subagent.Capability{"review"},
		result:       &subagent.Result{Text: "review-ok"},
	}

	registry := subagent.NewRegistry(nil)
	registry.Upsert(subagent.Registration{Profile: subagent.Profile{Name: "plan-agent"}, Agent: planAgent})
	registry.Upsert(subagent.Registration{Profile: subagent.Profile{Name: "review-agent"}, Agent: reviewAgent})

	engine := NewEngine(q, registry)

	spec := &WorkflowSpec{
		Name: "reject-loop",
		Nodes: []NodeSpec{
			{ID: "plan", Name: "Plan", Capabilities: []string{"plan"}, PreferredAgents: []string{"plan-agent"}, Next: []string{"review"}},
			{ID: "review", Name: "Review", Capabilities: []string{"review"}, PreferredAgents: []string{"review-agent"}, RequiresApproval: true, OnApprove: []string{"complete"}, OnReject: []string{"plan"}},
			{ID: "complete", Terminal: "success"},
		},
	}

	wf, err := engine.CreateFromSpec(ctx, spec, CreateOptions{Title: "DAG Reject", ParentSessionID: ""})
	require.NoError(t, err)

	require.ErrorIs(t, engine.Run(ctx, wf.ID), ErrApprovalRequired)

	steps, err := engine.GetSteps(ctx, wf.ID)
	require.NoError(t, err)

	var reviewStep Step
	for _, s := range steps {
		if s.NodeID == "review" {
			reviewStep = s
		}
	}
	require.NotEmpty(t, reviewStep.ID)
	require.Equal(t, StepStatusWaiting, reviewStep.Status)
	require.Equal(t, ApprovalPending, reviewStep.ApprovalStatus)
	require.Equal(t, 1, planAgent.executeCalls)
	require.Equal(t, 0, reviewAgent.executeCalls)

	require.NoError(t, engine.Approve(ctx, reviewStep.ID, false))

	require.ErrorIs(t, engine.Run(ctx, wf.ID), ErrApprovalRequired)
	require.Equal(t, 2, planAgent.executeCalls)
	require.Equal(t, 0, reviewAgent.executeCalls)

	steps, err = engine.GetSteps(ctx, wf.ID)
	require.NoError(t, err)
	for _, s := range steps {
		if s.NodeID == "review" {
			require.Equal(t, StepStatusWaiting, s.Status)
			require.Equal(t, ApprovalPending, s.ApprovalStatus)
		}
	}

	require.NoError(t, engine.Approve(ctx, reviewStep.ID, true))
	require.NoError(t, engine.Run(ctx, wf.ID))

	updated, err := engine.GetWorkflow(ctx, wf.ID)
	require.NoError(t, err)
	require.Equal(t, WorkflowStateCompleted, updated.State)
	require.Equal(t, 2, planAgent.executeCalls)
	require.Equal(t, 1, reviewAgent.executeCalls)
}

func TestDAGOnFailTransitionsToRecoveryNode(t *testing.T) {
	ctx := context.Background()

	conn, err := db.Connect(ctx, t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	q := db.New(conn)

	badAgent := &fakeSubagent{
		name:         "bad-agent",
		capabilities: []subagent.Capability{"code"},
		err:          assertErr("boom"),
	}
	recoverAgent := &fakeSubagent{
		name:         "recover-agent",
		capabilities: []subagent.Capability{"docs"},
		result:       &subagent.Result{Text: "recovered"},
	}

	registry := subagent.NewRegistry(nil)
	registry.Upsert(subagent.Registration{Profile: subagent.Profile{Name: "bad-agent"}, Agent: badAgent})
	registry.Upsert(subagent.Registration{Profile: subagent.Profile{Name: "recover-agent"}, Agent: recoverAgent})

	engine := NewEngine(q, registry)

	spec := &WorkflowSpec{
		Name: "fail-recover",
		Nodes: []NodeSpec{
			{ID: "code", Name: "Code", Capabilities: []string{"code"}, PreferredAgents: []string{"bad-agent"}, OnFail: []string{"recover"}},
			{ID: "recover", Name: "Recover", Capabilities: []string{"docs"}, PreferredAgents: []string{"recover-agent"}, Next: []string{"complete"}},
			{ID: "complete", Terminal: "success"},
		},
	}

	wf, err := engine.CreateFromSpec(ctx, spec, CreateOptions{Title: "DAG Fail", ParentSessionID: ""})
	require.NoError(t, err)

	require.NoError(t, engine.Run(ctx, wf.ID))

	updated, err := engine.GetWorkflow(ctx, wf.ID)
	require.NoError(t, err)
	require.Equal(t, WorkflowStateCompleted, updated.State)
	require.Equal(t, 2, badAgent.executeCalls)      // MaxRetries defaults to 1 => 2 attempts
	require.Equal(t, 1, recoverAgent.executeCalls)  // Recovery runs once

	steps, err := engine.GetSteps(ctx, wf.ID)
	require.NoError(t, err)
	for _, s := range steps {
		if s.NodeID == "code" {
			require.Equal(t, StepStatusFailed, s.Status)
		}
		if s.NodeID == "recover" {
			require.Equal(t, StepStatusCompleted, s.Status)
		}
	}
}

type assertErr string

func (e assertErr) Error() string { return string(e) }

