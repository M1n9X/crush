// Package workflow provides a state-machine-driven workflow engine for orchestrating
// multi-step agent workflows with approval gates, pause/resume, and retry logic.
package workflow

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/crush/internal/db"
)

// =============================================================================
// SQLite Store Implementation - Aligned with HumanLayer Pattern
// =============================================================================

type sqlConn interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// SQLiteStore implements WorkflowStore using SQLite with SQLC-generated queries.
// For dynamic updates, it uses raw SQL similar to humanlayer's UpdateSession pattern.
type SQLiteStore struct {
	queries *db.Queries
	db      *sql.DB
	conn    sqlConn
}

// NewSQLiteStore creates a new SQLite-backed WorkflowStore.
func NewSQLiteStore(queries *db.Queries, database *sql.DB) *SQLiteStore {
	return &SQLiteStore{
		queries: queries,
		db:      database,
		conn:    database,
	}
}

// Ensure SQLiteStore implements WorkflowStore.
var _ WorkflowStore = (*SQLiteStore)(nil)

// =============================================================================
// Workflow CRUD Operations
// =============================================================================

// CreateWorkflow creates a new workflow in the database.
func (s *SQLiteStore) CreateWorkflow(ctx context.Context, workflow *Workflow) error {
	if workflow.SpecJSON != "" || workflow.CurrentNodeID != "" {
		params := db.CreateWorkflowWithSpecParams{
			ID:               workflow.ID,
			Title:            workflow.Title,
			State:            string(workflow.State),
			CurrentStepIndex: int64(workflow.CurrentStepIndex),
		}

		if workflow.ParentSessionID != "" {
			params.ParentSessionID = sql.NullString{String: workflow.ParentSessionID, Valid: true}
		}
		if workflow.PlanJSON != "" {
			params.PlanJson = sql.NullString{String: workflow.PlanJSON, Valid: true}
		}
		if workflow.ConfigJSON != "" {
			params.ConfigJson = sql.NullString{String: workflow.ConfigJSON, Valid: true}
		}
		if workflow.SpecJSON != "" {
			params.SpecJson = sql.NullString{String: workflow.SpecJSON, Valid: true}
		}
		if workflow.CurrentNodeID != "" {
			params.CurrentNodeID = sql.NullString{String: workflow.CurrentNodeID, Valid: true}
		}
		if workflow.ErrorMessage != "" {
			params.ErrorMessage = sql.NullString{String: workflow.ErrorMessage, Valid: true}
		}

		created, err := s.queries.CreateWorkflowWithSpec(ctx, params)
		if err != nil {
			return fmt.Errorf("failed to create workflow: %w", err)
		}

		*workflow = WorkflowFromDB(created)
		return nil
	}

	params := db.CreateWorkflowParams{
		ID:               workflow.ID,
		Title:            workflow.Title,
		State:            string(workflow.State),
		CurrentStepIndex: int64(workflow.CurrentStepIndex),
	}

	if workflow.ParentSessionID != "" {
		params.ParentSessionID = sql.NullString{String: workflow.ParentSessionID, Valid: true}
	}
	if workflow.PlanJSON != "" {
		params.PlanJson = sql.NullString{String: workflow.PlanJSON, Valid: true}
	}
	if workflow.ConfigJSON != "" {
		params.ConfigJson = sql.NullString{String: workflow.ConfigJSON, Valid: true}
	}
	if workflow.ErrorMessage != "" {
		params.ErrorMessage = sql.NullString{String: workflow.ErrorMessage, Valid: true}
	}

	created, err := s.queries.CreateWorkflow(ctx, params)
	if err != nil {
		return fmt.Errorf("failed to create workflow: %w", err)
	}

	// Update workflow with DB-generated timestamps
	*workflow = WorkflowFromDB(created)
	return nil
}

