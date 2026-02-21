package app

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/josephgoksu/TaskWing/internal/agents/impl"
	"github.com/josephgoksu/TaskWing/internal/git"
	"github.com/josephgoksu/TaskWing/internal/policy"
	"github.com/josephgoksu/TaskWing/internal/task"
)

// TaskResult contains the result of a task operation.
// This is the canonical response type used by both CLI and MCP.
type TaskResult struct {
	Success bool       `json:"success"`
	Message string     `json:"message,omitempty"`
	Task    *task.Task `json:"task,omitempty"`
	Plan    *task.Plan `json:"plan,omitempty"`
	Hint    string     `json:"hint,omitempty"`
	Context string     `json:"context,omitempty"` // Rich Markdown context

	// Git workflow fields
	GitBranch          string `json:"git_branch,omitempty"`
	GitWorkflowApplied bool   `json:"git_workflow_applied,omitempty"`
	GitUnpushedCommits bool   `json:"git_unpushed_commits,omitempty"`
	GitUnpushedBranch  string `json:"git_unpushed_branch,omitempty"`

	// PR fields
	PRURL     string `json:"pr_url,omitempty"`
	PRCreated bool   `json:"pr_created,omitempty"`

	// Audit fields
	AuditTriggered  bool            `json:"audit_triggered,omitempty"`
	AuditStatus     string          `json:"audit_status,omitempty"`
	AuditPlanStatus task.PlanStatus `json:"audit_plan_status,omitempty"`

	// Sentinel fields - deviation detection between plan and execution
	SentinelReport *task.SentinelReport `json:"sentinel_report,omitempty"`

	// Policy enforcement fields
	PolicyViolation bool     `json:"policy_violation,omitempty"` // True if blocked by policy
	PolicyErrors    []string `json:"policy_errors,omitempty"`    // List of policy violations
}

// TaskNextOptions configures the behavior of getting the next task.
type TaskNextOptions struct {
	PlanID            string // Optional: specific plan ID (defaults to active)
	SessionID         string // Required for auto-start: unique session ID
	AutoStart         bool   // If true, automatically claim the task
	CreateBranch      bool   // Create a new git branch for this plan (default: true)
	SkipUnpushedCheck bool   // If true, proceed despite unpushed commits
}

// TaskStartOptions configures the behavior of starting a task.
type TaskStartOptions struct {
	TaskID    string // Required: task to start
	SessionID string // Required: unique session ID
}

// TaskCompleteOptions configures the behavior of completing a task.
type TaskCompleteOptions struct {
	TaskID        string   // Required: task to complete
	Summary       string   // Optional: what was accomplished
	FilesModified []string // Optional: files changed
}

// TaskApp provides task lifecycle operations.
// This is THE implementation - CLI and MCP both call these methods.
type TaskApp struct {
	ctx *Context
}

// NewTaskApp creates a new task application service.
func NewTaskApp(ctx *Context) *TaskApp {
	return &TaskApp{ctx: ctx}
}

