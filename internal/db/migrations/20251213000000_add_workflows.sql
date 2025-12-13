-- +goose Up
-- +goose StatementBegin
-- Workflows table
CREATE TABLE
    IF NOT EXISTS workflows (
        id TEXT PRIMARY KEY,
        parent_session_id TEXT REFERENCES sessions (id),
        title TEXT NOT NULL,
        state TEXT NOT NULL DEFAULT 'draft',
        plan_json TEXT,
        config_json TEXT,
        current_step_index INTEGER NOT NULL DEFAULT 0,
        error_message TEXT,
        created_at INTEGER NOT NULL DEFAULT (strftime ('%s', 'now')),
        updated_at INTEGER NOT NULL DEFAULT (strftime ('%s', 'now')),
        completed_at INTEGER
    );

-- Workflow Steps table
CREATE TABLE
    IF NOT EXISTS workflow_steps (
        id TEXT PRIMARY KEY,
        workflow_id TEXT NOT NULL REFERENCES workflows (id) ON DELETE CASCADE,
        step_index INTEGER NOT NULL,
        step_type TEXT NOT NULL,
        agent TEXT NOT NULL,
        agent_session_id TEXT,
        status TEXT NOT NULL DEFAULT 'pending',
        title TEXT,
        input_context_json TEXT,
        output_json TEXT,
        review_result_json TEXT,
        retry_count INTEGER NOT NULL DEFAULT 0,
        max_retries INTEGER NOT NULL DEFAULT 3,
        requires_approval INTEGER NOT NULL DEFAULT 0,
        approval_status TEXT,
        error_message TEXT,
        created_at INTEGER NOT NULL DEFAULT (strftime ('%s', 'now')),
        updated_at INTEGER NOT NULL DEFAULT (strftime ('%s', 'now')),
        started_at INTEGER,
        completed_at INTEGER,
        UNIQUE (workflow_id, step_index)
    );

-- Indexes
CREATE INDEX idx_workflows_parent_session ON workflows (parent_session_id);

CREATE INDEX idx_workflows_state ON workflows (state);

CREATE INDEX idx_workflow_steps_workflow ON workflow_steps (workflow_id);

CREATE INDEX idx_workflow_steps_status ON workflow_steps (status);

-- Trigger: auto-update updated_at on workflows
CREATE TRIGGER update_workflows_updated_at AFTER
UPDATE ON workflows FOR EACH ROW BEGIN
UPDATE workflows
SET
    updated_at = strftime ('%s', 'now')
WHERE
    id = NEW.id;

END;

-- Trigger: auto-update updated_at on workflow_steps and bump parent workflow
CREATE TRIGGER update_workflow_steps_updated_at AFTER
UPDATE ON workflow_steps FOR EACH ROW BEGIN
UPDATE workflow_steps
SET
    updated_at = strftime ('%s', 'now')
WHERE
    id = NEW.id;

UPDATE workflows
SET
    updated_at = strftime ('%s', 'now')
WHERE
    id = NEW.workflow_id;

END;

-- Trigger: touch parent workflow on step insert
CREATE TRIGGER insert_workflow_steps_touch_workflow AFTER INSERT ON workflow_steps FOR EACH ROW BEGIN
UPDATE workflows
SET
    updated_at = strftime ('%s', 'now')
WHERE
    id = NEW.workflow_id;

END;

-- Trigger: touch parent workflow on step delete
CREATE TRIGGER delete_workflow_steps_touch_workflow AFTER DELETE ON workflow_steps FOR EACH ROW BEGIN
UPDATE workflows
SET
    updated_at = strftime ('%s', 'now')
WHERE
    id = OLD.workflow_id;

END;

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DROP TRIGGER IF EXISTS delete_workflow_steps_touch_workflow;

DROP TRIGGER IF EXISTS insert_workflow_steps_touch_workflow;

DROP TRIGGER IF EXISTS update_workflow_steps_updated_at;

DROP TRIGGER IF EXISTS update_workflows_updated_at;

DROP INDEX IF EXISTS idx_workflow_steps_status;

DROP INDEX IF EXISTS idx_workflow_steps_workflow;

DROP INDEX IF EXISTS idx_workflows_state;

DROP INDEX IF EXISTS idx_workflows_parent_session;

DROP TABLE IF EXISTS workflow_steps;

DROP TABLE IF EXISTS workflows;

-- +goose StatementEnd