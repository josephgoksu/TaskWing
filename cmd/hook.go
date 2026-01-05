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
	"time"

	"github.com/josephgoksu/TaskWing/internal/config"
	"github.com/josephgoksu/TaskWing/internal/knowledge"
	"github.com/josephgoksu/TaskWing/internal/llm"
	"github.com/josephgoksu/TaskWing/internal/memory"
	"github.com/josephgoksu/TaskWing/internal/task"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// HookSession tracks session state for circuit breakers
type HookSession struct {
	SessionID      string    `json:"session_id"`
	StartedAt      time.Time `json:"started_at"`
	TasksCompleted int       `json:"tasks_completed"`
	TasksStarted   int       `json:"tasks_started"`
	CurrentTaskID  string    `json:"current_task_id,omitempty"`
	PlanID         string    `json:"plan_id,omitempty"`
}

// HookResponse is the JSON response format for Claude Code hooks
type HookResponse struct {
	Decision string `json:"decision"`          // "approve" or "block"
	Reason   string `json:"reason,omitempty"`  // Explanation
	Context  string `json:"context,omitempty"` // Additional context to inject (for block)
}

// Circuit breaker defaults
const (
	DefaultMaxTasksPerSession = 5
	DefaultMaxSessionMinutes  = 30
)

var hookCmd = &cobra.Command{
	Use:   "hook",
	Short: "Hook commands for Claude Code integration",
	Long: `Commands designed to be called by Claude Code hooks for autonomous task execution.

These commands enable TaskWing to work with Claude Code's hook system to create
an autonomous task execution loop with appropriate circuit breakers.

Example .claude/settings.json configuration:
{
  "hooks": {
    "Stop": [{
      "hooks": [{
        "type": "command",
        "command": "taskwing hook continue-check",
        "timeout": 10
      }]
    }],
    "SessionStart": [{
      "hooks": [{
        "type": "command",
        "command": "taskwing hook session-init"
      }]
    }]
  }
}`,
}

var hookContinueCheckCmd = &cobra.Command{
	Use:   "continue-check",
	Short: "Check if Claude should continue to next task (for Stop hook)",
	Long: `Called by Claude Code's Stop hook to determine if execution should continue.

Returns JSON with decision "approve" (allow stop) or "block" (inject next task).
Implements circuit breakers for:
- Maximum tasks per session (default: 5)
- Maximum session duration (default: 30 minutes)
- Blocked task detection`,
	RunE: func(cmd *cobra.Command, args []string) error {
		maxTasks, _ := cmd.Flags().GetInt("max-tasks")
		maxMinutes, _ := cmd.Flags().GetInt("max-minutes")

		return runContinueCheck(maxTasks, maxMinutes)
	},
}

var hookSessionInitCmd = &cobra.Command{
	Use:   "session-init",
	Short: "Initialize session tracking (for SessionStart hook)",
	Long:  `Called by Claude Code's SessionStart hook to initialize session state.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runSessionInit()
	},
}

var hookSessionEndCmd = &cobra.Command{
	Use:   "session-end",
	Short: "End session and cleanup (for SessionEnd hook)",
	Long:  `Called by Claude Code's SessionEnd hook to cleanup session state.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runSessionEnd()
	},
}

var hookStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current hook session status",
	RunE: func(cmd *cobra.Command, args []string) error {
		session, err := loadHookSession()
		if err != nil {
			return printJSON(map[string]any{
				"active":  false,
				"message": "No active session",
			})
		}

		elapsed := time.Since(session.StartedAt)
		return printJSON(map[string]any{
			"active":          true,
			"session_id":      session.SessionID,
			"started_at":      session.StartedAt.Format(time.RFC3339),
			"elapsed_minutes": int(elapsed.Minutes()),
			"tasks_completed": session.TasksCompleted,
			"tasks_started":   session.TasksStarted,
			"current_task_id": session.CurrentTaskID,
			"plan_id":         session.PlanID,
		})
	},
}

func init() {
	rootCmd.AddCommand(hookCmd)
	hookCmd.AddCommand(hookContinueCheckCmd)
	hookCmd.AddCommand(hookSessionInitCmd)
	hookCmd.AddCommand(hookSessionEndCmd)
	hookCmd.AddCommand(hookStatusCmd)

	// Circuit breaker flags
	hookContinueCheckCmd.Flags().Int("max-tasks", DefaultMaxTasksPerSession, "Maximum tasks to complete per session")
	hookContinueCheckCmd.Flags().Int("max-minutes", DefaultMaxSessionMinutes, "Maximum session duration in minutes")
}

