# Security Assessment

Crush runs on the developer’s machine and exposes powerful tooling (shell, filesystem, MCP). This section catalogs the main security considerations in the current codebase and links to the specific files that enforce – or still need – protections.

## 1. Shell Command Execution (🟠 High)

- **Code**: `internal/agent/tools/bash.go`
- **Mitigations in place**:
  - Built-in allowlist of “safe” read-only commands; everything else requires an explicit permission prompt (`permissions.Request`).
  - Network/package-management binaries (curl, wget, apt, etc.) are hard-blocked, so even an auto-approved session cannot run them.
  - Background jobs are tracked via `internal/shell` and can be inspected/terminated from the UI.
- **Residual risks**:
  - Once a user approves a command, it executes with the user’s OS permissions. Consider sandboxing or allowing commands to run inside an opt-in container for high-assurance environments.
  - The allowlist is prefix-based; multi-command constructs (`echo foo && rm -rf /tmp/bar`) can still run if the prefix matches a safe command. A future enhancement could parse commands before comparing.

## 2. File System Access (🟡 Medium)

- **Code**: `internal/agent/tools/view.go`, `edit.go`, `write.go`, `multiedit.go`.
- **Mitigations**:
  - Each tool normalizes the requested path relative to the configured working directory and rejects attempts to escape via `..`.
  - Mutations always flow through `internal/history`, creating an auditable version tree.
  - Permission prompts include the normalized path so users know exactly which file is targeted.
- **Residual risks**:
  - The `view` tool can read any file inside the working directory subtree. If users open Crush at `$HOME`, secrets in dotfiles are still reachable. Documenting best practices (run Crush inside the repo root) helps reduce exposure.

## 3. MCP Clients (🟡 Medium)

- **Code**: `internal/agent/tools/mcp/*.go`
- **Mitigations**:
  - Supported transports are `stdio`, `http`, and `sse`. Each client runs in its own process, and stdout/stderr are monitored for crashes.
  - Tool output is treated as plain text; MCP servers cannot directly run shell commands unless the user grants permission to the associated MCP tool.
- **Residual risks**:
  - Stdio MCP servers inherit the user’s environment. Malicious MCP binaries could abuse that access. Consider running long-lived stdio MCP servers inside a sandboxed shell.
  - HTTP/SSE transports trust the remote hostname configured in `crush.json`. Users should only enable MCP servers they trust.

## 4. API Keys & Configuration (🟡 Medium)

- **Code**: `internal/config/merge.go`, `README` onboarding instructions.
- **Mitigations**:
  - Configuration supports environment variable references (`"$(echo $OPENAI_API_KEY)"`), so secrets never need to be committed.
  - `config.Config.Redacted()` ensures keys are omitted when printing debug output.
- **Residual risks**:
  - Keys are stored on disk (usually `~/.config/crush/crush.json`). Users should ensure filesystem permissions restrict access. Future enhancements could read secrets from OS keyrings.

## 5. LSP Server Execution (🟢 Low)

- **Code**: `internal/lsp/client.go`
- **Mitigations**:
  - LSP servers run as child processes with user-provided commands; Crush does not ship third-party binaries.
  - The client enforces timeouts when waiting for servers to become ready and terminates them cleanly on shutdown.
- **Residual risks**:
  - Misconfigured `command` entries could execute arbitrary programs. Because LSP servers typically need access to the repo, this is acceptable, but we should document that entries in `config.lsp` act like shell commands.

## Recommendations

1. **Optional Sandboxing** – Provide an opt-in mode that runs bash commands and stdio MCP servers inside a container or namespace.
2. **Command Parsing** – Extend the bash tool to parse multiple commands and evaluate the allowlist on each discrete command rather than string prefixes.
3. **Secret Hygiene Docs** – Expand the getting-started guide with concrete steps for keeping `crush.json` private and using environment variables by default.
4. **Audit Logging** – Persist an append-only log of approved permission requests (tool, path, timestamp). This would make investigations easier if something goes wrong.
