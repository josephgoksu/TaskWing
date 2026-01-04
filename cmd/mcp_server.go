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

	"github.com/josephgoksu/TaskWing/internal/agents/core"
	"github.com/josephgoksu/TaskWing/internal/agents/planning"
	"github.com/josephgoksu/TaskWing/internal/config"
	"github.com/josephgoksu/TaskWing/internal/knowledge"
	"github.com/josephgoksu/TaskWing/internal/llm"
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
	Query string `json:"query,omitempty"`
}

// TaskNextParams defines the parameters for the task_next tool
type TaskNextParams struct {
	PlanID    string `json:"plan_id,omitempty"`    // Optional: specific plan ID (defaults to active plan)
	SessionID string `json:"session_id,omitempty"` // Required: unique ID for this AI session
	AutoStart bool   `json:"auto_start,omitempty"` // If true, automatically claim the task
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

// TaskBlockParams defines the parameters for the task_block tool
type TaskBlockParams struct {
	TaskID string `json:"task_id"` // Required: task to block
	Reason string `json:"reason"`  // Required: why the task is blocked
}

// TaskUnblockParams defines the parameters for the task_unblock tool
type TaskUnblockParams struct {
	TaskID string `json:"task_id"` // Required: task to unblock
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
		Description: "Retrieve codebase architecture knowledge: decisions, patterns, constraints, and features. Returns overview by default. Use {\"query\":\"search term\"} for semantic search (e.g., {\"query\":\"authentication\"} or {\"query\":\"error handling\"}).",
	}

	mcpsdk.AddTool(server, tool, func(ctx context.Context, session *mcpsdk.ServerSession, params *mcpsdk.CallToolParamsFor[ProjectContextParams]) (*mcpsdk.CallToolResultFor[any], error) {
		query := strings.TrimSpace(params.Arguments.Query)

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
		return handleNodeContext(ctx, repo, query, nodes)
	})

	// Register task_next tool - get the next pending task from a plan
	taskNextTool := &mcpsdk.Tool{
		Name:        "task_next",
		Description: "Get the next pending task from a plan. Returns the highest priority pending task with its context. Use auto_start=true to immediately claim the task. Always call recall tool with task's suggested_recall_queries after getting a task.",
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

	// Register task_block tool - mark a task as blocked
	taskBlockTool := &mcpsdk.Tool{
		Name:        "task_block",
		Description: "Mark a task as blocked with a reason. Use this when you cannot proceed due to missing information, dependencies, or external factors. The task can be unblocked later.",
	}
	mcpsdk.AddTool(server, taskBlockTool, func(ctx context.Context, session *mcpsdk.ServerSession, params *mcpsdk.CallToolParamsFor[TaskBlockParams]) (*mcpsdk.CallToolResultFor[any], error) {
		return handleTaskBlock(repo, params.Arguments)
	})

	// Register task_unblock tool - unblock a blocked task
	taskUnblockTool := &mcpsdk.Tool{
		Name:        "task_unblock",
		Description: "Unblock a previously blocked task, returning it to pending status. Use this when the blocker has been resolved.",
	}
	mcpsdk.AddTool(server, taskUnblockTool, func(ctx context.Context, session *mcpsdk.ServerSession, params *mcpsdk.CallToolParamsFor[TaskUnblockParams]) (*mcpsdk.CallToolResultFor[any], error) {
		return handleTaskUnblock(repo, params.Arguments)
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

	// Run the server (stdio transport only)
	if err := server.Run(ctx, mcpsdk.NewStdioTransport()); err != nil {
		return fmt.Errorf("MCP server failed: %w", err)
	}

	return nil
}

// handleNodeContext returns context using the knowledge.Service (same as CLI).
// This ensures MCP and CLI use identical search logic with zero drift.
func handleNodeContext(ctx context.Context, repo *memory.Repository, query string, nodes []memory.Node) (*mcpsdk.CallToolResultFor[any], error) {
	// Use knowledge.Service.GetProjectSummary() — IDENTICAL to CLI, zero drift
	llmCfg, err := config.LoadLLMConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "[WARN] LLM config unavailable: %v\n", err)
		llmCfg = llm.Config{}
	}
	ks := knowledge.NewService(repo, llmCfg)

	if query == "" {
		summary, err := ks.GetProjectSummary(ctx)
		if err != nil {
			return nil, fmt.Errorf("get summary: %w", err)
		}
		return mcpJSONResponse(summary)
	}

	scored, err := ks.Search(ctx, query, 5)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	// Convert to NodeResponse (shared type, no embeddings)
	var responses []knowledge.NodeResponse
	for _, sn := range scored {
		responses = append(responses, knowledge.ScoredNodeToResponse(sn))
	}

	result := struct {
		Query   string                   `json:"query"`
		Results []knowledge.NodeResponse `json:"results"`
		Total   int                      `json:"total"`
	}{
		Query:   query,
		Results: responses,
		Total:   len(responses),
	}

	return mcpJSONResponse(result)
}

// === Task Lifecycle Handlers ===

// TaskResponse is the standardized response for task operations
type TaskResponse struct {
	Success bool       `json:"success"`
	Message string     `json:"message,omitempty"`
	Task    *task.Task `json:"task,omitempty"`
	Plan    *task.Plan `json:"plan,omitempty"`
	Hint    string     `json:"hint,omitempty"` // Suggestions for next actions
}

// handleTaskNext returns the next pending task from a plan
func handleTaskNext(repo *memory.Repository, params TaskNextParams) (*mcpsdk.CallToolResultFor[any], error) {
	var planID string

	// Determine which plan to use
	if params.PlanID != "" {
		planID = params.PlanID
	} else {
		// Get active plan
		activePlan, err := repo.GetActivePlan()
		if err != nil {
			return nil, fmt.Errorf("get active plan: %w", err)
		}
		if activePlan == nil {
			return mcpJSONResponse(TaskResponse{
				Success: false,
				Message: "No active plan found. Create and start a plan first with 'taskwing plan new' and 'taskwing plan start'.",
			})
		}
		planID = activePlan.ID
	}

	// Get next pending task
	nextTask, err := repo.GetNextTask(planID)
	if err != nil {
		return nil, fmt.Errorf("get next task: %w", err)
	}
	if nextTask == nil {
		return mcpJSONResponse(TaskResponse{
			Success: true,
			Message: "No pending tasks in this plan. All tasks may be completed or blocked.",
			Hint:    "Check plan status with 'taskwing plan list' or create new tasks.",
		})
	}

	// Auto-start if requested
	if params.AutoStart && params.SessionID != "" {
		if err := repo.ClaimTask(nextTask.ID, params.SessionID); err != nil {
			// Race condition: another session claimed this task between GetNextTask and ClaimTask
			// Return error so caller knows to retry
			return mcpJSONResponse(TaskResponse{
				Success: false,
				Message: fmt.Sprintf("Failed to claim task (may have been claimed by another session): %v", err),
				Hint:    "Call task_next again to get the next available task.",
			})
		}
		// Re-fetch the task to get accurate ClaimedAt timestamp set by ClaimTask
		claimedTask, err := repo.GetTask(nextTask.ID)
		if err != nil {
			return nil, fmt.Errorf("get claimed task: %w", err)
		}
		nextTask = claimedTask
	}

	hint := "Call recall tool with suggested queries for context before starting work."
	if len(nextTask.SuggestedRecallQueries) > 0 {
		hint = fmt.Sprintf("Call recall tool with queries: %v", nextTask.SuggestedRecallQueries)
	}

	return mcpJSONResponse(TaskResponse{
		Success: true,
		Task:    nextTask,
		Hint:    hint,
	})
}

// handleTaskCurrent returns the current in-progress task
func handleTaskCurrent(repo *memory.Repository, params TaskCurrentParams) (*mcpsdk.CallToolResultFor[any], error) {
	// Try to find by session ID first
	if params.SessionID != "" {
		currentTask, err := repo.GetCurrentTask(params.SessionID)
		if err != nil {
			return nil, fmt.Errorf("get current task by session: %w", err)
		}
		if currentTask != nil {
			return mcpJSONResponse(TaskResponse{
				Success: true,
				Task:    currentTask,
			})
		}
	}

	// Fallback: find any in-progress task in the plan
	var planID string
	if params.PlanID != "" {
		planID = params.PlanID
	} else {
		activePlan, err := repo.GetActivePlan()
		if err != nil {
			return nil, fmt.Errorf("get active plan: %w", err)
		}
		if activePlan == nil {
			return mcpJSONResponse(TaskResponse{
				Success: false,
				Message: "No active plan found.",
			})
		}
		planID = activePlan.ID
	}

	inProgressTask, err := repo.GetAnyInProgressTask(planID)
	if err != nil {
		return nil, fmt.Errorf("get in-progress task: %w", err)
	}
	if inProgressTask == nil {
		return mcpJSONResponse(TaskResponse{
			Success: true,
			Message: "No task currently in progress.",
			Hint:    "Use task_next to get the next pending task.",
		})
	}

	return mcpJSONResponse(TaskResponse{
		Success: true,
		Task:    inProgressTask,
		Message: "Found in-progress task (may be from a different session).",
	})
}

// handleTaskStart claims a specific task for a session
func handleTaskStart(repo *memory.Repository, params TaskStartParams) (*mcpsdk.CallToolResultFor[any], error) {
	if params.TaskID == "" {
		return mcpJSONResponse(TaskResponse{
			Success: false,
			Message: "task_id is required",
		})
	}
	if params.SessionID == "" {
		return mcpJSONResponse(TaskResponse{
			Success: false,
			Message: "session_id is required",
		})
	}

	// Claim the task
	if err := repo.ClaimTask(params.TaskID, params.SessionID); err != nil {
		return mcpJSONResponse(TaskResponse{
			Success: false,
			Message: err.Error(),
		})
	}

	// Return the updated task
	startedTask, err := repo.GetTask(params.TaskID)
	if err != nil {
		return nil, fmt.Errorf("get started task: %w", err)
	}

	hint := "Call recall tool with suggested queries for relevant context."
	if len(startedTask.SuggestedRecallQueries) > 0 {
		hint = fmt.Sprintf("Call recall tool with queries: %v", startedTask.SuggestedRecallQueries)
	}

	return mcpJSONResponse(TaskResponse{
		Success: true,
		Message: "Task started successfully.",
		Task:    startedTask,
		Hint:    hint,
	})
}

// handleTaskComplete marks a task as completed
func handleTaskComplete(repo *memory.Repository, params TaskCompleteParams) (*mcpsdk.CallToolResultFor[any], error) {
	if params.TaskID == "" {
		return mcpJSONResponse(TaskResponse{
			Success: false,
			Message: "task_id is required",
		})
	}

	// Complete the task
	if err := repo.CompleteTask(params.TaskID, params.Summary, params.FilesModified); err != nil {
		return mcpJSONResponse(TaskResponse{
			Success: false,
			Message: err.Error(),
		})
	}

	// Get the completed task
	completedTask, err := repo.GetTask(params.TaskID)
	if err != nil {
		return nil, fmt.Errorf("get completed task: %w", err)
	}

	// Check if there are more pending tasks
	plan, err := repo.GetPlan(completedTask.PlanID)
	if err != nil {
		return nil, fmt.Errorf("get plan: %w", err)
	}

	pendingCount := 0
	for _, t := range plan.Tasks {
		if t.Status == task.StatusPending {
			pendingCount++
		}
	}

	hint := "Great work! "
	if pendingCount > 0 {
		hint += fmt.Sprintf("There are %d more pending tasks. Use task_next to continue.", pendingCount)
	} else {
		hint += "All tasks in this plan are complete!"
	}

	return mcpJSONResponse(TaskResponse{
		Success: true,
		Message: "Task completed successfully.",
		Task:    completedTask,
		Hint:    hint,
	})
}

// handleTaskBlock marks a task as blocked
func handleTaskBlock(repo *memory.Repository, params TaskBlockParams) (*mcpsdk.CallToolResultFor[any], error) {
	if params.TaskID == "" {
		return mcpJSONResponse(TaskResponse{
			Success: false,
			Message: "task_id is required",
		})
	}
	if params.Reason == "" {
		return mcpJSONResponse(TaskResponse{
			Success: false,
			Message: "reason is required",
		})
	}

	// Block the task
	if err := repo.BlockTask(params.TaskID, params.Reason); err != nil {
		return mcpJSONResponse(TaskResponse{
			Success: false,
			Message: err.Error(),
		})
	}

	// Get the blocked task
	blockedTask, err := repo.GetTask(params.TaskID)
	if err != nil {
		return nil, fmt.Errorf("get blocked task: %w", err)
	}

	return mcpJSONResponse(TaskResponse{
		Success: true,
		Message: fmt.Sprintf("Task blocked. Reason: %s", params.Reason),
		Task:    blockedTask,
		Hint:    "Use task_next to work on a different task, or use task_unblock when the blocker is resolved.",
	})
}

// handleTaskUnblock unblocks a blocked task
func handleTaskUnblock(repo *memory.Repository, params TaskUnblockParams) (*mcpsdk.CallToolResultFor[any], error) {
	if params.TaskID == "" {
		return mcpJSONResponse(TaskResponse{
			Success: false,
			Message: "task_id is required",
		})
	}

	// Unblock the task
	if err := repo.UnblockTask(params.TaskID); err != nil {
		return mcpJSONResponse(TaskResponse{
			Success: false,
			Message: err.Error(),
		})
	}

	// Get the unblocked task
	unblockedTask, err := repo.GetTask(params.TaskID)
	if err != nil {
		return nil, fmt.Errorf("get unblocked task: %w", err)
	}

	return mcpJSONResponse(TaskResponse{
		Success: true,
		Message: "Task unblocked and returned to pending status.",
		Task:    unblockedTask,
		Hint:    "Use task_next or task_start to begin working on this task.",
	})
}

// === Plan Creation Handlers ===

// parseQuestionsFromMetadata extracts questions from agent metadata,
// handling both []string and []any (from JSON unmarshaling).
func parseQuestionsFromMetadata(metadata map[string]any) []string {
	// Try direct []string first
	if questions, ok := metadata["questions"].([]string); ok {
		return questions
	}
	// Handle []any from JSON unmarshaling
	if qAny, ok := metadata["questions"].([]any); ok {
		var questions []string
		for _, q := range qAny {
			if qs, ok := q.(string); ok {
				questions = append(questions, qs)
			}
		}
		return questions
	}
	return nil
}

// handlePlanClarify runs the ClarifyingAgent to refine a goal
func handlePlanClarify(ctx context.Context, repo *memory.Repository, params PlanClarifyParams) (*mcpsdk.CallToolResultFor[any], error) {
	if params.Goal == "" {
		return mcpJSONResponse(PlanClarifyResponse{
			Success: false,
			Message: "goal is required",
		})
	}

	// Load LLM config
	llmCfg, err := config.LoadLLMConfig()
	if err != nil {
		return mcpJSONResponse(PlanClarifyResponse{
			Success: false,
			Message: fmt.Sprintf("LLM config error: %v", err),
		})
	}

	// Fetch context from knowledge graph
	ks := knowledge.NewService(repo, llmCfg)
	contextStr := ""
	scored, err := ks.Search(ctx, params.Goal, 5)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[WARN] KG search failed for plan_clarify: %v\n", err)
	} else if len(scored) > 0 {
		var parts []string
		for _, sn := range scored {
			parts = append(parts, fmt.Sprintf("[%s] %s: %s", sn.Node.Type, sn.Node.Summary, sn.Node.Content))
		}
		contextStr = strings.Join(parts, "\n\n")
	}

	// Create and run ClarifyingAgent
	clarifyingAgent := planning.NewClarifyingAgent(llmCfg)
	defer func() { _ = clarifyingAgent.Close() }()

	input := core.Input{
		ExistingContext: map[string]any{
			"goal":    params.Goal,
			"history": params.History,
			"context": contextStr,
		},
	}

	output, err := clarifyingAgent.Run(ctx, input)
	if err != nil {
		return mcpJSONResponse(PlanClarifyResponse{
			Success: false,
			Message: fmt.Sprintf("Clarifying agent failed: %v", err),
		})
	}
	if output.Error != nil {
		return mcpJSONResponse(PlanClarifyResponse{
			Success: false,
			Message: fmt.Sprintf("Clarifying agent error: %v", output.Error),
		})
	}

	// Parse agent output
	if len(output.Findings) == 0 {
		return mcpJSONResponse(PlanClarifyResponse{
			Success: false,
			Message: "No findings from clarifying agent",
		})
	}

	finding := output.Findings[0]
	questions := parseQuestionsFromMetadata(finding.Metadata)
	goalSummary, _ := finding.Metadata["goal_summary"].(string)
	enrichedGoal, _ := finding.Metadata["enriched_goal"].(string)
	isReady, _ := finding.Metadata["is_ready_to_plan"].(bool)

	// If auto_answer and we have questions, try to answer them
	if params.AutoAnswer && len(questions) > 0 && !isReady {
		answer, err := clarifyingAgent.AutoAnswer(ctx, enrichedGoal, questions, contextStr)
		if err == nil && answer != "" {
			enrichedGoal = answer
			// Re-run to check if now ready
			input.ExistingContext["history"] = fmt.Sprintf("%s\n\nAuto-answered questions with:\n%s", params.History, answer)
			output2, err := clarifyingAgent.Run(ctx, input)
			if err == nil && len(output2.Findings) > 0 {
				finding2 := output2.Findings[0]
				questions = parseQuestionsFromMetadata(finding2.Metadata)
				goalSummary, _ = finding2.Metadata["goal_summary"].(string)
				enrichedGoal, _ = finding2.Metadata["enriched_goal"].(string)
				isReady, _ = finding2.Metadata["is_ready_to_plan"].(bool)
			}
		}
	}

	contextSummary := ""
	if len(scored) > 0 {
		contextSummary = fmt.Sprintf("Retrieved %d relevant nodes from knowledge graph", len(scored))
	}

	return mcpJSONResponse(PlanClarifyResponse{
		Success:       true,
		Questions:     questions,
		GoalSummary:   goalSummary,
		EnrichedGoal:  enrichedGoal,
		IsReadyToPlan: isReady,
		ContextUsed:   contextSummary,
	})
}

