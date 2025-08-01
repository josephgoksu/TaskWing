/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/josephgoksu/taskwing.app/store"
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

Please analyze this request and create well-structured tasks. For each task, provide:
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

Please format your response as a structured breakdown with clear task definitions that can be easily converted into TaskWing tasks.`,
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

Please format your response as a structured breakdown with clear subtask definitions that can be easily converted into TaskWing tasks.`,
			task.Title, task.Description, task.AcceptanceCriteria, task.Priority, task.Status, relatedTasksContext)

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
