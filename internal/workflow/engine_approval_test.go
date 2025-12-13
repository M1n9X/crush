package workflow

import (
	"context"
	"testing"

	"github.com/charmbracelet/crush/internal/db"
	"github.com/charmbracelet/crush/internal/subagent"
	"github.com/stretchr/testify/require"
)

func TestWaitingWorkflowCanResumeAfterApproval(t *testing.T) {
	ctx := context.Background()

	conn, err := db.Connect(ctx, t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	q := db.New(conn)

	registry := subagent.NewRegistry(nil)
	registry.Upsert(subagent.Registration{
		Profile: subagent.Profile{Name: "test-agent"},
		Agent: &fakeSubagent{
			name: "test-agent",
			result: &subagent.Result{
				Text: "ok",
			},
		},
	})

	engine := NewEngine(q, registry)

	wf, err := engine.Create(ctx, CreateOptions{
		Title:           "Approval Workflow",
		ParentSessionID: "",
		Config: WorkflowConfig{
			Title: "Approval Workflow",
			Steps: []StepConfig{
				{
					StepType:         StepTypeReview,
					Agent:            "test-agent",
					Title:            "Needs approval",
					RequiresApproval: true,
					MaxRetries:       2,
				},
			},
		},
	})
	require.NoError(t, err)

	require.ErrorIs(t, engine.Run(ctx, wf.ID), ErrApprovalRequired)

	updated, err := engine.GetWorkflow(ctx, wf.ID)
	require.NoError(t, err)
	require.Equal(t, WorkflowStateWaitingInput, updated.State)

	steps, err := engine.GetSteps(ctx, wf.ID)
	require.NoError(t, err)
	require.Len(t, steps, 1)
	require.Equal(t, StepStatusWaiting, steps[0].Status)
	require.Equal(t, ApprovalPending, steps[0].ApprovalStatus)

	require.NoError(t, engine.Approve(ctx, steps[0].ID, true))

	require.NoError(t, engine.Run(ctx, wf.ID))

	updated, err = engine.GetWorkflow(ctx, wf.ID)
	require.NoError(t, err)
	require.Equal(t, WorkflowStateCompleted, updated.State)

	steps, err = engine.GetSteps(ctx, wf.ID)
	require.NoError(t, err)
	require.Len(t, steps, 1)
	require.Equal(t, StepStatusCompleted, steps[0].Status)
	require.Equal(t, ApprovalApproved, steps[0].ApprovalStatus)
	require.Equal(t, "ok", steps[0].OutputJSON)
}

