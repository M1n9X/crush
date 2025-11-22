Launch a configured subagent using the agent profiles on disk.

<usage>
- Provide a concise prompt; optionally add a short description for logging.
- Set `subagent_type` to one of the profiles defined in ~/.claude/agents, ~/.codebreeze/agents, ~/.crush/agents or their project equivalents. Defaults to general-purpose.
- Profiles control their system prompt, allowed tools, and optional model override. The subagent cannot request additional permissions.
- Use for scoped lookups, reconnaissance, or delegated workstreams without interrupting the main session.
</usage>
