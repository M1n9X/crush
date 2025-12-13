package workflow

import (
	"context"
	"database/sql"
	"testing"

	"github.com/charmbracelet/crush/internal/db"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/subagent"
	"github.com/stretchr/testify/require"
)

// fakeSubagent implements a mock subagent for testing
type fakeSubagent struct {
	name         string
	capabilities []subagent.Capability
	result       *subagent.Result
	err          error
	executeCalls int
}

func (f *fakeSubagent) Name() string {
	return f.name
}

func (f *fakeSubagent) Capabilities() []subagent.Capability {
	return f.capabilities
}

func (f *fakeSubagent) SupportsResume() bool {
	return true
}

func (f *fakeSubagent) Execute(ctx context.Context, req subagent.Request) (*subagent.Result, error) {
	f.executeCalls++
	if f.err != nil {
		return nil, f.err
	}
	return f.result, nil
}

func (f *fakeSubagent) Resume(ctx context.Context, resumeToken string, req subagent.Request) (*subagent.Result, error) {
	return f.Execute(ctx, req)
}

func (f *fakeSubagent) ExecuteStreamed(ctx context.Context, req subagent.Request, handler func(subagent.Event)) (*subagent.Result, error) {
	return f.Execute(ctx, req)
}

// fakeQueries implements a subset of db.Queries for testing
type fakeQueries struct {
	workflows map[string]db.Workflow
	steps     map[string][]db.WorkflowStep
	stepsById map[string]db.WorkflowStep
	createErr error
	getErr    error
}

func newFakeQueries() *fakeQueries {
	return &fakeQueries{
		workflows: make(map[string]db.Workflow),
		steps:     make(map[string][]db.WorkflowStep),
		stepsById: make(map[string]db.WorkflowStep),
	}
}

func TestNewEngine(t *testing.T) {
	registry := subagent.NewRegistry(nil)
	engine := NewEngine(nil, registry)

	require.NotNil(t, engine)
	require.NotNil(t, engine.Events())
}

func TestEngineEventsSubscription(t *testing.T) {
	registry := subagent.NewRegistry(nil)
	engine := NewEngine(nil, registry)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	events := engine.Events().Subscribe(ctx)
	require.NotNil(t, events)

	// Publish an event and verify it's received
	engine.events.Publish(pubsub.WorkflowStartedEvent, WorkflowEvent{
		WorkflowID: "test-wf",
		State:      WorkflowStateRunning,
	})

	select {
	case event := <-events:
		require.Equal(t, pubsub.WorkflowStartedEvent, event.Type)
		require.Equal(t, "test-wf", event.Payload.WorkflowID)
		require.Equal(t, WorkflowStateRunning, event.Payload.State)
	default:
		t.Fatal("expected event not received")
	}
}

func TestCreateOptions(t *testing.T) {
	opts := CreateOptions{
		Title:           "Test Workflow",
		ParentSessionID: "session-123",
		Config: WorkflowConfig{
			Title: "Test",
			Plan:  "Test plan",
			Steps: []StepConfig{
				{StepType: StepTypePlan, Agent: "claude-code"},
			},
		},
	}

	require.Equal(t, "Test Workflow", opts.Title)
	require.Equal(t, "session-123", opts.ParentSessionID)
	require.Len(t, opts.Config.Steps, 1)
}

func TestWorkflowEventType(t *testing.T) {
	event := WorkflowEvent{
		WorkflowID: "wf-1",
		StepID:     "step-1",
		StepIndex:  0,
		EventType:  pubsub.StepStartedEvent,
		State:      WorkflowStateRunning,
		StepStatus: StepStatusRunning,
	}

	require.Equal(t, "wf-1", event.WorkflowID)
	require.Equal(t, "step-1", event.StepID)
	require.Equal(t, pubsub.StepStartedEvent, event.EventType)
}

func TestBoolToInt64(t *testing.T) {
	require.Equal(t, int64(1), boolToInt64(true))
	require.Equal(t, int64(0), boolToInt64(false))
}

func TestWorkflowFromDB(t *testing.T) {
	dbWorkflow := db.Workflow{
		ID:               "wf-123",
		ParentSessionID:  sql.NullString{String: "session-456", Valid: true},
		Title:            "Test Workflow",
		State:            "running",
		PlanJson:         sql.NullString{String: `{"plan":"test"}`, Valid: true},
		ConfigJson:       sql.NullString{String: `{"title":"test"}`, Valid: true},
		SpecJson:         sql.NullString{String: `{"name":"spec","nodes":[]}`, Valid: true},
		CurrentNodeID:    sql.NullString{String: "plan", Valid: true},
		CurrentStepIndex: 2,
		ErrorMessage:     sql.NullString{String: "some error", Valid: true},
		CreatedAt:        1700000000,
		UpdatedAt:        1700000100,
		CompletedAt:      sql.NullInt64{Int64: 1700000200, Valid: true},
	}

	wf := WorkflowFromDB(dbWorkflow)

	require.Equal(t, "wf-123", wf.ID)
	require.Equal(t, "session-456", wf.ParentSessionID)
	require.Equal(t, "Test Workflow", wf.Title)
	require.Equal(t, WorkflowStateRunning, wf.State)
	require.Equal(t, `{"plan":"test"}`, wf.PlanJSON)
	require.Equal(t, `{"title":"test"}`, wf.ConfigJSON)
	require.Equal(t, `{"name":"spec","nodes":[]}`, wf.SpecJSON)
	require.Equal(t, "plan", wf.CurrentNodeID)
	require.Equal(t, 2, wf.CurrentStepIndex)
	require.Equal(t, "some error", wf.ErrorMessage)
	require.NotNil(t, wf.CompletedAt)
}

