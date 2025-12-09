# MCP Server Mode

> 来源: Codex CLI, OpenCode
> 优先级: 🟡 中

## 概述

Codex 可以作为 MCP (Model Context Protocol) 服务器运行，让其他工具或 AI Agent 框架调用 Codex 的能力。这实现了 AI 工具的互操作性。

## Codex 实现分析

### 启动方式

```bash
# 作为 MCP 服务器启动
codex mcp-server

# 使用 MCP Inspector 测试
npx @modelcontextprotocol/inspector codex mcp-server
```

### 暴露的工具

#### 1. `codex` - 启动新会话

```typescript
interface CodexTool {
  prompt: string;           // 必填
  model?: string;           // 可选，覆盖模型
  "approval-policy"?: string; // untrusted | on-failure | on-request | never
  sandbox?: string;         // read-only | workspace-write | danger-full-access
  cwd?: string;             // 工作目录
  profile?: string;         // 配置 profile
  "base-instructions"?: string; // 自定义指令
  config?: object;          // 配置覆盖
}
```

#### 2. `codex-reply` - 继续现有会话

```typescript
interface CodexReplyTool {
  prompt: string;           // 必填
  conversationId: string;   // 必填，会话 ID
}
```

### 使用场景

1. **多 Agent 协作**：其他 AI Agent 调用 Codex 处理编程任务
2. **自动化流水线**：CI/CD 中集成 AI 编程能力
3. **IDE 集成**：编辑器插件调用本地 Codex
4. **远程控制**：通过 HTTP/SSE 远程驱动 Codex

### 事件流

Codex MCP Server 通过事件流返回进度：

```json
{"type": "text_delta", "content": "Analyzing...", "turn_id": "123"}
{"type": "tool_call", "tool": "read", "arguments": {"file": "main.go"}}
{"type": "text_delta", "content": "Found issue...", "turn_id": "123"}
{"type": "turn_complete", "turn_id": "123"}
```

## 借鉴价值

### 对 Crush 的好处

1. **互操作性**：与其他 AI 工具集成
2. **远程控制**：支持移动端/Web 端控制
3. **自动化**：CI/CD 集成更灵活
4. **组合使用**：作为其他 Agent 的工具

### 实现建议

#### 命令入口

```go
// internal/cmd/mcp_server.go
package cmd

import (
    "github.com/spf13/cobra"
)

var mcpServerCmd = &cobra.Command{
    Use:   "mcp-server",
    Short: "Run Crush as an MCP server",
    RunE:  runMCPServer,
}

func runMCPServer(cmd *cobra.Command, args []string) error {
    // 初始化 MCP 服务器
    server := mcp.NewServer(mcp.ServerConfig{
        Name:    "crush",
        Version: version.Version,
    })
    
    // 注册工具
    server.RegisterTool("crush", crushTool)
    server.RegisterTool("crush-reply", crushReplyTool)
    
    // 启动 stdio 传输
    return server.ServeStdio()
}
```

#### 工具定义

```go
// internal/mcp/server/tools.go
package server

import (
    "context"
    "encoding/json"
)

type CrushToolParams struct {
    Prompt           string            `json:"prompt"`
    Model            string            `json:"model,omitempty"`
    ApprovalPolicy   string            `json:"approval_policy,omitempty"`
    SandboxMode      string            `json:"sandbox_mode,omitempty"`
    Cwd              string            `json:"cwd,omitempty"`
    Config           map[string]any    `json:"config,omitempty"`
}

func (s *Server) crushTool(ctx context.Context, params json.RawMessage) (*mcp.ToolResult, error) {
    var p CrushToolParams
    if err := json.Unmarshal(params, &p); err != nil {
        return nil, err
    }
    
    // 创建新会话
    session, err := s.sessionService.Create(ctx, session.CreateParams{
        Cwd: p.Cwd,
    })
    if err != nil {
        return nil, err
    }
    
    // 运行 agent
    resultChan := make(chan AgentResult)
    go func() {
        result := s.agent.Run(ctx, session.ID, p.Prompt)
        resultChan <- result
    }()
    
    // 流式返回进度
    for event := range s.agent.Events(session.ID) {
        s.stream.Send(event)
    }
    
    result := <-resultChan
    return &mcp.ToolResult{
        ConversationID: session.ID,
        Output:         result.Output,
    }, nil
}
```

#### 事件流

```go
// internal/mcp/server/events.go
package server

type Event struct {
    Type    string `json:"type"`
    Content string `json:"content,omitempty"`
    TurnID  string `json:"turn_id,omitempty"`
    
    // 工具调用事件
    Tool      string `json:"tool,omitempty"`
    Arguments any    `json:"arguments,omitempty"`
    Result    string `json:"result,omitempty"`
}

func (s *Server) streamEvents(sessionID string, stream mcp.Stream) {
    sub := s.pubsub.Subscribe(sessionID)
    defer sub.Close()
    
    for event := range sub.Events() {
        mcpEvent := convertToMCPEvent(event)
        stream.Send(mcpEvent)
    }
}
```

### 传输协议支持

| 传输 | 说明 | 用例 |
|------|------|------|
| stdio | 标准输入输出 | 本地调用、MCP Inspector |
| HTTP/SSE | HTTP + Server-Sent Events | 远程调用、Web 集成 |

```go
// internal/mcp/transport/transport.go
package transport

type Transport interface {
    Serve(ctx context.Context, handler Handler) error
}

type StdioTransport struct{}

func (t *StdioTransport) Serve(ctx context.Context, handler Handler) error {
    // 从 stdin 读取 JSON-RPC 请求
    // 向 stdout 写入 JSON-RPC 响应
}

type HTTPTransport struct {
    Listen string
}

func (t *HTTPTransport) Serve(ctx context.Context, handler Handler) error {
    // HTTP POST 接收请求
    // SSE 返回事件流
}
```

### 配置

```json
// crush.json
{
  "mcp_server": {
    "enabled": true,
    "transports": ["stdio", "http"],
    "http": {
      "listen": "127.0.0.1:8765",
      "cors": false
    },
    "allowed_tools": ["crush", "crush-reply"],
    "default_approval_policy": "on-request"
  }
}
```

### 客户端示例

在其他工具中调用 Crush：

```json
// 其他工具的 mcp 配置
{
  "mcp_servers": {
    "crush": {
      "command": "crush",
      "args": ["mcp-server"],
      "env": {}
    }
  }
}
```

## 实施计划

| 阶段 | 任务 | 工作量 |
|------|------|--------|
| 1 | MCP 协议实现 | 3-4 天 |
| 2 | stdio 传输 | 2 天 |
| 3 | 工具注册 | 2-3 天 |
| 4 | 事件流集成 | 2-3 天 |
| 5 | HTTP 传输（可选） | 2-3 天 |
| 6 | 文档和测试 | 2 天 |

**总计**: 约 13-17 天

## 风险与缓解

| 风险 | 缓解措施 |
|------|----------|
| MCP 协议复杂 | 使用现有 Go MCP 库 |
| 安全问题 | 默认仅 stdio，HTTP 需显式启用 |
| 并发会话管理 | 每个调用独立会话 |
| 资源泄露 | 会话超时自动清理 |

## 参考资料

- [Codex advanced.md - MCP Server](https://github.com/openai/codex/blob/main/docs/advanced.md)
- [Model Context Protocol](https://modelcontextprotocol.io/)
- [MCP Go SDK](https://github.com/mark3labs/mcp-go)

---

*MCP Server Mode 可以让 Crush 成为 AI 工具生态的一部分，建议在核心功能稳定后实现。*
