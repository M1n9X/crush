package workflow

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"

	"github.com/charmbracelet/crush/internal/db"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/subagent"
	"github.com/google/uuid"
)

var (
	// ErrWorkflowNotFound is returned when a workflow is not found.
	ErrWorkflowNotFound = errors.New("workflow not found")
	// ErrStepNotFound is returned when a workflow step is not found.
	ErrStepNotFound = errors.New("workflow step not found")
	// ErrInvalidState is returned when an operation is invalid for the current state.
	ErrInvalidState = errors.New("invalid workflow state for operation")
	// ErrApprovalRequired is returned when a step requires approval to proceed.
	ErrApprovalRequired = errors.New("step requires approval")
	// ErrNoSubagentAvailable is returned when no subagent can handle the step.
	ErrNoSubagentAvailable = errors.New("no subagent available for step")
)

// WorkflowEvent represents an event in the workflow lifecycle.
type WorkflowEvent struct {
	WorkflowID string
	StepID     string
	StepIndex  int
	EventType  pubsub.EventType
	State      WorkflowState
	StepStatus StepStatus
	Error      string
}

// Engine manages workflow execution with state machine logic.
type Engine struct {
	queries  *db.Queries
	registry *subagent.Registry
	events   *pubsub.Broker[WorkflowEvent]
	mu       sync.RWMutex
}

// NewEngine creates a new workflow engine.
func NewEngine(queries *db.Queries, registry *subagent.Registry) *Engine {
	return &Engine{
		queries:  queries,
		registry: registry,
		events:   pubsub.NewBroker[WorkflowEvent](),
	}
}

// Events returns the event broker for subscribing to workflow events.
func (e *Engine) Events() *pubsub.Broker[WorkflowEvent] {
	return e.events
}

// CreateOptions defines options for creating a new workflow.
type CreateOptions struct {
	Title           string
	ParentSessionID string
	Config          WorkflowConfig
}

// Create creates a new workflow with the given configuration.
func (e *Engine) Create(ctx context.Context, opts CreateOptions) (*Workflow, error) {
	configJSON, err := MarshalConfig(opts.Config)
	if err != nil {
		return nil, fmt.Errorf("marshal config: %w", err)
	}

	workflowID := uuid.New().String()
	dbWorkflow, err := e.queries.CreateWorkflow(ctx, db.CreateWorkflowParams{
		ID:               workflowID,
		ParentSessionID:  sql.NullString{String: opts.ParentSessionID, Valid: opts.ParentSessionID != ""},
		Title:            opts.Title,
		State:            string(WorkflowStateDraft),
		PlanJson:         sql.NullString{String: opts.Config.Plan, Valid: opts.Config.Plan != ""},
		ConfigJson:       sql.NullString{String: configJSON, Valid: true},
		CurrentStepIndex: 0,
		ErrorMessage:     sql.NullString{},
	})
	if err != nil {
		return nil, fmt.Errorf("create workflow: %w", err)
	}

	// Create workflow steps
	for i, stepCfg := range opts.Config.Steps {
		stepID := uuid.New().String()
		_, err := e.queries.CreateWorkflowStep(ctx, db.CreateWorkflowStepParams{
			ID:               stepID,
			WorkflowID:       workflowID,
			StepIndex:        int64(i),
			StepType:         string(stepCfg.StepType),
			Agent:            stepCfg.Agent,
			AgentSessionID:   sql.NullString{},
			Status:           string(StepStatusPending),
			Title:            sql.NullString{String: stepCfg.Title, Valid: stepCfg.Title != ""},
			InputContextJson: sql.NullString{},
			OutputJson:       sql.NullString{},
			ReviewResultJson: sql.NullString{},
			RetryCount:       0,
			MaxRetries:       int64(stepCfg.MaxRetries),
			RequiresApproval: boolToInt64(stepCfg.RequiresApproval),
			ApprovalStatus:   sql.NullString{},
			ErrorMessage:     sql.NullString{},
		})
		if err != nil {
			return nil, fmt.Errorf("create step %d: %w", i, err)
		}
	}

	workflow := WorkflowFromDB(dbWorkflow)
	return &workflow, nil
}

