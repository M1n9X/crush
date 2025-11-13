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
	"github.com/humanlayer/humanlayer/claudecode-go"
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
	LastResult      *claudecode.Result
	CreatedAt       time.Time
	UpdatedAt       time.Time
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

	// Build session config with streaming
	config := claudecode.SessionConfig{
		Query:        query,
		WorkingDir:   o.workingDir,
		MaxTurns:     DefaultMaxTurns,
		SessionID:    session.ClaudeSessionID,
		OutputFormat: claudecode.OutputStreamJSON, // Use streaming
		Verbose:      true,
	}

	// Create Claude Code client
	client, err := claudecode.NewClient()
	if err != nil {
		o.logger.Printf("[Orchestrator] Failed to create client: %v", err)
		interaction.Error = fmt.Sprintf("Failed to create client: %v", err)
		interaction.IsError = true
		return interaction, false
	}

	// Launch session
	o.logger.Printf("[Orchestrator] Launching Claude Code session")
	startTime := time.Now()

	claudeSession, err := client.Launch(config)
	if err != nil {
		o.logger.Printf("[Orchestrator] Failed to launch session: %v", err)
		interaction.Error = fmt.Sprintf("Failed to launch session: %v", err)
		interaction.IsError = true
		return interaction, false
	}

	// Save session ID if it's a new session
	if session.ClaudeSessionID == "" {
		session.ClaudeSessionID = claudeSession.ID
		o.logger.Printf("[Orchestrator] Created new Claude session: %s", session.ClaudeSessionID)
	}

	// Start goroutine to read and render events
	eventWg := sync.WaitGroup{}
	eventWg.Add(1)
	go func() {
		defer eventWg.Done()
		for {
			select {
			case event, ok := <-claudeSession.Events:
				if !ok {
					return
				}
				renderer.RenderEvent(event)
			case <-ctx.Done():
				return
			}
		}
	}()

	// Wait for completion with timeout
	result, err := o.waitWithTimeout(ctx, claudeSession)

	// Wait for event processing to complete
	eventWg.Wait()

	duration := time.Since(startTime)

	if err != nil {
		o.logger.Printf("[Orchestrator] Session failed: %v", err)
		interaction.Error = fmt.Sprintf("Session failed: %v", err)
		interaction.IsError = true
		interaction.DurationMS = int(duration.Milliseconds())
		return interaction, false
	}

	// Record result
	interaction.Result = result.Result
	interaction.CostUSD = result.CostUSD
	interaction.DurationMS = result.DurationMS
	interaction.NumTurns = result.NumTurns
	interaction.IsError = result.IsError
	if result.IsError {
		interaction.Error = result.Error
	}

	// Store result in session
	session.LastResult = result
	session.UpdatedAt = time.Now()

	o.logger.Printf("[Orchestrator] Step %d completed: cost=$%.4f, turns=%d, duration=%v",
		step, result.CostUSD, result.NumTurns, duration)

	// Render summary
	renderer.RenderSummary(result.CostUSD, duration, result.NumTurns)

	// Analyze result and decide next action
	analysis := o.analyzeResult(result, err)
	shouldContinue := o.shouldContinue(session, analysis)

	if shouldContinue {
		o.logger.Printf("[Orchestrator] Decided to continue to next step")
	}

	return interaction, shouldContinue
}

// analyzeResult analyzes the execution result
func (o *ClaudeCodeOrchestrator) analyzeResult(
	result *claudecode.Result,
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

		// Check for permission denials
		if pd := result.PermissionDenials; pd != nil && len(pd.Denials) > 0 {
			analysis.PermissionsDenied = pd.Denials
			analysis.NeedPermissionAdjustment = true
		}
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
	PermissionsDenied        []claudecode.PermissionDenial
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

// waitWithTimeout waits for Claude Code session completion with timeout support
func (o *ClaudeCodeOrchestrator) waitWithTimeout(
	ctx context.Context,
	claudeSession *claudecode.Session,
) (*claudecode.Result, error) {
	// Create timeout context
	ctx, cancel := context.WithTimeout(ctx, DefaultSessionTimeout)
	defer cancel()

	// Channels for communication
	resultChan := make(chan *claudecode.Result, 1)
	errChan := make(chan error, 1)

	// Start Wait in a goroutine
	go func() {
		result, err := claudeSession.Wait()
		if err != nil {
			errChan <- err
		} else {
			resultChan <- result
		}
	}()

	// Wait for completion, error, or timeout
	select {
	case result := <-resultChan:
		return result, nil
	case err := <-errChan:
		return nil, err
	case <-ctx.Done():
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("claude code session timed out after %v", DefaultSessionTimeout)
		}
		return nil, fmt.Errorf("claude code session cancelled: %v", ctx.Err())
	}
}
