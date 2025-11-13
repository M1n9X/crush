# Claude Code CLI vs SDK - Mode Analysis

## 问题澄清

用户指出：当前的实现使用的是 `claude -p` 命令，这与预期相差较远。

## 实际情况检查

### SDK 实际执行的命令

在 `humanlayer/claudecode-go/client.go` 中：

```go
// Always use print mode for SDK - MUST be the last flag before --
args = append(args, "--print")
args = append(args, "--")
args = append(args, config.Query)
```

所以实际执行的命令是：
```bash
claude --model sonnet \
       --output-format stream-json \
       --max-turns 15 \
       --verbose \
       --print \
       -- \
       "Create a web application..."
```

注意：`--print` 和 `-p` 是等价的（-p 是 --print 的简写）

## Claude Code CLI 的两种模式

### 1. Print Mode（--print / -p）

**命令**：`claude --print -- "query"`

**特点**：
- ✅ 非交互模式，执行后自动退出
- ✅ 适合程序化调用
- ✅ 输出可以捕获和解析（JSON）
- ❌ 不支持交互式功能（如中途输入）
- ❌ 某些高级功能受限

**适用场景**：
- Crush 等自动化工具
- 脚本化任务
- 集成到其他应用

**SDK 实现**：`Launch()` + `Wait()`

### 2. Interactive Mode（默认）

**命令**：`claude`

**特点**：
- ✅ 完整 TUI 界面
- ✅ 支持实时交互
- ✅ 所有功能可用
- ✅ 可以中途暂停、继续
- ❌ 需要终端交互
- ❌ 输出不适合程序化解析

**适用场景**：
- 开发人员直接使用
- 需要交互式调试
- 探索性任务

**执行方式**：直接运行 `claude` 命令

## SDK 的限制

`humanlayer/claudecode-go` SDK **仅支持 Print 模式**，原因是：

1. **设计目标**：为 Go 程序提供类型安全的调用接口
2. **输出解析**：需要可预测的输出格式（JSON/stream-json）
3. **进程管理**：需要知道何时任务完成
4. **错误处理**：需要捕获错误信息

Interactive 模式的输出（包含 TUI 控制序列、用户交互等）很难解析。

## 用户的期望

用户希望：
- 使用完整的 Claude Code 功能
- 不仅仅是简化的 print 模式
- 更深层次集成

## 解决方案选项

### 选项 1：使用当前 SDK（Print 模式）✅ 已实现

**优点**：
- SDK 已经提供此功能
- 输出可解析，适合编程使用
- 支持流式 JSON
- 集成简单

**缺点**：
- 功能受限（某些交互式功能不可用）
- 只能执行，不能交互

**实施**：已完成（claude_code_orchestrator.go）

### 选项 2：扩展 SDK 支持交互模式

**实现方式**：
- 修改 `humanlayer/claudecode-go`
- 允许以交互模式启动
- 进程间通信通过 stdin/stdout（而非参数）
- 解析 ANSI/控制序列

**优点**：
- 访问所有 Claude Code 功能
- 实时交互能力

**缺点**：
- 需要修改 SDK
- 输出解析复杂
- 进程管理困难
- 需要持续维护

**工作量**：很大（需要 fork 并维护 SDK）

### 选项 3：绕过 SDK，直接调用 Claude API

**实现方式**：
- 不使用 SDK
- 直接调用 Claude API
- 自己实现工具调用逻辑
- 完全自定义

**优点**：
- 完全控制
- 所有功能可用

**缺点**：
- 重新实现 Claude Code
- 工作量巨大
- 失去现有生态

**工作量**：巨大（复刻 Claude Code）

## 建议

对于 Crush 项目，**选项 1（当前实现）是最合适的**：

1. **Crush 的定位**：自动化编程助手，不需要人工交互
2. **集成需求**：需要将输出解析为结构化数据
3. **成本追踪**：需要捕获使用量信息
4. **维护成本**：使用官方 SDK，无需维护

虽然 Print 模式确实有限，但对于**自动化任务**来说：

**Print 模式支持的功能**：
- ✅ 所有工具（Bash, Read, Write, Edit, etc.）
- ✅ 流式输出
- ✅ 权限系统
- ✅ 多轮对话（通过 --resume）
- ✅ 会话管理
- ✅ 成本追踪
- ✅ MCP 服务器

**Print 模式不支持的功能**：
- ❌ 人工中途干预
- ❌ TUI 界面
- ❌ 实时聊天式交互

而 Crush 作为自动化工具，**不需要**这些交互式功能。

## 结论

当前的实现（使用 SDK 的 Print 模式）是**正确且合适的**。

虽然有些功能限制，但这些限制不影响 Crush 的核心用途：
- 自动化代码生成
- 多轮任务执行
- 工具调用和结果处理
- 进度追踪

所有需要的功能在 Print 模式下都可以使用。

如果未来有新需求需要交互式功能，我们可以再评估扩展 SDK 或绕过 SDK 的方案。
