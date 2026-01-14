/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/josephgoksu/TaskWing/internal/agents/core"
	"github.com/josephgoksu/TaskWing/internal/agents/impl"
	"github.com/josephgoksu/TaskWing/internal/codeintel"
	"github.com/josephgoksu/TaskWing/internal/config"
	"github.com/josephgoksu/TaskWing/internal/knowledge"
	"github.com/josephgoksu/TaskWing/internal/llm"
	"github.com/josephgoksu/TaskWing/internal/planner"
	"github.com/josephgoksu/TaskWing/internal/task"

	_ "modernc.org/sqlite" // SQLite driver
)

// ClarifyResult contains the result of plan clarification.
type ClarifyResult struct {
	Success       bool     `json:"success"`
	Questions     []string `json:"questions,omitempty"`
	GoalSummary   string   `json:"goal_summary,omitempty"`
	EnrichedGoal  string   `json:"enriched_goal,omitempty"`
	IsReadyToPlan bool     `json:"is_ready_to_plan"`
	ContextUsed   string   `json:"context_used,omitempty"`
	Message       string   `json:"message,omitempty"`
}

// ClarifyOptions configures the behavior of plan clarification.
type ClarifyOptions struct {
	Goal       string // Initial user goal
	History    string // History of Q&A
	AutoAnswer bool   // Whether to autonomously refine context
}

// GenerateResult contains the result of plan generation.
type GenerateResult struct {
	Success          bool                             `json:"success"`
	Tasks            []task.Task                      `json:"tasks,omitempty"`
	PlanID           string                           `json:"plan_id,omitempty"`
	Goal             string                           `json:"goal,omitempty"`
	EnrichedGoal     string                           `json:"enriched_goal,omitempty"`
	Message          string                           `json:"message,omitempty"`
	Hint             string                           `json:"hint,omitempty"`
	SemanticWarnings []string                         `json:"semantic_warnings,omitempty"`
	SemanticErrors   []string                         `json:"semantic_errors,omitempty"`
	ValidationStats  *planner.SemanticValidationStats `json:"validation_stats,omitempty"`
}

// GenerateOptions configures the behavior of plan generation.
type GenerateOptions struct {
	Goal         string // Original user goal
	EnrichedGoal string // Fully clarified specification
	Save         bool   // Whether to persist plan/tasks to DB
}

