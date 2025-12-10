# Subagent Interface (Shared for Claude/Codex/others)

> 目标：保持 agent 无状态、可替换，Claude Code 与 Codex 共享同一接口/基类；状态由 Crush Engine/DB 持有。

---

## Interface (Go 形态)

```go
type Capability string // e.g. "plan", "code", "review", "docs", "brainstorm"

type Subagent interface {
    Name() string
    Capabilities() []Capability
    SupportsResume() bool

    Execute(ctx context.Context, req Request) (*Result, error)
    Resume(ctx context.Context, resumeToken string, req Request) (*Result, error)
}

type Request struct {
    Task        string            // 用户/上游指令
    ContextJSON json.RawMessage   // ContextBuilder 产物（最小必要上下文）
    Files       []Attachment      // 可选文件/图片
    Sandbox     SandboxMode       // read-only/full-auto 等
    Timeout     time.Duration
    Metadata    map[string]string // 调用来源、workflow/step id 等
}

type Result struct {
    Text          string
    Artifacts     []Artifact     // patches/diffs/summaries
    ResumeToken   string         // e.g. Claude session id, Codex thread id
    Usage         Usage          // tokens/cost/duration
    Observations  []string       // 审计/日志摘要
    IsRetryable   bool
    ErrorMessage  string
}

// SandboxMode 控制 Agent 的执行权限级别
type SandboxMode string
const (
    SandboxReadOnly       SandboxMode = "read-only"        // 只读，无文件/命令写入
    SandboxApprovalRequired SandboxMode = "approval-required" // 需审批才可执行写操作
    SandboxFullAuto       SandboxMode = "full-auto"        // 完全自动，需显式批准启用
)

// Attachment 复用现有 message.Attachment
type Attachment = message.Attachment  // 定义于 internal/message/attachment.go

// Usage 记录 Agent 调用的资源消耗
type Usage struct {
    PromptTokens     int64         `json:"prompt_tokens"`
    CompletionTokens int64         `json:"completion_tokens"`
    TotalTokens      int64         `json:"total_tokens"`
    CostUSD          float64       `json:"cost_usd"`
    Duration         time.Duration `json:"duration"`
    ModelName        string        `json:"model_name,omitempty"`
}

// Artifact 表示 Agent 产出的结构化产物
type Artifact struct {
    Type    string          `json:"type"`    // "patch", "diff", "summary", "plan", "code"
    Name    string          `json:"name"`
    Content json.RawMessage `json:"content"` // 类型相关的内容
}
```

### Adapter 约束

- **无状态**：不持久化任何本地状态；返回的 `ResumeToken` 由 Engine 存入 DB（workflow_steps.agent_session_id）。
- **上下文只读**：不直接读写磁盘，所有需要的上下文由 Engine/ContextBuilder 提供。
- **权限**：Sandbox/审批由 Engine 决定；适配器只执行已批准的模式。
- **观测**：stderr/stdout/事件需汇总为 `Observations`，供日志/审计；不得直接写全局日志文件。

### Capability 例子

- `plan` / `brainstorm` / `code` / `review` / `docs` / `search`.
- Registry 按 capability + `preferred_agents` 选用实现，默认继承调用者模型/权限，定义可覆盖。

---

## Claude Code Adapter (示意)

- `Capabilities`: `["plan","code","docs","review"]`（可配置）。
- `SupportsResume`: true（session id）。
- `Execute`: 调用现有 session agent；返回 resume id、usage、diff/patch summary。
- `Resume`: 复用 session id 继续对话。

## Codex Adapter (示意)

- `Capabilities`: `["review","brainstorm","plan","code"]`（可配置）。
- `SupportsResume`: true（thread id）。
- `Execute`: 使用 Codex Go SDK `Exec`/`Run`，支持 `sandbox/full-auto/enable/output-schema/plan-tool`。
- `Resume`: 调用 `resume <thread>`。
- 复用 myclaude codex-wrapper 的日志/超时/stderr 处理策略。

---

## State Ownership（关键约束）

- Engine/StateManager 持久化：workflow/steps、agent_session_id、cost/tokens、审批、events。
- Subagent 不管理状态；Resume 仅使用 Engine 注入的 `ResumeToken`。
- 所有事件通过 Engine 的 pubsub 发布；Subagent 不直接发事件。

---

## Registry/Dispatch

- Registry 维护 `Subagent` 实例，按 `capabilities`+偏好选择；若多个匹配，按优先级或可用性（健康检查/版本）。
- 支持热加载（沿用 `SubAgentWatcher`），新定义可增删 agent metadata，而无需改代码。

---

## 与现有代码的关系

### 现有 SubAgentDefinition 的处置

现有 `internal/agent/subagent_defs.go` 中的 `SubAgentDefinition` 将作为 **配置层**：

```go
// 现有结构（保留）
type SubAgentDefinition struct {
    Name         string   // → Subagent.Name()
    Description  string   // → 用于 Registry 匹配描述
    Tools        []string // → Request.Metadata["allowed_tools"]
    Wildcard     bool     // → SandboxMode 计算
    ModelName    string   // → 传入 Coordinator
    SystemPrompt string   // → Request.ContextJSON 的一部分
    Source       string   // → 用于日志/调试
    Color        string   // → TUI 展示
}
```

新的 `Subagent` 接口是 **执行层**，由 Adapter 实现：

| 配置 (SubAgentDefinition) | 执行 (Subagent Interface) |
|---------------------------|---------------------------|
| 从 `.md` 文件加载 | 运行时实例 |
| 声明式（tools, model） | 命令式（Execute/Resume） |
| `SubAgentWatcher` 热重载 | Registry 动态选择 |

### 实现路径

1. **Phase 1**: 保留现有 `SubAgentDefinition` + `subagent_tool.go`，在其上封装 `Subagent` 接口
2. **Phase 2**: 新增 `ClaudeCodeSubagent` 和 `CodexSubagent` 实现
3. **Phase 3**: 迁移 `subAgentTool` 内部逻辑到新的 Adapter 模式

### 代码映射

| 设计组件 | 现有代码位置 | 操作 |
|----------|--------------|------|
| `Subagent` interface | 无 | **新增** `internal/subagent/types.go` |
| `ClaudeCodeSubagent` | 无 | **新增** `internal/subagent/claude_code.go` |
| `CodexSubagent` | 无 | **新增** `internal/subagent/codex.go` |
| `Registry` | 无 | **新增** `internal/subagent/registry.go` |
| `SubAgentDefinition` | `internal/agent/subagent_defs.go` | **保留** |
| `loadSubAgentDefinitions` | `internal/agent/subagent_defs.go` | **保留** |
| `SubAgentWatcher` | `internal/agent/subagent_watcher.go` | **保留**，扩展通知 Registry |
| `subAgentTool` | `internal/agent/subagent_tool.go` | **重构**，调用 Subagent 接口 |