// GetWorkflow retrieves a workflow by ID.
func (s *SQLiteStore) GetWorkflow(ctx context.Context, id string) (*Workflow, error) {
	dbWorkflow, err := s.queries.GetWorkflowByID(ctx, id)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("workflow not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get workflow: %w", err)
	}

	workflow := WorkflowFromDB(dbWorkflow)

	// Load steps
	steps, err := s.GetStepsByWorkflow(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to load workflow steps: %w", err)
	}
	workflow.Steps = steps

	return &workflow, nil
}

// UpdateWorkflow performs a partial update on a workflow.
// This follows humanlayer's dynamic SQL pattern for flexible field updates.
func (s *SQLiteStore) UpdateWorkflow(ctx context.Context, id string, updates WorkflowUpdate) error {
	if updates.IsEmpty() {
		return nil // No-op
	}

	query := `UPDATE workflows SET`
	args := []interface{}{}
	setParts := []string{}

	if updates.Title != nil {
		setParts = append(setParts, "title = ?")
		args = append(args, *updates.Title)
	}
	if updates.State != nil {
		setParts = append(setParts, "state = ?")
		args = append(args, string(*updates.State))
	}
	if updates.CurrentStepIndex != nil {
		setParts = append(setParts, "current_step_index = ?")
		args = append(args, *updates.CurrentStepIndex)
	}
	if updates.CurrentNodeID != nil {
		setParts = append(setParts, "current_node_id = ?")
		args = append(args, *updates.CurrentNodeID)
	}
	if updates.PlanJSON != nil {
		setParts = append(setParts, "plan_json = ?")
		args = append(args, *updates.PlanJSON)
	}
	if updates.ConfigJSON != nil {
		setParts = append(setParts, "config_json = ?")
		args = append(args, *updates.ConfigJSON)
	}
	if updates.SpecJSON != nil {
		setParts = append(setParts, "spec_json = ?")
		args = append(args, *updates.SpecJSON)
	}
	if updates.ErrorMessage != nil {
		setParts = append(setParts, "error_message = ?")
		args = append(args, *updates.ErrorMessage)
	}
	if updates.CompletedAt != nil {
		setParts = append(setParts, "completed_at = ?")
		args = append(args, updates.CompletedAt.Unix())
	}

	if len(setParts) == 0 {
		return nil
	}

	query += " " + strings.Join(setParts, ", ")
	query += " WHERE id = ?"
	args = append(args, id)

	result, err := s.conn.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to update workflow: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("workflow not found: %s", id)
	}

	return nil
}

// DeleteWorkflow deletes a workflow and its steps.
func (s *SQLiteStore) DeleteWorkflow(ctx context.Context, id string) error {
	if err := s.queries.DeleteWorkflowSteps(ctx, id); err != nil {
		return fmt.Errorf("failed to delete workflow steps: %w", err)
	}
	if err := s.queries.DeleteWorkflow(ctx, id); err != nil {
		return fmt.Errorf("failed to delete workflow: %w", err)
	}
	return nil
}

// =============================================================================
// Workflow Listing
// =============================================================================

// ListWorkflows returns all workflows ordered by updated_at descending.
func (s *SQLiteStore) ListWorkflows(ctx context.Context) ([]Workflow, error) {
	dbWorkflows, err := s.queries.ListWorkflows(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list workflows: %w", err)
	}

	workflows := make([]Workflow, len(dbWorkflows))
	for i, dbW := range dbWorkflows {
		workflows[i] = WorkflowFromDB(dbW)
	}
	return workflows, nil
}

// ListWorkflowsBySession returns workflows for a specific parent session.
func (s *SQLiteStore) ListWorkflowsBySession(ctx context.Context, sessionID string) ([]Workflow, error) {
	dbWorkflows, err := s.queries.ListWorkflowsBySession(ctx, sql.NullString{String: sessionID, Valid: true})
	if err != nil {
		return nil, fmt.Errorf("failed to list workflows by session: %w", err)
	}

	workflows := make([]Workflow, len(dbWorkflows))
	for i, dbW := range dbWorkflows {
		workflows[i] = WorkflowFromDB(dbW)
	}
	return workflows, nil
}

