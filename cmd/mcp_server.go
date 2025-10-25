/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

// MCP server bootstrap and registration (renamed from mcpsdk.go)

import (
	"context"
	"fmt"
	"log"
	"os"

	taskwingmcp "github.com/josephgoksu/TaskWing/mcp"
	"github.com/josephgoksu/TaskWing/models"
	"github.com/josephgoksu/TaskWing/types"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
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

	// Configure shared hooks for MCP helpers
	taskwingmcp.ConfigureHooks(taskwingmcp.Hooks{
		GetCurrentTask:       GetCurrentTask,
		GetConfig:            GetConfig,
		LogInfo:              logInfo,
		LogError:             logError,
		LogToolCall:          logToolCall,
		GetArchiveStore:      getArchiveStore,
		SuggestLessons:       aiSuggestLessons,
		PolishLessons:        aiPolishLessons,
		ArchiveAndDelete:     archiveAndDeleteSubtree,
		EncryptFile:          encryptFile,
		DecryptFile:          decryptFile,
		ResolveTaskReference: resolveTaskReference,
		GetVersion:           func() string { return version },
	})

	defer func() {
		if err := taskStore.Close(); err != nil {
			logError(fmt.Errorf("failed to close task store: %w", err))
		}
	}()

	// Create MCP server
	impl := &mcpsdk.Implementation{
		Name:    "taskwing",
		Version: version,
	}

	// Create server options with notification handlers
	serverOpts := &mcpsdk.ServerOptions{
		InitializedHandler: func(ctx context.Context, session *mcpsdk.ServerSession, params *mcpsdk.InitializedParams) {
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

	server := mcpsdk.NewServer(impl, serverOpts)

	// Register MCP tools
	if err := taskwingmcp.RegisterCoreTools(server, taskStore); err != nil {
		return fmt.Errorf("failed to register MCP tools: %w", err)
	}

	// Register advanced MCP tools
	if err := taskwingmcp.RegisterAdvancedMCPTools(server, taskStore); err != nil {
		return fmt.Errorf("failed to register advanced MCP tools: %w", err)
	}

	// Register task resolution tools
	if err := taskwingmcp.RegisterTaskResolutionTools(server, taskStore); err != nil {
		return fmt.Errorf("failed to register task resolution tools: %w", err)
	}

	// Register JSON processing tools
	if err := taskwingmcp.RegisterJSONProcessingTools(server, taskStore); err != nil {
		return fmt.Errorf("failed to register JSON processing tools: %w", err)
	}

	// Register workflow integration tools
	if err := taskwingmcp.RegisterWorkflowIntegrationTools(server, taskStore); err != nil {
		return fmt.Errorf("failed to register workflow integration tools: %w", err)
	}

	// Register intelligent MCP tools with natural language support and smart matching
	if err := taskwingmcp.RegisterIntelligentMCPTools(server, taskStore); err != nil {
		return fmt.Errorf("failed to register intelligent MCP tools: %w", err)
	}

	// Register planning tools (document → plan)
	if err := taskwingmcp.RegisterPlanningTools(server, taskStore); err != nil {
		return fmt.Errorf("failed to register planning MCP tools: %w", err)
	}

	// Register simple plan/iterate tools matching CLI
	if err := taskwingmcp.RegisterSimplePlanTools(server, taskStore); err != nil {
		return fmt.Errorf("failed to register simple planning tools: %w", err)
	}

	// Register board tools
	if err := taskwingmcp.RegisterBoardTools(server, taskStore); err != nil {
		return fmt.Errorf("failed to register board tools: %w", err)
	}

	// Register archive tools
	if err := taskwingmcp.RegisterArchiveTools(server, taskStore); err != nil {
		return fmt.Errorf("failed to register archive tools: %w", err)
	}

	// Register MCP resources
	if err := taskwingmcp.RegisterMCPResources(server, taskStore); err != nil {
		return fmt.Errorf("failed to register MCP resources: %w", err)
	}

	// Register MCP prompts
	if err := taskwingmcp.RegisterMCPPrompts(server, taskStore); err != nil {
		return fmt.Errorf("failed to register MCP prompts: %w", err)
	}

	// Run the server over stdin/stdout
	if err := server.Run(ctx, mcpsdk.NewStdioTransport()); err != nil {
		return fmt.Errorf("MCP server failed: %w", err)
	}

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
