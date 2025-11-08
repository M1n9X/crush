# Configuring MCP Integration

Model Context Protocol (MCP) servers let you bolt on extra tools without recompiling Crush. Configuration lives under the `mcp` top-level key in `crush.json`.

## 1. Basic Structure

```json
{
  "$schema": "https://charm.land/crush.json",
  "mcp": {
    "filesystem": {
      "type": "stdio",
      "command": "npx",
      "args": ["@modelcontextprotocol/server-fs"],
      "timeout": 60,
      "env": {
        "FS_ROOT": "$(pwd)"
      }
    }
  }
}
```

`config.MCPConfig` fields:

| Field | Description |
| --- | --- |
| `type` | One of `stdio`, `http`, `sse`. Required. |
| `command` / `args` | Used for `stdio` transports. Executes the binary directly (no shell). |
| `env` | Environment variables for stdio servers. Values may use `$(echo $VAR)` expansion. |
| `url` | Base URL for HTTP/SSE servers. |
| `headers` | Extra HTTP headers (e.g., `Authorization`). |
| `timeout` | Connection/tool-call timeout in seconds (default 15). |
| `disabled` | Skip this server without deleting the block. |

## 2. Transport Examples

### stdio (local process)

```json
{
  "mcp": {
    "copier": {
      "type": "stdio",
      "command": "python3",
      "args": ["scripts/copier.py"],
      "env": {
        "API_KEY": "$(echo $COPIER_KEY)"
      }
    }
  }
}
```

### HTTP

```json
{
  "mcp": {
    "knowledge-base": {
      "type": "http",
      "url": "https://kb.example.com/mcp",
      "headers": {
        "Authorization": "Bearer $(echo $KB_TOKEN)"
      },
      "timeout": 120
    }
  }
}
```

### Server-Sent Events (SSE)

```json
{
  "mcp": {
    "streaming": {
      "type": "sse",
      "url": "https://example.com/mcp/stream",
      "headers": {
        "API-Key": "$(echo $STREAM_KEY)"
      }
    }
  }
}
```

## 3. Allowing Tools

The agent only exposes MCP tools that the active agent config allows. Add entries to `config.agents.coder.allowed_mcp` to opt in:

```json
{
  "agents": {
    "coder": {
      "allowed_tools": ["bash", "write", "filesystem.read"],
      "allowed_mcp": ["filesystem", "knowledge-base"]
    }
  }
}
```

## 4. Monitoring State

- Run `crush --debug` to see MCP startup logs.
- Inside the TUI, press `Ctrl+O` (commands) → `MCP` to check each server’s status and prompts.
- The status line shows `mcp:<name>=connected|error|starting|disabled`.

## 5. Troubleshooting

| Symptom | Fix |
| --- | --- |
| “mcp '<name>' not available” | Ensure the config key matches exactly and the server is not disabled. |
| Continuous restarting | Check stdout/stderr of the MCP binary; Crush logs them when `--debug` is enabled. |
| Tools missing | Confirm the MCP server advertises tools via `list_tools`. Some servers require an initial handshake or credentials. |
| Permission prompts for MCP tools | Expected—MCP tools are wrapped like native tools. Add entries to `permissions.allowed_tools` if desired. |

With MCP servers configured, their tools appear alongside built-in ones whenever the agent shares the tool schema with the LLM.