// ListWorkflowsByState returns workflows in the specified states.
func (s *SQLiteStore) ListWorkflowsByState(ctx context.Context, states ...WorkflowState) ([]Workflow, error) {
	if len(states) == 0 {
		return nil, nil
	}

	// Build dynamic query for multiple states
	placeholders := make([]string, len(states))
	args := make([]interface{}, len(states))
	for i, state := range states {
		placeholders[i] = "?"
		args[i] = string(state)
	}

	query := fmt.Sprintf(`
		SELECT id, parent_session_id, title, state, plan_json, config_json, 
		       current_step_index, error_message, created_at, updated_at, 
		       completed_at, spec_json, current_node_id
		FROM workflows 
		WHERE state IN (%s) 
		ORDER BY updated_at DESC
	`, strings.Join(placeholders, ","))

	rows, err := s.conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list workflows by state: %w", err)
	}
	defer rows.Close()

	var workflows []Workflow
	for rows.Next() {
		var w db.Workflow
		if err := rows.Scan(
			&w.ID, &w.ParentSessionID, &w.Title, &w.State, &w.PlanJson,
			&w.ConfigJson, &w.CurrentStepIndex, &w.ErrorMessage, &w.CreatedAt,
			&w.UpdatedAt, &w.CompletedAt, &w.SpecJson, &w.CurrentNodeID,
		); err != nil {
			return nil, fmt.Errorf("failed to scan workflow: %w", err)
		}
		workflows = append(workflows, WorkflowFromDB(w))
	}

	return workflows, nil
}

// =============================================================================
// Step CRUD Operations
// =============================================================================

// CreateStep creates a new workflow step.
func (s *SQLiteStore) CreateStep(ctx context.Context, step *Step) error {
	if step.NodeID != "" {
		params := db.CreateWorkflowStepWithNodeParams{
			ID:         step.ID,
			WorkflowID: step.WorkflowID,
			StepIndex:  int64(step.StepIndex),
			StepType:   string(step.StepType),
			Agent:      step.Agent,
			Status:     string(step.Status),
			RetryCount: int64(step.RetryCount),
			MaxRetries: int64(step.MaxRetries),
			NodeID:     sql.NullString{String: step.NodeID, Valid: true},
		}

		if step.RequiresApproval {
			params.RequiresApproval = 1
		}
		if step.AgentSessionID != "" {
			params.AgentSessionID = sql.NullString{String: step.AgentSessionID, Valid: true}
		}
		if step.Title != "" {
			params.Title = sql.NullString{String: step.Title, Valid: true}
		}
		if step.InputContextJSON != "" {
			params.InputContextJson = sql.NullString{String: step.InputContextJSON, Valid: true}
		}
		if step.OutputJSON != "" {
			params.OutputJson = sql.NullString{String: step.OutputJSON, Valid: true}
		}
		if step.ReviewResultJSON != "" {
			params.ReviewResultJson = sql.NullString{String: step.ReviewResultJSON, Valid: true}
		}
		if step.ApprovalStatus != "" {
			params.ApprovalStatus = sql.NullString{String: string(step.ApprovalStatus), Valid: true}
		}
		if step.ErrorMessage != "" {
			params.ErrorMessage = sql.NullString{String: step.ErrorMessage, Valid: true}
		}

		created, err := s.queries.CreateWorkflowStepWithNode(ctx, params)
		if err != nil {
			return fmt.Errorf("failed to create step: %w", err)
		}

		*step = StepFromDB(created)
		return nil
	}

	params := db.CreateWorkflowStepParams{
		ID:         step.ID,
		WorkflowID: step.WorkflowID,
		StepIndex:  int64(step.StepIndex),
		StepType:   string(step.StepType),
		Agent:      step.Agent,
		Status:     string(step.Status),
		RetryCount: int64(step.RetryCount),
		MaxRetries: int64(step.MaxRetries),
	}

	if step.RequiresApproval {
		params.RequiresApproval = 1
	}
	if step.AgentSessionID != "" {
		params.AgentSessionID = sql.NullString{String: step.AgentSessionID, Valid: true}
	}
	if step.Title != "" {
		params.Title = sql.NullString{String: step.Title, Valid: true}
	}
	if step.InputContextJSON != "" {
		params.InputContextJson = sql.NullString{String: step.InputContextJSON, Valid: true}
	}
	if step.OutputJSON != "" {
		params.OutputJson = sql.NullString{String: step.OutputJSON, Valid: true}
	}
	if step.ReviewResultJSON != "" {
		params.ReviewResultJson = sql.NullString{String: step.ReviewResultJSON, Valid: true}
	}
	if step.ApprovalStatus != "" {
		params.ApprovalStatus = sql.NullString{String: string(step.ApprovalStatus), Valid: true}
	}
	if step.ErrorMessage != "" {
		params.ErrorMessage = sql.NullString{String: step.ErrorMessage, Valid: true}
	}

	created, err := s.queries.CreateWorkflowStep(ctx, params)
	if err != nil {
		return fmt.Errorf("failed to create step: %w", err)
	}

	*step = StepFromDB(created)
	return nil
}

