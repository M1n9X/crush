# Context & Safety Implementation Plan

> **Phase 4 of Orchestration Design (Section 4.6)**  
> **Date**: 2025-12-14  
> **Status**: ✅ Implemented (see Limitations)

---

## Problem Description

The Crush workflow engine currently dispatches steps to subagents but lacks:

1. **Context Building** — Intelligent construction of minimal, relevant context (plan fragments, diffs, test outputs, related files) to inject into subagent requests
2. **Safety Hooks** — Detection and gating of dangerous operations (hazardous commands, sensitive file changes, Codex full-auto mode)

Section 4.6 of the Orchestration Design specifies:

> - **ContextBuilder**: 利用 `internal/diff`、`history`、`projectdoc` 生成最小审查上下文：plan 片段、受限 diff（行数上限）、测试输出摘要、相关文件列表；按步骤类型配置 token 预算。
> - **Safety Hooks**: 危险命令/文件变更检测，permission/approval gate；Plan 审批默认开启；Codex `full-auto` 需显式批准。

---

## Limitations

1. **Diff Source** — Context diffs are derived from `internal/history` session versions (not `git diff --cached`).
2. **Test Output** — `Context.TestOutput` is reserved but not populated yet.
3. **Safety Coverage** — The engine currently gates dangerous sandbox modes; command/file-change evaluation is implemented but not yet wired into execution tools.

## Implementation Notes

1. **Token Budget Defaults** (implemented in `DefaultStepContextConfig`):
   - Plan step: 4000 tokens
   - Code step: 8000 tokens
   - Review step: 6000 tokens
   - Docs step: 4000 tokens

2. **Dangerous Command Patterns** — Implemented as a configurable set of regex patterns.

3. **Codex Full-Auto Policy** — Implemented: `danger-full-access` requires explicit approval unless enabled via `SafetyConfig.EnableCodexFullAuto`.

---

## Proposed Changes

### Component 1: ContextBuilder Service

This component generates minimal, relevant context for subagent requests based on step type and token budget.

---

#### [NEW] [context.go](file:///Users/mxue/GitRepos/Coding/crush/internal/workflow/context.go)

New file implementing the ContextBuilder service:

```go
// ContextBuilder constructs minimal review context for workflow steps.
type ContextBuilder struct {
    historyService history.Service
    diffGenerator  func(before, after, filename string) (string, int, int)
    projectDocLoader func(cfg *config.Config) ([]projectdoc.Doc, []string)
}

// Context represents the assembled context for a step.
type Context struct {
    PlanFragment    string   `json:"plan_fragment,omitempty"`
    Diffs           []Diff   `json:"diffs,omitempty"`
    TestOutput      string   `json:"test_output,omitempty"`
    RelatedFiles    []string `json:"related_files,omitempty"`
    ProjectDocs     []string `json:"project_docs,omitempty"`
    TokenBudget     int      `json:"token_budget"`
    TokensUsed      int      `json:"tokens_used"`
}

// Diff represents a file change with line limits applied.
type Diff struct {
    Path      string `json:"path"`
    Content   string `json:"content"`
    Additions int    `json:"additions"`
    Removals  int    `json:"removals"`
    Truncated bool   `json:"truncated"`
}

// StepContextConfig defines token budgets per step type.
type StepContextConfig struct {
    PlanTokenBudget   int `json:"plan_token_budget"`
    CodeTokenBudget   int `json:"code_token_budget"`
    ReviewTokenBudget int `json:"review_token_budget"`
    DocsTokenBudget   int `json:"docs_token_budget"`
    MaxDiffLines      int `json:"max_diff_lines"`
}

// Build constructs context for the given step.
func (cb *ContextBuilder) Build(ctx context.Context, step Step, workflow Workflow, cfg StepContextConfig) (*Context, error)
```

**Key Logic:**

- Extract plan fragment from `workflow.PlanJSON` relevant to current step
- Query `history.Service` for files changed in the session
- Generate diffs with line limits using `diff.GenerateDiff`
- Load project docs via `projectdoc.Load`
- Apply token budget (approximate: 1 token ≈ 4 chars)

---

#### [NEW] [context_test.go](file:///Users/mxue/GitRepos/Coding/crush/internal/workflow/context_test.go)

Unit tests for ContextBuilder:

- `TestContextBuilder_Build_PlanStep` — Verify plan fragment extraction
- `TestContextBuilder_Build_CodeStep` — Verify diff generation and file list
- `TestContextBuilder_Build_TokenBudget` — Verify truncation at budget
- `TestContextBuilder_Build_EmptyHistory` — Handle no file changes
- `TestContextBuilder_Build_DiffLineLimits` — Verify diff truncation

---

### Component 2: Safety Hooks Service

This component detects dangerous operations and provides a gate decision that the workflow engine can route through the existing step approval flow.

---

#### [NEW] [safety.go](file:///Users/mxue/GitRepos/Coding/crush/internal/workflow/safety.go)

New file implementing Safety hooks:

```go
// SafetyService detects dangerous operations and returns a gate decision.
type SafetyService struct {
    config            SafetyConfig
    dangerousPatterns []DangerousPattern
    compiledPatterns  []*regexp.Regexp
}

// DangerousPattern defines a pattern for dangerous operations.
type DangerousPattern struct {
    Name        string
    Pattern     string // Regex pattern
    Description string
    Severity    string // "high", "medium", "low"
}

// SafetyCheck represents the result of a safety evaluation.
type SafetyCheck struct {
    IsDangerous   bool
    Violations    []SafetyViolation
    RequiresGate  bool
    GateType      string // "approval", "confirmation"
}

// SafetyViolation describes a detected dangerous operation.
type SafetyViolation struct {
    PatternName string
    Match       string
    Severity    string
    Location    string
}

// EvaluateCommand checks a command for dangerous patterns.
func (ss *SafetyService) EvaluateCommand(cmd string) *SafetyCheck

// EvaluateFileChange checks file changes for sensitive paths.
func (ss *SafetyService) EvaluateFileChange(path string, isDelete bool) *SafetyCheck

// EvaluateCodexMode checks if Codex full-auto mode requires approval.
func (ss *SafetyService) EvaluateCodexMode(sandboxMode subagent.SandboxMode) *SafetyCheck

// DefaultDangerousPatterns returns the built-in dangerous patterns.
func DefaultDangerousPatterns() []DangerousPattern
```

**Default Dangerous Patterns:**

- `rm -rf /`, `sudo rm`, `chmod 777`
- `curl | sh`, `wget | bash`
- `dd if=`, `mkfs`, `fdisk`
- API key/secret exposure patterns
- `.env`, `.ssh/`, credentials file modifications

---

#### [NEW] [safety_test.go](file:///Users/mxue/GitRepos/Coding/crush/internal/workflow/safety_test.go)

Unit tests for Safety service:

- `TestSafetyService_EvaluateCommand_Safe` — Non-dangerous commands pass
- `TestSafetyService_EvaluateCommand_Dangerous` — Dangerous patterns detected
- `TestSafetyService_EvaluateFileChange_Safe` — Normal file changes pass
- `TestSafetyService_EvaluateFileChange_Sensitive` — Sensitive paths flagged
- `TestSafetyService_EvaluateCodexMode_ReadOnly` — Read-only mode passes
- `TestSafetyService_EvaluateCodexMode_FullAccess` — Full-access gates

---

### Component 3: Engine Integration

Integrate ContextBuilder and Safety hooks into the workflow engine.

---

#### [MODIFY] [engine.go](file:///Users/mxue/GitRepos/Coding/crush/internal/workflow/engine.go)

Add ContextBuilder and SafetyService to Engine struct and integrate into step execution:

**Changes:**

1. Add `contextBuilder *ContextBuilder` and `safetyService *SafetyService` fields to `Engine` struct
2. Modify `dispatchToSubagent` to build context via ContextBuilder before dispatch
3. Derive `req.Sandbox` from the subagent profile permissions and gate dangerous sandbox modes via step approval
4. Store generated context in `workflow_steps.input_context_json`

```diff
 type Engine struct {
     queries  *db.Queries
     registry *subagent.Registry
     events   *pubsub.Broker[WorkflowEvent]
+    contextBuilder *ContextBuilder
+    safetyService  *SafetyService
     mu       sync.RWMutex
 }

 func (e *Engine) executeStep(ctx context.Context, workflowID string, step db.WorkflowStep) error {
     // ... StartWorkflowStep / publish StepStartedEvent ...
+
+    // Build request (sandbox derived from profile permissions)
+    reg, _ := e.registry.Resolve(step.Agent)
+    req := subagent.Request{
+        Task:    step.Title.String,
+        Sandbox: deriveSandbox(&reg.Profile),
+        Profile: &reg.Profile,
+    }
+
+    // Build and persist input context (workflow_steps.input_context_json)
+    if e.contextBuilder != nil {
+        // stepContext := e.contextBuilder.Build(...)
+        // req.ContextJSON = json.Marshal(stepContext)
+        // e.queries.SetStepInputContext(...)
+    }
+
+    // Safety gate: if sandbox is dangerous, convert into step approval
+    if e.safetyService != nil {
+        // check := e.safetyService.EvaluateSandbox(req.Sandbox)
+        // if check.RequiresGate { e.queries.SetStepRequiresApproval(...); step.RequiresApproval = 1 }
+    }
+
+    // Existing approval flow blocks execution until approved
     // ...
 }
```