// Run starts or resumes workflow execution.
func (e *Engine) Run(ctx context.Context, workflowID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	workflow, err := e.getWorkflow(ctx, workflowID)
	if err != nil {
		return err
	}

	// Validate state transition
	switch workflow.State {
	case WorkflowStateDraft, WorkflowStatePaused, WorkflowStateInterrupted:
		// Valid states to start/resume from
	case WorkflowStateRunning:
		return nil // Already running
	case WorkflowStateCompleted, WorkflowStateFailed:
		return ErrInvalidState
	case WorkflowStateWaitingInput:
		step, err := e.queries.GetCurrentWorkflowStep(ctx, workflowID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				// No steps remaining; allow runLoop to complete the workflow.
				break
			}
			return fmt.Errorf("get current step: %w", err)
		}

		if step.RequiresApproval != 0 && step.Status == string(StepStatusWaiting) {
			if !step.ApprovalStatus.Valid || step.ApprovalStatus.String == string(ApprovalPending) {
				return ErrApprovalRequired
			}
			if step.ApprovalStatus.String == string(ApprovalRejected) {
				return e.failWorkflow(ctx, workflowID, "step approval rejected")
			}
		}
	}

	// Update state to running
	_, err = e.queries.UpdateWorkflowState(ctx, db.UpdateWorkflowStateParams{
		State:        string(WorkflowStateRunning),
		ErrorMessage: sql.NullString{},
		ID:           workflowID,
	})
	if err != nil {
		return fmt.Errorf("update workflow state: %w", err)
	}

	e.publishEvent(pubsub.WorkflowStartedEvent, WorkflowEvent{
		WorkflowID: workflowID,
		State:      WorkflowStateRunning,
	})

	return e.runLoop(ctx, workflowID)
}

// runLoop executes the workflow step by step.
func (e *Engine) runLoop(ctx context.Context, workflowID string) error {
	for {
		select {
		case <-ctx.Done():
			// Mark as interrupted
			_, _ = e.queries.UpdateWorkflowState(ctx, db.UpdateWorkflowStateParams{
				State:        string(WorkflowStateInterrupted),
				ErrorMessage: sql.NullString{String: ctx.Err().Error(), Valid: true},
				ID:           workflowID,
			})
			return ctx.Err()
		default:
		}

		// Get current step
		step, err := e.queries.GetCurrentWorkflowStep(ctx, workflowID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				// No more steps - workflow complete
				return e.completeWorkflow(ctx, workflowID)
			}
			return fmt.Errorf("get current step: %w", err)
		}

		// Check if step requires approval and is waiting
		if step.RequiresApproval != 0 && step.Status == string(StepStatusWaiting) {
			if !step.ApprovalStatus.Valid || step.ApprovalStatus.String == string(ApprovalPending) {
				// Still waiting for approval
				_, _ = e.queries.UpdateWorkflowState(ctx, db.UpdateWorkflowStateParams{
					State:        string(WorkflowStateWaitingInput),
					ErrorMessage: sql.NullString{},
					ID:           workflowID,
				})
				return ErrApprovalRequired
			}
			if step.ApprovalStatus.String == string(ApprovalRejected) {
				// Approval rejected - fail workflow
				return e.failWorkflow(ctx, workflowID, "step approval rejected")
			}
		}

		// Execute the step
		err = e.executeStep(ctx, workflowID, step)
		if err != nil {
			if errors.Is(err, ErrApprovalRequired) {
				// Step is now awaiting user approval; mark workflow accordingly and stop.
				_, _ = e.queries.UpdateWorkflowState(ctx, db.UpdateWorkflowStateParams{
					State:        string(WorkflowStateWaitingInput),
					ErrorMessage: sql.NullString{},
					ID:           workflowID,
				})
				return ErrApprovalRequired
			}

			// Check if retryable
			if step.RetryCount < step.MaxRetries {
				_, _ = e.queries.IncrementStepRetry(ctx, step.ID)
				continue
			}
			return e.failWorkflow(ctx, workflowID, err.Error())
		}

		// Advance to next step
		workflow, _ := e.queries.GetWorkflowByID(ctx, workflowID)
		steps, _ := e.queries.ListWorkflowSteps(ctx, workflowID)

		nextIndex := workflow.CurrentStepIndex + 1
		if int(nextIndex) >= len(steps) {
			return e.completeWorkflow(ctx, workflowID)
		}

		_, err = e.queries.UpdateWorkflowStep(ctx, db.UpdateWorkflowStepParams{
			CurrentStepIndex: nextIndex,
			ID:               workflowID,
		})
		if err != nil {
			return fmt.Errorf("advance step: %w", err)
		}
	}
}

