# Crush Development Guide

## Project Overview

Crush is a terminal-based AI coding assistant built with Go and Charm's ecosystem (Bubble Tea, Lipgloss, Bubbles). It provides multi-model LLM support, session management, LSP integration, and extensible tooling via MCP (Model Context Protocol).

## Build/Test/Lint Commands

- **Build**: `go build .` or `go run .`
- **Test**: `task test` or `go test ./...` (run single test: `go test ./internal/llm/prompt -run TestGetContextFromPaths`)
- **Update Golden Files**: `go test ./... -update` (regenerates .golden files when test output changes)
  - Update specific package: `go test ./internal/tui/components/core -update` (in this case, we're updating "core")
- **Lint**: `task lint:fix`
- **Format**: `task fmt` (gofumpt -w .)
- **Dev**: `task dev` (runs with profiling enabled)
- **Install**: `task install` (installs to GOPATH/bin)
- **Schema**: `task schema` (generates JSON schema for configuration)
- **Release**: `task release` (creates and pushes semver tag)

## Code Style Guidelines

- **Imports**: Use goimports formatting, group stdlib, external, internal packages
- **Formatting**: Use gofumpt (stricter than gofmt), enabled in golangci-lint
- **Naming**: Standard Go conventions - PascalCase for exported, camelCase for unexported
- **Types**: Prefer explicit types, use type aliases for clarity (e.g., `type AgentName string`)
- **Error handling**: Return errors explicitly, use `fmt.Errorf` for wrapping
- **Context**: Always pass context.Context as first parameter for operations
- **Interfaces**: Define interfaces in consuming packages, keep them small and focused
- **Structs**: Use struct embedding for composition, group related fields
- **Constants**: Use typed constants with iota for enums, group in const blocks
- **Testing**: Use testify's `require` package, parallel tests with `t.Parallel()`,
  `t.SetEnv()` to set environment variables. Always use `t.Tempdir()` when in
  need of a temporary directory. This directory does not need to be removed.
- **JSON tags**: Use snake_case for JSON field names
- **File permissions**: Use octal notation (0o755, 0o644) for file permissions
- **Comments**: End comments in periods unless comments are at the end of the line.

## Architecture

### Core Components

- **Agent System** (`internal/agent/`): Orchestrates AI interactions, session management, and tool execution
- **TUI Layer** (`internal/tui/`): Terminal UI using Bubble Tea framework
- **Database Layer** (`internal/db/`): SQLite database with sqlc-generated code
- **LSP Integration** (`internal/lsp/`): Language Server Protocol client
- **MCP Integration** (`internal/agent/tools/mcp/`): Model Context Protocol tools
- **Configuration** (`internal/config/`): Configuration management and provider setup

### Key Patterns

- **Session-based Architecture**: All operations are scoped to sessions
- **Tool System**: Extensible tool framework with built-in and MCP tools
- **Provider Abstraction**: Multi-model LLM support via fantasy library
- **Event-driven**: Pub/sub system for component communication
- **Context Propagation**: Heavy use of context.Context for request lifecycle

## Testing with Mock Providers

When writing tests that involve provider configurations, use the mock providers to avoid API calls:

```go
func TestYourFunction(t *testing.T) {
    // Enable mock providers for testing
    originalUseMock := config.UseMockProviders
    config.UseMockProviders = true
    defer func() {
        config.UseMockProviders = originalUseMock
        config.ResetProviders()
    }()

    // Reset providers to ensure fresh mock data
    config.ResetProviders()

    // Your test code here - providers will now return mock data
    providers := config.Providers()
    // ... test logic
}
```

## Database

- **SQLite** database with migrations in `internal/db/migrations/`
- **sqlc** generates type-safe Go code from SQL queries
- **Goose** handles database migrations
- Database file stored in `~/.crush/crush.db`

## Configuration

- **Primary config**: `crush.json` in project root or `~/.crush/crush.json`
- **LSP configuration**: Supports multiple language servers per project
- **Provider configuration**: Model endpoints and API keys
- **Context files**: Searches for various agent instruction files (see `internal/config/config.go:25-42`)

## Tool System

### Built-in Tools
- File operations (read, write, edit, glob)
- Shell execution (bash, background jobs)
- Code analysis (grep, references, diagnostics)
- Web fetching (fetch, agentic_fetch)
- Download utility

### MCP Tools
- External tool integration via Model Context Protocol
- Supports http, stdio, and sse transport methods
- Configured in `crush.json` under `mcp` section

## Golden File Testing

Extensive use of golden files for UI component testing:
- Located in `testdata/` directories throughout the codebase
- Use `go test ./... -update` to regenerate after changes
- Test rendering output at different dimensions and states

## Formatting

- ALWAYS format any Go code you write.
  - First, try `gofumpt -w .`.
  - If `gofumpt` is not available, use `goimports`.
  - If `goimports` is not available, use `gofmt`.
  - You can also use `task fmt` to run `gofumpt -w .` on the entire project,
    as long as `gofumpt` is on the `PATH`.

## Comments

- Comments that live on their own lines should start with capital letters and
  end with periods. Wrap comments at 78 columns.

## Committing

- ALWAYS use semantic commits (`fix:`, `feat:`, `chore:`, `refactor:`, `docs:`, `sec:`, etc).
- Try to keep commits to one line, not including your attribution. Only use
  multi-line commits when additional context is truly necessary.

## Environment Variables

- `CRUSH_PROFILE`: Enables pprof profiling on localhost:6060
- `CGO_ENABLED=0`: Set in Taskfile for static builds
- `GOEXPERIMENT=greenteagc`: Go experiment flag for better GC performance
