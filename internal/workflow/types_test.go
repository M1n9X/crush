package workflow

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestWorkflowStates(t *testing.T) {
	states := []WorkflowState{
		WorkflowStateDraft,
		WorkflowStateRunning,
		WorkflowStateWaitingInput,
		WorkflowStatePaused,
		WorkflowStateCompleted,
		WorkflowStateFailed,
		WorkflowStateInterrupted,
	}

	require.Len(t, states, 7)
	require.Equal(t, "draft", string(WorkflowStateDraft))
	require.Equal(t, "running", string(WorkflowStateRunning))
	require.Equal(t, "waiting_input", string(WorkflowStateWaitingInput))
	require.Equal(t, "paused", string(WorkflowStatePaused))
	require.Equal(t, "completed", string(WorkflowStateCompleted))
	require.Equal(t, "failed", string(WorkflowStateFailed))
	require.Equal(t, "interrupted", string(WorkflowStateInterrupted))
}

func TestStepStatuses(t *testing.T) {
	statuses := []StepStatus{
		StepStatusPending,
		StepStatusRunning,
		StepStatusCompleted,
		StepStatusFailed,
		StepStatusSkipped,
		StepStatusWaiting,
	}

	require.Len(t, statuses, 6)
	require.Equal(t, "pending", string(StepStatusPending))
	require.Equal(t, "running", string(StepStatusRunning))
	require.Equal(t, "completed", string(StepStatusCompleted))
	require.Equal(t, "failed", string(StepStatusFailed))
	require.Equal(t, "skipped", string(StepStatusSkipped))
	require.Equal(t, "waiting", string(StepStatusWaiting))
}

func TestApprovalStatuses(t *testing.T) {
	statuses := []ApprovalStatus{
		ApprovalPending,
		ApprovalApproved,
		ApprovalRejected,
	}

	require.Len(t, statuses, 3)
	require.Equal(t, "pending", string(ApprovalPending))
	require.Equal(t, "approved", string(ApprovalApproved))
	require.Equal(t, "rejected", string(ApprovalRejected))
}

func TestStepTypes(t *testing.T) {
	types := []StepType{
		StepTypePlan,
		StepTypeCode,
		StepTypeReview,
		StepTypeDocs,
	}

	require.Len(t, types, 4)
	require.Equal(t, "plan", string(StepTypePlan))
	require.Equal(t, "code", string(StepTypeCode))
	require.Equal(t, "review", string(StepTypeReview))
	require.Equal(t, "docs", string(StepTypeDocs))
}

func TestDefaultWorkflowConfig(t *testing.T) {
	cfg := DefaultWorkflowConfig("Test Workflow", "Create a feature")

	require.Equal(t, "Test Workflow", cfg.Title)
	require.Equal(t, "Create a feature", cfg.Plan)
	require.Len(t, cfg.Steps, 6)

	// Verify step sequence
	require.Equal(t, StepTypePlan, cfg.Steps[0].StepType)
	require.Equal(t, "claude-code", cfg.Steps[0].Agent)
	require.False(t, cfg.Steps[0].RequiresApproval)

	require.Equal(t, StepTypeReview, cfg.Steps[1].StepType)
	require.Equal(t, "codex", cfg.Steps[1].Agent)
	require.True(t, cfg.Steps[1].RequiresApproval) // Plan review requires approval

	require.Equal(t, StepTypeCode, cfg.Steps[2].StepType)
	require.Equal(t, "claude-code", cfg.Steps[2].Agent)

	require.Equal(t, StepTypeReview, cfg.Steps[3].StepType)
	require.Equal(t, "codex", cfg.Steps[3].Agent)

	require.Equal(t, StepTypeReview, cfg.Steps[4].StepType)
	require.Equal(t, "codex", cfg.Steps[4].Agent)

	require.Equal(t, StepTypeDocs, cfg.Steps[5].StepType)
	require.Equal(t, "claude-code", cfg.Steps[5].Agent)
}

func TestMarshalUnmarshalConfig(t *testing.T) {
	cfg := WorkflowConfig{
		Title: "Test",
		Plan:  "Test plan",
		Steps: []StepConfig{
			{StepType: StepTypePlan, Agent: "test-agent", Title: "Plan", RequiresApproval: true, MaxRetries: 2},
		},
	}

	data, err := MarshalConfig(cfg)
	require.NoError(t, err)
	require.Contains(t, data, "test-agent")

	parsed, err := UnmarshalConfig(data)
	require.NoError(t, err)
	require.Equal(t, cfg.Title, parsed.Title)
	require.Equal(t, cfg.Plan, parsed.Plan)
	require.Len(t, parsed.Steps, 1)
	require.Equal(t, StepTypePlan, parsed.Steps[0].StepType)
	require.Equal(t, "test-agent", parsed.Steps[0].Agent)
	require.True(t, parsed.Steps[0].RequiresApproval)
	require.Equal(t, 2, parsed.Steps[0].MaxRetries)
}

func TestWorkflow(t *testing.T) {
	now := time.Now()
	wf := Workflow{
		ID:               "test-id",
		ParentSessionID:  "parent-123",
		Title:            "Test Workflow",
		State:            WorkflowStateRunning,
		CurrentStepIndex: 1,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	require.Equal(t, "test-id", wf.ID)
	require.Equal(t, "parent-123", wf.ParentSessionID)
	require.Equal(t, WorkflowStateRunning, wf.State)
	require.Equal(t, 1, wf.CurrentStepIndex)
}

func TestStep(t *testing.T) {
	now := time.Now()
	step := Step{
		ID:               "step-1",
		WorkflowID:       "wf-1",
		StepIndex:        0,
		StepType:         StepTypePlan,
		Agent:            "claude-code",
		Status:           StepStatusPending,
		Title:            "Planning",
		RequiresApproval: false,
		MaxRetries:       3,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	require.Equal(t, "step-1", step.ID)
	require.Equal(t, StepTypePlan, step.StepType)
	require.Equal(t, "claude-code", step.Agent)
	require.Equal(t, StepStatusPending, step.Status)
	require.False(t, step.RequiresApproval)
	require.Equal(t, 3, step.MaxRetries)
}