// handlePlanGenerate runs the PlanningAgent to create tasks
func handlePlanGenerate(ctx context.Context, repo *memory.Repository, params PlanGenerateParams) (*mcpsdk.CallToolResultFor[any], error) {
	if params.Goal == "" {
		return mcpJSONResponse(PlanGenerateResponse{
			Success: false,
			Message: "goal is required",
		})
	}
	if params.EnrichedGoal == "" {
		return mcpJSONResponse(PlanGenerateResponse{
			Success: false,
			Message: "enriched_goal is required (run plan_clarify first)",
		})
	}

	// Load LLM config
	llmCfg, err := config.LoadLLMConfig()
	if err != nil {
		return mcpJSONResponse(PlanGenerateResponse{
			Success: false,
			Message: fmt.Sprintf("LLM config error: %v", err),
		})
	}

	// Fetch context from knowledge graph
	ks := knowledge.NewService(repo, llmCfg)
	contextStr := ""
	scored, err := ks.Search(ctx, params.EnrichedGoal, 5)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[WARN] KG search failed for plan_generate: %v\n", err)
	} else if len(scored) > 0 {
		var parts []string
		for _, sn := range scored {
			parts = append(parts, fmt.Sprintf("[%s] %s: %s", sn.Node.Type, sn.Node.Summary, sn.Node.Content))
		}
		contextStr = strings.Join(parts, "\n\n")
	}

	// Create and run PlanningAgent
	planningAgent := planning.NewPlanningAgent(llmCfg)
	defer func() { _ = planningAgent.Close() }()

	input := core.Input{
		ExistingContext: map[string]any{
			"goal":          params.Goal,
			"enriched_goal": params.EnrichedGoal,
			"context":       contextStr,
		},
	}

	output, err := planningAgent.Run(ctx, input)
	if err != nil {
		return mcpJSONResponse(PlanGenerateResponse{
			Success: false,
			Message: fmt.Sprintf("Planning agent failed: %v", err),
		})
	}
	if output.Error != nil {
		return mcpJSONResponse(PlanGenerateResponse{
			Success: false,
			Message: fmt.Sprintf("Planning agent error: %v", output.Error),
		})
	}

	// Parse tasks from output
	if len(output.Findings) == 0 {
		return mcpJSONResponse(PlanGenerateResponse{
			Success: false,
			Message: "No findings from planning agent",
		})
	}

	finding := output.Findings[0]
	tasksRaw, _ := finding.Metadata["tasks"].([]planning.PlanningTask)

	// Handle tasks as []any (JSON unmarshaling)
	var tasks []task.Task
	if tasksRaw != nil {
		for _, pt := range tasksRaw {
			t := task.Task{
				Title:              pt.Title,
				Description:        pt.Description,
				AcceptanceCriteria: pt.AcceptanceCriteria,
				ValidationSteps:    pt.ValidationSteps,
				Priority:           pt.Priority,
				Status:             task.StatusPending,
				AssignedAgent:      pt.AssignedAgent,
			}
			t.EnrichAIFields() // Populate Scope, Keywords, SuggestedRecallQueries
			tasks = append(tasks, t)
		}
	} else if tasksAny, ok := finding.Metadata["tasks"].([]any); ok {
		for _, t := range tasksAny {
			if tm, ok := t.(map[string]any); ok {
				title, _ := tm["title"].(string)
				desc, _ := tm["description"].(string)
				priority, _ := tm["priority"].(float64)
				agent, _ := tm["assigned_agent"].(string)

				var criteria []string
				if ac, ok := tm["acceptance_criteria"].([]any); ok {
					for _, c := range ac {
						if cs, ok := c.(string); ok {
							criteria = append(criteria, cs)
						}
					}
				}

				var validation []string
				if vs, ok := tm["validation_steps"].([]any); ok {
					for _, v := range vs {
						if vs, ok := v.(string); ok {
							validation = append(validation, vs)
						}
					}
				}

				t := task.Task{
					Title:              title,
					Description:        desc,
					AcceptanceCriteria: criteria,
					ValidationSteps:    validation,
					Priority:           int(priority),
					Status:             task.StatusPending,
					AssignedAgent:      agent,
				}
				t.EnrichAIFields() // Populate Scope, Keywords, SuggestedRecallQueries
				tasks = append(tasks, t)
			}
		}
	}

	if len(tasks) == 0 {
		return mcpJSONResponse(PlanGenerateResponse{
			Success: false,
			Message: "No tasks generated",
		})
	}

	// Always save the plan. The Save param in PlanGenerateParams is reserved
	// for future use but currently ignored because JSON-RPC in Go cannot
	// distinguish \"save: false\" from \"save field omitted\".

	var planID string
	{
		// Create plan
		plan := &task.Plan{
			Goal:         params.Goal,
			EnrichedGoal: params.EnrichedGoal,
			Status:       task.PlanStatusActive,
			Tasks:        tasks,
		}

		if err := repo.CreatePlan(plan); err != nil {
			return mcpJSONResponse(PlanGenerateResponse{
				Success: false,
				Message: fmt.Sprintf("Failed to save plan: %v", err),
			})
		}
		planID = plan.ID

		// Set as active plan (using the helper from plan.go)
		if err := setActivePlan(planID); err != nil {
			fmt.Fprintf(os.Stderr, "[WARN] Failed to set active plan: %v\n", err)
		}
	}

	return mcpJSONResponse(PlanGenerateResponse{
		Success:      true,
		PlanID:       planID,
		Goal:         params.Goal,
		EnrichedGoal: params.EnrichedGoal,
		Tasks:        tasks,
		Hint:         "Use task_next to begin working on the first task.",
	})
}