// Next gets the next pending task with optional git workflow and auto-start.
func (a *TaskApp) Next(ctx context.Context, opts TaskNextOptions) (*TaskResult, error) {
	repo := a.ctx.Repo

	// Determine which plan to use
	var plan *task.Plan
	var planID string

	if opts.PlanID != "" {
		planID = opts.PlanID
		var err error
		plan, err = repo.GetPlan(planID)
		if err != nil {
			return nil, fmt.Errorf("get plan: %w", err)
		}
	} else {
		activePlan, err := repo.GetActivePlan()
		if err != nil {
			return nil, fmt.Errorf("get active plan: %w", err)
		}
		if activePlan == nil {
			return &TaskResult{
				Success: false,
				Message: "No active plan found. Create one with 'taskwing goal \"<goal>\"'.",
			}, nil
		}
		planID = activePlan.ID
		plan = activePlan
	}

	// Get next pending task
	nextTask, err := repo.GetNextTask(planID)
	if err != nil {
		return nil, fmt.Errorf("get next task: %w", err)
	}
	if nextTask == nil {
		return &TaskResult{
			Success: true,
			Message: "No pending tasks in this plan. All tasks may be completed or blocked.",
			Hint:    "Check plan status with 'taskwing plan list' or create new tasks.",
		}, nil
	}

	// Git workflow - creates feature branch by default
	// Set CreateBranch=false to skip branch creation
	var gitBranch string
	var gitWorkflowApplied bool

	if opts.CreateBranch {
		result, gitErr := a.executeGitWorkflow(plan, opts.SkipUnpushedCheck)
		if gitErr != nil {
			// Check for unpushed commits error
			if git.IsUnpushedCommitsError(gitErr) {
				unpushedErr := gitErr.(*git.UnpushedCommitsError)
				return &TaskResult{
					Success:            false,
					Message:            fmt.Sprintf("You have unpushed commits on branch %q. Please push or use skip_unpushed_check=true to proceed.", unpushedErr.Branch),
					Hint:               "Push your commits first, or use skip_unpushed_check option.",
					GitUnpushedCommits: true,
					GitUnpushedBranch:  unpushedErr.Branch,
				}, nil
			}
			// Check for unrelated branch error
			if git.IsUnrelatedBranchError(gitErr) {
				unrelatedErr := gitErr.(*git.UnrelatedBranchError)
				return &TaskResult{
					Success: false,
					Message: fmt.Sprintf("You are on branch %q which is unrelated to this plan. Switch to %q or %q first, or set create_branch=false to work on current branch.", unrelatedErr.CurrentBranch, "main", unrelatedErr.ExpectedBranch),
					Hint:    "Run: git checkout main, or set create_branch=false to stay on current branch.",
				}, nil
			}
			// For other git errors, continue (git workflow is optional)
		} else if result != nil {
			gitBranch = result.BranchName
			gitWorkflowApplied = true
		}
	}

	// Auto-start if requested
	if opts.AutoStart && opts.SessionID != "" {
		if err := repo.ClaimTask(nextTask.ID, opts.SessionID); err != nil {
			return &TaskResult{
				Success: false,
				Message: fmt.Sprintf("Failed to claim task (may have been claimed by another session): %v", err),
				Hint:    "Try again to get the next available task.",
			}, nil
		}

		// Capture git baseline for accurate deviation detection
		workDir, _ := os.Getwd()
		if task.IsGitRepo(workDir) {
			verifier := task.NewGitVerifier(workDir)
			baseline, baselineErr := verifier.GetActualModifications(ctx)
			if baselineErr == nil && len(baseline) > 0 {
				_ = repo.SetGitBaseline(nextTask.ID, baseline)
			}
		}

		// Re-fetch to get accurate ClaimedAt timestamp
		nextTask, err = repo.GetTask(nextTask.ID)
		if err != nil {
			return nil, fmt.Errorf("get claimed task: %w", err)
		}
	}

	// Build hint
	hint := "Call ask tool with suggested queries for context before starting work."
	if len(nextTask.SuggestedAskQueries) > 0 {
		hint = fmt.Sprintf("Call ask tool with queries: %v", nextTask.SuggestedAskQueries)
	}

	// Build rich context
	richContext := a.buildRichContext(ctx, nextTask, plan)

	return &TaskResult{
		Success:            true,
		Task:               nextTask,
		Plan:               plan,
		Hint:               hint,
		Context:            richContext,
		GitBranch:          gitBranch,
		GitWorkflowApplied: gitWorkflowApplied,
	}, nil
}

// Current gets the current in-progress task for a session.
func (a *TaskApp) Current(ctx context.Context, sessionID, planID string) (*TaskResult, error) {
	repo := a.ctx.Repo

	// Try to find by session ID first
	if sessionID != "" {
		currentTask, err := repo.GetCurrentTask(sessionID)
		if err != nil {
			return nil, fmt.Errorf("get current task by session: %w", err)
		}
		if currentTask != nil {
			plan, _ := repo.GetPlan(currentTask.PlanID)
			return &TaskResult{
				Success: true,
				Task:    currentTask,
				Plan:    plan,
				Context: a.buildRichContext(ctx, currentTask, plan),
			}, nil
		}
	}

	// Fallback: find any in-progress task in the plan
	if planID == "" {
		activePlan, err := repo.GetActivePlan()
		if err != nil {
			return nil, fmt.Errorf("get active plan: %w", err)
		}
		if activePlan == nil {
			return &TaskResult{
				Success: false,
				Message: "No active plan found.",
			}, nil
		}
		planID = activePlan.ID
	}

	inProgressTask, err := repo.GetAnyInProgressTask(planID)
	if err != nil {
		return nil, fmt.Errorf("get in-progress task: %w", err)
	}
	if inProgressTask == nil {
		return &TaskResult{
			Success: true,
			Message: "No task currently in progress.",
			Hint:    "Use task next to get the next pending task.",
		}, nil
	}

	plan, _ := repo.GetPlan(inProgressTask.PlanID)
	return &TaskResult{
		Success: true,
		Task:    inProgressTask,
		Plan:    plan,
		Message: "Found in-progress task (may be from a different session).",
		Context: a.buildRichContext(ctx, inProgressTask, plan),
	}, nil
}