// executeStep executes a single workflow step.
func (e *Engine) executeStep(ctx context.Context, workflowID string, step db.WorkflowStep) error {
	// Mark step as running
	_, err := e.queries.StartWorkflowStep(ctx, step.ID)
	if err != nil {
		return fmt.Errorf("start step: %w", err)
	}

	e.publishEvent(pubsub.StepStartedEvent, WorkflowEvent{
		WorkflowID: workflowID,
		StepID:     step.ID,
		StepIndex:  int(step.StepIndex),
		StepStatus: StepStatusRunning,
	})

	// Check if approval required for this step
	if step.RequiresApproval != 0 && step.Status != string(StepStatusWaiting) {
		_, _ = e.queries.UpdateWorkflowStepStatus(ctx, db.UpdateWorkflowStepStatusParams{
			Status:       string(StepStatusWaiting),
			ErrorMessage: sql.NullString{},
			ID:           step.ID,
		})
		_, _ = e.queries.SetStepApprovalStatus(ctx, db.SetStepApprovalStatusParams{
			ApprovalStatus: sql.NullString{String: string(ApprovalPending), Valid: true},
			ID:             step.ID,
		})

		e.publishEvent(pubsub.StepWaitingEvent, WorkflowEvent{
			WorkflowID: workflowID,
			StepID:     step.ID,
			StepIndex:  int(step.StepIndex),
			StepStatus: StepStatusWaiting,
		})
		e.publishEvent(pubsub.ApprovalRequestedEvent, WorkflowEvent{
			WorkflowID: workflowID,
			StepID:     step.ID,
			StepIndex:  int(step.StepIndex),
		})

		return ErrApprovalRequired
	}

	// Dispatch to subagent
	result, err := e.dispatchToSubagent(ctx, step)
	if err != nil {
		_, _ = e.queries.FailWorkflowStep(ctx, db.FailWorkflowStepParams{
			ErrorMessage: sql.NullString{String: err.Error(), Valid: true},
			ID:           step.ID,
		})

		e.publishEvent(pubsub.StepFailedEvent, WorkflowEvent{
			WorkflowID: workflowID,
			StepID:     step.ID,
			StepIndex:  int(step.StepIndex),
			StepStatus: StepStatusFailed,
			Error:      err.Error(),
		})

		return err
	}

	// Mark step as completed
	_, err = e.queries.CompleteWorkflowStep(ctx, db.CompleteWorkflowStepParams{
		OutputJson: sql.NullString{String: result.Text, Valid: true},
		ID:         step.ID,
	})
	if err != nil {
		return fmt.Errorf("complete step: %w", err)
	}

	// Store resume token if available
	if result.ResumeToken != "" {
		_, _ = e.queries.SetStepAgentSession(ctx, db.SetStepAgentSessionParams{
			AgentSessionID: sql.NullString{String: result.ResumeToken, Valid: true},
			ID:             step.ID,
		})
	}

	e.publishEvent(pubsub.StepCompletedEvent, WorkflowEvent{
		WorkflowID: workflowID,
		StepID:     step.ID,
		StepIndex:  int(step.StepIndex),
		StepStatus: StepStatusCompleted,
	})

	return nil
}

// dispatchToSubagent calls the appropriate subagent for the step.
func (e *Engine) dispatchToSubagent(ctx context.Context, step db.WorkflowStep) (*subagent.Result, error) {
	if e.registry == nil {
		return nil, ErrNoSubagentAvailable
	}

	reg, err := e.registry.Resolve(step.Agent)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrNoSubagentAvailable, step.Agent)
	}
	if reg.Agent == nil {
		return nil, fmt.Errorf("%w: %s has no agent", ErrNoSubagentAvailable, step.Agent)
	}

	// Build request
	req := subagent.Request{
		Task:    step.Title.String,
		Sandbox: subagent.SandboxReadOnly,
		Profile: &reg.Profile,
	}
	if step.InputContextJson.Valid {
		req.ContextJSON = []byte(step.InputContextJson.String)
	}

	return reg.Agent.Execute(ctx, req)
}

// Pause pauses a running workflow.
func (e *Engine) Pause(ctx context.Context, workflowID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	workflow, err := e.getWorkflow(ctx, workflowID)
	if err != nil {
		return err
	}

	if workflow.State != WorkflowStateRunning {
		return ErrInvalidState
	}

	_, err = e.queries.UpdateWorkflowState(ctx, db.UpdateWorkflowStateParams{
		State:        string(WorkflowStatePaused),
		ErrorMessage: sql.NullString{},
		ID:           workflowID,
	})
	if err != nil {
		return fmt.Errorf("pause workflow: %w", err)
	}

	e.publishEvent(pubsub.WorkflowPausedEvent, WorkflowEvent{
		WorkflowID: workflowID,
		State:      WorkflowStatePaused,
	})

	return nil
}

