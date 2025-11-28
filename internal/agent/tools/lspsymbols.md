Comprehensive guide to the LSP symbol tools. Use these when you need structure-aware navigation, editing, and retrieval instead of plain-text grep.

## Quick chooser
- `lspoverview`: cheapest, top-level symbols of a single file (no bodies).
- `lspfind`: primary symbol finder (regex or substring). Keep `include_body=false`, `depth=0`, small `max_results` for low tokens.
- `lsprefs`: references/usages of a known symbol. Start with `max_results` 20–50, `include_imports=false`.
- `lspreplace`: replace a symbol body by name_path.
- `lspinsertbefore` / `lspinsertafter`: insert content relative to a symbol.
- `lsprename`: rename symbol and references via LSP.

## Token guidance
- Overview/find/refs default `max_answer_chars=150000` (~37k tokens). Typical responses are smaller if you keep bodies off.
- Avoid `include_body` and large `depth` unless necessary; bodies drive token size.
- Set `max_results` conservatively. Raise only if you miss results.
- `include_imports=false` keeps reference lists compact.

## Common parameters
- `path`: file (or directory for find) relative to working directory.
- `name_path_pattern` / `name_path`: hierarchical symbol path (e.g., `pkg/Class/method`). `substring=true` disables regex for simple contains match on last segment.
- `include_kinds` / `exclude_kinds`: integers 1–26 per LSP SymbolKind (Function=12, Class=5, Method=6, Interface=11, Struct=23).
- `max_results`: cap result count (0 = no cap).
- `max_answer_chars`: cap response length (defaults provided).

## lspoverview
- Purpose: top-level structure of one file. No bodies; minimal tokens.
- Use before drilling in with `lspfind` or `view`.

## lspfind
- Purpose: locate definitions with structure awareness.
- Matching modes: regex (default) or `substring=true` for lightweight contains match. `is_regex` can be forced.
- Tips: scope `path` to a dir/file; keep `max_results` small; use `include_kinds` to narrow.

## lsprefs
- Purpose: where/how a symbol is used.
- Tips: start with `include_imports=false`; set `max_results`; filter with kinds if noisy.

## Editing tools (LSP-assisted)
- `lspreplace`: replace body while preserving signature.
- `lspinsertbefore` / `lspinsertafter`: insert relative to a symbol anchor.
- `lsprename`: rename symbol and references when supported by the LSP.
- Safety: require write permission; rely on LSP ranges; consider running `lspoverview`/`lspfind` first to confirm targets.

## When to use grep instead
- Searching comments/strings/docs.
- Symbol not indexed by LSP or language server unavailable.
- Non-code files or free-form text patterns.
