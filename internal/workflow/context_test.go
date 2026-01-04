package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/history"
	"github.com/charmbracelet/crush/internal/projectdoc"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/stretchr/testify/require"
)

type fakeHistoryService struct {
	files []history.File
}

func (f *fakeHistoryService) Subscribe(ctx context.Context) <-chan pubsub.Event[history.File] {
	ch := make(chan pubsub.Event[history.File])
	close(ch)
	return ch
}

func (f *fakeHistoryService) Create(ctx context.Context, sessionID, path, content string) (history.File, error) {
	return history.File{}, errors.New("not implemented")
}

func (f *fakeHistoryService) CreateVersion(ctx context.Context, sessionID, path, content string) (history.File, error) {
	return history.File{}, errors.New("not implemented")
}

func (f *fakeHistoryService) Get(ctx context.Context, id string) (history.File, error) {
	return history.File{}, errors.New("not implemented")
}

func (f *fakeHistoryService) GetByPathAndSession(ctx context.Context, path, sessionID string) (history.File, error) {
	return history.File{}, errors.New("not implemented")
}

func (f *fakeHistoryService) ListBySession(ctx context.Context, sessionID string) ([]history.File, error) {
	return f.files, nil
}

func (f *fakeHistoryService) ListLatestSessionFiles(ctx context.Context, sessionID string) ([]history.File, error) {
	return nil, errors.New("not implemented")
}

func (f *fakeHistoryService) Delete(ctx context.Context, id string) error {
	return errors.New("not implemented")
}

func (f *fakeHistoryService) DeleteSessionFiles(ctx context.Context, sessionID string) error {
	return errors.New("not implemented")
}

func TestDefaultStepContextConfig(t *testing.T) {
	cfg := DefaultStepContextConfig()

	require.Equal(t, 4000, cfg.PlanTokenBudget)
	require.Equal(t, 8000, cfg.CodeTokenBudget)
	require.Equal(t, 6000, cfg.ReviewTokenBudget)
	require.Equal(t, 4000, cfg.DocsTokenBudget)
	require.Equal(t, 500, cfg.MaxDiffLines)
}

func TestContextBuilder_Build_PlanStep(t *testing.T) {
	cb := &ContextBuilder{
		diffGenerator: func(before, after, filename string) (string, int, int) {
			return "", 0, 0
		},
		projectDocLoader: func(cfg *config.Config) ([]projectdoc.Doc, []string) {
			return nil, nil
		},
	}

	step := Step{
		StepType:  StepTypePlan,
		StepIndex: 0,
	}

	workflow := Workflow{
		PlanJSON: `{"title":"Test Plan","plan":"This is a test plan","steps":[{"title":"Planning","step_type":"plan"}]}`,
	}

	ctx := context.Background()
	result, err := cb.Build(ctx, step, workflow, DefaultStepContextConfig())

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 4000, result.TokenBudget)
	require.Contains(t, result.PlanFragment, "Test Plan")
	require.Contains(t, result.PlanFragment, "This is a test plan")
}

func TestContextBuilder_Build_CodeStep(t *testing.T) {
	cb := &ContextBuilder{
		diffGenerator: func(before, after, filename string) (string, int, int) {
			return "+added line\n-removed line", 1, 1
		},
		projectDocLoader: func(cfg *config.Config) ([]projectdoc.Doc, []string) {
			return nil, nil
		},
	}

	step := Step{
		StepType:  StepTypeCode,
		StepIndex: 1,
	}

	workflow := Workflow{
		PlanJSON: `{"title":"Code Task"}`,
	}

	ctx := context.Background()
	result, err := cb.Build(ctx, step, workflow, DefaultStepContextConfig())

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 8000, result.TokenBudget)
}

func TestContextBuilder_Build_ReviewStep(t *testing.T) {
	cb := &ContextBuilder{
		diffGenerator: func(before, after, filename string) (string, int, int) {
			return "+new code\n-old code", 1, 1
		},
		projectDocLoader: func(cfg *config.Config) ([]projectdoc.Doc, []string) {
			return nil, nil
		},
	}

	step := Step{
		StepType:  StepTypeReview,
		StepIndex: 2,
	}

	workflow := Workflow{
		PlanJSON: `{"title":"Review Task"}`,
	}

	ctx := context.Background()
	result, err := cb.Build(ctx, step, workflow, DefaultStepContextConfig())

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 6000, result.TokenBudget)
}

func TestContextBuilder_Build_EmptyWorkflow(t *testing.T) {
	cb := &ContextBuilder{
		diffGenerator: func(before, after, filename string) (string, int, int) {
			return "", 0, 0
		},
		projectDocLoader: func(cfg *config.Config) ([]projectdoc.Doc, []string) {
			return nil, nil
		},
	}

	step := Step{
		StepType:  StepTypeCode,
		StepIndex: 0,
	}

	workflow := Workflow{}

	ctx := context.Background()
	result, err := cb.Build(ctx, step, workflow, DefaultStepContextConfig())

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "", result.PlanFragment)
	require.Empty(t, result.Diffs)
	require.Empty(t, result.RelatedFiles)
}

