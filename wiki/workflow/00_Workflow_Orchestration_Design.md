# Crush Workflow Orchestration System

> **Version**: 2.0  
> **Date**: 2025-12-10  
> **Status**: Design (Ready for implementation)

---

## 1. Executive Summary

目标：把 Crush 演进为一个“可长线演化”的编排内核，新增 Coding/Review agent、扩展 memory/context builder、调整 Tool 集合时无需破坏核心架构或迁移数据。

关键思路：

- **统一 Subagent 层**：在现有 `internal/agent` 上扩展，吸收 opencode 的模式（primary/subagent/all、权限/模型/工具矩阵、内置 general/explore/plan），避免平行的第二套 Registry。
- **Codex Go SDK 一等公民**：基于 `CODEX-SDK-GO/openai`（对齐官方 TS SDK）完成 SDK，暴露 Exec/Resume/事件流；在 Crush 中作为模型提供者 + Subagent 适配层。
- **单一工作流引擎**：替换/封装当前 `ClaudeCodeOrchestrator`，状态机驱动多阶段步骤，所有 agent 调用通过 Subagent 接口（Claude Code 或 Codex）。
- **健壮状态管理**：借鉴 humanlayer 的三分模型（存储结构/Info 视图/Update 补丁）+ 细粒度状态机（running/interrupted/waiting_input 等）+ 事件总线，持久化走 SQLite/SQLC/触发器。
- **上下文与安全内建**：ContextBuilder 生成最小必要审查上下文；Safety Hooks/权限服务处理危险操作和审批；审批/暂停/恢复是默认能力。

---

## 2. Requirements & Constraints

| ID | Requirement | Priority |
|----|-------------|----------|
| REQ-1 | Claude Code + Codex 双 Agent 协作工作流 | P0 |
| REQ-2 | Subagent 体系完整（模式、权限、模型、工具、子会话） | P0 |
| REQ-3 | Plan/危险操作审批，暂停恢复 | P0 |
| REQ-4 | 工作流状态机持久化（中断恢复、重试、事件流） | P0 |
| REQ-5 | Codex Go SDK 与 TS SDK 对齐，支持 Exec/Resume/事件流 | P0 |
| REQ-6 | 上下文裁剪与审查（最小必要 diff/历史/plan） | P1 |
| REQ-7 | UI/CLI/TUI 统一查看/控制工作流 | P1 |
| REQ-8 | 可扩展：新增 Agent/Tool/Context 组件无需破坏架构 | P1 |

技术约束：

- 复用现有 SQLite/SQLC/Goose 迁移格式（触发器 + `strftime('%s','now')`）。
- 保持 `internal/agent` 的权限、watcher、capability、tools 体系。
- Codex SDK 需要 CLI 兼容，后续可替换为本地/远端服务。

---

## 3. Reference Cross-Check

| Source | 借鉴点 |
|--------|--------|
| **opencode** (`packages/opencode/src/agent/agent.ts`, `tool/task.ts`) | Agent 模式 primary/subagent/all，内置 general/explore/plan，权限矩阵/模型覆盖，子任务工具生成子会话并路由结果，父子会话导航 |
| **humanlayer** (`hld/session`, `hld/store`) | Session 状态机（draft/starting/running/waiting_input/interrupted/...），三分结构（存储/Info/Update），事件总线 + 审批存储，恢复/中断 |
| **agentsdk-go** | Model/ToolExecutor 分离，middleware 链接可插拔观测/安全；超时/重试上限 |
| **CODEX-SDK-GO/openai** + TS SDK | Exec/Resume 参数面（sandbox/full-auto/enable/output-schema/resume by thread），事件流解码，originator/env 注入 |
| **codex-mcp-server** | MCP 工具暴露/提示管理，可作为 Codex/Claude 的上下文工具源 |
| **Antigravity workflow** | Service Registry + Trajectory 状态机，步骤抽象与审批/安全门控 |

---

## 4. Target Architecture

### 4.1 分层