// Resume resumes a paused workflow.
func (e *Engine) Resume(ctx context.Context, workflowID string) error {
	return e.Run(ctx, workflowID)
}

// Approve approves or rejects a pending step.
func (e *Engine) Approve(ctx context.Context, stepID string, approved bool) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	step, err := e.queries.GetWorkflowStepByID(ctx, stepID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrStepNotFound
		}
		return err
	}

	if step.Status != string(StepStatusWaiting) {
		return ErrInvalidState
	}

	status := ApprovalApproved
	eventType := pubsub.ApprovalGrantedEvent
	if !approved {
		status = ApprovalRejected
		eventType = pubsub.ApprovalDeniedEvent
	}

	_, err = e.queries.SetStepApprovalStatus(ctx, db.SetStepApprovalStatusParams{
		ApprovalStatus: sql.NullString{String: string(status), Valid: true},
		ID:             stepID,
	})
	if err != nil {
		return fmt.Errorf("set approval status: %w", err)
	}

	e.publishEvent(eventType, WorkflowEvent{
		WorkflowID: step.WorkflowID,
		StepID:     stepID,
		StepIndex:  int(step.StepIndex),
	})

	return nil
}

// Cancel cancels a workflow.
func (e *Engine) Cancel(ctx context.Context, workflowID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	return e.failWorkflow(ctx, workflowID, "cancelled by user")
}

// GetWorkflow returns the current state of a workflow.
func (e *Engine) GetWorkflow(ctx context.Context, workflowID string) (*Workflow, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	return e.getWorkflow(ctx, workflowID)
}

// GetSteps returns all steps for a workflow.
func (e *Engine) GetSteps(ctx context.Context, workflowID string) ([]Step, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	dbSteps, err := e.queries.ListWorkflowSteps(ctx, workflowID)
	if err != nil {
		return nil, err
	}

	steps := make([]Step, len(dbSteps))
	for i, s := range dbSteps {
		steps[i] = StepFromDB(s)
	}
	return steps, nil
}

// ListWorkflows returns all workflows.
func (e *Engine) ListWorkflows(ctx context.Context) ([]Workflow, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	dbWorkflows, err := e.queries.ListWorkflows(ctx)
	if err != nil {
		return nil, err
	}

	workflows := make([]Workflow, len(dbWorkflows))
	for i, w := range dbWorkflows {
		workflows[i] = WorkflowFromDB(w)
	}
	return workflows, nil
}

func (e *Engine) getWorkflow(ctx context.Context, workflowID string) (*Workflow, error) {
	dbWorkflow, err := e.queries.GetWorkflowByID(ctx, workflowID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrWorkflowNotFound
		}
		return nil, err
	}
	workflow := WorkflowFromDB(dbWorkflow)
	return &workflow, nil
}

func (e *Engine) completeWorkflow(ctx context.Context, workflowID string) error {
	_, err := e.queries.CompleteWorkflow(ctx, db.CompleteWorkflowParams{
		State: string(WorkflowStateCompleted),
		ID:    workflowID,
	})
	if err != nil {
		return fmt.Errorf("complete workflow: %w", err)
	}

	e.publishEvent(pubsub.WorkflowCompletedEvent, WorkflowEvent{
		WorkflowID: workflowID,
		State:      WorkflowStateCompleted,
	})

	return nil
}

func (e *Engine) failWorkflow(ctx context.Context, workflowID string, errMsg string) error {
	_, err := e.queries.UpdateWorkflowState(ctx, db.UpdateWorkflowStateParams{
		State:        string(WorkflowStateFailed),
		ErrorMessage: sql.NullString{String: errMsg, Valid: true},
		ID:           workflowID,
	})
	if err != nil {
		return fmt.Errorf("fail workflow: %w", err)
	}

	e.publishEvent(pubsub.WorkflowFailedEvent, WorkflowEvent{
		WorkflowID: workflowID,
		State:      WorkflowStateFailed,
		Error:      errMsg,
	})

	return errors.New(errMsg)
}

func (e *Engine) publishEvent(eventType pubsub.EventType, event WorkflowEvent) {
	event.EventType = eventType
	e.events.Publish(eventType, event)
}

func boolToInt64(b bool) int64 {
	if b {
		return 1
	}
	return 0
}
