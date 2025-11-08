# Code Highlights

A few implementation details in Crush are worth pointing out because they illustrate the design philosophy of the project.

## 1. Type-Safe Pub/Sub (`internal/pubsub/broker.go`)

```go
type Broker[T any] struct {
    subs map[chan Event[T]]struct{}
    mu   sync.RWMutex
    done chan struct{}
}

func (b *Broker[T]) Subscribe(ctx context.Context) <-chan Event[T] {
    sub := make(chan Event[T], 64)
    b.mu.Lock()
    b.subs[sub] = struct{}{}
    b.mu.Unlock()

    go func() {
        <-ctx.Done()
        b.mu.Lock()
        delete(b.subs, sub)
        close(sub)
        b.mu.Unlock()
    }()
    return sub
}
```

- Works with any payload type (`Event[message.Message]`, `Event[permission.PermissionRequest]`, etc.).
- Channels are buffered and never block the publisher; slow subscribers simply drop messages.
- Context cancellation automatically drains subscribers, which keeps long-running services leak-free.

## 2. Concurrent Collections (`internal/csync`)

`csync.Map` and `csync.VersionedMap` wrap common patterns such as `map + RWMutex` and expose helper methods like `Seq2` so you can iterate safely:

```go
func (m *Map[K, V]) Seq2() iter.Seq2[K, V] {
    return func(yield func(K, V) bool) {
        m.mu.RLock(); defer m.mu.RUnlock()
        for k, v := range m.m {
            if !yield(k, v) {
                return
            }
        }
    }
}
```

The agent, permission service, MCP manager, and shell job manager all rely on these helpers to avoid subtle races.

## 3. Streaming Agent Pipeline (`internal/agent/agent.go`)

```go
result, err := agent.Stream(genCtx, fantasy.AgentStreamCall{
    Prompt:          call.Prompt,
    Files:           files,
    Messages:        history,
    ProviderOptions: call.ProviderOptions,
    PrepareStep: func(ctx context.Context, opts fantasy.PrepareStepFunctionOptions) (context.Context, fantasy.PrepareStepResult, error) {
        // Flush queued prompts, create assistant placeholder, add cache hints
    },
    OnTextDelta: func(id, text string) error {
        currentAssistant.AppendContent(text)
        return a.messages.Update(genCtx, *currentAssistant)
    },
    OnToolInputStart: func(id, name string) error {
        currentAssistant.AddToolCall(message.ToolCall{ID: id, Name: name})
        return a.messages.Update(genCtx, *currentAssistant)
    },
    OnToolResult: func(res fantasy.ToolResultContent) error {
        // Persist as message.Tool result row
    },
})
```

The `sessionAgent` wires every fantasy hook directly into the persistence layer, so the UI gets fine-grained updates without polling, and transcripts remain faithful even if the process crashes mid-stream.

## 4. Permission Service (`internal/permission/permission.go`)

```go
func (s *permissionService) Request(opts CreatePermissionRequest) bool {
    if s.skip || allowListed(opts, s.allowedTools) {
        return true
    }
    if s.sessionHasGrant(opts) {
        return true
    }
    req := PermissionRequest{ID: uuid.NewString(), ...}
    respCh := make(chan bool, 1)
    s.pendingRequests.Set(req.ID, respCh)
    s.Publish(pubsub.CreatedEvent, req) // UI opens dialog
    return <-respCh
}
```

- Keeps a per-session cache of grants so “Allow for session” works as expected.
- Separates notification events (`SubscribeNotifications`) from full request events so the TUI can update inline indicators without duplicating dialogs.
- Automatically normalizes paths relative to the working directory, reducing the chance of path-traversal surprises.

## 5. File Versioning (`internal/history/file.go`)

```go
func (s *service) createWithVersion(ctx context.Context, sessionID, path, content string, version int64) (File, error) {
    for attempt := 0; attempt < 3; attempt++ {
        tx, _ := s.db.BeginTx(ctx, nil)
        dbFile, err := s.q.WithTx(tx).CreateFile(ctx, db.CreateFileParams{...})
        if sqliteErrIsUnique(err) {
            version++
            tx.Rollback()
            continue
        }
        tx.Commit()
        return s.fromDBItem(dbFile), nil
    }
    return File{}, err
}
```

Every edit/write tool funnels through this service, guaranteeing that users (and the agent) can browse prior versions per session directly inside the UI.

## 6. LSP Client Wrapper (`internal/lsp/client.go`)

```go
clientConfig := powernap.ClientConfig{
    Command: home.Long(command),
    Args:    config.Args,
    RootURI: uriFromCWD(),
    Settings: config.Options,
}
powernapClient, err := powernap.NewClient(clientConfig)
...
c.RegisterNotificationHandler("textDocument/publishDiagnostics", func(_ context.Context, _ string, params json.RawMessage) {
    HandleDiagnostics(c, params)
})
```

- Uses the `charmbracelet/x/powernap` library to manage stdio LSP servers reliably.
- Tracks open files and diagnostics in `csync.VersionedMap` so only deltas reach the TUI.
- Exposes helper functions for tool implementations (`diagnostics`, `references`) without requiring the agent to understand raw LSP protocol messages.

These patterns show up throughout the codebase and are good reference points when adding new capabilities.
