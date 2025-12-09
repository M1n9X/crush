# Custom Prompts 可复用提示词系统

> 来源: Codex CLI
> 优先级: 🟡 中

## 概述

Codex 的 Custom Prompts 功能允许用户将常用的指令保存为 Markdown 文件，通过 slash 命令快速调用。这减少了重复输入，提高了工作效率。

## Codex 实现分析

### 文件位置

```
~/.codex/prompts/
├── review.md           # /prompts:review
├── ticket.md           # /prompts:ticket
├── explain.md          # /prompts:explain
└── refactor.md         # /prompts:refactor
```

### 文件格式

```markdown
---
description: Request a concise git diff review
argument-hint: FILE=<path> [FOCUS=<section>]
---

Review the code in $FILE. Pay special attention to $FOCUS.
Provide actionable feedback with specific examples.
```

### 占位符系统

| 占位符 | 说明 | 示例 |
|--------|------|------|
| `$1` - `$9` | 位置参数 | `/prompts:test $1` → 第一个参数 |
| `$ARGUMENTS` | 所有位置参数 | 空格连接 |
| `$NAME` | 命名参数 | `NAME=value` |
| `$$` | 转义 $ 符号 | 输出字面 `$` |

### 调用方式

```bash
# 使用命名参数
/prompts:ticket TICKET_ID=JIRA-1234 TICKET_TITLE="Fix login bug"

# 使用位置参数
/prompts:explain $1 src/auth.js

# 混合使用
/prompts:review FILE=src/main.go FOCUS="error handling"
```

### 验证机制

- 如果提示词包含命名占位符，调用时必须全部提供
- 缺少参数时显示验证错误
- 带空格的值需要用双引号包裹

## 借鉴价值

### 对 Crush 的好处

1. **减少重复**：常用指令只需定义一次
2. **团队共享**：项目级提示词可以纳入版本控制
3. **标准化**：统一团队的代码审查、重构等流程
4. **易于维护**：Markdown 格式易于编辑

### 实现建议

#### 文件位置策略

```
~/.crush/prompts/           # 用户全局
.crush/prompts/             # 项目级（支持版本控制）
```

#### 加载逻辑

```go
// internal/prompt/loader.go
package prompt

import (
    "os"
    "path/filepath"
)

type Prompt struct {
    Name        string            `yaml:"name"`
    Description string            `yaml:"description"`
    ArgHint     string            `yaml:"argument-hint"`
    Body        string            // markdown body
    Source      string            // home/project
}

func LoadPrompts(workingDir string) ([]Prompt, error) {
    var prompts []Prompt
    
    // 1. 加载用户全局 prompts
    homeDir, _ := os.UserHomeDir()
    globalDir := filepath.Join(homeDir, ".crush", "prompts")
    prompts = append(prompts, loadFromDir(globalDir, "home")...)
    
    // 2. 加载项目级 prompts（覆盖同名）
    projectDir := filepath.Join(workingDir, ".crush", "prompts")
    prompts = append(prompts, loadFromDir(projectDir, "project")...)
    
    return prompts, nil
}
```

#### 占位符解析

```go
// internal/prompt/expand.go
package prompt

import (
    "regexp"
    "strings"
)

var namedPlaceholder = regexp.MustCompile(`\$([A-Z][A-Z0-9_]*)`)
var positionalPlaceholder = regexp.MustCompile(`\$([1-9])`)

func (p *Prompt) Expand(args map[string]string) (string, error) {
    result := p.Body
    
    // 替换命名占位符
    result = namedPlaceholder.ReplaceAllStringFunc(result, func(match string) string {
        key := match[1:] // 去掉 $
        if val, ok := args[key]; ok {
            return val
        }
        return match // 保持原样（验证时会报错）
    })
    
    // 验证所有占位符都被替换
    remaining := namedPlaceholder.FindAllString(result, -1)
    if len(remaining) > 0 {
        return "", fmt.Errorf("missing required arguments: %v", remaining)
    }
    
    // 处理 $$ 转义
    result = strings.ReplaceAll(result, "$$", "$")
    
    return result, nil
}
```

#### Slash 命令集成

