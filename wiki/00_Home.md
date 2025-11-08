# Crush - Terminal-Based AI Assistant Wiki

<p align="center">
    <img width="450" alt="Charm Crush Logo" src="https://github.com/user-attachments/assets/adc1a6f4-b284-4603-836c-59038caa2e8b" />
</p>

## 🚀 Welcome to Crush

Crush is a powerful terminal-based AI assistant designed specifically for software development. It provides an interactive chat interface with AI capabilities, code analysis, and Language Server Protocol (LSP) integration to assist developers in writing, debugging, and understanding code directly from the terminal.

## 📚 Documentation Structure

This Wiki is organized into the following sections:

### 🏁 Getting Started
- **[Installation & Quick Start](01_Getting_Started.md)** - Get up and running in minutes
- **[Configuration Guide](guides/01_Installation_Guide.md)** - Detailed setup and configuration

### 🏗️ Architecture & Design
- **[Architecture Overview](02_Architecture_Overview.md)** - High-level system design and philosophy
- **[Core Modules](architecture/01_Core_Modules/)** - Deep dive into each system component
- **[Business Workflows](architecture/02_Business_Workflows.md)** - Key user journeys and data flows
- **[Data Model](architecture/03_Data_Model.md)** - Database schema and entity relationships

### 💎 Code Excellence
- **[Code Highlights](03_Code_Highlights.md)** - Elegant implementations and design patterns
- **[Testing Strategy](04_Testing_Strategy.md)** - Testing approach and best practices

### 🛡️ Risk Assessment
- **[Security Vulnerabilities](risks/01_Security_Vulnerabilities.md)** - Security analysis and recommendations
- **[Known Issues](risks/02_Known_Bugs_And_Issues.md)** - Current limitations and bugs
- **[Refactoring Suggestions](risks/03_Refactoring_Suggestions.md)** - Code quality improvements

### 📖 Guides & References
- **[Deployment Guide](guides/02_Deployment_Guide.md)** - Production deployment strategies
- **[Contribution Guide](guides/03_Contribution_Guide.md)** - How to contribute to the project

## 🎯 Project Overview

### Core Purpose
Crush serves as your coding companion, integrating seamlessly with your development workflow to provide:
- **Multi-Model AI Support** - Choose from OpenAI, Anthropic, Google Gemini, and more
- **LSP Integration** - Leverage Language Server Protocols for enhanced code understanding
- **MCP Support** - Extensible via Model Context Protocol servers
- **Session Management** - Maintain context across multiple work sessions
- **Terminal-First Design** - Built for developers who live in the terminal

### Key Features
- 🤖 **Multi-Provider AI** - Support for major AI providers with easy switching
- 🔄 **Session Persistence** - Maintain conversation history and context
- 🛠️ **Developer Tools** - Built-in code analysis, file operations, and shell integration
- 🔌 **Extensible Architecture** - Plugin system via MCP and LSP protocols
- 🎨 **Rich TUI** - Beautiful terminal interface with syntax highlighting
- 🔒 **Permission System** - Granular control over tool execution
- 📊 **Usage Tracking** - Token usage and cost monitoring

### Technology Stack
- **Language**: Go 1.25+
- **UI Framework**: Bubble Tea v2 (Terminal UI)
- **Database**: SQLite with SQLC for type-safe queries
- **AI Integration**: Multiple provider SDKs (OpenAI, Anthropic, etc.)
- **Configuration**: JSON with schema validation
- **Build System**: Task (Taskfile.yaml)
- **Testing**: Go testing with testify

## 🚦 Quick Navigation

| I want to... | Go to... |
|---------------|----------|
| **Get started quickly** | [Quick Start Guide](01_Getting_Started.md) |
| **Understand the architecture** | [Architecture Overview](02_Architecture_Overview.md) |
| **See elegant code examples** | [Code Highlights](03_Code_Highlights.md) |
| **Deploy to production** | [Deployment Guide](guides/02_Deployment_Guide.md) |
| **Contribute to the project** | [Contribution Guide](guides/03_Contribution_Guide.md) |
| **Understand security implications** | [Security Assessment](risks/01_Security_Vulnerabilities.md) |

## 🤝 Community & Support

- **GitHub**: [charmbracelet/crush](https://github.com/charmbracelet/crush)
- **Discord**: [Charm Community](https://charm.land/discord)
- **Twitter**: [@charmcli](https://twitter.com/charmcli)
- **License**: FSL-1.1-MIT

---

*This documentation is generated from comprehensive codebase analysis and is designed to provide both high-level understanding and deep technical insights for developers of all levels.*