func TestContextBuilder_estimateTokens(t *testing.T) {
	cb := &ContextBuilder{}

	// 1 token ≈ 4 chars
	require.Equal(t, 1, cb.estimateTokens("abc"))
	require.Equal(t, 1, cb.estimateTokens("abcd"))
	require.Equal(t, 2, cb.estimateTokens("abcde"))
	require.Equal(t, 25, cb.estimateTokens("This is a test string that is 100 characters long approximately for testing token estimation logic."))
}

func TestContextBuilder_truncateToTokens(t *testing.T) {
	cb := &ContextBuilder{}

	// Short string should not be truncated
	short := "Hello"
	require.Equal(t, short, cb.truncateToTokens(short, 10))

	// Long string should be truncated
	long := "This is a very long string that should be truncated when the token limit is low"
	result := cb.truncateToTokens(long, 5)
	require.Contains(t, result, "... [truncated]")
	require.Less(t, len(result), len(long)+20) // Allow for truncation suffix
}

func TestContextBuilder_getTokenBudget(t *testing.T) {
	cb := &ContextBuilder{}
	cfg := DefaultStepContextConfig()

	require.Equal(t, 4000, cb.getTokenBudget(StepTypePlan, cfg))
	require.Equal(t, 8000, cb.getTokenBudget(StepTypeCode, cfg))
	require.Equal(t, 6000, cb.getTokenBudget(StepTypeReview, cfg))
	require.Equal(t, 4000, cb.getTokenBudget(StepTypeDocs, cfg))
	require.Equal(t, 8000, cb.getTokenBudget("unknown", cfg)) // Default to code
}

func TestContextBuilder_extractPlanFragment(t *testing.T) {
	cb := &ContextBuilder{}

	t.Run("structured plan", func(t *testing.T) {
		planJSON := `{"title":"My Plan","plan":"Do the thing","steps":[{"title":"Step 1","step_type":"plan"},{"title":"Step 2","step_type":"code"}]}`
		step := Step{StepIndex: 1}

		fragment := cb.extractPlanFragment(planJSON, step)
		require.Contains(t, fragment, "My Plan")
		require.Contains(t, fragment, "Do the thing")
		require.Contains(t, fragment, "Step 2")
		require.Contains(t, fragment, "code")
	})

	t.Run("raw plan", func(t *testing.T) {
		rawPlan := "Just a simple text plan"
		step := Step{StepIndex: 0}

		fragment := cb.extractPlanFragment(rawPlan, step)
		require.Equal(t, rawPlan, fragment)
	})
}

func TestContext_JSON_Serialization(t *testing.T) {
	ctx := Context{
		PlanFragment: "Test plan",
		Diffs: []Diff{
			{Path: "file.go", Content: "+line", Additions: 1, Removals: 0},
		},
		RelatedFiles: []string{"file1.go", "file2.go"},
		TokenBudget:  8000,
		TokensUsed:   500,
	}

	result, err := json.Marshal(ctx)
	require.NoError(t, err)
	require.Contains(t, string(result), "plan_fragment")
	require.Contains(t, string(result), "Test plan")
}

func TestContextBuilder_Build_ReviewStep_UsesLatestTwoVersions(t *testing.T) {
	var gotBefore string
	var gotAfter string
	var gotFilename string

	cb := &ContextBuilder{
		historyService: &fakeHistoryService{
			files: []history.File{
				{SessionID: "s1", Path: "main.go", Content: "old", Version: 1, CreatedAt: 1},
				{SessionID: "s1", Path: "main.go", Content: "new", Version: 2, CreatedAt: 2},
			},
		},
		diffGenerator: func(before, after, filename string) (string, int, int) {
			gotBefore = before
			gotAfter = after
			gotFilename = filename
			return "+new\n-old", 1, 1
		},
		projectDocLoader: func(cfg *config.Config) ([]projectdoc.Doc, []string) {
			return nil, nil
		},
	}

	step := Step{
		StepType:  StepTypeReview,
		StepIndex: 0,
	}

	workflow := Workflow{
		ParentSessionID: "s1",
	}

	result, err := cb.Build(context.Background(), step, workflow, DefaultStepContextConfig())
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "old", gotBefore)
	require.Equal(t, "new", gotAfter)
	require.Equal(t, "main.go", gotFilename)
	require.Contains(t, result.RelatedFiles, "main.go")
	require.Len(t, result.Diffs, 1)
	require.Equal(t, "main.go", result.Diffs[0].Path)
}
