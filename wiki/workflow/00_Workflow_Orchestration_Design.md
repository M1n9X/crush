# Crush Workflow Orchestration System

> **Version**: 1.0  
> **Date**: 2025-12-10  
> **Status**: Design Document (Pending Approval)

---

## 1. Executive Summary

### 1.1 Problem Statement

当前 Crush 项目已具备基本的 Claude Code SDK 集成能力，但缺乏**多阶段工作流编排**和**双 Agent 协作**能力。用户需要手动介入多个开发阶段，无法实现：

- Claude Code (规划/编码) + Codex (审查/验证) 的自动化协作
- 跨阶段的状态持久化与恢复
- 质量门控和自动化审查循环

### 1.2 Solution Overview

设计并实现一个**工作流编排层**，将 Claude Code 和 Codex 作为 Subagent 协调调度，支持：

1. **状态驱动的多阶段工作流** - 需求→规划→审查→编码→验证→文档
2. **智能上下文管理** - 为审查阶段构建最小必要上下文
3. **全生命周期持久化** - SQLite 存储，支持中断恢复
4. **灵活的人类介入点** - Plan Review 暂停 + 关键操作确认

---

## 2. Requirements Analysis

### 2.1 User Requirements

| ID | Requirement | Priority |
|----|-------------|----------|
| REQ-1 | Claude Code + Codex 双 Agent 协作工作流 | P0 |
| REQ-2 | Plan 阶段需要人工审批 | P0 |
| REQ-3 | 关键操作(删除等)需要确认 | P0 |
| REQ-4 | 工作流状态持久化到 SQLite | P0 |
| REQ-5 | 支持工作流中断后恢复 | P1 |
| REQ-6 | 为 Codex 构建最小化审查上下文 | P1 |
| REQ-7 | Codex 使用 CLI 调用，结构模仿 SDK | P1 |

### 2.2 Technical Constraints

- **Codex 集成**: 当前只支持 CLI 调用，未来会做成 Go SDK
- **持久化**: 使用现有 SQLite 基础设施
- **兼容性**: 不破坏现有 Crush 功能

---

## 3. Reference Analysis

### 3.1 Existing Projects Overview

| Project | Role | Key Patterns |
|---------|------|--------------|
| **crush** | Base platform | Session/Message Service, SQLite, Subagent |
| **myclaude** | Workflow reference | Dual-agent, codex-wrapper, parallel execution |
| **agentsdk-go** | SDK patterns | Model/Agent interface, middleware chain |
| **codex-mcp-server** | MCP integration | Tool exposure via MCP protocol |
| **humanlayer** | State management | Session lifecycle, ConversationStore |

### 3.2 Humanlayer Session Management (Deep Analysis)

Humanlayer 项目在状态分离方面做了成熟的设计，值得借鉴：

#### 3.2.1 Type Separation Pattern

```go
// 内部 Session (完整数据)
type Session struct {
    ID, RunID, ClaudeSessionID string
    Status                     string
    Config                     claudecode.SessionConfig
    Result                     *claudecode.Result
    // ... 30+ fields
}

// 外部 Info (JSON-safe view)
type Info struct {
    ID, RunID, ClaudeSessionID string
    Status                     Status
    StartTime, EndTime         time.Time
    Query, Summary, Title      string
    Result                     *claudecode.Result
}

// Update 结构 (部分更新)
type SessionUpdate struct {
    Status         *string
    Query          *string
    ClaudeSessionID *string
    // ... optional pointer fields
}
```

**借鉴点**: 分离存储结构、API 展示结构、增量更新结构

#### 3.2.2 Status State Machine

```go
const (
    StatusDraft        = "draft"        // 配置中
    StatusStarting     = "starting"     // 启动中
    StatusRunning      = "running"      // 运行中
    StatusCompleted    = "completed"    // 完成
    StatusFailed       = "failed"       // 失败
    StatusInterrupting = "interrupting" // 中断中
    StatusInterrupted  = "interrupted"  // 已中断(可恢复)
    StatusWaitingInput = "waiting_input" // 等待输入
    StatusDiscarded    = "discarded"    // 已丢弃
)
```

