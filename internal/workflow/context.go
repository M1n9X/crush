// Package workflow provides context building for workflow step execution.
package workflow

import (
	"context"
	"encoding/json"
	"slices"
	"strings"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/diff"
	"github.com/charmbracelet/crush/internal/history"
	"github.com/charmbracelet/crush/internal/projectdoc"
)

// StepContextConfig defines token budgets per step type.
type StepContextConfig struct {
	PlanTokenBudget   int `json:"plan_token_budget"`
	CodeTokenBudget   int `json:"code_token_budget"`
	ReviewTokenBudget int `json:"review_token_budget"`
	DocsTokenBudget   int `json:"docs_token_budget"`
	MaxDiffLines      int `json:"max_diff_lines"`
}

// DefaultStepContextConfig returns the default context configuration.
func DefaultStepContextConfig() StepContextConfig {
	return StepContextConfig{
		PlanTokenBudget:   4000,
		CodeTokenBudget:   8000,
		ReviewTokenBudget: 6000,
		DocsTokenBudget:   4000,
		MaxDiffLines:      500,
	}
}

// Context represents the assembled context for a workflow step.
type Context struct {
	PlanFragment string   `json:"plan_fragment,omitempty"`
	Diffs        []Diff   `json:"diffs,omitempty"`
	TestOutput   string   `json:"test_output,omitempty"`
	RelatedFiles []string `json:"related_files,omitempty"`
	ProjectDocs  []string `json:"project_docs,omitempty"`
	TokenBudget  int      `json:"token_budget"`
	TokensUsed   int      `json:"tokens_used"`
}

// Diff represents a file change with line limits applied.
type Diff struct {
	Path      string `json:"path"`
	Content   string `json:"content"`
	Additions int    `json:"additions"`
	Removals  int    `json:"removals"`
	Truncated bool   `json:"truncated"`
}

// ContextBuilder constructs minimal review context for workflow steps.
type ContextBuilder struct {
	historyService   history.Service
	config           *config.Config
	diffGenerator    func(before, after, filename string) (string, int, int)
	projectDocLoader func(cfg *config.Config) ([]projectdoc.Doc, []string)
}

// NewContextBuilder creates a new ContextBuilder with the given dependencies.
func NewContextBuilder(historyService history.Service, cfg *config.Config) *ContextBuilder {
	return &ContextBuilder{
		historyService:   historyService,
		config:           cfg,
		diffGenerator:    diff.GenerateDiff,
		projectDocLoader: projectdoc.Load,
	}
}

// Build constructs context for the given step based on step type and workflow state.
func (cb *ContextBuilder) Build(ctx context.Context, step Step, workflow Workflow, cfg StepContextConfig) (*Context, error) {
	budget := cb.getTokenBudget(step.StepType, cfg)
	result := &Context{
		TokenBudget: budget,
	}

	remaining := budget

	// 1. Extract plan fragment from workflow
	if workflow.PlanJSON != "" {
		fragment := cb.extractPlanFragment(workflow.PlanJSON, step)
		if len(fragment) > 0 {
			tokensUsed := cb.estimateTokens(fragment)
			if tokensUsed <= remaining {
				result.PlanFragment = fragment
				remaining -= tokensUsed
			} else {
				// Truncate to fit budget
				result.PlanFragment = cb.truncateToTokens(fragment, remaining/2)
				remaining = remaining / 2
			}
		}
	}

	// 2. Load project docs (AGENTS.md etc.)
	if cb.config != nil && remaining > 0 {
		docs, _ := cb.projectDocLoader(cb.config)
		for _, doc := range docs {
			tokensUsed := cb.estimateTokens(doc.Content)
			if tokensUsed <= remaining {
				result.ProjectDocs = append(result.ProjectDocs, doc.Content)
				remaining -= tokensUsed
			}
		}
	}

	// 3. Get file history and generate diffs
	if cb.historyService != nil && workflow.ParentSessionID != "" && remaining > 0 {
		files, err := cb.historyService.ListBySession(ctx, workflow.ParentSessionID)
		if err == nil {
			type filePair struct {
				Latest    history.File
				HasLatest bool
				Previous  history.File
				HasPrev   bool
			}

			isAfter := func(a, b history.File) bool {
				if a.Version != b.Version {
					return a.Version > b.Version
				}
				return a.CreatedAt > b.CreatedAt
			}

			byPath := make(map[string]filePair)
			for _, file := range files {
				pair := byPath[file.Path]

				if !pair.HasLatest {
					pair.Latest = file
					pair.HasLatest = true
					byPath[file.Path] = pair
					continue
				}

				if isAfter(file, pair.Latest) {
					if pair.HasLatest {
						pair.Previous = pair.Latest
						pair.HasPrev = true
					}
					pair.Latest = file
					pair.HasLatest = true
					byPath[file.Path] = pair
					continue
				}

				if !pair.HasPrev || isAfter(file, pair.Previous) {
					pair.Previous = file
					pair.HasPrev = true
					byPath[file.Path] = pair
				}
			}

			paths := make([]string, 0, len(byPath))
			for path := range byPath {
				paths = append(paths, path)
			}
			slices.Sort(paths)

			for _, path := range paths {
				pair := byPath[path]
				result.RelatedFiles = append(result.RelatedFiles, path)

				// Generate diffs only for review steps (budgeted).
				if remaining > 0 && step.StepType == StepTypeReview && pair.HasLatest {
					before := ""
					if pair.HasPrev {
						before = pair.Previous.Content
					}
					diffContent, additions, removals := cb.diffGenerator(before, pair.Latest.Content, path)
					if diffContent != "" {
						d := Diff{
							Path:      path,
							Additions: additions,
							Removals:  removals,
						}

						// Apply line limits
						lines := strings.Split(diffContent, "\n")
						if len(lines) > cfg.MaxDiffLines {
							d.Content = strings.Join(lines[:cfg.MaxDiffLines], "\n")
							d.Truncated = true
						} else {
							d.Content = diffContent
						}

						tokensUsed := cb.estimateTokens(d.Content)
						if tokensUsed <= remaining {
							result.Diffs = append(result.Diffs, d)
							remaining -= tokensUsed
						}
					}
				}
			}
		}
	}

	result.TokensUsed = budget - remaining
	return result, nil
}