// AuditResult contains the result of plan auditing.
type AuditResult struct {
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

// AuditOptions configures the behavior of plan auditing.
type AuditOptions struct {
	PlanID  string // Optional: specific plan ID (defaults to active plan)
	AutoFix bool   // If true, attempt to fix failures automatically
}

// GoalsClarifier defines the interface for the clarifying agent.
type GoalsClarifier interface {
	Run(ctx context.Context, input core.Input) (core.Output, error)
	AutoAnswer(ctx context.Context, currentSpec string, questions []string, kgContext string) (string, error)
	Close() error
}

// TaskPlanner defines the interface for the planning agent.
type TaskPlanner interface {
	Run(ctx context.Context, input core.Input) (core.Output, error)
	Close() error
}

// PlanApp provides plan lifecycle operations.
// This is THE implementation - CLI and MCP both call these methods.
type PlanApp struct {
	ctx              *Context
	Repo             task.Repository
	ClarifierFactory func(llm.Config) GoalsClarifier
	PlannerFactory   func(llm.Config) TaskPlanner
	ContextRetriever func(ctx context.Context, ks *knowledge.Service, goal, memoryPath string) (impl.SearchStrategyResult, error)
}

// NewPlanApp creates a new plan application service.
func NewPlanApp(ctx *Context) *PlanApp {
	return &PlanApp{
		ctx:  ctx,
		Repo: ctx.Repo,
		ClarifierFactory: func(cfg llm.Config) GoalsClarifier {
			return impl.NewClarifyingAgent(cfg)
		},
		PlannerFactory: func(cfg llm.Config) TaskPlanner {
			return impl.NewPlanningAgent(cfg)
		},
		ContextRetriever: impl.RetrieveContext,
	}
}

// Clarify refines a development goal by asking clarifying questions.
// Call this in a loop until IsReadyToPlan is true.
func (a *PlanApp) Clarify(ctx context.Context, opts ClarifyOptions) (*ClarifyResult, error) {
	if opts.Goal == "" {
		return &ClarifyResult{
			Success: false,
			Message: "goal is required",
		}, nil
	}

	llmCfg := a.ctx.LLMCfg

	// Fetch context from knowledge graph using canonical shared function
	// Context retrieval is optional enhancement - log errors but don't fail
	ks := knowledge.NewService(a.ctx.Repo, llmCfg) // Still needs concrete repo for now?
	// ks := knowledge.NewService(repo, llmCfg) // Error: repo is interface, NewService takes *memory.Repository
	// We handle this by keeping a.ctx.Repo for NewService, but using a.ContextRetriever which can be mocked to ignore ks.
	var contextStr string
	if memoryPath, err := config.GetMemoryBasePath(); err == nil {
		if result, err := a.ContextRetriever(ctx, ks, opts.Goal, memoryPath); err == nil {
			contextStr = result.Context
		}
	}

	// Create and run ClarifyingAgent
	clarifyingAgent := a.ClarifierFactory(llmCfg)
	defer func() { _ = clarifyingAgent.Close() }()

	input := core.Input{
		ExistingContext: map[string]any{
			"goal":    opts.Goal,
			"history": opts.History,
			"context": contextStr,
		},
	}

	output, err := clarifyingAgent.Run(ctx, input)
	if err != nil {
		return &ClarifyResult{
			Success: false,
			Message: fmt.Sprintf("Clarifying agent failed: %v", err),
		}, nil
	}
	if output.Error != nil {
		return &ClarifyResult{
			Success: false,
			Message: fmt.Sprintf("Clarifying agent error: %v", output.Error),
		}, nil
	}

	// Parse agent output
	if len(output.Findings) == 0 {
		return &ClarifyResult{
			Success: false,
			Message: "No findings from clarifying agent",
		}, nil
	}

	finding := output.Findings[0]
	questions := parseQuestionsFromMetadata(finding.Metadata)
	goalSummary, _ := finding.Metadata["goal_summary"].(string)
	enrichedGoal, _ := finding.Metadata["enriched_goal"].(string)
	isReady, _ := finding.Metadata["is_ready_to_plan"].(bool)

	// If auto_answer and we have questions, try to answer them
	if opts.AutoAnswer && len(questions) > 0 && !isReady {
		const maxAutoAnswerAttempts = 3
		attempts := 0

		for len(questions) > 0 && !isReady && attempts < maxAutoAnswerAttempts {
			attempts++
			answer, err := clarifyingAgent.AutoAnswer(ctx, enrichedGoal, questions, contextStr)
			if err != nil || answer == "" {
				break // Stop if LLM fails or returns empty
			}

			enrichedGoal = answer
			// Re-run to check if now ready
			input.ExistingContext["history"] = fmt.Sprintf("%s\n\nAuto-answered questions (Attempt %d):\n%s", opts.History, attempts, answer)

			output2, err := clarifyingAgent.Run(ctx, input)
			if err != nil || len(output2.Findings) == 0 {
				break
			}

			finding2 := output2.Findings[0]
			questions = parseQuestionsFromMetadata(finding2.Metadata)
			goalSummary, _ = finding2.Metadata["goal_summary"].(string)
			enrichedGoal, _ = finding2.Metadata["enriched_goal"].(string)
			isReady, _ = finding2.Metadata["is_ready_to_plan"].(bool)
		}
	}

	contextSummary := ""
	if contextStr != "" {
		contextSummary = "Retrieved relevant nodes and constraints from knowledge graph"
	}

	return &ClarifyResult{
		Success:       true,
		Questions:     questions,
		GoalSummary:   goalSummary,
		EnrichedGoal:  enrichedGoal,
		IsReadyToPlan: isReady,
		ContextUsed:   contextSummary,
	}, nil
}

// Generate creates a development plan with tasks from a refined goal.
// Requires EnrichedGoal from Clarify.
func (a *PlanApp) Generate(ctx context.Context, opts GenerateOptions) (*GenerateResult, error) {
	if opts.Goal == "" {
		return &GenerateResult{
			Success: false,
			Message: "goal is required",
		}, nil
	}
	if opts.EnrichedGoal == "" {
		return &GenerateResult{
			Success: false,
			Message: "enriched_goal is required (run Clarify first)",
		}, nil
	}

	repo := a.Repo
	llmCfg := a.ctx.LLMCfg

	// Fetch context from knowledge graph using canonical shared function
	// Context retrieval is optional enhancement - log errors but don't fail
	ks := knowledge.NewService(a.ctx.Repo, llmCfg)
	var contextStr string
	if memoryPath, err := config.GetMemoryBasePath(); err == nil {
		if result, err := a.ContextRetriever(ctx, ks, opts.EnrichedGoal, memoryPath); err == nil {
			contextStr = result.Context
		}
	}

	// Create and run PlanningAgent
	planningAgent := a.PlannerFactory(llmCfg)
	defer func() { _ = planningAgent.Close() }()

	input := core.Input{
		ExistingContext: map[string]any{
			"goal":          opts.Goal,
			"enriched_goal": opts.EnrichedGoal,
			"context":       contextStr,
		},
	}

	output, err := planningAgent.Run(ctx, input)
	if err != nil {
		return &GenerateResult{
			Success: false,
			Message: fmt.Sprintf("Planning agent failed: %v", err),
		}, nil
	}
	if output.Error != nil {
		return &GenerateResult{
			Success: false,
			Message: fmt.Sprintf("Planning agent error: %v", output.Error),
		}, nil
	}

	// Parse tasks from output
	if len(output.Findings) == 0 {
		return &GenerateResult{
			Success: false,
			Message: "No findings from planning agent",
		}, nil
	}

	finding := output.Findings[0]
	tasks := parseTasksFromMetadata(finding.Metadata)

	if len(tasks) == 0 {
		return &GenerateResult{
			Success: false,
			Message: "No tasks generated",
		}, nil
	}

	// Validate tasks
	for i, t := range tasks {
		if err := t.Validate(); err != nil {
			return &GenerateResult{
				Success: false,
				Message: fmt.Sprintf("Task %d validation failed: %v", i+1, err),
			}, nil
		}
	}

	// Run semantic validation (file paths, shell commands)
	var semanticWarnings, semanticErrors []string
	var validationStats *planner.SemanticValidationStats
	{
		// Prefer project base path when available (MCP/CLI may run from different cwd)
		workDir := a.ctx.BasePath
		if workDir == "" {
			workDir, _ = os.Getwd()
		}
		middleware := planner.NewSemanticMiddleware(planner.MiddlewareConfig{
			BasePath:          workDir,
			AllowMissingFiles: true, // Warnings, not errors - plans often create new files
		})

		// Convert tasks to planner schema for validation
		plannerTasks := make([]planner.LLMTaskSchema, len(tasks))
		for i, t := range tasks {
			plannerTasks[i] = planner.LLMTaskSchema{
				Title:              t.Title,
				Description:        t.Description,
				AcceptanceCriteria: t.AcceptanceCriteria,
				ValidationSteps:    t.ValidationSteps,
			}
		}

		semanticResult := middleware.Validate(&planner.LLMPlanResponse{
			GoalSummary:         truncateString(opts.Goal, 100),
			Rationale:           opts.EnrichedGoal,
			Tasks:               plannerTasks,
			EstimatedComplexity: "medium", // Default
		})

		validationStats = &semanticResult.Stats

		// Collect warnings
		for _, w := range semanticResult.Warnings {
			semanticWarnings = append(semanticWarnings, fmt.Sprintf("[Task %d] %s: %s", w.TaskIndex+1, w.Type, w.Message))
		}

		// Collect errors (these are non-blocking but logged)
		for _, e := range semanticResult.Errors {
			semanticErrors = append(semanticErrors, fmt.Sprintf("[Task %d] %s: %s", e.TaskIndex+1, e.Type, e.Message))
		}

		// Log validation results
		if len(semanticWarnings) > 0 || len(semanticErrors) > 0 {
			slog.Debug("semantic validation completed",
				"warnings", len(semanticWarnings),
				"errors", len(semanticErrors),
				"paths_checked", semanticResult.Stats.PathsChecked,
				"commands_validated", semanticResult.Stats.CommandsValidated)
		}
	}

	// Run PlanVerifier to auto-correct paths and commands using code intelligence
	{
		// Try to get codeintel QueryService (optional - best effort)
		var queryService *codeintel.QueryService
		var db *sql.DB
		if memoryPath, err := config.GetMemoryBasePath(); err == nil {
			dbPath := filepath.Join(memoryPath, "memory.db")
			if _, statErr := os.Stat(dbPath); statErr == nil {
				if db, err = sql.Open("sqlite", dbPath); err == nil {
					defer func() { _ = db.Close() }()
					repo := codeintel.NewRepository(db)
					queryService = codeintel.NewQueryService(repo, llmCfg)
				}
			}
		}

		verifier := planner.NewPlanVerifierWithConfig(queryService, planner.VerifierConfig{
			BasePath: a.ctx.BasePath,
		})

		// Convert tasks to planner schema for verification
		plannerTasks := make([]planner.LLMTaskSchema, len(tasks))
		for i, t := range tasks {
			plannerTasks[i] = planner.LLMTaskSchema{
				Title:              t.Title,
				Description:        t.Description,
				AcceptanceCriteria: t.AcceptanceCriteria,
				ValidationSteps:    t.ValidationSteps,
			}
		}

		// Run verification and apply corrections
		correctedTasks, err := verifier.Verify(ctx, plannerTasks)
		if err != nil {
			slog.Debug("plan verification skipped", "error", err)
		} else {
			// Track corrections
			var pathCorrections, commandCorrections int

			// Apply path corrections
			for i := range correctedTasks {
				verifyResult := verifier.VerifyTaskPaths(ctx, i, &correctedTasks[i])
				if verifyResult.Corrections > 0 {
					pathCorrections += verifyResult.Corrections
					// Apply corrections to descriptions
					corrections := make(map[string]string)
					for _, pr := range verifyResult.PathResults {
						if pr.Corrected != "" {
							corrections[pr.Original] = pr.Corrected
						}
					}
					if len(corrections) > 0 {
						correctedTasks[i].Description = planner.ApplyCorrections(correctedTasks[i].Description, corrections)
						correctedTasks[i].Title = planner.ApplyCorrections(correctedTasks[i].Title, corrections)
						for j, criterion := range correctedTasks[i].AcceptanceCriteria {
							correctedTasks[i].AcceptanceCriteria[j] = planner.ApplyCorrections(criterion, corrections)
						}
					}
				}

				// Apply command corrections
				corrected, notes := verifier.CorrectTaskCommands(ctx, &correctedTasks[i])
				if corrected {
					commandCorrections++
					for _, note := range notes {
						semanticWarnings = append(semanticWarnings, fmt.Sprintf("[Task %d] %s", i+1, note))
					}
				}
			}

			// Update original tasks with corrections
			for i := range tasks {
				tasks[i].Title = correctedTasks[i].Title
				tasks[i].Description = correctedTasks[i].Description
				tasks[i].AcceptanceCriteria = correctedTasks[i].AcceptanceCriteria
				tasks[i].ValidationSteps = correctedTasks[i].ValidationSteps
			}

			// Log corrections
			if pathCorrections > 0 || commandCorrections > 0 {
				slog.Info("plan verifier applied corrections",
					"path_corrections", pathCorrections,
					"command_corrections", commandCorrections)
				semanticWarnings = append(semanticWarnings,
					fmt.Sprintf("Auto-corrected %d paths and %d commands using code intelligence", pathCorrections, commandCorrections))
			}
		}
	}

	// Save the plan
	var planID string
	{
		plan := &task.Plan{
			Goal:         opts.Goal,
			EnrichedGoal: opts.EnrichedGoal,
			Status:       task.PlanStatusActive,
			Tasks:        tasks,
		}

		if opts.Save {
			// Transactional creation logic is handled by repo.CreatePlan
			if err := repo.CreatePlan(plan); err != nil {
				return &GenerateResult{
					Success: false,
					Message: fmt.Sprintf("Failed to save plan: %v", err),
				}, nil
			}
			planID = plan.ID

			// Set as active plan (fail if we can't set it active)
			if memoryPathSvc, err := config.GetMemoryBasePath(); err == nil {
				svc := task.NewService(repo, memoryPathSvc)
				if err := svc.SetActivePlan(planID); err != nil {
					return &GenerateResult{
						Success: false,
						Message: fmt.Sprintf("Plan created but failed to set active: %v", err),
						PlanID:  planID,
					}, nil
				}
			} else {
				if err := repo.SetActivePlan(planID); err != nil {
					return &GenerateResult{
						Success: false,
						Message: fmt.Sprintf("Plan created but failed to set active: %v", err),
						PlanID:  planID,
					}, nil
				}
			}
		} else {
			// Even if not saving to DB, generate a temporary ID or leave empty
			planID = plan.ID // will be empty or 0 if not saved
		}
	}

	return &GenerateResult{
		Success:          true,
		Tasks:            tasks,
		PlanID:           planID,
		Goal:             opts.Goal,
		EnrichedGoal:     opts.EnrichedGoal,
		Message:          "Plan generated successfully",
		Hint:             "Use task_next to begin working on the first task.",
		SemanticWarnings: semanticWarnings,
		SemanticErrors:   semanticErrors,
		ValidationStats:  validationStats,
	}, nil
}

// truncateString truncates a string to maxLen characters.
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}

