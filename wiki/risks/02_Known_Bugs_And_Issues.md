# Known Bugs & Limitations

This document captures concrete gaps observed in the current codebase. Each entry includes a link to the relevant file and suggested remediation.

## 1. ~~`ListNewFiles` Query References Missing Column~~ ✅ FIXED

- **Status**: Resolved on 2025-12-09
- **Resolution**: The `ListNewFiles` query was removed from `internal/db/sql/files.sql` and sqlc was regenerated. The query referenced a non-existent `is_new` column in the `files` table.

## 2. ~~Retry Logic Not Wired (`sessionAgent` OnRetry)~~ ✅ FIXED

- **Status**: Resolved
- **Resolution**: The `OnRetry` callback in `internal/agent/agent.go` is now fully implemented with error classification, user-friendly messages, stats tracking, and logging.

## 3. Non-Interactive `crush run` Requires Double Ctrl+C (🟢 Low)

- **Location**: `internal/cmd/run.go` lines 55–61.
- **Issue**: Marked with `// TODO: We currently need to press ^c twice to cancel. Fix that.`. Because the command installs a signal handler that triggers after the first interrupt, the user must send the signal twice for the program to exit.
- **Impact**: Minor usability annoyance when running long prompts from scripts.
- **Suggested fix**: Wire the interrupt handler to cancel the context and exit the process on the first signal (or reuse the logic from the interactive command).

## 4. Idle MCP Sessions Are Never Reaped (🟢 Low)

- **Location**: `internal/agent/tools/mcp/init.go` (`getOrRenewClient`).
- **Issue**: When an MCP server dies, `getOrRenewClient` attempts to `RenewSession`, but failed renewals leave the old state in `sessions`/`states`. There is no background job that removes permanently dead sessions.
- **Impact**: The TUI may continue to show stale "connected" MCP entries even though the underlying process is gone, and repeated failures keep consuming reconnection attempts.
- **Suggested fix**: Periodically verify each session (e.g., ping) or drop it from the map after `RenewSession` fails N times.

---

If you encounter additional issues, add them here with a code reference and a proposed remediation so contributors know what to tackle.
