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
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"
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

Example usage with Claude Code:
  taskwing mcp

The server will run until the client disconnects.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runMCPServer(cmd.Context())
	},
}

func init() {
	rootCmd.AddCommand(mcpCmd)
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
	
	// Create server options with notification handlers
	serverOpts := &mcp.ServerOptions{
		// Handle the initialized notification properly
		InitializedHandler: func(ctx context.Context, serverSession *mcp.ServerSession, params *mcp.InitializedParams) {
			logInfo("MCP client initialized successfully")
		},
	}
	
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

// Task-related type definitions for MCP tools
type AddTaskParams struct {
	Title              string   `json:"title" mcp:"Task title (required)"`
	Description        string   `json:"description,omitempty" mcp:"Task description"`
	AcceptanceCriteria string   `json:"acceptanceCriteria,omitempty" mcp:"Acceptance criteria for task completion"`
	Priority           string   `json:"priority,omitempty" mcp:"Task priority: low, medium, high, urgent"`
	Dependencies       []string `json:"dependencies,omitempty" mcp:"List of task IDs this task depends on"`
}

type ListTasksParams struct {
	Status    string `json:"status,omitempty" mcp:"Filter by status: pending, in-progress, completed, cancelled, on-hold, blocked, needs-review"`
	Priority  string `json:"priority,omitempty" mcp:"Filter by priority: low, medium, high, urgent"`
	Search    string `json:"search,omitempty" mcp:"Search in title and description"`
	ParentID  string `json:"parentId,omitempty" mcp:"Filter by parent task ID"`
	SortBy    string `json:"sortBy,omitempty" mcp:"Sort by: id, title, priority, createdAt, updatedAt"`
	SortOrder string `json:"sortOrder,omitempty" mcp:"Sort order: asc, desc"`
}

type UpdateTaskParams struct {
	ID                 string   `json:"id" mcp:"Task ID to update (required)"`
	Title              string   `json:"title,omitempty" mcp:"New task title"`
	Description        string   `json:"description,omitempty" mcp:"New task description"`
	AcceptanceCriteria string   `json:"acceptanceCriteria,omitempty" mcp:"New acceptance criteria"`
	Status             string   `json:"status,omitempty" mcp:"New task status"`
	Priority           string   `json:"priority,omitempty" mcp:"New task priority"`
	Dependencies       []string `json:"dependencies,omitempty" mcp:"New dependencies list"`
}

type DeleteTaskParams struct {
	ID string `json:"id" mcp:"Task ID to delete (required)"`
}

type MarkDoneParams struct {
	ID string `json:"id" mcp:"Task ID to mark as done (required)"`
}

type GetTaskParams struct {
	ID string `json:"id" mcp:"Task ID to retrieve (required)"`
}

// Tool response types
type TaskResponse struct {
	ID                 string   `json:"id"`
	Title              string   `json:"title"`
	Description        string   `json:"description"`
	AcceptanceCriteria string   `json:"acceptanceCriteria"`
	Status             string   `json:"status"`
	Priority           string   `json:"priority"`
	Dependencies       []string `json:"dependencies"`
	Dependents         []string `json:"dependents"`
	CreatedAt          string   `json:"createdAt"`
	UpdatedAt          string   `json:"updatedAt"`
	CompletedAt        *string  `json:"completedAt"`
}

type TaskListResponse struct {
	Tasks []TaskResponse `json:"tasks"`
	Count int            `json:"count"`
}

func taskToResponse(task models.Task) TaskResponse {
	var completedAt *string
	if task.CompletedAt != nil {
		completed := task.CompletedAt.Format("2006-01-02T15:04:05Z")
		completedAt = &completed
	}

	return TaskResponse{
		ID:                 task.ID,
		Title:              task.Title,
		Description:        task.Description,
		AcceptanceCriteria: task.AcceptanceCriteria,
		Status:             string(task.Status),
		Priority:           string(task.Priority),
		Dependencies:       task.Dependencies,
		Dependents:         task.Dependents,
		CreatedAt:          task.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:          task.UpdatedAt.Format("2006-01-02T15:04:05Z"),
		CompletedAt:        completedAt,
	}
}

func logError(err error) {
	if verbose {
		log.Printf("Error: %v", err)
	}
}

func logInfo(msg string) {
	if verbose {
		log.Printf("Info: %s", msg)
	}
}
