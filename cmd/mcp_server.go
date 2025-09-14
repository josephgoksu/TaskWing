/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

// MCP server bootstrap and registration (renamed from mcp.go)

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/josephgoksu/TaskWing/models"
	"github.com/josephgoksu/TaskWing/store"
	"github.com/josephgoksu/TaskWing/types"
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
	// Print startup message for AI tools
	fmt.Fprintf(os.Stderr, "\n🎯 TaskWing MCP Server Starting...\n")
	fmt.Fprintf(os.Stderr, "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	fmt.Fprintf(os.Stderr, "CRITICAL: Professional task management system active\n")
	fmt.Fprintf(os.Stderr, "AI tools MUST:\n")
	fmt.Fprintf(os.Stderr, "  1. Call 'task-summary' first to understand project\n")
	fmt.Fprintf(os.Stderr, "  2. Use TaskWing tools instead of markdown lists\n")
	fmt.Fprintf(os.Stderr, "  3. Set current task when user starts work\n")
	fmt.Fprintf(os.Stderr, "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")

	// Initialize TaskWing store
	taskStore, err := GetStore()
	if err != nil {
		return fmt.Errorf("failed to initialize task store: %w", err)
	}
	defer func() {
		if err := taskStore.Close(); err != nil {
			logError(fmt.Errorf("failed to close task store: %w", err))
		}
	}()

	// Create MCP server
	impl := &mcp.Implementation{
		Name:    "taskwing",
		Version: version,
	}

	// Create server options with notification handlers
	serverOpts := &mcp.ServerOptions{
		InitializedHandler: func(ctx context.Context, session *mcp.ServerSession, params *mcp.InitializedParams) {
			// Client has completed initialization - announce TaskWing availability
			fmt.Fprintf(os.Stderr, "\n🎯 TASKWING MCP CONNECTION ESTABLISHED\n")
			fmt.Fprintf(os.Stderr, "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
			fmt.Fprintf(os.Stderr, "AI Tool: You now have access to TaskWing professional\n")
			fmt.Fprintf(os.Stderr, "task management. Use these tools instead of markdown lists:\n")
			fmt.Fprintf(os.Stderr, "  • task-summary - ALWAYS call first\n")
			fmt.Fprintf(os.Stderr, "  • add-task - Create rich tasks\n")
			fmt.Fprintf(os.Stderr, "  • query-tasks - Natural language search\n")
			fmt.Fprintf(os.Stderr, "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")
			
			if viper.GetBool("verbose") {
				fmt.Fprintf(os.Stderr, "[DEBUG] MCP client initialization complete\n")
			}
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

	// Register task resolution tools
	if err := RegisterTaskResolutionTools(server, taskStore); err != nil {
		return fmt.Errorf("failed to register task resolution tools: %w", err)
	}

	// Register JSON processing tools
	if err := RegisterJSONProcessingTools(server, taskStore); err != nil {
		return fmt.Errorf("failed to register JSON processing tools: %w", err)
	}

	// Register workflow integration tools
	if err := RegisterWorkflowIntegrationTools(server, taskStore); err != nil {
		return fmt.Errorf("failed to register workflow integration tools: %w", err)
	}

	// Register intelligent MCP tools with natural language support and smart matching
	if err := RegisterIntelligentMCPTools(server, taskStore); err != nil {
		return fmt.Errorf("failed to register intelligent MCP tools: %w", err)
	}

	// Register planning tools (document → plan)
	if err := RegisterPlanningTools(server, taskStore); err != nil {
		return fmt.Errorf("failed to register planning MCP tools: %w", err)
	}

	// Register simple plan/iterate tools matching CLI
	if err := RegisterSimplePlanTools(server, taskStore); err != nil {
		return fmt.Errorf("failed to register simple planning tools: %w", err)
	}

	// Register board tools
	if err := RegisterBoardTools(server, taskStore); err != nil {
		return fmt.Errorf("failed to register board tools: %w", err)
	}

	// Register archive tools
	if err := RegisterArchiveTools(server, taskStore); err != nil {
		return fmt.Errorf("failed to register archive tools: %w", err)
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
	// CRITICAL: task-summary MUST be first - AI tools should always call this first
	mcp.AddTool(server, &mcp.Tool{
		Name:        "task-summary",
		Description: "🎯 ALWAYS CALL FIRST: Get project overview with total tasks, active, completed today, and project health. Essential for understanding context before any operations.",
	}, taskSummaryHandler(taskStore))

	// Add task tool
	mcp.AddTool(server, &mcp.Tool{
		Name:        "add-task",
		Description: "🎯 CREATE PROFESSIONAL TASK (use instead of simple todos): title, description, acceptanceCriteria, priority [low|medium|high|urgent], parentId, dependencies[]. Validates and maintains relationships.",
	}, addTaskHandler(taskStore))

	// List tasks tool
	mcp.AddTool(server, &mcp.Tool{
		Name:        "list-tasks",
		Description: "List tasks with filters. Args: status [todo|doing|review|done], priority [low|medium|high|urgent], search, parentId, sortBy [id|title|priority|createdAt|updatedAt], sortOrder [asc|desc]. Returns tasks+count.",
	}, listTasksHandler(taskStore))

	// Update task tool
	mcp.AddTool(server, &mcp.Tool{
		Name:        "update-task",
		Description: "Update a task by id or reference. Updatable: title, description, acceptanceCriteria, status [todo|doing|review|done], priority, parentId, dependencies[].",
	}, updateTaskHandler(taskStore))

	// Delete task tool
	mcp.AddTool(server, &mcp.Tool{
		Name:        "delete-task",
		Description: "Delete a task by id or reference. Blocks if task has dependents or subtasks. Use 'bulk-tasks' or 'clear-tasks' for batch deletes.",
	}, deleteTaskHandler(taskStore))

	// Mark task done tool
	mcp.AddTool(server, &mcp.Tool{
		Name:        "mark-done",
		Description: "Complete a task by id or reference (partial ID/title). Sets status=done and completedAt timestamp.",
	}, markDoneHandler(taskStore))

	// Get task tool
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get-task",
		Description: "Get one task by id or reference (partial ID/title). Returns full metadata, relationships, and timestamps.",
	}, getTaskHandler(taskStore))

	// Current task management tools
	mcp.AddTool(server, &mcp.Tool{
		Name:        "set-current-task",
		Description: "🎯 SET FOCUS TASK (essential for context): Set active task id used for context-aware responses. Persists in project config.",
	}, setCurrentTaskHandler(taskStore))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get-current-task",
		Description: "Return current active task (if set) with full details for context.",
	}, getCurrentTaskHandler(taskStore))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "clear-current-task",
		Description: "Clear the active task reference from project config.",
	}, clearCurrentTaskHandler(taskStore))

	// Clear tasks tool for bulk clearing with safety features
	mcp.AddTool(server, &mcp.Tool{
		Name:        "clear-tasks",
		Description: "Bulk delete with safety. Args: status (csv), priority (csv), completed (bool), all (bool), force (bool), no_backup (bool). Default clears status=done when no filters.",
	}, clearTasksHandler(taskStore))

	return nil
}

