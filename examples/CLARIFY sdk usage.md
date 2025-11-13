# 关于 SDK 使用方式的澄清

## 用户的观察完全正确 ✅

**当前的实现确实使用了 `claude --print` 模式（等同于 `-p`）。**

证据在 `humanlayer/claudecode-go/client.go`：

```go
// Always use print mode for SDK - MUST be the last flag before --
args = append(args, "--print")
args = append(args, "--")
args = append(args, config.Query)
```

## 为什么会这样？

### SDK 的设计

`humanlayer/claudecode-go` SDK **本身就是设计为使用 `--print` 模式**：

1. **自动化调用**：SDK 用于程序化调用，不是人工交互
2. **输出解析**：需要可解析的 JSON/stream-json 格式
3. **进程管理**：需要知道何时执行完成
4. **错误处理**：需要捕获错误信息

### 交互模式 vs Print 模式

| 特性 | 交互模式 (默认) | Print 模式 (--print/-p) |
|------|----------------|-------------------------|
| 命令 | `claude` | `claude --print -- "query"` |
| 界面 | 完整 TUI | 非交互 |
| 输出 | ANSI/控制序列 | JSON/stream-json |
| 可解析性 | ❌ 困难 | ✅ 容易 |
| 适合自动化 | ❌ 否 | ✅ 是 |
| 人工交互 | ✅ 支持 | ❌ 不支持 |

## 我们的使用方式

### 当前代码

```go
// 在 internal/orchestrator/claude_code_orchestrator.go

import "github.com/humanlayer/humanlayer/claudecode-go"

// 创建 SDK 客户端
client, err := claudecode.NewClient()

// 配置会话
config := claudecode.SessionConfig{
    Query:              query,
    OutputFormat:       claudecode.OutputStreamJSON,  // 流式 JSON
    MaxTurns:           15,
    SessionID:          session.ClaudeSessionID,
    Verbose:            true,
}

// 启动会话（使用 SDK）
claudeSession, err := client.Launch(config)  // ✅ 使用 SDK 的 Launch 方法

// 读取流式事件
for event := range claudeSession.Events {
    renderer.RenderEvent(event)  // 实时渲染
}

// 等待完成
result, err := claudeSession.Wait()
```

**这是完整使用 SDK 的方式**，没有绕过 SDK。

### SDK 做了什么

当调用 `client.Launch(config)` 时，SDK 内部：

1. 构建命令行参数（包括 `--print`）
2. 调用 `exec.Command()` 执行 `claude` 命令
3. 捕获 stdout/stderr
4. 解析输出（JSON/stream-json）
5. 提供类型安全的 API

## Print 模式的功能限制

**确实存在的限制**：

❌ **不支持的功能**：
- 人工中途干预
- TUI 界面
- 实时聊天式交互

✅ **支持的功能**（对 Crush 重要）：
- 所有工具（Bash, Read, Write, Edit, TodoWrite, etc.）
- 流式输出
- 权限系统
- 多轮对话（通过 --resume）
- 会话管理
- 成本追踪
- MCP 服务器
- 模型配置
- 工具权限控制

## 对 Crush 是否合适？

**是，Print 模式完全适合 Crush**：

1. **自动化工具**：Crush 不需要人工交互
2. **输出解析**：需要结构化数据
3. **会话管理**：需要保存/恢复会话
4. **成本追踪**：需要捕获使用量
5. **错误处理**：需要捕获错误

## 如果不使用 Print 模式

### 选项 1：修改 SDK

Fork `humanlayer/claudecode-go`，修改 `buildArgs()` 不使用 `--print`：

**难点**：
- 需要解析交互模式下的 ANSI/控制序列
- 进程管理（不知道何时完成）
- 输出格式不可预测
- 需要大量重构

### 选项 2：绕过 SDK

完全不使用 SDK，直接调用 Claude API

**难点**：
- 重新实现 Claude Code 的所有功能
- 工作量巨大
- 失去现有生态系统

## 结论

**当前实现是正确的**：
- ✅ 完整使用了 SDK（没有绕过）
- ✅ 使用了 SDK 设计的 Print 模式
- ✅ Print 模式功能足够（所有工具都可用）
- ✅ 适合 Crush 的自动化场景

虽然你在技术上是正确的（确实使用了 `-p` / `--print`），但这个限制来自 SDK 本身的设计，而**不是我们的实现问题**。

对于 Crush 的需求来说，Print 模式提供的功能已经完全足够。
