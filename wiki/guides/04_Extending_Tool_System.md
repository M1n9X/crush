# Extending the Tool System

Follow the steps below to add a new agent tool in Crush. All examples reference `internal/agent/tools`.

## 1. Understand the Building Blocks

- Tools are plain Go functions created via `fantasy.NewAgentTool`.
- The handler receives a strongly typed parameter struct (decoded from JSON) and the raw `fantasy.ToolCall` metadata.
- Dependencies (history, permissions, LSP, working directory, etc.) are injected via the constructor and stored in the closure.

## 2. Scaffold a Tool

```go
package tools

import (
    "context"
    "encoding/json"
    "fmt"

    "charm.land/fantasy"
    "github.com/charmbracelet/crush/internal/permission"
)

type MyToolParams struct {
    Path string `json:"path" description:"File to inspect"`
}

type myToolDeps struct {
    permissions permission.Service
}

func NewMyTool(deps myToolDeps) fantasy.AgentTool {
    return fantasy.NewAgentTool(
        "my_tool",
        "Reads metadata about a file and reports findings.",
        func(ctx context.Context, params MyToolParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
            if params.Path == "" {
                return fantasy.NewTextErrorResponse("path is required"), nil
            }

            if !deps.permissions.Request(permission.CreatePermissionRequest{
                SessionID:   tools.GetSessionFromContext(ctx),
                ToolCallID:  call.ID,
                ToolName:    "my_tool",
                Action:      "read",
                Path:        params.Path,
                Description: fmt.Sprintf("Read metadata for %s", params.Path),
                Params:      params,
            }) {
                return fantasy.ToolResponse{}, permission.ErrorPermissionDenied
            }

            info, err := os.Stat(params.Path)
            if err != nil {
                return fantasy.NewTextErrorResponse(err.Error()), nil
            }
            payload, _ := json.Marshal(map[string]any{
                "name": info.Name(),
                "size": info.Size(),
            })
            return fantasy.NewJSONResponse(string(payload)), nil
        },
    )
}
```

Key points:

- The handler returns either `fantasy.NewTextResponse` or `fantasy.NewJSONResponse` (plus optional metadata).
- Permission prompts are optional but recommended for anything that touches disk/network.
- Use `tools.GetSessionFromContext` / `GetMessageFromContext` to associate work with the right session/message.

## 3. Register the Tool

Edit `coordinator.buildTools` and append your tool when building the slice. Respect `config.Agent.AllowedTools` so that agents can opt in/out:

```go
if slices.Contains(agent.AllowedTools, "my_tool") {
    allTools = append(allTools, tools.NewMyTool(myToolDeps{
        permissions: c.permissions,
    }))
}
```

## 4. Document & Test

- Add unit tests under `internal/agent/tools/my_tool_test.go`. Reuse `internal/agent/common_test.go` to spin up in-memory services.
- Update `README.md` or relevant wiki pages so users know how/when to use the new tool.
- Consider adding a usage example to `wiki/guides/04_Extending_Tool_System.md` (this doc) or the tool’s Markdown help file in `internal/agent/tools/*.md` if the tool merits inline instructions.

## 5. Tips

- **Input validation**: Fail fast with a helpful error; the agent will surface the text back to the model.
- **Streaming output**: If the tool produces a lot of data, cap it (see `bash` tool’s `MaxOutputLength`) and attach metadata with links to full logs.
- **Context data**: Use the `messageID` context value if you need to update the assistant message directly (e.g., annotate with structured metadata).
- **Reusability**: Keep constructors thin so tests can inject fake dependencies.

That’s it—once registered, the agent will advertise the tool to providers automatically.