// BuildJSON constructs context and returns it as JSON bytes.
func (cb *ContextBuilder) BuildJSON(ctx context.Context, step Step, workflow Workflow, cfg StepContextConfig) (json.RawMessage, error) {
	c, err := cb.Build(ctx, step, workflow, cfg)
	if err != nil {
		return nil, err
	}
	return json.Marshal(c)
}

// getTokenBudget returns the appropriate token budget for the step type.
func (cb *ContextBuilder) getTokenBudget(stepType StepType, cfg StepContextConfig) int {
	switch stepType {
	case StepTypePlan:
		return cfg.PlanTokenBudget
	case StepTypeCode:
		return cfg.CodeTokenBudget
	case StepTypeReview:
		return cfg.ReviewTokenBudget
	case StepTypeDocs:
		return cfg.DocsTokenBudget
	default:
		return cfg.CodeTokenBudget
	}
}

// extractPlanFragment extracts the relevant portion of the plan for the current step.
func (cb *ContextBuilder) extractPlanFragment(planJSON string, step Step) string {
	// Try to parse as a structured plan
	var plan struct {
		Title string `json:"title"`
		Plan  string `json:"plan"`
		Steps []struct {
			Title    string `json:"title"`
			StepType string `json:"step_type"`
		} `json:"steps"`
	}
	if err := json.Unmarshal([]byte(planJSON), &plan); err != nil {
		// Return raw plan if not structured
		return planJSON
	}

	// Build a focused fragment
	var sb strings.Builder
	if plan.Title != "" {
		sb.WriteString("# ")
		sb.WriteString(plan.Title)
		sb.WriteString("\n\n")
	}
	if plan.Plan != "" {
		sb.WriteString(plan.Plan)
		sb.WriteString("\n\n")
	}

	// Highlight current step
	if step.StepIndex < len(plan.Steps) {
		sb.WriteString("## Current Step: ")
		sb.WriteString(plan.Steps[step.StepIndex].Title)
		sb.WriteString(" (")
		sb.WriteString(plan.Steps[step.StepIndex].StepType)
		sb.WriteString(")\n")
	}

	return sb.String()
}

// estimateTokens provides a rough token estimate (1 token ≈ 4 chars).
func (cb *ContextBuilder) estimateTokens(s string) int {
	return (len(s) + 3) / 4
}

// truncateToTokens truncates a string to approximately the given token count.
func (cb *ContextBuilder) truncateToTokens(s string, tokens int) string {
	maxChars := tokens * 4
	if len(s) <= maxChars {
		return s
	}
	return s[:maxChars] + "\n... [truncated]"
}
