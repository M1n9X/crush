# Known Bugs & Limitations

This document captures concrete gaps observed in the current codebase. Each entry includes a link to the relevant file and suggested remediation.

## 1. `ListNewFiles` Query References Missing Column (🟠 High)

- **Location**: `internal/db/sql/files.sql` (`ListNewFiles`) and generated code `internal/db/files.sql.go`.
- **Issue**: The query filters on `WHERE is_new = 1`, but the `files` table defined in `internal/db/migrations/20250424200609_initial.sql` does not contain an `is_new` column. If the query is ever executed, SQLite will error with `no such column: is_new`.
- **Impact**: Any feature that tries to call `Queries.ListNewFiles` will panic at runtime.
- **Suggested fix**: Either remove the query or add a migration that introduces `is_new` with the intended semantics (e.g., `BOOLEAN DEFAULT 0`).

## 2. Retry Logic Not Wired (`sessionAgent` OnRetry) (🟡 Medium)

- **Location**: `internal/agent/agent.go` line ~309.
- **Issue**: The `OnRetry` callback in `fantasy.AgentStreamCall` contains `// TODO: implement`. The agent therefore ignores retry hints from providers (e.g., rate-limits, throttling) and immediately surfaces the error to the user.
- **Impact**: Temporary provider errors (HTTP 429, network hiccups) bubble straight to the UI instead of being retried automatically.
- **Suggested fix**: Implement exponential backoff inside `OnRetry`, log the wait time via `a.eventPromptRetry`, and surface a status message in the TUI.

## 3. Non-Interactive `crush run` Requires Double Ctrl+C (🟢 Low)

- **Location**: `internal/cmd/run.go` lines 55–61.
- **Issue**: Marked with `// TODO: We currently need to press ^c twice to cancel. Fix that.`. Because the command installs a signal handler that triggers after the first interrupt, the user must send the signal twice for the program to exit.
- **Impact**: Minor usability annoyance when running long prompts from scripts.
- **Suggested fix**: Wire the interrupt handler to cancel the context and exit the process on the first signal (or reuse the logic from the interactive command).

## 4. Idle MCP Sessions Are Never Reaped (🟢 Low)

- **Location**: `internal/agent/tools/mcp/init.go` (`getOrRenewClient`).
- **Issue**: When an MCP server dies, `getOrRenewClient` attempts to `RenewSession`, but failed renewals leave the old state in `sessions`/`states`. There is no background job that removes permanently dead sessions.
- **Impact**: The TUI may continue to show stale “connected” MCP entries even though the underlying process is gone, and repeated failures keep consuming reconnection attempts.
- **Suggested fix**: Periodically verify each session (e.g., ping) or drop it from the map after `RenewSession` fails N times.

---

If you encounter additional issues, add them here with a code reference and a proposed remediation so contributors know what to tackle.
