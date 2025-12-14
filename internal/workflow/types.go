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
	SpecJSON         string
	CurrentNodeID    string
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
	NodeID           string
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
	if w.SpecJson.Valid {
		wf.SpecJSON = w.SpecJson.String
	}
	if w.CurrentNodeID.Valid {
		wf.CurrentNodeID = w.CurrentNodeID.String
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
	if s.NodeID.Valid {
		step.NodeID = s.NodeID.String
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

// =============================================================================
// Info Types (External API View) - Aligned with HumanLayer Pattern
// =============================================================================

// WorkflowInfo provides a JSON-safe view of the workflow for API responses.
// This is the external representation with computed fields for display.
type WorkflowInfo struct {
	ID               string        `json:"id"`
	ParentSessionID  string        `json:"parent_session_id,omitempty"`
	Title            string        `json:"title"`
	State            WorkflowState `json:"state"`
	CurrentStepIndex int           `json:"current_step_index"`
	CurrentNodeID    string        `json:"current_node_id,omitempty"`
	TotalSteps       int           `json:"total_steps"`
	CompletedSteps   int           `json:"completed_steps"`
	Progress         float64       `json:"progress"` // 0.0 - 1.0
	ErrorMessage     string        `json:"error_message,omitempty"`
	CreatedAt        time.Time     `json:"created_at"`
	UpdatedAt        time.Time     `json:"updated_at"`
	CompletedAt      *time.Time    `json:"completed_at,omitempty"`
	Steps            []StepInfo    `json:"steps,omitempty"`
}

// StepInfo provides a JSON-safe view of the step for API responses.
type StepInfo struct {
	ID               string         `json:"id"`
	WorkflowID       string         `json:"workflow_id"`
	StepIndex        int            `json:"step_index"`
	StepType         StepType       `json:"step_type"`
	Agent            string         `json:"agent"`
	AgentSessionID   string         `json:"agent_session_id,omitempty"`
	Status           StepStatus     `json:"status"`
	Title            string         `json:"title"`
	NodeID           string         `json:"node_id,omitempty"`
	RetryCount       int            `json:"retry_count"`
	MaxRetries       int            `json:"max_retries"`
	RequiresApproval bool           `json:"requires_approval"`
	ApprovalStatus   ApprovalStatus `json:"approval_status,omitempty"`
	ErrorMessage     string         `json:"error_message,omitempty"`
	CreatedAt        time.Time      `json:"created_at"`
	StartedAt        *time.Time     `json:"started_at,omitempty"`
	CompletedAt      *time.Time     `json:"completed_at,omitempty"`
}

// =============================================================================
// Update Types (Partial Update Patch) - Aligned with HumanLayer Pattern
// =============================================================================

// WorkflowUpdate contains fields that can be updated.
// nil pointer means no update; non-nil means update to the pointed value.
type WorkflowUpdate struct {
	Title            *string        `json:"title,omitempty"`
	State            *WorkflowState `json:"state,omitempty"`
	CurrentStepIndex *int           `json:"current_step_index,omitempty"`
	CurrentNodeID    *string        `json:"current_node_id,omitempty"`
	PlanJSON         *string        `json:"plan_json,omitempty"`
	ConfigJSON       *string        `json:"config_json,omitempty"`
	SpecJSON         *string        `json:"spec_json,omitempty"`
	ErrorMessage     *string        `json:"error_message,omitempty"`
	CompletedAt      *time.Time     `json:"completed_at,omitempty"`
}

// StepUpdate contains fields that can be updated for a step.
// nil pointer means no update; non-nil means update to the pointed value.
type StepUpdate struct {
	Status           *StepStatus     `json:"status,omitempty"`
	AgentSessionID   *string         `json:"agent_session_id,omitempty"`
	InputContextJSON *string         `json:"input_context_json,omitempty"`
	OutputJSON       *string         `json:"output_json,omitempty"`
	ReviewResultJSON *string         `json:"review_result_json,omitempty"`
	RetryCount       *int            `json:"retry_count,omitempty"`
	ApprovalStatus   *ApprovalStatus `json:"approval_status,omitempty"`
	ErrorMessage     *string         `json:"error_message,omitempty"`
	StartedAt        *time.Time      `json:"started_at,omitempty"`
	CompletedAt      *time.Time      `json:"completed_at,omitempty"`
}

// =============================================================================
// Conversion Functions
// =============================================================================

// WorkflowToInfo converts a Workflow domain model to the API-safe Info view.
func WorkflowToInfo(w Workflow) WorkflowInfo {
	info := WorkflowInfo{
		ID:               w.ID,
		ParentSessionID:  w.ParentSessionID,
		Title:            w.Title,
		State:            w.State,
		CurrentStepIndex: w.CurrentStepIndex,
		CurrentNodeID:    w.CurrentNodeID,
		TotalSteps:       len(w.Steps),
		ErrorMessage:     w.ErrorMessage,
		CreatedAt:        w.CreatedAt,
		UpdatedAt:        w.UpdatedAt,
		CompletedAt:      w.CompletedAt,
	}

	// Calculate completed steps and progress
	completedCount := 0
	for _, step := range w.Steps {
		if step.Status == StepStatusCompleted || step.Status == StepStatusSkipped {
			completedCount++
		}
	}
	info.CompletedSteps = completedCount

	if info.TotalSteps > 0 {
		info.Progress = float64(completedCount) / float64(info.TotalSteps)
	}

	// Convert steps to StepInfo
	if len(w.Steps) > 0 {
		info.Steps = make([]StepInfo, len(w.Steps))
		for i, step := range w.Steps {
			info.Steps[i] = StepToInfo(step)
		}
	}

	return info
}

// StepToInfo converts a Step domain model to the API-safe Info view.
func StepToInfo(s Step) StepInfo {
	return StepInfo{
		ID:               s.ID,
		WorkflowID:       s.WorkflowID,
		StepIndex:        s.StepIndex,
		StepType:         s.StepType,
		Agent:            s.Agent,
		AgentSessionID:   s.AgentSessionID,
		Status:           s.Status,
		Title:            s.Title,
		NodeID:           s.NodeID,
		RetryCount:       s.RetryCount,
		MaxRetries:       s.MaxRetries,
		RequiresApproval: s.RequiresApproval,
		ApprovalStatus:   s.ApprovalStatus,
		ErrorMessage:     s.ErrorMessage,
		CreatedAt:        s.CreatedAt,
		StartedAt:        s.StartedAt,
		CompletedAt:      s.CompletedAt,
	}
}

// ApplyWorkflowUpdate applies a WorkflowUpdate to a Workflow, returning the modified workflow.
func ApplyWorkflowUpdate(w Workflow, update WorkflowUpdate) Workflow {
	if update.Title != nil {
		w.Title = *update.Title
	}
	if update.State != nil {
		w.State = *update.State
	}
	if update.CurrentStepIndex != nil {
		w.CurrentStepIndex = *update.CurrentStepIndex
	}
	if update.CurrentNodeID != nil {
		w.CurrentNodeID = *update.CurrentNodeID
	}
	if update.PlanJSON != nil {
		w.PlanJSON = *update.PlanJSON
	}
	if update.ConfigJSON != nil {
		w.ConfigJSON = *update.ConfigJSON
	}
	if update.SpecJSON != nil {
		w.SpecJSON = *update.SpecJSON
	}
	if update.ErrorMessage != nil {
		w.ErrorMessage = *update.ErrorMessage
	}
	if update.CompletedAt != nil {
		w.CompletedAt = update.CompletedAt
	}
	w.UpdatedAt = time.Now()
	return w
}

// ApplyStepUpdate applies a StepUpdate to a Step, returning the modified step.
func ApplyStepUpdate(s Step, update StepUpdate) Step {
	if update.Status != nil {
		s.Status = *update.Status
	}
	if update.AgentSessionID != nil {
		s.AgentSessionID = *update.AgentSessionID
	}
	if update.InputContextJSON != nil {
		s.InputContextJSON = *update.InputContextJSON
	}
	if update.OutputJSON != nil {
		s.OutputJSON = *update.OutputJSON
	}
	if update.ReviewResultJSON != nil {
		s.ReviewResultJSON = *update.ReviewResultJSON
	}
	if update.RetryCount != nil {
		s.RetryCount = *update.RetryCount
	}
	if update.ApprovalStatus != nil {
		s.ApprovalStatus = *update.ApprovalStatus
	}
	if update.ErrorMessage != nil {
		s.ErrorMessage = *update.ErrorMessage
	}
	if update.StartedAt != nil {
		s.StartedAt = update.StartedAt
	}
	if update.CompletedAt != nil {
		s.CompletedAt = update.CompletedAt
	}
	s.UpdatedAt = time.Now()
	return s
}

// IsEmpty returns true if no fields are set in the WorkflowUpdate.
func (u WorkflowUpdate) IsEmpty() bool {
	return u.Title == nil &&
		u.State == nil &&
		u.CurrentStepIndex == nil &&
		u.CurrentNodeID == nil &&
		u.PlanJSON == nil &&
		u.ConfigJSON == nil &&
		u.SpecJSON == nil &&
		u.ErrorMessage == nil &&
		u.CompletedAt == nil
}

// IsEmpty returns true if no fields are set in the StepUpdate.
func (u StepUpdate) IsEmpty() bool {
	return u.Status == nil &&
		u.AgentSessionID == nil &&
		u.InputContextJSON == nil &&
		u.OutputJSON == nil &&
		u.ReviewResultJSON == nil &&
		u.RetryCount == nil &&
		u.ApprovalStatus == nil &&
		u.ErrorMessage == nil &&
		u.StartedAt == nil &&
		u.CompletedAt == nil
}