```text
User (CLI/TUI/HTTP)
    ↓ commands/events
Workflow Engine (单一状态机，暂停/恢复/审批)
    ↓ dispatch
Subagent Layer (统一 Registry)
    ├ Claude Code Adapter (现有，封装为 Subagent)
    ├ Codex Adapter (Go SDK，支持 resume/plan/full-auto)
    └ Future Agents (plugin-style)
    ↓
Agent Core (existing internal/agent coordinator, permissions, tools, capability, watcher)
    ↓
SQLite + SQLC (sessions/messages/workflows/steps/approvals/events)
    ↑ pubsub (workflow + session + message events)
Context/Safety (ContextBuilder, Permission/Safety hooks)
```

关键：只有一条 orchestrator 路径——Workflow Engine 驱动步骤，步骤通过 Subagent 接口调用 Claude/Codex，`ClaudeCodeOrchestrator` 被封装为子步骤或替换为 session agent 直连。

### 4.2 Subagent 设计（统一在 internal/agent 内）

- **Definition 扩展**：在现有 Markdown frontmatter 上增加 `mode` (primary/subagent/all)、`model_name`、`permission` 矩阵、`tools` 开关、`max_steps`、`color`，兼容 wildcard。
- **Built-ins**：general/explore/plan（opencode 风格）+ coder/reviewer（可选），默认继承调用者模型/权限，定义可覆盖。
- **Delegation Tool**：任务工具创建子 Session（存父子关联），message/store 隔离，结果回传父会话摘要。
- **Watcher**：继续使用 `SubAgentWatcher` 监听 `.crush/.claude/.codebreeze` 热加载。
- **Interface**：对齐 agentsdk-go（Generate/Tool + 取消/超时/重试 + middleware）。

### 4.3 Codex Go SDK & Adapter

- **SDK**：在 `internal/codexsdk`（或独立 module）实现与 TS SDK 对齐的 API：`Exec`/`Run` 支持 `--sandbox/--full-auto/--enable/--include-plan-tool/--output-schema/--image/--config/--resume(thread)`，事件流解码，stderr/退出码处理，originator/env 注入。补齐测试（stderr、timeout、resume、feature flags）。
- **Adapter**：Subagent 包装 SDK，暴露 `Execute(ctx, req)`/`Resume(ctx, sessionID, prompt)`，输出 session/thread id 供恢复；支持最小上下文注入（见 ContextBuilder）。

### 4.4 Workflow Engine

- **状态机来源**：支持内置顺序默认工作流，也支持从 DAG Spec（见 `wiki/spec/workflow_spec.md`）生成的状态机；分支/回跳按 DAG 边语义执行。
- **步骤模型**：`Workflow` + `WorkflowStep` 持久化；记录 agent 名称、上下文快照、审批需求、重试计数、错误、关联 session/message id。
- **执行循环**：Engine 拉取当前步骤，调用 Subagent，写入步骤输出，触发审批/暂停；恢复时从 DB 重建；若由 DAG 生成则按拓扑调度。
- **事件与审批**：工作流事件通过 pubsub；审批表复用/扩展，关联 session/message 事件。

### 4.5 状态与存储（对齐 humanlayer + Crush 现状）

- **三分结构**：存储结构 (DB model) / Info 视图 (外部响应) / Update 补丁 (指针字段)。
- **表**：`workflows`、`workflow_steps`、`approvals`（扩展）、`workflow_events`（可选）。时间戳与现有触发器一致，SQLC 生成 Go 绑定。
- **Session/Message 对齐**：workflow 记录 parent_session_id；子 agent 可创建子 session（delegation）；message 计数/费用沿用触发器。

### 4.6 Context & Safety

- **ContextBuilder**：利用 `internal/diff`、`history`、`projectdoc` 生成最小审查上下文：plan 片段、受限 diff（行数上限）、测试输出摘要、相关文件列表；按步骤类型配置 token 预算。
- **Safety Hooks**：危险命令/文件变更检测，permission/approval gate；Plan 审批默认开启；Codex `full-auto` 需显式批准。

