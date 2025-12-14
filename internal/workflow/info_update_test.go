package workflow

import (
	"testing"
	"time"
)

// =============================================================================
// WorkflowInfo Tests
// =============================================================================

func TestWorkflowToInfo(t *testing.T) {
	now := time.Now()
	completedAt := now.Add(-time.Hour)

	workflow := Workflow{
		ID:               "wf-123",
		ParentSessionID:  "session-456",
		Title:            "Test Workflow",
		State:            WorkflowStateRunning,
		CurrentStepIndex: 2,
		CurrentNodeID:    "node-review",
		ErrorMessage:     "",
		CreatedAt:        now.Add(-2 * time.Hour),
		UpdatedAt:        now.Add(-time.Minute),
		CompletedAt:      nil,
		Steps: []Step{
			{ID: "step-1", Status: StepStatusCompleted},
			{ID: "step-2", Status: StepStatusCompleted},
			{ID: "step-3", Status: StepStatusRunning},
			{ID: "step-4", Status: StepStatusPending},
		},
	}

	info := WorkflowToInfo(workflow)

	if info.ID != workflow.ID {
		t.Errorf("expected ID %s, got %s", workflow.ID, info.ID)
	}
	if info.TotalSteps != 4 {
		t.Errorf("expected TotalSteps 4, got %d", info.TotalSteps)
	}
	if info.CompletedSteps != 2 {
		t.Errorf("expected CompletedSteps 2, got %d", info.CompletedSteps)
	}
	if info.Progress != 0.5 {
		t.Errorf("expected Progress 0.5, got %f", info.Progress)
	}
	if len(info.Steps) != 4 {
		t.Errorf("expected 4 steps in info, got %d", len(info.Steps))
	}

	// Test with completed workflow
	workflow.CompletedAt = &completedAt
	workflow.State = WorkflowStateCompleted
	info = WorkflowToInfo(workflow)

	if info.CompletedAt == nil {
		t.Error("expected CompletedAt to be set")
	}
}

func TestWorkflowToInfoEmptySteps(t *testing.T) {
	workflow := Workflow{
		ID:    "wf-empty",
		Title: "Empty Workflow",
		State: WorkflowStateDraft,
		Steps: []Step{},
	}

	info := WorkflowToInfo(workflow)

	if info.TotalSteps != 0 {
		t.Errorf("expected TotalSteps 0, got %d", info.TotalSteps)
	}
	if info.Progress != 0 {
		t.Errorf("expected Progress 0, got %f", info.Progress)
	}
	if info.Steps != nil {
		t.Error("expected Steps to be nil for empty workflow")
	}
}

func TestStepToInfo(t *testing.T) {
	now := time.Now()
	startedAt := now.Add(-time.Minute)

	step := Step{
		ID:               "step-123",
		WorkflowID:       "wf-456",
		StepIndex:        1,
		StepType:         StepTypeCode,
		Agent:            "claude-code",
		AgentSessionID:   "session-789",
		Status:           StepStatusRunning,
		Title:            "Coding Step",
		NodeID:           "node-code",
		RetryCount:       0,
		MaxRetries:       3,
		RequiresApproval: false,
		ApprovalStatus:   "",
		ErrorMessage:     "",
		CreatedAt:        now.Add(-2 * time.Minute),
		StartedAt:        &startedAt,
		CompletedAt:      nil,
	}

	info := StepToInfo(step)

	if info.ID != step.ID {
		t.Errorf("expected ID %s, got %s", step.ID, info.ID)
	}
	if info.StepType != StepTypeCode {
		t.Errorf("expected StepType code, got %s", info.StepType)
	}
	if info.Status != StepStatusRunning {
		t.Errorf("expected Status running, got %s", info.Status)
	}
	if info.StartedAt == nil {
		t.Error("expected StartedAt to be set")
	}
	if info.CompletedAt != nil {
		t.Error("expected CompletedAt to be nil")
	}
}

// =============================================================================
// WorkflowUpdate Tests
// =============================================================================

