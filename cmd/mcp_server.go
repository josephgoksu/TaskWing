/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/josephgoksu/TaskWing/internal/app"
	"github.com/josephgoksu/TaskWing/internal/codeintel"
	"github.com/josephgoksu/TaskWing/internal/config"
	"github.com/josephgoksu/TaskWing/internal/knowledge"
	"github.com/josephgoksu/TaskWing/internal/llm"
	"github.com/josephgoksu/TaskWing/internal/mcp/presenter"
	"github.com/josephgoksu/TaskWing/internal/memory"
	"github.com/josephgoksu/TaskWing/internal/task"
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

// ProjectContextParams defines the parameters for the recall tool
type ProjectContextParams struct {
	Query  string `json:"query,omitempty"`
	Answer bool   `json:"answer,omitempty"` // If true, generate RAG answer using LLM
}

// TaskNextParams defines the parameters for the task_next tool
type TaskNextParams struct {
	PlanID            string `json:"plan_id,omitempty"`             // Optional: specific plan ID (defaults to active plan)
	SessionID         string `json:"session_id,omitempty"`          // Required: unique ID for this AI session
	AutoStart         bool   `json:"auto_start,omitempty"`          // If true, automatically claim the task
	CreateBranch      bool   `json:"create_branch,omitempty"`       // If true, create a new git branch for this plan
	SkipUnpushedCheck bool   `json:"skip_unpushed_check,omitempty"` // If true, proceed despite unpushed commits (only if create_branch=true)
}

// TaskCurrentParams defines the parameters for the task_current tool
type TaskCurrentParams struct {
	SessionID string `json:"session_id,omitempty"` // Required: unique ID for this AI session
	PlanID    string `json:"plan_id,omitempty"`    // Optional: fallback to find any in-progress task in plan
}

// TaskStartParams defines the parameters for the task_start tool
type TaskStartParams struct {
	TaskID    string `json:"task_id"`    // Required: task to start
	SessionID string `json:"session_id"` // Required: unique ID for this AI session
}

// TaskCompleteParams defines the parameters for the task_complete tool
type TaskCompleteParams struct {
	TaskID        string   `json:"task_id"`                  // Required: task to complete
	Summary       string   `json:"summary,omitempty"`        // Optional: what was accomplished
	FilesModified []string `json:"files_modified,omitempty"` // Optional: files that were changed
}

// PlanClarifyParams defines the parameters for the plan_clarify tool
type PlanClarifyParams struct {
	Goal       string `json:"goal"`                  // Required: the user's goal
	History    string `json:"history,omitempty"`     // Optional: JSON array of previous Q&A [{q, a}, ...]
	AutoAnswer bool   `json:"auto_answer,omitempty"` // If true, use KG to auto-answer questions
}

// PlanGenerateParams defines the parameters for the plan_generate tool
type PlanGenerateParams struct {
	Goal         string `json:"goal"`          // Required: user's original goal
	EnrichedGoal string `json:"enriched_goal"` // Required: full technical spec from plan_clarify
	Save         bool   `json:"save"`          // If true (default), save plan to database
}

// PlanClarifyResponse is the response from the plan_clarify tool
type PlanClarifyResponse struct {
	Success       bool     `json:"success"`
	Questions     []string `json:"questions,omitempty"`
	GoalSummary   string   `json:"goal_summary,omitempty"`
	EnrichedGoal  string   `json:"enriched_goal,omitempty"`
	IsReadyToPlan bool     `json:"is_ready_to_plan"`
	ContextUsed   string   `json:"context_used,omitempty"` // Summary of KG context retrieved
	Message       string   `json:"message,omitempty"`
}

// PlanGenerateResponse is the response from the plan_generate tool
type PlanGenerateResponse struct {
	Success      bool        `json:"success"`
	PlanID       string      `json:"plan_id,omitempty"`
	Goal         string      `json:"goal,omitempty"`
	EnrichedGoal string      `json:"enriched_goal,omitempty"`
	Tasks        []task.Task `json:"tasks,omitempty"`
	Message      string      `json:"message,omitempty"`
	Hint         string      `json:"hint,omitempty"`
}

