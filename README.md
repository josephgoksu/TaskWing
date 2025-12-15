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

See [MCP Setup Guide](MCP.md) for detailed configuration.

## Core Features

### 🎯 Task Management

- **Rich metadata**: Priority, status, dependencies, acceptance criteria
- **Flexible workflow**: `todo` → `doing` → `review` → `done`
- **Smart search**: Find tasks by title, description, or partial ID
- **Hierarchical tasks**: Parent/child relationships with subtasks

### 🤖 AI Integration

- **MCP Protocol**: Direct integration with Claude Code, Cursor, and AI tools
- **Context-aware**: AI knows your current task and project state
- **Intelligent suggestions**: Get next task recommendations via `taskwing next`
- **Task enhancement**: AI improves task titles, descriptions, and acceptance criteria

### 🔗 Smart Dependencies

- **Relationship tracking**: Parent/child tasks and dependencies
- **Circular prevention**: Automatic detection and prevention
- **Dependency validation**: Tasks blocked until dependencies complete

### ⚡ Developer Experience

- **Zero config**: Works immediately in any project
- **Local-first**: All data stored in your project directory
- **Fast commands**: Optimized for daily use with aliases
- **Interactive mode**: Menu-driven interface for exploration

## Commands Reference

| Command | Description |
|---------|-------------|
| `taskwing init` | Initialize TaskWing in current directory |
| `taskwing add "title"` | Add a new task (AI-enhanced by default) |
| `taskwing ls` | List all tasks |
| `taskwing show <id>` | Show task details |
| `taskwing start <id>` | Start working on a task |
| `taskwing done <id>` | Mark task as complete |
| `taskwing update <id>` | Update task properties |
| `taskwing delete <id>` | Delete a task |
| `taskwing search "query"` | Search tasks |
| `taskwing next` | Get AI-powered next task suggestion |
| `taskwing current` | Show current active task |
| `taskwing interactive` | Launch interactive menu |
| `taskwing mcp` | Start MCP server for AI tools |
| `taskwing config` | View/edit configuration |
| `taskwing reset` | Reset TaskWing data |
| `taskwing clear` | Clear completed tasks |

## Example Workflows

### Daily Development

```bash
# Morning: Check status
taskwing current               # What am I working on?
taskwing ls --status=doing     # All in-progress tasks

# Start new work
taskwing add "Implement OAuth2 flow" --priority high
taskwing start <task-id>

# End of day
taskwing done <task-id>
taskwing next                  # What's next?
```

### Sprint Planning

```bash
# Create parent task
taskwing add "Implement user auth" --priority high

# Add subtasks
taskwing add "Add login form" --parentID <auth-task-id>
taskwing add "Add logout functionality" --parentID <auth-task-id>

# Review hierarchy
taskwing ls --parent <auth-task-id>
```

## For Developers

### Quick Development Setup

```bash
git clone https://github.com/josephgoksu/TaskWing.git
cd TaskWing
make dev-setup    # Install dev tools
make build        # Build binary
make test-quick   # Fast tests
make lint         # Format and lint
```

### Architecture

- **Local-first**: All data in project `.taskwing/` directory
- **Go 1.24+**: Built with Cobra CLI framework
- **MCP Integration**: Full Model Context Protocol support
- **File-based storage**: JSON/YAML/TOML with file locking

### Contributing

We welcome contributions! Key areas:

- 🐛 **Bug fixes**: See [issues](https://github.com/josephgoksu/TaskWing/issues)
- ✨ **New MCP tools**: Extend AI capabilities
- 📄 **Documentation**: Improve user experience
- 🧪 **Testing**: Increase coverage and reliability

## License

MIT License - see [LICENSE](LICENSE) file.

---

**Built for the terminal. Powered by AI. Made for developers.** 🚀
