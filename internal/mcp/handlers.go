// Package mcp provides handlers for unified MCP tools.
package mcp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	agentcore "github.com/josephgoksu/TaskWing/internal/agents/core"
	agentimpl "github.com/josephgoksu/TaskWing/internal/agents/impl"
	"github.com/josephgoksu/TaskWing/internal/app"
	"github.com/josephgoksu/TaskWing/internal/codeintel"
	"github.com/josephgoksu/TaskWing/internal/config"
	"github.com/josephgoksu/TaskWing/internal/llm"
	"github.com/josephgoksu/TaskWing/internal/memory"
	"github.com/josephgoksu/TaskWing/internal/policy"
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
			Error:  fmt.Sprintf("invalid action %q, must be one of: find, search, explain, callers, impact, simplify", params.Action),
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
	case CodeActionSimplify:
		return handleCodeSimplify(ctx, repo, params)
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

// handleCodeSimplify implements the 'simplify' action - reduce code complexity.
func handleCodeSimplify(ctx context.Context, repo *memory.Repository, params CodeToolParams) (*CodeToolResult, error) {
	// Input validation: need either file_path or code
	code := strings.TrimSpace(params.Code)
	filePath := strings.TrimSpace(params.FilePath)

	if code == "" && filePath == "" {
		return &CodeToolResult{
			Action: "simplify",
			Error:  "either code or file_path is required for simplify action",
		}, nil
	}

	// If file_path provided, read the file content
	if filePath != "" && code == "" {
		projectRoot, _ := config.GetProjectRoot()

		// Validate path to prevent traversal attacks
		resolvedPath, err := validateAndResolvePath(filePath, projectRoot)
		if err != nil {
			return &CodeToolResult{
				Action: "simplify",
				Error:  fmt.Sprintf("invalid file path: %v", err),
			}, nil
		}

		content, err := readFileContent(resolvedPath)
		if err != nil {
			return &CodeToolResult{
				Action: "simplify",
				Error:  fmt.Sprintf("failed to read file %s: %v", filePath, err),
			}, nil
		}
		code = content
	}

	// Get architectural context for better simplification
	appCtx := app.NewContextForRole(repo, llm.RoleQuery)
	recallApp := app.NewRecallApp(appCtx)

	var kgContext string
	if filePath != "" {
		result, err := recallApp.Query(ctx, "patterns and constraints for "+filePath, app.RecallOptions{
			Limit:          3,
			GenerateAnswer: false,
		})
		if err == nil && result != nil {
			kgContext = formatRecallContext(result)
		}
	}

	// Create and run the SimplifyAgent
	llmCfg, err := config.LoadLLMConfigForRole(llm.RoleQuery)
	if err != nil {
		return &CodeToolResult{
			Action: "simplify",
			Error:  fmt.Sprintf("failed to load LLM config: %v", err),
		}, nil
	}
	agent := agentimpl.NewSimplifyAgent(llmCfg)
	defer func() { _ = agent.Close() }()

	input := agentcore.Input{
		ExistingContext: map[string]any{
			"code":      code,
			"file_path": filePath,
			"context":   kgContext,
		},
	}

	output, err := agent.Run(ctx, input)
	if err != nil {
		return &CodeToolResult{
			Action: "simplify",
			Error:  fmt.Sprintf("agent error: %v", err),
		}, nil
	}

	if output.Error != nil {
		return &CodeToolResult{
			Action: "simplify",
			Error:  output.Error.Error(),
		}, nil
	}

	// Format the output
	return &CodeToolResult{
		Action:  "simplify",
		Content: FormatSimplifyResult(output.Findings),
	}, nil
}

// readFileContent reads file content from disk.
func readFileContent(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

// validateAndResolvePath validates a file path to prevent path traversal attacks.
// Returns the resolved absolute path if valid, or an error if the path is unsafe.
func validateAndResolvePath(requestedPath string, projectRoot string) (string, error) {
	// Clean the path to normalize it
	cleanPath := filepath.Clean(requestedPath)

	// Reject paths with explicit traversal attempts
	if strings.Contains(cleanPath, "..") {
		return "", fmt.Errorf("path traversal not allowed: %s", requestedPath)
	}

	// Resolve to absolute path
	var absPath string
	if filepath.IsAbs(cleanPath) {
		absPath = cleanPath
	} else {
		if projectRoot == "" {
			return "", fmt.Errorf("cannot resolve relative path without project root")
		}
		absPath = filepath.Join(projectRoot, cleanPath)
	}

	// Ensure the resolved path is within the project root
	if projectRoot != "" {
		absProjectRoot, err := filepath.Abs(projectRoot)
		if err != nil {
			return "", fmt.Errorf("failed to resolve project root: %w", err)
		}
		absPath, err = filepath.Abs(absPath)
		if err != nil {
			return "", fmt.Errorf("failed to resolve path: %w", err)
		}

		// Check that the path starts with the project root
		if !strings.HasPrefix(absPath, absProjectRoot+string(filepath.Separator)) &&
			absPath != absProjectRoot {
			return "", fmt.Errorf("path outside project root not allowed: %s", requestedPath)
		}
	}

	// Verify the file exists and is a regular file
	info, err := os.Stat(absPath)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return "", fmt.Errorf("path is a directory, not a file: %s", requestedPath)
	}

	return absPath, nil
}