---

#### [MODIFY] [types.go](file:///Users/mxue/GitRepos/Coding/crush/internal/workflow/types.go)

Add context configuration constants:

```diff
+// DefaultStepContextConfig returns the default context configuration.
+func DefaultStepContextConfig() StepContextConfig {
+    return StepContextConfig{
+        PlanTokenBudget:   4000,
+        CodeTokenBudget:   8000,
+        ReviewTokenBudget: 6000,
+        DocsTokenBudget:   4000,
+        MaxDiffLines:      500,
+    }
+}
```

---

## Verification Plan

### Automated Tests

#### Unit Tests (run with `go test`)

```bash
# Run all workflow package tests including new context/safety tests
cd /Users/mxue/GitRepos/Coding/crush
go test -v ./internal/workflow/... -run "Test(Context|Safety)"
```

**Expected Coverage:**

- `context.go` — ContextBuilder.Build with various step types
- `safety.go` — Pattern matching, file change detection, Codex mode checks

#### Existing Tests (verify no regression)

```bash
# Run full workflow test suite
cd /Users/mxue/GitRepos/Coding/crush
go test -v ./internal/workflow/...
```

**Verify these existing tests still pass:**

- `engine_test.go` — fakeSubagent dispatch
- `engine_dag_test.go` — DAG workflow execution
- `engine_approval_test.go` — Approval flow

#### Integration Tests

```bash
# Run full integration test suite
cd /Users/mxue/GitRepos/Coding/crush
go test -v ./... -tags=integration
```

### Manual Verification

> [!NOTE]
> Manual verification requires a running crush instance and actual subagent backends.

1. **Context Building Verification**
   - Create a workflow with plan/code steps
   - Verify `input_context_json` in workflow_steps table contains expected context
   - Confirm token budget is respected (context truncated if over budget)

2. **Safety Hooks Verification**
   - Attempt to run a workflow step that triggers dangerous pattern
   - Verify approval gate is triggered
   - Confirm Codex `danger-full-access` mode requires explicit approval

---

## Implementation Order

1. **Phase 1**: Create `context.go` and `context_test.go`
2. **Phase 2**: Create `safety.go` and `safety_test.go`  
3. **Phase 3**: Modify `engine.go` to integrate ContextBuilder and SafetyService
4. **Phase 4**: Run tests and verify no regressions
5. **Phase 5**: Create documentation in `wiki/workflow`

---

## Related Files

| File | Purpose |
|------|---------|
| [engine.go](file:///Users/mxue/GitRepos/Coding/crush/internal/workflow/engine.go) | Main workflow engine, step dispatch |
| [types.go](file:///Users/mxue/GitRepos/Coding/crush/internal/workflow/types.go) | Workflow/Step domain types |
| [diff.go](file:///Users/mxue/GitRepos/Coding/crush/internal/diff/diff.go) | Unified diff generation |
| [file.go](file:///Users/mxue/GitRepos/Coding/crush/internal/history/file.go) | File history service |
| [projectdoc.go](file:///Users/mxue/GitRepos/Coding/crush/internal/projectdoc/projectdoc.go) | AGENTS.md discovery |
| [permission.go](file:///Users/mxue/GitRepos/Coding/crush/internal/permission/permission.go) | Permission/approval service |

---

## References

- [Orchestration Design 4.6](file:///Users/mxue/GitRepos/Coding/crush/wiki/workflow/00_Orchestration_Design.md) — Context & Safety specification
- [HumanLayer Session Manager](file:///Users/mxue/GitRepos/Coding/humanlayer/hld/session/manager.go) — Context persistence patterns
- [Subagent Types](file:///Users/mxue/GitRepos/Coding/crush/internal/subagent/types.go) — Request.ContextJSON field
