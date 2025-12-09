# 多语言 SDK 自动生成

> 来源: Codex CLI, OpenCode
> 优先级: 🟢 低

## 概述

Codex 和 OpenCode 都提供了多语言 SDK，让开发者可以在自己的应用中集成 AI 编程能力。这些 SDK 通常是从 API 规范自动生成的。

## 现有实现分析

### Codex

Codex 提供 TypeScript SDK：

```
codex/sdk/typescript/
├── src/
│   ├── index.ts
│   ├── codex.ts
│   └── types.ts
├── package.json
└── README.md
```

### OpenCode

OpenCode 通过 [Stainless](https://www.stainless.com/) 生成 SDK：

```
opencode/packages/sdk/
├── js/           # JavaScript/TypeScript SDK
│   ├── src/
│   └── package.json
└── go/           # Go SDK
    ├── client.go
    └── go.mod
```

**SDK 使用示例**：

```typescript
// TypeScript
import { OpenCode } from "@opencode/sdk";

const client = new OpenCode({ apiKey: "..." });
const session = await client.sessions.create({
  projectId: "my-project",
});
await client.messages.send(session.id, "Explain this code");
```

```go
// Go
import "github.com/opencode/sdk-go"

client := opencode.NewClient(opencode.WithAPIKey("..."))
session, _ := client.Sessions.Create(ctx, opencode.CreateSessionParams{
    ProjectID: "my-project",
})
```

## 对 Crush 的适用性分析

### 现状评估

Crush 目前是一个**独立的 CLI/TUI 应用**，不提供外部 API。因此：

| 方面 | 现状 | SDK 需求 |
|------|------|----------|
| 架构 | 单进程，无 HTTP API | 低 |
| 使用模式 | 终端交互 | 低 |
| 集成需求 | 通过 MCP 协议 | 可选 |
| 自动化 | 非交互模式 | 可选 |

### 何时需要 SDK

SDK 变得必要的场景：

1. **Client-Server 架构**：如果 Crush 未来采用类似 OpenCode 的架构
2. **远程控制**：移动/Web 客户端控制 Crush
3. **API 服务化**：Crush 作为服务运行
4. **深度集成**：其他应用需要代码级集成

### 当前替代方案

在没有正式 SDK 的情况下：

1. **MCP 协议**：通过 MCP Server Mode 提供工具调用（见 04_MCP_Server_Mode.md）
2. **CLI 包装**：通过子进程调用 `crush exec`
3. **非交互模式**：`crush --non-interactive` 用于脚本

## 可能的实现路径

### 方案一：轻量级 Go 库

将核心逻辑抽取为可导入的 Go 包：

```go
// github.com/charmbracelet/crush/pkg/crush

package crush

type Client struct {
    config Config
}

func NewClient(opts ...Option) *Client {
    // ...
}

func (c *Client) Run(ctx context.Context, prompt string) (*Result, error) {
    // 核心 agent 逻辑
}

func (c *Client) Chat(ctx context.Context, sessionID, prompt string) (*Message, error) {
    // 会话式交互
}
```

**使用示例**：

```go
package main

import (
    "context"
    "github.com/charmbracelet/crush/pkg/crush"
)

func main() {
    client := crush.NewClient(
        crush.WithModel("claude-3-5-sonnet"),
        crush.WithWorkingDir("/my/project"),
    )
    
    result, _ := client.Run(context.Background(), "Fix the tests")
    fmt.Println(result.Output)
}
```

### 方案二：HTTP API + 生成的 SDK

如果 Crush 演进为 Client-Server 架构：

```yaml
# api-spec.yaml (OpenAPI)
openapi: 3.0.0
info:
  title: Crush API
  version: 1.0.0
paths:
  /sessions:
    post:
      summary: Create a new session
      requestBody:
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/CreateSessionRequest'
      responses:
        200:
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/Session'
  /sessions/{id}/messages:
    post:
      summary: Send a message
      # ...
```

使用工具生成 SDK：

- [oapi-codegen](https://github.com/deepmap/oapi-codegen) (Go)
- [openapi-typescript](https://github.com/drwpow/openapi-typescript) (TypeScript)
- [Stainless](https://www.stainless.com/) (商业服务)

### 方案三：MCP 优先

依赖 MCP 协议作为 SDK 替代：

```typescript
// 其他应用通过 MCP 调用 Crush
import { Client } from "@modelcontextprotocol/sdk/client/index.js";

const client = new Client({
  name: "my-app",
  version: "1.0.0",
});

await client.connect(new StdioClientTransport({
  command: "crush",
  args: ["mcp-server"],
}));

const result = await client.callTool({
  name: "crush",
  arguments: { prompt: "Fix the tests" },
});
```

## 实施建议

### 短期（当前）

**不需要 SDK**。专注于：

- MCP Server Mode（04_MCP_Server_Mode.md）
- 非交互模式增强
- CLI 稳定性

### 中期（如果需求增长）

**Go 库抽取**：

1. 识别核心可复用组件
2. 创建 `pkg/crush` 公共包
3. 保持 CLI 作为该包的消费者
4. 添加文档和示例

### 长期（Client-Server 演进）

**完整 SDK 生成**：

1. 定义 OpenAPI 规范
2. 选择 SDK 生成工具
3. 建立 CI/CD 流程保持 SDK 同步
4. 发布到包管理器（npm, pkg.go.dev）

## 工作量估算

| 方案 | 工作量 | 适用场景 |
|------|--------|----------|
| Go 库抽取 | 5-8 天 | Go 生态集成 |
| HTTP API 设计 | 3-5 天 | API 服务化前提 |
| TypeScript SDK 生成 | 2-3 天 | 需要 HTTP API |
| Go SDK 生成 | 2-3 天 | 需要 HTTP API |
| 文档和示例 | 2-3 天 | 任何方案 |

## 参考资料

- [OpenCode SDK](https://github.com/sst/opencode/tree/main/packages/sdk)
- [Codex TypeScript SDK](https://github.com/openai/codex/tree/main/sdk/typescript)
- [Stainless API Platform](https://www.stainless.com/)
- [oapi-codegen](https://github.com/deepmap/oapi-codegen)

---

*SDK 生成目前不是优先事项。建议先通过 MCP Server Mode 满足集成需求，未来根据用户反馈决定是否投入。*