### 4.7 UI/CLI/TUI

- **CLI**：新增 `crush workflow create/run/status/pause/resume/approve`，全部走 Workflow Engine，可选择默认或自定义 DAG。
- **TUI**：Workflow 面板（步骤/状态/审批）+ 父子 session 导航（opencode 风格）+ Codex/Claude 日志入口；支持 DAG 视图（节点/边）。
- **Events**：复用 pubsub 渲染实时状态。

---

## 5. Database Sketch (Goose + SQLC)

### 5.1 WorkflowState 枚举

```go
// WorkflowState 定义工作流的生命周期状态
type WorkflowState string

const (
    WorkflowStateDraft        WorkflowState = "draft"         // 初始化，未开始
    WorkflowStateRunning      WorkflowState = "running"       // 正在执行
    WorkflowStateWaitingInput WorkflowState = "waiting_input" // 等待用户输入/审批
    WorkflowStatePaused       WorkflowState = "paused"        // 用户主动暂停
    WorkflowStateCompleted    WorkflowState = "completed"     // 成功完成
    WorkflowStateFailed       WorkflowState = "failed"        // 失败终止
    WorkflowStateInterrupted  WorkflowState = "interrupted"   // 中断（进程退出/崩溃）
)

// StepStatus 定义步骤的执行状态
type StepStatus string

const (
    StepStatusPending   StepStatus = "pending"    // 未开始
    StepStatusRunning   StepStatus = "running"    // 执行中
    StepStatusCompleted StepStatus = "completed"  // 完成
    StepStatusFailed    StepStatus = "failed"     // 失败
    StepStatusSkipped   StepStatus = "skipped"    // 跳过
    StepStatusWaiting   StepStatus = "waiting"    // 等待审批
)

// ApprovalStatus 定义审批状态
type ApprovalStatus string

const (
    ApprovalPending  ApprovalStatus = "pending"  // 待审批
    ApprovalApproved ApprovalStatus = "approved" // 已批准
    ApprovalRejected ApprovalStatus = "rejected" // 已拒绝
)
```

### 5.2 表结构

```sql
-- workflows
id TEXT PRIMARY KEY
parent_session_id TEXT REFERENCES sessions(id)
title TEXT NOT NULL
state TEXT NOT NULL
plan_json TEXT
config_json TEXT
current_step_index INTEGER NOT NULL DEFAULT 0
error_message TEXT
created_at INTEGER NOT NULL
updated_at INTEGER NOT NULL
completed_at INTEGER

-- workflow_steps
id TEXT PRIMARY KEY
workflow_id TEXT NOT NULL REFERENCES workflows(id) ON DELETE CASCADE
step_index INTEGER NOT NULL
step_type TEXT NOT NULL
agent TEXT NOT NULL
agent_session_id TEXT
status TEXT NOT NULL
title TEXT
input_context_json TEXT
output_json TEXT
review_result_json TEXT
retry_count INTEGER NOT NULL DEFAULT 0
max_retries INTEGER NOT NULL DEFAULT 3
requires_approval BOOLEAN NOT NULL DEFAULT FALSE
approval_status TEXT
error_message TEXT
created_at INTEGER NOT NULL
started_at INTEGER
completed_at INTEGER
UNIQUE(workflow_id, step_index)
```

触发器：对齐 `sessions/messages`，自动维护 `updated_at`。

### 5.3 完整迁移示例