// formatRecallContext formats recall results for agent context.
func formatRecallContext(result *app.RecallResult) string {
	if result == nil || len(result.Results) == 0 {
		return ""
	}
	var sb strings.Builder
	for _, r := range result.Results {
		sb.WriteString(fmt.Sprintf("- %s: %s\n", r.Type, r.ID))
		if r.Summary != "" {
			sb.WriteString(fmt.Sprintf("  %s\n", r.Summary))
		}
	}
	return sb.String()
}

// === Debug Tool Handler ===

// DebugToolResult represents the response from the debug tool.
type DebugToolResult struct {
	Content string `json:"content"`
	Error   string `json:"error,omitempty"`
}

// HandleDebugTool helps diagnose issues using the DebugAgent.
func HandleDebugTool(ctx context.Context, repo *memory.Repository, params DebugToolParams) (*DebugToolResult, error) {
	// Validate required fields
	problem := strings.TrimSpace(params.Problem)
	if problem == "" {
		return &DebugToolResult{
			Error: "problem description is required",
		}, nil
	}

	// Get architectural context for better diagnosis
	appCtx := app.NewContextForRole(repo, llm.RoleQuery)
	recallApp := app.NewRecallApp(appCtx)

	var kgContext string
	// Build context query from problem and file path
	contextQuery := problem
	if params.FilePath != "" {
		contextQuery = params.FilePath + " " + problem
	}

	result, err := recallApp.Query(ctx, contextQuery, app.RecallOptions{
		Limit:          5,
		GenerateAnswer: false,
	})
	if err == nil && result != nil {
		kgContext = formatRecallContext(result)
	}

	// Create and run the DebugAgent
	llmCfg, err := config.LoadLLMConfigForRole(llm.RoleQuery)
	if err != nil {
		return &DebugToolResult{
			Error: fmt.Sprintf("failed to load LLM config: %v", err),
		}, nil
	}
	agent := agentimpl.NewDebugAgent(llmCfg)
	defer func() { _ = agent.Close() }()

	input := agentcore.Input{
		ExistingContext: map[string]any{
			"problem":     problem,
			"error":       params.Error,
			"stack_trace": params.StackTrace,
			"context":     kgContext,
		},
	}

	output, err := agent.Run(ctx, input)
	if err != nil {
		return &DebugToolResult{
			Error: fmt.Sprintf("agent error: %v", err),
		}, nil
	}

	if output.Error != nil {
		return &DebugToolResult{
			Error: output.Error.Error(),
		}, nil
	}

	// Format the output
	return &DebugToolResult{
		Content: FormatDebugResult(output.Findings),
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

	// Default CreateBranch to true if not explicitly provided
	createBranch := true
	if params.CreateBranch != nil {
		createBranch = *params.CreateBranch
	}

	result, err := taskApp.Next(ctx, app.TaskNextOptions{
		PlanID:            params.PlanID,
		SessionID:         params.SessionID,
		AutoStart:         params.AutoStart,
		CreateBranch:      createBranch,
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
// Consolidates: plan_clarify, plan_generate, audit_plan
func HandlePlanTool(ctx context.Context, repo *memory.Repository, params PlanToolParams) (*PlanToolResult, error) {
	// Validate action
	if !params.Action.IsValid() {
		return &PlanToolResult{
			Action: string(params.Action),
			Error:  fmt.Sprintf("invalid action %q, must be one of: clarify, generate, audit", params.Action),
		}, nil
	}

	switch params.Action {
	case PlanActionClarify:
		return handlePlanClarify(ctx, repo, params)
	case PlanActionGenerate:
		return handlePlanGenerate(ctx, repo, params)
	case PlanActionAudit:
		return handlePlanAudit(ctx, repo, params)
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

// handlePlanAudit implements the 'audit' action - verify and fix a completed plan.
func handlePlanAudit(ctx context.Context, repo *memory.Repository, params PlanToolParams) (*PlanToolResult, error) {
	// Default autoFix to true
	autoFix := true
	if params.AutoFix != nil {
		autoFix = *params.AutoFix
	}

	// Use RoleBootstrap for audit operations
	appCtx := app.NewContextForRole(repo, llm.RoleBootstrap)
	planApp := app.NewPlanApp(appCtx)

	result, err := planApp.Audit(ctx, app.AuditOptions{
		PlanID:  params.PlanID,
		AutoFix: autoFix,
	})
	if err != nil {
		return &PlanToolResult{
			Action: "audit",
			Error:  err.Error(),
		}, nil
	}

	return &PlanToolResult{
		Action:  "audit",
		Content: FormatAuditResult(result),
	}, nil
}

// === Policy Tool Handler ===

// PolicyToolResult represents the response from the unified policy tool.
type PolicyToolResult struct {
	Action  string `json:"action"`
	Content string `json:"content"`
	Error   string `json:"error,omitempty"`
}

// HandlePolicyTool is the unified handler for all policy operations.
// It routes to the appropriate service logic based on the action parameter.
// Consolidates: policy check, policy list, policy explain
func HandlePolicyTool(ctx context.Context, params PolicyToolParams) (*PolicyToolResult, error) {
	// Validate action
	if !params.Action.IsValid() {
		return &PolicyToolResult{
			Action: string(params.Action),
			Error:  fmt.Sprintf("invalid action %q, must be one of: check, list, explain", params.Action),
		}, nil
	}

	switch params.Action {
	case PolicyActionCheck:
		return handlePolicyCheck(ctx, params)
	case PolicyActionList:
		return handlePolicyList(ctx, params)
	case PolicyActionExplain:
		return handlePolicyExplain(ctx, params)
	default:
		return &PolicyToolResult{
			Action: string(params.Action),
			Error:  fmt.Sprintf("unsupported action: %s", params.Action),
		}, nil
	}
}

// handlePolicyCheck implements the 'check' action - evaluate files against policies.
func handlePolicyCheck(ctx context.Context, params PolicyToolParams) (*PolicyToolResult, error) {
	// Validate required fields
	if len(params.Files) == 0 {
		return &PolicyToolResult{
			Action: "check",
			Error:  "files is required for check action",
		}, nil
	}

	// Get project root for policy engine
	projectRoot, err := config.GetProjectRoot()
	if err != nil {
		return &PolicyToolResult{
			Action: "check",
			Error:  fmt.Sprintf("get project root: %v", err),
		}, nil
	}

	// Create policy engine
	engine, err := policy.NewEngine(policy.EngineConfig{
		WorkDir: projectRoot,
	})
	if err != nil {
		return &PolicyToolResult{
			Action: "check",
			Error:  fmt.Sprintf("create policy engine: %v", err),
		}, nil
	}

	if engine.PolicyCount() == 0 {
		return &PolicyToolResult{
			Action:  "check",
			Content: "No policies loaded. Run 'tw policy init' to create the default policy.",
		}, nil
	}

	// Build policy input with context
	inputBuilder := policy.NewContextBuilder(projectRoot).
		WithTask(params.TaskID, params.TaskTitle).
		WithTaskFiles(params.Files, nil).
		WithPlan(params.PlanID, params.PlanGoal)

	// Evaluate against policies
	decision, err := engine.Evaluate(ctx, inputBuilder.Build())
	if err != nil {
		return &PolicyToolResult{
			Action: "check",
			Error:  fmt.Sprintf("evaluate policies: %v", err),
		}, nil
	}

	// Format the result
	return &PolicyToolResult{
		Action:  "check",
		Content: FormatPolicyCheckResult(decision, params.Files),
	}, nil
}

// handlePolicyList implements the 'list' action - list loaded policies.
func handlePolicyList(ctx context.Context, params PolicyToolParams) (*PolicyToolResult, error) {
	projectRoot, err := config.GetProjectRoot()
	if err != nil {
		return &PolicyToolResult{
			Action: "list",
			Error:  fmt.Sprintf("get project root: %v", err),
		}, nil
	}

	policiesDir := policy.GetPoliciesPath(projectRoot)
	loader := policy.NewOsLoader(policiesDir)

	policies, err := loader.LoadAll()
	if err != nil {
		return &PolicyToolResult{
			Action: "list",
			Error:  fmt.Sprintf("load policies: %v", err),
		}, nil
	}

	return &PolicyToolResult{
		Action:  "list",
		Content: FormatPolicyList(policies, policiesDir),
	}, nil
}

// handlePolicyExplain implements the 'explain' action - explain policy rules.
func handlePolicyExplain(ctx context.Context, params PolicyToolParams) (*PolicyToolResult, error) {
	projectRoot, err := config.GetProjectRoot()
	if err != nil {
		return &PolicyToolResult{
			Action: "explain",
			Error:  fmt.Sprintf("get project root: %v", err),
		}, nil
	}

	policiesDir := policy.GetPoliciesPath(projectRoot)
	loader := policy.NewOsLoader(policiesDir)

	policies, err := loader.LoadAll()
	if err != nil {
		return &PolicyToolResult{
			Action: "explain",
			Error:  fmt.Sprintf("load policies: %v", err),
		}, nil
	}

	if len(policies) == 0 {
		return &PolicyToolResult{
			Action:  "explain",
			Content: "No policies loaded. Run 'tw policy init' to create the default policy.",
		}, nil
	}

	// If a specific policy name is provided, filter to that one
	if params.PolicyName != "" {
		for _, p := range policies {
			if p.Name == params.PolicyName {
				return &PolicyToolResult{
					Action:  "explain",
					Content: FormatPolicyExplain(p),
				}, nil
			}
		}
		return &PolicyToolResult{
			Action: "explain",
			Error:  fmt.Sprintf("policy not found: %s", params.PolicyName),
		}, nil
	}

	// Explain all policies
	return &PolicyToolResult{
		Action:  "explain",
		Content: FormatPoliciesExplain(policies),
	}, nil
}
