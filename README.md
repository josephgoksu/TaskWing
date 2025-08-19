# TaskWing

**AI-powered CLI task manager built for developers**

[![Go](https://img.shields.io/badge/Go-1.24+-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![MCP](https://img.shields.io/badge/MCP-Compatible-purple.svg)](https://modelcontextprotocol.io)

TaskWing integrates directly with Claude Code, Cursor, and other AI tools via the Model Context Protocol. Manage tasks from your terminal or let AI handle it for you.

## Why TaskWing?

- **🤖 AI-Native**: First CLI tool with full MCP integration for Claude Code/Cursor
- **⚡ Zero Config**: Works out of the box, stores data locally
- **🔗 Smart Dependencies**: Automatic dependency tracking and circular reference prevention
- **📚 Learning System**: Captures patterns from completed projects for future AI assistance
- **🗄️ Knowledge Archive**: Preserves project history and lessons learned
- **🚀 Developer UX**: Built by developers who hate context switching

## Quick Start

### Installation

Choose your preferred method:

```bash
# Go install (recommended)
go install github.com/josephgoksu/taskwing.app@latest

# Or download from releases
# https://github.com/josephgoksu/taskwing.app/releases

# Or build from source
git clone https://github.com/josephgoksu/taskwing.app
cd taskwing-app && go build -o taskwing main.go
```

### First Steps

```bash
# Initialize in your project
taskwing init

# Add your first task
taskwing add --title "Fix auth bug" --priority urgent

# View tasks
taskwing list

# Mark task complete
taskwing done <task-id>
```

### AI Integration (Optional)

Enable AI-powered task management:

```bash
# Start MCP server
taskwing mcp

# Configure your AI tool (Claude Code, Cursor, etc.)
# See MCP.md for complete setup instructions
```

## Core Features

### Essential Commands

```bash
taskwing add                    # Interactive task creation
taskwing list                   # View all tasks
taskwing list --priority high   # Filter by priority
taskwing update <id>            # Modify existing task
taskwing done <id>              # Mark complete
taskwing delete <id>            # Remove task
```

### Smart Task Management

```bash
# Dependencies and relationships
taskwing add --dependencies "task1,task2"

# Current task tracking
taskwing current set <id>       # Set active task
taskwing current show           # Show current task

# Advanced search and filtering
taskwing search "auth bug"
taskwing list --status pending --sort-by priority

# Pattern-based task planning
taskwing patterns match "consolidate documentation"
taskwing patterns list                  # View all patterns

# Project archival and knowledge capture
taskwing archive                        # Archive completed work
taskwing retrospective                  # Generate project insights
```

## Documentation

- **[DOCS.md](DOCS.md)** - Complete user guide with examples and workflows
- **[MCP.md](MCP.md)** - AI integration setup and reference  
- **[CLAUDE.md](CLAUDE.md)** - Developer guide and architecture

## Architecture

```
CLI Commands ──► Task Store ──► Local Files (JSON/YAML/TOML)
     │                           │
     │                           ├─► Archive System
     │                           ├─► Pattern Library  
     │                           └─► Knowledge Base
     │
     └──► MCP Server ──► AI Tools (Claude, Cursor, etc.)
              │
              ├─► Historical Data Resources
              ├─► Pattern Suggestions
              └─► Task Generation
```

**Local-first**: Your data stays on your machine. No cloud dependencies.  
**AI-Enhanced**: Learns from your project history to provide better suggestions.

## Contributing

We welcome contributions! TaskWing is built with Go 1.24+ using Cobra CLI framework and the MCP SDK for AI integration.

See [CLAUDE.md](CLAUDE.md) for development setup and contributing guidelines.

## License

MIT License - see [LICENSE](LICENSE) file.

---

**Built for the terminal. Powered by AI. Made for developers.** 🚀