```sql
-- migrations/20251210000000_add_workflows.sql

-- +goose Up
CREATE TABLE IF NOT EXISTS workflows (
    id TEXT PRIMARY KEY,
    parent_session_id TEXT REFERENCES sessions(id),
    title TEXT NOT NULL,
    state TEXT NOT NULL DEFAULT 'draft',
    plan_json TEXT,
    config_json TEXT,
    current_step_index INTEGER NOT NULL DEFAULT 0,
    error_message TEXT,
    created_at INTEGER NOT NULL DEFAULT (strftime('%s','now')),
    updated_at INTEGER NOT NULL DEFAULT (strftime('%s','now')),
    completed_at INTEGER
);

CREATE TABLE IF NOT EXISTS workflow_steps (
    id TEXT PRIMARY KEY,
    workflow_id TEXT NOT NULL REFERENCES workflows(id) ON DELETE CASCADE,
    step_index INTEGER NOT NULL,
    step_type TEXT NOT NULL,        -- 'plan', 'code', 'review', 'docs'
    agent TEXT NOT NULL,            -- 'claude-code', 'codex'
    agent_session_id TEXT,          -- Resume token
    status TEXT NOT NULL DEFAULT 'pending',
    title TEXT,
    input_context_json TEXT,        -- ContextBuilder output
    output_json TEXT,               -- Agent result
    review_result_json TEXT,        -- Review feedback
    retry_count INTEGER NOT NULL DEFAULT 0,
    max_retries INTEGER NOT NULL DEFAULT 3,
    requires_approval INTEGER NOT NULL DEFAULT 0,  -- SQLite uses INTEGER for boolean
    approval_status TEXT,           -- 'pending', 'approved', 'rejected'
    error_message TEXT,
    created_at INTEGER NOT NULL DEFAULT (strftime('%s','now')),
    updated_at INTEGER NOT NULL DEFAULT (strftime('%s','now')),
    started_at INTEGER,
    completed_at INTEGER,
    UNIQUE(workflow_id, step_index)
);

CREATE INDEX idx_workflows_parent_session ON workflows(parent_session_id);
CREATE INDEX idx_workflows_state ON workflows(state);
CREATE INDEX idx_workflow_steps_workflow ON workflow_steps(workflow_id);
CREATE INDEX idx_workflow_steps_status ON workflow_steps(status);

-- Trigger: auto-update updated_at on workflows
CREATE TRIGGER update_workflows_updated_at
AFTER UPDATE ON workflows
FOR EACH ROW
BEGIN
    UPDATE workflows SET updated_at = strftime('%s','now') WHERE id = NEW.id;
END;

-- Trigger: auto-update updated_at on workflow_steps and bump parent workflow on change
CREATE TRIGGER update_workflow_steps_updated_at
AFTER UPDATE ON workflow_steps
FOR EACH ROW
BEGIN
    UPDATE workflow_steps SET updated_at = strftime('%s','now') WHERE id = NEW.id;
    UPDATE workflows SET updated_at = strftime('%s','now') WHERE id = NEW.workflow_id;
END;

CREATE TRIGGER insert_workflow_steps_touch_workflow
AFTER INSERT ON workflow_steps
FOR EACH ROW
BEGIN
    UPDATE workflows SET updated_at = strftime('%s','now') WHERE id = NEW.workflow_id;
END;

CREATE TRIGGER delete_workflow_steps_touch_workflow
AFTER DELETE ON workflow_steps
FOR EACH ROW
BEGIN
    UPDATE workflows SET updated_at = strftime('%s','now') WHERE id = OLD.workflow_id;
END;

-- +goose Down
DROP TRIGGER IF EXISTS delete_workflow_steps_touch_workflow;
DROP TRIGGER IF EXISTS insert_workflow_steps_touch_workflow;
DROP TRIGGER IF EXISTS update_workflow_steps_updated_at;
DROP TRIGGER IF EXISTS update_workflows_updated_at;
DROP INDEX IF EXISTS idx_workflow_steps_status;
DROP INDEX IF EXISTS idx_workflow_steps_workflow;
DROP INDEX IF EXISTS idx_workflows_state;
DROP INDEX IF EXISTS idx_workflows_parent_session;
DROP TABLE IF EXISTS workflow_steps;
DROP TABLE IF EXISTS workflows;
```

### 5.4 PubSub 事件扩展

现有 `internal/pubsub/events.go` 仅有 `CreatedEvent`, `UpdatedEvent`, `DeletedEvent`。

