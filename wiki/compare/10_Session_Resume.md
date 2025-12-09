# 会话恢复与历史管理

> 来源: Codex CLI
> 优先级: 🟡 中

## 概述

Codex 提供了丰富的会话恢复功能，包括 CLI 命令、会话选择器、Git 分支关联等。这对于长期项目和中断恢复非常有用。

## Codex 实现分析

### 会话恢复命令

```bash
# 打开会话选择器
codex resume

# 恢复最近的会话
codex resume --last

# 恢复指定会话
codex resume <SESSION_ID>
```

### 会话选择器

```
┌─────────────────────────────────────────────────────────────────┐
│ Recent Sessions                                                 │
├─────────────────────────────────────────────────────────────────┤
│ ▶ 7f9f9a2e  2024-12-09 14:30  /Users/me/project  [main]        │
│   a1b2c3d4  2024-12-09 12:15  /Users/me/other    [feature]     │
│   e5f6g7h8  2024-12-08 16:45  /Users/me/project  [fix-bug]     │
└─────────────────────────────────────────────────────────────────┘
```

**显示信息**：

- 会话 ID（短格式）
- 创建时间
- 工作目录
- Git 分支（如果适用）

### 会话存储

```
~/.codex/sessions/
├── 7f9f9a2e-1b3c-4c7a-9b0e-123456789abc/
│   ├── metadata.json
│   └── messages.json
└── a1b2c3d4-5e6f-7890-abcd-ef0123456789/
    ├── metadata.json
    └── messages.json
```

### /status 命令

在会话内查看状态：

```
Session: 7f9f9a2e-1b3c-4c7a-9b0e-123456789abc
Created: 2024-12-09 14:30:00
Working Directory: /Users/me/project
Git Branch: main
Model: gpt-4
Approval Policy: on-request
Sandbox: workspace-write
Writable Roots:
  - /Users/me/project
  - /tmp
```

## Crush 现状

### 已有功能

Crush 通过 SQLite 持久化会话：

- ✅ 会话创建和保存
- ✅ 消息历史存储
- ✅ 会话切换（TUI 内）
- ✅ 会话摘要

### 缺失功能

| 功能 | Codex | Crush |
|------|-------|-------|
| CLI resume 命令 | ✅ | ❌ |
| 会话选择器 TUI | ✅ | ⚠️ 基础 |
| --last 快捷方式 | ✅ | ❌ |
| Git 分支关联 | ✅ | ❌ |
| /status 命令 | ✅ | ⚠️ 部分 |
| 会话搜索/过滤 | ✅ | ❌ |
| 会话导出 | ❌ | ❌ |

## 借鉴建议

### 1. CLI resume 命令

```bash
# 打开选择器
crush resume

# 恢复最近
crush resume --last

# 恢复指定 ID
crush resume abc-123

# 按目录过滤
crush resume --cwd /Users/me/project

# 按分支过滤
crush resume --branch main
```

**实现**：

```go
// internal/cmd/resume.go
package cmd

var resumeCmd = &cobra.Command{
    Use:   "resume [session-id]",
    Short: "Resume a previous session",
    RunE: func(cmd *cobra.Command, args []string) error {
        last, _ := cmd.Flags().GetBool("last")
        cwd, _ := cmd.Flags().GetString("cwd")
        branch, _ := cmd.Flags().GetString("branch")
        
        sessions, err := sessionService.List(session.ListParams{
            Cwd:    cwd,
            Branch: branch,
        })
        if err != nil {
            return err
        }
        
        var sessionID string
        if len(args) > 0 {
            sessionID = args[0]
        } else if last {
            sessionID = sessions[0].ID
        } else {
            // 显示选择器
            sessionID = showSessionPicker(sessions)
        }
        
        return runTUI(sessionID)
    },
}

func init() {
    resumeCmd.Flags().BoolP("last", "l", false, "Resume most recent session")
    resumeCmd.Flags().String("cwd", "", "Filter by working directory")
    resumeCmd.Flags().String("branch", "", "Filter by git branch")
}
```

### 2. Git 分支关联

在会话创建时记录当前 Git 分支：

```go
// internal/session/session.go
type Session struct {
    ID          string    `db:"id"`
    CreatedAt   time.Time `db:"created_at"`
    UpdatedAt   time.Time `db:"updated_at"`
    Cwd         string    `db:"cwd"`
    GitBranch   string    `db:"git_branch"`   // 新增
    GitRemote   string    `db:"git_remote"`   // 新增
    Summary     string    `db:"summary"`
    // ...
}

func (s *Service) Create(ctx context.Context, cwd string) (*Session, error) {
    session := &Session{
        ID:        uuid.New().String(),
        CreatedAt: time.Now(),
        Cwd:       cwd,
    }
    
    // 检测 Git 信息
    if gitInfo, err := git.GetInfo(cwd); err == nil {
        session.GitBranch = gitInfo.Branch
        session.GitRemote = gitInfo.RemoteURL
    }
    
    // 保存到数据库
    return s.db.CreateSession(ctx, session)
}
```

