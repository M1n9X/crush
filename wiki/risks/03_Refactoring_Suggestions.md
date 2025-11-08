# Refactoring Suggestions & Code Quality Improvements

The items below are based on the current Go implementation (June 2025). Each entry names the relevant file, why it matters, and how to approach the improvement.

## 1. Retire the Global Configuration Singleton (🟠 High)

- **Location**: `internal/config/init.go`, `internal/app/app.go`, any file calling `config.Get()` or `config.HasInitialDataConfig()`.
- **Issue**: Configuration is stored in a package-level variable with TODO comments (“remove the global config instance…”). This makes it hard to reason about state in tests and leads to hidden dependencies throughout the codebase (e.g., tools pull values from `config.Get()` even though the coordinator already has a concrete config object).
- **Suggested approach**:
  1. Expose a lightweight `type Provider interface { Current() *Config }` and inject it where long-lived components need hot-reload behavior.
  2. Pass the concrete config pointer down the call chain (CLI → app.New → coordinator → tool constructors) and remove remaining global reads.
  3. Update tests to build configs via helpers instead of mutating globals.
- **Benefits**: deterministic tests, easier to reason about hot reloads, clearer dependency graph.

## 2. Modularize Tool Registration (🟡 Medium)

- **Location**: `internal/agent/coordinator.go` (`buildTools`).
- **Issue**: The tool list is constructed via a long literal slice with repeated `append` calls and conditional checks. Adding a new tool requires editing the same function, making it hard to customize per-agent.
- **Suggested approach**:
  - Introduce a `type ToolFactory func(*Dependencies) fantasy.AgentTool` and keep a registry keyed by tool name.
  - Let `config.Agent.AllowedTools` map directly to registry entries, so future agents can select subsets without editing the coordinator.
  - Provide shared dependencies (`WorkingDir`, `history.Service`, `permission.Service`, etc.) through a structured `Dependencies` object to avoid long parameter lists.
- **Benefits**: easier to add/remove tools, pave the way for optional community tool packs, enable per-agent customization.

## 3. Implement Provider Retry/Backoff (🟡 Medium)

- **Location**: `internal/agent/agent.go` (`OnRetry` callback currently contains `// TODO: implement`).
- **Issue**: Temporary provider errors (429s, network blips) are surfaced immediately. The UI shows an error even though the provider requested a retry.
- **Suggested approach**:
  - Maintain a retry counter per session request and use `time.After(delay)` to block the streaming goroutine before re-entering `fantasy.Agent.Stream`.
  - Surface the retry status to the TUI via a new pubsub event so users know the agent is waiting.
- **Benefits**: smoother experience on flaky networks, less manual re-sending of prompts.

## 4. Normalize Permission Cache (🟢 Low)

- **Location**: `internal/permission/permission.go`.
- **Issue**: `sessionPermissions` is a slice with duplicate scanning logic (two identical loops). On long sessions with many grants the slice can grow without bound and lookups remain O(n).
- **Suggested approach**:
  - Replace the slice with a `map[sessionKey]struct{}` where `sessionKey` is `{SessionID, ToolName, Action, Path}`.
  - Remove duplicate loops and guard modifications with the existing RWMutex.
- **Benefits**: faster permission checks, less memory churn, simpler code.

## 5. Guard Optional SQL Paths (🟢 Low)

- **Location**: `internal/db/sql/files.sql` (`ListNewFiles`).
- **Issue**: The query references `is_new`, which does not exist in the schema. Either delete the query or add the missing column/migration before wiring the feature.
- **Benefits**: prevents latent runtime panics and clarifies the intended data model.
