package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/josephgoksu/TaskWing/models"
	"github.com/josephgoksu/TaskWing/prompts"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

var (
	improveTaskID string
	improveApply  bool
	improvePlan   bool
)

// improveCmd enhances an existing task using AI and optionally creates subtasks
var improveCmd = &cobra.Command{
	Use:   "improve [task-id]",
	Short: "AI-enhance an existing task and optionally generate subtasks",
	Long:  "Select a task and use AI to refine its title, description, acceptance criteria, and priority. Optionally generate a concise subtask plan.",
	Example: `  # Interactive: pick a task to improve
  taskwing improve

  # Improve specific task and preview changes
  taskwing improve 1855a043

  # Auto-apply enhancements and create a plan
  taskwing improve 1855a043 --apply --plan`,
	RunE: func(cmd *cobra.Command, args []string) error {
		st, err := GetStore()
		if err != nil {
			return err
		}
		defer func() { _ = st.Close() }()

		// Resolve target task
		var tid string
		if len(args) > 0 {
			tid = args[0]
		} else if strings.TrimSpace(improveTaskID) != "" {
			tid = improveTaskID
		}

		var task models.Task
		if strings.TrimSpace(tid) == "" {
			// Interactive selection: any non-done task
			t, err := selectTaskInteractive(st, func(t models.Task) bool { return t.Status != models.StatusDone }, "Select a task to improve")
			if err != nil {
				return err
			}
			task = t
		} else {
			resolved, err := resolveTaskID(st, tid)
			if err != nil {
				return fmt.Errorf("task '%s' not found: %w", tid, err)
			}
			t, err := st.GetTask(resolved)
			if err != nil {
				return fmt.Errorf("task '%s' not found: %w", tid, err)
			}
			task = t
		}

		// Build provider and prompt
		prov, err := createLLMProvider(&GetConfig().LLM)
		if err != nil {
			return fmt.Errorf("llm provider: %w", err)
		}
		sysPrompt, err := prompts.GetPrompt(prompts.KeyEnhanceTask, GetConfig().Project.RootDir+"/"+GetConfig().Project.TemplatesDir)
		if err != nil {
			return fmt.Errorf("load prompt: %w", err)
		}

		// Compose input and context
		input := task.Title
		if ds := strings.TrimSpace(task.Description); ds != "" && ds != task.Title {
			input = fmt.Sprintf("%s\n\n%s", input, ds)
		}
		if ac := strings.TrimSpace(task.AcceptanceCriteria); ac != "" {
			input = fmt.Sprintf("%s\n\nAcceptance Criteria:\n%s", input, ac)
		}
		ctxInfo, _ := BuildTaskContext(st)
		var ctxStr string
		if ctxInfo != nil {
			ctxStr = fmt.Sprintf("Project: %d tasks, %d done", ctxInfo.TotalTasks, ctxInfo.TasksByStatus[string(models.StatusDone)])
		}

		// Call AI
		fmt.Print("🔄 Enhancing task with AI... ")
		enhanced, err := prov.EnhanceTask(context.Background(), sysPrompt, input, ctxStr, GetConfig().LLM.ModelName, GetConfig().LLM.APIKey, GetConfig().LLM.ProjectID, 1024, 0.3)
		if err != nil {
			fmt.Print("\r")
			return fmt.Errorf("enhancement failed: %w", err)
		}
		fmt.Print("\r")

		// Preview changes
		fmt.Printf("Proposed enhancements for %s (%s):\n", task.Title, task.ID[:8])
		fmt.Printf("  Title: %s\n", fallback(enhanced.Title, task.Title))
		if strings.TrimSpace(enhanced.Description) != "" {
			// Flatten newlines to keep preview compact
			flat := strings.ReplaceAll(enhanced.Description, "\n", " ")
			fmt.Printf("  Description: %s\n", summarize(flat, 140))
		}
		if strings.TrimSpace(enhanced.AcceptanceCriteria) != "" {
			fmt.Printf("  Acceptance Criteria:\n")
			for _, line := range strings.Split(enhanced.AcceptanceCriteria, "\n") {
				if strings.TrimSpace(line) != "" {
					fmt.Printf("    %s\n", line)
				}
			}
		}
		if strings.TrimSpace(enhanced.Priority) != "" {
			fmt.Printf("  Priority: %s\n", strings.ToLower(enhanced.Priority))
		}

		if !improveApply {
			// Interactive confirm by default
			confirm := promptui.Prompt{Label: "Apply these changes?", IsConfirm: true, Default: "y"}
			if _, err := confirm.Run(); err != nil {
				fmt.Println("\nPreview only. Use --apply to update the task.")
				return nil
			}
			improveApply = true
		}

		// Apply updates
		updates := map[string]interface{}{}
		if strings.TrimSpace(enhanced.Title) != "" && enhanced.Title != task.Title {
			updates["title"] = enhanced.Title
		}
		if strings.TrimSpace(enhanced.Description) != "" {
			cleaned := sanitizeDesc(enhanced.Description)
			if cleaned != task.Description {
				updates["description"] = cleaned
			}
		} else {
			// Even if description wasn't changed by AI, clean existing description if polluted
			cleaned := sanitizeDesc(task.Description)
			if cleaned != task.Description {
				updates["description"] = cleaned
			}
		}
		if strings.TrimSpace(enhanced.AcceptanceCriteria) != "" && enhanced.AcceptanceCriteria != task.AcceptanceCriteria {
			updates["acceptanceCriteria"] = enhanced.AcceptanceCriteria
		}
		if p := strings.ToLower(strings.TrimSpace(enhanced.Priority)); p != "" && p != strings.ToLower(string(task.Priority)) {
			updates["priority"] = p
		}

		if len(updates) == 0 {
			fmt.Println("No changes to apply.")
		} else {
			if _, err := st.UpdateTask(task.ID, updates); err != nil {
				return fmt.Errorf("apply updates: %w", err)
			}
			fmt.Println("✅ Task updated with AI enhancements.")
		}

		// Optional subtask planning
		if improvePlan {
			fmt.Println("\n📋 Generating plan...")
			// Set plan command flags and execute directly to share the same context
			planTaskID = task.ID
			planConfirm = true
			planCount = 5 // default value
			if err := planCmd.RunE(planCmd, []string{task.ID}); err != nil {
				return fmt.Errorf("plan failed: %w", err)
			}
		} else {
			fmt.Printf("\n💡 Next: preview a plan with 'taskwing plan %s' or apply with '--confirm'.\n", task.ID[:8])
		}

		return nil
	},
}

func fallback(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return v
}

// sanitizeDesc removes embedded acceptance criteria blocks from descriptions
func sanitizeDesc(s string) string {
	low := strings.ToLower(s)
	key := strings.ToLower("Acceptance Criteria:")
	if idx := strings.Index(low, key); idx >= 0 {
		return strings.TrimSpace(s[:idx])
	}
	return strings.TrimSpace(s)
}

func init() {
	rootCmd.AddCommand(improveCmd)
	improveCmd.Flags().StringVar(&improveTaskID, "task", "", "Task id to improve")
	improveCmd.Flags().BoolVar(&improveApply, "apply", false, "Apply enhancements (default: preview only)")
	improveCmd.Flags().BoolVar(&improvePlan, "plan", false, "After applying, generate and create a subtask plan")
	// Alias for convenience (matches plan's --confirm mental model)
	improveCmd.Flags().BoolVar(&improveApply, "confirm", false, "Apply enhancements (alias of --apply)")
}
