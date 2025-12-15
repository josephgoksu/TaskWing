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
	fmt.Fprintf(os.Stderr, "Configuration: 15 essential tools (~9.6k tokens, 71%% reduction)\n")
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
			fmt.Fprintf(os.Stderr, "task management with 15 optimized tools (~9.6k tokens).\n")
			fmt.Fprintf(os.Stderr, "Use these tools instead of markdown lists:\n")
			fmt.Fprintf(os.Stderr, "  • task-summary - ALWAYS call first\n")
			fmt.Fprintf(os.Stderr, "  • add-task - Create rich tasks\n")
			fmt.Fprintf(os.Stderr, "  • query-tasks - Natural language search\n")
			fmt.Fprintf(os.Stderr, "  • See MCP_TOOLS_REFERENCE.md for all 15 tools\n")
			fmt.Fprintf(os.Stderr, "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")

			if viper.GetBool("verbose") {
				fmt.Fprintf(os.Stderr, "[DEBUG] MCP client initialization complete\n")
			}
		},
	}

	server := mcpsdk.NewServer(impl, serverOpts)

	// Register ALL essential MCP tools (15 tools optimized for minimal token usage)
	// All other tool registrations are disabled to reduce context window usage
	if err := taskwingmcp.RegisterCoreTools(server, taskStore); err != nil {
		return fmt.Errorf("failed to register MCP tools: %w", err)
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
