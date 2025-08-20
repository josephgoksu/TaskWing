# TaskWing

**AI-powered CLI task manager built for developers**

[![Go](https://img.shields.io/badge/Go-1.24+-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![MCP](https://img.shields.io/badge/MCP-Compatible-purple.svg)](https://modelcontextprotocol.io)

TaskWing integrates directly with Claude Code, Cursor, and other AI tools via the Model Context Protocol. Manage tasks from your terminal or let AI handle it for you.

## Why TaskWing?

- **🤖 AI-Native**: Full MCP integration for Claude Code, Cursor, and other AI tools
- **⚡ Zero Config**: Works immediately, stores data locally in your project
- **🔗 Smart Dependencies**: Automatic dependency tracking and circular reference prevention
- **📚 Learning System**: Captures patterns from completed projects for future AI assistance
- **🚀 Developer-First**: Built for developers who value focus and efficiency

## Quick Start

### Installation

```bash
# One-liner install (recommended)
curl -sSfL https://raw.githubusercontent.com/josephgoksu/taskwing.app/main/install.sh | sh

# Or via Go
go install github.com/josephgoksu/taskwing.app@latest
```

For other installation methods, see [Installation Guide](DOCS.md#installation).

### First Steps

```bash
# Initialize in your project
taskwing init

# Add and manage tasks
taskwing add --title "Fix auth bug" --priority urgent
taskwing list
taskwing done <task-id>
```

### AI Integration

```bash
# Start MCP server for AI tools
taskwing mcp
```

See [MCP Setup Guide](MCP.md#quick-setup) for AI tool configuration.

## Core Features

- **Task Management**: Create, update, track, and complete tasks with rich metadata
- **Current Task Focus**: Track what you're actively working on for context-aware AI assistance
- **Smart Dependencies**: Manage task relationships and prevent circular dependencies
- **Pattern Library**: Learn from completed projects to improve future planning
- **MCP Integration**: Direct AI tool integration for intelligent task assistance

For complete command reference, see [User Guide](DOCS.md#commands-reference).

## Documentation

| Document | Purpose |
|----------|----------|
| **[DOCS.md](DOCS.md)** | User guide - Installation, commands, workflows |
| **[MCP.md](MCP.md)** | AI integration - Setup and tool reference |
| **[CLAUDE.md](CLAUDE.md)** | Developer guide - Architecture and contributing |

## Architecture Overview

TaskWing is a local-first CLI tool that stores all data in your project directory. It provides an MCP server for AI tool integration, enabling intelligent task management without cloud dependencies.

For detailed architecture information, see [Developer Guide](CLAUDE.md#architecture).

## Contributing

We welcome contributions! TaskWing is built with Go 1.24+ using Cobra CLI framework and the MCP SDK for AI integration.

See [CLAUDE.md](CLAUDE.md) for development setup and contributing guidelines.

## License

MIT License - see [LICENSE](LICENSE) file.

---

**Built for the terminal. Powered by AI. Made for developers.** 🚀
