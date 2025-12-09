# OpenTelemetry 可观测性

> 来源: Codex CLI
> 优先级: 🟡 中

## 概述

Codex 集成了 OpenTelemetry (OTEL) 用于日志和遥测，提供了对 AI 代理行为的深入可观测性。这对于调试、性能优化和合规审计非常有价值。

## Codex 实现分析

### 事件类型

Codex 会发出以下类型的事件：

| 事件类型 | 描述 | 关键字段 |
|----------|------|----------|
| `codex.conversation_starts` | 会话开始 | model, approval_policy, sandbox_policy |
| `codex.api_request` | API 请求 | attempt, duration_ms, status_code |
| `codex.sse_event` | 流式响应事件 | event.kind, input/output tokens |
| `codex.user_prompt` | 用户输入 | prompt_length, prompt (可选) |
| `codex.tool_decision` | 工具审批决策 | tool_name, decision, source |
| `codex.tool_result` | 工具执行结果 | tool_name, duration_ms, success |

### 通用元数据

每个事件都包含：

- `conversation.id`
- `app.version`
- `model`
- `auth_mode`
- `user.account_id` (可选)
- `terminal.type`

### 配置

```toml
# ~/.codex/config.toml
[otel]
environment = "staging"        # 环境标识
exporter = "otlp-http"         # none | otlp-http | otlp-grpc
log_user_prompt = false        # 是否记录用户提示词

[otel.exporter."otlp-http"]
endpoint = "https://otel.example.com/v1/logs"
protocol = "binary"

[otel.exporter."otlp-http".headers]
"x-otlp-api-key" = "${OTLP_TOKEN}"

[otel.exporter."otlp-http".tls]
ca-certificate = "certs/otel-ca.pem"
```

### 导出器选项

| 导出器 | 描述 | 配置项 |
|--------|------|--------|
| `none` | 不导出（默认） | - |
| `otlp-http` | OTLP/HTTP 协议 | endpoint, protocol, headers, tls |
| `otlp-grpc` | OTLP/gRPC 协议 | endpoint, headers |

## 借鉴价值

### 对 Crush 的好处

1. **调试能力**：追踪 Agent 决策过程
2. **性能分析**：识别瓶颈（API 延迟、工具执行时间）
3. **成本监控**：Token 使用统计
4. **合规审计**：记录所有 AI 操作
5. **运维**：集成到现有监控系统

### 实现建议

#### 事件定义

```go
// internal/otel/events.go
package otel

import (
    "time"
)

type ConversationStartEvent struct {
    Model           string `json:"model"`
    ProviderName    string `json:"provider_name"`
    ApprovalPolicy  string `json:"approval_policy"`
    SandboxPolicy   string `json:"sandbox_policy"`
    MCPServers      string `json:"mcp_servers"`
    ContextWindow   int    `json:"context_window,omitempty"`
}

type APIRequestEvent struct {
    Attempt       int    `json:"attempt"`
    DurationMs    int64  `json:"duration_ms"`
    StatusCode    int    `json:"http.response.status_code,omitempty"`
    ErrorMessage  string `json:"error.message,omitempty"`
}

type ToolDecisionEvent struct {
    ToolName   string `json:"tool_name"`
    CallID     string `json:"call_id"`
    Decision   string `json:"decision"`  // approved, denied, abort
    Source     string `json:"source"`    // config, user
}

type ToolResultEvent struct {
    ToolName    string `json:"tool_name"`
    CallID      string `json:"call_id,omitempty"`
    DurationMs  int64  `json:"duration_ms"`
    Success     bool   `json:"success"`
    Output      string `json:"output,omitempty"`
}

type TokenUsageEvent struct {
    InputTokens     int `json:"input_token_count"`
    OutputTokens    int `json:"output_token_count"`
    CachedTokens    int `json:"cached_token_count,omitempty"`
    ReasoningTokens int `json:"reasoning_token_count,omitempty"`
}
```

#### Tracer 接口

```go
// internal/otel/tracer.go
package otel

import (
    "context"
)

type Tracer interface {
    // 会话级
    ConversationStart(ctx context.Context, event ConversationStartEvent)
    ConversationEnd(ctx context.Context)
    
    // API 调用
    APIRequest(ctx context.Context, event APIRequestEvent)
    
    // 工具调用
    ToolDecision(ctx context.Context, event ToolDecisionEvent)
    ToolResult(ctx context.Context, event ToolResultEvent)
    
    // Token 使用
    TokenUsage(ctx context.Context, event TokenUsageEvent)
    
    // 用户输入（受隐私控制）
    UserPrompt(ctx context.Context, promptLength int, prompt string)
}

// 无操作 tracer（默认）
type NoopTracer struct{}

func (t *NoopTracer) ConversationStart(ctx context.Context, event ConversationStartEvent) {}
// ... 其他方法
```

#### OTLP 导出器

