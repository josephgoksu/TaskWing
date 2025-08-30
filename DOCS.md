# TaskWing User Guide

Comprehensive guide for using TaskWing CLI task manager with AI integration.

## Table of Contents

- [Installation](#installation)
- [Getting Started](#getting-started)
- [Commands Reference](#commands-reference)
- [Task Management](#task-management)
- [AI Integration](#ai-integration)
- [Configuration](#configuration)
- [Common Workflows](#common-workflows)
- [Troubleshooting](#troubleshooting)

## Installation

### Quick Install

```bash
# One-liner install (recommended)
curl -sSfL https://raw.githubusercontent.com/josephgoksu/TaskWing/main/install.sh | sh

# Via Go (requires Go 1.24+)
go install github.com/josephgoksu/TaskWing@latest
```

### Verify Installation

```bash
taskwing version
```

### PATH Setup

If `taskwing` is not found after installation, add to your PATH:

```bash
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.zshrc && source ~/.zshrc
```

## Getting Started

### New to TaskWing?

Start with the interactive guide:

```bash
taskwing quickstart
```

### Initialize Project

```bash
# Initialize in your project directory
taskwing init
```

This creates a `.taskwing/` directory with local task storage.

### Your First Task

```bash
# Interactive menu (great for exploration)
taskwing interactive

# Quick task creation
taskwing add "Fix authentication bug"

# Detailed task creation
taskwing add "Implement OAuth2" --priority high
```

## Commands Reference

### Getting Started Commands

| Command       | Purpose                           | Example                |
| ------------- | --------------------------------- | ---------------------- |
| `quickstart`  | Interactive getting started guide | `taskwing quickstart`  |
| `interactive` | Menu-driven interface             | `taskwing interactive` |
| `init`        | Initialize project                | `taskwing init`        |

### Core Task Commands

| Command  | Aliases          | Purpose              | Example                        |
| -------- | ---------------- | -------------------- | ------------------------------ |
| `add`    | `mk`, `create`   | Create a new task    | `taskwing add "Fix login bug"` |
| `list`   | `ls`             | List tasks           | `taskwing ls --status todo`    |
| `show`   | `get`, `view`    | Show task details    | `taskwing show <task-id>`      |
| `update` | `edit`, `modify` | Update existing task | `taskwing update <task-id>`    |
| `delete` | `rm`, `remove`   | Delete a task        | `taskwing delete <task-id>`    |

### Workflow Commands

| Command   | Aliases         | Purpose                    | Example                     |
| --------- | --------------- | -------------------------- | --------------------------- |
| `start`   | `begin`, `work` | Start working on task      | `taskwing start <task-id>`  |
| `review`  |                 | Move task to review status | `taskwing review <task-id>` |
| `done`    | `finish`        | Mark task complete         | `taskwing done <task-id>`   |
| `current` |                 | Manage current active task | `taskwing current`          |

### Discovery Commands

| Command  | Purpose                  | Example                        |
| -------- | ------------------------ | ------------------------------ |
| `search` | Search tasks by text     | `taskwing search "auth"`       |
| `next`   | Get AI task suggestions  | `taskwing next`                |
| `expand` | Break task into subtasks | `taskwing expand <task-id>`    |
| `clear`  | Clear completed tasks    | `taskwing clear --status done` |

### Configuration Commands

| Command  | Purpose              | Example           |
| -------- | -------------------- | ----------------- |
| `config` | Manage configuration | `taskwing config` |
| `reset`  | Reset project data   | `taskwing reset`  |

### AI Integration Commands

| Command | Purpose                 | Example        |
| ------- | ----------------------- | -------------- |
| `mcp`   | Start MCP server for AI | `taskwing mcp` |

### Utility Commands

| Command      | Purpose                    | Example                   |
| ------------ | -------------------------- | ------------------------- |
| `generate`   | Generate project artifacts | `taskwing generate`       |
| `completion` | Shell completion           | `taskwing completion zsh` |
| `version`    | Show version information   | `taskwing version`        |

## Task Management

### Task Properties

- **Title**: Short description (required)
- **Description**: Detailed explanation
- **Status**: `todo`, `doing`, `review`, `done`
- **Priority**: `low`, `medium`, `high`, `urgent`
- **Dependencies**: Other tasks that must complete first
- **Acceptance Criteria**: Definition of done

### Creating Tasks

```bash
# Simple task
taskwing add "Fix login issue"

# Task with details
taskwing add "Implement OAuth2" --priority high --description "Add Google and GitHub OAuth support"

# Task with dependencies
taskwing add "Deploy feature" --dependencies <task-id-1>,<task-id-2>
```

### Managing Task Status

```bash
# Workflow progression
taskwing start <task-id>    # todo -> doing
taskwing review <task-id>   # doing -> review
taskwing done <task-id>     # any -> done

# Direct status update
taskwing update <task-id> --status doing
```

### Filtering and Searching

```bash
# Filter by status
taskwing ls --status todo,doing

# Filter by priority
taskwing ls --priority high,urgent

# Search in titles/descriptions
taskwing search "authentication"

# Combined filtering
taskwing ls --status todo --priority high
```

### Working with Task IDs

TaskWing uses short UUID prefixes for convenience:

```bash
# Full: 7b3e4f2a-8c9d-4e5f-b0a1-2c3d4e5f6a7b
# Short: 7b3e4f2a

taskwing show 7b3e4f2a
taskwing done 7b3e4f2a
```

## AI Integration

### Quick Setup for Claude Code

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

3. Ask Claude: _"What tasks do I have?"_ or _"Create a task for the auth feature"_

### AI Capabilities

- **Task Management**: Create, read, update, delete tasks
- **Intelligent Suggestions**: Get next task recommendations
- **Project Planning**: Break down features into tasks
- **Context Awareness**: AI knows your current task
- **Bulk Operations**: Handle multiple tasks efficiently

See [MCP.md](MCP.md) for detailed AI integration setup.

## Configuration

### Configuration Files

TaskWing loads config in this order:

1. Command flags
2. Environment variables (`TASKWING_*`)
3. Project config (`.taskwing/.taskwing.yaml`)
4. Global config (`$HOME/.taskwing.yaml`)

### Environment Variables

```bash
export TASKWING_DATA_FORMAT="yaml"     # json, yaml, toml
export TASKWING_PROJECT_ROOTDIR="/custom/path"
```

### Example Config

Create `.taskwing/.taskwing.yaml`:

```yaml
project:
  rootDir: ".taskwing"
  tasksDir: "tasks"

data:
  file: "tasks.json"
  format: "json"
```

## Common Workflows

### Daily Development

```bash
# Morning: Check current work
taskwing current
taskwing ls --priority high,urgent

# Add urgent task
taskwing add "Fix production bug" --priority urgent

# Start work
taskwing start <task-id>

# Mark complete
taskwing done <task-id>
```

### Project Planning

```bash
# Create main tasks
taskwing add "Design API" --priority high
taskwing add "Implement endpoints" --dependencies <design-id>
taskwing add "Write tests" --dependencies <implement-id>

# View project structure
taskwing ls --sort created
```

### AI-Enhanced Planning

```bash
# Start MCP server
taskwing mcp

# Then ask Claude:
# "Break down this user story into development tasks"
# "What should I work on next based on dependencies?"
# "Create tasks from this GitHub issue URL"
```

## Troubleshooting

### Common Issues

#### Command Not Found

```bash
# Check installation
which taskwing

# Add to PATH
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.zshrc
source ~/.zshrc
```

#### Not Initialized

```bash
# Initialize project
taskwing init

# Verify setup
ls -la .taskwing/
```

#### Verbose Logging

```bash
taskwing --verbose list
taskwing mcp -v
```

### Getting Help

```bash
# Command help
taskwing --help
taskwing add --help

# Interactive guidance
taskwing quickstart
taskwing interactive
```

### Still Need Help?

- [GitHub Issues](https://github.com/josephgoksu/TaskWing/issues)
- [MCP Integration Guide](MCP.md)
- [AI Examples](EXAMPLES.md)
- [Developer Guide](CLAUDE.md)