// GetStep retrieves a step by ID.
func (s *SQLiteStore) GetStep(ctx context.Context, id string) (*Step, error) {
	dbStep, err := s.queries.GetWorkflowStepByID(ctx, id)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("step not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get step: %w", err)
	}

	step := StepFromDB(dbStep)
	return &step, nil
}

// UpdateStep performs a partial update on a step.
func (s *SQLiteStore) UpdateStep(ctx context.Context, id string, updates StepUpdate) error {
	if updates.IsEmpty() {
		return nil
	}

	query := `UPDATE workflow_steps SET`
	args := []interface{}{}
	setParts := []string{}

	if updates.Status != nil {
		setParts = append(setParts, "status = ?")
		args = append(args, string(*updates.Status))
	}
	if updates.AgentSessionID != nil {
		setParts = append(setParts, "agent_session_id = ?")
		args = append(args, *updates.AgentSessionID)
	}
	if updates.InputContextJSON != nil {
		setParts = append(setParts, "input_context_json = ?")
		args = append(args, *updates.InputContextJSON)
	}
	if updates.OutputJSON != nil {
		setParts = append(setParts, "output_json = ?")
		args = append(args, *updates.OutputJSON)
	}
	if updates.ReviewResultJSON != nil {
		setParts = append(setParts, "review_result_json = ?")
		args = append(args, *updates.ReviewResultJSON)
	}
	if updates.RetryCount != nil {
		setParts = append(setParts, "retry_count = ?")
		args = append(args, *updates.RetryCount)
	}
	if updates.ApprovalStatus != nil {
		setParts = append(setParts, "approval_status = ?")
		args = append(args, string(*updates.ApprovalStatus))
	}
	if updates.ErrorMessage != nil {
		setParts = append(setParts, "error_message = ?")
		args = append(args, *updates.ErrorMessage)
	}
	if updates.StartedAt != nil {
		setParts = append(setParts, "started_at = ?")
		args = append(args, updates.StartedAt.Unix())
	}
	if updates.CompletedAt != nil {
		setParts = append(setParts, "completed_at = ?")
		args = append(args, updates.CompletedAt.Unix())
	}

	if len(setParts) == 0 {
		return nil
	}

	query += " " + strings.Join(setParts, ", ")
	query += " WHERE id = ?"
	args = append(args, id)

	result, err := s.conn.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to update step: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("step not found: %s", id)
	}

	return nil
}

// =============================================================================
// Step Queries
// =============================================================================

// GetStepsByWorkflow returns all steps for a workflow.
func (s *SQLiteStore) GetStepsByWorkflow(ctx context.Context, workflowID string) ([]Step, error) {
	dbSteps, err := s.queries.ListWorkflowSteps(ctx, workflowID)
	if err != nil {
		return nil, fmt.Errorf("failed to list workflow steps: %w", err)
	}

	steps := make([]Step, len(dbSteps))
	for i, dbS := range dbSteps {
		steps[i] = StepFromDB(dbS)
	}
	return steps, nil
}

