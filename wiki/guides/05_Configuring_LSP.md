# Configuring LSP Integration

Language Server Protocol (LSP) support lets Crush surface diagnostics, completions, and references from the tools layer. LSP servers are defined in the main configuration file (`crush.json`, `.crush.json`, or `~/.config/crush/crush.json`).

## 1. Minimal Configuration

```json
{
  "$schema": "https://charm.land/crush.json",
  "lsp": {
    "go": {
      "command": "gopls",
      "filetypes": ["go", "mod"],
      "root_markers": ["go.mod"],
      "env": {
        "GOTOOLCHAIN": "go1.24.5"
      }
    }
  }
}
```

Fields map directly to `internal/config.LSPConfig`:

| Field | Description |
| --- | --- |
| `command` | Binary to execute (required). The string is resolved via the config resolver, so `"$(echo $GOPLS)"` works. |
| `args` | Optional arguments array (`["--stdio"]`). |
| `env` | Extra environment variables passed to the process. |
| `filetypes` | Extensions handled by this server; diagnostics/references tools use this to select a client. |
| `root_markers` | Files or directories that mark the workspace root (e.g., `package.json`). |
| `init_options` | JSON object forwarded to the LSP `initialize` request. |
| `options` | Server-specific settings (powernap “settings” field). |
| `disabled` | Set to `true` to leave the server out of the launch sequence. |
| `timeout` | Optional integer (seconds) for readiness checks. |

## 2. Examples

### TypeScript / JavaScript

```json
{
  "lsp": {
    "ts": {
      "command": "typescript-language-server",
      "args": ["--stdio"],
      "filetypes": ["ts", "tsx", "js", "jsx"],
      "root_markers": ["package.json"],
      "options": {
        "typescript": {
          "tsserver": {"logVerbosity": "verbose"}
        }
      }
    }
  }
}
```

### Python

```json
{
  "lsp": {
    "python": {
      "command": "pylsp",
      "filetypes": ["py"],
      "env": {
        "VIRTUAL_ENV": "$(echo $VIRTUAL_ENV)"
      }
    }
  }
}
```

### Rust

```json
{
  "lsp": {
    "rust": {
      "command": "rust-analyzer",
      "filetypes": ["rs", "toml"],
      "root_markers": ["Cargo.toml"]
    }
  }
}
```

## 3. Advanced Options

- **Initialization options**: Add nested JSON under `init_options`. Example for gopls:
  ```json
  "init_options": {
    "hints": {
      "assignVariableTypes": true,
      "parameterNames": true
    },
    "analyses": {
      "unusedparams": true
    }
  }
  ```
- **Settings**: Use `options` when the server expects `workspace/configuration` style settings.
- **Debugging**: Set `config.options.debug_lsp` to `true` to log LSP traffic (see `config.Options`).

## 4. Enabling Diagnostics & Reference Tools

The diagnostics and references tools only appear when at least one LSP server is configured. Ensure the server’s `filetypes` cover the extensions in your project; otherwise the tools will skip files.

## 5. Troubleshooting

- Run `crush --debug` to see LSP startup logs and stderr output.
- Use absolute paths or ensure the binary is on `$PATH`.
- If a server never becomes ready, verify `timeout` is large enough; some servers need 30+ seconds on first run.
- Delete `.crush/crush.db` if you remove servers and want to drop cached diagnostics (not usually necessary, but helpful when experimenting).

Once configured, the TUI status bar shows whether each LSP server is ready, and tools like `diagnostics`/`references` become available automatically.
