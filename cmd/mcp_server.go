/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/josephgoksu/TaskWing/internal/app"
	"github.com/josephgoksu/TaskWing/internal/config"
	"github.com/josephgoksu/TaskWing/internal/knowledge"
	"github.com/josephgoksu/TaskWing/internal/llm"
	mcppresenter "github.com/josephgoksu/TaskWing/internal/mcp"
	"github.com/josephgoksu/TaskWing/internal/memory"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// mcpCmd represents the mcp command
var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Start MCP server for AI tool integration",
	Long: `Start a Model Context Protocol (MCP) server to enable AI tools like Claude Code,
Cursor, and other AI assistants to interact with TaskWing project memory.

The MCP server provides the recall tool that gives AI tools access to:
- Knowledge nodes (decisions, features, plans, notes)
- Semantic search across project memory
- Relationships between components

Example usage with Claude Code:
  taskwing mcp

The server will run until the client disconnects.`,
	Args: cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		// If arguments are provided but no subcommand was matched by Cobra,
		// it might mean an invalid subcommand or argument.
		// However, to maintain "taskwing mcp" as the way to start the server,
		// we only error if it looks like a subcommand attempt.
		if len(args) > 0 {
			return fmt.Errorf("unknown command %q for %q\nRun '%s --help' for usage", args[0], cmd.CommandPath(), cmd.Root().Name())
		}
		return runMCPServer(cmd.Context())
	},
}

func init() {
	rootCmd.AddCommand(mcpCmd)
	// NOTE: SSE transport (--port) intentionally removed. Stdio is the standard.
}

// mcpMarkdownResponse wraps Markdown content in an MCP tool result.
// Use this instead of mcpJSONResponse for token-efficient LLM responses.
// The presenter package provides formatting functions for common response types.
func mcpMarkdownResponse(markdown string) (*mcpsdk.CallToolResultFor[any], error) {
	return &mcpsdk.CallToolResultFor[any]{
		Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: markdown}},
	}, nil
}

// mcpErrorResponse wraps an error in an MCP tool result with IsError=true.
// Per MCP spec: tool errors should be returned in the result (not as protocol errors)
// so the LLM can see them and self-correct. Use mcppresenter.FormatError for formatting.
func mcpErrorResponse(err error) (*mcpsdk.CallToolResultFor[any], error) {
	return &mcpsdk.CallToolResultFor[any]{
		Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: mcppresenter.FormatError(err.Error())}},
		IsError: true,
	}, nil
}

// mcpValidationErrorResponse wraps a validation error with IsError=true.
// Use for input validation failures (missing required fields, invalid values).
func mcpValidationErrorResponse(field, message string) (*mcpsdk.CallToolResultFor[any], error) {
	return &mcpsdk.CallToolResultFor[any]{
		Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: mcppresenter.FormatValidationError(field, message)}},
		IsError: true,
	}, nil
}

// mcpFormattedErrorResponse wraps pre-formatted error text with IsError=true.
// Use when error message is already formatted (e.g., from presenter functions).
func mcpFormattedErrorResponse(formattedError string) (*mcpsdk.CallToolResultFor[any], error) {
	return &mcpsdk.CallToolResultFor[any]{
		Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: formattedError}},
		IsError: true,
	}, nil
}

// initMCPRepository initializes the memory repository.
// For MCP server, we use GetMemoryBasePathOrGlobal which allows fallback to global
// config for sandboxed environments where project detection may fail.
func initMCPRepository() (*memory.Repository, error) {
	// MCP server is a special case - it may run in sandboxed environments
	// where project context isn't available. Use the fallback-enabled path.
	memoryPath := config.GetMemoryBasePathOrGlobal()

	repo, err := memory.NewDefaultRepository(memoryPath)
	if err != nil {
		return nil, fmt.Errorf("open memory at %s: %w", memoryPath, err)
	}

	if viper.GetBool("verbose") {
		fmt.Fprintf(os.Stderr, "[DEBUG] Using memory path: %s\n", memoryPath)
	}
	return repo, nil
}

