# Workflow Spec (Draft)

> 目标：让编排可配置且与具体 Agent 解耦，Engine 读取该 Spec 生成可执行状态机，兼容新增步骤/替换 Agent（如把 Review 从 Codex 切换到 Claude Code，或新增 brainstorm by Codex）。

---

## Schema（YAML/JSON）

```yaml
version: 1
name: "default-dev-workflow"
description: "Plan→Review→Code→Feature Review→Final Review→Docs"
vars:
  max_retries: 2
steps:
  - id: plan
    name: "Planning"
    capabilities: ["plan"]          # 由 Engine 匹配具备该 capability 的 subagent
    preferred_agents: ["claude-code"]
    input:
      context:
        include: ["repo_structure", "recent_messages"]
      budget:
        tokens: 6000
    outputs:
      save_as: ["plan"]
    requires_approval: false
    on_error:
      retry: 1
      goto: fail

  - id: plan_review
    name: "Plan Review"
    capabilities: ["review"]
    preferred_agents: ["codex"]
    input:
      context:
        include: ["plan", "repo_structure"]
      budget:
        tokens: 6000
    requires_approval: true          # 进入 waiting_input
    on_approve: next
    on_reject: plan                  # 回到 plan 重新生成

  - id: coding
    name: "Coding"
    capabilities: ["code"]
    preferred_agents: ["claude-code"]
    input:
      context:
        include: ["plan", "recent_messages"]
    outputs:
      save_as: ["deltas"]            # 变更摘要/patch 列表
    on_error:
      retry: 2
      goto: fail

  - id: feature_review
    name: "Feature Review"
    capabilities: ["review"]
    preferred_agents: ["codex"]
    input:
      context:
        include: ["plan", "deltas", "tests"]
      budget:
        tokens: 4000
    on_fail: goto: coding            # 失败回到 coding

  - id: final_review
    name: "Final Review"
    capabilities: ["review"]
    preferred_agents: ["codex"]
    input:
      context:
        include: ["plan_summary", "deltas", "coverage"]
      budget:
        tokens: 8000
    on_fail: goto: coding

  - id: docs
    name: "Docs"
    capabilities: ["docs"]
    preferred_agents: ["claude-code"]
    input:
      context:
        include: ["plan", "deltas"]
    on_error:
      retry: 1

  - id: complete
    terminal: success

  - id: fail
    terminal: fail
```

---

## DAG 变体（用户自定义工作流）

> 语法目标：简单、安全、可验证；形态类似 Argo/Temporal 的节点/依赖写法，但保留我们自定义的字段。

```yaml
version: 1
name: "custom-workflow"
description: "Example DAG"
vars:
  max_retries: 2

nodes:
  - id: brainstorm
    capabilities: ["brainstorm"]
    preferred_agents: ["codex"]
    next: [plan]                   # 出边

  - id: plan
    capabilities: ["plan"]
    preferred_agents: ["claude-code"]
    next: [plan_review]

  - id: plan_review
    capabilities: ["review"]
    preferred_agents: ["codex","claude-code"]
    requires_approval: true
    on_approve: [coding]           # 多出边视为并行启动
    on_reject: [plan]

  - id: coding
    capabilities: ["code"]
    preferred_agents: ["claude-code"]
    next: [feature_review]

  - id: feature_review
    capabilities: ["review"]
    preferred_agents: ["codex"]
    on_fail: [coding]
    next: [final_review]

  - id: final_review
    capabilities: ["review"]
    preferred_agents: ["codex"]
    on_fail: [coding]
    next: [docs]

  - id: docs
    capabilities: ["docs"]
    preferred_agents: ["claude-code"]
    next: [complete]

  - id: complete
    terminal: success

  - id: fail
    terminal: fail
```

### DAG 约束

- 必须有且仅有一个 `terminal: success` 节点；可选一个 `terminal: fail`。
- 图需无环（Engine 加载时执行拓扑检查）；允许菱形分支/汇合。
- 出边语义：`next`/`on_approve`/`on_reject`/`on_fail`/`on_error` 统一为边集合。
- **并行执行**：v1 版本暂不支持真正的并行执行；多个出边按拓扑顺序依次调度。后续版本可通过 `parallel: true` 标志启用。
- 每节点至少一种 `capabilities`；Engine 需能选出可用 Subagent，否则配置错误。

### 默认 vs 自定义

1) 用户未提供：加载内置默认 DAG（顺序版可等价为线性 DAG）。  
2) 用户提供自然语言：Crush 自动转换 NL → DAG 草案 → 验证/补全后执行。  
3) 验证失败：回退默认 DAG，提示错误。

---

## 自然语言 → DAG 管道

