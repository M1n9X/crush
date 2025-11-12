# Claude Code Tool

The `claude_code` tool provides headless access to Claude Code, Anthropic's AI coding assistant that can execute commands, edit files, and perform complex development tasks autonomously.

## Capabilities

Claude Code can:
- Write, edit, and analyze code in multiple programming languages
- Execute shell commands and manage processes
- Navigate and manipulate filesystems
- Perform git operations and version control tasks
- Run tests and debug applications
- Search and analyze codebases
- Set up development environments
- Create and modify configuration files
- And much more...

## Usage Notes

- **Model Selection**: Choose between `opus` (most capable), `sonnet` (balanced), or `haiku` (fastest). Defaults to `sonnet`.
- **Working Directory**: Operations are relative to the current working directory unless specified.
- **Session Management**: Sessions can be resumed by providing a `session_id`, or forked for variations.
- **Tool Restrictions**: Use `allowed_tools` and `disallowed_tools` to control which Claude Code tools can be used.
- **Cost Tracking**: Each session reports cost in USD for API usage.
- **Permission System**: Respects the project's permission system for security.

## Examples

### Basic Code Generation
```json
{
  "query": "Create a Go HTTP server with basic routing and middleware",
  "model": "sonnet"
}
```

### Complex Development Task
```json
{
  "query": "Set up a complete React TypeScript project with testing, linting, and build configuration",
  "model": "opus",
  "max_turns": 15,
  "custom_instructions": "Use modern best practices and include comprehensive error handling"
}
```

### Resume Previous Session
```json
{
  "query": "Add unit tests to the authentication module we created earlier",
  "session_id": "550e8400-e29b-41d4-a716-446655440000"
}
```

### Restricted Tool Usage
```json
{
  "query": "Analyze the codebase structure and create a documentation outline",
  "allowed_tools": ["bash", "glob", "view"],
  "disallowed_tools": ["edit", "write"]
}
```

## Response Format

The tool returns a JSON object containing:
- `result`: The main output from Claude Code
- `session_id`: Unique identifier for the session (useful for resuming)
- `cost_usd`: Total cost of the session in USD
- `duration_ms`: Duration in milliseconds
- `num_turns`: Number of conversation turns
- `is_error`: Whether the session completed with errors
- `error`: Error message if `is_error` is true
- `model_used`: The actual model that was used

## Security Considerations

- Claude Code operates with the same permissions as the Crush process
- File system operations are restricted to the specified working directory
- Network access and dangerous commands respect permission settings
- Sessions are isolated and don't persist sensitive information
- All operations are logged for audit purposes

## Integration with Crush

This tool seamlessly integrates Claude Code's capabilities with Crush's:
- Permission system for security
- Session management for tracking
- Tool ecosystem for extensibility
- Configuration management for consistency
- Error handling and logging for reliability