工作流需扩展以下事件类型：

```go
// 在 internal/pubsub/events.go 或 internal/workflow/events.go 中扩展
const (
    // Workflow lifecycle
    WorkflowStartedEvent   EventType = "workflow_started"
    WorkflowPausedEvent    EventType = "workflow_paused"
    WorkflowResumedEvent   EventType = "workflow_resumed"
    WorkflowCompletedEvent EventType = "workflow_completed"
    WorkflowFailedEvent    EventType = "workflow_failed"
    
    // Step lifecycle
    StepStartedEvent       EventType = "step_started"
    StepCompletedEvent     EventType = "step_completed"
    StepFailedEvent        EventType = "step_failed"
    StepWaitingEvent       EventType = "step_waiting"  // 等待审批
    
    // Approval
    ApprovalRequestedEvent EventType = "approval_requested"
    ApprovalGrantedEvent   EventType = "approval_granted"
    ApprovalDeniedEvent    EventType = "approval_denied"
)
```

---

## 6. Implementation Plan (Phased)

1) **Subagent Foundation**  
   - 扩展 Subagent 定义 schema（mode/model/permission/tools/max_steps/color）。  
   - 内置 general/explore/plan；保留 wildcard；新增 delegation 工具创建子 session，结果回传父会话。  
   - Session/Message 支撑父子关联 + 事件；tests 覆盖解析/dispatch。

2) **Codex Go SDK 完成度 & Adapter**  
   - 实现与 TS SDK 对齐的 `Exec/Run/Resume`、事件流、sandbox/full-auto/enable/output-schema/plan-tool。  
   - 覆盖 stderr/exit/resume/timeout/feature flag 测试。  
   - Subagent adapter 暴露 session/thread id，支持最小上下文注入。

3) **Workflow Engine & Schema**  
   - Goose 迁移 + SQLC 查询生成；Workflow/Step/Approval/Events Go 类型（存储/Info/Update）。  
   - Engine 状态机（暂停/重试/中断/恢复）；封装现有 `ClaudeCodeOrchestrator` 或直接用 session agent 调度。  
   - Pubsub 事件流发布。

4) **Context & Safety**  
   - ContextBuilder（plan 摘要 + diff 上限 + 测试输出 + 相关文件）。  
   - Safety hooks 调用 permission/approval；危险步骤自动 gate；Codex full-auto 需审批。

5) **CLI/TUI 集成**  
   - CLI 命令组 `/workflow`（create/run/status/pause/resume/approve）。  
   - TUI 面板：工作流进度、审批、父子 session 导航、日志。

6) **Hardening & Docs**  
   - 集成测试：生命周期、暂停恢复、中断重启、审批拒绝、重试路径。  
   - 文档更新（本文件 + 体系结构总览 + CLI/TUI 使用说明）。

---

## 7. Open Points / Decisions to Lock

1. `ClaudeCodeOrchestrator` 完全替换为 session agent 调用，还是包成 Workflow 的一步？（推荐替换，避免双轨。）  
2. Approval 表与现有 session/message 审批的复用方式：同表扩展 vs 新表视图。  
3. Codex adapter 默认 sandbox 策略：read-only + 需要审批的 full-auto。  
4. ContextBuilder 的预算与裁剪策略：按步骤类型配置（plan/feature review/final review）。

---

## 8. Appendix

### 8.1 与现有代码的映射关系

