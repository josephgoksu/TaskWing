# TaskWing User Guide

Comprehensive guide for using TaskWing CLI task manager.

## Table of Contents

- [Installation](#installation)
- [Getting Started](#getting-started)
- [Commands Reference](#commands-reference)
- [Task Properties](#task-properties)
- [Configuration](#configuration)
- [Common Workflows](#common-workflows)
- [Automation & Scripting](#automation--scripting)
- [Troubleshooting](#troubleshooting)

## Installation

### Quick Install

```bash
# One-liner install (recommended)
curl -sSfL https://raw.githubusercontent.com/josephgoksu/taskwing.app/main/install.sh | sh

# Via Go (requires Go 1.24+)
go install github.com/josephgoksu/taskwing.app@latest
```

### Other Methods

- **Download Binary**: Get pre-built binaries from [GitHub Releases](https://github.com/josephgoksu/taskwing.app/releases)
- **Build from Source**:

  ```bash
  git clone https://github.com/josephgoksu/taskwing.app
  cd taskwing-app && go build -o taskwing main.go
  ```

- **Homebrew** (coming soon): `brew install josephgoksu/tap/taskwing`

### Verify Installation

```bash
taskwing --version
```

## Getting Started

### Initialize TaskWing

Run this in your project directory:

```bash
taskwing init
```

This creates a `.taskwing/` directory with your task storage. TaskWing works on a per-project basis.

### Your First Task

```bash
# Interactive mode (guided prompts)
taskwing add

# Non-interactive mode
taskwing add --title "Review pull requests" --priority high --description "Review open PRs for security issues"
```

### View Tasks

```bash
# List all tasks
taskwing list

# Filter by status
taskwing list --status pending

# Filter by priority
taskwing list --priority high,urgent

# Search tasks
taskwing search "review"
```

## Commands Reference

### Core Commands

| Command  | Purpose                                  | Example                            |
| -------- | ---------------------------------------- | ---------------------------------- |
| `init`   | Initialize TaskWing in current directory | `taskwing init`                    |
| `add`    | Create a new task                        | `taskwing add --title "Fix bug"`   |
| `list`   | Display tasks with optional filtering    | `taskwing list --status pending`   |
| `show`   | Show detailed task information           | `taskwing show <task-id>`          |
| `update` | Modify an existing task                  | `taskwing update <task-id>`        |
| `done`   | Mark task as completed                   | `taskwing done <task-id>`          |
| `delete` | Remove a task                            | `taskwing delete <task-id>`        |
| `search` | Search tasks by text                     | `taskwing search "authentication"` |

### Current Task Management

| Command         | Purpose                               | Example                          |
| --------------- | ------------------------------------- | -------------------------------- |
| `current set`   | Set the active task you're working on | `taskwing current set <task-id>` |
| `current show`  | Display current active task           | `taskwing current show`          |
| `current clear` | Clear current task                    | `taskwing current clear`         |

### Archive & Knowledge Management

| Command         | Purpose                                              | Example                  |
| --------------- | ---------------------------------------------------- | ------------------------ |
| `archive`       | Archive completed tasks and capture project insights | `taskwing archive`       |
| `retrospective` | Generate retrospective from completed tasks          | `taskwing retrospective` |

### Pattern Library

| Command                 | Purpose                                 | Example                                  |
| ----------------------- | --------------------------------------- | ---------------------------------------- |
| `patterns extract`      | Extract patterns from archived projects | `taskwing patterns extract`              |
| `patterns list`         | View all available patterns             | `taskwing patterns list`                 |
| `patterns match <desc>` | Find patterns matching description      | `taskwing patterns match "api refactor"` |
| `patterns update`       | Update patterns with latest archives    | `taskwing patterns update`               |

### Configuration Commands

| Command       | Purpose                       | Example                |
| ------------- | ----------------------------- | ---------------------- |
| `config show` | Display current configuration | `taskwing config show` |
| `config path` | Show config file location     | `taskwing config path` |

### MCP Integration

| Command  | Purpose                               | Example           |
| -------- | ------------------------------------- | ----------------- |
| `mcp`    | Start MCP server for AI integration   | `taskwing mcp`    |
| `mcp -v` | Start MCP server with verbose logging | `taskwing mcp -v` |

## Task Properties

| Property                | Description                       | Values                                                                                   |
| ----------------------- | --------------------------------- | ---------------------------------------------------------------------------------------- |
| **Title**               | Short descriptive name (required) | 3-255 characters                                                                         |
| **Description**         | Detailed explanation              | Any text                                                                                 |
| **Status**              | Current task state                | `pending`, `in-progress`, `completed`, `cancelled`, `on-hold`, `blocked`, `needs-review` |
| **Priority**            | Task urgency                      | `low`, `medium`, `high`, `urgent`                                                        |
| **Dependencies**        | Tasks that must complete first    | Array of task IDs                                                                        |
| **Parent/Subtasks**     | Hierarchical relationships        | Parent ID or subtask IDs                                                                 |
| **Acceptance Criteria** | Definition of done                | Any text                                                                                 |

### Creating Tasks

#### Interactive Mode

```bash
taskwing add
# Follow the prompts to enter task details
```

#### Non-Interactive Mode

```bash
# Basic task
taskwing add --title "Implement user login"

# Task with full details
taskwing add \
  --title "Implement user authentication" \
  --description "Add login and registration functionality" \
  --priority high \
  --acceptance-criteria "Users can log in, register, and reset passwords"
```

#### Tasks with Dependencies

```bash
# Add task that depends on others
taskwing add --title "Deploy to production" --dependencies "task-id-1,task-id-2"
```

### Managing Task Status

```bash
# Mark task as done
taskwing done <task-id>

# Update task status
taskwing update <task-id>  # Interactive mode
taskwing update <task-id> --status in-progress
```

### Filtering and Searching

```bash
# Filter by status
taskwing list --status pending,in-progress

# Filter by priority
taskwing list --priority high,urgent

# Sort tasks
taskwing list --sort-by priority
taskwing list --sort-by created-at --sort-order desc

# Search in titles and descriptions
taskwing search "authentication"
taskwing search "bug fix"

# Combine filters
taskwing list --status pending --priority high --search "api"
```

### Working with Task IDs

TaskWing uses short UUID prefixes for convenience:

```bash
# Full UUID: 7b3e4f2a-8c9d-4e5f-b0a1-2c3d4e5f6a7b
# Short form: 7b3e4f2a

taskwing show 7b3e4f2a
taskwing done 7b3e4f2a
```

## Configuration

### Configuration Hierarchy

TaskWing loads configuration in this order (highest priority first):

1. Command line flags
2. Environment variables (`TASKWING_*` prefix)
3. Project config (`.taskwing/.taskwing.yaml`)
4. Directory config (`./.taskwing.yaml`)
5. Global config (`$HOME/.taskwing.yaml`)
6. Built-in defaults

### Environment Variables

```bash
export TASKWING_PROJECT_ROOTDIR="/custom/path/.taskwing"
export TASKWING_DATA_FORMAT="yaml"  # json, yaml, or toml
export TASKWING_PROJECT_TASKSDIR="my-tasks"
```

### Config File Example

Create `.taskwing/.taskwing.yaml`:

```yaml
project:
  rootDir: ".taskwing"
  tasksDir: "tasks"

data:
  file: "tasks.json"
  format: "json" # json, yaml, or toml
```

## Common Workflows

### Daily Task Management

```bash
# Morning: Check high-priority tasks
taskwing list --priority high,urgent

# Add urgent task
taskwing add --title "Fix production issue" --priority urgent

# Set current task
taskwing current set <task-id>

# Throughout day: Update progress
taskwing update <task-id> --status in-progress
taskwing done <task-id>

# Evening: Review remaining tasks
taskwing list --status pending
```

### Project Planning

```bash
# Create main project tasks
taskwing add --title "Design API architecture" --priority high
taskwing add --title "Implement core endpoints" --dependencies <design-task-id>
taskwing add --title "Write integration tests" --dependencies <implement-task-id>
taskwing add --title "Deploy to staging" --dependencies <tests-task-id>

# View project structure
taskwing list --sort-by created-at
```

### Sprint Management

```bash
# Plan sprint tasks
taskwing list --status pending --priority high
taskwing add --title "Sprint goal: User management features"

# Track progress
taskwing list --status in-progress
taskwing current show

# Sprint review
taskwing list --status completed --since "1 week ago"
```

### Bug Tracking

```bash
# Add bug report
taskwing add \
  --title "Login fails with OAuth" \
  --description "Users cannot log in using Google OAuth" \
  --priority urgent \
  --acceptance-criteria "OAuth login works for all providers"

# Track investigation
taskwing update <bug-id> --status in-progress
taskwing current set <bug-id>

# Close when fixed
taskwing done <bug-id>
```

### Project Completion & Knowledge Capture

```bash
# When finishing a project, capture insights for future use
taskwing archive                     # Archive completed tasks
taskwing retrospective              # Generate project retrospective

# Extract patterns for AI assistance
taskwing patterns extract           # Build pattern library
taskwing patterns list             # Review available patterns
```

### AI-Enhanced Task Planning

```bash
# Use patterns to plan similar work
taskwing patterns match "documentation cleanup"
taskwing patterns match "api refactoring"

# Let AI suggest patterns via MCP (requires Claude Code/Cursor)
# Use suggest-patterns tool through your AI assistant
```

### Automated Planning from a PRD (non-interactive)

```bash
# Preview only (no changes)
taskwing generate tasks --file product.xml --preview-only --no-improve

# Create tasks without prompts (clears existing tasks if any)
taskwing generate tasks --file product.xml --yes --create
```

Flags:

- `--yes`: accept confirmations (overwrite existing, create)
- `--no-improve`: skip PRD improvement
- `--preview-only`: show proposed tasks and exit
- `--create`: create tasks without interactive confirmation

### Advanced Pattern Workflows

```bash
# Keep pattern library current
taskwing patterns update            # Refresh with latest archives

# View pattern details for planning
taskwing patterns list              # Show all patterns with metrics

# Apply patterns manually
taskwing patterns match "system implementation"
# Follow suggested task breakdown from pattern
```

## Automation & Scripting

### JSON Output

All commands support `--json` flag for machine-readable output:

```bash
taskwing list --json
taskwing show <task-id> --json
```

### Script Integration

```bash
#!/bin/bash
# Batch task creation
while IFS= read -r line; do
  taskwing add --title "$line" --priority medium
done < task-list.txt

# CI/CD integration
if [ "$CI_PIPELINE_STATUS" = "failed" ]; then
  taskwing add --title "Fix CI failure" --priority high
fi
```

### Data Export

```bash
# Backup tasks
taskwing list --json > backup-$(date +%Y%m%d).json
```

**Note**: For advanced automation with AI tools, see [MCP Integration Guide](MCP.md).

## Troubleshooting

### Common Issues

#### TaskWing Not Found

```bash
# Check if installed
which taskwing

# Check Go bin path
echo $GOPATH/bin
ls $GOPATH/bin/taskwing

# Add to PATH if needed
export PATH=$PATH:$(go env GOPATH)/bin
```

#### No Tasks Directory

```bash
# Ensure TaskWing is initialized
taskwing init

# Check if directory exists
ls -la .taskwing/
```

#### Permission Issues

```bash
# Check directory permissions
ls -la .taskwing/
chmod 755 .taskwing/
chmod 644 .taskwing/tasks/tasks.json
```

#### Config File Not Found

```bash
# Show config location
taskwing config path

# Show current config
taskwing config show

# Create minimal config
echo "project:\n  rootDir: .taskwing" > .taskwing/.taskwing.yaml
```

### Verbose Output

Use `--verbose` flag for detailed logging:

```bash
taskwing --verbose list
taskwing --verbose add --title "Debug task"
```

### Data File Issues

#### Corrupted Task File

```bash
# Backup current file
cp .taskwing/tasks/tasks.json .taskwing/tasks/tasks.json.backup

# Initialize new file
rm .taskwing/tasks/tasks.json
taskwing init
```

#### Multiple Projects

Each project directory needs its own initialization:

```bash
cd /project1
taskwing init

cd /project2
taskwing init
```

### Getting Help

```bash
# General help
taskwing --help

# Command-specific help
taskwing add --help
taskwing list --help

# Version information
taskwing --version
```

### Still Need Help?

1. Check the [GitHub Issues](https://github.com/josephgoksu/taskwing.app/issues)
2. Review [CLAUDE.md](CLAUDE.md) for development details
3. For AI integration issues, see [MCP.md](MCP.md)