// runContinueCheck implements the main circuit breaker logic
func runContinueCheck(maxTasks, maxMinutes int) error {
	// Load session state
	session, err := loadHookSession()
	if err != nil {
		// Auto-initialize session on first continue-check call
		// This is necessary because Claude Code does NOT support SessionStart events
		// (only PreToolUse, PostToolUse, Notification, Stop are valid)
		fmt.Fprintf(os.Stderr, "[INFO] No active session, auto-initializing...\n")
		if initErr := runSessionInit(); initErr != nil {
			return outputHookResponse(HookResponse{
				Decision: "approve",
				Reason:   fmt.Sprintf("Failed to auto-initialize session: %v", initErr),
			})
		}
		// Reload session after init
		session, err = loadHookSession()
		if err != nil {
			return outputHookResponse(HookResponse{
				Decision: "approve",
				Reason:   fmt.Sprintf("Session initialization succeeded but failed to load: %v", err),
			})
		}
	}

	// Circuit breaker 1: Max tasks reached
	if session.TasksCompleted >= maxTasks {
		return outputHookResponse(HookResponse{
			Decision: "approve",
			Reason:   fmt.Sprintf("Circuit breaker: Completed %d/%d tasks this session. Take a break for human review.", session.TasksCompleted, maxTasks),
		})
	}

	// Circuit breaker 2: Max duration reached
	elapsed := time.Since(session.StartedAt)
	if int(elapsed.Minutes()) >= maxMinutes {
		return outputHookResponse(HookResponse{
			Decision: "approve",
			Reason:   fmt.Sprintf("Circuit breaker: Session duration %d/%d minutes. Take a break for human review.", int(elapsed.Minutes()), maxMinutes),
		})
	}

	// Open repository
	repo, err := openRepo()
	if err != nil {
		return outputHookResponse(HookResponse{
			Decision: "approve",
			Reason:   fmt.Sprintf("Failed to open repository: %v", err),
		})
	}
	defer func() { _ = repo.Close() }()

	// Check for active plan
	activePlan, err := repo.GetActivePlan()
	if err != nil || activePlan == nil {
		return outputHookResponse(HookResponse{
			Decision: "approve",
			Reason:   "No active plan. Use 'taskwing plan start <plan-id>' to set one.",
		})
	}

	// Check current task status
	var currentTask *task.Task
	if session.CurrentTaskID != "" {
		var err error
		currentTask, err = repo.GetTask(session.CurrentTaskID)
		if err != nil && viper.GetBool("verbose") {
			fmt.Fprintf(os.Stderr, "[DEBUG] Could not load current task %s: %v\n", session.CurrentTaskID, err)
		}
	}

	// Get next pending task
	nextTask, err := repo.GetNextTask(activePlan.ID)
	if err != nil {
		return outputHookResponse(HookResponse{
			Decision: "approve",
			Reason:   fmt.Sprintf("Error getting next task: %v", err),
		})
	}

	// No more tasks = plan complete
	if nextTask == nil {
		// Check if all tasks are done
		allDone := true
		for _, t := range activePlan.Tasks {
			if t.Status != task.StatusCompleted {
				allDone = false
				break
			}
		}
		if allDone {
			return outputHookResponse(HookResponse{
				Decision: "approve",
				Reason:   fmt.Sprintf("Plan complete! All %d tasks finished.", len(activePlan.Tasks)),
			})
		}
		return outputHookResponse(HookResponse{
			Decision: "approve",
			Reason:   "No pending tasks available. Some tasks may be blocked or have unmet dependencies.",
		})
	}

	// Build context for next task
	contextStr := buildTaskContext(repo, nextTask, activePlan)

	// Update session state
	if currentTask != nil && currentTask.Status == task.StatusCompleted {
		session.TasksCompleted++
	}
	session.CurrentTaskID = nextTask.ID
	session.PlanID = activePlan.ID
	session.TasksStarted++
	if err := saveHookSession(session); err != nil {
		// Log to stderr but don't fail - hook must return valid JSON
		fmt.Fprintf(os.Stderr, "[WARN] Failed to save session state: %v\n", err)
	}

	// Return block with next task context
	return outputHookResponse(HookResponse{
		Decision: "block",
		Reason:   fmt.Sprintf("Continue to task %d/%d: %s", session.TasksCompleted+1, len(activePlan.Tasks), nextTask.Title),
		Context:  contextStr,
	})
}