// Audit runs verification on a completed plan.
func (a *PlanApp) Audit(ctx context.Context, opts AuditOptions) (*AuditResult, error) {
	repo := a.Repo
	llmCfg := a.ctx.LLMCfg

	// Determine which plan to audit
	var plan *task.Plan
	var err error

	if opts.PlanID != "" {
		plan, err = repo.GetPlan(opts.PlanID)
		if err != nil {
			return &AuditResult{
				Success: false,
				Message: fmt.Sprintf("Failed to get plan: %v", err),
			}, nil
		}
	} else {
		plan, err = repo.GetActivePlan()
		if err != nil {
			return &AuditResult{
				Success: false,
				Message: fmt.Sprintf("Failed to get active plan: %v", err),
			}, nil
		}
	}

	if plan == nil {
		return &AuditResult{
			Success: false,
			Message: "No plan found. Create a plan first with plan_clarify and plan_generate.",
			Hint:    "Use plan_clarify to start defining your development goal.",
		}, nil
	}

	// Check if plan has completed tasks
	completedCount := 0
	for _, t := range plan.Tasks {
		if t.Status == task.StatusCompleted {
			completedCount++
		}
	}

	if completedCount == 0 {
		return &AuditResult{
			Success: false,
			PlanID:  plan.ID,
			Message: "No completed tasks to impl. Complete tasks first.",
			Hint:    "Use task_next to get the next pending task.",
		}, nil
	}

	// Get working directory
	workDir, _ := os.Getwd()

	// Create audit service
	auditService := impl.NewService(workDir, llmCfg)

	if !opts.AutoFix {
		auditResult, err := auditService.Audit(ctx, plan)
		if err != nil {
			return &AuditResult{
				Success: false,
				PlanID:  plan.ID,
				Message: fmt.Sprintf("Audit failed: %v", err),
			}, nil
		}

		result := &AuditResult{
			Success:        true,
			PlanID:         plan.ID,
			RetryCount:     1,
			BuildPassed:    auditResult.BuildResult.Passed,
			TestsPassed:    auditResult.TestResult.Passed,
			SemanticIssues: auditResult.SemanticResult.Issues,
		}

		// Update plan status in database
		var newStatus task.PlanStatus
		if auditResult.Status == "passed" {
			result.Status = "verified"
			newStatus = task.PlanStatusVerified
			result.Message = "Plan verified successfully. All checks passed."
			result.Hint = "The plan is complete and verified. You can create a PR or start a new plan."
		} else {
			result.Status = "needs_revision"
			newStatus = task.PlanStatusNeedsRevision
			result.Message = "Plan needs revision. One or more checks failed."
			result.Hint = "Review the failed checks and fix them, then run audit again."
		}
		result.PlanStatus = newStatus

		// Store audit report
		report := task.AuditReport{
			Status:         auditResult.Status,
			BuildOutput:    auditResult.BuildResult.Output,
			TestOutput:     auditResult.TestResult.Output,
			SemanticIssues: auditResult.SemanticResult.Issues,
			RetryCount:     1,
			CompletedAt:    time.Now().UTC(),
		}
		if !auditResult.BuildResult.Passed && auditResult.BuildResult.Error != "" {
			report.ErrorMessage = "Build failed: " + auditResult.BuildResult.Error
		} else if !auditResult.TestResult.Passed && auditResult.TestResult.Error != "" {
			report.ErrorMessage = "Tests failed: " + auditResult.TestResult.Error
		}
		reportJSON, marshalErr := json.Marshal(report)
		if marshalErr == nil {
			_ = repo.UpdatePlanAuditReport(plan.ID, newStatus, string(reportJSON))
		}

		return result, nil
	}

	// Run audit with auto-fix
	autoFixResult, err := auditService.AuditWithAutoFix(ctx, plan)
	if err != nil {
		return &AuditResult{
			Success: false,
			PlanID:  plan.ID,
			Message: fmt.Sprintf("Audit failed: %v", err),
		}, nil
	}

	result := &AuditResult{
		Success:    true,
		PlanID:     plan.ID,
		Status:     autoFixResult.FinalStatus,
		RetryCount: autoFixResult.Attempts,
	}
	result.FixesApplied = autoFixResult.FixesApplied

	if autoFixResult.FinalAudit != nil {
		result.BuildPassed = autoFixResult.FinalAudit.BuildResult.Passed
		result.TestsPassed = autoFixResult.FinalAudit.TestResult.Passed
		result.SemanticIssues = autoFixResult.FinalAudit.SemanticResult.Issues
	}

	// Update plan status in database
	var newStatus task.PlanStatus
	if autoFixResult.FinalStatus == "verified" {
		newStatus = task.PlanStatusVerified
		result.Message = "Plan verified successfully. All checks passed."
		result.Hint = "The plan is complete and verified. You can create a PR or start a new plan."
	} else {
		newStatus = task.PlanStatusNeedsRevision
		result.Message = fmt.Sprintf("Plan needs revision after %d fix attempts.", autoFixResult.Attempts)
		result.Hint = "Review the semantic issues and fix them manually, then run audit again."
	}
	result.PlanStatus = newStatus

	// Store audit report
	auditReport := autoFixResult.ToAuditReportWithFixes()
	reportJSON, marshalErr := json.Marshal(auditReport)
	if marshalErr == nil {
		_ = repo.UpdatePlanAuditReport(plan.ID, newStatus, string(reportJSON))
	}

	return result, nil
}

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

