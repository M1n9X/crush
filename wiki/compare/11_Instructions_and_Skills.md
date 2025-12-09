# 指令聚合与可复用技能

> 来源: Codex CLI, OpenCode  
> 优先级: 🟡 中

## 概述

Codex 与 OpenCode 都提供“项目/全局指令”聚合机制（AGENTS.md）以及可复用技能/规则文件，帮助在会话启动时自动注入环境知识，降低反复提示成本。Crush 目前缺少对应的自动收集与验证逻辑。

## Codex 做法

- **AGENTS.md 发现顺序**：全局 `~/.codex/AGENTS.md` / `AGENTS.override.md`，再从仓库根到工作目录逐级查找 `AGENTS.override.md` → `AGENTS.md` → 自定义 fallback 名单（config 中配置）。见 `docs/agents_md.md`。  
- **大小/合并策略**：按路径顺序拼接，空文件跳过，默认 32 KiB 上限，深层目录覆盖浅层。  
- **Skills**：递归扫描 `~/.codex/skills/**/SKILL.md`，要求 YAML frontmatter 中 name/description，正文不注入，只给路径和描述，失效文件会弹错误提示。见 `docs/skills.md`。  
- **快速生成**：`/init` slash 命令生成 AGENTS.md（在 docs/slash_commands.md）。  

## OpenCode 做法

- **AGENTS.md + Rules**：支持全局 `~/.config/opencode/AGENTS.md` 与项目 `AGENTS.md`，并可在 `opencode.json` 的 `instructions` 字段添加额外规则文件或通配。参见 `packages/web/src/content/docs/rules.mdx`。  
- **初始化**：`/init` 生成/补全 AGENTS.md。  
- **模式/agent 结合**：模式可指定自定义 prompt 文件（见 `docs/modes.mdx`）。  

## 对 Crush 的借鉴点

1. **启动时指令聚合**：从用户 home 与仓库路径向下查找 `AGENTS.override.md`、`AGENTS.md`，可配置 fallback（如 TEAM_GUIDE.md）；设置总大小上限与排序规则。  
2. **技能目录**：支持 `~/.crush/skills/**/SKILL.md`，仅注入名称/描述/路径；无效文件给出一次性警告弹窗。  
3. **Slash/快捷命令**：提供 `/init` 快速生成 AGENTS.md 模板；在状态栏显示已加载指令来源。  
4. **配置项**：在 `crush.json` 增加 `project_doc_fallback_filenames`、`project_doc_max_bytes`、`skills_dir`。  
5. **隐私/安全**：默认只读加载；提示文件超长或含敏感路径时给出警告。

## 预期收益

- 减少重复提示、统一团队规范  
- 为模型提供上下文底线，降低跑偏风险  
- 易于分层管理（全局/项目/子目录）并可视化来源

## 进一步工作

- Lint 与诊断：在 TUI 给出无效 frontmatter/超长警告。  
- 文档：新增 wiki 指南和示例模板。  
- 兼容现有 Subagent：指令层与 Subagent 工具选择互补。