**借鉴点**: 细粒度状态支持中断/恢复、等待输入等场景

#### 3.2.3 Manager Pattern

```go
type SessionManager interface {
    LaunchSession(ctx, config, isDraft) (*Session, error)
    ContinueSession(ctx, ContinueSessionConfig) (*Session, error)
    InterruptSession(ctx, sessionID) error
    LaunchDraftSession(ctx, sessionID, prompt, createDir) error
    UpdateSessionSettings(ctx, sessionID, updates) error
    GetSessionInfo(sessionID) (*Info, error)
    ListSessions() []Info
}
```

**借鉴点**:

- Launch/Continue 分离支持会话链
- Draft 模式支持预配置
- 统一的 Settings 更新接口

#### 3.2.4 Event Bus Integration

```go
// 状态变更时发布事件
m.eventBus.Publish(bus.Event{
    Type: bus.EventSessionStatusChanged,
    Data: map[string]interface{}{
        "session_id": sessionID,
        "old_status": string(StatusRunning),
        "new_status": string(StatusCompleted),
    },
})
```

**借鉴点**: 解耦状态变更通知，支持 UI 实时更新

#### 3.2.5 Store Interface

```go
type ConversationStore interface {
    // Session CRUD
    CreateSession(ctx, session) error
    UpdateSession(ctx, sessionID, updates) error
    GetSession(ctx, sessionID) (*Session, error)
    ListSessions(ctx) ([]*Session, error)
    
    // Event storage
    AddConversationEvent(ctx, event) error
    GetConversation(ctx, claudeSessionID) ([]*ConversationEvent, error)
    
    // Approval integration
    CreateApproval(ctx, approval) error
    GetPendingApprovals(ctx, sessionID) ([]*Approval, error)
}
```

**借鉴点**: 抽象 Store 接口，集成事件和审批管理

### 3.3 Crush Current Architecture

```
internal/
├── agent/           # Agent 系统 (Coordinator, SessionAgent)
│   ├── coordinator.go   # 协调多 Agent
│   ├── agent.go         # SessionAgent 实现
│   └── subagent_*.go    # Subagent 工具
├── orchestrator/    # Claude Code 编排
│   └── claude_code_orchestrator.go
├── session/         # 会话服务
│   └── session.go   # Session CRUD
├── message/         # 消息服务
│   └── message.go   # Message CRUD
├── db/              # 数据库层
│   ├── migrations/  # SQL 迁移
│   └── sql/         # SQLC 查询
└── permission/      # 权限服务
```

**现有能力**:

- `ClaudeCodeOrchestrator`: 多轮 Claude Code 交互
- `Session/Message Service`: SQLite CRUD + PubSub
- `Subagent`: 单轮轻量 Agent 调用

**缺失能力**:

- 多阶段工作流状态机
- Codex 集成
- 跨阶段上下文管理

---

## 4. Technical Architecture

### 4.1 System Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                      User Layer                              │
│  ┌─────────┐  ┌─────────────┐  ┌────────────────────────┐   │
│  │   TUI   │  │ /workflow   │  │ Approval Dialogs       │   │
│  └────┬────┘  └──────┬──────┘  └───────────┬────────────┘   │
└───────┼──────────────┼─────────────────────┼────────────────┘
        │              │                     │
        ▼              ▼                     ▼
┌─────────────────────────────────────────────────────────────┐
│                  Workflow Orchestration Layer (NEW)          │
│  ┌──────────────┐  ┌──────────────┐  ┌────────────────────┐ │
│  │   Workflow   │  │    State     │  │     Context        │ │
│  │    Engine    │◄─┤   Manager    │◄─┤     Builder        │ │
│  └──────┬───────┘  └──────┬───────┘  └────────────────────┘ │
│         │                 │                                  │
│         ▼                 ▼                                  │
│  ┌──────────────────────────────────────────────────────┐   │
│  │              Subagent Registry                        │   │
│  │  ┌─────────────────┐      ┌─────────────────────┐    │   │
│  │  │ Claude Code     │      │ Codex Adapter       │    │   │
│  │  │ Adapter         │      │ (CLI Wrapper)       │    │   │
│  │  └────────┬────────┘      └──────────┬──────────┘    │   │
│  └───────────┼──────────────────────────┼───────────────┘   │
└──────────────┼──────────────────────────┼───────────────────┘
               │                          │
               ▼                          ▼
