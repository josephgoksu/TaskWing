/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

import (
	"context"
	"fmt"
	"log"

	"github.com/josephgoksu/taskwing.app/models"
	"github.com/josephgoksu/taskwing.app/store"
	"github.com/josephgoksu/taskwing.app/types"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// mcpCmd represents the mcp command
var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Start MCP server for AI tool integration",
	Long: `Start a Model Context Protocol (MCP) server to enable AI tools like Claude Code,
Cursor, and other AI assistants to interact with TaskWing tasks.

The MCP server runs over stdin/stdout and provides tools for:
- Adding new tasks
- Listing and filtering tasks
- Updating existing tasks
- Deleting tasks
- Marking tasks as done
- Getting task details
- Managing current active task

Example usage with Claude Code:
  taskwing mcp

The server will run until the client disconnects.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runMCPServer(cmd.Context())
	},
}

func init() {
	rootCmd.AddCommand(mcpCmd)
	// MCP server inherits verbose flag from root command
}

func runMCPServer(ctx context.Context) error {

	// Initialize TaskWing store
	taskStore, err := GetStore()
	if err != nil {
		return fmt.Errorf("failed to initialize task store: %w", err)
	}
	defer taskStore.Close()

	// Create MCP server
	impl := &mcp.Implementation{
		Name:    "taskwing",
		Version: version,
	}

	// Create server options
	serverOpts := &mcp.ServerOptions{}

	server := mcp.NewServer(impl, serverOpts)

	// Register MCP tools
	if err := registerMCPTools(server, taskStore); err != nil {
		return fmt.Errorf("failed to register MCP tools: %w", err)
	}

	// Register advanced MCP tools
	if err := RegisterAdvancedMCPTools(server, taskStore); err != nil {
		return fmt.Errorf("failed to register advanced MCP tools: %w", err)
	}

	// Register MCP resources
	if err := registerMCPResources(server, taskStore); err != nil {
		return fmt.Errorf("failed to register MCP resources: %w", err)
	}

	// Register MCP prompts
	if err := registerMCPPrompts(server, taskStore); err != nil {
		return fmt.Errorf("failed to register MCP prompts: %w", err)
	}

	// Run the server over stdin/stdout
	if err := server.Run(ctx, mcp.NewStdioTransport()); err != nil {
		return fmt.Errorf("MCP server failed: %w", err)
	}

	return nil
}

func registerMCPTools(server *mcp.Server, taskStore store.TaskStore) error {
	// Add task tool
	mcp.AddTool(server, &mcp.Tool{
		Name:        "add-task",
		Description: "Create a new task with comprehensive details. Returns the created task with its unique ID. Validates all inputs and checks for dependency conflicts.",
	}, addTaskHandler(taskStore))

	// List tasks tool
	mcp.AddTool(server, &mcp.Tool{
		Name:        "list-tasks",
		Description: "List and filter tasks with powerful search capabilities. Supports filtering by status, priority, parent task, and text search. Returns task count and detailed task information.",
	}, listTasksHandler(taskStore))

	// Update task tool
	mcp.AddTool(server, &mcp.Tool{
		Name:        "update-task",
		Description: "Update any properties of an existing task. Supports partial updates - only provide fields you want to change. Validates all changes and maintains data integrity.",
	}, updateTaskHandler(taskStore))

	// Delete task tool
	mcp.AddTool(server, &mcp.Tool{
		Name:        "delete-task",
		Description: "Safely delete a task by ID. Prevents deletion of tasks with dependents to maintain referential integrity. Returns clear error messages if deletion is blocked.",
	}, deleteTaskHandler(taskStore))

	// Mark task done tool
	mcp.AddTool(server, &mcp.Tool{
		Name:        "mark-done",
		Description: "Mark a task as completed and set its completion timestamp. This is a convenience method that updates status to 'completed' and records completion time.",
	}, markDoneHandler(taskStore))

	// Get task tool
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get-task",
		Description: "Retrieve comprehensive details about a specific task including all metadata, relationships, and timestamps. Useful for examining task state before updates.",
	}, getTaskHandler(taskStore))

	// Current task management tools
	mcp.AddTool(server, &mcp.Tool{
		Name:        "set-current-task",
		Description: "Set the current active task that you're working on. This helps AI tools understand what you're currently focused on.",
	}, setCurrentTaskHandler(taskStore))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get-current-task",
		Description: "Get the current active task that you're working on. Returns the task details or indicates if no current task is set.",
	}, getCurrentTaskHandler(taskStore))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "clear-current-task",
		Description: "Clear the current active task setting. Use this when you're finished with the current task or want to reset.",
	}, clearCurrentTaskHandler(taskStore))

	return nil
}

func registerMCPResources(server *mcp.Server, taskStore store.TaskStore) error {
	// Tasks resource - provides access to task data
	server.AddResource(&mcp.Resource{
		URI:         "taskwing://tasks",
		Name:        "tasks",
		Description: "Access to all tasks in JSON format",
		MIMEType:    "application/json",
	}, tasksResourceHandler(taskStore))

	// Config resource - provides access to TaskWing configuration
	server.AddResource(&mcp.Resource{
		URI:         "taskwing://config",
		Name:        "config",
		Description: "TaskWing configuration settings",
		MIMEType:    "application/json",
	}, configResourceHandler())

	return nil
}

func registerMCPPrompts(server *mcp.Server, taskStore store.TaskStore) error {
	// Task generation prompt
	server.AddPrompt(&mcp.Prompt{
		Name:        "task-generation",
		Description: "Generate tasks from natural language descriptions",
		Arguments: []*mcp.PromptArgument{
			{
				Name:        "description",
				Description: "Natural language description of work to be done",
				Required:    true,
			},
		},
	}, taskGenerationPromptHandler(taskStore))

	// Task breakdown prompt
	server.AddPrompt(&mcp.Prompt{
		Name:        "task-breakdown",
		Description: "Break down a complex task into smaller subtasks",
		Arguments: []*mcp.PromptArgument{
			{
				Name:        "task_id",
				Description: "ID of the task to break down",
				Required:    true,
			},
		},
	}, taskBreakdownPromptHandler(taskStore))

	return nil
}

// Type aliases for backward compatibility and convenience
type AddTaskParams = types.AddTaskParams
type ListTasksParams = types.ListTasksParams
type UpdateTaskParams = types.UpdateTaskParams
type DeleteTaskParams = types.DeleteTaskParams
type MarkDoneParams = types.MarkDoneParams
type GetTaskParams = types.GetTaskParams
type TaskResponse = types.TaskResponse
type TaskListResponse = types.TaskListResponse
type DeleteTaskResponse = types.DeleteTaskResponse

func taskToResponse(task models.Task) types.TaskResponse {
	var completedAt *string
	if task.CompletedAt != nil {
		completed := task.CompletedAt.Format("2006-01-02T15:04:05Z")
		completedAt = &completed
	}

	return types.TaskResponse{
		ID:                 task.ID,
		Title:              task.Title,
		Description:        task.Description,
		AcceptanceCriteria: task.AcceptanceCriteria,
		Status:             string(task.Status),
		Priority:           string(task.Priority),
		ParentID:           task.ParentID,
		SubtaskIDs:         task.SubtaskIDs,
		Dependencies:       task.Dependencies,
		Dependents:         task.Dependents,
		CreatedAt:          task.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:          task.UpdatedAt.Format("2006-01-02T15:04:05Z"),
		CompletedAt:        completedAt,
	}
}

func logError(err error) {
	if viper.GetBool("verbose") {
		log.Printf("[MCP ERROR] %v", err)
	}
}

func logInfo(msg string) {
	if viper.GetBool("verbose") {
		log.Printf("[MCP INFO] %s", msg)
	}
}

func logDebug(msg string) {
	if viper.GetBool("verbose") {
		log.Printf("[MCP DEBUG] %s", msg)
	}
}

func logToolCall(toolName string, params interface{}) {
	if viper.GetBool("verbose") {
		log.Printf("[MCP TOOL] %s called with params: %+v", toolName, params)
	}
}
