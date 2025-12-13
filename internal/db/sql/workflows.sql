-- name: CreateWorkflow :one
INSERT INTO workflows (
    id,
    parent_session_id,
    title,
    state,
    plan_json,
    config_json,
    current_step_index,
    error_message
) VALUES (
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?
) RETURNING *;

-- name: GetWorkflowByID :one
SELECT * FROM workflows WHERE id = ? LIMIT 1;

-- name: ListWorkflows :many
SELECT * FROM workflows ORDER BY updated_at DESC;

-- name: ListWorkflowsByState :many
SELECT * FROM workflows WHERE state = ? ORDER BY updated_at DESC;

-- name: ListWorkflowsBySession :many
SELECT * FROM workflows WHERE parent_session_id = ? ORDER BY updated_at DESC;

-- name: UpdateWorkflowState :one
UPDATE workflows SET
    state = ?,
    error_message = ?
WHERE id = ?
RETURNING *;

-- name: UpdateWorkflowStep :one
UPDATE workflows SET
    current_step_index = ?
WHERE id = ?
RETURNING *;

-- name: CompleteWorkflow :one
UPDATE workflows SET
    state = ?,
    completed_at = strftime('%s','now')
WHERE id = ?
RETURNING *;

-- name: DeleteWorkflow :exec
DELETE FROM workflows WHERE id = ?;

-- name: CreateWorkflowStep :one
INSERT INTO workflow_steps (
    id,
    workflow_id,
    step_index,
    step_type,
    agent,
    agent_session_id,
    status,
    title,
    input_context_json,
    output_json,
    review_result_json,
    retry_count,
    max_retries,
    requires_approval,
    approval_status,
    error_message
) VALUES (
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?
) RETURNING *;

-- name: GetWorkflowStepByID :one
SELECT * FROM workflow_steps WHERE id = ? LIMIT 1;

-- name: ListWorkflowSteps :many
SELECT * FROM workflow_steps 
WHERE workflow_id = ? 
ORDER BY step_index ASC;

-- name: GetCurrentWorkflowStep :one
SELECT ws.* FROM workflow_steps ws
JOIN workflows w ON ws.workflow_id = w.id
WHERE w.id = ? AND ws.step_index = w.current_step_index
LIMIT 1;

-- name: UpdateWorkflowStepStatus :one
UPDATE workflow_steps SET
    status = ?,
    error_message = ?
WHERE id = ?
RETURNING *;

-- name: StartWorkflowStep :one
UPDATE workflow_steps SET
    status = 'running',
    started_at = strftime('%s','now')
WHERE id = ?
RETURNING *;

-- name: CompleteWorkflowStep :one
UPDATE workflow_steps SET
    status = 'completed',
    output_json = ?,
    completed_at = strftime('%s','now')
WHERE id = ?
RETURNING *;

-- name: FailWorkflowStep :one
UPDATE workflow_steps SET
    status = 'failed',
    error_message = ?,
    completed_at = strftime('%s','now')
WHERE id = ?
RETURNING *;

-- name: SetStepApprovalStatus :one
UPDATE workflow_steps SET
    approval_status = ?
WHERE id = ?
RETURNING *;

-- name: IncrementStepRetry :one
UPDATE workflow_steps SET
    retry_count = retry_count + 1,
    status = 'pending'
WHERE id = ?
RETURNING *;

-- name: SetStepAgentSession :one
UPDATE workflow_steps SET
    agent_session_id = ?
WHERE id = ?
RETURNING *;

-- name: DeleteWorkflowSteps :exec
DELETE FROM workflow_steps WHERE workflow_id = ?;

-- DAG-specific queries

-- name: CreateWorkflowWithSpec :one
INSERT INTO workflows (
    id,
    parent_session_id,
    title,
    state,
    plan_json,
    config_json,
    spec_json,
    current_step_index,
    current_node_id,
    error_message
) VALUES (
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?
) RETURNING *;

-- name: UpdateWorkflowCurrentNode :one
UPDATE workflows SET
    current_node_id = ?
WHERE id = ?
RETURNING *;

-- name: GetWorkflowStepByNodeID :one
SELECT * FROM workflow_steps 
WHERE workflow_id = ? AND node_id = ?
LIMIT 1;

-- name: CreateWorkflowStepWithNode :one
INSERT INTO workflow_steps (
    id,
    workflow_id,
    step_index,
    step_type,
    agent,
    agent_session_id,
    status,
    title,
    input_context_json,
    output_json,
    review_result_json,
    retry_count,
    max_retries,
    requires_approval,
    approval_status,
    error_message,
    node_id
) VALUES (
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?
) RETURNING *;

-- name: GetCurrentDAGStep :one
SELECT ws.* FROM workflow_steps ws
JOIN workflows w ON ws.workflow_id = w.id
WHERE w.id = ? AND ws.node_id = w.current_node_id
LIMIT 1;
