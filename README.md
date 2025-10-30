# TaskWing

**AI-powered CLI task manager built for developers**

[![Go Report Card](https://goreportcard.com/badge/github.com/josephgoksu/TaskWing)](https://goreportcard.com/report/github.com/josephgoksu/TaskWing)
[![GitHub release](https://img.shields.io/github/release/josephgoksu/TaskWing.svg)](https://github.com/josephgoksu/TaskWing/releases)
[![Go](https://img.shields.io/badge/Go-1.24+-00ADD8.svg?logo=go&logoColor=white)](https://golang.org)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![MCP Compatible](https://img.shields.io/badge/MCP-Compatible-9C59D1.svg?logo=anthropic&logoColor=white)](https://modelcontextprotocol.io)

[![GitHub issues](https://img.shields.io/github/issues/josephgoksu/TaskWing)](https://github.com/josephgoksu/TaskWing/issues)
[![GitHub pull requests](https://img.shields.io/github/issues-pr/josephgoksu/TaskWing)](https://github.com/josephgoksu/TaskWing/pulls)
[![Platform Support](https://img.shields.io/badge/platform-macOS%20%7C%20Linux%20%7C%20Windows-lightgrey)](https://github.com/josephgoksu/TaskWing/releases)

TaskWing integrates directly with Claude Code, Cursor, and other AI tools via the Model Context Protocol. Manage tasks from your terminal or let AI handle it for you.

## Why TaskWing?

- **🤖 AI-Native**: Full MCP integration for Claude Code, Cursor, and other AI tools
- **⚡ Zero Config**: Works immediately, stores data locally in your project
- **🔗 Smart Dependencies**: Automatic dependency tracking and circular reference prevention
- **📚 Learning System**: Captures patterns from completed projects for future AI assistance
- **🚀 Developer-First**: Built for developers who value focus and efficiency

## By [@josephgoksu](https://x.com/josephgoksu)

[![Twitter Follow](https://img.shields.io/twitter/follow/josephgoksu)](https://x.com/josephgoksu)

## Quick Start

### Installation

#### One-liner install (recommended)

```bash
curl -sSfL https://raw.githubusercontent.com/josephgoksu/TaskWing/main/install.sh | sh
```

> **Note**: After installation, you may need to add `~/.local/bin` to your PATH:
>
> ```bash
> echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.zshrc && source ~/.zshrc
> ```

#### Alternative: Go install

If you have Go installed:

```bash
go install github.com/josephgoksu/TaskWing@latest
```

#### Verify installation

```bash
taskwing version
```

### First Steps

**New to TaskWing?** Start with the interactive guide:

```bash
taskwing quickstart
```

**Or jump straight in:**

```bash
# Initialize in your project
taskwing init

# Interactive menu (great for beginners)
taskwing interactive

# Or use direct commands
taskwing add "Fix authentication bug" --priority urgent
taskwing ls                    # List all tasks
taskwing start <task-id>       # Begin working
taskwing done <task-id>        # Mark complete
taskwing improve <task-id>     # AI‑enhance task details; add --plan to create subtasks
```

### AI Integration

**Quick setup for Claude Code:**

1. Start MCP server:

   ```bash
   taskwing mcp
   ```

2. Add to Claude Code config:

   ```json
   {
     "mcpServers": {
       "taskwing": {
         "command": "taskwing",
         "args": ["mcp"]
       }
     }
   }
   ```

3. Ask Claude: _"What tasks do I have?"_ or _"Create a task to refactor the auth module"_

**Example AI interactions:**

- _"Break down this feature into tasks"_
- _"What should I work on next?"_
- _"Create tasks from this GitHub issue"_
- _"Show me my current sprint status"_

See [MCP Setup Guide](MCP.md) for detailed configuration and [AI Examples](EXAMPLES.md) for advanced patterns.

## Core Features

### 🎯 Task Management

- **Rich metadata**: Priority, status, dependencies, acceptance criteria
- **Flexible workflow**: `todo` → `doing` → `review` → `done`
- **Smart search**: Find tasks by title, description, or partial ID
- **Board view**: Kanban-style project visualization

### 🤖 AI Integration

- **MCP Protocol**: Direct integration with Claude Code, Cursor, and AI tools
- **Context-aware**: AI knows your current task and project state
- **Intelligent suggestions**: Get next task recommendations
- **Planning assistance**: Break down features into actionable tasks

### 🔗 Smart Dependencies

- **Relationship tracking**: Parent/child tasks and dependencies
- **Circular prevention**: Automatic detection and prevention
- **Dependency health**: Analyze and fix broken relationships

### ⚡ Developer Experience

- **Zero config**: Works immediately in any project
- **Local-first**: All data stored in your project directory
- **Fast commands**: Optimized for daily use with aliases
- **Interactive mode**: Menu-driven interface for exploration

**Example workflows:**

```bash
# Sprint planning
taskwing add "Implement user auth" --priority high
taskwing add "Add login form" --parent <auth-task-id>
taskwing add "Add logout functionality" --parent <auth-task-id>
taskwing ls --status todo,doing --tree   # See the plan in CLI

# Daily workflow
taskwing current                        # See active task
taskwing next                          # Get AI suggestions
taskwing start <task-id>               # Focus on task
taskwing done <task-id>                # Mark complete
taskwing improve <task-id> --apply     # Refine title/desc/criteria/priority

# Project analysis
taskwing search "auth"                 # Find auth-related tasks
taskwing ls --status done --json       # Export completion metrics
taskwing flow                          # Guided status and next steps

# Need richer dashboards? Use MCP tools via Claude/Cursor:
# board-snapshot (MCP)   → Kanban-style summary
# analytics (MCP)        → Completion metrics
# workflow-status (MCP)  → Project phase overview
```

For complete command reference, run `taskwing --help` or `taskwing <command> --help`.

## Real-World Examples

### Daily Development Workflow

```bash
# Morning standup
taskwing current                    # What am I working on?
taskwing ls --status todo,doing --tree  # Sprint overview in CLI

# Start new work
taskwing add "Implement OAuth2 flow" --priority high
taskwing start <task-id>           # Focus mode

# AI assistance
# Ask Claude: "Break down this OAuth task into smaller steps"
# Claude creates subtasks automatically via MCP

# End of day
taskwing done <task-id>            # Mark complete
taskwing next                      # What's next?
```

### Project Planning with AI

```bash
# Upload PRD to Claude and ask:
# "Create a task breakdown for this feature spec"

# Claude uses MCP tools to:
# - batch-create-tasks (MCP)   → Create multiple tasks at once
# - board-reconcile (MCP)      → Organize dependencies
# - workflow-status (MCP)      → Summarize project phases
# - improve (CLI/MCP)          → Refine task details (AI assist)
```

## Documentation

| Document                                           | Purpose                                         |
| -------------------------------------------------- | ----------------------------------------------- |
| **CLI Help (`taskwing --help`)**                   | Built-in user guide for installation & commands |
| **[MCP.md](MCP.md)**                               | AI integration - Setup and tool reference       |
| **[EXAMPLES.md](EXAMPLES.md)**                     | AI interaction examples - Common usage patterns |
| **[CLAUDE.md](CLAUDE.md)**                         | Developer guide - Architecture and contributing |
| **[docs/ARCHIVE_DESIGN.md](docs/ARCHIVE_DESIGN.md)** | Archive system design notes                     |

## For Developers

### Quick Development Setup

```bash
# Clone and setup
git clone https://github.com/josephgoksu/TaskWing.git
cd TaskWing
make dev-setup                 # Install dev tools

# Development workflow
make build                     # Build binary
make test-quick               # Fast tests
make lint                     # Format and lint

# Testing
make test-all                 # Comprehensive test suite
make test-mcp                 # Test MCP integration
make coverage                 # Generate coverage report
```

### Architecture

- **Local-first**: All data in project `.taskwing/` directory
- **Go 1.24+**: Built with Cobra CLI framework
- **MCP Integration**: Full Model Context Protocol support
- **File-based storage**: JSON/YAML/TOML with file locking
- **33+ MCP tools**: Comprehensive AI integration

### Contributing

We welcome contributions! Key areas:

- 🐛 **Bug fixes**: See [issues](https://github.com/josephgoksu/TaskWing/issues)
- ✨ **New MCP tools**: Extend AI capabilities
- 📄 **Documentation**: Improve user experience
- 🧪 **Testing**: Increase coverage and reliability

See [CLAUDE.md](CLAUDE.md) for detailed development guide.

## License

MIT License - see [LICENSE](LICENSE) file.

---

[![Star History Chart](https://api.star-history.com/svg?repos=josephgoksu/TaskWing&type=Date)](https://www.star-history.com/#josephgoksu/TaskWing&Date)

**Built for the terminal. Powered by AI. Made for developers.** 🚀
