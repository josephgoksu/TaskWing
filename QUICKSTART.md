# TaskWing Quick Start Guide 🚀

Get up and running with TaskWing in 5 minutes!

## Installation

Choose your preferred method:

```bash
# Option 1: Go install (recommended for developers)
go install github.com/josephgoksu/taskwing.app@latest

# Option 2: Download binary from releases
# Visit https://github.com/josephgoksu/taskwing.app/releases

# Option 3: Build from source
git clone https://github.com/josephgoksu/taskwing.app
cd taskwing-app
go build -o taskwing main.go
```

## First Steps

### 1. Initialize TaskWing

```bash
# Initialize in your project directory
taskwing init
```

This creates a `.taskwing/` directory with your task storage.

### 2. Add Your First Task

```bash
# Interactive mode (guided prompts)
taskwing add

# Non-interactive mode (automation-friendly)
taskwing add --title "Review pull requests" --priority high
```

### 3. View Your Tasks

```bash
# List all tasks
taskwing list

# Search tasks
taskwing search "review"

# Filter by status/priority
taskwing list --status pending --priority high
```

### 4. Manage Tasks

```bash
# Mark task as done
taskwing done <task-id>

# Update a task
taskwing update <task-id>

# Get detailed info
taskwing show <task-id>

# Delete a task
taskwing delete <task-id>
```

## Common Workflows

### Daily Task Management

```bash
# Morning: Check today's priorities
taskwing list --priority high,urgent

# Add a quick task
taskwing add --title "Fix critical bug" --priority urgent

# Throughout the day: Mark tasks complete
taskwing done <task-id>

# Evening: Review what's left
taskwing list --status pending
```

### Project Planning

```bash
# Add project tasks with dependencies
taskwing add --title "Design API" --priority high
taskwing add --title "Implement API" --dependencies <design-task-id>
taskwing add --title "Write tests" --dependencies <implement-task-id>

# View project structure
taskwing list
```

### Team Collaboration

```bash
# Export tasks for sharing
taskwing list --json > team-tasks.json

# Add tasks in scripts/CI
taskwing add --non-interactive --title "Deploy to staging" --priority medium
```

## AI Integration (MCP)

Connect TaskWing to Claude or other AI tools:

```bash
# Start MCP server
taskwing mcp

# Use with Claude Code or other MCP clients
# The AI can now create, update, and manage your tasks!
```

## JSON Output for Automation

```bash
# Get JSON output for scripts
taskwing list --json
taskwing show <task-id> --json

# Pipe to other tools
taskwing list --json | jq '.tasks[] | select(.priority == "urgent")'
```

## Configuration

TaskWing works out of the box, but you can customize:

```bash
# View current config
taskwing config show

# Set data directory
taskwing config data_dir ~/.my-tasks

# View config file location
taskwing config path
```

## Tips & Tricks

**🔥 Power User Tips:**

- Use short task IDs: Just type the first few characters
- Chain commands: `taskwing add --title "Fix bug" && taskwing list`
- Search everything: `taskwing search` looks in titles, descriptions, and IDs
- JSON everywhere: Add `--json` to any command for automation
- Non-interactive mode: Perfect for scripts and CI/CD

**🤖 AI Integration:**

- Start `taskwing mcp` and connect to Claude Code
- Let AI help you break down complex projects
- Use AI to prioritize and organize tasks
- Generate tasks from meeting notes or emails

**📁 Organization:**

- Use project-specific `.taskwing/` directories
- Set up different priorities for different types of work
- Use dependencies to model project workflows
- Export tasks to JSON for backup or sharing

## Need Help?

```bash
# Command help
taskwing --help
taskwing add --help

# Verbose output for debugging
taskwing --verbose list

# Check configuration
taskwing config show
```

**Ready to be productive? Start with:**

```bash
taskwing init
taskwing add
```

That's it! You're now ready to manage tasks like a pro with TaskWing! 🎯
