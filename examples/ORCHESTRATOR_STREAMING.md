# Claude Code Orchestrator with Streaming Output

## 🎯 Problem Solved

**Before**: When running Claude Code with orchestrator, users had to wait 76+ seconds with no feedback, only to see a failure message at the end.

**After**: Real-time streaming shows every step as it happens - system initialization, assistant thinking, tool usage, and results.

## ✨ New Features

### 1. **Real-time Event Streaming**
- Parses and displays Claude Code's stream-json format in real-time
- Shows thinking, tool usage, and progress as it happens
- No more waiting in the dark!

### 2. **Beautiful Terminal UI**
```
═══════ Step 1

▲ Claude Code (model: sonnet)
  ◆ Available tools: Bash, Glob, Grep, Read, Edit, Write, ...

💭 thinking......

● I'll help you create a complete Go web application. Let me start by
  planning the implementation steps.

→ Using tool:
    TodoWrite (id: tool_abc123)

  ✓ Tool execution complete

▼ Result
  • Cost: $0.0134
  • Duration: 5.234s
  • Turns: 8
  • Model usage:
    • sonnet: 341 in, 892 out (0.0134 USD)

═══════════════════════════════════════════
          Session Complete
═══════════════════════════════════════════

Total Cost: $0.0134
Duration: 5.234s
Total Turns: 8
Tools Used: 1
```

### 3. **Event Type Support**
- ✅ **system**: Initialization, model info, available tools
- ✅ **assistant**: Thinking, text messages, tool usage
- ✅ **user**: Tool results
- ✅ **result**: Final output with metrics (cost, duration, turns, model usage)

### 4. **Color-coded Output**
- 🟦 **Blue** (system): Claude Code initialization
- 🟪 **Magenta** (assistant): Assistant messages and thinking
- 🟡 **Yellow** (tools): Tool usage notifications
- 🟢 **Green** (success): Tool completion
- 🔵 **Cyan** (result): Final results and summary

## 📁 Files Added/Modified

```
internal/
├── orchestrator/
│   ├── claude_code_orchestrator.go  (modified - uses streaming)
│   └── stream_renderer.go           (new - real-time rendering)
└── agent/tools/
    └── claude_code.go              (modified - params updated)

examples/
├── claude_code_orchestrator_example.go    (original)
└── claude_code_streaming_demo.go          (new demo!)
```

## 🚀 Usage

### Enable Streaming (API)

```go
params := tools.ClaudeCodeParams{
    Query: "Create a complete web application",
    EnableOrchestrator: true,  // ✨ Enable multi-turn orchestration
    MaxIterations: 3,          // Max iterations
    Verbose: true,             // Show verbose output
}

// Streaming is automatically enabled with orchestrator!
```

### Run the Demo

```bash
cd /Users/mxue/GitRepos/Coding/crush

# Build and run the streaming demo
go run examples/claude_code_streaming_demo.go
```

This demo shows a complete Python script creation task with real-time output!

### Update Existing Code

If you have existing code using `claude_code` tool:

```json
{
  "query": "Your complex task",
  "enable_orchestrator": true,
  "max_iterations": 3
}
```

Just add `enable_orchestrator: true` and streaming is automatically enabled!

## 🎯 Key Differences

### Before (Regular Mode)
```
🚀 Starting...
[waits 76 seconds...]
❌ Failed: claude process failed
```

### After (Streaming Mode)
```
🚀 Starting...

═══════ Step 1

▲ Claude Code (model: sonnet)
  ◆ Available tools: Bash, Glob, Grep, Read, Edit, Write...

💭 thinking...

● I'll create the project structure for you.

→ Using tool:
    Write (id: tool_001)

  ✓ Tool execution complete

→ Using tool:
    Bash (id: tool_002)

  ✓ Tool execution complete

💭 thinking...

● Project created successfully!

▼ Result
  • Cost: $0.0234
  • Duration: 12.456s
  • Turns: 15

═══════════════════════════════════════════
          Session Complete
═══════════════════════════════════════════
```

## 🎨 Event Flow

```
User Query
    ↓
Orchestrator Starts
    ↓
System Event → Shows Claude Code init, model, tools
    ↓
Assistant Event → Shows thinking, messages, tool plans
    ↓
Assistant Event (tool_use) → Shows which tool will be used
    ↓
Assistant Event (text) → Shows assistant's message
    ↓
User Event (tool_result) → Confirms tool completion
    ↓
[Repeats as needed...]
    ↓
Result Event → Shows final result with metrics
    ↓
Orchestrator Analyzes → Decides if another iteration needed
    ↓
[Continues to next iteration or finishes]
    ↓
Show Summary → Total cost, duration, turns, tools used
```

## 🔧 Technical Details

### Streaming Output Format

Claude Code's `OutputStreamJSON` produces newline-delimited JSON:

```json
{"type":"system","subtype":"init","model":"sonnet","tools":["Bash","Write"]}
{"type":"assistant","message":{"role":"assistant","content":[{"type":"thinking","thinking":"Planning..."}]}}
{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"I'll help you..."}]}}
{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","name":"Bash","id":"tool_123"}]}}
{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"tool_123"}]}}
{"type":"result","total_cost_usd":0.0134,"num_turns":8,"result":"Final output"}
```

### Parser Architecture

```
Claude Code Process
         ↓
stdout (stream-json)
         ↓
Bufio Scanner (line by line)
         ↓
JSON Unmarshal
         ↓
StreamEvent Channel
         ↓
StreamRenderer
         ↓
Terminal Output
```

## 📊 Benefits

1. **Immediate Feedback**: See what's happening in real-time
2. **Better Debugging**: Identify where failures occur instantly
3. **User Experience**: No more staring at blank screen
4. **Progress Tracking**: Watch tools execute and complete
5. **Cost Awareness**: See costs accumulate in real-time

## 🎮 Usage Tips

### For Complex Tasks
Use orchestrator with higher iteration limits:
```json
{
  "query": "Build a complete web app with frontend, backend, and database",
  "enable_orchestrator": true,
  "max_iterations": 5
}
```

### For Simple Tasks
Use regular mode (faster, single-turn):
```json
{
  "query": "Create a simple hello world in Go"
}
```

## 🛠️ Next Steps

Current implementation covers:
- ✅ System events (init)
- ✅ Assistant messages (text, thinking, tool_use)
- ✅ User events (tool_result)
- ✅ Result events with metrics
- ✅ Permission denial notifications
- ✅ Error display
- ✅ Tool usage tracking
- ✅ Cost and duration tracking

Potential enhancements:
- Progress bars for long operations
- Interactive permission granting
- Real-time todo list visualization
- Expand/collapse sections
- Search/filter events
- Export event log to file

## 📝 Example Output

See `claude_code_output/claude_code_results.txt` for a complete example of streaming output with permission denials and final results.

---

**Ready to try?** Run `go run examples/claude_code_streaming_demo.go`!
