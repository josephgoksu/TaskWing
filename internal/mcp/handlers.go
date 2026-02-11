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
// Supports lifecycle actions: next, current, start, complete
func HandleTaskTool(ctx context.Context, repo *memory.Repository, params TaskToolParams, defaultSessionID string) (*TaskToolResult, error) {
	// Validate action
	if !params.Action.IsValid() {
		return &TaskToolResult{
			Action: string(params.Action),
			Error:  fmt.Sprintf("invalid action %q, must be one of: next, current, start, complete", params.Action),
		}, nil
	}

	switch params.Action {
	case TaskActionNext:
		return handleTaskNext(ctx, repo, params, defaultSessionID)
	case TaskActionCurrent:
		return handleTaskCurrent(ctx, repo, params, defaultSessionID)
	case TaskActionStart:
		return handleTaskStart(ctx, repo, params, defaultSessionID)
	case TaskActionComplete:
		return handleTaskComplete(ctx, repo, params)
	default:
		return &TaskToolResult{
			Action: string(params.Action),
			Error:  fmt.Sprintf("unsupported action: %s", params.Action),
		}, nil
	}
}

func resolveTaskSessionID(explicit, fallback string) string {
	sessionID := strings.TrimSpace(explicit)
	if sessionID != "" {
		return sessionID
	}
	return strings.TrimSpace(fallback)
}