```go
// internal/otel/exporter.go
package otel

import (
    "go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
    "go.opentelemetry.io/otel/sdk/log"
)

type OTLPExporter struct {
    provider *log.LoggerProvider
}

func NewOTLPExporter(config OTLPConfig) (*OTLPExporter, error) {
    exporter, err := otlploghttp.New(
        context.Background(),
        otlploghttp.WithEndpoint(config.Endpoint),
        otlploghttp.WithHeaders(config.Headers),
    )
    if err != nil {
        return nil, err
    }
    
    provider := log.NewLoggerProvider(
        log.WithProcessor(log.NewBatchProcessor(exporter)),
    )
    
    return &OTLPExporter{provider: provider}, nil
}

func (e *OTLPExporter) Emit(record EventRecord) {
    logger := e.provider.Logger("crush")
    // 发送日志记录
}
```

#### 配置扩展

```go
// internal/config/otel.go
package config

type OTELConfig struct {
    Enabled       bool              `json:"enabled"`
    Environment   string            `json:"environment"`
    Exporter      string            `json:"exporter"`       // none, otlp-http, otlp-grpc
    LogUserPrompt bool              `json:"log_user_prompt"`
    
    // OTLP 配置
    Endpoint      string            `json:"endpoint,omitempty"`
    Headers       map[string]string `json:"headers,omitempty"`
    TLS           TLSConfig         `json:"tls,omitempty"`
}
```

### 事件流集成

```go
// internal/agent/agent.go
func (a *Agent) Run(ctx context.Context, sessionID, prompt string) {
    // 记录会话开始
    a.tracer.ConversationStart(ctx, otel.ConversationStartEvent{
        Model:          a.config.Model,
        ApprovalPolicy: a.config.ApprovalPolicy,
        SandboxPolicy:  a.config.SandboxMode,
    })
    defer a.tracer.ConversationEnd(ctx)
    
    // 记录用户输入
    if a.otelConfig.LogUserPrompt {
        a.tracer.UserPrompt(ctx, len(prompt), prompt)
    } else {
        a.tracer.UserPrompt(ctx, len(prompt), "[redacted]")
    }
    
    // ... agent 执行逻辑 ...
}

func (a *Agent) callTool(ctx context.Context, tool Tool, params any) (*ToolResult, error) {
    start := time.Now()
    
    result, err := tool.Execute(ctx, params)
    
    a.tracer.ToolResult(ctx, otel.ToolResultEvent{
        ToolName:   tool.Name(),
        DurationMs: time.Since(start).Milliseconds(),
        Success:    err == nil,
    })
    
    return result, err
}
```

### 隐私保护

```go
// internal/otel/privacy.go
package otel

// 敏感信息脱敏
func SanitizeOutput(output string, maxLength int) string {
    if len(output) > maxLength {
        return output[:maxLength] + "... [truncated]"
    }
    return output
}

// 检查是否应该记录用户提示词
func ShouldLogPrompt(config OTELConfig) bool {
    return config.LogUserPrompt && config.Exporter != "none"
}
```

### 本地日志增强

即使不启用 OTLP 导出，也可以增强本地日志：

```go
// internal/log/structured.go
package log

type StructuredLogger struct {
    underlying *slog.Logger
    sessionID  string
}

func (l *StructuredLogger) ToolCall(name string, duration time.Duration, success bool) {
    l.underlying.Info("tool_call",
        "tool", name,
        "duration_ms", duration.Milliseconds(),
        "success", success,
        "session_id", l.sessionID,
    )
}
```

## 实施计划

| 阶段 | 任务 | 工作量 |
|------|------|--------|
| 1 | 事件定义和 Tracer 接口 | 2 天 |
| 2 | NoopTracer 和本地日志 | 1 天 |
| 3 | OTLP HTTP 导出器 | 3 天 |
| 4 | 配置系统集成 | 1 天 |
| 5 | Agent/工具集成 | 2-3 天 |
| 6 | 隐私控制 | 1 天 |
| 7 | 文档和测试 | 2 天 |

**总计**: 约 12-14 天

## 使用场景示例

### 性能监控

查看 API 调用延迟分布：

```sql
-- 假设使用支持 OTEL 的后端
SELECT 
    avg(duration_ms) as avg_latency,
    percentile(duration_ms, 0.95) as p95,
    count(*) as request_count
FROM otel_logs
WHERE event_type = 'codex.api_request'
GROUP BY date_trunc('hour', timestamp)
```

### Token 使用统计

```sql
SELECT 
    sum(input_token_count) + sum(output_token_count) as total_tokens,
    sum(output_token_count) / sum(input_token_count) as expansion_ratio
FROM otel_logs
WHERE event_type = 'codex.sse_event'
GROUP BY date_trunc('day', timestamp)
```

### 工具使用分析

```sql
SELECT 
    tool_name,
    count(*) as call_count,
    avg(duration_ms) as avg_duration,
    sum(case when success then 1 else 0 end)::float / count(*) as success_rate
FROM otel_logs
WHERE event_type = 'codex.tool_result'
GROUP BY tool_name
ORDER BY call_count DESC
```

## 参考资料

- [Codex config.md - OTEL](https://github.com/openai/codex/blob/main/docs/config.md#observability-and-telemetry)
- [OpenTelemetry Go SDK](https://github.com/open-telemetry/opentelemetry-go)
- [OTLP Specification](https://opentelemetry.io/docs/specs/otlp/)

---

*OTEL 集成是企业级可观测性的标准方案，对于生产环境部署非常重要。*
