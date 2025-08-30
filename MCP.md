# TaskWing MCP Integration Guide

Complete guide for integrating TaskWing with AI assistants via Model Context Protocol (MCP).

## Quick Setup

### 1. Start MCP Server

```bash
# Start server (communicates via stdin/stdout)
taskwing mcp

# With verbose logging for debugging
taskwing mcp -v
```

### 2. Configure Claude Code

Add to your Claude Code configuration:

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

### 3. Test Integration

Ask Claude:

- _"What tasks do I have?"_
- _"Create a task to implement user authentication"_
- _"What should I work on next?"_

## AI Tool Configuration

### System Prompt

Use this prompt to ensure AI tools leverage TaskWing correctly:

```
Use TaskWing MCP tools for all task management. Do not create separate lists.

WORKFLOW:
- First: Call task-summary, then get-current-task
- Status: todo|doing|review|done
- Priority: low|medium|high|urgent
- Create: add-task (single), batch-create-tasks (multiple)
- Find: find-task or query-tasks for searching
- Update: update-task for fields, mark-done to complete
- Bulk: bulk-tasks or clear-tasks for batch operations

CONTEXT MANAGEMENT:
- Use set-current-task to track active work
- Always check current task for context-aware responses
- Include project status in responses using task-summary

TASK CREATION:
- Use batch-create-tasks for multiple related tasks
- Set dependencies between tasks when needed
- Include acceptance criteria for clarity
```

### Cursor Configuration

For Cursor IDE, add to `.cursor/mcp.json`:

```json
{
  "servers": {
    "taskwing": {
      "command": "taskwing",
      "args": ["mcp"]
    }
  }
}
```

## Available MCP Tools

### Core Task Management

- `add-task` - Create single task
- `batch-create-tasks` - Create multiple tasks with dependencies
- `get-task` - Get task details
- `update-task` - Modify task properties
- `delete-task` - Remove task
- `mark-done` - Complete task

### Task Discovery

- `list-tasks` - List with filtering
- `search-tasks` - Full-text search
- `query-tasks` - Natural language queries
- `find-task` - Find by ID/title fragment
- `filter-tasks` - Advanced filtering

### Context Management

- `set-current-task` - Set active task
- `get-current-task` - Get current task
- `clear-current-task` - Clear active task
- `task-summary` - Project overview

### Intelligent Features

- `suggest-tasks` - AI task recommendations
- `smart-task-transition` - Next steps guidance
- `dependency-health` - Dependency analysis

### Bulk Operations

- `bulk-tasks` - Batch operations on multiple tasks
- `bulk-by-filter` - Bulk operations with filtering
- `clear-tasks` - Clean up completed tasks

### Project Management

- `board-snapshot` - Kanban view
- `board-reconcile` - Apply multiple operations
- `workflow-status` - Project lifecycle view
- `task-analytics` - Completion metrics

## Common AI Interactions

### Project Planning

```
User: I'm starting a new REST API project. Can you help me plan the tasks?

AI: I'll help you plan your REST API project. Let me first check your current task status.

[Uses task-summary and batch-create-tasks to create:]
- Design API architecture (high priority)
- Set up project structure (depends on architecture)
- Implement authentication (high priority)
- Create core endpoints (depends on auth)
- Write API tests (depends on endpoints)
- Set up CI/CD pipeline
- Write documentation
```

### Daily Workflow

```
User: What should I work on today?

AI: [Uses get-current-task and suggest-tasks]

Based on your current project status, here are your top priorities:

1. **Continue**: "Implement OAuth2 authentication" (currently in progress)
2. **Next**: "Set up database models" (high priority, no blockers)
3. **Ready**: "Write user endpoint tests" (depends on completed auth task)

Would you like me to update your current task focus?
```

### Task Breakdown

```
User: Break down "Implement user management" into smaller tasks

AI: [Uses expand or batch-create-tasks to create subtasks:]
- Create user model and validation
- Implement user registration endpoint
- Add user authentication middleware
- Create user profile endpoints
- Add password reset functionality
- Write user management tests
```

### Progress Tracking

```
User: How is the project going?

AI: [Uses task-analytics and workflow-status]

**Project Status:**
- **Completed**: 8 tasks (67% of current sprint)
- **In Progress**: 2 tasks
- **Blocked**: 1 task (waiting for API keys)
- **Ready**: 3 tasks

**This Week:** Completed authentication system and core endpoints. On track for Friday delivery.
```

## Best Practices

### For AI Assistants

1. **Always check context first**: Use `task-summary` and `get-current-task`
2. **Use batch operations**: Create related tasks together with `batch-create-tasks`
3. **Set dependencies**: Link tasks properly for project flow
4. **Update current task**: Keep context accurate with `set-current-task`
5. **Provide summaries**: Include project status in responses

### For Users

1. **Keep MCP running**: Start `taskwing mcp` in a dedicated terminal
2. **Use descriptive titles**: Help AI understand task context
3. **Set current task**: Use `taskwing start <id>` or ask AI to set it
4. **Regular check-ins**: Ask "What should I work on?" daily
5. **Provide context**: Share PRDs, requirements, or goals with AI

## Testing MCP Integration

### Manual Testing

```bash
# Test server starts
taskwing mcp -v

# In another terminal, test basic commands
taskwing add "Test task"
taskwing list
```

### With AI Assistant

Test these interactions:

1. "Show me my current tasks"
2. "Create a task to refactor the authentication module"
3. "What's my current task?"
4. "Mark task X as complete"
5. "What should I work on next?"

## Troubleshooting

### Common Issues

#### MCP Server Won't Start

```bash
# Check TaskWing works normally
taskwing --help
taskwing init
taskwing add "Test"

# Try verbose mode
taskwing mcp -v
```

#### AI Can't See Tasks

1. Verify MCP server is running: `taskwing mcp`
2. Check AI tool configuration
3. Restart AI tool after config changes
4. Test with: "Call task-summary tool"

#### Task Context Lost

```bash
# Manually set current task
taskwing start <task-id>

# Or ask AI: "Set my current task to X"
```

#### Slow Responses

- MCP server handles each tool call synchronously
- Large task lists may slow responses
- Use `clear-tasks` to archive completed work

### Getting Help

- **Configuration**: Check [DOCS.md](DOCS.md#configuration)
- **Commands**: Run `taskwing --help` or `taskwing <command> --help`
- **Issues**: [GitHub Issues](https://github.com/josephgoksu/TaskWing/issues)
- **Examples**: See [EXAMPLES.md](EXAMPLES.md) for more AI interaction patterns
