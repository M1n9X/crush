package workflow

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
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
	queries        *db.Queries
	registry       *subagent.Registry
	events         *pubsub.Broker[WorkflowEvent]
	contextBuilder *ContextBuilder
	safetyService  *SafetyService
	contextConfig  StepContextConfig
	mu             sync.RWMutex
}

// EngineOption configures the workflow engine.
type EngineOption func(*Engine)

// WithContextBuilder sets the context builder for the engine.
func WithContextBuilder(cb *ContextBuilder) EngineOption {
	return func(e *Engine) {
		e.contextBuilder = cb
	}
}

// WithSafetyService sets the safety service for the engine.
func WithSafetyService(ss *SafetyService) EngineOption {
	return func(e *Engine) {
		e.safetyService = ss
	}
}

// WithContextConfig sets the context configuration for the engine.
func WithContextConfig(cfg StepContextConfig) EngineOption {
	return func(e *Engine) {
		e.contextConfig = cfg
	}
}

// NewEngine creates a new workflow engine.
func NewEngine(queries *db.Queries, registry *subagent.Registry, opts ...EngineOption) *Engine {
	e := &Engine{
		queries:       queries,
		registry:      registry,
		events:        pubsub.NewBroker[WorkflowEvent](),
		contextConfig: DefaultStepContextConfig(),
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

func deriveSandbox(profile *subagent.Profile) subagent.SandboxMode {
	if profile == nil {
		return subagent.SandboxReadOnly
	}
	switch strings.ToLower(strings.TrimSpace(profile.Permissions.Edit)) {
	case "allow", "ask":
		return subagent.SandboxWorkspaceWrite
	case "danger", "full":
		return subagent.SandboxDangerFullAccess
	default:
		return subagent.SandboxReadOnly
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

// CreateFromSpec creates a new workflow from a WorkflowSpec (DAG mode).
func (e *Engine) CreateFromSpec(ctx context.Context, spec *WorkflowSpec, opts CreateOptions) (*Workflow, error) {
	if err := spec.Validate(); err != nil {
		return nil, fmt.Errorf("validate spec: %w", err)
	}

	specJSON, err := MarshalSpecJSON(spec)
	if err != nil {
		return nil, fmt.Errorf("marshal spec: %w", err)
	}

	startNode := spec.GetStartNode()
	if startNode == nil {
		return nil, ErrNoStartNode
	}

	workflowID := uuid.New().String()
	dbWorkflow, err := e.queries.CreateWorkflowWithSpec(ctx, db.CreateWorkflowWithSpecParams{
		ID:               workflowID,
		ParentSessionID:  sql.NullString{String: opts.ParentSessionID, Valid: opts.ParentSessionID != ""},
		Title:            opts.Title,
		State:            string(WorkflowStateDraft),
		PlanJson:         sql.NullString{},
		ConfigJson:       sql.NullString{},
		SpecJson:         sql.NullString{String: specJSON, Valid: true},
		CurrentStepIndex: 0,
		CurrentNodeID:    sql.NullString{String: startNode.ID, Valid: true},
		ErrorMessage:     sql.NullString{},
	})
	if err != nil {
		return nil, fmt.Errorf("create workflow: %w", err)
	}

	// Create workflow steps from spec nodes
	for i, node := range spec.Nodes {
		if node.IsTerminal() {
			continue // Skip terminal nodes
		}

		agent, err := e.resolveAgentForNode(&node)
		if err != nil {
			return nil, fmt.Errorf("resolve agent for node %q: %w", node.ID, err)
		}

		stepID := uuid.New().String()
		maxRetries := int64(1)
		if node.OnError != nil && node.OnError.Retry > 0 {
			maxRetries = int64(node.OnError.Retry)
		}

		_, err = e.queries.CreateWorkflowStepWithNode(ctx, db.CreateWorkflowStepWithNodeParams{
			ID:               stepID,
			WorkflowID:       workflowID,
			StepIndex:        int64(i),
			StepType:         getStepTypeFromCapabilities(node.Capabilities),
			Agent:            agent,
			AgentSessionID:   sql.NullString{},
			Status:           string(StepStatusPending),
			Title:            sql.NullString{String: node.Name, Valid: node.Name != ""},
			InputContextJson: sql.NullString{},
			OutputJson:       sql.NullString{},
			ReviewResultJson: sql.NullString{},
			RetryCount:       0,
			MaxRetries:       maxRetries,
			RequiresApproval: boolToInt64(node.RequiresApproval),
			ApprovalStatus:   sql.NullString{},
			ErrorMessage:     sql.NullString{},
			NodeID:           sql.NullString{String: node.ID, Valid: true},
		})
		if err != nil {
			return nil, fmt.Errorf("create step for node %q: %w", node.ID, err)
		}
	}

	workflow := WorkflowFromDB(dbWorkflow)
	return &workflow, nil
}

// resolveAgentForNode finds the best agent for a node based on capabilities.
func (e *Engine) resolveAgentForNode(node *NodeSpec) (string, error) {
	if e.registry == nil {
		if len(node.PreferredAgents) > 0 {
			return node.PreferredAgents[0], nil
		}
		return "", ErrNoSubagentAvailable
	}

	// Try preferred agents first
	for _, agentName := range node.PreferredAgents {
		reg, err := e.registry.Resolve(agentName)
		if err == nil && hasCapabilities(reg, node.Capabilities) {
			return agentName, nil
		}
	}

	// Fallback to any agent with matching capabilities
	for _, reg := range e.registry.List() {
		if hasCapabilities(reg, node.Capabilities) {
			return reg.Profile.Name, nil
		}
	}

	// Last resort: use first preferred agent if available
	if len(node.PreferredAgents) > 0 {
		return node.PreferredAgents[0], nil
	}

	return "", fmt.Errorf("%w: no agent matches capabilities %v", ErrNoSubagentAvailable, node.Capabilities)
}

// hasCapabilities checks if a registration has all required capabilities.
func hasCapabilities(reg subagent.Registration, required []string) bool {
	caps := make(map[string]bool)
	for _, c := range reg.Agent.Capabilities() {
		caps[string(c)] = true
	}
	for _, r := range required {
		if !caps[r] {
			return false
		}
	}
	return true
}

// getStepTypeFromCapabilities infers step type from capabilities.
func getStepTypeFromCapabilities(caps []string) string {
	for _, c := range caps {
		switch c {
		case "plan":
			return string(StepTypePlan)
		case "code":
			return string(StepTypeCode)
		case "review":
			return string(StepTypeReview)
		case "docs":
			return string(StepTypeDocs)
		}
	}
	if len(caps) > 0 {
		return caps[0]
	}
	return string(StepTypeCode)
}

// Run starts or resumes workflow execution.
func (e *Engine) Run(ctx context.Context, workflowID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	dbWorkflow, err := e.queries.GetWorkflowByID(ctx, workflowID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrWorkflowNotFound
		}
		return fmt.Errorf("get workflow: %w", err)
	}

	// Validate state transition
	switch WorkflowState(dbWorkflow.State) {
	case WorkflowStateDraft, WorkflowStatePaused, WorkflowStateInterrupted:
		// Valid states to start/resume from
	case WorkflowStateRunning:
		return nil // Already running
	case WorkflowStateCompleted, WorkflowStateFailed:
		return ErrInvalidState
	case WorkflowStateWaitingInput:
		step, err := e.getCurrentStep(ctx, dbWorkflow)
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
				// Sequential workflows treat rejection as terminal failure; DAG workflows may have on_reject transitions.
				if !e.isDAGWorkflow(dbWorkflow) {
					return e.failWorkflow(ctx, workflowID, "step approval rejected")
				}
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

func (e *Engine) isDAGWorkflow(w db.Workflow) bool {
	return w.SpecJson.Valid && w.SpecJson.String != ""
}

func (e *Engine) getCurrentStep(ctx context.Context, w db.Workflow) (db.WorkflowStep, error) {
	if e.isDAGWorkflow(w) {
		return e.queries.GetCurrentDAGStep(ctx, w.ID)
	}
	return e.queries.GetCurrentWorkflowStep(ctx, w.ID)
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

		workflow, err := e.queries.GetWorkflowByID(ctx, workflowID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrWorkflowNotFound
			}
			return fmt.Errorf("get workflow: %w", err)
		}

		if e.isDAGWorkflow(workflow) {
			done, err := e.runDAGTick(ctx, workflow)
			if err != nil {
				return err
			}
			if done {
				return nil
			}
			continue
		}

		// --- Sequential mode ---

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

func (e *Engine) runDAGTick(ctx context.Context, workflow db.Workflow) (bool, error) {
	spec, err := ParseSpecJSON([]byte(workflow.SpecJson.String))
	if err != nil {
		return true, e.failWorkflow(ctx, workflow.ID, fmt.Sprintf("invalid workflow spec_json: %v", err))
	}

	currentNodeID := ""
	if workflow.CurrentNodeID.Valid {
		currentNodeID = workflow.CurrentNodeID.String
	}
	if currentNodeID == "" {
		start := spec.GetStartNode()
		if start == nil {
			return true, e.failWorkflow(ctx, workflow.ID, ErrNoStartNode.Error())
		}
		_, err := e.queries.UpdateWorkflowCurrentNode(ctx, db.UpdateWorkflowCurrentNodeParams{
			CurrentNodeID: sql.NullString{String: start.ID, Valid: true},
			ID:            workflow.ID,
		})
		if err != nil {
			return true, fmt.Errorf("set start node: %w", err)
		}
		return false, nil
	}

	node := spec.GetNode(currentNodeID)
	if node == nil {
		return true, e.failWorkflow(ctx, workflow.ID, fmt.Sprintf("current node %q not found in workflow spec", currentNodeID))
	}
	if node.Terminal == "success" {
		return true, e.completeWorkflow(ctx, workflow.ID)
	}
	if node.Terminal == "fail" {
		return true, e.failWorkflow(ctx, workflow.ID, fmt.Sprintf("reached terminal fail node %q", currentNodeID))
	}

	step, err := e.queries.GetCurrentDAGStep(ctx, workflow.ID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return true, e.failWorkflow(ctx, workflow.ID, fmt.Sprintf("no step found for current node %q", currentNodeID))
		}
		return true, fmt.Errorf("get current DAG step: %w", err)
	}

	// If step requires approval and is waiting, handle approval result before executing.
	if step.RequiresApproval != 0 && step.Status == string(StepStatusWaiting) {
		if !step.ApprovalStatus.Valid || step.ApprovalStatus.String == string(ApprovalPending) {
			_, _ = e.queries.UpdateWorkflowState(ctx, db.UpdateWorkflowStateParams{
				State:        string(WorkflowStateWaitingInput),
				ErrorMessage: sql.NullString{},
				ID:           workflow.ID,
			})
			return true, ErrApprovalRequired
		}

		if step.ApprovalStatus.String == string(ApprovalRejected) {
			next := spec.GetNextNodes(currentNodeID, StepStatusCompleted, false)
			if len(next) == 0 {
				return true, e.failWorkflow(ctx, workflow.ID, "step approval rejected")
			}
			done, err := e.transitionToDAGNode(ctx, workflow.ID, spec, next[0])
			if err != nil {
				return true, err
			}
			return done, nil
		}
	}

	// If the step already has a terminal status (e.g., after interruption), advance without re-executing.
	switch StepStatus(step.Status) {
	case StepStatusCompleted, StepStatusFailed:
		approved := step.ApprovalStatus.Valid && step.ApprovalStatus.String == string(ApprovalApproved)
		next := spec.GetNextNodes(currentNodeID, StepStatus(step.Status), approved)
		if len(next) == 0 {
			if StepStatus(step.Status) == StepStatusCompleted {
				return true, e.completeWorkflow(ctx, workflow.ID)
			}
			errMsg := "step failed"
			if step.ErrorMessage.Valid && step.ErrorMessage.String != "" {
				errMsg = step.ErrorMessage.String
			}
			return true, e.failWorkflow(ctx, workflow.ID, errMsg)
		}
		done, err := e.transitionToDAGNode(ctx, workflow.ID, spec, next[0])
		if err != nil {
			return true, err
		}
		return done, nil
	}

	// Execute the step
	err = e.executeStep(ctx, workflow.ID, step)
	if err != nil {
		if errors.Is(err, ErrApprovalRequired) {
			_, _ = e.queries.UpdateWorkflowState(ctx, db.UpdateWorkflowStateParams{
				State:        string(WorkflowStateWaitingInput),
				ErrorMessage: sql.NullString{},
				ID:           workflow.ID,
			})
			return true, ErrApprovalRequired
		}

		// Retry if allowed
		if step.RetryCount < step.MaxRetries {
			_, _ = e.queries.IncrementStepRetry(ctx, step.ID)
			return false, nil
		}

		next := spec.GetNextNodes(currentNodeID, StepStatusFailed, false)
		if len(next) == 0 {
			return true, e.failWorkflow(ctx, workflow.ID, err.Error())
		}
		done, terr := e.transitionToDAGNode(ctx, workflow.ID, spec, next[0])
		if terr != nil {
			return true, terr
		}
		return done, nil
	}

	approved := step.ApprovalStatus.Valid && step.ApprovalStatus.String == string(ApprovalApproved)
	next := spec.GetNextNodes(currentNodeID, StepStatusCompleted, approved)
	if len(next) == 0 {
		return true, e.completeWorkflow(ctx, workflow.ID)
	}

	done, err := e.transitionToDAGNode(ctx, workflow.ID, spec, next[0])
	if err != nil {
		return true, err
	}
	return done, nil
}

func (e *Engine) transitionToDAGNode(ctx context.Context, workflowID string, spec *WorkflowSpec, nextNodeID string) (bool, error) {
	node := spec.GetNode(nextNodeID)
	if node == nil {
		return true, e.failWorkflow(ctx, workflowID, fmt.Sprintf("next node %q not found in workflow spec", nextNodeID))
	}

	if node.Terminal == "success" {
		return true, e.completeWorkflow(ctx, workflowID)
	}
	if node.Terminal == "fail" {
		return true, e.failWorkflow(ctx, workflowID, fmt.Sprintf("reached terminal fail node %q", nextNodeID))
	}

	nextStep, err := e.queries.GetWorkflowStepByNodeID(ctx, db.GetWorkflowStepByNodeIDParams{
		WorkflowID: workflowID,
		NodeID:     sql.NullString{String: nextNodeID, Valid: true},
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return true, e.failWorkflow(ctx, workflowID, fmt.Sprintf("no step found for next node %q", nextNodeID))
		}
		return true, fmt.Errorf("get step for next node: %w", err)
	}

	if err := e.queries.ResetWorkflowStepRuntime(ctx, nextStep.ID); err != nil {
		return true, fmt.Errorf("reset step for node %q: %w", nextNodeID, err)
	}

	if _, err := e.queries.UpdateWorkflowCurrentNode(ctx, db.UpdateWorkflowCurrentNodeParams{
		CurrentNodeID: sql.NullString{String: nextNodeID, Valid: true},
		ID:            workflowID,
	}); err != nil {
		return true, fmt.Errorf("update current node: %w", err)
	}

	// Keep current_step_index roughly aligned with the node's configured step_index for UI/debugging.
	if _, err := e.queries.UpdateWorkflowStep(ctx, db.UpdateWorkflowStepParams{
		CurrentStepIndex: nextStep.StepIndex,
		ID:               workflowID,
	}); err != nil {
		return true, fmt.Errorf("update current step index: %w", err)
	}

	return false, nil
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

	if e.registry == nil {
		return ErrNoSubagentAvailable
	}

	reg, err := e.registry.Resolve(step.Agent)
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
		return fmt.Errorf("%w: %s", ErrNoSubagentAvailable, step.Agent)
	}
	if reg.Agent == nil {
		err := fmt.Errorf("%w: %s has no agent", ErrNoSubagentAvailable, step.Agent)
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

	req := subagent.Request{
		Task:    step.Title.String,
		Sandbox: deriveSandbox(&reg.Profile),
		Profile: &reg.Profile,
	}

	// Build context for the step if ContextBuilder is configured, and persist it.
	if e.contextBuilder != nil {
		workflow, wfErr := e.queries.GetWorkflowByID(ctx, step.WorkflowID)
		if wfErr == nil {
			stepContext, ctxErr := e.contextBuilder.Build(
				ctx,
				StepFromDB(step),
				WorkflowFromDB(workflow),
				e.contextConfig,
			)
			if ctxErr == nil && stepContext != nil {
				if contextJSON, err := json.Marshal(stepContext); err == nil && len(contextJSON) > 0 {
					req.ContextJSON = contextJSON
					_, _ = e.queries.SetStepInputContext(ctx, db.SetStepInputContextParams{
						InputContextJson: sql.NullString{String: string(contextJSON), Valid: true},
						ID:               step.ID,
					})
				}
			}
		}
	}
	if len(req.ContextJSON) == 0 && step.InputContextJson.Valid {
		req.ContextJSON = []byte(step.InputContextJson.String)
	}

	// Safety check for sandbox mode if SafetyService is configured.
	if e.safetyService != nil {
		check := e.safetyService.EvaluateSandbox(req.Sandbox)
		if check.RequiresGate && step.RequiresApproval == 0 {
			_, _ = e.queries.SetStepRequiresApproval(ctx, db.SetStepRequiresApprovalParams{
				RequiresApproval: 1,
				ID:               step.ID,
			})
			step.RequiresApproval = 1
		}
	}

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
	result, err := reg.Agent.Execute(ctx, req)
	if err != nil {
		if errors.Is(err, ErrApprovalRequired) {
			return err
		}
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
