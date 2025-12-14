// Package workflow provides a state-machine-driven workflow engine for orchestrating
// multi-step agent workflows with approval gates, pause/resume, and retry logic.
package workflow

import (
	"context"
)

// =============================================================================
// Store Interface - Aligned with HumanLayer Pattern
// =============================================================================

// WorkflowStore defines the interface for workflow persistence.
// This interface follows the humanlayer pattern of separating storage operations
// from the domain model, enabling flexible backend implementations.
type WorkflowStore interface {
	// Workflow CRUD operations
	CreateWorkflow(ctx context.Context, workflow *Workflow) error
	GetWorkflow(ctx context.Context, id string) (*Workflow, error)
	UpdateWorkflow(ctx context.Context, id string, updates WorkflowUpdate) error
	DeleteWorkflow(ctx context.Context, id string) error

	// Workflow listing and filtering
	ListWorkflows(ctx context.Context) ([]Workflow, error)
	ListWorkflowsBySession(ctx context.Context, sessionID string) ([]Workflow, error)
	ListWorkflowsByState(ctx context.Context, states ...WorkflowState) ([]Workflow, error)

	// Step CRUD operations
	CreateStep(ctx context.Context, step *Step) error
	GetStep(ctx context.Context, id string) (*Step, error)
	UpdateStep(ctx context.Context, id string, updates StepUpdate) error

	// Step queries
	GetStepsByWorkflow(ctx context.Context, workflowID string) ([]Step, error)
	GetStepByNodeID(ctx context.Context, workflowID, nodeID string) (*Step, error)
	GetCurrentStep(ctx context.Context, workflowID string) (*Step, error)

	// Approval operations
	GetPendingApprovalSteps(ctx context.Context, workflowID string) ([]Step, error)
	ApproveStep(ctx context.Context, stepID string, approved bool, comment string) error

	// Transaction support
	WithTx(ctx context.Context, fn func(store WorkflowStore) error) error
}

// ApprovalComment stores the comment when a step is approved or rejected.
type ApprovalComment struct {
	StepID   string `json:"step_id"`
	Approved bool   `json:"approved"`
	Comment  string `json:"comment,omitempty"`
}
