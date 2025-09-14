package cmd

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/josephgoksu/TaskWing/models"
	"github.com/josephgoksu/TaskWing/prompts"
	"github.com/spf13/cobra"
)

var (
	iterTaskID  string
	iterStep    string
	iterPrompt  string
	iterSplit   bool
	iterConfirm bool
)

// iterateCmd: refine or split a specific subtask
var iterateCmd = &cobra.Command{
	Use:   "iterate [parent-task-id] [step-index]",
	Short: "Refine or split a specific subtask with precision",
	Long:  `Iterate on a specific subtask by either refining it (improving title/description) or splitting it into 2-3 more focused steps.`,
	Example: `  # Interactive mode - select parent and step
  taskwing iterate

  # Refine first step of a task
  taskwing iterate abc123 1 --prompt "make it more specific"

  # Split a step into smaller steps
  taskwing iterate abc123 2 --split

  # Apply changes immediately
  taskwing iterate abc123 1 --confirm`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if strings.TrimSpace(iterTaskID) == "" {
			return fmt.Errorf("--task is required (parent task id)")
		}
		if strings.TrimSpace(iterStep) == "" {
			return fmt.Errorf("--step is required (index or subtask id)")
		}
		if strings.TrimSpace(iterPrompt) == "" && !iterSplit {
			return fmt.Errorf("--prompt is required for refine (omit only with --split)")
		}

		st, err := GetStore()
		if err != nil {
			return err
		}
		defer func() { _ = st.Close() }()

		parent, err := st.GetTask(iterTaskID)
		if err != nil {
			return fmt.Errorf("parent task not found: %w", err)
		}

		// Load subtasks of parent
		all, err := st.ListTasks(func(t models.Task) bool { return t.ParentID != nil && *t.ParentID == parent.ID }, nil)
		if err != nil {
			return err
		}
		sort.Slice(all, func(i, j int) bool { return all[i].CreatedAt.Before(all[j].CreatedAt) })
		if len(all) == 0 {
			return fmt.Errorf("no subtasks to iterate under '%s'", parent.Title)
		}

		// Pick target by index or id
		var target models.Task
		if idx, errConv := strconv.Atoi(iterStep); errConv == nil {
			if idx <= 0 || idx > len(all) {
				return fmt.Errorf("--step index out of range (1..%d)", len(all))
			}
			target = all[idx-1]
		} else {
			// assume ID
			found := false
			for _, t := range all {
				if t.ID == iterStep {
					target = t
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("subtask with id '%s' not found under parent", iterStep)
			}
		}

		if iterSplit {
			// Split: call BreakdownTask on the selected subtask
			sys, err := prompts.GetPrompt(prompts.KeyBreakdownTask, GetConfig().Project.RootDir+"/"+GetConfig().Project.TemplatesDir)
			if err != nil {
				return err
			}
			provider, err := createLLMProvider(&GetConfig().LLM)
			if err != nil {
				return err
			}
			ctx := context.Background()
			subs, err := provider.BreakdownTask(ctx, sys, target.Title, target.Description, target.AcceptanceCriteria, iterPrompt, GetConfig().LLM.ModelName, GetConfig().LLM.APIKey, GetConfig().LLM.ProjectID, GetConfig().LLM.MaxOutputTokens, GetConfig().LLM.Temperature)
			if err != nil {
				return fmt.Errorf("split failed: %w", err)
			}
			// Truncate to 2-3 for precision
			if len(subs) > 3 {
				subs = subs[:3]
			}
			if len(subs) < 2 {
				fmt.Println("Proposed less than 2 steps; will still preview/apply as-is.")
			}

			fmt.Printf("Proposed replacement for '%s' → %d new steps:\n", target.Title, len(subs))
			for i, s := range subs {
				fmt.Printf("  %d) %s\n", i+1, s.Title)
			}
			if !iterConfirm {
				fmt.Println("\nPreview only. Use --confirm to apply replacement.")
				return nil
			}

			// Apply: create new subtasks then delete old
			created := 0
			for _, s := range subs {
				nt := models.Task{Title: s.Title, Description: s.Description, AcceptanceCriteria: s.AcceptanceCriteria, Status: models.StatusTodo, Priority: mapPriorityOrDefault(s.Priority)}
				pid := parent.ID
				nt.ParentID = &pid
				if _, err := st.CreateTask(nt); err != nil {
					return fmt.Errorf("create split subtask: %w", err)
				}
				created++
			}
			if err := st.DeleteTask(target.ID); err != nil {
				return fmt.Errorf("failed to remove original step: %w", err)
			}
			fmt.Printf("Applied: replaced 1 step with %d new steps.\n", created)
			return nil
		}

		// Refine: EnhanceTask for a single subtask using iterPrompt as guidance
		sys, err := prompts.GetPrompt(prompts.KeyEnhanceTask, GetConfig().Project.RootDir+"/"+GetConfig().Project.TemplatesDir)
		if err != nil {
			return err
		}
		provider, err := createLLMProvider(&GetConfig().LLM)
		if err != nil {
			return err
		}
		ctx := context.Background()
		enhanced, err := provider.EnhanceTask(ctx, sys, fmt.Sprintf("%s\n\n%s", target.Title, target.Description), iterPrompt, GetConfig().LLM.ModelName, GetConfig().LLM.APIKey, GetConfig().LLM.ProjectID, GetConfig().LLM.MaxOutputTokens, GetConfig().LLM.Temperature)
		if err != nil {
			return fmt.Errorf("refine failed: %w", err)
		}

		fmt.Printf("Current: %s\nNew:     %s\n", target.Title, enhanced.Title)
		if !iterConfirm {
			fmt.Println("\nPreview only. Use --confirm to apply.")
			return nil
		}

		updates := map[string]interface{}{
			"title":              enhanced.Title,
			"description":        enhanced.Description,
			"acceptanceCriteria": enhanced.AcceptanceCriteria,
			"priority":           strings.ToLower(enhanced.Priority),
		}
		if _, err := st.UpdateTask(target.ID, updates); err != nil {
			return fmt.Errorf("apply update: %w", err)
		}
		fmt.Println("Applied.")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(iterateCmd)
	iterateCmd.Flags().StringVar(&iterTaskID, "task", "", "Parent task id")
	iterateCmd.Flags().StringVar(&iterStep, "step", "", "Step index (1-based) or subtask id")
	iterateCmd.Flags().StringVar(&iterPrompt, "prompt", "", "Refinement/splitting guidance")
	iterateCmd.Flags().BoolVar(&iterSplit, "split", false, "Split the step into 2-3 steps")
	iterateCmd.Flags().BoolVar(&iterConfirm, "confirm", false, "Apply changes (default: preview only)")
}
