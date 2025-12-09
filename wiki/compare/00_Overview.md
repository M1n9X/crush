# Crush 功能借鉴分析索引

> 最后更新: 2025-12-09

本目录记录从 **Codex CLI** (OpenAI) 和 **OpenCode** (SST/Terminal.shop) 两个项目中可借鉴的优秀特性，用于完善 Crush 项目。

---

## 🔍 调研范围

| 项目 | 仓库 | 定位 | 技术栈 |
|------|------|------|--------|
| **Codex CLI** | openai/codex | 本地运行的 AI 编程代理 | Rust + Node.js |
| **OpenCode** | sst/opencode | 终端优先的 AI 编程代理（Client-Server） | TypeScript + Go |
| **Crush** | charmbracelet/crush | 终端 AI 助手 | Go (Bubble Tea) |

---

## 📁 功能分类文档

| 文档 | 描述 | 优先级 |
|------|------|--------|
| [01_Sandbox_Security.md](./01_Sandbox_Security.md) | OS 级沙箱与命令执行策略 | 🔴 高 |
| [02_Execpolicy_Rules.md](./02_Execpolicy_Rules.md) | 自定义命令执行规则系统 | 🔴 高 |
| [03_Custom_Prompts.md](./03_Custom_Prompts.md) | 可复用提示词模板 | 🟡 中 |
| [04_MCP_Server_Mode.md](./04_MCP_Server_Mode.md) | 将 CLI 作为 MCP 服务器 | 🟡 中 |
| [05_Observability_OTEL.md](./05_Observability_OTEL.md) | OpenTelemetry 日志与遥测 | 🟡 中 |
| [06_Config_Profiles.md](./06_Config_Profiles.md) | 配置文件与多 Profile 支持 | 🟢 低 |
| [07_SDK_Generation.md](./07_SDK_Generation.md) | 多语言 SDK 自动生成 | 🟢 低 |
| [08_Client_Server_Arch.md](./08_Client_Server_Arch.md) | 客户端-服务端分离架构 | 🟢 低 |
| [09_Plugin_System.md](./09_Plugin_System.md) | 插件系统与扩展机制 | 🟡 中 |
| [10_Session_Resume.md](./10_Session_Resume.md) | 会话恢复与历史管理 | 🟡 中 |
| [11_Instructions_and_Skills.md](./11_Instructions_and_Skills.md) | 指令聚合与可复用技能 | 🟡 中 |
| [12_Noninteractive_Automation.md](./12_Noninteractive_Automation.md) | 非交互自动化与结构化输出 | 🟡 中 |
| [13_Share_and_Modes.md](./13_Share_and_Modes.md) | 会话分享与模式配置 | 🟡 中 |

---

## 🎯 优先级说明

- 🔴 **高优先级**：核心安全与用户体验功能，建议优先实现
- 🟡 **中优先级**：增强功能，可在稳定版后考虑
- 🟢 **低优先级**：高级功能，长期路线图考虑

---

## 📊 功能对比速览

| 功能领域 | Codex | OpenCode | Crush 现状 | 建议 |
|----------|-------|----------|------------|------|
| **沙箱隔离** | ✅ seatbelt/landlock | ❌ | ❌ | 🔴 借鉴 Codex |
| **命令规则** | ✅ execpolicy | ❌ | ❌ | 🔴 借鉴 Codex |
| **自定义提示词** | ✅ prompts/ | ❌ | ❌ | 🟡 借鉴 Codex |
| **MCP 服务器模式** | ✅ mcp-server | ❌ | ❌ | 🟡 借鉴 Codex |
| **OpenTelemetry** | ✅ OTEL | ❌ | ❌ | 🟡 借鉴 Codex |
| **配置 Profiles** | ✅ profiles | ❌ | ❌ | 🟢 借鉴 Codex |
| **Client-Server 架构** | ❌ | ✅ | ❌ | 🟢 长期考虑 |
| **SDK 生成** | ✅ JS/TS | ✅ JS/Go | ❌ | 🟢 长期考虑 |
| **插件系统** | ✅ MCP | ✅ plugins | ✅ MCP | 🟡 增强 |
| **会话恢复** | ✅ resume | ✅ | ✅ | 🟡 增强 |
| **指令聚合 (AGENTS/Skills)** | ✅ AGENTS + skills | ✅ AGENTS/init | ❌ | 🟡 借鉴 Codex/OpenCode |
| **非交互自动化/JSONL** | ✅ codex exec + JSONL | ⚠️ 基础 CLI | ❌ | 🟡 借鉴 Codex |
| **分享/Auto-share** | ❌ | ✅ share/auto/unshare | ❌ | 🟡 借鉴 OpenCode |
| **模式/Agent 预设** | ❌ | ✅ build/plan/modes | ⚠️ Subagent | 🟡 借鉴 OpenCode |
| **Subagent** | ❌ | ✅ general | ✅ | ✅ 已实现 |
| **LSP 语义检索** | ❌ | ✅ | ✅ | ✅ 已实现 |
| **SQLite 持久化** | ❌ | ❌ | ✅ | ✅ 已实现 |
| **热重载配置** | ❌ | ❌ | ✅ | ✅ 已实现 |

---

## 🚀 实施建议

### Phase 1: 安全增强（优先级最高）

1. **OS 级沙箱** - 参考 Codex 的 seatbelt (macOS) 和 landlock (Linux) 实现
2. **Execpolicy 规则** - 引入自定义命令白名单/黑名单规则

### Phase 2: 用户体验增强

3. **Custom Prompts** - 支持 `~/.crush/prompts/` 目录下的可复用提示词
4. **MCP Server Mode** - 让 Crush 可作为 MCP 服务器供其他工具调用
5. **Session Resume 增强** - 支持 Git 分支关联和更丰富的历史信息

### Phase 3: 可观测性与运维

6. **OTEL 集成** - 添加 OpenTelemetry 日志和遥测支持
7. **Config Profiles** - 支持命名配置 profile 快速切换

### Phase 4: 长期架构演进

8. **Plugin System 增强** - 支持 JavaScript/WASM 插件
9. **Client-Server 分离** - 考虑远程控制场景
10. **SDK 生成** - 为 Crush 生成多语言 SDK

---

## 📚 参考资源

- [Codex CLI 文档](https://github.com/openai/codex/tree/main/docs)
- [OpenCode 文档](https://opencode.ai/docs)
- [Crush Wiki](../00_Home.md)
- [OpenCode vs Crush 对比](https://github.com/sst/opencode/blob/dev/wiki/05_Opencode_vs_Crush.md) (opencode 项目内)

---

*本文档将随着功能研究的深入不断更新。*
