# Feature Porting: CodeBreeze -> Crush
> Status: In Progress  
> Updated: 2025-02-16

## Purpose
- Establish a clear feature diff between **Crush** (Go, Bubble Tea TUI, MCP/LSP/tool system) and **CodeBreeze** (TS mono-repo with Ink UI, DI runtime, capability registry).
- Identify CodeBreeze-only strengths that are worth migrating.
- Propose a migration plan that respects Crush’s existing architecture and user experience.

## Side-by-Side Feature Map

| Area | CodeBreeze (source) | Crush (source) | Gap / Opportunity |
| --- | --- | --- | --- |
| Runtime composition | DI container + registrar pattern (`packages/runtime/src/registrars/*.ts`); capability registry (`packages/runtime/src/capabilities/registry.ts`) | Lightweight registrars (`internal/runtime/registrars.go`) seed MCP/telemetry/permissions; capability registry for tools | Expand registrar set (tools/plugins) and surface status in UI; keep Go wiring minimal. |
| Tool descriptors & permissions | Declarative capability descriptors with permission workflow + per-project stores (`packages/runtime/src/capabilities`) | Capability registry + `crush permissions` CLI (list/grant/revoke/clear/export); per-workspace persistent store | Add UI surfacing (permissions overlay), and per-tool manifest metadata (schema/safety). |
| Tool surface area | 18+ built-in tools (file I/O, search, command exec, media inspection, docs) exposed via capability registry (README) | Rich core tools plus MCP + LSP tools; no unified manifest or per-tool telemetry | Normalize tool manifest + telemetry hooks so tools report usage/cost and can be dynamically toggled. |
| Context resilience | Auto-compact conversation + file recovery | Auto-compaction with LLM summary + freshness-prioritized file recovery + disk fallback | Add UI notice/log panel; tune thresholds per provider. |
| Reasoning control | Keyword-driven thinking tokens + reasoning effort override | Keyword parser mapping to provider thinking/reasoning options and max_tokens | Expose toggle in config/CLI; add TUI indicator. |
| Context ingestion | Automatic project scan + AGENTS ingestion, freshness tracking | Freshness service seeded from history + context_paths + AGENTS scan; persisted to data dir | Add deeper scan (LSP/activity) and UI surfacing. |
| Observability | Telemetry bus + cost tracking, debug overlays, Ink UI events | Telemetry recorder + tool-finish/token events; registrar logging + TUI “Telemetry Pulse” overlay | Add richer filtering/trends and per-provider costs. |
| Plugin/extension model | Versioned plugin API + runtime host | Plugin host + manifest loader (`plugins_manifest` / auto-detect) with CLI listing | Wire plugins into capability registry + permissions; add enable/disable + sandbox policy. |
| Config durability | Auto-backup/repair | Backup + backup fallback + lenient load | Add schema-based repair and warning surfacing. |
| UI ergonomics | Ink overlays, streamed panes | Bubble Tea TUI; permission dialog exists | Add overlays for telemetry/permissions & tool manifest view. |

## What’s Already Migrated
- **Capability registry + manifests**: Go registry for tools/plugins (`crush tools list`, `crush plugins list`); auto-detect tool/plugin manifests (`tools.json`, `plugins.json`, `.crush/tools.json` etc.) and register plugin descriptors as `plugin:*` capabilities.
- **Permissions durability & UX**: Workspace-scoped JSON store (path hashed) with CLI (`crush permissions list|grant|revoke|clear|export`) and TUI command palette entry **View Permissions** (read-only) to inspect persistent approvals.
- **Context resilience**: Auto-compaction with LLM summaries plus freshness-aware file recovery (seeded from history, context_paths, initialize_as, AGENTS/CRUSH/CLAUDE markers) persisted to `data_directory/freshness.json`.
- **Thinking controls**: Keyword parser maps “think harder” phrases to provider-specific options (Anthropic/OpenAI/OpenRouter/Google/OpenAI-compatible) with configurable defaults.
- **Telemetry**: Recorder emits `tool_finished` / `tokens_used` events, telemetry registrar logs, and TUI command palette entry **Telemetry Pulse** renders recent events with token/cost totals plus top tools by spend.
- **Runtime composition**: Registrars for tools, plugins, permissions, telemetry, MCP init, manifest loaders, capability registry, and plugin host are wired for App + Coordinator lifecycles.
- **Config resilience**: Lenient config loader with automatic `.backup` fallback; manifest paths are normalized; corrupt JSON is auto-repaired when possible; unknown top-level keys are pruned with a warning.
- **Plugin controls**: CLI `crush plugins list|enable|disable` with per-workspace persistence and sandbox label display, plus TUI **Manage Plugins** palette entry to toggle state (enable is permission-checked); plugin descriptors always registered for visibility.
- **Context surfacing**: Command palette entry **View Fresh Context** lists freshest files for the active session (based on recovery/freshness tracking).
- **Telemetry surfacing**: Status bar shows cumulative tokens/cost; telemetry dialog supports per-session filtering and top tools by spend.

## How to Use the New Surfaces
- **Permissions**: `crush permissions list` (CLI) or open “Commands” → “View Permissions” to inspect stored approvals. Grants/revokes persist per workspace; the TUI list is read-only to avoid accidental changes.
- **Telemetry**: Open “Commands” → “Telemetry Pulse” to see the latest tool events, tokens in/out, and cumulative cost (newest first, last 20).
- **Plugins/Tools manifests**: Provide `tools_manifest` / `plugins_manifest` in `crush.json` or drop `tools.json` / `plugins.json` / `.crush/tools.json` in the project; `crush tools list` / `crush plugins list` shows what was registered.

## Remaining Gaps to Close
- **Plugins UX**: Permission prompts per plugin capability, sandbox policy enforcement, and descriptor-driven capability permissions (TUI toggles ship; execution sandboxing still TODO).
- **Config repair**: Surface validation warnings and schema-based default insertion beyond top-level pruning.
- **Context surfacing**: Inline UI hints when recovery replays files; optional deeper scan hooks (LSP activity, recent edits).
- **Telemetry depth**: Filters per tool, trendlines, per-provider cost summaries; richer status-bar hooks.

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
1) Registry + permission store scaffolding ✅  
2) Context compaction + recovery ✅  
3) Reasoning controls + provider mapping ✅  
4) Telemetry bus + TUI surface ✅ (pulse view shipped; needs trends/filters)  
5) Plugin/descriptor ingestion + CLI permissions ✅ (enable/disable UX pending)  
6) Config durability polish ⏳

## Risks / Mitigations
- **Token estimation mismatch**: validate compaction thresholds per provider; keep a manual escape hatch to disable auto-compact.  
- **Permission fatigue**: batch prompts and allow ephemeral grants per session.  
- **UI divergence**: prototype overlays in a feature flag to avoid regressing current Bubble Tea UX.  
- **Plugin security**: keep default sandbox to MCP-only until in-process plugins are threat-modeled.