┌──────────────────────────┐   ┌────────────────────────────┐
│  Existing Crush Layer    │   │   External Agents          │
│  ┌────────────────────┐  │   │  ┌──────────────────────┐  │
│  │ ClaudeCodeOrch.    │  │   │  │    Claude Code CLI   │  │
│  ├────────────────────┤  │   │  ├──────────────────────┤  │
│  │ Session Service    │  │   │  │    Codex CLI         │  │
│  ├────────────────────┤  │   │  └──────────────────────┘  │
│  │ Message Service    │  │   └────────────────────────────┘
│  ├────────────────────┤  │
│  │ Permission Service │  │
│  └────────┬───────────┘  │
└───────────┼──────────────┘
            ▼
┌───────────────────────────┐
│   SQLite (Existing + New) │
│  ┌─────────┐ ┌──────────┐ │
│  │sessions │ │workflows │ │ ← NEW
│  ├─────────┤ ├──────────┤ │
│  │messages │ │wf_steps  │ │ ← NEW
│  └─────────┘ └──────────┘ │
└───────────────────────────┘
```

### 4.2 Component Design

#### 4.2.1 Workflow Engine (`internal/workflow/engine.go`)

**职责**: 驱动工作流状态机，协调各阶段执行

```go
type Engine struct {
    stateManager   *StateManager
    contextBuilder *ContextBuilder
    registry       *SubagentRegistry
    permissions    permission.Service
    eventBus       *pubsub.Broker[WorkflowEvent]
}

func (e *Engine) Run(ctx context.Context, wf *Workflow) error {
    for wf.State != StateComplete && wf.State != StateFailed {
        if wf.State == StatePaused {
            return nil // 等待用户输入
        }
        nextState, err := e.executePhase(ctx, wf)
        if err != nil {
            return e.handleError(ctx, wf, err)
        }
        e.stateManager.Transition(ctx, wf, nextState)
    }
    return nil
}
```

#### 4.2.2 State Manager (`internal/workflow/state.go`)

**职责**: 管理工作流和步骤的持久化状态

```go
type StateManager struct {
    db      *db.Queries
    eventBus *pubsub.Broker[WorkflowEvent]
}

// 借鉴 Humanlayer 的 Type Separation
type Workflow struct {
    ID              string
    ParentSessionID string
    State           WorkflowState
    Plan            *WorkflowPlan
    CurrentStepIdx  int
    CreatedAt       time.Time
    UpdatedAt       time.Time
}

type WorkflowUpdate struct {
    State          *WorkflowState
    Plan           *WorkflowPlan
    CurrentStepIdx *int
    ErrorMessage   *string
}
```

#### 4.2.3 Subagent Registry (`internal/subagent/registry.go`)

**职责**: 注册和分发 Claude Code / Codex 调用

```go
// 借鉴 agentsdk-go Model 接口
type Subagent interface {
    Name() string
    Execute(ctx context.Context, req Request) (*Result, error)
    Resume(ctx context.Context, sessionID string, prompt string) (*Result, error)
    CanResume(ctx context.Context, sessionID string) bool
}

type Registry struct {
    agents map[string]Subagent
}

func (r *Registry) Dispatch(name string, req Request) (*Result, error) {
    agent, ok := r.agents[name]
    if !ok {
        return nil, fmt.Errorf("unknown agent: %s", name)
    }
    return agent.Execute(context.Background(), req)
}
```

#### 4.2.4 Codex Adapter (`internal/subagent/codex.go`)

**职责**: CLI 封装，模仿 SDK 接口便于未来迁移

```go
// 模仿 agentsdk-go 的 Model 接口
type CodexAdapter struct {
    cliPath    string
    workingDir string
}

