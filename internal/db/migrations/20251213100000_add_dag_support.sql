-- +goose Up
-- +goose StatementBegin

-- Add spec_json to workflows for storing DAG spec
ALTER TABLE workflows ADD COLUMN spec_json TEXT;

-- Add current_node_id for DAG navigation
ALTER TABLE workflows ADD COLUMN current_node_id TEXT;

-- Add node_id to workflow_steps for DAG reference
ALTER TABLE workflow_steps ADD COLUMN node_id TEXT;

-- Create index for node_id lookups
CREATE INDEX idx_workflow_steps_node_id ON workflow_steps(node_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS idx_workflow_steps_node_id;

-- SQLite doesn't support DROP COLUMN directly, so we need to recreate tables
-- For simplicity in development, we'll just leave the columns (they'll be ignored)
-- In production, a proper migration would recreate the tables

-- +goose StatementEnd