func TestWorkflowFromDBWithNulls(t *testing.T) {
	dbWorkflow := db.Workflow{
		ID:               "wf-123",
		ParentSessionID:  sql.NullString{Valid: false},
		Title:            "Test",
		State:            "draft",
		PlanJson:         sql.NullString{Valid: false},
		ConfigJson:       sql.NullString{Valid: false},
		CurrentStepIndex: 0,
		ErrorMessage:     sql.NullString{Valid: false},
		CreatedAt:        1700000000,
		UpdatedAt:        1700000100,
		CompletedAt:      sql.NullInt64{Valid: false},
	}

	wf := WorkflowFromDB(dbWorkflow)

	require.Equal(t, "wf-123", wf.ID)
	require.Equal(t, "", wf.ParentSessionID)
	require.Equal(t, "", wf.PlanJSON)
	require.Equal(t, "", wf.ConfigJSON)
	require.Equal(t, "", wf.ErrorMessage)
	require.Nil(t, wf.CompletedAt)
}

func TestStepFromDB(t *testing.T) {
	dbStep := db.WorkflowStep{
		ID:               "step-123",
		WorkflowID:       "wf-456",
		StepIndex:        1,
		StepType:         "code",
		Agent:            "claude-code",
		AgentSessionID:   sql.NullString{String: "agent-session-789", Valid: true},
		Status:           "running",
		Title:            sql.NullString{String: "Coding Step", Valid: true},
		InputContextJson: sql.NullString{String: `{"context":"test"}`, Valid: true},
		OutputJson:       sql.NullString{String: `{"output":"done"}`, Valid: true},
		ReviewResultJson: sql.NullString{String: `{"approved":true}`, Valid: true},
		RetryCount:       1,
		MaxRetries:       3,
		RequiresApproval: 1,
		ApprovalStatus:   sql.NullString{String: "approved", Valid: true},
		ErrorMessage:     sql.NullString{String: "some step error", Valid: true},
		NodeID:           sql.NullString{String: "coding", Valid: true},
		CreatedAt:        1700000000,
		UpdatedAt:        1700000100,
		StartedAt:        sql.NullInt64{Int64: 1700000050, Valid: true},
		CompletedAt:      sql.NullInt64{Int64: 1700000150, Valid: true},
	}

	step := StepFromDB(dbStep)

	require.Equal(t, "step-123", step.ID)
	require.Equal(t, "wf-456", step.WorkflowID)
	require.Equal(t, 1, step.StepIndex)
	require.Equal(t, StepTypeCode, step.StepType)
	require.Equal(t, "claude-code", step.Agent)
	require.Equal(t, "agent-session-789", step.AgentSessionID)
	require.Equal(t, StepStatusRunning, step.Status)
	require.Equal(t, "Coding Step", step.Title)
	require.Equal(t, `{"context":"test"}`, step.InputContextJSON)
	require.Equal(t, `{"output":"done"}`, step.OutputJSON)
	require.Equal(t, `{"approved":true}`, step.ReviewResultJSON)
	require.Equal(t, 1, step.RetryCount)
	require.Equal(t, 3, step.MaxRetries)
	require.True(t, step.RequiresApproval)
	require.Equal(t, ApprovalApproved, step.ApprovalStatus)
	require.Equal(t, "some step error", step.ErrorMessage)
	require.Equal(t, "coding", step.NodeID)
	require.NotNil(t, step.StartedAt)
	require.NotNil(t, step.CompletedAt)
}

