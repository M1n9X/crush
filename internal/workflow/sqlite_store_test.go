package workflow

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/db"
	"github.com/google/uuid"
)

// setupTestStore creates a test SQLite database with migrations applied.
func setupTestStore(t *testing.T) (*SQLiteStore, func()) {
	t.Helper()

	ctx := context.Background()
	database, err := db.Connect(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}

	queries := db.New(database)
	store := NewSQLiteStore(queries, database)

	cleanup := func() {
		database.Close()
	}

	return store, cleanup
}

// =============================================================================
// Workflow CRUD Tests
// =============================================================================

func TestSQLiteStore_CreateWorkflow(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	workflow := &Workflow{
		ID:         uuid.New().String(),
		Title:      "Test Workflow",
		State:      WorkflowStateDraft,
		PlanJSON:   `{"plan": "test"}`,
		ConfigJSON: `{"config": true}`,
	}

	err := store.CreateWorkflow(ctx, workflow)
	if err != nil {
		t.Fatalf("failed to create workflow: %v", err)
	}

	// Verify workflow was created with timestamps
	if workflow.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}
	if workflow.UpdatedAt.IsZero() {
		t.Error("expected UpdatedAt to be set")
	}
}

func TestSQLiteStore_CreateWorkflowWithSpec(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	workflow := &Workflow{
		ID:            uuid.New().String(),
		Title:         "DAG Workflow",
		State:         WorkflowStateDraft,
		SpecJSON:      `{"nodes":[]}`,
		CurrentNodeID: "start",
	}

	if err := store.CreateWorkflow(ctx, workflow); err != nil {
		t.Fatalf("failed to create workflow: %v", err)
	}

	if workflow.SpecJSON != `{"nodes":[]}` {
		t.Fatalf("expected SpecJSON to be persisted, got %q", workflow.SpecJSON)
	}
	if workflow.CurrentNodeID != "start" {
		t.Fatalf("expected CurrentNodeID to be persisted, got %q", workflow.CurrentNodeID)
	}
}

func TestSQLiteStore_GetWorkflow(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	id := uuid.New().String()
	original := &Workflow{
		ID:    id,
		Title: "Test Workflow",
		State: WorkflowStateDraft,
	}

	if err := store.CreateWorkflow(ctx, original); err != nil {
		t.Fatalf("failed to create workflow: %v", err)
	}

	retrieved, err := store.GetWorkflow(ctx, id)
	if err != nil {
		t.Fatalf("failed to get workflow: %v", err)
	}

	if retrieved.ID != id {
		t.Errorf("expected ID %s, got %s", id, retrieved.ID)
	}
	if retrieved.Title != "Test Workflow" {
		t.Errorf("expected Title 'Test Workflow', got %s", retrieved.Title)
	}
}

func TestSQLiteStore_GetWorkflowNotFound(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	_, err := store.GetWorkflow(ctx, "nonexistent-id")
	if err == nil {
		t.Error("expected error for nonexistent workflow")
	}
}

func TestSQLiteStore_UpdateWorkflow(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	id := uuid.New().String()
	workflow := &Workflow{
		ID:    id,
		Title: "Original Title",
		State: WorkflowStateDraft,
	}

	if err := store.CreateWorkflow(ctx, workflow); err != nil {
		t.Fatalf("failed to create workflow: %v", err)
	}

	// Update with partial fields
	newState := WorkflowStateRunning
	newTitle := "Updated Title"
	err := store.UpdateWorkflow(ctx, id, WorkflowUpdate{
		Title: &newTitle,
		State: &newState,
	})
	if err != nil {
		t.Fatalf("failed to update workflow: %v", err)
	}

	// Verify updates
	updated, err := store.GetWorkflow(ctx, id)
	if err != nil {
		t.Fatalf("failed to get updated workflow: %v", err)
	}

	if updated.Title != "Updated Title" {
		t.Errorf("expected Title 'Updated Title', got %s", updated.Title)
	}
	if updated.State != WorkflowStateRunning {
		t.Errorf("expected State running, got %s", updated.State)
	}
}

func TestSQLiteStore_UpdateWorkflowEmpty(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	id := uuid.New().String()
	workflow := &Workflow{
		ID:    id,
		Title: "Title",
		State: WorkflowStateDraft,
	}

	if err := store.CreateWorkflow(ctx, workflow); err != nil {
		t.Fatalf("failed to create workflow: %v", err)
	}

	// Empty update should be a no-op
	err := store.UpdateWorkflow(ctx, id, WorkflowUpdate{})
	if err != nil {
		t.Fatalf("empty update should succeed: %v", err)
	}
}

func TestSQLiteStore_UpdateWorkflowNotFound(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	newTitle := "New Title"
	err := store.UpdateWorkflow(ctx, "nonexistent-id", WorkflowUpdate{
		Title: &newTitle,
	})
	if err == nil {
		t.Error("expected error for nonexistent workflow")
	}
}

