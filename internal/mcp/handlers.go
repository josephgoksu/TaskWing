// Package mcp provides handlers for unified MCP tools.
package mcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/josephgoksu/TaskWing/internal/app"
	"github.com/josephgoksu/TaskWing/internal/codeintel"
	"github.com/josephgoksu/TaskWing/internal/config"
	"github.com/josephgoksu/TaskWing/internal/llm"
	"github.com/josephgoksu/TaskWing/internal/memory"
)

// CodeToolResult represents the response from the unified code tool.
type CodeToolResult struct {
	Action  string `json:"action"`
	Content string `json:"content"`
	Error   string `json:"error,omitempty"`
}

// HandleCodeTool is the unified handler for all code intelligence operations.
// It routes to the appropriate service logic based on the action parameter.
// Consolidates: find_symbol, semantic_search_code, explain_symbol, get_callers, analyze_impact
func HandleCodeTool(ctx context.Context, repo *memory.Repository, params CodeToolParams) (*CodeToolResult, error) {
	// Validate action
	if !params.Action.IsValid() {
		return &CodeToolResult{
			Action: string(params.Action),
			Error:  fmt.Sprintf("invalid action %q, must be one of: find, search, explain, callers, impact", params.Action),
		}, nil
	}

	switch params.Action {
	case CodeActionFind:
		return handleCodeFind(ctx, repo, params)
	case CodeActionSearch:
		return handleCodeSearch(ctx, repo, params)
	case CodeActionExplain:
		return handleCodeExplain(ctx, repo, params)
	case CodeActionCallers:
		return handleCodeCallers(ctx, repo, params)
	case CodeActionImpact:
		return handleCodeImpact(ctx, repo, params)
	default:
		// This should never happen due to IsValid() check above
		return &CodeToolResult{
			Action: string(params.Action),
			Error:  fmt.Sprintf("unsupported action: %s", params.Action),
		}, nil
	}
}

// handleCodeFind implements the 'find' action - locate symbols by name, ID, or file.
func handleCodeFind(ctx context.Context, repo *memory.Repository, params CodeToolParams) (*CodeToolResult, error) {
	appCtx := app.NewContext(repo)
	codeIntelApp := app.NewCodeIntelApp(appCtx)

	result, err := codeIntelApp.FindSymbol(ctx, app.FindSymbolOptions{
		Name:     params.Query,
		ID:       params.SymbolID,
		FilePath: params.FilePath,
		Language: params.Language,
	})
	if err != nil {
		return &CodeToolResult{
			Action: "find",
			Error:  err.Error(),
		}, nil
	}

	return &CodeToolResult{
		Action:  "find",
		Content: FormatSymbolList(result.Symbols),
	}, nil
}

// handleCodeSearch implements the 'search' action - hybrid semantic + lexical search.
func handleCodeSearch(ctx context.Context, repo *memory.Repository, params CodeToolParams) (*CodeToolResult, error) {
	// Input validation
	query := strings.TrimSpace(params.Query)
	if query == "" {
		return &CodeToolResult{
			Action: "search",
			Error:  "query is required for search action",
		}, nil
	}

	// Limit query length to prevent abuse
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
		return &CodeToolResult{
			Action: "search",
			Error:  err.Error(),
		}, nil
	}

	return &CodeToolResult{
		Action:  "search",
		Content: FormatSearchResults(result.Results),
	}, nil
}

// handleCodeExplain implements the 'explain' action - deep dive into a symbol.
func handleCodeExplain(ctx context.Context, repo *memory.Repository, params CodeToolParams) (*CodeToolResult, error) {
	// Input validation
	query := strings.TrimSpace(params.Query)
	if params.SymbolID == 0 && query == "" {
		return &CodeToolResult{
			Action: "explain",
			Error:  "query or symbol_id is required for explain action",
		}, nil
	}

	// Clamp depth to valid range
	depth := params.Depth
	if depth <= 0 {
		depth = 2 // default
	}
	const maxDepth = 5
	if depth > maxDepth {
		depth = maxDepth
	}

	// Get base path for source code fetching
	basePath, _ := config.GetProjectRoot()

	appCtx := app.NewContextForRole(repo, llm.RoleQuery)
	appCtx.BasePath = basePath
	explainApp := app.NewExplainApp(appCtx)

	result, err := explainApp.Explain(ctx, app.ExplainRequest{
		Query:       query,
		SymbolID:    params.SymbolID,
		Depth:       depth,
		IncludeCode: true, // Always include for MCP context
	})
	if err != nil {
		return &CodeToolResult{
			Action: "explain",
			Error:  err.Error(),
		}, nil
	}

	return &CodeToolResult{
		Action:  "explain",
		Content: FormatExplainResult(result),
	}, nil
}

