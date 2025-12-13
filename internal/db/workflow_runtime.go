package db

import "context"

// ResetWorkflowStepRuntime resets a workflow step so it can be re-executed as part of DAG transitions.
// This is intentionally separate from sqlc-generated queries so the runtime can re-queue nodes
// without requiring regeneration.
func (q *Queries) ResetWorkflowStepRuntime(ctx context.Context, stepID string) error {
	_, err := q.db.ExecContext(ctx, `
UPDATE workflow_steps SET
	status = 'pending',
	approval_status = NULL,
	error_message = NULL,
	retry_count = 0,
	agent_session_id = NULL,
	output_json = NULL,
	review_result_json = NULL,
	started_at = NULL,
	completed_at = NULL
WHERE id = ?
`, stepID)
	return err
}