func runMCPServer(ctx context.Context) error {
	// NOTE: MCP uses stdio transport. stdout MUST be pure JSON-RPC.
	// All status/debug output goes to stderr only.
	fmt.Fprintln(os.Stderr, "TaskWing MCP Server starting...")

	// Initialize memory repository with fallback paths
	// Try: 1) configured path, 2) global ~/.taskwing/memory
	repo, err := initMCPRepository()
	if err != nil {
		return fmt.Errorf("failed to initialize memory repo: %w", err)
	}
	defer func() { _ = repo.Close() }()

	// Automatic embedding dimension consistency check on startup
	llmCfg, llmErr := config.LoadLLMConfig()
	if llmErr != nil {
		fmt.Fprintf(os.Stderr, "⚠  LLM config warning: %v\n", llmErr)
	}
	ks := knowledge.NewService(repo, llmCfg)
	if check, err := ks.CheckEmbeddingConsistency(); err != nil {
		fmt.Fprintf(os.Stderr, "⚠  Embedding check failed: %v\n", err)
	} else if check != nil {
		fmt.Fprintf(os.Stderr, "⚠  %s\n", check.Message)
	}

	// Create MCP server
	impl := &mcpsdk.Implementation{
		Name:    "taskwing",
		Version: version,
	}

	serverOpts := &mcpsdk.ServerOptions{
		InitializedHandler: func(ctx context.Context, session *mcpsdk.ServerSession, params *mcpsdk.InitializedParams) {
			fmt.Fprintf(os.Stderr, "✓ MCP connection established\n")
			if viper.GetBool("verbose") {
				fmt.Fprintf(os.Stderr, "[DEBUG] Client initialized\n")
			}
		},
	}

	server := mcpsdk.NewServer(impl, serverOpts)

	// Register recall tool - retrieves stored codebase knowledge for AI context
	tool := &mcpsdk.Tool{
		Name:        "recall",
		Description: "Retrieve codebase architecture knowledge: decisions, patterns, constraints, and features. Returns an AI-synthesized answer and relevant context by default. Use {\"query\":\"search term\"} for semantic search.",
	}

	mcpsdk.AddTool(server, tool, func(ctx context.Context, session *mcpsdk.ServerSession, params *mcpsdk.CallToolParamsFor[mcppresenter.ProjectContextParams]) (*mcpsdk.CallToolResultFor[any], error) {
		// Node-based system only
		nodes, err := repo.ListNodes("")
		if err != nil {
			return mcpErrorResponse(fmt.Errorf("list nodes: %w", err))
		}
		if len(nodes) == 0 {
			return &mcpsdk.CallToolResultFor[any]{
				Content: []mcpsdk.Content{
					&mcpsdk.TextContent{Text: "Project memory is empty. Run 'taskwing bootstrap' to analyze this repository and generate context."},
				},
			}, nil
		}
		return handleNodeContext(ctx, repo, params.Arguments)
	})

	// Register remember tool - add knowledge to project memory
	rememberTool := &mcpsdk.Tool{
		Name:        "remember",
		Description: "Add knowledge to project memory. Use this to persist decisions, patterns, or insights discovered during the session. Content will be classified automatically using AI.",
	}
	mcpsdk.AddTool(server, rememberTool, func(ctx context.Context, session *mcpsdk.ServerSession, params *mcpsdk.CallToolParamsFor[mcppresenter.RememberParams]) (*mcpsdk.CallToolResultFor[any], error) {
		return handleRemember(ctx, repo, params.Arguments)
	})

	// === Unified Tools (consolidated from multiple single-purpose tools) ===

	// Register unified 'code' tool - consolidates find_symbol, semantic_search_code, explain_symbol, get_callers, analyze_impact
	codeTool := &mcpsdk.Tool{
		Name: "code",
		Description: `Unified code intelligence tool. Use action parameter to select operation:
- find: Locate symbols by name, ID, or file path
- search: Hybrid semantic + lexical code search
- explain: Deep dive into a symbol with call graph and AI explanation
- callers: Get call graph relationships (who calls it, what it calls)
- impact: Analyze change impact via recursive call graph traversal
- simplify: Reduce code complexity while preserving behavior`,
	}
	mcpsdk.AddTool(server, codeTool, func(ctx context.Context, session *mcpsdk.ServerSession, params *mcpsdk.CallToolParamsFor[mcppresenter.CodeToolParams]) (*mcpsdk.CallToolResultFor[any], error) {
		result, err := mcppresenter.HandleCodeTool(ctx, repo, params.Arguments)
		if err != nil {
			return mcpErrorResponse(err)
		}
		if result.Error != "" {
			return mcpFormattedErrorResponse(mcppresenter.FormatError(result.Error))
		}
		return mcpMarkdownResponse(result.Content)
	})

	// Register unified 'task' tool - consolidates task_next, task_current, task_start, task_complete
	taskTool := &mcpsdk.Tool{
		Name: "task",
		Description: `Unified task lifecycle tool. Use action parameter to select operation:
- next: Get next pending task from plan (use auto_start=true to claim immediately)
- current: Get current in-progress task for session
- start: Claim a specific task by ID
- complete: Mark task as completed with summary`,
	}
	mcpsdk.AddTool(server, taskTool, func(ctx context.Context, session *mcpsdk.ServerSession, params *mcpsdk.CallToolParamsFor[mcppresenter.TaskToolParams]) (*mcpsdk.CallToolResultFor[any], error) {
		result, err := mcppresenter.HandleTaskTool(ctx, repo, params.Arguments)
		if err != nil {
			return mcpErrorResponse(err)
		}
		if result.Error != "" {
			return mcpFormattedErrorResponse(mcppresenter.FormatError(result.Error))
		}
		return mcpMarkdownResponse(result.Content)
	})

	// Register unified 'plan' tool - consolidates plan_clarify, plan_generate, audit_plan
	planTool := &mcpsdk.Tool{
		Name: "plan",
		Description: `Unified plan creation tool. Use action parameter to select operation:
- clarify: Refine goal with clarifying questions (loop until is_ready_to_plan=true)
- generate: Create plan with tasks from enriched goal
- audit: Verify completed plan with build/test/semantic checks (auto-fixes failures)`,
	}
	mcpsdk.AddTool(server, planTool, func(ctx context.Context, session *mcpsdk.ServerSession, params *mcpsdk.CallToolParamsFor[mcppresenter.PlanToolParams]) (*mcpsdk.CallToolResultFor[any], error) {
		result, err := mcppresenter.HandlePlanTool(ctx, repo, params.Arguments)
		if err != nil {
			return mcpErrorResponse(err)
		}
		if result.Error != "" {
			return mcpFormattedErrorResponse(mcppresenter.FormatError(result.Error))
		}
		return mcpMarkdownResponse(result.Content)
	})

	// Register 'debug' tool - helps diagnose issues using the DebugAgent
	debugTool := &mcpsdk.Tool{
		Name: "debug",
		Description: `Diagnose issues systematically using AI-powered analysis.
- Analyzes error symptoms and generates ranked hypotheses
- Provides investigation steps with commands to run
- Suggests quick fixes when applicable
- Uses architectural context for better diagnosis`,
	}
	mcpsdk.AddTool(server, debugTool, func(ctx context.Context, session *mcpsdk.ServerSession, params *mcpsdk.CallToolParamsFor[mcppresenter.DebugToolParams]) (*mcpsdk.CallToolResultFor[any], error) {
		result, err := mcppresenter.HandleDebugTool(ctx, repo, params.Arguments)
		if err != nil {
			return mcpErrorResponse(err)
		}
		if result.Error != "" {
			return mcpFormattedErrorResponse(mcppresenter.FormatError(result.Error))
		}
		return mcpMarkdownResponse(result.Content)
	})

	// Register 'policy' tool - OPA-powered policy enforcement for enterprise guardrails
	policyTool := &mcpsdk.Tool{
		Name: "policy",
		Description: `Evaluate code changes against OPA policies for enterprise compliance.
- check: Evaluate files against loaded Rego policies
- list: List all loaded policy files
- explain: Show policy rules and their purpose`,
	}
	mcpsdk.AddTool(server, policyTool, func(ctx context.Context, session *mcpsdk.ServerSession, params *mcpsdk.CallToolParamsFor[mcppresenter.PolicyToolParams]) (*mcpsdk.CallToolResultFor[any], error) {
		result, err := mcppresenter.HandlePolicyTool(ctx, params.Arguments)
		if err != nil {
			return mcpErrorResponse(err)
		}
		if result.Error != "" {
			return mcpFormattedErrorResponse(mcppresenter.FormatError(result.Error))
		}
		return mcpMarkdownResponse(result.Content)
	})

	// Run the server (stdio transport only)
	if err := server.Run(ctx, mcpsdk.NewStdioTransport()); err != nil {
		return fmt.Errorf("MCP server failed: %w", err)
	}

	return nil
}