// handleCodeCallers implements the 'callers' action - get call graph relationships.
func handleCodeCallers(ctx context.Context, repo *memory.Repository, params CodeToolParams) (*CodeToolResult, error) {
	// Input validation - need either symbol_id or query (as symbol name)
	symbolName := strings.TrimSpace(params.Query)
	if params.SymbolID == 0 && symbolName == "" {
		return &CodeToolResult{
			Action: "callers",
			Error:  "symbol_id or query (symbol name) is required for callers action",
		}, nil
	}

	appCtx := app.NewContext(repo)
	codeIntelApp := app.NewCodeIntelApp(appCtx)

	result, err := codeIntelApp.GetCallers(ctx, app.GetCallersOptions{
		SymbolID:   params.SymbolID,
		SymbolName: symbolName,
		Direction:  params.Direction,
	})
	if err != nil {
		return &CodeToolResult{
			Action: "callers",
			Error:  err.Error(),
		}, nil
	}

	return &CodeToolResult{
		Action:  "callers",
		Content: FormatCallers(result),
	}, nil
}

// handleCodeImpact implements the 'impact' action - analyze change impact.
func handleCodeImpact(ctx context.Context, repo *memory.Repository, params CodeToolParams) (*CodeToolResult, error) {
	// Input validation
	symbolName := strings.TrimSpace(params.Query)
	if params.SymbolID == 0 && symbolName == "" {
		return &CodeToolResult{
			Action: "impact",
			Error:  "symbol_id or query (symbol name) is required for impact action",
		}, nil
	}

	// Clamp max_depth to prevent deep recursion
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
		return &CodeToolResult{
			Action: "impact",
			Error:  err.Error(),
		}, nil
	}

	return &CodeToolResult{
		Action:  "impact",
		Content: FormatImpact(result),
	}, nil
}

// === Task Tool Handler ===

// TaskToolResult represents the response from the unified task tool.
type TaskToolResult struct {
	Action  string `json:"action"`
	Content string `json:"content"`
	Error   string `json:"error,omitempty"`
}

// HandleTaskTool is the unified handler for all task lifecycle operations.
// It routes to the appropriate service logic based on the action parameter.
// Consolidates: task_next, task_current, task_start, task_complete
func HandleTaskTool(ctx context.Context, repo *memory.Repository, params TaskToolParams) (*TaskToolResult, error) {
	// Validate action
	if !params.Action.IsValid() {
		return &TaskToolResult{
			Action: string(params.Action),
			Error:  fmt.Sprintf("invalid action %q, must be one of: next, current, start, complete", params.Action),
		}, nil
	}

	switch params.Action {
	case TaskActionNext:
		return handleTaskNext(ctx, repo, params)
	case TaskActionCurrent:
		return handleTaskCurrent(ctx, repo, params)
	case TaskActionStart:
		return handleTaskStart(ctx, repo, params)
	case TaskActionComplete:
		return handleTaskComplete(ctx, repo, params)
	default:
		return &TaskToolResult{
			Action: string(params.Action),
			Error:  fmt.Sprintf("unsupported action: %s", params.Action),
		}, nil
	}
}

// handleTaskNext implements the 'next' action - get the next pending task.
func handleTaskNext(ctx context.Context, repo *memory.Repository, params TaskToolParams) (*TaskToolResult, error) {
	appCtx := app.NewContext(repo)
	taskApp := app.NewTaskApp(appCtx)

	result, err := taskApp.Next(ctx, app.TaskNextOptions{
		PlanID:            params.PlanID,
		SessionID:         params.SessionID,
		AutoStart:         params.AutoStart,
		CreateBranch:      params.CreateBranch,
		SkipUnpushedCheck: params.SkipUnpushedCheck,
	})
	if err != nil {
		return &TaskToolResult{
			Action: "next",
			Error:  err.Error(),
		}, nil
	}

	return &TaskToolResult{
		Action:  "next",
		Content: FormatTask(result),
	}, nil
}

// handleTaskCurrent implements the 'current' action - get the current in-progress task.
func handleTaskCurrent(ctx context.Context, repo *memory.Repository, params TaskToolParams) (*TaskToolResult, error) {
	appCtx := app.NewContext(repo)
	taskApp := app.NewTaskApp(appCtx)

	result, err := taskApp.Current(ctx, params.SessionID, params.PlanID)
	if err != nil {
		return &TaskToolResult{
			Action: "current",
			Error:  err.Error(),
		}, nil
	}

	return &TaskToolResult{
		Action:  "current",
		Content: FormatTask(result),
	}, nil
}

// handleTaskStart implements the 'start' action - claim a specific task.
func handleTaskStart(ctx context.Context, repo *memory.Repository, params TaskToolParams) (*TaskToolResult, error) {
	// Validate required fields
	taskID := strings.TrimSpace(params.TaskID)
	if taskID == "" {
		return &TaskToolResult{
			Action: "start",
			Error:  "task_id is required for start action",
		}, nil
	}

	sessionID := strings.TrimSpace(params.SessionID)
	if sessionID == "" {
		return &TaskToolResult{
			Action: "start",
			Error:  "session_id is required for start action",
		}, nil
	}

	appCtx := app.NewContext(repo)
	taskApp := app.NewTaskApp(appCtx)

	result, err := taskApp.Start(ctx, app.TaskStartOptions{
		TaskID:    taskID,
		SessionID: sessionID,
	})
	if err != nil {
		return &TaskToolResult{
			Action: "start",
			Error:  err.Error(),
		}, nil
	}

	return &TaskToolResult{
		Action:  "start",
		Content: FormatTask(result),
	}, nil
}