// handleTaskNext implements the 'next' action - get the next pending task.
func handleTaskNext(ctx context.Context, repo *memory.Repository, params TaskToolParams, defaultSessionID string) (*TaskToolResult, error) {
	// Validate required fields
	sessionID := resolveTaskSessionID(params.SessionID, defaultSessionID)
	if sessionID == "" {
		return &TaskToolResult{
			Action: "next",
			Error:  "session_id is required for next action",
			Content: FormatMultiValidationError(
				"next",
				[]string{"session_id"},
				"Provide a unique session identifier (e.g., hook session-init or UUID). MCP tools can omit this when transport session identity is available.",
			),
		}, nil
	}

	appCtx := app.NewContext(repo)
	taskApp := app.NewTaskApp(appCtx)

	// Default CreateBranch to true if not explicitly provided
	createBranch := true
	if params.CreateBranch != nil {
		createBranch = *params.CreateBranch
	}

	result, err := taskApp.Next(ctx, app.TaskNextOptions{
		PlanID:            params.PlanID,
		SessionID:         sessionID, // Use validated/trimmed value
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
func handleTaskCurrent(ctx context.Context, repo *memory.Repository, params TaskToolParams, defaultSessionID string) (*TaskToolResult, error) {
	// Validate required fields
	sessionID := resolveTaskSessionID(params.SessionID, defaultSessionID)
	if sessionID == "" {
		return &TaskToolResult{
			Action: "current",
			Error:  "session_id is required for current action",
			Content: FormatMultiValidationError(
				"current",
				[]string{"session_id"},
				"Provide the session identifier used when starting the task. MCP tools can omit this when transport session identity is available.",
			),
		}, nil
	}

	appCtx := app.NewContext(repo)
	taskApp := app.NewTaskApp(appCtx)

	result, err := taskApp.Current(ctx, sessionID, params.PlanID)
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
func handleTaskStart(ctx context.Context, repo *memory.Repository, params TaskToolParams, defaultSessionID string) (*TaskToolResult, error) {
	// Validate required fields
	taskID := strings.TrimSpace(params.TaskID)
	if taskID == "" {
		return &TaskToolResult{
			Action: "start",
			Error:  "task_id is required for start action",
		}, nil
	}

	sessionID := resolveTaskSessionID(params.SessionID, defaultSessionID)
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

	// Check for policy violations (task not completed successfully)
	if !result.Success {
		return &TaskToolResult{
			Action:  "complete",
			Content: FormatTaskCompletionBlocked(result),
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
// Supports planning actions: clarify, decompose, expand, generate, finalize, audit
func HandlePlanTool(ctx context.Context, repo *memory.Repository, params PlanToolParams) (*PlanToolResult, error) {
	// Validate action
	if !params.Action.IsValid() {
		return &PlanToolResult{
			Action: string(params.Action),
			Error:  fmt.Sprintf("invalid action %q, must be one of: clarify, decompose, expand, generate, finalize, audit", params.Action),
		}, nil
	}

	switch params.Action {
	case PlanActionClarify:
		return handlePlanClarify(ctx, repo, params)
	case PlanActionDecompose:
		return handlePlanDecompose(ctx, repo, params)
	case PlanActionExpand:
		return handlePlanExpand(ctx, repo, params)
	case PlanActionGenerate:
		return handlePlanGenerate(ctx, repo, params)
	case PlanActionFinalize:
		return handlePlanFinalize(ctx, repo, params)
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
	goal := strings.TrimSpace(params.Goal)
	sessionID := strings.TrimSpace(params.ClarifySessionID)

	// First call requires goal.
	if sessionID == "" && goal == "" {
		return &PlanToolResult{
			Action: "clarify",
			Error:  "goal is required for clarify action",
			Content: FormatMultiValidationError(
				"clarify",
				[]string{"goal"},
				"First clarify call requires a goal. Follow-up calls require clarify_session_id and answers.",
			),
		}, nil
	}

	// Follow-up calls require answers unless auto_answer is requested.
	if sessionID != "" && !params.AutoAnswer && len(params.Answers) == 0 {
		return &PlanToolResult{
			Action: "clarify",
			Error:  "answers are required for clarify follow-up action",
			Content: FormatMultiValidationError(
				"clarify",
				[]string{"answers"},
				"Provide answers for pending questions, or set auto_answer=true to let TaskWing continue automatically.",
			),
		}, nil
	}

	answers := make([]app.ClarifyAnswer, 0, len(params.Answers))
	for _, ans := range params.Answers {
		answer := strings.TrimSpace(ans.Answer)
		if answer == "" {
			continue
		}
		answers = append(answers, app.ClarifyAnswer{
			Question: strings.TrimSpace(ans.Question),
			Answer:   answer,
		})
	}

	// Use RoleBootstrap for planning operations
	appCtx := app.NewContextForRole(repo, llm.RoleBootstrap)
	planApp := app.NewPlanApp(appCtx)

	result, err := planApp.Clarify(ctx, app.ClarifyOptions{
		Goal:             goal,
		ClarifySessionID: sessionID,
		Answers:          answers,
		AutoAnswer:       params.AutoAnswer,
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
	// Validate ALL required fields at once to avoid sequential error frustration
	goal := strings.TrimSpace(params.Goal)
	enrichedGoal := strings.TrimSpace(params.EnrichedGoal)
	clarifySessionID := strings.TrimSpace(params.ClarifySessionID)

	var missingFields []string
	if goal == "" {
		missingFields = append(missingFields, "goal")
	}
	if enrichedGoal == "" {
		missingFields = append(missingFields, "enriched_goal")
	}
	if clarifySessionID == "" {
		missingFields = append(missingFields, "clarify_session_id")
	}

	if len(missingFields) > 0 {
		return &PlanToolResult{
			Action: "generate",
			Error:  fmt.Sprintf("missing required fields: %v", missingFields),
			Content: FormatMultiValidationError(
				"generate",
				missingFields,
				"First call `plan clarify` until is_ready_to_plan=true, then pass goal, enriched_goal, and clarify_session_id to generate.",
			),
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
		Goal:             goal,
		ClarifySessionID: clarifySessionID,
		EnrichedGoal:     enrichedGoal,
		Save:             save,
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

// handlePlanDecompose implements the 'decompose' action - break goal into phases.
// This is Stage 2 of the interactive planning workflow.
func handlePlanDecompose(ctx context.Context, repo *memory.Repository, params PlanToolParams) (*PlanToolResult, error) {
	// Validate required fields
	enrichedGoal := strings.TrimSpace(params.EnrichedGoal)
	if enrichedGoal == "" {
		return &PlanToolResult{
			Action: "decompose",
			Error:  "enriched_goal is required for decompose action",
			Content: FormatMultiValidationError(
				"decompose",
				[]string{"enriched_goal"},
				"First call `plan clarify` to refine your goal into an enriched specification, then pass `enriched_goal` to decompose.",
			),
		}, nil
	}

	// Use RoleBootstrap for planning operations
	appCtx := app.NewContextForRole(repo, llm.RoleBootstrap)
	planApp := app.NewPlanApp(appCtx)

	result, err := planApp.Decompose(ctx, app.DecomposeOptions{
		PlanID:       params.PlanID,
		Goal:         params.Goal,
		EnrichedGoal: enrichedGoal,
		Feedback:     params.Feedback,
	})
	if err != nil {
		return &PlanToolResult{
			Action: "decompose",
			Error:  err.Error(),
		}, nil
	}

	return &PlanToolResult{
		Action:  "decompose",
		Content: FormatDecomposeResult(result),
	}, nil
}

// handlePlanExpand implements the 'expand' action - generate tasks for a phase.
// This is Stage 3 of the interactive planning workflow (repeated per phase).
func handlePlanExpand(ctx context.Context, repo *memory.Repository, params PlanToolParams) (*PlanToolResult, error) {
	// Validate required fields
	planID := strings.TrimSpace(params.PlanID)
	if planID == "" {
		return &PlanToolResult{
			Action: "expand",
			Error:  "plan_id is required for expand action",
			Content: FormatMultiValidationError(
				"expand",
				[]string{"plan_id"},
				"First call `plan decompose` to create phases, then pass `plan_id` and `phase_id` or `phase_index` to expand.",
			),
		}, nil
	}

	// Need either phase_id or phase_index
	phaseID := strings.TrimSpace(params.PhaseID)
	if phaseID == "" && params.PhaseIndex == nil {
		return &PlanToolResult{
			Action: "expand",
			Error:  "either phase_id or phase_index is required for expand action",
			Content: FormatMultiValidationError(
				"expand",
				[]string{"phase_id", "phase_index"},
				"Provide the ID or 0-based index of the phase to expand.",
			),
		}, nil
	}

	// Use RoleBootstrap for planning operations
	appCtx := app.NewContextForRole(repo, llm.RoleBootstrap)
	planApp := app.NewPlanApp(appCtx)

	opts := app.ExpandOptions{
		PlanID:   planID,
		PhaseID:  phaseID,
		Feedback: params.Feedback,
	}
	if params.PhaseIndex != nil {
		opts.PhaseIndex = *params.PhaseIndex
	} else {
		opts.PhaseIndex = -1 // Signal to use PhaseID
	}

	result, err := planApp.Expand(ctx, opts)
	if err != nil {
		return &PlanToolResult{
			Action: "expand",
			Error:  err.Error(),
		}, nil
	}

	return &PlanToolResult{
		Action:  "expand",
		Content: FormatExpandResult(result),
	}, nil
}

// handlePlanFinalize implements the 'finalize' action - save completed interactive plan.
// This is Stage 4 of the interactive planning workflow.
func handlePlanFinalize(ctx context.Context, repo *memory.Repository, params PlanToolParams) (*PlanToolResult, error) {
	// Validate required fields
	planID := strings.TrimSpace(params.PlanID)
	if planID == "" {
		return &PlanToolResult{
			Action: "finalize",
			Error:  "plan_id is required for finalize action",
			Content: FormatMultiValidationError(
				"finalize",
				[]string{"plan_id"},
				"Provide the plan_id from the decompose step to finalize.",
			),
		}, nil
	}

	// Use RoleBootstrap for planning operations
	appCtx := app.NewContextForRole(repo, llm.RoleBootstrap)
	planApp := app.NewPlanApp(appCtx)

	result, err := planApp.Finalize(ctx, app.FinalizeOptions{
		PlanID: planID,
	})
	if err != nil {
		return &PlanToolResult{
			Action: "finalize",
			Error:  err.Error(),
		}, nil
	}

	return &PlanToolResult{
		Action:  "finalize",
		Content: FormatFinalizeResult(result),
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