```go
// internal/tui/commands/prompts.go
package commands

type PromptsCommand struct {
    promptLoader *prompt.Loader
}

func (c *PromptsCommand) Complete(input string) []Suggestion {
    prompts, _ := c.promptLoader.LoadPrompts(workingDir)
    
    var suggestions []Suggestion
    for _, p := range prompts {
        suggestions = append(suggestions, Suggestion{
            Name:        "prompts:" + p.Name,
            Description: p.Description,
            ArgHint:     p.ArgHint,
        })
    }
    return suggestions
}

func (c *PromptsCommand) Execute(name string, rawArgs string) (string, error) {
    prompt, err := c.promptLoader.Get(name)
    if err != nil {
        return "", err
    }
    
    args, err := parseArgs(rawArgs)
    if err != nil {
        return "", err
    }
    
    return prompt.Expand(args)
}
```

#### 参数解析

```go
// internal/prompt/args.go
package prompt

import (
    "regexp"
    "strings"
)

// 支持 KEY=value 和 KEY="value with spaces"
var argPattern = regexp.MustCompile(`([A-Z][A-Z0-9_]*)=(?:"([^"]+)"|(\S+))`)

func ParseArgs(input string) (map[string]string, error) {
    args := make(map[string]string)
    
    matches := argPattern.FindAllStringSubmatch(input, -1)
    for _, m := range matches {
        key := m[1]
        value := m[2]
        if value == "" {
            value = m[3]
        }
        args[key] = value
    }
    
    return args, nil
}
```

### TUI 集成

#### 补全弹窗

在输入 `/` 时显示可用 prompts：

```
┌─────────────────────────────────────────────────────┐
│ /prompts:                                           │
├─────────────────────────────────────────────────────┤
│ ▶ review     Review code in a specific file        │
│   ticket     Generate commit message for a ticket   │
│   explain    Explain code with examples             │
│   refactor   Suggest refactoring improvements       │
└─────────────────────────────────────────────────────┘
```

#### 参数提示

选择 prompt 后显示参数提示：

```
/prompts:review FILE=<path> [FOCUS=<section>]
                ^^^^^^^^^^^^^^^^^^^^^^^^^^^^
                Hint: Required arguments shown
```

### 预设 Prompts

提供一些开箱即用的 prompts：

```markdown
<!-- ~/.crush/prompts/review.md -->
---
description: Review code for bugs and improvements
argument-hint: FILE=<path>
---

Please review the code in $FILE. Focus on:
1. Potential bugs and edge cases
2. Performance issues
3. Code style and readability
4. Security vulnerabilities

Provide specific, actionable feedback with code examples.
```

```markdown
<!-- ~/.crush/prompts/explain.md -->
---
description: Explain code with examples
argument-hint: FILE=<path>
---

Please explain the code in $FILE:
1. What is its purpose?
2. How does it work step by step?
3. What are the key design decisions?
4. Give usage examples if applicable.
```

```markdown
<!-- ~/.crush/prompts/test.md -->
---
description: Generate tests for a file
argument-hint: FILE=<path> [FRAMEWORK=<name>]
---

Generate comprehensive tests for $FILE using $FRAMEWORK (or infer the best framework).
Include:
1. Unit tests for each public function
2. Edge cases and error conditions
3. Mock dependencies appropriately
4. Add helpful test descriptions
```

## 实施计划

| 阶段 | 任务 | 工作量 |
|------|------|--------|
| 1 | Prompt 加载器实现 | 1-2 天 |
| 2 | 占位符解析和展开 | 1-2 天 |
| 3 | Slash 命令集成 | 2 天 |
| 4 | TUI 补全集成 | 2 天 |
| 5 | 预设 prompts 库 | 1 天 |
| 6 | 文档和测试 | 1 天 |

**总计**: 约 8-10 天

## 与现有 Subagent 的关系

Custom Prompts 和 Subagent 是互补的功能：

| 特性 | Custom Prompts | Subagent |
|------|----------------|----------|
| 用途 | 快速发送预定义提示 | 执行独立任务 |
| 工具访问 | 主 agent 工具 | 受限工具集 |
| 会话方式 | 在当前会话中 | 独立单轮 |
| 定义格式 | 纯 Markdown | YAML frontmatter + tools |

可以组合使用：

```markdown
<!-- ~/.crush/prompts/analyze-codebase.md -->
---
description: Deep analysis using subagent
---

Use the @general subagent to perform a comprehensive analysis of this codebase.
Focus on architecture, patterns, and potential improvements.
```

## 参考资料

- [Codex prompts.md](https://github.com/openai/codex/blob/main/docs/prompts.md)

---

*Custom Prompts 是提升日常工作效率的实用功能，实现相对简单，建议在稳定版后尽快添加。*
