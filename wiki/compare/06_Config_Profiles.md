# 配置 Profiles 系统

> 来源: Codex CLI
> 优先级: 🟢 低

## 概述

Codex 支持配置 Profiles，允许用户定义多组配置并在运行时快速切换。这对于在不同场景（开发/生产、不同项目、不同安全级别）间切换非常有用。

## Codex 实现分析

### Profile 定义

```toml
# ~/.codex/config.toml

# 默认配置
model = "o3"
approval_policy = "untrusted"

# 默认使用的 profile
profile = "o3"

# Profile 定义
[profiles.o3]
model = "o3"
model_provider = "openai"
approval_policy = "never"
model_reasoning_effort = "high"

[profiles.fast]
model = "gpt-4o"
model_provider = "openai"
approval_policy = "on-request"

[profiles.safe]
model = "o3"
approval_policy = "untrusted"
sandbox_mode = "read-only"

[profiles.local]
model = "mistral"
model_provider = "ollama"
approval_policy = "never"
```

### 使用方式

```bash
# 使用默认 profile
codex

# 指定 profile
codex --profile fast

# 在配置中设置默认
# profile = "o3"  在 config.toml 中
```

### 可配置项

每个 profile 可以覆盖以下配置：

| 配置项 | 描述 |
|--------|------|
| `model` | 模型名称 |
| `model_provider` | 提供商 |
| `model_reasoning_effort` | 推理深度 |
| `approval_policy` | 审批策略 |
| `sandbox_mode` | 沙箱模式 |
| `mcp_servers` | MCP 服务器 |

## 借鉴价值

### 对 Crush 的好处

1. **场景切换**：快速切换不同用例配置
2. **安全分级**：不同敏感度的配置组
3. **模型选择**：轻松切换不同模型
4. **团队共享**：通过项目配置共享 profile

### 实现建议

#### 配置结构

```go
// internal/config/profile.go
package config

type Profile struct {
    Name             string            `json:"name"`
    Model            string            `json:"model,omitempty"`
    ModelProvider    string            `json:"model_provider,omitempty"`
    ApprovalPolicy   string            `json:"approval_policy,omitempty"`
    SandboxMode      string            `json:"sandbox_mode,omitempty"`
    MCPServers       map[string]any    `json:"mcp_servers,omitempty"`
    ReasoningEffort  string            `json:"reasoning_effort,omitempty"`
    
    // 工具配置
    AllowedTools     []string          `json:"allowed_tools,omitempty"`
    DisabledTools    []string          `json:"disabled_tools,omitempty"`
}

type Config struct {
    // 基础配置
    Model          string `json:"model"`
    // ... 其他字段
    
    // Profile 相关
    DefaultProfile string              `json:"default_profile,omitempty"`
    Profiles       map[string]Profile  `json:"profiles,omitempty"`
}
```

#### Profile 解析

```go
// internal/config/resolve.go
package config

func (c *Config) ResolveProfile(profileName string) (*ResolvedConfig, error) {
    resolved := &ResolvedConfig{
        Model:          c.Model,
        ModelProvider:  c.ModelProvider,
        ApprovalPolicy: c.ApprovalPolicy,
        SandboxMode:    c.SandboxMode,
        // ... 复制基础配置
    }
    
    // 确定要使用的 profile
    name := profileName
    if name == "" {
        name = c.DefaultProfile
    }
    
    if name == "" {
        return resolved, nil  // 使用基础配置
    }
    
    profile, ok := c.Profiles[name]
    if !ok {
        return nil, fmt.Errorf("profile not found: %s", name)
    }
    
    // 覆盖配置
    if profile.Model != "" {
        resolved.Model = profile.Model
    }
    if profile.ModelProvider != "" {
        resolved.ModelProvider = profile.ModelProvider
    }
    if profile.ApprovalPolicy != "" {
        resolved.ApprovalPolicy = profile.ApprovalPolicy
    }
    if profile.SandboxMode != "" {
        resolved.SandboxMode = profile.SandboxMode
    }
    
    return resolved, nil
}
```

#### CLI 集成