// RememberParams defines the parameters for the remember tool
type RememberParams struct {
	Content string `json:"content"`        // Required: knowledge to store
	Type    string `json:"type,omitempty"` // Optional: decision, feature, plan, note
}

// AuditPlanParams defines the parameters for the audit_plan tool
type AuditPlanParams struct {
	PlanID  string `json:"plan_id,omitempty"`  // Optional: specific plan ID (defaults to active plan)
	AutoFix bool   `json:"auto_fix,omitempty"` // If true, attempt to fix failures automatically (default: true)
}

// RememberResponse is the response from the remember tool
type RememberResponse struct {
	Success      bool   `json:"success"`
	ID           string `json:"id,omitempty"`
	Type         string `json:"type,omitempty"`
	Summary      string `json:"summary,omitempty"`
	HasEmbedding bool   `json:"has_embedding"`
	Message      string `json:"message,omitempty"`
}

// AuditPlanResponse is the response from the audit_plan tool
type AuditPlanResponse struct {
	Success        bool            `json:"success"`
	PlanID         string          `json:"plan_id,omitempty"`
	Status         string          `json:"status,omitempty"`      // "verified", "needs_revision", "failed"
	PlanStatus     task.PlanStatus `json:"plan_status,omitempty"` // Updated plan status
	BuildPassed    bool            `json:"build_passed,omitempty"`
	TestsPassed    bool            `json:"tests_passed,omitempty"`
	SemanticIssues []string        `json:"semantic_issues,omitempty"`
	FixesApplied   []string        `json:"fixes_applied,omitempty"`
	RetryCount     int             `json:"retry_count,omitempty"`
	Message        string          `json:"message,omitempty"`
	Hint           string          `json:"hint,omitempty"`
}

// === Code Intelligence Tool Parameters ===

// FindSymbolParams defines the parameters for the find_symbol tool
type FindSymbolParams struct {
	Name     string `json:"name,omitempty"`      // Symbol name to find (exact match)
	ID       uint32 `json:"id,omitempty"`        // Symbol ID for direct lookup
	FilePath string `json:"file_path,omitempty"` // File path to search in
	Language string `json:"language,omitempty"`  // Language filter (e.g., "go")
}

// SemanticSearchCodeParams defines the parameters for the semantic_search_code tool
type SemanticSearchCodeParams struct {
	Query    string `json:"query"`              // Required: search query
	Limit    int    `json:"limit,omitempty"`    // Max results (default 20)
	Kind     string `json:"kind,omitempty"`     // Filter by symbol kind (function, struct, etc.)
	FilePath string `json:"file_path,omitempty"` // Filter by file path
}

// GetCallersParams defines the parameters for the get_callers tool
type GetCallersParams struct {
	SymbolID   uint32 `json:"symbol_id,omitempty"`   // Symbol ID to get callers for
	SymbolName string `json:"symbol_name,omitempty"` // Symbol name (if ID not provided)
	Direction  string `json:"direction,omitempty"`   // "callers", "callees", or "both" (default: "both")
}

// AnalyzeImpactParams defines the parameters for the analyze_impact tool
type AnalyzeImpactParams struct {
	SymbolID   uint32 `json:"symbol_id,omitempty"`   // Symbol ID to analyze
	SymbolName string `json:"symbol_name,omitempty"` // Symbol name (if ID not provided)
	MaxDepth   int    `json:"max_depth,omitempty"`   // Max recursion depth (default 5)
}

// Response DTOs are defined in internal/knowledge/response.go for DRY.
// TypeSummary is MCP-specific (overview mode only).

// TypeSummary is now defined in internal/knowledge/response.go

// mcpJSONResponse wraps data in an MCP tool result with JSON content
func mcpJSONResponse(data any) (*mcpsdk.CallToolResultFor[any], error) {
	jsonBytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal response: %w", err)
	}
	return &mcpsdk.CallToolResultFor[any]{
		Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: string(jsonBytes)}},
	}, nil
}

// mcpMarkdownResponse wraps Markdown content in an MCP tool result.
// Use this instead of mcpJSONResponse for token-efficient LLM responses.
// The presenter package provides formatting functions for common response types.
func mcpMarkdownResponse(markdown string) (*mcpsdk.CallToolResultFor[any], error) {
	return &mcpsdk.CallToolResultFor[any]{
		Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: markdown}},
	}, nil
}