// buildTaskContext creates the context string to inject for the next task.
// Delegates to task.FormatRichContext for consistent presentation across CLI and MCP.
func buildTaskContext(repo *memory.Repository, nextTask *task.Task, plan *task.Plan) string {
	ctx := context.Background()

	// Get knowledge service for recall context
	llmCfg, _ := getLLMConfigFromViper()
	ks := knowledge.NewService(repo, llmCfg)

	// Create search adapter that wraps knowledge.Service for the task package
	searchFn := func(ctx context.Context, query string, limit int) ([]task.RecallResult, error) {
		searchCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		results, err := ks.Search(searchCtx, query, limit)
		if err != nil {
			return nil, err
		}
		var adapted []task.RecallResult
		for _, r := range results {
			adapted = append(adapted, task.RecallResult{
				Summary: r.Node.Summary,
				Type:    r.Node.Type,
				Content: r.Node.Content,
			})
		}
		return adapted, nil
	}

	return task.FormatRichContext(ctx, nextTask, plan, searchFn)
}

// runSessionInit initializes a new hook session
func runSessionInit() error {
	// Check if session already exists
	existingSession, loadErr := loadHookSession()
	if loadErr == nil && existingSession != nil {
		elapsed := time.Since(existingSession.StartedAt)
		fmt.Fprintf(os.Stderr, "[WARN] Overwriting existing session %s (started %d minutes ago, %d tasks completed)\n",
			existingSession.SessionID, int(elapsed.Minutes()), existingSession.TasksCompleted)
	}

	session := HookSession{
		SessionID:      fmt.Sprintf("session-%d", time.Now().Unix()),
		StartedAt:      time.Now(),
		TasksCompleted: 0,
		TasksStarted:   0,
	}

	// Check for active plan and set it
	repo, repoErr := openRepo()
	if repoErr == nil {
		defer func() { _ = repo.Close() }()
		if plan, planErr := repo.GetActivePlan(); planErr == nil && plan != nil {
			session.PlanID = plan.ID
		}
	}

	if err := saveHookSession(&session); err != nil {
		return fmt.Errorf("failed to save session: %w", err)
	}

	// Output context for SessionStart (gets added to conversation)
	// Note: Circuit breaker values shown are defaults; actual values depend on hook config
	planInfo := session.PlanID
	if planInfo == "" {
		planInfo = "(none - use 'taskwing plan start <id>' to set)"
	}

	fmt.Printf(`TaskWing Session Initialized
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Session ID: %s
Started: %s
Active Plan: %s

The Stop hook is configured to automatically continue to the next task.
Circuit breakers are configured in .claude/settings.json (defaults: %d tasks, %d min).

Use /tw-next to start the first task, or it will auto-continue after each task.
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
`, session.SessionID, session.StartedAt.Format("15:04:05"), planInfo, DefaultMaxTasksPerSession, DefaultMaxSessionMinutes)

	return nil
}

// runSessionEnd cleans up session state
func runSessionEnd() error {
	session, err := loadHookSession()
	if err != nil {
		return nil // No session to end
	}

	elapsed := time.Since(session.StartedAt)

	// Output summary
	fmt.Printf(`TaskWing Session Complete
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Session: %s
Duration: %d minutes
Tasks Completed: %d
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
`, session.SessionID, int(elapsed.Minutes()), session.TasksCompleted)

	// Remove session file
	sessionPath := getHookSessionPath()
	_ = os.Remove(sessionPath)

	return nil
}

// Session persistence helpers

func getHookSessionPath() string {
	return filepath.Join(config.GetMemoryBasePath(), "hook_session.json")
}

func loadHookSession() (*HookSession, error) {
	data, err := os.ReadFile(getHookSessionPath())
	if err != nil {
		return nil, err
	}

	var session HookSession
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, err
	}

	return &session, nil
}

func saveHookSession(session *HookSession) error {
	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return err
	}

	sessionPath := getHookSessionPath()
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(sessionPath), 0755); err != nil {
		return fmt.Errorf("create session directory: %w", err)
	}

	return os.WriteFile(sessionPath, data, 0644)
}

func outputHookResponse(resp HookResponse) error {
	data, err := json.Marshal(resp)
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

// getLLMConfigFromViper returns LLM config without requiring cobra command
func getLLMConfigFromViper() (llm.Config, error) {
	return config.LoadLLMConfig()
}
