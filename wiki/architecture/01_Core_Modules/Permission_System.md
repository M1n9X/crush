# Permission System & Tool Safeguards

## 🎯 Overview

Crush gates every high‑impact tool invocation through the permission service (`internal/permission/permission.go`). The service sits between the AI agent and the operating system, ensuring that file mutations, shell commands, network fetches, and MCP interactions are only executed after the user explicitly approves them (or after they have been allow‑listed). It also keeps the TUI informed of pending requests and outcomes via the pub/sub bus.

## 🧱 Service Structure

```go
type Service interface {
    pubsub.Suscriber[PermissionRequest]
    GrantPersistent(permission PermissionRequest)
    Grant(permission PermissionRequest)
    Deny(permission PermissionRequest)
    Request(opts CreatePermissionRequest) bool
    AutoApproveSession(sessionID string)
    SetSkipRequests(skip bool)
    SkipRequests() bool
    SubscribeNotifications(ctx context.Context) <-chan pubsub.Event[PermissionNotification]
}
```

- `CreatePermissionRequest` captures the session, tool call ID, tool name, action (for example `execute`, `write`, `read`), optional parameter payload, and resolved filesystem path.
- `PermissionRequest` adds a generated UUID so the service can correlate UI responses with the original request.
- The broker publishes two channels:
  - `Service.Subscribe` streams structured requests to the TUI so a dialog can be rendered.
  - `SubscribeNotifications` emits lightweight `PermissionNotification` events that pages can use to update inline indicators (e.g. letting the chat view disable a spinner once a user responds).

## 🔐 Decision Flow

1. **Allow‑list check** – When a tool calls `permissions.Request(...)`, the service first compares `toolName` and the combined `toolName:action` against the configured `allowed_tools`. Matches are auto‑approved without surfacing a dialog. This makes it easy to permanently allow safe read‑only helpers such as `view:read`.
2. **YOLO / Skip mode** – `Permissions.SetSkipRequests(true)` (surfaced in the TUI via `commands.ToggleYoloModeMsg`) bypasses every prompt. This is primarily used for scripted or non‑interactive sessions (`app.RunNonInteractive` auto‑approves per session through `AutoApproveSession`).
3. **Session auto‑approval cache** – If the user picks “Allow for session,” `GrantPersistent` stores the tuple `(tool, action, sessionID, path)` in `sessionPermissions`. Future identical operations inside that session proceed silently.
4. **Interactive prompt** – For everything else, `Request` publishes a `PermissionRequest` event, waits on an internal channel keyed by the request ID, and blocks until `Grant`/`GrantPersistent`/`Deny` is invoked from the TUI’s permissions dialog (`internal/tui/components/dialogs/permissions`).
5. **Notifications** – Regardless of outcome, a `PermissionNotification` with `{tool_call_id, granted|denied}` is emitted so running tasks (e.g. `internal/tui/page/chat`) can update status.

## 🧰 Integration Points

- **Agent tools**: Every built‑in tool in `internal/agent/tools` calls `permissions.Request` before touching the filesystem or the shell. For example, the Bash tool (`bash.go`) wraps unsafe commands (anything outside the small `safeCommands` allow‑list) with a `CreatePermissionRequest{ToolName: BashToolName, Action: "execute", Path: workingDir}`.
- **Background shell jobs**: When a long‑running job is launched, the permission metadata (working directory, human description) is attached so the UI can present meaningful context alongside the approval buttons.
- **MCP tool registration**: Dynamically discovered MCP tools inherit the same flow; `mcp-tools.go` injects the permission service when constructing each tool so that even remote capabilities are mediated.
- **TUI dialogs**: `app.setupEvents` subscribes to both permission streams (`internal/app/app.go:262-270`), so the UI receives `pubsub.Event[permission.PermissionRequest]` to show the dialog and `pubsub.Event[permission.PermissionNotification]` to update in-place indicators (see `internal/tui/tui.go:292-309`).

## 📦 Configuration Hooks

The JSON configuration file exposes:

```json
"permissions": {
  "skip_requests": false,
  "allowed_tools": [
    "view:read",
    "ls:read"
  ]
}
```

- `skip_requests` maps to `SetSkipRequests`.
- `allowed_tools` entries may be either `toolName` (applies to all actions) or `toolName:action` combinations for fine-grained control.

Because the service is initialized with `permission.NewPermissionService(cfg.WorkingDir(), skip, allowedTools)` (`internal/app/app.go:74-88`), path validation is always performed relative to the current workspace root. Requests are normalized so that accidental relative paths (“.”) are expanded to the configured working directory before hitting the UI, ensuring the user sees exactly which folder a tool wants to touch.

## 🚨 Operational Safeguards

- **Single active prompt**: `permissionService` tracks `activeRequest` so that only one dialog is displayed at a time, reducing the chance of mismatched approvals.
- **Timeout resilience**: If the user exits the UI while a request is pending, context cancellation unwinds the subscription, closes the waiting channel, and the tool receives a denial (protecting the host from stray operations).
- **Session scoping**: `AutoApproveSession` is used in non-interactive contexts and by tests to keep automation safe without globally disabling prompts.

Together these mechanisms ensure that every privileged operation the agent attempts is auditable, cancellable, and explicitly tied back to user intent.