// Start claims a specific task for a session.
func (a *TaskApp) Start(ctx context.Context, opts TaskStartOptions) (*TaskResult, error) {
	if opts.TaskID == "" {
		return &TaskResult{
			Success: false,
			Message: "task_id is required",
		}, nil
	}
	if opts.SessionID == "" {
		return &TaskResult{
			Success: false,
			Message: "session_id is required",
		}, nil
	}

	repo := a.ctx.Repo

	// Claim the task
	if err := repo.ClaimTask(opts.TaskID, opts.SessionID); err != nil {
		return &TaskResult{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	// Capture git baseline for accurate deviation detection
	// This records what files were already modified before task execution
	workDir, _ := os.Getwd()
	if task.IsGitRepo(workDir) {
		verifier := task.NewGitVerifier(workDir)
		baseline, err := verifier.GetActualModifications(ctx)
		if err == nil && len(baseline) > 0 {
			// Save baseline - ignore errors, this is best-effort
			_ = repo.SetGitBaseline(opts.TaskID, baseline)
		}
	}

	// Return the updated task
	startedTask, err := repo.GetTask(opts.TaskID)
	if err != nil {
		return nil, fmt.Errorf("get started task: %w", err)
	}

	plan, _ := repo.GetPlan(startedTask.PlanID)

	hint := "Call ask tool with suggested queries for relevant context."
	if len(startedTask.SuggestedAskQueries) > 0 {
		hint = fmt.Sprintf("Call ask tool with queries: %v", startedTask.SuggestedAskQueries)
	}

	return &TaskResult{
		Success: true,
		Message: "Task started successfully.",
		Task:    startedTask,
		Plan:    plan,
		Hint:    hint,
		Context: a.buildRichContext(ctx, startedTask, plan),
	}, nil
}

// List returns all tasks, optionally filtered by plan.
func (a *TaskApp) List(ctx context.Context, planID string) ([]task.Task, error) {
	repo := a.ctx.Repo

	if planID != "" {
		return repo.ListTasks(planID)
	}

	// Get all tasks across all plans
	plans, err := repo.ListPlans()
	if err != nil {
		return nil, fmt.Errorf("list plans: %w", err)
	}

	var allTasks []task.Task
	for _, p := range plans {
		tasks, err := repo.ListTasks(p.ID)
		if err != nil {
			continue
		}
		allTasks = append(allTasks, tasks...)
	}
	return allTasks, nil
}

// Complete marks a task as completed with git workflow and optional PR creation.
func (a *TaskApp) Complete(ctx context.Context, opts TaskCompleteOptions) (*TaskResult, error) {
	if opts.TaskID == "" {
		return &TaskResult{
			Success: false,
			Message: "task_id is required",
		}, nil
	}

	repo := a.ctx.Repo

	// Get task before completing (need title for commit message)
	taskBeforeComplete, err := repo.GetTask(opts.TaskID)
	if err != nil {
		return &TaskResult{
			Success: false,
			Message: fmt.Sprintf("task not found: %v", err),
		}, nil
	}

	// Get plan early - needed for policy enforcement
	plan, err := repo.GetPlan(taskBeforeComplete.PlanID)
	if err != nil {
		return nil, fmt.Errorf("get plan: %w", err)
	}

	// Get working directory for policy engine and git operations
	workDir, _ := os.Getwd()

	// === Policy Enforcement (BEFORE database write) ===
	// Create a task object with the files_modified from completion options
	taskForPolicy := &task.Task{
		ID:            taskBeforeComplete.ID,
		Title:         taskBeforeComplete.Title,
		Description:   taskBeforeComplete.Description,
		FilesModified: opts.FilesModified,
	}

	// Create policy engine from .taskwing/policies/
	policyEngine, policyErr := policy.NewEngine(policy.EngineConfig{
		WorkDir: workDir,
	})

	// Log warning if policy engine fails to load (silent failure is dangerous)
	if policyErr != nil {
		log.Printf("[WARN] Policy engine failed to load: %v. Policies will NOT be enforced.", policyErr)
	}

	// Only enforce if policies are loaded (no error and policies exist)
	if policyErr == nil && policyEngine.PolicyCount() > 0 {
		// Create the adapter and enforcer
		adapter := policy.NewPolicyEvaluatorAdapter(policyEngine, nil, "")
		enforcer := task.NewPolicyEnforcer(adapter, "")

		// Enforce policies
		result := enforcer.Enforce(ctx, taskForPolicy, plan.Goal)
		if !result.Allowed {
			var violationMsg string
			var policyErrors []string

			if result.Error != nil {
				violationMsg = fmt.Sprintf("Policy evaluation error: %v", result.Error)
				policyErrors = []string{result.Error.Error()}
			} else if len(result.Violations) > 0 {
				policyErrors = result.Violations
				// Format violations as a readable list
				violationMsg = "Policy violations blocked task completion:\n"
				for i, v := range result.Violations {
					violationMsg += fmt.Sprintf("  %d. %s\n", i+1, v)
				}
				violationMsg += "\nTask remains in_progress. Fix the violations and retry."
			} else {
				violationMsg = "Policy violations detected (no details provided)"
				policyErrors = []string{"Unknown policy violation"}
			}
			return &TaskResult{
				Success:         false,
				Message:         violationMsg,
				Task:            taskBeforeComplete,
				PolicyViolation: true,
				PolicyErrors:    policyErrors,
			}, nil
		}
	}
	// === End Policy Enforcement ===

	// Complete the task in SQLite
	if err := repo.CompleteTask(opts.TaskID, opts.Summary, opts.FilesModified); err != nil {
		return &TaskResult{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	// Get the completed task
	completedTask, err := repo.GetTask(opts.TaskID)
	if err != nil {
		return nil, fmt.Errorf("get completed task: %w", err)
	}

	// Run Sentinel analysis with git verification to detect deviations from plan
	// Git verification catches cases where an agent lies about what files it modified
	sentinel := task.NewSentinel()
	sentinelReport := sentinel.AnalyzeWithVerification(ctx, completedTask, workDir)

	// Git auto-commit and push
	var gitBranch string
	var gitCommitApplied bool
	var gitPushApplied bool

	gitClient := git.NewClient(workDir)

	if gitClient.IsRepository() {
		// Get current branch for push
		currentBranch, branchErr := gitClient.CurrentBranch()
		if branchErr == nil {
			gitBranch = currentBranch
		}

		// Commit task progress with conventional commit message
		if err := gitClient.CommitTaskProgress(taskBeforeComplete.Title, taskBeforeComplete.Scope); err == nil {
			gitCommitApplied = true
		}

		// Push to remote if we have a branch and commit was successful
		if gitCommitApplied && gitBranch != "" {
			if err := gitClient.PushTaskProgress(gitBranch); err == nil {
				gitPushApplied = true
			}
		}
	}

	pendingCount := 0
	inProgressCount := 0
	for _, t := range plan.Tasks {
		switch t.Status {
		case task.StatusPending:
			pendingCount++
		case task.StatusInProgress:
			inProgressCount++
		}
	}

	// PR creation variables
	var prURL string
	var prCreated bool

	// Audit variables
	var auditTriggered bool
	var auditStatus string
	var auditPlanStatus task.PlanStatus

	hint := "Great work! "
	if pendingCount > 0 {
		hint += fmt.Sprintf("There are %d more pending tasks. Use task action=next to continue.", pendingCount)
	} else if inProgressCount > 0 {
		hint += fmt.Sprintf("There are %d tasks still in progress.", inProgressCount)
	} else {
		// All tasks complete - run audit first, then create PR if verified
		hint += "All tasks in this plan are complete!"

		// Trigger automatic audit
		auditTriggered = true
		hint += " Running audit verification..."

		// Create audit service and run with auto-fix
		auditService := impl.NewService(workDir, a.ctx.LLMCfg)
		auditCtx, auditCancel := context.WithTimeout(ctx, 5*time.Minute)
		defer auditCancel()

		auditResult, auditErr := auditService.AuditWithAutoFix(auditCtx, plan)
		if auditErr != nil {
			auditStatus = "error"
			hint += fmt.Sprintf(" Audit failed: %v", auditErr)
		} else {
			auditStatus = auditResult.FinalStatus
			if auditResult.FinalStatus == "verified" {
				auditPlanStatus = task.PlanStatusVerified
				hint += " Plan VERIFIED - all checks passed!"
			} else {
				auditPlanStatus = task.PlanStatusNeedsRevision
				hint += fmt.Sprintf(" Plan needs revision after %d fix attempts.", auditResult.Attempts)
				if auditResult.FinalAudit != nil && len(auditResult.FinalAudit.SemanticResult.Issues) > 0 {
					hint += fmt.Sprintf(" Issues: %v", auditResult.FinalAudit.SemanticResult.Issues)
				}
			}

			// Store audit report in database
			auditReport := auditResult.ToAuditReportWithFixes()
			reportJSON, marshalErr := json.Marshal(auditReport)
			if marshalErr == nil {
				_ = repo.UpdatePlanAuditReport(plan.ID, auditPlanStatus, string(reportJSON))
			}
		}

		// Only create PR if audit passed
		if auditStatus == "verified" && gitClient.IsRepository() && gitClient.IsGhInstalled() {
			// Gather completed tasks for PR body
			var taskInfos []git.TaskInfo
			for _, t := range plan.Tasks {
				if t.Status == task.StatusCompleted {
					taskInfos = append(taskInfos, git.TaskInfo{
						Title:   t.Title,
						Summary: t.CompletionSummary,
					})
				}
			}

			// Create the PR
			prInfo, prErr := gitClient.CreatePlanPR(plan.Goal, taskInfos, "")
			if prErr != nil {
				hint += " PR creation failed - you can create it manually with 'gh pr create'."
			} else if prInfo != nil {
				prURL = prInfo.URL
				prCreated = true
				hint += fmt.Sprintf(" PR created: %s", prURL)
			}
		} else if auditStatus != "verified" {
			hint += " PR not created - fix issues and run plan action=audit again."
		} else if !gitClient.IsGhInstalled() {
			hint += " Install 'gh' CLI to auto-create PRs."
		}
	}

	// Build message with git status
	message := "Task completed successfully."
	if gitCommitApplied {
		message += " Changes committed."
		if gitPushApplied {
			message += " Pushed to origin."
		}
	}
	if auditTriggered {
		switch auditStatus {
		case "verified":
			message += " Audit passed."
		case "needs_revision":
			message += " Audit found issues."
		case "error":
			message += " Audit encountered an error."
		}
	}
	if prCreated {
		message += " PR created."
	}

	// Add Sentinel deviation warning to message and hint
	if sentinelReport.HasDeviations() {
		message += fmt.Sprintf(" Sentinel: %s", sentinelReport.Summary)
		if sentinelReport.HasCriticalDeviations() {
			hint += " WARNING: Critical deviations detected - review before proceeding."
		}
	}

	return &TaskResult{
		Success:            true,
		Message:            message,
		Task:               completedTask,
		Plan:               plan,
		Hint:               hint,
		GitBranch:          gitBranch,
		GitWorkflowApplied: gitCommitApplied,
		PRURL:              prURL,
		PRCreated:          prCreated,
		AuditTriggered:     auditTriggered,
		AuditStatus:        auditStatus,
		AuditPlanStatus:    auditPlanStatus,
		SentinelReport:     sentinelReport,
	}, nil
}

// executeGitWorkflow handles git branch creation for a plan.
func (a *TaskApp) executeGitWorkflow(plan *task.Plan, skipUnpushedCheck bool) (*git.WorkflowResult, error) {
	workDir, _ := os.Getwd()
	gitClient := git.NewClient(workDir)

	if !gitClient.IsRepository() {
		return nil, nil // Not a git repo, skip silently
	}

	// Check if this is the first task (no tasks in progress or completed)
	isFirstTask := true
	for _, t := range plan.Tasks {
		if t.Status == task.StatusInProgress || t.Status == task.StatusCompleted {
			isFirstTask = false
			break
		}
	}

	// Generate expected branch name
	expectedBranch := git.GenerateBranchName(plan.ID, plan.Goal)
	currentBranch, _ := gitClient.CurrentBranch()

	// Only run workflow if first task or not on expected branch
	if !isFirstTask && currentBranch == expectedBranch {
		return &git.WorkflowResult{BranchName: currentBranch}, nil
	}

	return gitClient.StartPlanWorkflow(plan.ID, plan.Goal, skipUnpushedCheck)
}

// buildRichContext creates markdown context for a task using AskApp.
func (a *TaskApp) buildRichContext(ctx context.Context, t *task.Task, plan *task.Plan) string {
	if plan == nil {
		return ""
	}

	// Create a search function that uses AskApp
	askApp := NewAskApp(a.ctx)
	searchFunc := func(ctx context.Context, query string, limit int) ([]task.AskResult, error) {
		result, err := askApp.Query(ctx, query, AskOptions{
			Limit:          limit,
			GenerateAnswer: false,
		})
		if err != nil {
			return nil, err
		}

		var adapted []task.AskResult
		for _, r := range result.Results {
			adapted = append(adapted, task.AskResult{
				Summary: r.Summary,
				Type:    r.Type,
				Content: r.Content,
			})
		}
		return adapted, nil
	}

	return task.FormatRichContext(ctx, t, plan, searchFunc)
}
