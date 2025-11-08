# Installation & Configuration Guide

## 🎯 Overview

This comprehensive guide covers all aspects of installing, configuring, and customizing Crush for your development environment. From basic setup to advanced configurations, this guide will help you get the most out of your AI coding assistant.

## 📋 System Requirements

### Minimum Requirements
- **Operating System**: macOS 10.15+, Linux (Ubuntu 18.04+, CentOS 7+), Windows 10+
- **Memory**: 4GB RAM (8GB recommended)
- **Storage**: 500MB free space
- **Terminal**: Any modern terminal emulator
- **Network**: Internet connection for AI provider APIs

### Recommended Requirements
- **Memory**: 8GB+ RAM for optimal performance
- **Storage**: 2GB+ for session data and logs
- **Terminal**: Terminal with 256-color support and Unicode
- **Shell**: bash, zsh, fish, or PowerShell

### Language Server Requirements (Optional)
- **Go**: `gopls` for Go language support
- **TypeScript/JavaScript**: `typescript-language-server`
- **Python**: `pylsp` or `pyright`
- **Rust**: `rust-analyzer`
- **C/C++**: `clangd`

## 🚀 Installation Methods

### 1. Package Managers (Recommended)

#### macOS - Homebrew
```bash
# Install Homebrew if not already installed
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"

# Add Charm tap and install Crush
brew tap charmbracelet/tap
brew install charmbracelet/tap/crush

# Verify installation
crush --version
```

#### Linux - Package Managers

**Arch Linux (AUR)**:
```bash
# Using yay
yay -S crush-bin

# Using paru
paru -S crush-bin

# Manual AUR installation
git clone https://aur.archlinux.org/crush-bin.git
cd crush-bin
makepkg -si
```

**Ubuntu/Debian**:
```bash
# Add Charm repository
sudo mkdir -p /etc/apt/keyrings
curl -fsSL https://repo.charm.sh/apt/gpg.key | sudo gpg --dearmor -o /etc/apt/keyrings/charm.gpg
echo "deb [signed-by=/etc/apt/keyrings/charm.gpg] https://repo.charm.sh/apt/ * *" | sudo tee /etc/apt/sources.list.d/charm.list

# Update and install
sudo apt update
sudo apt install crush

# Verify installation
crush --version
```

**Fedora/RHEL/CentOS**:
```bash
# Add Charm repository
sudo tee /etc/yum.repos.d/charm.repo <<EOF
[charm]
name=Charm
baseurl=https://repo.charm.sh/yum/
enabled=1
gpgcheck=1
gpgkey=https://repo.charm.sh/yum/gpg.key
EOF

# Install
sudo dnf install crush  # Fedora
sudo yum install crush  # RHEL/CentOS

# Verify installation
crush --version
```

#### Windows

**Winget**:
```powershell
# Install using Windows Package Manager
winget install charmbracelet.crush

# Verify installation
crush --version
```

**Scoop**:
```powershell
# Add Charm bucket
scoop bucket add charm https://github.com/charmbracelet/scoop-bucket.git

# Install Crush
scoop install crush

# Verify installation
crush --version
```

**Chocolatey**:
```powershell
# Install Chocolatey if not already installed
Set-ExecutionPolicy Bypass -Scope Process -Force; [System.Net.ServicePointManager]::SecurityProtocol = [System.Net.ServicePointManager]::SecurityProtocol -bor 3072; iex ((New-Object System.Net.WebClient).DownloadString('https://community.chocolatey.org/install.ps1'))

# Install Crush
choco install crush

# Verify installation
crush --version
```

### 2. Node.js/NPM
```bash
# Install globally via NPM
npm install -g @charmland/crush

# Verify installation
crush --version
```

### 3. Nix Package Manager
```bash
# Run directly
nix run github:numtide/nix-ai-tools#crush

# Install in profile
nix profile install github:numtide/nix-ai-tools#crush

# Using flakes
nix shell github:numtide/nix-ai-tools#crush
```

### 4. From Source (Advanced)

**Prerequisites**:
- Go 1.25.0 or later
- Git

```bash
# Clone the repository
git clone https://github.com/charmbracelet/crush.git
cd crush

# Build from source
go build -o crush .

# Install to system PATH
sudo mv crush /usr/local/bin/

# Or install using Go
go install github.com/charmbracelet/crush@latest
```

## 🔧 Initial Configuration

### 1. First Launch Setup

When you first run Crush, it will guide you through the initial setup:

```bash
# Launch Crush
crush
```

The onboarding process will:
1. Detect available AI providers
2. Prompt for API keys
3. Create local configuration
4. Initialize the database
5. Set up default preferences

### 2. API Key Configuration

#### Environment Variables (Recommended)
```bash
# Add to your shell profile (.bashrc, .zshrc, etc.)
export OPENAI_API_KEY="your-openai-api-key"
export ANTHROPIC_API_KEY="your-anthropic-api-key"
export GEMINI_API_KEY="your-gemini-api-key"
export GROQ_API_KEY="your-groq-api-key"

# Reload your shell
source ~/.bashrc  # or ~/.zshrc
```

