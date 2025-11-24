# Feature Porting: CodeBreeze -> Crush
> Status: Draft  
> Updated: 2025-02-15

## Purpose
- Establish a clear feature diff between **Crush** (Go, Bubble Tea TUI, MCP/LSP/tool system) and **CodeBreeze** (TS mono-repo with Ink UI, DI runtime, capability registry).
- Identify CodeBreeze-only strengths that are worth migrating.
- Propose a migration plan that respects Crush’s existing architecture and user experience.

## Side-by-Side Feature Map

| Area | CodeBreeze (source) | Crush (source) | Gap / Opportunity |
| --- | --- | --- | --- |
| Runtime composition | DI container + registrar pattern (`packages/runtime/src/registrars/*.ts`); capability registry (`packages/runtime/src/capabilities/registry.ts`) | Direct wiring in Go structs; no DI container; tool registration lives in `internal/agent/tools` | Introduce a lightweight registry layer in Go to decouple tool/provider wiring and enable swappable implementations. |
| Tool descriptors & permissions | Declarative capability descriptors with permission workflow + per-project stores (`packages/runtime/src/capabilities`); CLI to list/grant/revoke permissions (docs/architecture.md) | Simple allowlist + prompt (`crush.json` `permissions.allowed_tools`, `--yolo`) without capability metadata | Add a capability descriptor model (id, schema, safety posture) and permission store; CLI/TUI affordances for per-session overrides. |
| Tool surface area | 18+ built-in tools (file I/O, search, command exec, media inspection, docs) exposed via capability registry (README) | Rich core tools plus MCP + LSP tools; no unified manifest or per-tool telemetry | Normalize tool manifest + telemetry hooks so tools report usage/cost and can be dynamically toggled. |
| Context resilience | Auto-compact conversation + file recovery (`src/utils/autoCompactCore.ts`, `src/utils/fileRecoveryCore.ts`), message context manager | Long sessions stored in SQLite; no automatic compaction/recovery; manual summaries only | Port auto-compaction with token thresholds, LLM summaries, and recent-file replay. |
| Reasoning control | Keyword-driven thinking tokens + reasoning effort override (`src/utils/thinking.ts`) | Static model params; no user-level reasoning keyword controls | Add reasoning-effort parser that maps keywords to token/effort overrides per provider. |
| Context ingestion | Automatic project scan + AGENTS.md ingestion, freshness tracking (`docs/architecture.md`, `fileFreshness` services) | Initialization writes AGENTS/CRUSH file; ignores freshness ranking beyond LSP diagnostics | Reuse freshness tracking to prioritize files for context and recovery. |
| Observability | Telemetry bus + cost tracking, debug overlays, Ink UI events (`packages/core/src/orchestration/telemetry.ts`, docs/architecture.md) | Cost persisted per session (`internal/db`); limited UI surfacing; no event bus for tools/providers | Emit structured telemetry events (turn lifecycle, tool exec) and surface in TUI (status bar/log drawer). |
| Plugin/extension model | Versioned plugin API (`docs/plugins/migration.md`), runtime plugin host | MCP integration only; no first-class plugin host for Go tools | Provide plugin hooks or map plugin API to MCP-like loaders with capability descriptors. |
| Config durability | Global config auto-backup/repair (`~/.codebreeze/config.json` + `.backup`) | JSON schema validation only; no automated backup/repair on save | Add config backup + repair routine to avoid breakage after edits. |
| UI ergonomics | Ink-based CLI with overlays, streamed command panes, Vim-like navigation | Bubble Tea TUI with chat/sidebar; fewer overlays; commands panel available | Selectively port overlay patterns (permission prompts, telemetry panels) without breaking Bubble Tea UX. |