func TestStepFromDBWithNulls(t *testing.T) {
	dbStep := db.WorkflowStep{
		ID:               "step-123",
		WorkflowID:       "wf-456",
		StepIndex:        0,
		StepType:         "plan",
		Agent:            "codex",
		AgentSessionID:   sql.NullString{Valid: false},
		Status:           "pending",
		Title:            sql.NullString{Valid: false},
		InputContextJson: sql.NullString{Valid: false},
		OutputJson:       sql.NullString{Valid: false},
		ReviewResultJson: sql.NullString{Valid: false},
		RetryCount:       0,
		MaxRetries:       2,
		RequiresApproval: 0,
		ApprovalStatus:   sql.NullString{Valid: false},
		ErrorMessage:     sql.NullString{Valid: false},
		CreatedAt:        1700000000,
		UpdatedAt:        1700000100,
		StartedAt:        sql.NullInt64{Valid: false},
		CompletedAt:      sql.NullInt64{Valid: false},
	}

	step := StepFromDB(dbStep)

	require.Equal(t, "step-123", step.ID)
	require.Equal(t, "", step.AgentSessionID)
	require.Equal(t, "", step.Title)
	require.Equal(t, "", step.InputContextJSON)
	require.Equal(t, "", step.OutputJSON)
	require.False(t, step.RequiresApproval)
	require.Equal(t, ApprovalStatus(""), step.ApprovalStatus)
	require.Equal(t, "", step.ErrorMessage)
	require.Nil(t, step.StartedAt)
	require.Nil(t, step.CompletedAt)
}

func TestErrConstants(t *testing.T) {
	require.Error(t, ErrWorkflowNotFound)
	require.Error(t, ErrStepNotFound)
	require.Error(t, ErrInvalidState)
	require.Error(t, ErrApprovalRequired)
	require.Error(t, ErrNoSubagentAvailable)

	require.Contains(t, ErrWorkflowNotFound.Error(), "workflow not found")
	require.Contains(t, ErrStepNotFound.Error(), "step not found")
	require.Contains(t, ErrInvalidState.Error(), "invalid")
	require.Contains(t, ErrApprovalRequired.Error(), "approval")
	require.Contains(t, ErrNoSubagentAvailable.Error(), "subagent")
}

func TestHasCapabilities(t *testing.T) {
	agent := &fakeSubagent{
		name:         "test-agent",
		capabilities: []subagent.Capability{"plan", "code", "review"},
	}
	reg := subagent.Registration{
		Profile: subagent.Profile{Name: "test"},
		Agent:   agent,
	}

	t.Run("has all capabilities", func(t *testing.T) {
		require.True(t, hasCapabilities(reg, []string{"plan", "code"}))
	})

	t.Run("missing capability", func(t *testing.T) {
		require.False(t, hasCapabilities(reg, []string{"plan", "docs"}))
	})

	t.Run("empty required", func(t *testing.T) {
		require.True(t, hasCapabilities(reg, []string{}))
	})
}

func TestGetStepTypeFromCapabilities(t *testing.T) {
	require.Equal(t, string(StepTypePlan), getStepTypeFromCapabilities([]string{"plan"}))
	require.Equal(t, string(StepTypeCode), getStepTypeFromCapabilities([]string{"code"}))
	require.Equal(t, string(StepTypeReview), getStepTypeFromCapabilities([]string{"review"}))
	require.Equal(t, string(StepTypeDocs), getStepTypeFromCapabilities([]string{"docs"}))
	require.Equal(t, "brainstorm", getStepTypeFromCapabilities([]string{"brainstorm"}))
	require.Equal(t, string(StepTypeCode), getStepTypeFromCapabilities([]string{}))
}

func TestResolveAgentForNode(t *testing.T) {
	engine := NewEngine(nil, nil)

	t.Run("no registry uses preferred agent", func(t *testing.T) {
		node := &NodeSpec{
			ID:              "test",
			Capabilities:    []string{"plan"},
			PreferredAgents: []string{"claude-code"},
		}
		agent, err := engine.resolveAgentForNode(node)
		require.NoError(t, err)
		require.Equal(t, "claude-code", agent)
	})

	t.Run("no registry no preferred returns error", func(t *testing.T) {
		node := &NodeSpec{
			ID:           "test",
			Capabilities: []string{"plan"},
		}
		_, err := engine.resolveAgentForNode(node)
		require.Error(t, err)
	})
}

func TestResolveAgentWithRegistry(t *testing.T) {
	// Create agent with capabilities
	agent := &fakeSubagent{
		name:         "codex",
		capabilities: []subagent.Capability{"plan", "review"},
		result:       &subagent.Result{Text: "done"},
	}

	registry := subagent.NewRegistry(nil)
	registry.Upsert(subagent.Registration{
		Profile: subagent.Profile{Name: "codex"},
		Agent:   agent,
	})

	engine := NewEngine(nil, registry)

	t.Run("preferred agent with matching capabilities", func(t *testing.T) {
		node := &NodeSpec{
			ID:              "test",
			Capabilities:    []string{"review"},
			PreferredAgents: []string{"codex"},
		}
		agentName, err := engine.resolveAgentForNode(node)
		require.NoError(t, err)
		require.Equal(t, "codex", agentName)
	})

	t.Run("fallback to preferred when no capability match", func(t *testing.T) {
		node := &NodeSpec{
			ID:              "test",
			Capabilities:    []string{"docs"}, // codex doesn't have docs
			PreferredAgents: []string{"codex"},
		}
		agentName, err := engine.resolveAgentForNode(node)
		require.NoError(t, err)
		require.Equal(t, "codex", agentName) // Falls back to first preferred
	})
}
