# Claude Code Integration for Crush

This document describes the Claude Code integration that has been successfully implemented in the Crush project.

## Overview

The Claude Code tool (`claude_code`) provides headless access to Claude Code, Anthropic's AI coding assistant that can execute commands, edit files, and perform complex development tasks autonomously.

## Features

- **Multi-Model Support**: Choose between Claude Opus, Sonnet, and Haiku models
- **Session Management**: Resume existing sessions or create new ones
- **Tool Control**: Fine-grained control over which tools Claude Code can use
- **Cost Tracking**: Transparent cost reporting for API usage
- **Permission Integration**: Respects Crush's permission system for security
- **Structured Output**: JSON responses with detailed session information

## Installation

1. Ensure Claude Code is installed on your system:
   ```bash
   npm install -g @anthropic-ai/claude-code
   ```

2. The integration is automatically available once Crush is built with the Claude Code dependency.

## Usage Examples

### Basic Code Generation

```json
{
  "tool": "claude_code",
  "params": {
    "query": "Create a simple Go HTTP server with basic routing and middleware",
    "model": "sonnet"
  }
}
```

### Complex Development Task

```json
{
  "tool": "claude_code",
  "params": {
    "query": "Set up a complete React TypeScript project with testing, linting, and build configuration",
    "model": "opus",
    "max_turns": 15,
    "custom_instructions": "Use modern best practices and include comprehensive error handling",
    "working_dir": "frontend"
  }
}
```

### Resume Previous Session

```json
{
  "tool": "claude_code",
  "params": {
    "query": "Add unit tests to the authentication module we created earlier",
    "session_id": "550e8400-e29b-41d4-a716-446655440000"
  }
}
```

### Restricted Tool Usage

```json
{
  "tool": "claude_code",
  "params": {
    "query": "Analyze the codebase structure and create a documentation outline",
    "allowed_tools": ["bash", "glob", "view"],
    "disallowed_tools": ["edit", "write"]
  }
}
```

## Response Format

The tool returns a JSON object containing:

```json
{
  "result": "The main output from Claude Code",
  "session_id": "unique-session-identifier",
  "cost_usd": 0.0123,
  "duration_ms": 15420,
  "num_turns": 5,
  "is_error": false,
  "error": "",
  "model_used": "claude-sonnet"
}
```

## Configuration

### Model Selection

- `opus`: Most capable model for complex tasks
- `sonnet`: Balanced performance and speed (default)
- `haiku`: Fastest model for simpler tasks

### Tool Control

- `allowed_tools`: List of tools Claude Code is permitted to use
- `disallowed_tools`: List of tools Claude Code is prohibited from using
- If neither is specified, all built-in tools are available

### Session Management

- `session_id`: Resume an existing session
- `fork_session`: Create a new session based on an existing one
- Sessions persist across tool calls, allowing for complex multi-step workflows

## Security Considerations

- Claude Code operates with the same permissions as the Crush process
- File system operations are restricted to the specified working directory
- Network access and dangerous commands respect permission settings
- All operations are logged for audit purposes
- Sessions are isolated and don't persist sensitive information

## Integration with Crush

The Claude Code tool seamlessly integrates with Crush's existing infrastructure:

- **Permission System**: Uses Crush's permission framework for security
- **Session Management**: Integrates with Crush's session tracking
- **Tool Ecosystem**: Works alongside other Crush tools
- **Configuration Management**: Respects Crush's configuration system
- **Error Handling**: Follows Crush's error handling patterns

## Testing

The integration includes comprehensive tests covering:

- Basic functionality and parameter validation
- Permission system integration
- Error handling for invalid inputs
- Model selection and validation
- Session management
- JSON parsing and response formatting

Run tests with:
```bash
go test ./internal/agent/tools -run TestClaudeCode -v
```

## Implementation Details

The integration follows Crush's existing tool patterns:

- **Tool Registration**: Registered in `coordinator.go` alongside other tools
- **Parameter Validation**: Comprehensive input validation and error handling
- **Permission Integration**: Full integration with Crush's permission system
- **Structured Responses**: JSON responses with detailed metadata
- **Error Handling**: Proper error propagation and user-friendly messages

## Future Enhancements

Potential improvements for future versions:

- **Streaming Support**: Real-time streaming of Claude Code responses
- **MCP Integration**: Support for Model Context Protocol servers
- **Custom Tools**: Ability to define custom tools for Claude Code
- **Advanced Filtering**: More sophisticated tool permission filtering
- **Performance Metrics**: Detailed performance and usage analytics

## Troubleshooting

### Common Issues

1. **Claude Code Not Found**: Ensure Claude Code is installed globally
   ```bash
   npm install -g @anthropic-ai/claude-code
   ```

2. **Permission Denied**: Check Crush's permission settings

3. **Session Not Found**: Verify the session ID is correct and hasn't expired

4. **Model Not Available**: Ensure the requested model is available in your Claude Code installation

### Debug Information

The tool provides detailed logging for troubleshooting:
- Session creation and management
- Model selection and configuration
- Tool permission requests
- Error messages and stack traces

## Conclusion

The Claude Code integration provides a powerful way to leverage Anthropic's AI coding assistant within the Crush environment. It maintains security, provides comprehensive functionality, and follows Crush's established patterns for consistency and reliability.