// handleTaskComplete implements the 'complete' action - mark a task as done.
func handleTaskComplete(ctx context.Context, repo *memory.Repository, params TaskToolParams) (*TaskToolResult, error) {
	// Validate required fields
	taskID := strings.TrimSpace(params.TaskID)
	if taskID == "" {
		return &TaskToolResult{
			Action: "complete",
			Error:  "task_id is required for complete action",
		}, nil
	}

	// Use RoleBootstrap for audit operations triggered on plan completion
	appCtx := app.NewContextForRole(repo, llm.RoleBootstrap)
	taskApp := app.NewTaskApp(appCtx)

	result, err := taskApp.Complete(ctx, app.TaskCompleteOptions{
		TaskID:        taskID,
		Summary:       params.Summary,
		FilesModified: params.FilesModified,
	})
	if err != nil {
		return &TaskToolResult{
			Action: "complete",
			Error:  err.Error(),
		}, nil
	}

	return &TaskToolResult{
		Action:  "complete",
		Content: FormatTask(result),
	}, nil
}

// === Plan Tool Handler ===

// PlanToolResult represents the response from the unified plan tool.
type PlanToolResult struct {
	Action  string `json:"action"`
	Content string `json:"content"`
	Error   string `json:"error,omitempty"`
}

// HandlePlanTool is the unified handler for all plan operations.
// It routes to the appropriate service logic based on the action parameter.
// Consolidates: plan_clarify, plan_generate
func HandlePlanTool(ctx context.Context, repo *memory.Repository, params PlanToolParams) (*PlanToolResult, error) {
	// Validate action
	if !params.Action.IsValid() {
		return &PlanToolResult{
			Action: string(params.Action),
			Error:  fmt.Sprintf("invalid action %q, must be one of: clarify, generate", params.Action),
		}, nil
	}

	switch params.Action {
	case PlanActionClarify:
		return handlePlanClarify(ctx, repo, params)
	case PlanActionGenerate:
		return handlePlanGenerate(ctx, repo, params)
	default:
		return &PlanToolResult{
			Action: string(params.Action),
			Error:  fmt.Sprintf("unsupported action: %s", params.Action),
		}, nil
	}
}

// handlePlanClarify implements the 'clarify' action - refine a goal with questions.
func handlePlanClarify(ctx context.Context, repo *memory.Repository, params PlanToolParams) (*PlanToolResult, error) {
	// Validate required fields
	goal := strings.TrimSpace(params.Goal)
	if goal == "" {
		return &PlanToolResult{
			Action: "clarify",
			Error:  "goal is required for clarify action",
		}, nil
	}

	// Use RoleBootstrap for planning operations
	appCtx := app.NewContextForRole(repo, llm.RoleBootstrap)
	planApp := app.NewPlanApp(appCtx)

	result, err := planApp.Clarify(ctx, app.ClarifyOptions{
		Goal:       goal,
		History:    params.History,
		AutoAnswer: params.AutoAnswer,
	})
	if err != nil {
		return &PlanToolResult{
			Action: "clarify",
			Error:  err.Error(),
		}, nil
	}

	return &PlanToolResult{
		Action:  "clarify",
		Content: FormatClarifyResult(result),
	}, nil
}

// handlePlanGenerate implements the 'generate' action - create a plan with tasks.
func handlePlanGenerate(ctx context.Context, repo *memory.Repository, params PlanToolParams) (*PlanToolResult, error) {
	// Validate required fields
	goal := strings.TrimSpace(params.Goal)
	if goal == "" {
		return &PlanToolResult{
			Action: "generate",
			Error:  "goal is required for generate action",
		}, nil
	}
	enrichedGoal := strings.TrimSpace(params.EnrichedGoal)
	if enrichedGoal == "" {
		return &PlanToolResult{
			Action: "generate",
			Error:  "enriched_goal is required for generate action",
		}, nil
	}

	// Default save to true
	save := true
	if params.Save != nil {
		save = *params.Save
	}

	// Use RoleBootstrap for planning operations
	appCtx := app.NewContextForRole(repo, llm.RoleBootstrap)
	planApp := app.NewPlanApp(appCtx)

	result, err := planApp.Generate(ctx, app.GenerateOptions{
		Goal:         goal,
		EnrichedGoal: enrichedGoal,
		Save:         save,
	})
	if err != nil {
		return &PlanToolResult{
			Action: "generate",
			Error:  err.Error(),
		}, nil
	}

	return &PlanToolResult{
		Action:  "generate",
		Content: FormatGenerateResult(result),
	}, nil
}
