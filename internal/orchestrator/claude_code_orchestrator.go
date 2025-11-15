package orchestrator

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/crush/internal/permission"
	"github.com/google/uuid"
	claude "github.com/M1n9X/claude-agent-sdk-go"
	"github.com/M1n9X/claude-agent-sdk-go/types"
)

// Default configuration constants
const (
	// DefaultMaxSteps is the default maximum number of orchestrator iterations
	DefaultMaxSteps = 3

	// DefaultMaxTurns is the default maximum number of turns per Claude Code session
	DefaultMaxTurns = 15

	// DefaultSessionTimeout is the default timeout for a single Claude Code session
	DefaultSessionTimeout = 30 * time.Minute

	//RetryDelay is the delay between orchestrator iterations to avoid rate limiting
	DefaultRetryDelay = 500 * time.Millisecond
)

// ClaudeCodeOrchestrator manages multi-turn interactions with Claude Code
type ClaudeCodeOrchestrator struct {
	// Session state management
	sessions   map[string]*OrchestratorSession
	sessionsMu sync.RWMutex

	// Dependencies
	permissions permission.Service
	workingDir  string

	// Logger
	logger *log.Logger
}

// OrchestratorSession represents a multi-turn orchestrator session
type OrchestratorSession struct {
	ID              string
	CrushSessionID  string
	ClaudeSessionID string
	CurrentStep     int
	MaxSteps        int
	History         []Interaction
	LastResult      *ClaudeResult
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// ClaudeResult represents the result from Claude Agent SDK
type ClaudeResult struct {
	Result     string
	SessionID  string
	CostUSD    float64
	DurationMS int
	NumTurns   int
	IsError    bool
	Error      string
	ModelUsage map[string]interface{}
}

// Interaction records each interaction with Claude Code
type Interaction struct {
	Step       int
	Query      string
	Result     string
	CostUSD    float64
	DurationMS int
	NumTurns   int
	Error      string
	IsError    bool
	CreatedAt  time.Time
}

// OrchestratorResult is the final result of orchestration
type OrchestratorResult struct {
	FinalResult   string
	SessionID     string
	TotalCost     float64
	TotalDuration time.Duration
	TotalTurns    int
	Iterations    int
	IsError       bool
	Error         string
	History       []Interaction
}

// OrchestratorOptions configures the orchestrator
type OrchestratorOptions struct {
	MaxSteps int
}

// NewClaudeCodeOrchestrator creates a new orchestrator
func NewClaudeCodeOrchestrator(
	permissions permission.Service,
	workingDir string,
) *ClaudeCodeOrchestrator {
	return &ClaudeCodeOrchestrator{
		sessions:    make(map[string]*OrchestratorSession),
		permissions: permissions,
		workingDir:  workingDir,
		logger:      log.Default(),
	}
}

// Run executes a multi-turn interaction with Claude Code
func (o *ClaudeCodeOrchestrator) Run(
	ctx context.Context,
	crushSessionID string,
	initialQuery string,
) (*OrchestratorResult, error) {
	// Get or create session
	session := o.getOrCreateSession(crushSessionID)

	// Log start
	o.logger.Printf("[Orchestrator] Starting session %s, step %d", session.ID, session.CurrentStep+1)

	// Execute main loop
	currentQuery := initialQuery
	for {
		// Check if we should stop
		if session.CurrentStep >= session.MaxSteps {
			o.logger.Printf("[Orchestrator] Max steps (%d) reached, stopping", session.MaxSteps)
			break
		}

		// Execute one step
		interaction, shouldContinue := o.executeStep(ctx, session, currentQuery)
		if interaction != nil {
			session.History = append(session.History, *interaction)
		}

		// If we shouldn't continue, break
		if !shouldContinue {
			break
		}

		// Increment step and continue
		session.CurrentStep++
		currentQuery = fmt.Sprintf("Continue from where you left off. Previous result was: %s", interaction.Result)

		// Delay to avoid rate limiting
		time.Sleep(DefaultRetryDelay)
	}

	// Build and return result
	return o.buildResult(session), nil
}

// executeStep performs a single step of interaction
func (o *ClaudeCodeOrchestrator) executeStep(
	ctx context.Context,
	session *OrchestratorSession,
	query string,
) (*Interaction, bool) {
	step := session.CurrentStep + 1
	o.logger.Printf("[Orchestrator] Executing step %d", step)

	// Create stream renderer for real-time output
	renderer := NewStreamRenderer()
	fmt.Printf("\n%s Step %d\n\n", colorize(brightBlue, "═══════"), step)

	// Create interaction record
	interaction := &Interaction{
		Step:      step,
		Query:     query,
		CreatedAt: time.Now(),
	}

	// Configure Claude Agent options
	opts := types.NewClaudeAgentOptions().
		WithCWD(o.workingDir).
		WithVerbose(true)
	// Do not set model - let Claude Code CLI use its default configuration
	// The model is configured in Claude Code CLI itself

	// Create Claude client
	client, err := claude.NewClient(ctx, opts)
	if err != nil {
		o.logger.Printf("[Orchestrator] Failed to create client: %v", err)
		interaction.Error = fmt.Sprintf("Failed to create client: %v", err)
		interaction.IsError = true
		return interaction, false
	}

	// Connect to Claude
	if err := client.Connect(ctx); err != nil {
		o.logger.Printf("[Orchestrator] Failed to connect to Claude: %v", err)
		interaction.Error = fmt.Sprintf("Failed to connect to Claude: %v", err)
		interaction.IsError = true
		return interaction, false
	}
	defer client.Close(ctx)

	// Start a timeout context for the operation
	ctx, cancel := context.WithTimeout(ctx, DefaultSessionTimeout)
	defer cancel()

	// Send the query to Claude
	if err := client.Query(ctx, query); err != nil {
		o.logger.Printf("[Orchestrator] Failed to send query to Claude: %v", err)
		interaction.Error = fmt.Sprintf("Failed to send query to Claude: %v", err)
		interaction.IsError = true
		return interaction, false
	}

	// Collect all responses
	var allTextContent []string
	var totalCost float64
	var numTurns int
	var isError bool
	var errorMsg string
	startTime := time.Now()

	// Receive and process the response
	for msg := range client.ReceiveResponse(ctx) {
		// Render the event
		renderer.RenderMessage(msg)
		switch m := msg.(type) {
		case *types.AssistantMessage:
			for _, block := range m.Content {
				if textBlock, ok := block.(*types.TextBlock); ok {
					allTextContent = append(allTextContent, textBlock.Text)
				}
			}
		case *types.ResultMessage:
			if m.TotalCostUSD != nil {
				totalCost = *m.TotalCostUSD
			}
			numTurns = m.NumTurns
			isError = m.IsError
			if isError && m.Result != nil {
				errorMsg = *m.Result
			}
		}
	}

	// Calculate duration
	duration := time.Since(startTime)

	// Combine all text content
	resultText := strings.Join(allTextContent, "\n")

	// Record result
	interaction.Result = resultText
	interaction.CostUSD = totalCost
	interaction.DurationMS = int(duration.Milliseconds())
	interaction.NumTurns = numTurns
	interaction.IsError = isError
	if isError {
		interaction.Error = errorMsg
	}

	// Create Claude result for storing in session
	claudeResult := &ClaudeResult{
		Result:     resultText,
		SessionID:  session.ClaudeSessionID,
		CostUSD:    totalCost,
		DurationMS: int(duration.Milliseconds()),
		NumTurns:   numTurns,
		IsError:    isError,
		Error:      errorMsg,
	}

	// Store result in session
	session.LastResult = claudeResult
	session.UpdatedAt = time.Now()

	o.logger.Printf("[Orchestrator] Step %d completed: cost=$%.4f, turns=%d, duration=%v",
		step, totalCost, numTurns, duration)

	// Render summary
	renderer.RenderSummary(totalCost, duration, numTurns)

	// Analyze result and decide next action
	analysis := o.analyzeResult(claudeResult, nil)
	shouldContinue := o.shouldContinue(session, analysis)

	if shouldContinue {
		o.logger.Printf("[Orchestrator] Decided to continue to next step")
	}

	return interaction, shouldContinue
}

// analyzeResult analyzes the execution result
func (o *ClaudeCodeOrchestrator) analyzeResult(
	result *ClaudeResult,
	err error,
) *ResultAnalysis {
	analysis := &ResultAnalysis{
		Success: err == nil && !result.IsError,
	}

	if err != nil {
		analysis.ErrorType = "execution_error"
		analysis.ShouldRetry = false
		return analysis
	}

	if result.IsError {
		analysis.ErrorType = "claude_error"
		analysis.ErrorMessage = result.Error
		// Note: Permission denials are handled differently in the new SDK
		// analysis.PermissionsDenied would be different for the new SDK
	}

	// Store basic metrics
	analysis.CostUSD = result.CostUSD
	analysis.NumTurns = result.NumTurns
	analysis.DurationMS = result.DurationMS
	analysis.ResultText = result.Result

	// Check for incomplete work indicators
	if o.detectIncompleteWork(result.Result) {
		analysis.HasIncompleteWork = true
	}

	return analysis
}

// shouldContinue decides whether to continue to next step
func (o *ClaudeCodeOrchestrator) shouldContinue(
	session *OrchestratorSession,
	analysis *ResultAnalysis,
) bool {
	// If execution failed and we can adjust permissions, continue
	if analysis.NeedPermissionAdjustment {
		o.logger.Printf("[Orchestrator] Permission adjustment needed")
		// For MVP, we'll just continue and hope the user grants permissions
		return true
	}

	// If we have incomplete work and haven't reached max steps, continue
	if analysis.HasIncompleteWork && session.CurrentStep < session.MaxSteps {
		o.logger.Printf("[Orchestrator] Incomplete work detected, continuing")
		return true
	}

	// If the result suggests more work, continue
	if analysis.Success && o.detectContinuationNeeded(analysis.ResultText) {
		o.logger.Printf("[Orchestrator] Continuation needed based on result analysis")
		return session.CurrentStep < session.MaxSteps
	}

	// Default: stop
	return false
}

// detectIncompleteWork checks if the result indicates incomplete work
func (o *ClaudeCodeOrchestrator) detectIncompleteWork(result string) bool {
	lower := strings.ToLower(result)
	indicators := []string{
		"todo",
		"not implemented",
		"placeholder",
		"incomplete",
		"finish",
		"continue",
		"next step",
		"additional work",
	}

	for _, indicator := range indicators {
		if strings.Contains(lower, indicator) {
			return true
		}
	}

	return false
}

// detectContinuationNeeded checks if continuation is needed
func (o *ClaudeCodeOrchestrator) detectContinuationNeeded(result string) bool {
	lower := strings.ToLower(result)
	indicators := []string{
		"this is just the beginning",
		"further improvements",
		"can be enhanced",
		"next steps include",
		"to complete this",
	}

	for _, indicator := range indicators {
		if strings.Contains(lower, indicator) {
			return true
		}
	}

	return false
}

// buildResult builds the final orchestrator result
func (o *ClaudeCodeOrchestrator) buildResult(session *OrchestratorSession) *OrchestratorResult {
	// Calculate totals
	totalCost := 0.0
	totalTurns := 0
	totalDuration := time.Duration(0)

	for _, interaction := range session.History {
		totalCost += interaction.CostUSD
		totalTurns += interaction.NumTurns
		totalDuration += time.Duration(interaction.DurationMS) * time.Millisecond
	}

	// Get final result from last interaction
	var finalResult string
	var isError bool
	var errorMsg string
	if len(session.History) > 0 {
		last := session.History[len(session.History)-1]
		finalResult = last.Result
		isError = last.IsError
		errorMsg = last.Error
	}

	return &OrchestratorResult{
		FinalResult:   finalResult,
		SessionID:     session.ClaudeSessionID,
		TotalCost:     totalCost,
		TotalDuration: totalDuration,
		TotalTurns:    totalTurns,
		Iterations:    len(session.History),
		IsError:       isError,
		Error:         errorMsg,
		History:       session.History,
	}
}

// getOrCreateSession gets or creates an orchestrator session
func (o *ClaudeCodeOrchestrator) getOrCreateSession(crushSessionID string) *OrchestratorSession {
	o.sessionsMu.Lock()
	defer o.sessionsMu.Unlock()

	// Try to find existing session
	if session, ok := o.sessions[crushSessionID]; ok {
		o.logger.Printf("[Orchestrator] Found existing session %s", session.ID)
		return session
	}

	// Create new session
	session := &OrchestratorSession{
		ID:             uuid.New().String(),
		CrushSessionID: crushSessionID,
		MaxSteps:       DefaultMaxSteps,
		History:        []Interaction{},
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	o.sessions[crushSessionID] = session
	o.logger.Printf("[Orchestrator] Created new session %s", session.ID)

	return session
}

// ResultAnalysis holds analysis of execution result
type ResultAnalysis struct {
	Success                  bool
	ErrorType                string
	ErrorMessage             string
	ShouldRetry              bool
	PermissionsDenied        []string // Changed from claudecode.PermissionDenial to string slice
	NeedPermissionAdjustment bool
	CostUSD                  float64
	NumTurns                 int
	DurationMS               int
	ResultText               string
	HasIncompleteWork        bool
}

// NextActionDecision decides the next action
type NextActionDecision struct {
	ShouldContinue bool
	NextQuery      string
	Adjustments    []Adjustment
}

// Adjustment represents a parameter adjustment
type Adjustment struct {
	Type   string      // "permission", "tool"
	Action string      // "grant", "add", "remove"
	Value  interface{}
}

