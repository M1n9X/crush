Writes to a per-agent memory file stored under `data_directory/memory/agents/<agent_id>`.

- `file_path`: Relative path under the agent memory directory. Defaults to `memory.md`.
- `content`: Full content to store (overwrites the target file).
- Paths are sandboxed to the agent memory directory.
- Use this to persist commands, preferences, or notes you want available in future sessions.
