# Tool System

Crush ships with a rich set of tools that the agent can call whenever a provider requests a `tool_call`. All built-in tools live in `internal/agent/tools` and expose the `fantasy.AgentTool` interface.

## How Tools Are Defined

```go
func NewBashTool(perm permission.Service, workingDir string, attr *config.Attribution) fantasy.AgentTool {
    return fantasy.NewAgentTool(
        BashToolName,
        string(renderDescription(attr)),
        func(ctx context.Context, params BashParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
            // request permission, run command via internal/shell, stream metadata
        },
    )
}
```

- `fantasy.NewAgentTool` takes a name, description, and strongly typed handler function. Input schemas are generated from the handler parameter (`BashParams`).
- The agent injects contextual values (`tools.SessionIDContextKey`, `tools.MessageIDContextKey`) so handlers know which session/message to update.

## Built-in Categories

| Category | Files | Notes |
| --- | --- | --- |
| Files | `view.go`, `edit.go`, `write.go`, `multiedit.go` | Use `history.Service` for snapshots and optionally `lsp.Client` for inline diagnostics. `edit`/`write` return diffs plus instructions for the agent. |
| Shell & Jobs | `bash.go`, `job_output.go`, `job_kill.go` | Integrate with `internal/shell` to run commands (foreground/background) and inspect or terminate jobs later. Safe commands auto-run; others go through permission prompts. |
| Search & Navigation | `grep.go`, `glob.go`, `ls.go` | Respect the working directory, support include patterns, and reuse ripgrep when available. |
| Network | `fetch.go`, `download.go`, `web_fetch.go` | Guarded by permission requests and trimmed output to avoid flooding the context. |
| Editor aids | `diagnostics.go`, `references.go` | Query the LSP client cache for issues or symbol references. |
| MCP proxies | `mcp-tools.go` | Convert MCP tool definitions into `fantasy.AgentTool`s so remote servers behave like native ones. |

## Permission Flow

Every tool that may modify disk or invoke external resources calls `permission.Service.Request` before acting:

```go
if !permissions.Request(permission.CreatePermissionRequest{
    SessionID:  sessionID,
    ToolCallID: call.ID,
    ToolName:   WriteToolName,
    Action:     "write",
    Path:       normalizedPath,
    Description: fmt.Sprintf("Write file %s", normalizedPath),
    Params:     WritePermissionsParams(params),
}) {
    return fantasy.ToolResponse{}, permission.ErrorPermissionDenied
}
```

- Tools pass structured metadata to the dialog so the user sees what will happen.
- Some tools (e.g., `view`, `ls`) can be allow-listed via `config.permissions.allowed_tools` to skip prompts entirely.

## History & Context

File-manipulating tools always interact with `internal/history`:

1. **Read** – `view` pulls the latest version via `history.Service.ListLatestSessionFiles` or falls back to disk.
2. **Diff** – `edit` calculates unified diffs to send back to the provider.
3. **Write** – `write`/`multiedit` call `history.Service.CreateVersion` to track changes, then update the working tree.

This ensures the agent has a consistent view of the project even when the filesystem changes outside of Crush.

## Extending the System

- Add a new Go file under `internal/agent/tools` and return a `fantasy.NewAgentTool`.
- Inject dependencies through the constructor (history, permissions, LSP clients, working directory). Reuse `coordinator.buildTools` to wire it in.
- Document the new tool in `wiki/guides/04_Extending_Tool_System.md` and add permission copy where appropriate.

For a step-by-step walkthrough, see the [Extending the Tool System guide](../guides/04_Extending_Tool_System.md).