// GetStepByNodeID retrieves a step by workflow ID and node ID.
func (s *SQLiteStore) GetStepByNodeID(ctx context.Context, workflowID, nodeID string) (*Step, error) {
	dbStep, err := s.queries.GetWorkflowStepByNodeID(ctx, db.GetWorkflowStepByNodeIDParams{
		WorkflowID: workflowID,
		NodeID:     sql.NullString{String: nodeID, Valid: true},
	})
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get step by node ID: %w", err)
	}

	step := StepFromDB(dbStep)
	return &step, nil
}

// GetCurrentStep retrieves the current step for a workflow.
func (s *SQLiteStore) GetCurrentStep(ctx context.Context, workflowID string) (*Step, error) {
	dbStep, err := s.queries.GetCurrentWorkflowStep(ctx, workflowID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get current step: %w", err)
	}

	step := StepFromDB(dbStep)
	return &step, nil
}

// =============================================================================
// Approval Operations
// =============================================================================

// GetPendingApprovalSteps returns steps awaiting approval.
func (s *SQLiteStore) GetPendingApprovalSteps(ctx context.Context, workflowID string) ([]Step, error) {
	query := `
		SELECT id, workflow_id, step_index, step_type, agent, agent_session_id,
		       status, title, input_context_json, output_json, review_result_json,
		       retry_count, max_retries, requires_approval, approval_status,
		       error_message, created_at, updated_at, started_at, completed_at, node_id
		FROM workflow_steps
		WHERE workflow_id = ? AND status = 'waiting' AND requires_approval = 1
		ORDER BY step_index ASC
	`

	rows, err := s.conn.QueryContext(ctx, query, workflowID)
	if err != nil {
		return nil, fmt.Errorf("failed to get pending approval steps: %w", err)
	}
	defer rows.Close()

	var steps []Step
	for rows.Next() {
		var s db.WorkflowStep
		if err := rows.Scan(
			&s.ID, &s.WorkflowID, &s.StepIndex, &s.StepType, &s.Agent, &s.AgentSessionID,
			&s.Status, &s.Title, &s.InputContextJson, &s.OutputJson, &s.ReviewResultJson,
			&s.RetryCount, &s.MaxRetries, &s.RequiresApproval, &s.ApprovalStatus,
			&s.ErrorMessage, &s.CreatedAt, &s.UpdatedAt, &s.StartedAt, &s.CompletedAt, &s.NodeID,
		); err != nil {
			return nil, fmt.Errorf("failed to scan step: %w", err)
		}
		steps = append(steps, StepFromDB(s))
	}

	return steps, nil
}

// ApproveStep approves or rejects a pending step.
func (s *SQLiteStore) ApproveStep(ctx context.Context, stepID string, approved bool, comment string) error {
	var status ApprovalStatus
	var stepStatus StepStatus

	if approved {
		status = ApprovalApproved
		stepStatus = StepStatusPending // Ready to continue
	} else {
		status = ApprovalRejected
		stepStatus = StepStatusFailed
	}

	now := time.Now()
	return s.UpdateStep(ctx, stepID, StepUpdate{
		Status:         &stepStatus,
		ApprovalStatus: &status,
		CompletedAt:    &now,
	})
}

// =============================================================================
// Transaction Support
// =============================================================================

// WithTx executes a function within a database transaction.
func (s *SQLiteStore) WithTx(ctx context.Context, fn func(store WorkflowStore) error) error {
	if _, ok := s.conn.(*sql.Tx); ok {
		return fn(s)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Create a new store with the transaction
	txStore := &SQLiteStore{
		queries: s.queries.WithTx(tx),
		db:      s.db, // Note: raw queries must use the same transaction
		conn:    tx,
	}

	if err := fn(txStore); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("rollback failed: %v (original error: %w)", rbErr, err)
		}
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}