1) **解析**：用 Crush 自身（Claude/Codex 均可）根据 NL 描述生成 DAG 草案（nodes/edges/capabilities/agents）。  
2) **验证**：Engine 静态检查：有向无环、终态存在、capabilities 可满足、审批/安全默认保守、sandbox 默认 read-only。  
3) **补全/规范化**：缺省字段填充默认值（max_retries=1、requires_approval=false、budget/token 上限、sandbox=read-only）。  
4) **确认/落盘**：规范化 DAG 存入 `workflow_specs`（或文件），workflow 实例记录 `spec_version`。  
5) **执行**：Runtime 读取规范化 DAG 作为状态机源。

> 业界参考：Argo Workflows（YAML DAG）、Airflow DAG（Python）、Temporal（代码 DSL）。此处选 YAML DAG：易校验、易存档、无需执行用户代码。

---

## Schema 校验

### JSON Schema 路径

- **Schema 文件**: `schema/workflow_spec.json`（与 `schema.json` crush config 同目录）
- **CUE Schema（可选）**: `schema/workflow_spec.cue`

### 校验规则

- 提供 JSON Schema / CUE Schema：校验终态唯一、无环、节点/边引用合法、capabilities 非空、出边字段互斥/可选、默认值可补全。
- Load 时执行：Schema 校验 → 拓扑排序/环检测 → capability 可用性检查 → 安全默认补全（sandbox=read-only、max_retries=1、requires_approval=false）。

---

## 改进建议（保持 DSL 简洁但完备）

- 统一出边字段（可选）：`transitions.on_success|on_fail|on_approve|on_reject`；`next` 作为糖写入 `on_success`。
- 并行标志：可选 `parallel: true`（fan-out 同步执行），Engine 可序列化执行作 fallback。
- 约束/守卫（可选）：`guards`（需要的上下文产物、审批结果、开关），让状态机语义更明确。
- 模板：提供默认线性 DAG + 带 brainstorm 分支的示例，供用户修改。

### 字段说明

- `capabilities`: Engine 通过 Subagent Registry 选择满足这些能力的 Agent（支持多候选）。
- `preferred_agents`: 优先候选；无则由 capability 匹配。
- `input.context.include`: 由 ContextBuilder 解析并装配（plan、deltas、tests、coverage、repo_structure、recent_messages 等）。
- `budget.tokens`: 上下文/模型 token 预算，用于裁剪。
- `requires_approval`: 标记审批节点；Engine 将状态置为 waiting_input。
- `on_error`/`on_fail`/`on_approve`/`on_reject`: 状态机跳转规则；未声明则默认 `next`。
- `outputs.save_as`: Engine 将步骤输出命名存入 workflow context（内存+DB）。
- `terminal`: `success` 或 `fail`，结束状态。

### 存储与加载

- Spec 可放文件（`wiki/specs/*.yaml`）或存 DB（`workflow_specs`），Engine 在 run 时加载并实例化。
- Spec 版本写入 `workflows.spec_version`，便于兼容迁移。

### 运行时绑定

- Engine 计算 step → agent 绑定：按 `preferred_agents` 过滤 `capabilities`，若都不可用则 fail。
- 每步都记录 `agent_session_id`（Claude/Codex resume token）到 `workflow_steps`，并通过 State Manager 持久化。
- 上下文构建/审批/重试逻辑由 Engine 统一处理，Subagent 不做状态管理。

---

## ContextBuilder 约定

- 支持的 context key：`plan`, `plan_summary`, `deltas`, `tests`, `coverage`, `repo_structure`, `recent_messages`, `related_files`.
- 预算策略：按 step 的 `budget.tokens` 对 diff/文件/日志做裁剪；行数/文件数限额可配置。
- 输出：结构化 JSON（供 Subagent 直接消费）。

### Context Key 与现有代码映射

| Context Key | 数据来源 | 现有代码位置 |
|-------------|---------|-------------|
| `plan` | Workflow 的 `plan_json` 字段 | `workflows.plan_json` |
| `plan_summary` | 从 `plan` 提取的摘要 | Engine 生成 |
| `deltas` | `internal/diff.GenerateDiff()` 产出 | `internal/diff/diff.go` |
| `tests` | 测试命令输出（stderr/stdout 截取） | Engine 执行 `go test`/`npm test` |
| `coverage` | 覆盖率报告解析 | Engine 执行并解析 |
| `repo_structure` | `projectdoc.Memory` 或 tree 命令 | `internal/projectdoc/` |
| `recent_messages` | `message.Service.ListBySession()` | `internal/message/` |
| `related_files` | `history.Service.ListLatestSessionFiles()` | `internal/history/file.go` |

---

## 状态机执行规则（摘要）

1) 从 spec 首步开始（或从 DB 恢复的 current_step_index）。  
2) 若 `requires_approval` → 状态置 waiting_input，等待用户决策。  
3) 调度 Subagent：传入上下文 + sandbox/permission 策略。  
4) 记录输出、agent_session_id、cost、tokens；根据 on_error/on_fail 跳转或重试。  
5) 终止于 `terminal` 节点（complete/fail）。  
6) 所有状态写 DB，事件经 pubsub 广播。
