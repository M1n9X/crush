## Memory 机制对比

- Codex：长效记忆基于分层指令文件 AGENTS.md（全局 ~/.codex、repo 目录链、可用 AGENTS.override.md 覆盖），作为初始上下
    文注入会话；无专门的记忆读写工具，长对话依赖 rollout 持久化/恢复与历史归档。参见 docs/getting-started.md:63 与会话
    种子逻辑 codex-rs/core/src/codex.rs。
- CodeBreeze：提供持久化记忆目录 ~/.codebreeze/memory/agents/<agentId>，暴露 MemoryRead/MemoryWrite 工具（当
    前 isEnabled=false，但实现完备，含路径校验与写入）用于显式存取笔记；路径见 src/cli/tools/MemoryReadTool/
    MemoryReadTool.tsx 与 MemoryWriteTool.tsx，目录常量 src/utils/env.ts。还带文件新鲜度记录，用于后续上下文选择。

  Auto compact 机制对比

- Codex（codex-rs/core/src/codex.rs + core/src/compact.rs）：在每轮模型输出处理后用真实 TokenUsage 累计值与
    model_auto_compact_token_limit（配置或模型默认）比对，超阈值即触发。优先远程 compaction（ChatGPT 专用，core/src/
    compact_remote.rs），否则走本地 summarize prompt（core/templates/compact/prompt.md），重建历史：保留初始系统/指令、
    按 token 预算（~20k）回填最近用户消息、追加带 SUMMARY_PREFIX 的摘要用户消息，并保留 ghost snapshots 以支持 undo/
    rollout；持久化 Compacted rollout，重算 token 并提示用户。上下文管理器保证 call/output 成对与 token 估算，防止坏
    历史。
- CodeBreeze（src/utils/autoCompactCore.ts + src/query.ts）：每次 query 前估算消息 token（countTokens）与主模型
    contextLength 的 92% 阈值比对；超限则用主模型按 8 段结构 prompt 生成摘要，插入 “Context automatically compressed…”
    用户消息 + 摘要消息。额外自动恢复近期重要文件（src/utils/fileRecoveryCore.ts，限制文件数/单文件与总 token），并
    清理缓存/状态；UI 发出 compaction notice，记录 compactionInfo。Runtime 层另有 DefaultContextCompactor（packages/
    runtime/src/inference/engine/DefaultContextCompactor.ts）用于通用保留最近 + 简易摘要的降载，但未融合文件恢复。触发
    点提前于请求发送，但 token 估算粗粒度，且依赖同一模型完成摘要。

  综合评估

- 稳定性/可恢复性：Codex 依托真实 token 使用、历史规范化、ghost snapshot 与 rollout 持久化，支持 resume/fork/undo，整
    体鲁棒性更强。CodeBreeze 估算式触发更早，但可能因估算误差过早/过晚压缩；摘要失败会直接回退原消息。
- 上下文保真：Codex 保留近期用户消息并用统一摘要前缀，且可走远程压缩；但摘要结构简单，缺乏额外上下文注入。CodeBreeze
    摘要提示更结构化且自动回填关键文件，用户感知信息（compaction notice + recovered files）更透明。
- 性能/成本：Codex 只在超限后触发（基于实际消耗），避免不必要摘要；CodeBreeze 在 92% 阈值前置触发，可能增加额外一次主
    模型调用。
- Memory：Codex 倾向“指令记忆”（AGENTS + rollout），无需工具；CodeBreeze 具备显式笔记存储接口，更适合长期知识库，但目
    前默认关闭。

  对 crush 项目的建议

  1. 采用 Codex 的历史管理骨架：真实 token 监控 + compaction 结果持久化/可恢复（rollout/undo 概念）、call/output 规范化
     和 ghost snapshot 等，避免坏历史与重复 compaction。
  2. 在 compaction 内容上融合 CodeBreeze 优点：使用结构化摘要 prompt（8 段）并返回 compaction notice + summary
     preview；加入最近重要文件恢复（带 per-file/total token 上限与截断标记）。
  3. 触发策略可折中：默认真实 token 用量阈值（模型配置驱动），并在接近阈值（如 90–92%）提前预警/可选预压缩。
  4. Memory 方案：保留 Codex 式 AGENTS/目录继承作为“长期指令”，再引入 CodeBreeze 式 per-agent memory 目录与读写工具（默
     认可开启），用于显式知识存档；注意路径沙箱与权限校验。
  5. 用户体验：沿用 CodeBreeze 的 compactionInfo 事件（剩余 token、恢复文件列表），并保留 Codex 的风险警告文案；失败时
     回退原历史，避免中断对话。

## Crush 实现进度

- 长期记忆：新增 per-agent memory 目录 `data_directory/memory/agents/<agent>/`，默认开启；提供 `memory_read`/`memory_write` 工具并沙箱路径；系统提示在每轮调用前注入当前 memory 文件（但 memory 文件不会计入新鲜度或自动恢复列表，防止压缩时被当作工作区文件回填）。
- 自动压缩：基于真实 token 用量触发，在 92% 阈值强制压缩；在 90% 发出预警提示；压缩改用结构化摘要模板（8 段），保留摘要预览并显示恢复文件列表。
- 上下文恢复：压缩后尝试回填最近工作区文件（新鲜度 + 历史兜底，忽略 memory 路径），带行号与截断标记；压缩通知包含窗口使用率、token 数、摘要预览与已恢复文件。
- 稳定性：压缩失败记录告警但不中断对话；系统提示支持热刷新，始终包含最新 memory 内容。