// parseTasksFromMetadata extracts tasks from agent metadata,
// handling both []impl.PlanningTask and []any (from JSON unmarshaling).
func parseTasksFromMetadata(metadata map[string]any) []task.Task {
	var tasks []task.Task

	// Map title -> ID for dependency resolution
	titleToID := make(map[string]string)
	// Temp storage for title dependencies
	type pendingDep struct {
		taskIdx int
		titles  []string
	}
	var pendingDeps []pendingDep

	// Helper to generate IDs immediately so we can link them
	genID := func() string {
		return "task-" + uuid.New().String()[:8]
	}

	// Try typed slice first
	if tasksRaw, ok := metadata["tasks"].([]impl.PlanningTask); ok {
		for i, pt := range tasksRaw {
			id := genID()
			t := task.Task{
				ID:                 id,
				Title:              pt.Title,
				Description:        pt.Description,
				AcceptanceCriteria: pt.AcceptanceCriteria,
				ValidationSteps:    pt.ValidationSteps,
				Priority:           pt.Priority,
				Status:             task.StatusPending,
				AssignedAgent:      pt.AssignedAgent,
				Complexity:         pt.Complexity,
				Scope:              pt.Scope,
				Keywords:           pt.Keywords,
			}
			t.EnrichAIFields()
			tasks = append(tasks, t)
			titleToID[pt.Title] = id

			if len(pt.Dependencies) > 0 {
				pendingDeps = append(pendingDeps, pendingDep{taskIdx: i, titles: pt.Dependencies})
			}
		}
	} else if tasksAny, ok := metadata["tasks"].([]any); ok {
		// Handle []any from JSON unmarshaling
		for i, t := range tasksAny {
			if tm, ok := t.(map[string]any); ok {
				title, _ := tm["title"].(string)
				desc, _ := tm["description"].(string)
				priority, _ := tm["priority"].(float64)
				agent, _ := tm["assigned_agent"].(string)
				complexity, _ := tm["complexity"].(string)
				scope, _ := tm["scope"].(string)

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

				var deps []string
				if ds, ok := tm["dependencies"].([]any); ok {
					for _, d := range ds {
						if dsStr, ok := d.(string); ok {
							deps = append(deps, dsStr)
						}
					}
				}

				var keywords []string
				if kw, ok := tm["keywords"].([]any); ok {
					for _, k := range kw {
						if ks, ok := k.(string); ok {
							keywords = append(keywords, ks)
						}
					}
				}

				id := genID()
				newTask := task.Task{
					ID:                 id,
					Title:              title,
					Description:        desc,
					AcceptanceCriteria: criteria,
					ValidationSteps:    validation,
					Priority:           int(priority),
					Status:             task.StatusPending,
					AssignedAgent:      agent,
					Complexity:         complexity,
					Scope:              scope,
					Keywords:           keywords,
				}
				newTask.EnrichAIFields()
				tasks = append(tasks, newTask)
				titleToID[title] = id

				if len(deps) > 0 {
					pendingDeps = append(pendingDeps, pendingDep{taskIdx: i, titles: deps})
				}
			}
		}
	}

	// Resolve dependencies
	for _, pd := range pendingDeps {
		for _, depTitle := range pd.titles {
			if depID, ok := titleToID[depTitle]; ok {
				tasks[pd.taskIdx].Dependencies = append(tasks[pd.taskIdx].Dependencies, depID)
			}
		}
	}

	return tasks
}
