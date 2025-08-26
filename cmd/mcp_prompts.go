/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/josephgoksu/TaskWing/store"
	"github.com/josephgoksu/TaskWing/types"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// taskGenerationPromptHandler generates tasks from natural language descriptions
func taskGenerationPromptHandler(taskStore store.TaskStore) func(context.Context, *mcp.ServerSession, *mcp.GetPromptParams) (*mcp.GetPromptResult, error) {
	return func(ctx context.Context, ss *mcp.ServerSession, params *mcp.GetPromptParams) (*mcp.GetPromptResult, error) {
		// Get the description argument
		description := params.Arguments["description"]

		if strings.TrimSpace(description) == "" {
			return nil, fmt.Errorf("description argument is required")
		}

		// Get existing tasks for context
		tasks, err := taskStore.ListTasks(nil, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to get existing tasks: %w", err)
		}

		// Create context about existing tasks
		var existingTasksContext string
		if len(tasks) > 0 {
			existingTasksContext = fmt.Sprintf("\n\nExisting tasks in the system (%d tasks):\n", len(tasks))
			for _, task := range tasks {
				existingTasksContext += fmt.Sprintf("- %s (ID: %s, Status: %s, Priority: %s)\n",
					task.Title, task.ID, task.Status, task.Priority)
			}
		} else {
			existingTasksContext = "\n\nNo existing tasks in the system."
		}

		// Generate the prompt
		prompt := fmt.Sprintf(`You are a task management assistant helping to break down work into manageable tasks.

User's Request: %s

IMPORTANT: After providing your analysis, you should actually CREATE the tasks using the TaskWing MCP tools available to you.

You have access to these TaskWing MCP tools:
- add-task: Create a new task with title, description, acceptanceCriteria, priority, and dependencies
- batch-create-tasks: Create multiple tasks at once with automatic dependency resolution (RECOMMENDED for task generation)
- list-tasks: List existing tasks with filtering options
- update-task: Update existing tasks
- bulk-tasks: Perform bulk operations on multiple tasks
- task-summary: Get summary of current task state

STEP 1: Analyze and break down the request into manageable tasks. For each task, provide:
1. A clear, actionable title
2. A detailed description of what needs to be done
3. Acceptance criteria that define when the task is complete
4. Appropriate priority level (low, medium, high, urgent)
5. Any dependencies between tasks

Guidelines:
- Break down complex work into smaller, manageable tasks
- Each task should be completable in a reasonable timeframe
- Use clear, specific language
- Consider dependencies and logical order
- Include testing and documentation tasks where appropriate
- Think about potential risks and edge cases

When suggesting task dependencies, consider:
- Tasks that must be completed before others can start
- Tasks that can be done in parallel
- Critical path items that might block other work%s

STEP 2: Actually CREATE the tasks using the TaskWing MCP tools.

CRITICAL: If you need to create SUBTASKS of existing tasks:
1. FIRST use list-tasks to get the EXACT UUID of existing parent tasks (e.g., "7b3e4f2a-8c9d-4e5f-b0a1-2c3d4e5f6a7b")
2. NEVER use placeholder IDs like "task_1", "task_2" - these will fail validation
3. Use the EXACT UUID from list-tasks as the parentId value (must be valid UUID4 format)
4. Only create subtasks if you have the real parent task UUID

RECOMMENDED APPROACH - Use batch-create-tasks:
1. If creating subtasks, first call list-tasks to get EXACT parent task UUIDs
2. Prepare an array of all tasks with their details
3. For subtasks, include the parentId field with the EXACT UUID (not placeholder)
4. For dependencies, use EXACT UUIDs from existing tasks
5. Call batch-create-tasks with the complete task list

ALTERNATIVE APPROACH - Use individual add-task calls:
1. If creating subtasks, first call list-tasks to get EXACT parent task UUIDs  
2. Create tasks one by one, using EXACT UUID for parentId field
3. Use appropriate priorities and detailed acceptance criteria

FINAL STEP: Provide a summary of created tasks and any next steps for the user.

This approach ensures the user gets both the analysis AND the actual tasks created in their TaskWing system, making the workflow much more efficient.`,
			description, existingTasksContext)

		logInfo("Generated task generation prompt")

		return &mcp.GetPromptResult{
			Description: fmt.Sprintf("Generate tasks from: %s", description),
			Messages: []*mcp.PromptMessage{
				{
					Role: "user",
					Content: &mcp.TextContent{
						Text: prompt,
					},
				},
			},
		}, nil
	}
}