## CodeBreeze Strengths Worth Migrating
- **Auto-compaction + recovery**: `src/utils/autoCompactCore.ts` compresses conversation when usage >92% of context, generates a structured summary, and reattaches recent files chosen by `src/utils/fileRecoveryCore.ts`. Failures degrade gracefully.
- **Thinking controls**: `src/utils/thinking.ts` maps user keywords (“think harder”, “ultrathink”) to max_tokens and reasoning_effort so the user can request deeper reasoning ad-hoc.
- **Capability registry + permissions**: `packages/runtime/src/capabilities/*` keeps declarative descriptors, enforces permission prompts, and persists decisions per project. Permission CLI commands ride on runtime façades.
- **Telemetry pipeline**: turn lifecycle emits events and cost metrics (`packages/core/src/orchestration/telemetry.ts`); runtime registrars attach telemetry sinks and capability hooks.
- **DI + registrar composition**: runtime bootstraps via registrars (`registerInference`, `registerTooling`, `registerPermissions`, `registerTelemetry`, `registerPlugins`), making it easy to swap adapters in tests or headless runtimes.
- **Config resilience**: config writes produce `.backup` siblings and include validation/auto-repair; malformed configs do not block startup.

## Crush Baseline (to preserve)
- Bubble Tea TUI with chat, session sidebar, and compact layout (`internal/tui/page/chat`).
- Multi-provider + LSP + MCP support configured via `crush.json` and `internal/config`.
- Tooling framework with file ops, shell exec, MCP, and semantic retrieval/subagents (`internal/agent/tools`, `wiki/features/semantic-retrieval.md`, `wiki/features/Subagent.md`).
- SQLite-backed session history/cost tracking (`internal/db`), testable Go codebase with Taskfile automation.

## Migration Plan

### 1) Runtime/Tooling Backbone
- Introduce a Go capability descriptor struct (id, description, input schema pointer, permissions posture, isReadOnly) and registry living near `internal/agent/tools`.
- Add optional telemetry/perms hooks to tool execution coordinator; align with existing permission prompts instead of replacing them outright.
- Prepare a registrar-style bootstrap (mirroring CodeBreeze) to wire providers/tools/LSP/MCP; keep defaults to avoid breaking CLI entrypoints.

### 2) Context Resilience
- Add token accounting for conversation messages (reuse existing token counts from providers where possible).
- Implement auto-compaction: threshold check, LLM summary prompt, message replacement, and state cleanup. Persist a summary marker in DB (`is_summary_message`) to keep history consistent.
- Port file recovery: track “fresh” files (via tool calls/LSP touches) and replay a bounded set with truncation when compacting.

### 3) Reasoning Controls
- Add a small parser that inspects the latest user message for “think” keywords and sets `max_tokens` / provider-specific reasoning effort before dispatch.
- Expose a config toggle to disable/limit this behavior for cautious users.

### 4) Permissions & Capability UX
- Persist tool approval decisions per project (JSON under `~/.crush/permissions/<project>.json` akin to CodeBreeze).
- Extend TUI with a permissions overlay listing pending/approved tools; add CLI subcommands for `permissions list/grant/revoke`.
- Store capability descriptors alongside tools so approvals can be described clearly.

### 5) Telemetry & Observability
- Emit turn/tool events on an internal bus (step transitions, tool start/finish, cost deltas).
- Render lightweight telemetry in TUI (status bar with token/cost, optional debug panel).
- Keep DB cost fields but also stream events for real-time visibility.

### 6) Plugin/Extension Surface
- Short term: treat CodeBreeze plugin descriptors as MCP-compatible manifests where feasible (id, apiVersion, capabilities).
- Mid term: add a Go plugin host that can register capability descriptors into the registry, gated by permissions and sandbox rules.

### 7) Config Durability
- On write, emit `.backup` next to `crush.json` and validate; if parsing fails on boot, auto-restore the latest backup and notify.
- Add minimal “repair” (e.g., pruning unknown fields) backed by `schema.json`.

## Suggested Work Sequencing
1) Registry + permission store scaffolding (unblocks later steps).  
2) Context compaction + recovery (user-facing win, contained blast radius).  
3) Reasoning controls + provider mapping.  
4) Telemetry bus + TUI surface.  
5) Plugin/descriptor ingestion + CLI permissions.  
6) Config durability polish.

## Risks / Mitigations
- **Token estimation mismatch**: validate compaction thresholds per provider; keep a manual escape hatch to disable auto-compact.  
- **Permission fatigue**: batch prompts and allow ephemeral grants per session.  
- **UI divergence**: prototype overlays in a feature flag to avoid regressing current Bubble Tea UX.  
- **Plugin security**: keep default sandbox to MCP-only until in-process plugins are threat-modeled.
