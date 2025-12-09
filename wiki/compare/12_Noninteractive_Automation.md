# 非交互自动化与结构化输出

> 来源: Codex CLI  
> 优先级: 🟡 中

## 概述

Codex 的 `codex exec` 提供非交互自动化：默认只读、安全流式日志、JSONL 事件输出、结构化 JSON 响应、以及可恢复的 exec 会话。Crush 当前缺少等价的自动化入口和事件流格式。

## Codex 关键点

- **默认安全**：`codex exec` 默认 `read-only`，无命令/编辑审批；`--full-auto` 允许写入，`--sandbox danger-full-access` 解除限制。  
- **流式事件**：`--json` 输出 JSONL 事件（thread/turn started/completed、command_execution、file_change、mcp_tool_call 等），便于脚本消费。  
- **结构化输出**：`--output-schema` 传入 JSON Schema，最终回复按 schema 输出，可配合 `-o` 仅打印最终 JSON。  
- **会话恢复**：`codex exec resume --last|<ID>` 保留上下文继续自动化任务。  
- **管道友好**：过程日志到 stderr，最终消息到 stdout，便于重定向。详见 `docs/exec.md`。  

## OpenCode 现状（参考）

- 具备 CLI 自动化能力（`opencode run/serve`），但未提供 JSONL 事件/结构化输出与 resume 的等价接口。  

## 对 Crush 的建议

1. **新增非交互子命令**：`crush exec "<prompt>"`，默认 `sandbox=read-only`，可选 `--sandbox workspace-write|danger-full-access`。  
2. **事件流格式**：定义 JSONL 事件（会话/回合、命令执行、文件变更、工具调用、错误），供脚本消费；同时保留 TTY 友好 stderr 输出。  
3. **结构化输出**：支持 `--output-schema <file>`；若提供则最终输出 JSON（校验失败给出错误）。  
4. **Resume 支持**：`crush exec resume --last|<id>`，复用同一自动化上下文。  
5. **安全默认**：在非交互模式禁止网络/写入，除非明确 flag；检测 Git 仓库缺失时提供 `--skip-git-check`。  
6. **配置/日志**：允许 `--output-last-message <path>` 将最终回复写文件；在 JSONL 中记录 token 使用、sandbox/approval 配置。

## 预期收益

- 便于在 CI/CD、脚本或其他 Agent 中调用 Crush  
- 提供可观测的机器可读事件流  
- 降低二次包装成本（直接使用 CLI 而非额外适配）