func registerMCPResources(server *mcp.Server, taskStore store.TaskStore) error {
	// TaskWing availability announcement - helps AI tools discover TaskWing is active
	server.AddResource(&mcp.Resource{
		URI:         "taskwing://system-status",
		Name:        "taskwing-system-status", 
		Description: "🎯 TASKWING ACTIVE - Professional task management system available. AI tools should use TaskWing MCP tools instead of generic alternatives.",
		MIMEType:    "application/json",
	}, systemStatusResourceHandler(taskStore))

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

	// Note: archive and knowledge resources have been removed.

	return nil
}

func registerMCPPrompts(server *mcp.Server, taskStore store.TaskStore) error {
	// TaskWing onboarding prompt - AI tools should read this on connection
	server.AddPrompt(&mcp.Prompt{
		Name:        "taskwing-onboarding", 
		Description: "🎯 CRITICAL ONBOARDING - TaskWing is active. AI tools must read this to understand how to use TaskWing instead of generic task management.",
	}, taskWingOnboardingPromptHandler(taskStore))

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

	// TaskWing usage guidance prompt
	server.AddPrompt(&mcp.Prompt{
		Name:        "taskwing-usage-guide",
		Description: "Get guidance on using TaskWing instead of generic task management tools",
	}, taskWingUsagePromptHandler(taskStore))

	return nil
}

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

func logToolCall(toolName string, params interface{}) {
	if viper.GetBool("verbose") {
		log.Printf("[MCP TOOL] %s called with params: %+v", toolName, params)
	}
}