func (a *CodexAdapter) Execute(ctx context.Context, req Request) (*Result, error) {
    // 1. 构建 CLI 参数
    args := []string{"--sandbox", "read-only", "--quiet"}
    
    // 2. 构建提示词 (包含上下文)
    prompt := a.buildPrompt(req)
    
    // 3. 执行 CLI
    cmd := exec.CommandContext(ctx, a.cliPath, args...)
    cmd.Stdin = strings.NewReader(prompt)
    
    // 4. 解析输出
    output, err := cmd.Output()
    return a.parseOutput(output)
}
```

### 4.3 Workflow State Machine

```
┌─────────────┐
│   START     │
└──────┬──────┘
       ▼
┌─────────────────┐
│  Requirements   │◄────────────────────────┐
│  (Claude Code)  │                         │
└────────┬────────┘                         │
         ▼                                  │
┌─────────────────┐                         │
│    Planning     │                         │
│  (Claude Code)  │                         │
└────────┬────────┘                         │
         ▼                                  │
┌─────────────────┐     ┌──────────┐        │
│   Plan Review   │────►│  PAUSED  │────────┘ (用户拒绝)
│    (Codex)      │     │ (人工审批) │
└────────┬────────┘     └────┬─────┘
         │                   │ (用户批准)
         │◄──────────────────┘
         ▼
┌─────────────────┐
│     Coding      │◄───────────────────┐
│  (Claude Code)  │                    │
└────────┬────────┘                    │
         ▼                             │
┌─────────────────┐                    │
│ Feature Review  │────────────────────┘ (发现问题)
│    (Codex)      │
└────────┬────────┘
         │ (全部 Feature 完成)
         ▼
┌─────────────────┐
│  Final Review   │────────────────────┐ (发现问题)
│    (Codex)      │                    │
└────────┬────────┘                    │
         │ (通过)                       │
         ▼                             │
┌─────────────────┐                    │
│  Documentation  │                    │
│  (Claude Code)  │                    │
└────────┬────────┘                    │
         ▼                             │
┌─────────────────┐                    │
│    COMPLETE     │                    │
└─────────────────┘                    │
                                       │
┌─────────────────┐                    │
│     FAILED      │◄───────────────────┘ (max retries)
└─────────────────┘
```

### 4.4 Database Schema

```sql
-- 工作流表
CREATE TABLE workflows (
    id TEXT PRIMARY KEY,
    parent_session_id TEXT REFERENCES sessions(id),
    title TEXT NOT NULL,
    state TEXT NOT NULL DEFAULT 'requirements',
    plan_json TEXT,
    config_json TEXT,
    current_step_index INTEGER DEFAULT 0,
    error_message TEXT,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    completed_at INTEGER
);

-- 工作流步骤表
CREATE TABLE workflow_steps (
    id TEXT PRIMARY KEY,
    workflow_id TEXT NOT NULL REFERENCES workflows(id) ON DELETE CASCADE,
    step_index INTEGER NOT NULL,
    step_type TEXT NOT NULL,  -- 'planning', 'coding', 'review', 'documentation'
    agent TEXT NOT NULL,      -- 'claude_code' or 'codex'
    agent_session_id TEXT,    -- 子 Agent 会话 ID
    status TEXT NOT NULL DEFAULT 'pending',
    title TEXT,
    input_context_json TEXT,
    output TEXT,
    review_result_json TEXT,
    retry_count INTEGER DEFAULT 0,
    max_retries INTEGER DEFAULT 3,
    requires_approval BOOLEAN DEFAULT FALSE,
    approval_status TEXT,
    created_at INTEGER NOT NULL,
    started_at INTEGER,
    completed_at INTEGER,
    UNIQUE(workflow_id, step_index)
);

CREATE INDEX idx_workflows_state ON workflows(state);
CREATE INDEX idx_steps_workflow ON workflow_steps(workflow_id);
```

### 4.5 Context Management Strategy

| 阶段 | 上下文内容 | Token 预算 |
|------|-----------|-----------|
| **Plan Review** | 完整 Plan + 代码结构摘要 | ~6,000 |
| **Feature Review** | Plan 片段 + 文件 Diff + 测试输出 | ~4,000 |
| **Final Review** | Plan 摘要 + 全部 Diff + 覆盖率 | ~8,000 |

```go
type ContextBuilder struct {
    workingDir string
    git        GitClient
}