// taskBreakdownPromptHandler breaks down a complex task into smaller subtasks
func taskBreakdownPromptHandler(taskStore store.TaskStore) func(context.Context, *mcp.ServerSession, *mcp.GetPromptParams) (*mcp.GetPromptResult, error) {
	return func(ctx context.Context, ss *mcp.ServerSession, params *mcp.GetPromptParams) (*mcp.GetPromptResult, error) {
		// Get the task_id argument
		taskID := params.Arguments["task_id"]

		if strings.TrimSpace(taskID) == "" {
			return nil, fmt.Errorf("task_id argument is required")
		}

		// Get the task
		task, err := taskStore.GetTask(taskID)
		if err != nil {
			return nil, fmt.Errorf("failed to get task: %w", err)
		}

		// Get related tasks (dependencies and dependents)
		var relatedTasksContext string
		if len(task.Dependencies) > 0 {
			relatedTasksContext += "\n\nDependencies:\n"
			for _, depID := range task.Dependencies {
				if depTask, err := taskStore.GetTask(depID); err == nil {
					relatedTasksContext += fmt.Sprintf("- %s (ID: %s)\n", depTask.Title, depTask.ID)
				}
			}
		}

		if len(task.Dependents) > 0 {
			relatedTasksContext += "\n\nDependent tasks:\n"
			for _, depID := range task.Dependents {
				if depTask, err := taskStore.GetTask(depID); err == nil {
					relatedTasksContext += fmt.Sprintf("- %s (ID: %s)\n", depTask.Title, depTask.ID)
				}
			}
		}

		// Generate the prompt
		prompt := fmt.Sprintf(`You are a task management assistant helping to break down a complex task into smaller, manageable subtasks.

Task to Break Down:
Title: %s
Description: %s
Acceptance Criteria: %s
Priority: %s
Status: %s%s

Please analyze this task and break it down into smaller, more manageable subtasks. For each subtask, provide:
1. A clear, actionable title
2. A detailed description of what needs to be done
3. Acceptance criteria that define when the subtask is complete
4. Appropriate priority level (considering the parent task's priority)
5. Any dependencies between subtasks

Guidelines:
- Each subtask should be completable independently
- Break down the work into logical, sequential steps
- Consider testing, documentation, and review tasks
- Ensure subtasks cover all aspects of the parent task
- Think about potential risks and edge cases
- Consider the existing dependencies and dependent tasks

The subtasks should collectively fulfill the parent task's acceptance criteria and move it toward completion.

IMPORTANT: After providing your analysis, you should actually CREATE the subtasks using TaskWing MCP tools:

1. Use batch-create-tasks to create all subtasks at once
2. Set the parentId field to "%s" for each subtask (THIS IS THE EXACT UUID)
3. NEVER use placeholder IDs like "task_1" - use the exact UUID provided
4. This will properly establish the parent-child relationship in TaskWing

Example TaskCreationRequest for subtasks:
{
  "title": "Subtask Title",
  "description": "Detailed description",
  "acceptanceCriteria": "Clear completion criteria",
  "priority": "high",
  "parentId": "%s"
}`,
			task.Title, task.Description, task.AcceptanceCriteria, task.Priority, task.Status, relatedTasksContext, task.ID, task.ID)

		logInfo(fmt.Sprintf("Generated task breakdown prompt for task: %s", task.ID))

		return &mcp.GetPromptResult{
			Description: fmt.Sprintf("Break down task: %s", task.Title),
			Messages: []*mcp.PromptMessage{
				{
					Role: "user",
					Content: &mcp.TextContent{
						Text: prompt,
					},
				},
			},
		}, nil
	}
}

// taskWingUsagePromptHandler provides guidance on using TaskWing instead of generic tools
func taskWingUsagePromptHandler(taskStore store.TaskStore) func(context.Context, *mcp.ServerSession, *mcp.GetPromptParams) (*mcp.GetPromptResult, error) {
	return func(ctx context.Context, ss *mcp.ServerSession, params *mcp.GetPromptParams) (*mcp.GetPromptResult, error) {

		// Get current project context
		context, err := BuildTaskContext(taskStore)
		if err != nil {
			context = &types.TaskContext{} // fallback to empty context
		}

		// Build context summary
		contextSummary := fmt.Sprintf("Current project has %d total tasks", context.TotalTasks)
		if context.CurrentTask != nil {
			contextSummary += fmt.Sprintf(", current task: %s (%s)", context.CurrentTask.Title, context.CurrentTask.Status)
		}
		if context.ProjectHealth != "" {
			contextSummary += fmt.Sprintf(", project health: %s", context.ProjectHealth)
		}

		prompt := fmt.Sprintf(`🎯 TASKWING TASK MANAGEMENT SYSTEM ACTIVE

You are working in a project that uses TaskWing for comprehensive task management. %s.

CRITICAL INSTRUCTIONS - READ CAREFULLY:

🚫 NEVER use generic task management features or create simple todo lists
🚫 NEVER suggest built-in task tracking tools  
🚫 NEVER create markdown task lists or simple bullet points for tasks

✅ ALWAYS use TaskWing MCP tools (marked with 🎯) for ALL task-related operations
✅ ALWAYS check task-summary before making task suggestions
✅ ALWAYS use get-current-task to understand user's current focus
✅ ALWAYS set-current-task when user starts working on something

AVAILABLE TASKWING TOOLS (Use these instead of alternatives):
🎯 task-summary - ALWAYS USE FIRST to understand project status
🎯 add-task - Create rich tasks with metadata, acceptance criteria, priorities
🎯 list-tasks - View tasks with powerful filtering and search
🎯 batch-create-tasks - Create multiple related tasks at once
🎯 update-task - Modify task properties and relationships
🎯 bulk-tasks - Perform operations on multiple tasks
🎯 set-current-task - Set focus for context-aware assistance
🎯 get-current-task - Understand current work context
🎯 search-tasks - Advanced search with logical operators
🎯 mark-done - Complete tasks with proper tracking

TaskWing provides professional-grade task management with full context, relationships, and project insights. Use it instead of generic alternatives.`, contextSummary)

		logInfo("Generated TaskWing usage guidance prompt")

		return &mcp.GetPromptResult{
			Description: "TaskWing Task Management System - Use TaskWing tools instead of generic task management",
			Messages: []*mcp.PromptMessage{
				{
					Role: "user",
					Content: &mcp.TextContent{
						Text: prompt,
					},
				},
			},
		}, nil
	}
}