| 设计组件 | 现有代码位置 | 操作 | 备注 |
|----------|--------------|------|------|
| **Subagent 体系** | | | |
| `Subagent` interface | 无 | **新增** `internal/subagent/types.go` | 见 `wiki/spec/subagent_interface.md` |
| `ClaudeCodeSubagent` | 无 | **新增** `internal/subagent/claude_code.go` | 封装现有 session agent |
| `CodexSubagent` | 无 | **新增** `internal/subagent/codex.go` | 基于 Codex Go SDK |
| `SubAgentDefinition` | `internal/agent/subagent_defs.go` | **保留** | 配置层 |
| `SubAgentWatcher` | `internal/agent/subagent_watcher.go` | **保留** | 热重载 |
| `subAgentTool` | `internal/agent/subagent_tool.go` | **重构** | 调用 Subagent 接口 |
| **Workflow 体系** | | | |
| `WorkflowEngine` | 无 | **新增** `internal/workflow/engine.go` | 状态机核心 |
| `WorkflowState` types | 无 | **新增** `internal/workflow/types.go` | 见本文档 5.1 |
| `ContextBuilder` | 无 | **新增** `internal/workflow/context.go` | 上下文构建 |
| `ClaudeCodeOrchestrator` | `internal/orchestrator/claude_code_orchestrator.go` | **包装/替换** | 渐进迁移 |
| **数据层** | | | |
| `workflows` 表 | 无 | **新增** Goose 迁移 | 见本文档 5.3 |
| `workflow_steps` 表 | 无 | **新增** Goose 迁移 | 见本文档 5.3 |
| `sessions.parent_session_id` | `internal/db/models.go` | **已存在** ✅ | 支持父子关联 |
| **事件/权限** | | | |
| Workflow PubSub 事件 | `internal/pubsub/events.go` | **扩展** | 见本文档 5.4 |
| Permission Service | `internal/permission/permission.go` | **复用** | 审批 gate |
| **Diff/History** | | | |
| `diff.GenerateDiff` | `internal/diff/diff.go` | **复用** | deltas 生成 |
| `history.Service` | `internal/history/file.go` | **复用** | 相关文件历史 |

### 8.2 Codex SDK 集成路径

**当前状态**: 项目中不存在 `internal/codexsdk` 目录。

**集成方案**:

1. **外部依赖引入**:

   ```go
   // go.mod
   require github.com/M1n9X/codex-sdk-go v0.x.x
   ```

2. **Adapter 封装**:

   ```go
   // internal/subagent/codex.go
   package subagent

   import codex "github.com/M1n9X/codex-sdk-go"

   type CodexSubagent struct {
       client *codex.Client
       config CodexConfig
   }

   type CodexConfig struct {
       Sandbox   SandboxMode
       Model     string
       MaxTokens int
   }
   ```

3. **CLI Wrapper 备选**（当 SDK 不可用时）:
   - 复用 `myclaude/codex-wrapper` 的 CLI 封装模式
   - 通过 `exec.Command` 调用 `codex` CLI
   - 解析 stdout/stderr 提取结果

### 8.3 ClaudeCodeOrchestrator 迁移策略

**推荐方案**: 渐进式替换

| 阶段 | 动作 | 回滚风险 |
|------|------|----------|
| Phase 1 | 将 `ClaudeCodeOrchestrator` 包装为 `ClaudeCodeSubagent` | 低 |
| Phase 2 | 新 Workflow 默认使用 `WorkflowEngine` | 中 |
| Phase 3 | 迁移存量代码路径 | 中 |
| Phase 4 | 废弃 `ClaudeCodeOrchestrator` | 高（需充分测试） |

### 8.4 参考资源

- 对照参考：opencode (subagent/权限/子会话)、humanlayer (Session 状态/Store/事件)、agentsdk-go (agent loop/middleware)、CODEX-SDK-GO + TS SDK (Codex surface)、Antigravity (trajectory/registry)  
- 相关文档：
  - [`wiki/compare/00_Overview.md`](../compare/00_Overview.md) - 功能对比总览
  - [`wiki/features/Subagent.md`](../features/Subagent.md) - 现有 Subagent 文档（待更新）
  - [`wiki/spec/subagent_interface.md`](../spec/subagent_interface.md) - Subagent 接口规范
  - [`wiki/spec/workflow_spec.md`](../spec/workflow_spec.md) - Workflow DAG 规范
  - [`wiki/compare/12_Noninteractive_Automation.md`](../compare/12_Noninteractive_Automation.md) - 非交互自动化参考
