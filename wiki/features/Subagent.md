# Subagent Architecture Overview

## 目标
- 支持 Main Agent 和 Sub Agent 之间的双向通信与多轮会话。
- Main 为每个任务下发固定的工具 Allowlist、路径/操作权限、成本/回合上限；默认不做动态提权。
- Sub Agent 在固定能力边界内工作，必要时只能通过正式请求通道申请升级（建议用“新子代理配置”替代临时提权）。
- 后续两侧都可对接各自的记忆组件，但 Sub 的记忆应仅覆盖其执行域。
- 兼容直接调用 CLI/SDK（Claude Code、未来 Codex/Gemini CLI）以及现有 Claude Code Tool 的快速路径。

## 能力边界与提权策略
- 默认固定能力：每个子代理预先定义工具/权限包，便于治理、审计和安全边界控制。
- 如需更多权限/工具，优先创建新的子代理配置（新的 allowlist/路径/上限），而不是临时提权。
- 若必须动态提权，也需经 `permission_request`/`tool_request` 流程，记录理由与批准路径。

## 协议建议
- 消息类型：`assign_task`、`progress`、`clarification_request`、`permission_request`、`tool_request`、`result`、`heartbeat`、`cancel`.
- 每条消息包含：`task_id`、`session_id`、`role`（main/sub）、`payload`（结构化字段）、`visibility`（对用户可见/内部）。
- 许可模型：Main 下发 capability bundle（允许工具列表、路径/写权限、成本与回合上限）。Sub 只能在 bundle 内工作；超出时必须发起 `permission_request` / `tool_request`。
- 成本/计量：对子调用/子工具链标记 `parent_tool_use_id`（或等效字段）以避免重复计费；每个 Sub Agent 维护独立会话成本与摘要，Main 汇总父会话。

## 路由与模式
- **快路径**：Main 直接多次调用 Claude Code Tool（或未来 Codex/Gemini Tool），适合简单/低风险任务；无专门子代理会话。
- **子代理路径**：Main 派单给专门的 Sub Agent（如 ClaudeCodeAgent/CodexAgent/GeminiAgent），使用上述协议进行多轮协作和权限调解。
- 选择策略：规则/启发式或模型打分；失败/超时可降级为 Main 接管或改派其它子代理。

## 记忆与有/无状态
- Main：全局/母会话记忆（用户意图、上下文裁剪、关键决策、权限升级轨迹），并保存子代理返回的“句柄/摘要”。
- Sub：可“轻度有状态”，仅负责自身执行域的操作性记忆（如 Claude Code session resume/fork ID、局部上下文），按 agent_id/任务隔离，不共享给其它子代理。
- 句柄传递：Sub 将 resume/fork 等句柄返回给 Main；Main 在后续调用时再传回句柄，Sub 自行管理细节。
- 若成本/风险更重视隔离，可让 Sub 完全无状态，每次由 Main 注入必要上下文；代价是 prompt 更长、无法保留底层工具会话。

## 审批与安全
- 权限守卫：按路径/操作/工具/成本限额下发；默认最小权限。
- 升级通道：子代理通过 `permission_request`/`tool_request` 明确说明所需范围与理由，Main 决定批准/拒绝。
- 超时/心跳：子代理周期性 `heartbeat`，Main 可中断或回收。

## 最小可用版本（当前落地）
- 单轮子代理（无中途交互），固定工具/权限，调用 Task/子 Agent 完成一次性任务。
- 使用独立会话 ID 与成本回填，便于后续接入正式协议与记忆。

## 后续迭代方向
- 将子代理返回结构化为事件流（权限/工具请求、澄清、进度）。
- 引入路由器：动态选择直接 Tool vs 子代理。
- 接入 MCP/审批服务作为权限升级的外部通道。
- 为各 Agent 接入各自记忆存储与摘要同步。