func TestSQLiteStore_WithTxRollback(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	id := uuid.New().String()
	workflow := &Workflow{
		ID:    id,
		Title: "Original Title",
		State: WorkflowStateDraft,
	}

	if err := store.CreateWorkflow(ctx, workflow); err != nil {
		t.Fatalf("failed to create workflow: %v", err)
	}

	err := store.WithTx(ctx, func(txStore WorkflowStore) error {
		newTitle := "Updated Title"
		if err := txStore.UpdateWorkflow(ctx, id, WorkflowUpdate{Title: &newTitle}); err != nil {
			return err
		}
		return fmt.Errorf("trigger rollback")
	})
	if err == nil {
		t.Fatal("expected rollback error")
	}

	after, err := store.GetWorkflow(ctx, id)
	if err != nil {
		t.Fatalf("failed to get workflow after rollback: %v", err)
	}
	if after.Title != "Original Title" {
		t.Fatalf("expected title rollback to preserve original title, got %q", after.Title)
	}
}

func TestSQLiteStore_DeleteWorkflow(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	id := uuid.New().String()
	workflow := &Workflow{
		ID:    id,
		Title: "To Delete",
		State: WorkflowStateDraft,
	}

	if err := store.CreateWorkflow(ctx, workflow); err != nil {
		t.Fatalf("failed to create workflow: %v", err)
	}

	if err := store.DeleteWorkflow(ctx, id); err != nil {
		t.Fatalf("failed to delete workflow: %v", err)
	}

	_, err := store.GetWorkflow(ctx, id)
	if err == nil {
		t.Error("expected error getting deleted workflow")
	}
}

// =============================================================================
// Workflow Listing Tests
// =============================================================================

func TestSQLiteStore_ListWorkflows(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()

	// Create multiple workflows
	for i := 0; i < 3; i++ {
		workflow := &Workflow{
			ID:    uuid.New().String(),
			Title: "Workflow",
			State: WorkflowStateDraft,
		}
		if err := store.CreateWorkflow(ctx, workflow); err != nil {
			t.Fatalf("failed to create workflow: %v", err)
		}
	}

	workflows, err := store.ListWorkflows(ctx)
	if err != nil {
		t.Fatalf("failed to list workflows: %v", err)
	}

	if len(workflows) != 3 {
		t.Errorf("expected 3 workflows, got %d", len(workflows))
	}
}

func TestSQLiteStore_ListWorkflowsByState(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()

	// Create workflows with different states
	states := []WorkflowState{
		WorkflowStateDraft,
		WorkflowStateRunning,
		WorkflowStateRunning,
		WorkflowStateCompleted,
	}

	for _, state := range states {
		workflow := &Workflow{
			ID:    uuid.New().String(),
			Title: "Workflow",
			State: state,
		}
		if err := store.CreateWorkflow(ctx, workflow); err != nil {
			t.Fatalf("failed to create workflow: %v", err)
		}
	}

	// List only running workflows
	running, err := store.ListWorkflowsByState(ctx, WorkflowStateRunning)
	if err != nil {
		t.Fatalf("failed to list workflows by state: %v", err)
	}

	if len(running) != 2 {
		t.Errorf("expected 2 running workflows, got %d", len(running))
	}

	// List running and completed
	active, err := store.ListWorkflowsByState(ctx, WorkflowStateRunning, WorkflowStateCompleted)
	if err != nil {
		t.Fatalf("failed to list workflows by states: %v", err)
	}

	if len(active) != 3 {
		t.Errorf("expected 3 active workflows, got %d", len(active))
	}
}

// =============================================================================
// Step Tests
// =============================================================================

func TestSQLiteStore_CreateStep(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()

	// Create workflow first
	workflowID := uuid.New().String()
	workflow := &Workflow{
		ID:    workflowID,
		Title: "Workflow",
		State: WorkflowStateDraft,
	}
	if err := store.CreateWorkflow(ctx, workflow); err != nil {
		t.Fatalf("failed to create workflow: %v", err)
	}

	// Create step
	step := &Step{
		ID:               uuid.New().String(),
		WorkflowID:       workflowID,
		StepIndex:        0,
		StepType:         StepTypePlan,
		Agent:            "claude-code",
		Status:           StepStatusPending,
		Title:            "Planning Step",
		RequiresApproval: true,
		MaxRetries:       3,
	}

	if err := store.CreateStep(ctx, step); err != nil {
		t.Fatalf("failed to create step: %v", err)
	}

	if step.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}
}

