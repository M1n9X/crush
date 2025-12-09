# 会话分享与模式配置

> 来源: OpenCode  
> 优先级: 🟡 中

## 概述

OpenCode 提供会话分享/自动分享以及可配置的模式（build/plan 及自定义 agent 模式），便于协作与安全分级。Crush 目前无分享功能，模式仅有 Subagent 概念，缺少用户可切换的权限/工具预设。

## OpenCode 关键点

- **分享**：`/share` 生成公开链接，`share` 配置支持 `manual`（默认）、`auto`、`disabled`，`/unshare` 撤销。见 `packages/web/src/content/docs/share.mdx` 与 `packages/opencode/src/session/index.ts`。  
- **模式/agents**：内置 `build`（全权限）与 `plan`（默认禁用 write/edit/bash）。模式可用 JSON 或 Markdown 定义，包含模型、温度、工具开关、prompt 文件等；Tab 或快捷键切换。见 `docs/modes.mdx`、`docs/agents.mdx`。  
- **规则融合**：模式可引用自定义 prompt 文件，与 AGENTS.md/instructions 一起作用。  

## Codex 状态

- 无分享/auto-share；无显式模式切换，但可通过 approval/sandbox/profile 变通。  

## 对 Crush 的建议

1. **分享开关**：在 `crush.json` 增加 `share` 配置（manual/auto/disabled）；`/share`、`/unshare` 命令生成/撤销链接（可先本地导出 Markdown/JSON，未来可选远程发布）。  
2. **模式预设**：提供 `build`/`plan` 核心模式，配置项涵盖模型、sandbox、approval-policy、启用/禁用工具列表；允许项目/全局定义自定义模式。  
3. **快捷切换**：在 TUI 状态栏展示当前模式，支持快捷键/命令面板切换；切换后提示生效的工具/权限变化。  
4. **安全默认**：Plan 模式默认只读/禁止 bash；切换到高权限模式时给出一次性确认。  
5. **导出/协作**：分享链接最小实现可为本地生成的静态 HTML/Markdown 文件，后续再考虑远程同步。

## 预期收益

- 团队协作与异步查看（分享）  
- 针对场景的权限/工具模板（模式），降低误操作风险  
- 清晰可见的安全态指示，提升信任度