#### Configuration File
Create a configuration file with API keys:

```json
{
  "$schema": "https://charm.land/crush.json",
  "providers": {
    "openai": {
      "api_key": "$OPENAI_API_KEY"
    },
    "anthropic": {
      "api_key": "$ANTHROPIC_API_KEY"
    }
  }
}
```

### 3. Configuration File Locations

Crush looks for configuration files in this order:

1. **Project-level**: `.crush.json` (highest priority)
2. **Project-level**: `crush.json`
3. **User-level**: 
   - Unix: `~/.config/crush/crush.json`
   - Windows: `%USERPROFILE%\AppData\Local\crush\crush.json`

## ⚙️ Advanced Configuration

### 1. Complete Configuration Schema

```json
{
  "$schema": "https://charm.land/crush.json",
  "models": {
    "large": {
      "model": "gpt-4o",
      "provider": "openai",
      "max_tokens": 4096
    },
    "small": {
      "model": "gpt-3.5-turbo",
      "provider": "openai",
      "max_tokens": 2048
    }
  },
  "providers": {
    "openai": {
      "name": "OpenAI",
      "type": "openai",
      "base_url": "https://api.openai.com/v1",
      "api_key": "$OPENAI_API_KEY",
      "extra_headers": {
        "User-Agent": "Crush/1.0"
      }
    },
    "anthropic": {
      "name": "Anthropic",
      "type": "anthropic",
      "base_url": "https://api.anthropic.com/v1",
      "api_key": "$ANTHROPIC_API_KEY"
    }
  },
  "lsp": {
    "go": {
      "command": "gopls",
      "env": {
        "GOTOOLCHAIN": "go1.25.0"
      }
    },
    "typescript": {
      "command": "typescript-language-server",
      "args": ["--stdio"]
    },
    "python": {
      "command": "pylsp"
    }
  },
  "mcp": {
    "filesystem": {
      "type": "stdio",
      "command": "node",
      "args": ["/path/to/mcp-server.js"]
    },
    "github": {
      "type": "http",
      "url": "https://api.github.com/mcp",
      "headers": {
        "Authorization": "$(echo Bearer $GITHUB_TOKEN)"
      }
    }
  },
  "options": {
    "context_paths": [
      ".cursorrules",
      "CRUSH.md",
      "README.md",
      "docs/CONTEXT.md"
    ],
    "data_directory": ".crush",
    "debug": false,
    "debug_lsp": false,
    "disable_auto_summarize": false,
    "tui": {
      "compact_mode": false,
      "diff_mode": "unified"
    }
  },
  "permissions": {
    "allowed_tools": [
      "view",
      "ls",
      "grep"
    ]
  }
}
```

### 2. Provider-Specific Configurations

#### OpenAI Configuration
```json
{
  "providers": {
    "openai": {
      "type": "openai",
      "base_url": "https://api.openai.com/v1",
      "api_key": "$OPENAI_API_KEY",
      "extra_headers": {
        "OpenAI-Organization": "org-your-org-id"
      },
      "models": [
        {
          "id": "gpt-4o",
          "name": "GPT-4 Omni",
          "context_window": 128000,
          "default_max_tokens": 4096,
          "cost_per_1m_in": 5.0,
          "cost_per_1m_out": 15.0
        }
      ]
    }
  }
}
```

#### Anthropic Configuration
```json
{
  "providers": {
    "anthropic": {
      "type": "anthropic",
      "base_url": "https://api.anthropic.com/v1",
      "api_key": "$ANTHROPIC_API_KEY",
      "extra_headers": {
        "anthropic-version": "2023-06-01"
      },
      "models": [
        {
          "id": "claude-3-5-sonnet-20241022",
          "name": "Claude 3.5 Sonnet",
          "context_window": 200000,
          "default_max_tokens": 8192,
          "cost_per_1m_in": 3.0,
          "cost_per_1m_out": 15.0
        }
      ]
    }
  }
}
```

#### Local Model Configuration (Ollama)
```json
{
  "providers": {
    "ollama": {
      "name": "Ollama",
      "type": "openai",
      "base_url": "http://localhost:11434/v1/",
      "api_key": "not-required",
      "models": [
        {
          "id": "llama3.1:8b",
          "name": "Llama 3.1 8B",
          "context_window": 128000,
          "default_max_tokens": 4096,
          "cost_per_1m_in": 0.0,
          "cost_per_1m_out": 0.0
        }
      ]
    }
  }
}
```

### 3. Language Server Protocol (LSP) Setup