func TestSQLiteStore_UpdateStep(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()

	// Create workflow and step
	workflowID := uuid.New().String()
	workflow := &Workflow{ID: workflowID, Title: "Workflow", State: WorkflowStateDraft}
	if err := store.CreateWorkflow(ctx, workflow); err != nil {
		t.Fatalf("failed to create workflow: %v", err)
	}

	stepID := uuid.New().String()
	step := &Step{
		ID:         stepID,
		WorkflowID: workflowID,
		StepIndex:  0,
		StepType:   StepTypePlan,
		Agent:      "claude-code",
		Status:     StepStatusPending,
	}
	if err := store.CreateStep(ctx, step); err != nil {
		t.Fatalf("failed to create step: %v", err)
	}

	// Update step
	now := time.Now()
	newStatus := StepStatusRunning
	err := store.UpdateStep(ctx, stepID, StepUpdate{
		Status:    &newStatus,
		StartedAt: &now,
	})
	if err != nil {
		t.Fatalf("failed to update step: %v", err)
	}

	// Verify
	updated, err := store.GetStep(ctx, stepID)
	if err != nil {
		t.Fatalf("failed to get step: %v", err)
	}

	if updated.Status != StepStatusRunning {
		t.Errorf("expected Status running, got %s", updated.Status)
	}
	if updated.StartedAt == nil {
		t.Error("expected StartedAt to be set")
	}
}

func TestSQLiteStore_GetStepsByWorkflow(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()

	// Create workflow
	workflowID := uuid.New().String()
	workflow := &Workflow{ID: workflowID, Title: "Workflow", State: WorkflowStateDraft}
	if err := store.CreateWorkflow(ctx, workflow); err != nil {
		t.Fatalf("failed to create workflow: %v", err)
	}

	// Create multiple steps
	for i := 0; i < 3; i++ {
		step := &Step{
			ID:         uuid.New().String(),
			WorkflowID: workflowID,
			StepIndex:  i,
			StepType:   StepTypeCode,
			Agent:      "claude-code",
			Status:     StepStatusPending,
		}
		if err := store.CreateStep(ctx, step); err != nil {
			t.Fatalf("failed to create step: %v", err)
		}
	}

	steps, err := store.GetStepsByWorkflow(ctx, workflowID)
	if err != nil {
		t.Fatalf("failed to get steps: %v", err)
	}

	if len(steps) != 3 {
		t.Errorf("expected 3 steps, got %d", len(steps))
	}

	// Verify order
	for i, step := range steps {
		if step.StepIndex != i {
			t.Errorf("expected step index %d, got %d", i, step.StepIndex)
		}
	}
}

// =============================================================================
// Approval Tests
// =============================================================================

func TestSQLiteStore_ApproveStep(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()

	// Create workflow and step requiring approval
	workflowID := uuid.New().String()
	workflow := &Workflow{ID: workflowID, Title: "Workflow", State: WorkflowStateRunning}
	if err := store.CreateWorkflow(ctx, workflow); err != nil {
		t.Fatalf("failed to create workflow: %v", err)
	}

	stepID := uuid.New().String()
	step := &Step{
		ID:               stepID,
		WorkflowID:       workflowID,
		StepIndex:        0,
		StepType:         StepTypeReview,
		Agent:            "codex",
		Status:           StepStatusWaiting,
		RequiresApproval: true,
		ApprovalStatus:   ApprovalPending,
	}
	if err := store.CreateStep(ctx, step); err != nil {
		t.Fatalf("failed to create step: %v", err)
	}

	// Approve the step
	err := store.ApproveStep(ctx, stepID, true, "Looks good!")
	if err != nil {
		t.Fatalf("failed to approve step: %v", err)
	}

	// Verify
	approved, err := store.GetStep(ctx, stepID)
	if err != nil {
		t.Fatalf("failed to get step: %v", err)
	}

	if approved.ApprovalStatus != ApprovalApproved {
		t.Errorf("expected ApprovalStatus approved, got %s", approved.ApprovalStatus)
	}
	if approved.Status != StepStatusPending {
		t.Errorf("expected Status pending (ready to continue), got %s", approved.Status)
	}
}

func TestSQLiteStore_RejectStep(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()

	// Create workflow and step
	workflowID := uuid.New().String()
	workflow := &Workflow{ID: workflowID, Title: "Workflow", State: WorkflowStateRunning}
	if err := store.CreateWorkflow(ctx, workflow); err != nil {
		t.Fatalf("failed to create workflow: %v", err)
	}

	stepID := uuid.New().String()
	step := &Step{
		ID:               stepID,
		WorkflowID:       workflowID,
		StepIndex:        0,
		StepType:         StepTypeReview,
		Agent:            "codex",
		Status:           StepStatusWaiting,
		RequiresApproval: true,
		ApprovalStatus:   ApprovalPending,
	}
	if err := store.CreateStep(ctx, step); err != nil {
		t.Fatalf("failed to create step: %v", err)
	}

	// Reject the step
	err := store.ApproveStep(ctx, stepID, false, "Needs changes")
	if err != nil {
		t.Fatalf("failed to reject step: %v", err)
	}

	// Verify
	rejected, err := store.GetStep(ctx, stepID)
	if err != nil {
		t.Fatalf("failed to get step: %v", err)
	}

	if rejected.ApprovalStatus != ApprovalRejected {
		t.Errorf("expected ApprovalStatus rejected, got %s", rejected.ApprovalStatus)
	}
	if rejected.Status != StepStatusFailed {
		t.Errorf("expected Status failed, got %s", rejected.Status)
	}
}
