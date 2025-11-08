# App Controller Module

The App Controller (`internal/app/app.go`) wires together every service, manages lifecycle/shutdown, and bridges the CLI/TUI to the agent coordinator.

## Structure

```go
type App struct {
    Sessions    session.Service
    Messages    message.Service
    History     history.Service
    Permissions permission.Service

    AgentCoordinator agent.Coordinator
    LSPClients       *csync.Map[string, *lsp.Client]

    config *config.Config

    serviceEventsWG *sync.WaitGroup
    eventsCtx       context.Context
    events          chan tea.Msg
    tuiWG           *sync.WaitGroup

    globalCtx    context.Context
    cleanupFuncs []func() error
}
```

Key points:

- Services are thin wrappers over sqlc queries and publish `pubsub.Event[T]` values whenever data changes.
- `AgentCoordinator` is instantiated lazily once valid provider credentials exist.
- `LSPClients` is a concurrent map because LSP processes start/stop asynchronously.

## Initialization

```go
func New(ctx context.Context, conn *sql.DB, cfg *config.Config) (*App, error) {
    q := db.New(conn)
    sessions := session.NewService(q)
    messages := message.NewService(q)
    files := history.NewService(q, conn)

    perm := permission.NewPermissionService(
        cfg.WorkingDir(),
        cfg.Permissions != nil && cfg.Permissions.SkipRequests,
        cfg.Permissions.AllowedTools,
    )

    app := &App{
        Sessions:    sessions,
        Messages:    messages,
        History:     files,
        Permissions: perm,
        LSPClients:  csync.NewMap[string, *lsp.Client](),
        config:      cfg,
        globalCtx:   ctx,
        events:      make(chan tea.Msg, 100),
        serviceEventsWG: &sync.WaitGroup{},
        tuiWG:           &sync.WaitGroup{},
    }

    app.setupEvents()
    app.initLSPClients(ctx)
    go mcp.Initialize(ctx, app.Permissions, cfg)
    app.cleanupFuncs = append(app.cleanupFuncs, conn.Close, mcp.Close)

    if cfg.IsConfigured() {
        if err := app.InitCoderAgent(ctx); err != nil {
            return nil, err
        }
    }
    return app, nil
}
```

## Event Flow

- `setupEvents` subscribes to every service (`Sessions`, `Messages`, `History`, `Permissions`, MCP events, LSP events).
- Each subscriber writes to the buffered `events` channel; the TUI goroutine reads from this channel and forwards messages into Bubble Tea via `program.Send`.
- Cleanup functions cancel the event context and wait for all subscriber goroutines to exit before shutting down the rest of the app.

## Agent Lifecycle

```go
func (app *App) InitCoderAgent(ctx context.Context) error {
    coord, err := agent.NewCoordinator(
        ctx,
        app.config,
        app.Sessions,
        app.Messages,
        app.Permissions,
        app.History,
        app.LSPClients,
    )
    if err != nil {
        return err
    }
    app.AgentCoordinator = coord
    return nil
}
```

- Interactive runs call `app.AgentCoordinator.Run` from the TUI when the user submits a prompt.
- Non-interactive runs (`crush run ...`) reuse the same coordinator but wire output to stdout instead of the TUI.
- `Shutdown` cancels pending agent requests via `AgentCoordinator.CancelAll()`.

## LSP Management

`initLSPClients` loops over `config.LSP` entries and launches each client in its own goroutine. Successful clients are stored in `LSPClients`; failures are logged and surfaced to the UI through `SubscribeLSPEvents`.

`App.Shutdown` iterates over the map and calls `client.Shutdown(ctx)` with a short timeout to avoid hanging on exit.

## Permissions & MCP Integration

- MCP state changes are funneled into the same event bus via `mcp.SubscribeEvents`, so the TUI can show live status.
- Permission requests/notifications are also part of the bus; the UI opens dialogs when it receives `pubsub.Event[permission.PermissionRequest]` and updates inline indicators when `PermissionNotification` arrives.

## Cleanup

```go
func (app *App) Shutdown() {
    if app.AgentCoordinator != nil {
        app.AgentCoordinator.CancelAll()
    }
    close(app.events)
    for _, fn := range app.cleanupFuncs {
        if fn != nil {
            fn()
        }
    }
}
```

Each subsystem registers its own cleanup function (database close, MCP shutdown, TUI goroutine cancellation). Because everything is tied to `globalCtx`, canceling the context automatically tears down subscribers and background routines.

## Related Reading

- [Architecture Overview](../02_Architecture_Overview.md)
- [AI Agent System](AI_Agent_System.md)
- [Permission System](Permission_System.md)