#### Go Development
```bash
# Install gopls
go install golang.org/x/tools/gopls@latest

# Configuration
{
  "lsp": {
    "go": {
      "command": "gopls",
      "env": {
        "GOTOOLCHAIN": "go1.25.0",
        "GOPROXY": "https://proxy.golang.org"
      },
      "options": {
        "gofumpt": true,
        "staticcheck": true
      }
    }
  }
}
```

#### TypeScript/JavaScript Development
```bash
# Install TypeScript language server
npm install -g typescript-language-server typescript

# Configuration
{
  "lsp": {
    "typescript": {
      "command": "typescript-language-server",
      "args": ["--stdio"],
      "filetypes": ["typescript", "javascript", "typescriptreact", "javascriptreact"]
    }
  }
}
```

#### Python Development
```bash
# Install Python LSP server
pip install python-lsp-server[all]

# Configuration
{
  "lsp": {
    "python": {
      "command": "pylsp",
      "options": {
        "plugins": {
          "pycodestyle": {"enabled": false},
          "mccabe": {"enabled": false},
          "pyflakes": {"enabled": true},
          "pylsp_mypy": {"enabled": true}
        }
      }
    }
  }
}
```

#### Rust Development
```bash
# Install rust-analyzer
rustup component add rust-analyzer

# Configuration
{
  "lsp": {
    "rust": {
      "command": "rust-analyzer",
      "options": {
        "cargo": {
          "buildScripts": {
            "enable": true
          }
        },
        "procMacro": {
          "enable": true
        }
      }
    }
  }
}
```

### 4. Model Context Protocol (MCP) Setup

#### File System MCP Server
```json
{
  "mcp": {
    "filesystem": {
      "type": "stdio",
      "command": "node",
      "args": ["/path/to/filesystem-mcp-server.js"],
      "env": {
        "NODE_ENV": "production"
      }
    }
  }
}
```

#### HTTP-based MCP Server
```json
{
  "mcp": {
    "github": {
      "type": "http",
      "url": "https://api.github.com/mcp",
      "headers": {
        "Authorization": "$(echo Bearer $GITHUB_TOKEN)",
        "Accept": "application/vnd.github.v3+json"
      },
      "timeout": 30
    }
  }
}
```

#### Server-Sent Events MCP Server
```json
{
  "mcp": {
    "realtime": {
      "type": "sse",
      "url": "https://realtime-api.example.com/mcp/sse",
      "headers": {
        "API-Key": "$(echo $REALTIME_API_KEY)"
      }
    }
  }
}
```

## 🔒 Security Configuration

### 1. Permission System
```json
{
  "permissions": {
    "allowed_tools": [
      "view",
      "ls",
      "grep",
      "git_status"
    ]
  }
}
```

### 2. File Ignoring
Create a `.crushignore` file in your project root:

```
# Build artifacts
dist/
build/
target/
*.o
*.so
*.dylib

# Dependencies
node_modules/
vendor/
.venv/

# Sensitive files
.env
.env.local
secrets/
*.key
*.pem

# IDE files
.vscode/
.idea/
*.swp
*.swo

# OS files
.DS_Store
Thumbs.db

# Logs
*.log
logs/
```

### 3. API Key Security
```bash
# Use a secure environment file
echo "OPENAI_API_KEY=your-key-here" >> ~/.crush_env
chmod 600 ~/.crush_env

# Source in your shell profile
echo "source ~/.crush_env" >> ~/.bashrc
```

## 🛠️ Troubleshooting

### Common Installation Issues

#### Issue: Command not found
```bash
# Check if Crush is in PATH
which crush

# If not found, add to PATH
export PATH="$PATH:/usr/local/bin"
```

#### Issue: Permission denied
```bash
# Fix permissions
chmod +x /usr/local/bin/crush

# Or reinstall with proper permissions
sudo chown $(whoami) /usr/local/bin/crush
```

#### Issue: LSP server not found
```bash
# Check if LSP server is installed
which gopls
which typescript-language-server

# Install missing LSP servers
go install golang.org/x/tools/gopls@latest
npm install -g typescript-language-server
```

### Configuration Validation
```bash
# Validate configuration
crush config validate

# Show current configuration
crush config show

# Test API connectivity
crush config test-providers
```

### Debug Mode
```bash
# Run with debug logging
crush --debug

# Enable LSP debugging
export CRUSH_DEBUG_LSP=true
crush

# View logs
crush logs --tail 100
crush logs --follow
```

## 📊 Performance Tuning

### 1. Memory Optimization
```json
{
  "options": {
    "data_directory": ".crush",
    "disable_auto_summarize": false
  }
}
```

### 2. Network Optimization
```json
{
  "providers": {
    "openai": {
      "extra_headers": {
        "Connection": "keep-alive"
      }
    }
  }
}
```

### 3. Database Optimization
```bash
# Vacuum database periodically
sqlite3 .crush/crush.db "VACUUM;"

# Check database size
du -h .crush/crush.db
```

---

*This installation guide provides comprehensive coverage of all installation methods and configuration options. For additional help, consult the project documentation or community support channels.*
