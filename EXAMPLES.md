# TaskWing AI Interactions Examples

Here are common interactions with AI tools (Claude Code, Cursor, etc.) when using TaskWing via the Model Context Protocol (MCP):

## Starting a New Project

```
I've just initialized a new project with TaskWing. I have a project requirements document at docs/requirements.md.
Can you help me parse it and set up the initial tasks?
```

```
Create a project plan from my PRD document and break it down into manageable tasks with dependencies.
```

## Task Discovery and Planning

```
What's the next task I should work on? Please consider dependencies and priorities.
```

```
Can you analyze my current tasks and suggest what I should focus on today?
```

```
Show me all high-priority tasks that are ready to start (no blocking dependencies).
```

## Working with Specific Tasks

```
I'd like to work on task abc123. Can you show me the details and any dependencies?
```

```
Set task xyz789 as my current active task and show me what needs to be done.
```

```
I'm implementing the user authentication system. Can you find the related task and set it as current?
```

## Viewing Multiple Tasks

```
Can you show me tasks related to the API implementation so I can understand their relationships?
```

```
Show me all tasks that depend on the database schema task.
```

```
Display all subtasks under the "Frontend Implementation" parent task.
```

## Managing Task Status and Progress

```
I've finished implementing the login functionality. Mark the current task as done and suggest what to work on next.
```

```
Move the authentication task to review status - it's ready for code review.
```

```
I'm starting work on the payment integration task. Update its status to 'doing'.
```

## Creating and Managing Tasks

```
Add a new urgent task: "Fix memory leak in payment processor" with acceptance criteria for performance benchmarks.
```

```
Create a task for implementing user profile pictures with dependencies on the user management system.
```

```
I need to add several tasks for the mobile app feature. Can you create them with proper priorities and dependencies?
```

## Task Relationships and Dependencies

```
The API authentication task should be completed before the frontend login task. Can you set up this dependency?
```

```
Check if there are any circular dependencies in my current task graph and suggest fixes.
```

## Bulk Operations and Organization

```
Mark all completed tasks as done and archive them.
```

```
Show me all tasks that have been in 'doing' status for more than a week.
```

```
Update all API-related tasks to high priority since we're prioritizing backend work this sprint.
```

## Board View and Project Overview

```
Show me a kanban-style view of all my tasks organized by status.
```

```
Give me a project summary with task counts by status and priority.
```

```
What's the overall progress on this project? Show me completion statistics.
```

## Search and Filtering

```
Find all tasks related to "authentication" or "login" functionality.
```

```
Show me all urgent tasks that are currently blocked by dependencies.
```

```
List all tasks assigned to the database implementation with their current status.
```

## Smart Task Management

```
Suggest which tasks I should tackle next based on my current progress and dependencies.
```

```
Analyze my task completion patterns and recommend optimizations.
```

```
What are the critical path tasks that could block other work if delayed?
```

## Planning and Documentation

```
Generate a project plan from my requirements document and create tasks automatically.
```

```
Create a development roadmap from my current tasks showing the logical sequence.
```

```
Export my current task list as a markdown report for stakeholder review.
```

## Integration Workflows

```
I'm about to start a new sprint. Can you help me select and prioritize tasks for the next two weeks?
```

```
We're having a planning meeting. Show me all unestimated tasks so we can size them.
```

```
I just merged a feature branch. Mark all related tasks as complete and suggest the next feature to work on.
```

## Problem Solving and Debugging

```
I'm stuck on the WebSocket implementation task. Can you show me the task details and suggest breaking it into smaller subtasks?
```

```
This task is taking longer than expected. Help me break it down into more manageable pieces.
```

```
I need to pivot our approach for the caching system. Update the related tasks to reflect using Redis instead of in-memory caching.
```

## Current Task Context

```
What am I currently working on? Show me my active task and its progress.
```

```
Clear my current task - I need to switch to something more urgent.
```

```
Set the database migration task as my current focus and show me what's required.
```

## Advanced Task Operations

```
Batch create tasks for implementing the user dashboard: user profile, settings, activity log, and notifications.
```

```
Find and resolve any dependency conflicts in my task graph.
```

```
Create a task template for bug fixes that includes standard acceptance criteria.
```

## AI-Powered Insights

```
Analyze my task completion velocity and predict when this project will be finished.
```

```
Suggest which tasks might be over-scoped based on their descriptions.
```

```
Recommend task groupings for more efficient development workflow.
```

## Getting Started with AI Integration

To use these examples:

1. **Start the MCP server**: Run `taskwing mcp` in your project directory
2. **Connect your AI tool**: Configure Claude Code, Cursor, or your preferred AI assistant to use TaskWing's MCP server
3. **Natural interaction**: Use natural language - the AI will translate your requests into the appropriate TaskWing MCP tool calls

## Available MCP Tools

TaskWing provides 30+ MCP tools including:

- **Basic Operations**: add-task, get-task, update-task, delete-task, mark-done
- **Bulk Operations**: batch-create-tasks, bulk-tasks, bulk-by-filter, clear-tasks
- **Search & Filter**: list-tasks, search-tasks, query-tasks, filter-tasks, find-task
- **Board Management**: board-snapshot, board-reconcile
- **Context Management**: set/get/clear-current-task, task-summary
- **Smart Features**: suggest-tasks, smart-task-transition, dependency-health
- **Planning**: plan-from-document, workflow-status
- **Analytics**: task-analytics, extract-task-ids

The AI tools handle the complexity - you just describe what you want to accomplish!
