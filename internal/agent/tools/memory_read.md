Reads a per-agent memory file stored under `data_directory/memory/agents/<agent_id>`.

- `file_path`: Relative path under the agent memory directory. Defaults to `memory.md`.
- Paths are sandboxed to the agent memory directory.
- Use this to recall long-term notes, commands, or preferences across sessions.