```go
// internal/cmd/root.go
package cmd

var rootCmd = &cobra.Command{
    Use:   "crush",
    Short: "Terminal-based AI assistant",
}

func init() {
    rootCmd.PersistentFlags().StringP("profile", "p", "", "Configuration profile to use")
}

func getConfig(cmd *cobra.Command) (*config.ResolvedConfig, error) {
    profileName, _ := cmd.Flags().GetString("profile")
    
    cfg, err := config.Load()
    if err != nil {
        return nil, err
    }
    
    return cfg.ResolveProfile(profileName)
}
```

#### TUI Profile 切换

在命令面板或 Slash 命令中添加 profile 切换：

```go
// internal/tui/commands/profile.go
package commands

type ProfileCommand struct {
    config *config.Config
}

func (c *ProfileCommand) Complete(input string) []Suggestion {
    var suggestions []Suggestion
    for name, profile := range c.config.Profiles {
        suggestions = append(suggestions, Suggestion{
            Name:        name,
            Description: fmt.Sprintf("Model: %s, Policy: %s", profile.Model, profile.ApprovalPolicy),
        })
    }
    return suggestions
}

func (c *ProfileCommand) Execute(name string) error {
    // 切换到新 profile（需要重启会话或热重载）
    return c.switchProfile(name)
}
```

添加 Slash 命令：

```
/profile list          # 列出所有 profiles
/profile show <name>   # 显示 profile 详情
/profile use <name>    # 切换 profile（新会话生效）
```

### 配置文件示例

```json
// ~/.crush/crush.json
{
  "model": "claude-3-5-sonnet",
  "model_provider": "anthropic",
  "default_profile": "balanced",
  
  "profiles": {
    "fast": {
      "model": "gpt-4o-mini",
      "model_provider": "openai",
      "approval_policy": "on-request"
    },
    "balanced": {
      "model": "claude-3-5-sonnet",
      "model_provider": "anthropic",
      "approval_policy": "untrusted"
    },
    "powerful": {
      "model": "claude-3-opus",
      "model_provider": "anthropic",
      "approval_policy": "never",
      "reasoning_effort": "high"
    },
    "safe": {
      "model": "claude-3-haiku",
      "model_provider": "anthropic",
      "approval_policy": "untrusted",
      "sandbox_mode": "read-only",
      "disabled_tools": ["bash", "write"]
    },
    "local": {
      "model": "codellama",
      "model_provider": "ollama",
      "approval_policy": "never"
    }
  }
}
```

### 状态显示

在 TUI 状态栏显示当前 profile：

```
┌─────────────────────────────────────────────────────────────────┐
│ Crush v1.0.0  │  Profile: balanced  │  Model: claude-3-5-sonnet │
└─────────────────────────────────────────────────────────────────┘
```

或在 `/status` 输出中包含：

```
Session: abc-123
Profile: balanced
Model: claude-3-5-sonnet (anthropic)
Approval Policy: untrusted
Sandbox: workspace-write
```

## 实施计划

| 阶段 | 任务 | 工作量 |
|------|------|--------|
| 1 | Profile 数据结构 | 1 天 |
| 2 | 配置解析和合并 | 1-2 天 |
| 3 | CLI --profile 参数 | 0.5 天 |
| 4 | Slash 命令集成 | 1 天 |
| 5 | TUI 状态显示 | 0.5 天 |
| 6 | 文档和测试 | 1 天 |

**总计**: 约 5-6 天

## 扩展机会

### 项目级 Profiles

支持项目目录下的 `.crush/profiles.json`：

```json
// .crush/profiles.json
{
  "profiles": {
    "ci": {
      "model": "gpt-4o-mini",
      "approval_policy": "never",
      "sandbox_mode": "read-only"
    }
  }
}
```

### 环境变量覆盖

```bash
CRUSH_PROFILE=fast crush
```

### Profile 继承

```json
{
  "profiles": {
    "base": {
      "model": "claude-3-5-sonnet"
    },
    "safe": {
      "extends": "base",
      "sandbox_mode": "read-only"
    }
  }
}
```

## 参考资料

- [Codex config.md - Profiles](https://github.com/openai/codex/blob/main/docs/config.md#profiles)

---

*Profile 系统是一个便利功能，实现相对简单，可以在后续版本中逐步完善。*
