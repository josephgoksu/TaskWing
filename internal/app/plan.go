/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package app

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/josephgoksu/TaskWing/internal/agents/audit"
	"github.com/josephgoksu/TaskWing/internal/agents/core"
	"github.com/josephgoksu/TaskWing/internal/agents/planning"
	"github.com/josephgoksu/TaskWing/internal/config"
	"github.com/josephgoksu/TaskWing/internal/knowledge"
	"github.com/josephgoksu/TaskWing/internal/task"
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
	Goal       string // Required: the user's goal
	History    string // Optional: JSON array of previous Q&A [{q, a}, ...]
	AutoAnswer bool   // If true, use KG to auto-answer questions
}

// GenerateResult contains the result of plan generation.
type GenerateResult struct {
	Success      bool        `json:"success"`
	PlanID       string      `json:"plan_id,omitempty"`
	Goal         string      `json:"goal,omitempty"`
	EnrichedGoal string      `json:"enriched_goal,omitempty"`
	Tasks        []task.Task `json:"tasks,omitempty"`
	Message      string      `json:"message,omitempty"`
	Hint         string      `json:"hint,omitempty"`
}

// GenerateOptions configures the behavior of plan generation.
type GenerateOptions struct {
	Goal         string // Required: user's original goal
	EnrichedGoal string // Required: full technical spec from clarify
	Save         bool   // If true (default), save plan to database
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

// PlanApp provides plan lifecycle operations.
// This is THE implementation - CLI and MCP both call these methods.
type PlanApp struct {
	ctx *Context
}

// NewPlanApp creates a new plan application service.
func NewPlanApp(ctx *Context) *PlanApp {
	return &PlanApp{ctx: ctx}
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

	repo := a.ctx.Repo
	llmCfg := a.ctx.LLMCfg

	// Fetch context from knowledge graph using canonical shared function
	// Context retrieval is optional enhancement - log errors but don't fail
	ks := knowledge.NewService(repo, llmCfg)
	var contextStr string
	if memoryPath, err := config.GetMemoryBasePath(); err == nil {
		if result, err := planning.RetrieveContext(ctx, ks, opts.Goal, memoryPath); err == nil {
			contextStr = result.Context
		}
	}

	// Create and run ClarifyingAgent
	clarifyingAgent := planning.NewClarifyingAgent(llmCfg)
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
		answer, err := clarifyingAgent.AutoAnswer(ctx, enrichedGoal, questions, contextStr)
		if err == nil && answer != "" {
			enrichedGoal = answer
			// Re-run to check if now ready
			input.ExistingContext["history"] = fmt.Sprintf("%s\n\nAuto-answered questions with:\n%s", opts.History, answer)
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

	repo := a.ctx.Repo
	llmCfg := a.ctx.LLMCfg

	// Fetch context from knowledge graph using canonical shared function
	// Context retrieval is optional enhancement - log errors but don't fail
	ks := knowledge.NewService(repo, llmCfg)
	var contextStr string
	if memoryPath, err := config.GetMemoryBasePath(); err == nil {
		if result, err := planning.RetrieveContext(ctx, ks, opts.EnrichedGoal, memoryPath); err == nil {
			contextStr = result.Context
		}
	}

	// Create and run PlanningAgent
	planningAgent := planning.NewPlanningAgent(llmCfg)
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

	// Save the plan
	var planID string
	{
		plan := &task.Plan{
			Goal:         opts.Goal,
			EnrichedGoal: opts.EnrichedGoal,
			Status:       task.PlanStatusActive,
			Tasks:        tasks,
		}

		if err := repo.CreatePlan(plan); err != nil {
			return &GenerateResult{
				Success: false,
				Message: fmt.Sprintf("Failed to save plan: %v", err),
			}, nil
		}
		planID = plan.ID

		// Set as active plan (best effort - plan was created successfully)
		// If memory path is unavailable, skip active plan setting
		if memoryPathSvc, err := config.GetMemoryBasePath(); err == nil {
			svc := task.NewService(repo, memoryPathSvc)
			_ = svc.SetActivePlan(planID)
		}
	}

	return &GenerateResult{
		Success:      true,
		PlanID:       planID,
		Goal:         opts.Goal,
		EnrichedGoal: opts.EnrichedGoal,
		Tasks:        tasks,
		Hint:         "Use task_next to begin working on the first task.",
	}, nil
}

// Audit runs verification on a completed plan.
func (a *PlanApp) Audit(ctx context.Context, opts AuditOptions) (*AuditResult, error) {
	repo := a.ctx.Repo
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
			Message: "No completed tasks to audit. Complete tasks first.",
			Hint:    "Use task_next to get the next pending task.",
		}, nil
	}

	// Get working directory
	workDir, _ := os.Getwd()

	// Create audit service
	auditService := audit.NewService(workDir, llmCfg)

	// Run audit with auto-fix
	auditResult, err := auditService.AuditWithAutoFix(ctx, plan)
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
		Status:     auditResult.FinalStatus,
		RetryCount: auditResult.Attempts,
	}
	result.FixesApplied = auditResult.FixesApplied

	if auditResult.FinalAudit != nil {
		result.BuildPassed = auditResult.FinalAudit.BuildResult.Passed
		result.TestsPassed = auditResult.FinalAudit.TestResult.Passed
		result.SemanticIssues = auditResult.FinalAudit.SemanticResult.Issues
	}

	// Update plan status in database
	var newStatus task.PlanStatus
	if auditResult.FinalStatus == "verified" {
		newStatus = task.PlanStatusVerified
		result.Message = "Plan verified successfully. All checks passed."
		result.Hint = "The plan is complete and verified. You can create a PR or start a new plan."
	} else {
		newStatus = task.PlanStatusNeedsRevision
		result.Message = fmt.Sprintf("Plan needs revision after %d fix attempts.", auditResult.Attempts)
		result.Hint = "Review the semantic issues and fix them manually, then run audit again."
	}
	result.PlanStatus = newStatus

	// Store audit report
	auditReport := auditResult.ToAuditReportWithFixes()
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
// handling both []planning.PlanningTask and []any (from JSON unmarshaling).
func parseTasksFromMetadata(metadata map[string]any) []task.Task {
	var tasks []task.Task

	// Try typed slice first
	if tasksRaw, ok := metadata["tasks"].([]planning.PlanningTask); ok {
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
			t.EnrichAIFields()
			tasks = append(tasks, t)
		}
		return tasks
	}

	// Handle []any from JSON unmarshaling
	if tasksAny, ok := metadata["tasks"].([]any); ok {
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
				t.EnrichAIFields()
				tasks = append(tasks, t)
			}
		}
	}

	return tasks
}
