# LSP Integration

Crush can talk to language servers so the agent (and tools) can access diagnostics, references, and other semantic data. The integration lives entirely in `internal/lsp` and uses [charmbracelet/x/powernap](https://pkg.go.dev/github.com/charmbracelet/x/powernap) under the hood.

## Components

| File | Responsibility |
| --- | --- |
| `internal/lsp/client.go` | Spins up a single LSP server, forwards JSON-RPC traffic through powernap, tracks diagnostics, and manages open documents. |
| `internal/lsp/handlers.go` | Notification and request handlers (`workspace/applyEdit`, `textDocument/publishDiagnostics`, etc.). |
| `internal/lsp/language.go` | Maps file extensions to configured LSP clients and exposes helper lookups for the tools layer. |
| `internal/app/app.go` | Calls `initLSPClients`, keeps a map of clients, and exposes it to the agent tool constructors. |

## Client Lifecycle

```go
func New(ctx context.Context, name string, cfg config.LSPConfig, resolver config.VariableResolver) (*Client, error) {
    rootURI := protocol.URIFromPath(workDir)
    command, err := resolver.ResolveValue(cfg.Command)
    clientConfig := powernap.ClientConfig{
        Command: home.Long(command),
        Args:    cfg.Args,
        RootURI: string(rootURI),
        Environment: cfg.Env,
        Settings:    cfg.Options,
        InitOptions: cfg.InitOptions,
    }
    pnClient, err := powernap.NewClient(clientConfig)
    return &Client{
        client:      pnClient,
        name:        name,
        fileTypes:   cfg.FileTypes,
        diagnostics: csync.NewVersionedMap[protocol.DocumentURI, []protocol.Diagnostic](),
        openFiles:   csync.NewMap[string, *OpenFileInfo](),
        config:      cfg,
    }, nil
}
```

- `Initialize` sends the JSON-RPC `initialize` request and registers notification/command handlers.
- `WaitForServerReady` attempts a lightweight request with a 30 s timeout to ensure the server is responsive before surfacing it to tools/the UI.
- `Close` gracefully shuts down the server, closes open files, and terminates the child process.

## Configuration (`crush.json`)

```json
{
  "lsp": {
    "go": {
      "command": "gopls",
      "args": ["-remote=auto"],
      "filetypes": ["go", "mod"],
      "root_markers": ["go.mod"],
      "env": {
        "GOTOOLCHAIN": "go1.24.5"
      },
      "init_options": {
        "hints": {"assignVariableTypes": true}
      },
      "options": {
        "gopls": {"ui.diagnostic.underline": true}
      }
    }
  }
}
```

Fields mirror `config.LSPConfig`:

- `command` / `args` – binary and CLI arguments.
- `env` – environment variables handed to the subprocess (values can include `$(echo $VAR)` expressions).
- `filetypes` – extensions the client should claim; used by the diagnostics/reference tools to select the right client.
- `root_markers` – files that mark the project root (passed to powernap).
- `init_options` / `options` – forwarded to the LSP `initialize` request.
- `disabled` – skip launching the client.

## Integration with Tools & UI

- The diagnostics and references tools (`internal/agent/tools/diagnostics.go`, `references.go`) look up the correct client from `App.LSPClients` and call helper functions to read cached diagnostics or issue `textDocument/references` requests.
- The TUI status bar subscribes to `SubscribeLSPEvents` (see `internal/app/app.go`) to show whether each configured client is ready or erroring.
- `view`/`edit` tools pass the active LSP diagnostics into their responses so the agent sees lint errors alongside file content.

## Troubleshooting Tips

- Use `config.Options.DebugLSP` to enable verbose logging from the powernap client.
- Ensure the `command` is the absolute path or resolvable via `$PATH`; the client does not spawn through a shell.
- If a server fails to start, check the application log (`crush --debug`) for stderr captured from the LSP process.
