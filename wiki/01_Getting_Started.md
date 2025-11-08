# Getting Started with Crush

## 🎯 Overview

Crush is your terminal-based AI coding assistant that integrates seamlessly with your development workflow. This guide will get you up and running in under 5 minutes.

## 📋 Prerequisites

- **Operating System**: macOS, Linux, Windows (PowerShell/WSL), FreeBSD, OpenBSD, or NetBSD
- **Terminal**: Any modern terminal emulator
- **Go** (optional): Go 1.25+ if building from source
- **AI Provider**: API key from at least one supported provider

### Supported AI Providers

| Provider | Environment Variable | Notes |
|----------|---------------------|-------|
| **OpenAI** | `OPENAI_API_KEY` | GPT-4, GPT-3.5, etc. |
| **Anthropic** | `ANTHROPIC_API_KEY` | Claude models |
| **Google Gemini** | `GEMINI_API_KEY` | Gemini Pro, etc. |
| **OpenRouter** | `OPENROUTER_API_KEY` | Access to multiple models |
| **Groq** | `GROQ_API_KEY` | Fast inference |
| **AWS Bedrock** | `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY` | Claude via AWS |
| **Azure OpenAI** | `AZURE_OPENAI_ENDPOINT`, `AZURE_OPENAI_API_KEY` | Enterprise OpenAI |

## 🚀 Installation

### Package Managers (Recommended)

```bash
# macOS - Homebrew
brew install charmbracelet/tap/crush

# Linux - Arch
yay -S crush-bin

# Node.js - NPM
npm install -g @charmland/crush

# Windows - Winget
winget install charmbracelet.crush

# Windows - Scoop
scoop bucket add charm https://github.com/charmbracelet/scoop-bucket.git
scoop install crush

# Nix
nix run github:numtide/nix-ai-tools#crush
```

### Debian/Ubuntu

```bash
sudo mkdir -p /etc/apt/keyrings
curl -fsSL https://repo.charm.sh/apt/gpg.key | sudo gpg --dearmor -o /etc/apt/keyrings/charm.gpg
echo "deb [signed-by=/etc/apt/keyrings/charm.gpg] https://repo.charm.sh/apt/ * *" | sudo tee /etc/apt/sources.list.d/charm.list
sudo apt update && sudo apt install crush
```

### From Source

```bash
go install github.com/charmbracelet/crush@latest
```

## ⚡ Quick Start (5-Minute Setup)

### 1. Set Your API Key

Choose your preferred AI provider and set the environment variable:

```bash
# For OpenAI
export OPENAI_API_KEY="your-api-key-here"

# For Anthropic (Claude)
export ANTHROPIC_API_KEY="your-api-key-here"

# For Google Gemini
export GEMINI_API_KEY="your-api-key-here"
```

### 2. Launch Crush

```bash
crush
```

On first launch, Crush will:
- Detect your API keys automatically
- Create a `.crush` directory in your current project
- Initialize the local database
- Present you with the onboarding interface

### 3. Your First Conversation

Try these example prompts:

```
# Code analysis
"Analyze the structure of this Go project"

# Code generation
"Create a REST API handler for user authentication in Go"

# Debugging help
"Help me debug this function - it's not handling errors correctly"

# Documentation
"Generate documentation for this module"
```

### 4. Essential Commands

| Key Binding | Action | Description |
|-------------|--------|-------------|
| `Ctrl+P` | Commands | Open command palette |
| `Ctrl+S` | Sessions | Switch between chat sessions |
| `Ctrl+M` | Models | Switch AI models |
| `Ctrl+Q` | Quit | Exit application |
| `Ctrl+C` | Cancel | Stop current AI response |
| `?` | Help | Show all keybindings |

## 🔧 Basic Configuration

### Project-Level Configuration

Create a `crush.json` file in your project root:

```json
{
  "$schema": "https://charm.land/crush.json",
  "lsp": {
    "go": {
      "command": "gopls"
    },
    "typescript": {
      "command": "typescript-language-server",
      "args": ["--stdio"]
    }
  },
  "options": {
    "context_paths": [
      ".cursorrules",
      "CRUSH.md",
      "README.md"
    ]
  }
}
```

### Global Configuration

For user-wide settings, create:
- **Unix**: `~/.config/crush/crush.json`
- **Windows**: `%USERPROFILE%\AppData\Local\crush\crush.json`

## 🛠️ Development Workflow Integration

### 1. LSP Integration

Crush automatically detects and uses Language Server Protocols for better code understanding:

```json
{
  "lsp": {
    "go": { "command": "gopls" },
    "rust": { "command": "rust-analyzer" },
    "python": { "command": "pylsp" },
    "javascript": { "command": "typescript-language-server", "args": ["--stdio"] }
  }
}
```

### 2. Context Files

Crush reads project context from these files (in order of priority):

- `.cursorrules`
- `CRUSH.md` / `crush.md`
- `CLAUDE.md`
- `GEMINI.md`
- `.github/copilot-instructions.md`

### 3. File Ignoring

Create a `.crushignore` file to exclude files from AI context:

```
# Ignore build artifacts
dist/
build/
*.log

# Ignore sensitive files
.env
secrets/
```

## 🎨 Interface Overview

```
┌─────────────────────────────────────────────────────────────┐
│ 🤖 Crush - Chat Session: "Project Analysis"                │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│ 👤 You: Analyze this Go project structure                   │
│                                                             │
│ 🤖 Assistant: I'll analyze your Go project structure...     │
│    Based on the codebase, this is a terminal-based AI      │
│    assistant with the following architecture:              │
│    • cmd/ - CLI commands and entry points                  │
│    • internal/ - Core application logic                    │
│    • pkg/ - Reusable packages                              │
│                                                             │
├─────────────────────────────────────────────────────────────┤
│ Type your message... (Ctrl+P for commands)                 │
├─────────────────────────────────────────────────────────────┤
│ Model: gpt-4o | Session: main | Tokens: 1.2K | Cost: $0.02 │
└─────────────────────────────────────────────────────────────┘
```

## 🚨 Common Issues & Solutions

### Issue: "No API key found"
**Solution**: Set the appropriate environment variable for your chosen provider.

### Issue: "LSP server not found"
**Solution**: Install the required language server (e.g., `go install golang.org/x/tools/gopls@latest` for Go).

### Issue: "Permission denied for tool execution"
**Solution**: Use `--yolo` flag for auto-approval or configure `allowed_tools` in your config.

### Issue: "Context window exceeded"
**Solution**: Use the compact feature (`Ctrl+P` → Compact) to summarize the conversation.

## 🎯 Next Steps

Now that you're set up:

1. **[Explore Architecture](02_Architecture_Overview.md)** - Understand how Crush works internally
2. **[Advanced Configuration](guides/01_Installation_Guide.md)** - Customize Crush for your workflow
3. **[Code Highlights](03_Code_Highlights.md)** - See examples of elegant implementations
4. **[Contribute](guides/03_Contribution_Guide.md)** - Help improve Crush

## 💡 Pro Tips

- **Use sessions** to maintain context for different projects or tasks
- **Configure LSPs** for your languages to get better code understanding
- **Set up context files** to provide project-specific instructions
- **Use the command palette** (`Ctrl+P`) to discover all available features
- **Monitor token usage** in the status bar to manage costs

---

*Ready to boost your coding productivity? Start chatting with your new AI assistant!*
