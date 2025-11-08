# Database Layer

## Overview

Crush persists everything in a single SQLite database located in the working directory (default `.crush/crush.db`). SQL migrations live in `internal/db/migrations`, and sqlc turns handwritten SQL in `internal/db/sql` into strongly typed Go code under `internal/db`.

## Components

| File | Purpose |
| --- | --- |
| `internal/db/connect.go` | Opens the SQLite file via `github.com/ncruces/go-sqlite3`, applies performance pragmas (WAL, cache size, synchronous=NORMAL), runs Goose migrations, and returns `*sql.DB`. |
| `internal/db/migrations/*.sql` | Schema evolution (sessions/messages/files tables, summary/message flags, provider columns, timestamp indexes). |
| `internal/db/sql/*.sql` | Query definitions consumed by sqlc (`CreateSession`, `ListMessagesBySession`, `CreateFile`, etc.). |
| `internal/db/*.go` | Generated models and query methods. |
| `internal/session`, `internal/message`, `internal/history` | Service wrappers that embed `db.Querier`, expose application-friendly methods, and publish pubsub events. |

## Schema Snapshot

```sql
CREATE TABLE sessions (
    id TEXT PRIMARY KEY,
    parent_session_id TEXT,
    title TEXT NOT NULL,
    message_count INTEGER NOT NULL DEFAULT 0,
    prompt_tokens INTEGER NOT NULL DEFAULT 0,
    completion_tokens INTEGER NOT NULL DEFAULT 0,
    cost REAL NOT NULL DEFAULT 0,
    summary_message_id TEXT,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

CREATE TABLE messages (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    role TEXT NOT NULL,
    parts TEXT NOT NULL DEFAULT '[]',
    model TEXT,
    provider TEXT,
    finished_at INTEGER,
    is_summary_message INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

CREATE TABLE files (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    path TEXT NOT NULL,
    content TEXT NOT NULL,
    version INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    UNIQUE(path, session_id, version)
);
```

Triggers keep `message_count` and `updated_at` in sync automatically, so higher-level services rarely need manual bookkeeping.

## sqlc Configuration

`sqlc.yaml` (excerpt):

```yaml
version: "2"
sql:
  - engine: sqlite
    schema: internal/db/migrations
    queries: internal/db/sql
    gen:
      go:
        package: db
        out: internal/db
        emit_json_tags: true
        emit_prepared_queries: true
        emit_interface: true
```

The generated `db.Querier` interface is implemented by both the raw DB and any transaction (`q.WithTx(tx)`). Services hold on to `db.New(conn)` and wrap operations with higher-level logic (permission checks, history snapshots, etc.).

## Connection Management

`db.Connect` applies:

- `PRAGMA foreign_keys = ON` – ensures cascading deletes work.
- `PRAGMA journal_mode = WAL` – better concurrency for the TUI + background tasks.
- `PRAGMA cache_size = -8000` – ~8 MB cache.
- `PRAGMA synchronous = NORMAL` – balance safety/perf for local databases.

Because SQLite doesn’t benefit from multiple concurrent writers, `sql.DB` is left at its defaults (`SetMaxOpenConns(0)`), and higher layers serialize writes through services.

## Usage Patterns

### Sessions Service (`internal/session/service.go`)

- Wraps `Queries` and publishes `pubsub.Event[Session]` for create/update/delete.
- Provides convenience methods such as `Create(ctx, title)` and `Save(ctx, session)` which the agent calls after updating token usage.

### Messages Service (`internal/message/message.go`)

- Converts between typed `message.ContentPart` structs and the JSON stored in `messages.parts`.
- Exposes helper methods to append reasoning/tool call metadata while streaming.

### History Service (`internal/history/file.go`)

- Stores file snapshots per session and handles optimistic version increments.
- Provides `CreateVersion`, `ListLatestSessionFiles`, and `DeleteSessionFiles` helpers used by view/edit/write tools.

## Future Work

- Remove unused queries (e.g., `ListNewFiles` currently references an `is_new` column that hasn’t been migrated yet).
- Consider migrating token usage totals to dedicated tables if per-provider analytics become necessary.

For a higher-level walkthrough of entities and relationships, see [Data Model & Persistence Layer](../architecture/03_Data_Model.md).
