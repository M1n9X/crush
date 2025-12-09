# Memory System

## 🧠 Overview

The Memory system in Crush provides persistent per-agent memory storage, allowing agents to maintain long-term notes and context across sessions. This is implemented in `internal/memory/service.go`.

## 📁 Storage Location

Memory files are stored under:

```bash
<data_directory>/memory/agents/<agent>/
```

For example:

- `~/.local/share/crush/memory/agents/default/notes.md`
- `~/.local/share/crush/memory/agents/task/memory.md`

## 🔧 Core Components

### Service

```go
// Service manages per-agent memory files stored under the data directory.
type Service struct {
    root string
}

// NewService constructs a memory service rooted at dataDir/memory/agents.
func NewService(dataDir string) *Service
```

### Entry

```go
// Entry represents a memory file on disk.
type Entry struct {
    Path      string  // User-friendly path (e.g. memory/<agent>/notes.md)
    FullPath  string  // Absolute path on disk
    Content   string  // File body (possibly truncated)
    Truncated bool    // Whether content was shortened to respect limits
}
```

## 🔑 API Methods

### List

Returns memory entries for an agent, bounded by count/size budgets.

```go
func (s *Service) List(agent string) ([]Entry, error)
```

- Returns up to `maxMemoryFiles` (10) entries
- Total size capped at `maxTotalMemoryBytes` (256KB) across all returned files; content is truncated mid-list if needed to stay within the budget
- Entries sorted by modification time (freshest first)

### Read

Loads a single memory entry for an agent.

```go
func (s *Service) Read(agent, filePath string) (Entry, error)
```

- Single file size capped at `maxSingleMemoryFileSize` (64KB)
- Content truncated if exceeding limit

### Write

Persists a memory entry for an agent.

```go
func (s *Service) Write(agent, filePath, content string) (Entry, error)
```

- Creates directories as needed
- Uses secure file permissions (0600)
- Path sanitization prevents directory traversal

## 🛡️ Security Features

1. **Agent Name Sanitization**: Agent names are sanitized using regex `[^a-zA-Z0-9._-]+` to prevent path injection
2. **Path Traversal Prevention**: Paths containing `..` are rejected
3. **Root Escape Detection**: Resolved paths must stay within agent root directory
4. **Secure Permissions**: Memory files created with mode 0600 (owner read/write only)

## 📊 Limits & Budgets

| Limit | Value | Purpose |
|-------|-------|---------|
| `maxMemoryFiles` | 10 | Maximum number of memory entries per agent |
| `maxTotalMemoryBytes` | 256KB | Total memory content size limit (budget enforced across returned entries; content may be truncated) |
| `maxSingleMemoryFileSize` | 64KB | Maximum single file size |

## 🔗 Integration with Agent System

Memory files are:

- Injected into the system prompt at each turn
- Not counted in freshness tracking (to avoid being replayed during auto-compaction)
- Persisted independently from session history

### Memory Tools

The agent exposes two tools for memory management:

- `memory_read` - Read memory file contents
- `memory_write` - Write/update memory files

## 🔌 Related Docs

- [Auto-Compaction](../../features/memory-autocompact.md) - How memory interacts with context compression
- [Agent System](AI_Agent_System.md) - How memory is injected into prompts
- [Business Workflows](../02_Business_Workflows.md) - Session and memory lifecycle
