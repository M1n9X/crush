// Package workflow provides a state-machine-driven workflow engine for orchestrating
// multi-step agent workflows with approval gates, pause/resume, and retry logic.
package workflow

import (
	"encoding/json"
	"time"

	"github.com/charmbracelet/crush/internal/db"
)

// WorkflowState defines the lifecycle state of a workflow.
type WorkflowState string

const (
	WorkflowStateDraft        WorkflowState = "draft"         // Initialized, not started
	WorkflowStateRunning      WorkflowState = "running"       // Currently executing
	WorkflowStateWaitingInput WorkflowState = "waiting_input" // Awaiting user input/approval
	WorkflowStatePaused       WorkflowState = "paused"        // User-initiated pause
	WorkflowStateCompleted    WorkflowState = "completed"     // Successfully completed
	WorkflowStateFailed       WorkflowState = "failed"        // Failed and terminated
	WorkflowStateInterrupted  WorkflowState = "interrupted"   // Interrupted (crash/exit)
)

// StepStatus defines the execution status of a workflow step.
type StepStatus string

const (
	StepStatusPending   StepStatus = "pending"   // Not started
	StepStatusRunning   StepStatus = "running"   // Currently executing
	StepStatusCompleted StepStatus = "completed" // Successfully completed
	StepStatusFailed    StepStatus = "failed"    // Failed
	StepStatusSkipped   StepStatus = "skipped"   // Skipped
	StepStatusWaiting   StepStatus = "waiting"   // Awaiting approval
)

// ApprovalStatus defines the approval state for steps requiring approval.
type ApprovalStatus string

const (
	ApprovalPending  ApprovalStatus = "pending"  // Awaiting decision
	ApprovalApproved ApprovalStatus = "approved" // Approved by user
	ApprovalRejected ApprovalStatus = "rejected" // Rejected by user
)

// StepType defines the type of work a step performs.
type StepType string

const (
	StepTypePlan   StepType = "plan"   // Planning step
	StepTypeCode   StepType = "code"   // Coding step
	StepTypeReview StepType = "review" // Review step
	StepTypeDocs   StepType = "docs"   // Documentation step
)

// Workflow represents a running or completed workflow instance.
type Workflow struct {
	ID               string
	ParentSessionID  string
	Title            string
	State            WorkflowState
	PlanJSON         string
	ConfigJSON       string
	CurrentStepIndex int
	ErrorMessage     string
	CreatedAt        time.Time
	UpdatedAt        time.Time
	CompletedAt      *time.Time
	Steps            []Step
}

// Step represents a single step within a workflow.
type Step struct {
	ID               string
	WorkflowID       string
	StepIndex        int
	StepType         StepType
	Agent            string
	AgentSessionID   string
	Status           StepStatus
	Title            string
	InputContextJSON string
	OutputJSON       string
	ReviewResultJSON string
	RetryCount       int
	MaxRetries       int
	RequiresApproval bool
	ApprovalStatus   ApprovalStatus
	ErrorMessage     string
	CreatedAt        time.Time
	UpdatedAt        time.Time
	StartedAt        *time.Time
	CompletedAt      *time.Time
}

// StepConfig defines the configuration for a workflow step.
type StepConfig struct {
	StepType         StepType `json:"step_type"`
	Agent            string   `json:"agent"`
	Title            string   `json:"title"`
	RequiresApproval bool     `json:"requires_approval"`
	MaxRetries       int      `json:"max_retries"`
}

// WorkflowConfig defines the configuration for creating a workflow.
type WorkflowConfig struct {
	Title   string       `json:"title"`
	Plan    string       `json:"plan"`
	Steps   []StepConfig `json:"steps"`
	Context interface{}  `json:"context,omitempty"`
}