// handleNodeContext returns context using the knowledge.Service (same as CLI).
// This ensures MCP and CLI use identical search logic with zero drift.
// Uses the app.RecallApp for all business logic - single source of truth.
func handleNodeContext(ctx context.Context, repo *memory.Repository, params mcppresenter.ProjectContextParams) (*mcpsdk.CallToolResultFor[any], error) {
	// Create app context with query role - respects llm.models.query config (same as CLI)
	appCtx := app.NewContextForRole(repo, llm.RoleQuery)
	recallApp := app.NewRecallApp(appCtx)

	query := strings.TrimSpace(params.Query)

	// No query = return project summary
	if query == "" {
		summary, err := recallApp.Summary(ctx)
		if err != nil {
			return mcpErrorResponse(fmt.Errorf("get summary: %w", err))
		}
		// Return token-efficient Markdown instead of verbose JSON
		return mcpMarkdownResponse(mcppresenter.FormatSummary(summary))
	}

	// Resolve workspace filtering
	// params.All=true or empty workspace = search all workspaces
	var workspace string
	if !params.All && params.Workspace != "" {
		if err := app.ValidateWorkspace(params.Workspace); err != nil {
			return mcpValidationErrorResponse("workspace", err.Error())
		}
		workspace = params.Workspace
	}

	// Execute query via app layer (ALL business logic delegated)
	// Include symbols in MCP recall for enhanced context
	// Note: Only generate answer if explicitly requested (params.Answer=true)
	// to avoid expensive LLM calls on every search
	result, err := recallApp.Query(ctx, query, app.RecallOptions{
		Limit:          5,
		SymbolLimit:    5,
		GenerateAnswer: params.Answer, // Only when explicitly requested
		IncludeSymbols: true,          // Include code symbols alongside knowledge
		Workspace:      workspace,
		IncludeRoot:    true, // Always include root knowledge when filtering by workspace
	})
	if err != nil {
		return mcpErrorResponse(fmt.Errorf("search failed: %w", err))
	}

	// Return token-efficient Markdown instead of verbose JSON
	return mcpMarkdownResponse(mcppresenter.FormatRecall(result))
}

// === Shared Tool Handlers ===

// handleRemember adds knowledge to project memory.
// Uses app.MemoryApp for all business logic - single source of truth.
func handleRemember(ctx context.Context, repo *memory.Repository, params mcppresenter.RememberParams) (*mcpsdk.CallToolResultFor[any], error) {
	content := strings.TrimSpace(params.Content)
	if content == "" {
		return mcpValidationErrorResponse("content", "content is required")
	}

	// Use MemoryApp for add (same as CLI `tw add`)
	// Use RoleBootstrap for knowledge ingestion (classification + embedding)
	appCtx := app.NewContextForRole(repo, llm.RoleBootstrap)
	memoryApp := app.NewMemoryApp(appCtx)

	result, err := memoryApp.Add(ctx, content, app.AddOptions{
		Type: params.Type,
	})
	if err != nil {
		return mcpErrorResponse(fmt.Errorf("failed to add knowledge: %w", err))
	}

	// Return token-efficient Markdown instead of verbose JSON
	return mcpMarkdownResponse(mcppresenter.FormatRemember(result))
}