// initMCPRepository initializes the memory repository with fallback paths.
// It tries: 1) local .taskwing/memory, 2) global ~/.taskwing/memory
// This handles cases where MCP runs from read-only directories (e.g., sandboxed environments).
func initMCPRepository() (*memory.Repository, error) {
	// Build list of paths to try
	var pathsToTry []string

	// 1. Try configured path first (from viper)
	configuredPath := config.GetMemoryBasePath()
	pathsToTry = append(pathsToTry, configuredPath)

	// 2. Try global ~/.taskwing/memory as fallback
	if home, err := os.UserHomeDir(); err == nil {
		globalPath := filepath.Join(home, ".taskwing", "memory")
		// Only add if different from configured path
		if globalPath != configuredPath {
			pathsToTry = append(pathsToTry, globalPath)
		}
	}

	// Try each path in order
	var lastErr error
	for _, path := range pathsToTry {
		repo, err := memory.NewDefaultRepository(path)
		if err == nil {
			if viper.GetBool("verbose") {
				fmt.Fprintf(os.Stderr, "[DEBUG] Using memory path: %s\n", path)
			}
			return repo, nil
		}
		lastErr = err
		if viper.GetBool("verbose") {
			fmt.Fprintf(os.Stderr, "[DEBUG] Failed to use path %s: %v\n", path, err)
		}
	}

	return nil, fmt.Errorf("no writable memory path found (tried %v): %w", pathsToTry, lastErr)
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

	mcpsdk.AddTool(server, tool, func(ctx context.Context, session *mcpsdk.ServerSession, params *mcpsdk.CallToolParamsFor[ProjectContextParams]) (*mcpsdk.CallToolResultFor[any], error) {
		// Node-based system only
		nodes, err := repo.ListNodes("")
		if err != nil {
			return nil, fmt.Errorf("list nodes: %w", err)
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

	// Register task_next tool - get the next pending task from a plan
	taskNextTool := &mcpsdk.Tool{
		Name:        "task_next",
		Description: "Get the next pending task from a plan. Returns the highest priority pending task with its context. Use auto_start=true to immediately claim the task. Use create_branch=true to create a new git branch for this plan (default: work on current branch). Always call recall tool with task's suggested_recall_queries after getting a task.",
	}
	mcpsdk.AddTool(server, taskNextTool, func(ctx context.Context, session *mcpsdk.ServerSession, params *mcpsdk.CallToolParamsFor[TaskNextParams]) (*mcpsdk.CallToolResultFor[any], error) {
		return handleTaskNext(repo, params.Arguments)
	})

	// Register task_current tool - get the current in-progress task
	taskCurrentTool := &mcpsdk.Tool{
		Name:        "task_current",
		Description: "Get the current in-progress task for a session. Returns the task claimed by this session_id, or any in-progress task in the plan if session lookup fails.",
	}
	mcpsdk.AddTool(server, taskCurrentTool, func(ctx context.Context, session *mcpsdk.ServerSession, params *mcpsdk.CallToolParamsFor[TaskCurrentParams]) (*mcpsdk.CallToolResultFor[any], error) {
		return handleTaskCurrent(repo, params.Arguments)
	})

	// Register task_start tool - claim a specific task
	taskStartTool := &mcpsdk.Tool{
		Name:        "task_start",
		Description: "Start working on a specific task by claiming it. Sets status to in_progress and records the session_id. Fails if task is not in pending status.",
	}
	mcpsdk.AddTool(server, taskStartTool, func(ctx context.Context, session *mcpsdk.ServerSession, params *mcpsdk.CallToolParamsFor[TaskStartParams]) (*mcpsdk.CallToolResultFor[any], error) {
		return handleTaskStart(repo, params.Arguments)
	})

	// Register task_complete tool - mark a task as completed
	taskCompleteTool := &mcpsdk.Tool{
		Name:        "task_complete",
		Description: "Mark a task as completed. Requires the task to be in in_progress status. Optionally include a summary and list of files modified.",
	}
	mcpsdk.AddTool(server, taskCompleteTool, func(ctx context.Context, session *mcpsdk.ServerSession, params *mcpsdk.CallToolParamsFor[TaskCompleteParams]) (*mcpsdk.CallToolResultFor[any], error) {
		return handleTaskComplete(repo, params.Arguments)
	})

	// Register plan_clarify tool - refine a goal with clarifying questions
	planClarifyTool := &mcpsdk.Tool{
		Name:        "plan_clarify",
		Description: "Refine a development goal by asking clarifying questions. Call this in a loop until is_ready_to_plan is true. Pass user answers in the history parameter as JSON. When ready, call plan_generate with the enriched_goal.",
	}
	mcpsdk.AddTool(server, planClarifyTool, func(ctx context.Context, session *mcpsdk.ServerSession, params *mcpsdk.CallToolParamsFor[PlanClarifyParams]) (*mcpsdk.CallToolResultFor[any], error) {
		return handlePlanClarify(ctx, repo, params.Arguments)
	})

	// Register plan_generate tool - create a plan with tasks
	planGenerateTool := &mcpsdk.Tool{
		Name:        "plan_generate",
		Description: "Generate a development plan with tasks from a refined goal. Requires enriched_goal from plan_clarify. Saves the plan to the database and sets it as active.",
	}
	mcpsdk.AddTool(server, planGenerateTool, func(ctx context.Context, session *mcpsdk.ServerSession, params *mcpsdk.CallToolParamsFor[PlanGenerateParams]) (*mcpsdk.CallToolResultFor[any], error) {
		return handlePlanGenerate(ctx, repo, params.Arguments)
	})

	// Register remember tool - add knowledge to project memory
	rememberTool := &mcpsdk.Tool{
		Name:        "remember",
		Description: "Add knowledge to project memory. Use this to persist decisions, patterns, or insights discovered during the session. Content will be classified automatically using AI.",
	}
	mcpsdk.AddTool(server, rememberTool, func(ctx context.Context, session *mcpsdk.ServerSession, params *mcpsdk.CallToolParamsFor[RememberParams]) (*mcpsdk.CallToolResultFor[any], error) {
		return handleRemember(ctx, repo, params.Arguments)
	})

	// Register audit_plan tool - verify and fix completed plans
	auditPlanTool := &mcpsdk.Tool{
		Name:        "audit_plan",
		Description: "Audit a completed plan by running build, tests, and semantic verification. Automatically attempts to fix failures up to 3 times. Updates plan status to 'verified' or 'needs_revision'. Use this after all tasks are complete to validate the implementation.",
	}
	mcpsdk.AddTool(server, auditPlanTool, func(ctx context.Context, session *mcpsdk.ServerSession, params *mcpsdk.CallToolParamsFor[AuditPlanParams]) (*mcpsdk.CallToolResultFor[any], error) {
		return handleAuditPlan(ctx, repo, params.Arguments)
	})

	// === Code Intelligence Tools ===

	// Register find_symbol tool - locate symbols by name, ID, or file
	findSymbolTool := &mcpsdk.Tool{
		Name:        "find_symbol",
		Description: "Find code symbols (functions, structs, interfaces) by name, ID, or file path. Returns symbol metadata including location, signature, and documentation. Use this to locate specific code elements in the indexed codebase.",
	}
	mcpsdk.AddTool(server, findSymbolTool, func(ctx context.Context, session *mcpsdk.ServerSession, params *mcpsdk.CallToolParamsFor[FindSymbolParams]) (*mcpsdk.CallToolResultFor[any], error) {
		return handleFindSymbol(ctx, repo, params.Arguments)
	})

	// Register semantic_search_code tool - hybrid semantic + lexical search
	semanticSearchTool := &mcpsdk.Tool{
		Name:        "semantic_search_code",
		Description: "Search code using hybrid semantic (embedding) and lexical (FTS5) matching. Finds symbols by meaning, not just exact text. Supports filtering by symbol kind (function, struct, interface) or file path. Returns ranked results with relevance scores.",
	}
	mcpsdk.AddTool(server, semanticSearchTool, func(ctx context.Context, session *mcpsdk.ServerSession, params *mcpsdk.CallToolParamsFor[SemanticSearchCodeParams]) (*mcpsdk.CallToolResultFor[any], error) {
		return handleSemanticSearchCode(ctx, repo, params.Arguments)
	})

	// Register get_callers tool - get call graph relationships
	getCallersTool := &mcpsdk.Tool{
		Name:        "get_callers",
		Description: "Get the callers and/or callees of a symbol. Shows who calls this function and what it calls. Use direction='callers' for just callers, 'callees' for just callees, or 'both' (default) for both directions.",
	}
	mcpsdk.AddTool(server, getCallersTool, func(ctx context.Context, session *mcpsdk.ServerSession, params *mcpsdk.CallToolParamsFor[GetCallersParams]) (*mcpsdk.CallToolResultFor[any], error) {
		return handleGetCallers(ctx, repo, params.Arguments)
	})

	// Register analyze_impact tool - impact analysis via recursive CTEs
	analyzeImpactTool := &mcpsdk.Tool{
		Name:        "analyze_impact",
		Description: "Analyze the impact of changing a symbol. Uses recursive call graph traversal to find all downstream consumers that would be affected by a change. Returns affected symbols grouped by depth (distance from the changed symbol). Critical for understanding the 'blast radius' of code changes.",
	}
	mcpsdk.AddTool(server, analyzeImpactTool, func(ctx context.Context, session *mcpsdk.ServerSession, params *mcpsdk.CallToolParamsFor[AnalyzeImpactParams]) (*mcpsdk.CallToolResultFor[any], error) {
		return handleAnalyzeImpact(ctx, repo, params.Arguments)
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
func handleNodeContext(ctx context.Context, repo *memory.Repository, params ProjectContextParams) (*mcpsdk.CallToolResultFor[any], error) {
	// Create app context with query role - respects llm.models.query config (same as CLI)
	appCtx := app.NewContextForRole(repo, llm.RoleQuery)
	recallApp := app.NewRecallApp(appCtx)

	query := strings.TrimSpace(params.Query)

	// No query = return project summary
	if query == "" {
		summary, err := recallApp.Summary(ctx)
		if err != nil {
			return nil, fmt.Errorf("get summary: %w", err)
		}
		return mcpJSONResponse(summary)
	}

	// Execute query via app layer (ALL business logic delegated)
	// Include symbols in MCP recall for enhanced context
	result, err := recallApp.Query(ctx, query, app.RecallOptions{
		Limit:          5,
		SymbolLimit:    5,
		GenerateAnswer: params.Answer || query != "", // Default to answering if query present
		IncludeSymbols: true,                         // Include code symbols alongside knowledge
	})
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	// Return token-efficient Markdown instead of verbose JSON
	return mcpMarkdownResponse(presenter.FormatRecall(result))
}

// === Task Lifecycle Handlers ===

// TaskResponse is the standardized response for task operations
type TaskResponse struct {
	Success bool       `json:"success"`
	Message string     `json:"message,omitempty"`
	Task    *task.Task `json:"task,omitempty"`
	Plan    *task.Plan `json:"plan,omitempty"`
	Hint    string     `json:"hint,omitempty"`    // Suggestions for next actions
	Context string     `json:"context,omitempty"` // Rich Markdown context for task execution
	// Git workflow fields
	GitBranch          string `json:"git_branch,omitempty"`           // Feature branch for this plan
	GitWorkflowApplied bool   `json:"git_workflow_applied,omitempty"` // True if git workflow was executed
	GitUnpushedCommits bool   `json:"git_unpushed_commits,omitempty"` // True if blocked by unpushed commits
	GitUnpushedBranch  string `json:"git_unpushed_branch,omitempty"`  // Branch with unpushed commits
	// PR fields (populated when plan is complete)
	PRURL     string `json:"pr_url,omitempty"`     // URL of created PR
	PRCreated bool   `json:"pr_created,omitempty"` // True if PR was created
	// Audit fields (populated when all tasks complete)
	AuditTriggered  bool            `json:"audit_triggered,omitempty"`   // True if audit was started
	AuditStatus     string          `json:"audit_status,omitempty"`      // "verified", "needs_revision", or "running"
	AuditPlanStatus task.PlanStatus `json:"audit_plan_status,omitempty"` // Updated plan status after audit
}

// handleTaskNext returns the next pending task from a plan.
// Uses app.TaskApp for all business logic - single source of truth.
func handleTaskNext(repo *memory.Repository, params TaskNextParams) (*mcpsdk.CallToolResultFor[any], error) {
	appCtx := app.NewContext(repo)
	taskApp := app.NewTaskApp(appCtx)

	ctx := context.Background()
	result, err := taskApp.Next(ctx, app.TaskNextOptions{
		PlanID:            params.PlanID,
		SessionID:         params.SessionID,
		AutoStart:         params.AutoStart,
		CreateBranch:      params.CreateBranch,
		SkipUnpushedCheck: params.SkipUnpushedCheck,
	})
	if err != nil {
		return nil, err
	}

	// Return token-efficient Markdown instead of verbose JSON
	return mcpMarkdownResponse(presenter.FormatTask(result))
}

// handleTaskCurrent returns the current in-progress task.
// Uses app.TaskApp for all business logic - single source of truth.
func handleTaskCurrent(repo *memory.Repository, params TaskCurrentParams) (*mcpsdk.CallToolResultFor[any], error) {
	appCtx := app.NewContext(repo)
	taskApp := app.NewTaskApp(appCtx)

	ctx := context.Background()
	result, err := taskApp.Current(ctx, params.SessionID, params.PlanID)
	if err != nil {
		return nil, err
	}

	// Return token-efficient Markdown instead of verbose JSON
	return mcpMarkdownResponse(presenter.FormatTask(result))
}

// handleTaskStart claims a specific task for a session.
// Uses app.TaskApp for all business logic - single source of truth.
func handleTaskStart(repo *memory.Repository, params TaskStartParams) (*mcpsdk.CallToolResultFor[any], error) {
	appCtx := app.NewContext(repo)
	taskApp := app.NewTaskApp(appCtx)

	ctx := context.Background()
	result, err := taskApp.Start(ctx, app.TaskStartOptions{
		TaskID:    params.TaskID,
		SessionID: params.SessionID,
	})
	if err != nil {
		return nil, err
	}

	// Return token-efficient Markdown instead of verbose JSON
	return mcpMarkdownResponse(presenter.FormatTask(result))
}

// handleTaskComplete marks a task as completed.
// Uses app.TaskApp for all business logic - single source of truth.
func handleTaskComplete(repo *memory.Repository, params TaskCompleteParams) (*mcpsdk.CallToolResultFor[any], error) {
	// Use RoleBootstrap for audit operations triggered on plan completion
	appCtx := app.NewContextForRole(repo, llm.RoleBootstrap)
	taskApp := app.NewTaskApp(appCtx)

	ctx := context.Background()
	result, err := taskApp.Complete(ctx, app.TaskCompleteOptions{
		TaskID:        params.TaskID,
		Summary:       params.Summary,
		FilesModified: params.FilesModified,
	})
	if err != nil {
		return nil, err
	}

	// Return token-efficient Markdown instead of verbose JSON
	return mcpMarkdownResponse(presenter.FormatTask(result))
}

// === Plan Creation Handlers ===

// handlePlanClarify runs the ClarifyingAgent to refine a goal.
// Uses app.PlanApp for all business logic - single source of truth.
func handlePlanClarify(ctx context.Context, repo *memory.Repository, params PlanClarifyParams) (*mcpsdk.CallToolResultFor[any], error) {
	// Use RoleBootstrap for planning operations (same as CLI bootstrap/plan commands)
	appCtx := app.NewContextForRole(repo, llm.RoleBootstrap)
	planApp := app.NewPlanApp(appCtx)

	result, err := planApp.Clarify(ctx, app.ClarifyOptions{
		Goal:       params.Goal,
		History:    params.History,
		AutoAnswer: params.AutoAnswer,
	})
	if err != nil {
		return nil, err
	}

	return mcpJSONResponse(PlanClarifyResponse{
		Success:       result.Success,
		Questions:     result.Questions,
		GoalSummary:   result.GoalSummary,
		EnrichedGoal:  result.EnrichedGoal,
		IsReadyToPlan: result.IsReadyToPlan,
		ContextUsed:   result.ContextUsed,
		Message:       result.Message,
	})
}

// handlePlanGenerate runs the PlanningAgent to create tasks.
// Uses app.PlanApp for all business logic - single source of truth.
func handlePlanGenerate(ctx context.Context, repo *memory.Repository, params PlanGenerateParams) (*mcpsdk.CallToolResultFor[any], error) {
	// Use RoleBootstrap for planning operations (same as CLI bootstrap/plan commands)
	appCtx := app.NewContextForRole(repo, llm.RoleBootstrap)
	planApp := app.NewPlanApp(appCtx)

	result, err := planApp.Generate(ctx, app.GenerateOptions{
		Goal:         params.Goal,
		EnrichedGoal: params.EnrichedGoal,
		Save:         params.Save,
	})
	if err != nil {
		return nil, err
	}

	return mcpJSONResponse(PlanGenerateResponse{
		Success:      result.Success,
		PlanID:       result.PlanID,
		Goal:         result.Goal,
		EnrichedGoal: result.EnrichedGoal,
		Tasks:        result.Tasks,
		Message:      result.Message,
		Hint:         result.Hint,
	})
}

// handleRemember adds knowledge to project memory.
// Uses app.MemoryApp for all business logic - single source of truth.
func handleRemember(ctx context.Context, repo *memory.Repository, params RememberParams) (*mcpsdk.CallToolResultFor[any], error) {
	content := strings.TrimSpace(params.Content)
	if content == "" {
		return mcpJSONResponse(RememberResponse{
			Success: false,
			Message: "content is required",
		})
	}

	// Use MemoryApp for add (same as CLI `tw add`)
	// Use RoleBootstrap for knowledge ingestion (classification + embedding)
	appCtx := app.NewContextForRole(repo, llm.RoleBootstrap)
	memoryApp := app.NewMemoryApp(appCtx)

	result, err := memoryApp.Add(ctx, content, app.AddOptions{
		Type: params.Type,
	})
	if err != nil {
		return mcpJSONResponse(RememberResponse{
			Success: false,
			Message: fmt.Sprintf("failed to add knowledge: %v", err),
		})
	}

	return mcpJSONResponse(RememberResponse{
		Success:      true,
		ID:           result.ID,
		Type:         result.Type,
		Summary:      result.Summary,
		HasEmbedding: result.HasEmbedding,
		Message:      fmt.Sprintf("Knowledge stored as [%s]", result.Type),
	})
}

// handleAuditPlan runs the audit service on a plan.
// Uses app.PlanApp for all business logic - single source of truth.
func handleAuditPlan(ctx context.Context, repo *memory.Repository, params AuditPlanParams) (*mcpsdk.CallToolResultFor[any], error) {
	// Use RoleBootstrap for audit operations (same as CLI plan/audit commands)
	appCtx := app.NewContextForRole(repo, llm.RoleBootstrap)
	planApp := app.NewPlanApp(appCtx)

	result, err := planApp.Audit(ctx, app.AuditOptions{
		PlanID:  params.PlanID,
		AutoFix: params.AutoFix,
	})
	if err != nil {
		return nil, err
	}

	return mcpJSONResponse(AuditPlanResponse{
		Success:        result.Success,
		PlanID:         result.PlanID,
		Status:         result.Status,
		PlanStatus:     result.PlanStatus,
		BuildPassed:    result.BuildPassed,
		TestsPassed:    result.TestsPassed,
		SemanticIssues: result.SemanticIssues,
		FixesApplied:   result.FixesApplied,
		RetryCount:     result.RetryCount,
		Message:        result.Message,
		Hint:           result.Hint,
	})
}

// === Code Intelligence Handlers ===

// handleFindSymbol finds symbols by name, ID, or file.
// Uses app.CodeIntelApp for all business logic - single source of truth.
func handleFindSymbol(ctx context.Context, repo *memory.Repository, params FindSymbolParams) (*mcpsdk.CallToolResultFor[any], error) {
	appCtx := app.NewContext(repo)
	codeIntelApp := app.NewCodeIntelApp(appCtx)

	result, err := codeIntelApp.FindSymbol(ctx, app.FindSymbolOptions{
		Name:     params.Name,
		ID:       params.ID,
		FilePath: params.FilePath,
		Language: params.Language,
	})
	if err != nil {
		return nil, err
	}

	// Return token-efficient Markdown instead of verbose JSON
	return mcpMarkdownResponse(presenter.FormatSymbolList(result.Symbols))
}

// handleSemanticSearchCode performs hybrid semantic + lexical search.
// Uses app.CodeIntelApp for all business logic - single source of truth.
// H4 FIX: Added input validation for query length and limit bounds.
func handleSemanticSearchCode(ctx context.Context, repo *memory.Repository, params SemanticSearchCodeParams) (*mcpsdk.CallToolResultFor[any], error) {
	// H4 FIX: Input validation
	query := strings.TrimSpace(params.Query)
	if query == "" {
		return mcpJSONResponse(app.SearchCodeResult{
			Success: false,
			Message: "query is required",
		})
	}
	// Limit query length to prevent abuse (1000 chars is generous for a code search)
	const maxQueryLength = 1000
	if len(query) > maxQueryLength {
		query = query[:maxQueryLength]
	}

	// Clamp limit to reasonable bounds
	limit := params.Limit
	if limit <= 0 {
		limit = 20
	}
	const maxLimit = 100
	if limit > maxLimit {
		limit = maxLimit
	}

	appCtx := app.NewContextForRole(repo, llm.RoleQuery)
	codeIntelApp := app.NewCodeIntelApp(appCtx)

	result, err := codeIntelApp.SearchCode(ctx, app.SearchCodeOptions{
		Query:    query,
		Limit:    limit,
		Kind:     codeintel.SymbolKind(params.Kind),
		FilePath: params.FilePath,
	})
	if err != nil {
		return nil, err
	}

	// Return token-efficient Markdown instead of verbose JSON
	return mcpMarkdownResponse(presenter.FormatSearchResults(result.Results))
}

// handleGetCallers returns the callers and/or callees of a symbol.
// Uses app.CodeIntelApp for all business logic - single source of truth.
func handleGetCallers(ctx context.Context, repo *memory.Repository, params GetCallersParams) (*mcpsdk.CallToolResultFor[any], error) {
	appCtx := app.NewContext(repo)
	codeIntelApp := app.NewCodeIntelApp(appCtx)

	result, err := codeIntelApp.GetCallers(ctx, app.GetCallersOptions{
		SymbolID:   params.SymbolID,
		SymbolName: params.SymbolName,
		Direction:  params.Direction,
	})
	if err != nil {
		return nil, err
	}

	// Return token-efficient Markdown instead of verbose JSON
	return mcpMarkdownResponse(presenter.FormatCallers(result))
}

// handleAnalyzeImpact finds all symbols affected by changing a given symbol.
// Uses app.CodeIntelApp for all business logic - single source of truth.
// H4 FIX: Added input validation for max_depth to prevent deep recursion.
func handleAnalyzeImpact(ctx context.Context, repo *memory.Repository, params AnalyzeImpactParams) (*mcpsdk.CallToolResultFor[any], error) {
	// H4 FIX: Input validation - at least one identifier required
	symbolName := strings.TrimSpace(params.SymbolName)
	if params.SymbolID == 0 && symbolName == "" {
		return mcpJSONResponse(app.AnalyzeImpactResult{
			Success: false,
			Message: "symbol_id or symbol_name is required",
		})
	}

	// H4 FIX: Clamp max_depth to prevent deep recursion (reasonable max is 10)
	maxDepth := params.MaxDepth
	if maxDepth <= 0 {
		maxDepth = 5 // default
	}
	const maxAllowedDepth = 10
	if maxDepth > maxAllowedDepth {
		maxDepth = maxAllowedDepth
	}

	appCtx := app.NewContext(repo)
	codeIntelApp := app.NewCodeIntelApp(appCtx)

	result, err := codeIntelApp.AnalyzeImpact(ctx, app.AnalyzeImpactOptions{
		SymbolID:   params.SymbolID,
		SymbolName: symbolName,
		MaxDepth:   maxDepth,
	})
	if err != nil {
		return nil, err
	}

	// Return token-efficient Markdown instead of verbose JSON
	return mcpMarkdownResponse(presenter.FormatImpact(result))
}