// DefaultWorkflowConfig returns the default sequential workflow configuration.
func DefaultWorkflowConfig(title, plan string) WorkflowConfig {
	return WorkflowConfig{
		Title: title,
		Plan:  plan,
		Steps: []StepConfig{
			{StepType: StepTypePlan, Agent: "claude-code", Title: "Planning", RequiresApproval: false, MaxRetries: 1},
			{StepType: StepTypeReview, Agent: "codex", Title: "Plan Review", RequiresApproval: true, MaxRetries: 1},
			{StepType: StepTypeCode, Agent: "claude-code", Title: "Coding", RequiresApproval: false, MaxRetries: 2},
			{StepType: StepTypeReview, Agent: "codex", Title: "Feature Review", RequiresApproval: false, MaxRetries: 1},
			{StepType: StepTypeReview, Agent: "codex", Title: "Final Review", RequiresApproval: false, MaxRetries: 1},
			{StepType: StepTypeDocs, Agent: "claude-code", Title: "Documentation", RequiresApproval: false, MaxRetries: 1},
		},
	}
}

// WorkflowFromDB converts a database Workflow to the domain model.
func WorkflowFromDB(w db.Workflow) Workflow {
	wf := Workflow{
		ID:               w.ID,
		Title:            w.Title,
		State:            WorkflowState(w.State),
		CurrentStepIndex: int(w.CurrentStepIndex),
		CreatedAt:        time.Unix(w.CreatedAt, 0),
		UpdatedAt:        time.Unix(w.UpdatedAt, 0),
	}
	if w.ParentSessionID.Valid {
		wf.ParentSessionID = w.ParentSessionID.String
	}
	if w.PlanJson.Valid {
		wf.PlanJSON = w.PlanJson.String
	}
	if w.ConfigJson.Valid {
		wf.ConfigJSON = w.ConfigJson.String
	}
	if w.ErrorMessage.Valid {
		wf.ErrorMessage = w.ErrorMessage.String
	}
	if w.CompletedAt.Valid {
		t := time.Unix(w.CompletedAt.Int64, 0)
		wf.CompletedAt = &t
	}
	return wf
}

// StepFromDB converts a database WorkflowStep to the domain model.
func StepFromDB(s db.WorkflowStep) Step {
	step := Step{
		ID:               s.ID,
		WorkflowID:       s.WorkflowID,
		StepIndex:        int(s.StepIndex),
		StepType:         StepType(s.StepType),
		Agent:            s.Agent,
		Status:           StepStatus(s.Status),
		RetryCount:       int(s.RetryCount),
		MaxRetries:       int(s.MaxRetries),
		RequiresApproval: s.RequiresApproval != 0,
		CreatedAt:        time.Unix(s.CreatedAt, 0),
		UpdatedAt:        time.Unix(s.UpdatedAt, 0),
	}
	if s.AgentSessionID.Valid {
		step.AgentSessionID = s.AgentSessionID.String
	}
	if s.Title.Valid {
		step.Title = s.Title.String
	}
	if s.InputContextJson.Valid {
		step.InputContextJSON = s.InputContextJson.String
	}
	if s.OutputJson.Valid {
		step.OutputJSON = s.OutputJson.String
	}
	if s.ReviewResultJson.Valid {
		step.ReviewResultJSON = s.ReviewResultJson.String
	}
	if s.ApprovalStatus.Valid {
		step.ApprovalStatus = ApprovalStatus(s.ApprovalStatus.String)
	}
	if s.ErrorMessage.Valid {
		step.ErrorMessage = s.ErrorMessage.String
	}
	if s.StartedAt.Valid {
		t := time.Unix(s.StartedAt.Int64, 0)
		step.StartedAt = &t
	}
	if s.CompletedAt.Valid {
		t := time.Unix(s.CompletedAt.Int64, 0)
		step.CompletedAt = &t
	}
	return step
}

// MarshalConfig serializes a WorkflowConfig to JSON.
func MarshalConfig(cfg WorkflowConfig) (string, error) {
	data, err := json.Marshal(cfg)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// UnmarshalConfig deserializes a WorkflowConfig from JSON.
func UnmarshalConfig(data string) (WorkflowConfig, error) {
	var cfg WorkflowConfig
	if err := json.Unmarshal([]byte(data), &cfg); err != nil {
		return WorkflowConfig{}, err
	}
	return cfg, nil
}