func TestWorkflowUpdateIsEmpty(t *testing.T) {
	tests := []struct {
		name     string
		update   WorkflowUpdate
		expected bool
	}{
		{
			name:     "empty update",
			update:   WorkflowUpdate{},
			expected: true,
		},
		{
			name:     "title set",
			update:   WorkflowUpdate{Title: strPtr("new title")},
			expected: false,
		},
		{
			name:     "state set",
			update:   WorkflowUpdate{State: workflowStatePtr(WorkflowStateRunning)},
			expected: false,
		},
		{
			name: "multiple fields set",
			update: WorkflowUpdate{
				Title:            strPtr("new title"),
				CurrentStepIndex: intPtr(5),
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.update.IsEmpty() != tt.expected {
				t.Errorf("expected IsEmpty() = %v, got %v", tt.expected, tt.update.IsEmpty())
			}
		})
	}
}

func TestApplyWorkflowUpdate(t *testing.T) {
	workflow := Workflow{
		ID:               "wf-123",
		Title:            "Original Title",
		State:            WorkflowStateDraft,
		CurrentStepIndex: 0,
	}

	update := WorkflowUpdate{
		Title:            strPtr("Updated Title"),
		State:            workflowStatePtr(WorkflowStateRunning),
		CurrentStepIndex: intPtr(3),
	}

	updated := ApplyWorkflowUpdate(workflow, update)

	if updated.Title != "Updated Title" {
		t.Errorf("expected Title 'Updated Title', got %s", updated.Title)
	}
	if updated.State != WorkflowStateRunning {
		t.Errorf("expected State running, got %s", updated.State)
	}
	if updated.CurrentStepIndex != 3 {
		t.Errorf("expected CurrentStepIndex 3, got %d", updated.CurrentStepIndex)
	}
	// Original should not be modified
	if workflow.Title != "Original Title" {
		t.Error("original workflow should not be modified")
	}
}

func TestApplyWorkflowUpdatePartial(t *testing.T) {
	workflow := Workflow{
		ID:               "wf-123",
		Title:            "Original Title",
		State:            WorkflowStateDraft,
		CurrentStepIndex: 5,
		ErrorMessage:     "previous error",
	}

	// Only update title, other fields should remain
	update := WorkflowUpdate{
		Title: strPtr("New Title"),
	}

	updated := ApplyWorkflowUpdate(workflow, update)

	if updated.Title != "New Title" {
		t.Errorf("expected Title 'New Title', got %s", updated.Title)
	}
	if updated.State != WorkflowStateDraft {
		t.Errorf("expected State draft (unchanged), got %s", updated.State)
	}
	if updated.CurrentStepIndex != 5 {
		t.Errorf("expected CurrentStepIndex 5 (unchanged), got %d", updated.CurrentStepIndex)
	}
	if updated.ErrorMessage != "previous error" {
		t.Errorf("expected ErrorMessage 'previous error' (unchanged), got %s", updated.ErrorMessage)
	}
}

// =============================================================================
// StepUpdate Tests
// =============================================================================

func TestStepUpdateIsEmpty(t *testing.T) {
	tests := []struct {
		name     string
		update   StepUpdate
		expected bool
	}{
		{
			name:     "empty update",
			update:   StepUpdate{},
			expected: true,
		},
		{
			name:     "status set",
			update:   StepUpdate{Status: stepStatusPtr(StepStatusCompleted)},
			expected: false,
		},
		{
			name:     "output json set",
			update:   StepUpdate{OutputJSON: strPtr("{}")},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.update.IsEmpty() != tt.expected {
				t.Errorf("expected IsEmpty() = %v, got %v", tt.expected, tt.update.IsEmpty())
			}
		})
	}
}

func TestApplyStepUpdate(t *testing.T) {
	now := time.Now()
	step := Step{
		ID:         "step-123",
		Status:     StepStatusPending,
		RetryCount: 0,
	}

	update := StepUpdate{
		Status:      stepStatusPtr(StepStatusCompleted),
		OutputJSON:  strPtr(`{"result": "success"}`),
		CompletedAt: &now,
	}

	updated := ApplyStepUpdate(step, update)

	if updated.Status != StepStatusCompleted {
		t.Errorf("expected Status completed, got %s", updated.Status)
	}
	if updated.OutputJSON != `{"result": "success"}` {
		t.Errorf("expected OutputJSON to be set, got %s", updated.OutputJSON)
	}
	if updated.CompletedAt == nil {
		t.Error("expected CompletedAt to be set")
	}
}

// =============================================================================
// Helper Functions
// =============================================================================

func strPtr(s string) *string {
	return &s
}

func intPtr(i int) *int {
	return &i
}

func workflowStatePtr(s WorkflowState) *WorkflowState {
	return &s
}

func stepStatusPtr(s StepStatus) *StepStatus {
	return &s
}