### 3. 增强的会话选择器

```
┌─────────────────────────────────────────────────────────────────┐
│ 🔍 Filter: _                                    [↑↓] Navigate  │
├─────────────────────────────────────────────────────────────────┤
│ ▶ abc-123   2 hours ago    /Users/me/project         [main]    │
│   def-456   Yesterday      /Users/me/other           [feature] │
│   ghi-789   3 days ago     /Users/me/project         [fix-bug] │
│                                                                 │
│   Summary: Fixed authentication bug in login handler           │
├─────────────────────────────────────────────────────────────────┤
│ [Enter] Resume  [d] Delete  [e] Export  [/] Search  [Esc] Quit │
└─────────────────────────────────────────────────────────────────┘
```

**功能**：

- 搜索/过滤
- 显示摘要预览
- 删除会话
- 导出会话

### 4. 增强 /status 命令

```go
// internal/tui/commands/status.go
package commands

func (c *StatusCommand) Execute() string {
    session := c.app.CurrentSession()
    config := c.app.Config()
    
    var sb strings.Builder
    sb.WriteString("## Session Status\n\n")
    sb.WriteString(fmt.Sprintf("**Session ID**: `%s`\n", session.ID))
    sb.WriteString(fmt.Sprintf("**Created**: %s\n", formatTime(session.CreatedAt)))
    sb.WriteString(fmt.Sprintf("**Working Directory**: `%s`\n", session.Cwd))
    
    if session.GitBranch != "" {
        sb.WriteString(fmt.Sprintf("**Git Branch**: `%s`\n", session.GitBranch))
    }
    
    sb.WriteString(fmt.Sprintf("**Model**: %s (%s)\n", config.Model, config.ModelProvider))
    sb.WriteString(fmt.Sprintf("**Approval Policy**: %s\n", config.ApprovalPolicy))
    sb.WriteString(fmt.Sprintf("**Sandbox**: %s\n", config.SandboxMode))
    
    // 工具统计
    stats := c.app.GetToolStats(session.ID)
    sb.WriteString("\n### Tool Usage\n")
    for tool, count := range stats {
        sb.WriteString(fmt.Sprintf("- %s: %d calls\n", tool, count))
    }
    
    // Token 使用
    usage := c.app.GetTokenUsage(session.ID)
    sb.WriteString("\n### Token Usage\n")
    sb.WriteString(fmt.Sprintf("- Input: %d\n", usage.Input))
    sb.WriteString(fmt.Sprintf("- Output: %d\n", usage.Output))
    sb.WriteString(fmt.Sprintf("- Total: %d\n", usage.Total))
    
    return sb.String()
}
```

### 5. 会话导出

```bash
# 导出为 Markdown
crush export abc-123 --format md > session.md

# 导出为 JSON
crush export abc-123 --format json > session.json
```

**导出格式（Markdown）**：

```markdown
# Session Export

**Session ID**: abc-123
**Created**: 2024-12-09 14:30:00
**Working Directory**: /Users/me/project
**Git Branch**: main

---

## Conversation

### User (14:30:15)
Fix the authentication bug in the login handler

### Assistant (14:30:45)
I'll analyze the login handler and fix the authentication bug.

[Tool: view] Viewing `src/auth/login.go`

I found the issue. The token validation is missing...

### User (14:32:00)
Can you also add a test for this?

### Assistant (14:32:30)
...
```

## 实施计划

| 阶段 | 任务 | 工作量 |
|------|------|--------|
| 1 | Git 分支关联（数据库迁移） | 1 天 |
| 2 | CLI resume 命令 | 1-2 天 |
| 3 | 增强会话选择器 TUI | 2-3 天 |
| 4 | 增强 /status 命令 | 1 天 |
| 5 | 会话导出功能 | 1-2 天 |
| 6 | 会话搜索/过滤 | 1 天 |

**总计**: 约 7-10 天

## 数据库迁移

```sql
-- migrations/005_add_git_info.sql
ALTER TABLE sessions ADD COLUMN git_branch TEXT;
ALTER TABLE sessions ADD COLUMN git_remote TEXT;

-- 添加索引以加速查询
CREATE INDEX idx_sessions_git_branch ON sessions(git_branch);
CREATE INDEX idx_sessions_cwd ON sessions(cwd);
```

## 参考资料

- [Codex getting-started.md - Resume](https://github.com/openai/codex/blob/main/docs/getting-started.md#resuming-interactive-sessions)

---

*会话恢复增强是提升用户体验的重要功能，特别是对于长期项目开发。*