func (b *ContextBuilder) BuildReviewContext(
    plan *WorkflowPlan,
    feature *FeatureSpec,
    changedFiles []string,
) *StepContext {
    ctx := &StepContext{}
    ctx.PlanExcerpt = b.extractPlanSection(plan, feature)
    ctx.FileDiffs = b.getFileDiffs(changedFiles, 500) // max 500 lines/file
    ctx.RelevantFiles = b.identifyRelevantFiles(changedFiles, 3)
    return ctx
}
```

---

## 5. Implementation Plan

### 5.1 Phase Overview

| Phase | Components | Effort | Dependencies |
|-------|------------|--------|--------------|
| 1 | DB Schema + Types | 2 days | None |
| 2 | Subagent Interface + Claude Adapter | 2 days | Phase 1 |
| 3 | Codex Adapter (CLI wrapper) | 2 days | Phase 2 |
| 4 | Context Builder | 2 days | Phase 3 |
| 5 | Workflow Engine | 3 days | All above |
| 6 | TUI Integration | 2 days | Phase 5 |
| 7 | Testing + Polish | 2 days | All above |

**Total**: ~15 days

### 5.2 File Structure

```
internal/
├── workflow/                # NEW
│   ├── engine.go           # 工作流引擎
│   ├── state.go            # 状态管理
│   ├── context.go          # 上下文构建
│   ├── safety.go           # 关键操作检测
│   └── types.go            # 类型定义
├── subagent/               # NEW
│   ├── types.go            # Subagent 接口
│   ├── registry.go         # Agent 注册
│   ├── claude_code.go      # Claude Code 适配
│   └── codex.go            # Codex CLI 适配
├── db/
│   ├── migrations/
│   │   └── 20251210000000_add_workflows.sql  # NEW
│   └── sql/
│       └── workflows.sql   # NEW
└── tui/
    └── components/
        └── workflow/       # NEW
            ├── status.go
            └── approval.go
```

### 5.3 Key Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Codex 调用方式 | CLI | 当前最简单，结构模仿 SDK 便于迁移 |
| 自动化级别 | Plan 暂停 + 关键操作确认 | 平衡自动化与安全性 |
| 持久化 | SQLite | 复用现有基础设施 |
| 状态模式 | 借鉴 Humanlayer | 成熟的状态分离模式 |

---

## 6. Verification Plan

### 6.1 Unit Tests

```bash
go test ./internal/workflow/... -v
go test ./internal/subagent/... -v
```

### 6.2 Integration Tests

1. **工作流生命周期测试**: 创建→执行→完成
2. **中断恢复测试**: Kill 进程后重启验证状态恢复
3. **Pause/Resume 测试**: 验证 Plan Review 暂停和用户输入

### 6.3 Manual Verification

1. `/workflow "implement simple feature"` - 验证完整流程
2. 中途 Ctrl+C 后重启 - 验证恢复能力
3. Plan Review 拒绝 - 验证修订循环

---

## 7. Open Questions

1. **Session 复用**: Codex review 是否应该跨 feature 复用 session？
2. **并行执行**: 独立的 feature 是否应该并行开发？
3. **Telemetry**: 需要收集哪些指标用于性能分析？

---

## 8. Appendix

### 8.1 Related Documents

- [Crush Architecture Overview](../02_Architecture_Overview.md)
- [Subagent Feature](../features/Subagent.md)

### 8.2 Reference Projects

- [humanlayer/hld/session](https://github.com/humanlayer/humanlayer/tree/main/hld/session)
- [myclaude/codex-wrapper](file:///Users/mxue/GitRepos/Coding/myclaude/codex-wrapper)
- [agentsdk-go/pkg/agent](file:///Users/mxue/GitRepos/Coding/agentsdk-go/pkg/agent)
