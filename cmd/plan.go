package cmd

import (
	"context"
	"fmt"
	"sort"
	"strings"

	taskwingmcp "github.com/josephgoksu/TaskWing/mcp"
	"github.com/josephgoksu/TaskWing/models"
	"github.com/josephgoksu/TaskWing/prompts"
	"github.com/spf13/cobra"
)

var (
	planTaskID  string
	planCount   int
	planConfirm bool
)

// planCmd: Generate a concise set of subtasks for a parent task
var planCmd = &cobra.Command{
	Use:   "plan [task-id]",
	Short: "Generate a precise, actionable plan (subtasks) for a task",
	Long:  `Generate AI-powered subtasks for any task. Creates 3-7 focused, actionable steps with automatic dependencies.`,
	Example: `  # Interactive mode - select from todo tasks
  taskwing plan
  
  # Plan specific task
  taskwing plan abc123
  
  # Generate and apply immediately
  taskwing plan abc123 --confirm
  
  # Control number of steps (3-7)
  taskwing plan --count 5`,
	RunE: func(cmd *cobra.Command, args []string) error {
		st, err := GetStore()
		if err != nil {
			return err
		}
		defer func() { _ = st.Close() }()

		// Select parent task - support positional argument
		var taskID string
		if len(args) > 0 {
			taskID = args[0]
		} else if planTaskID != "" {
			taskID = planTaskID
		}

		var parent models.Task
		if strings.TrimSpace(taskID) == "" {
			// interactive: pick any non-done task
			p, err := selectTaskInteractive(st, func(t models.Task) bool { return t.Status != models.StatusDone && t.ParentID == nil }, "Select task to plan")
			if err != nil {
				return err
			}
			parent = p
		} else {
			// Try to resolve the task ID (supports partial IDs)
			resolved, err := resolveTaskID(st, taskID)
			if err != nil {
				return fmt.Errorf("task '%s' not found: %w", taskID, err)
			}
			p, err := st.GetTask(resolved)
			if err != nil {
				return fmt.Errorf("task '%s' not found: %w", taskID, err)
			}
			parent = p
		}

		// Build inputs
		ctx := context.Background()
		sysPrompt, err := prompts.GetPrompt(prompts.KeyBreakdownTask, GetConfig().Project.RootDir+"/"+GetConfig().Project.TemplatesDir)
		if err != nil {
			return fmt.Errorf("load prompt: %w", err)
		}

		// Build lightweight context string using existing builder
		taskCtx, err := taskwingmcp.BuildTaskContext(st)
		if err != nil {
			return fmt.Errorf("build context: %w", err)
		}

		// Compose context info
		var b strings.Builder
		b.WriteString("Parent Task:\n")
		b.WriteString(parent.Title)
		b.WriteString("\n\nDescription:\n")
		b.WriteString(parent.Description)
		if ac := strings.TrimSpace(parent.AcceptanceCriteria); ac != "" {
			b.WriteString("\n\nAcceptance Criteria:\n")
			b.WriteString(ac)
		}
		b.WriteString("\n\nProject Summary:\n")
		b.WriteString(fmt.Sprintf("Total tasks: %d, Done: %d\n", taskCtx.TotalTasks, taskCtx.TasksByStatus[string(models.StatusDone)]))

		// LLM provider
		provider, err := createLLMProvider(&GetConfig().LLM)
		if err != nil {
			return fmt.Errorf("llm provider: %w", err)
		}

		// Ask for subtasks using BreakdownTask
		if !planConfirm {
			fmt.Print("🔄 Analyzing task and generating plan... ")
		} else {
			fmt.Print("🔄 Generating and applying plan... ")
		}
		subs, err := provider.BreakdownTask(ctx, sysPrompt, parent.Title, parent.Description, parent.AcceptanceCriteria, b.String(), GetConfig().LLM.ModelName, GetConfig().LLM.APIKey, GetConfig().LLM.ProjectID, GetConfig().LLM.MaxOutputTokens, GetConfig().LLM.Temperature)
		if err != nil {
			fmt.Print("\r") // Clear loading message
			return fmt.Errorf("plan generation failed: %w", err)
		}
		fmt.Print("\r") // Clear loading message

		// Trim to count (default 5, min 3, max 7)
		n := planCount
		if n <= 0 {
			n = 5
		}
		if n < 3 {
			n = 3
		}
		if n > 7 {
			n = 7
		}
		if len(subs) > n {
			subs = subs[:n]
		}

		// Preview
		fmt.Printf("Proposed %d step plan for: %s\n", len(subs), parent.Title)
		for i, s := range subs {
			fmt.Printf("  %d) %s [%s]\n", i+1, s.Title, strings.ToLower(s.Priority))
			if ds := strings.TrimSpace(s.Description); ds != "" {
				fmt.Printf("     - %s\n", summarize(ds, 120))
			}
			if ac := strings.TrimSpace(s.AcceptanceCriteria); ac != "" {
				lines := strings.Split(ac, "\n")
				for _, line := range lines {
					if strings.TrimSpace(line) != "" {
						fmt.Printf("     %s\n", line)
					}
				}
			}
		}
		if !planConfirm {
			fmt.Println("\nPreview only. Use --confirm to create subtasks.")
			return nil
		}

		// Apply: create subtasks with optional linear dependencies
		created := make([]models.Task, 0, len(subs))
		for _, s := range subs {
			t := models.Task{Title: s.Title, Description: s.Description, AcceptanceCriteria: s.AcceptanceCriteria, Status: models.StatusTodo, Priority: mapPriorityOrDefault(s.Priority)}
			pid := parent.ID
			t.ParentID = &pid
			newT, err := st.CreateTask(t)
			if err != nil {
				return fmt.Errorf("create subtask '%s': %w", t.Title, err)
			}
			created = append(created, newT)
		}
		// Linear deps (step i depends on i-1)
		for i := 1; i < len(created); i++ {
			prev := created[i-1].ID
			if _, err := st.UpdateTask(created[i].ID, map[string]interface{}{"dependencies": append(created[i].Dependencies, prev)}); err != nil {
				return fmt.Errorf("link dependency: %w", err)
			}
		}
		// Show result ordered by CreatedAt
		sort.Slice(created, func(i, j int) bool { return created[i].CreatedAt.Before(created[j].CreatedAt) })
		fmt.Printf("\n✅ Created %d subtasks under '%s':\n", len(created), parent.Title)
		for i, c := range created {
			depInfo := ""
			if i > 0 {
				depInfo = fmt.Sprintf(" [depends on step %d]", i)
			}
			fmt.Printf("  %d) %s (%s)%s\n", i+1, c.Title, c.ID[:8], depInfo)
		}
		fmt.Printf("\n💡 View all subtasks: taskwing ls --parent %s\n", parent.ID[:8])
		if len(created) > 0 {
			fmt.Printf("💡 Start first task:  taskwing start %s\n", created[0].ID[:8])
		}
		return nil
	},
}

func mapPriorityOrDefault(p string) models.TaskPriority {
	switch strings.ToLower(strings.TrimSpace(p)) {
	case "urgent":
		return models.PriorityUrgent
	case "high":
		return models.PriorityHigh
	case "medium":
		return models.PriorityMedium
	case "low":
		return models.PriorityLow
	default:
		return models.PriorityMedium
	}
}

func summarize(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

func init() {
	rootCmd.AddCommand(planCmd)
	planCmd.Flags().StringVar(&planTaskID, "task", "", "Parent task id to plan")
	planCmd.Flags().IntVar(&planCount, "count", 5, "Number of steps to propose (3-7)")
	planCmd.Flags().BoolVar(&planConfirm, "confirm", false, "Apply changes (default: preview only)")
}
