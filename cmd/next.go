package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/josephgoksu/TaskWing/models"
	"github.com/spf13/cobra"
)

// nextCmd suggests the next best task to work on based on
// dependency readiness, priority, and simple tie-breakers.
var nextCmd = &cobra.Command{
	Use:   "next",
	Short: "Suggest the next ready task to start",
	Long: `Finds tasks that are ready to start (all dependencies completed)
and prioritizes them by priority, dependency count, and creation time.

Displays rich info and suggested follow-up commands.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		taskStore, err := GetStore()
		if err != nil {
			return fmt.Errorf("failed to get task store: %w", err)
		}
		defer func() {
			_ = taskStore.Close()
		}()

		// Load all tasks; we’ll filter locally
		allTasks, err := taskStore.ListTasks(nil, nil)
		if err != nil {
			return fmt.Errorf("failed to list tasks: %w", err)
		}
		if len(allTasks) == 0 {
			fmt.Println("No tasks found.")
			return nil
		}

		// Index by ID for quick lookups
		byID := map[string]models.Task{}
		for _, t := range allTasks {
			byID[t.ID] = t
		}

		// Ready = in todo or doing AND all dependencies done
		ready := make([]models.Task, 0)
		for _, t := range allTasks {
			if t.Status != models.StatusTodo && t.Status != models.StatusDoing {
				continue
			}
			depsDone := true
			for _, depID := range t.Dependencies {
				dep, ok := byID[depID]
				if !ok || dep.Status != models.StatusDone {
					depsDone = false
					break
				}
			}
			if depsDone {
				ready = append(ready, t)
			}
		}

		if len(ready) == 0 {
			fmt.Println("No ready tasks found (dependencies may be incomplete).")
			return nil
		}

		// Sort by priority (urgent > high > medium > low), then fewest deps, then createdAt asc
		prioRank := map[models.TaskPriority]int{
			models.PriorityUrgent: 0,
			models.PriorityHigh:   1,
			models.PriorityMedium: 2,
			models.PriorityLow:    3,
		}
		sort.SliceStable(ready, func(i, j int) bool {
			a, b := ready[i], ready[j]
			ra, rb := prioRank[a.Priority], prioRank[b.Priority]
			if ra != rb {
				return ra < rb
			}
			if len(a.Dependencies) != len(b.Dependencies) {
				return len(a.Dependencies) < len(b.Dependencies)
			}
			if !a.CreatedAt.Equal(b.CreatedAt) {
				return a.CreatedAt.Before(b.CreatedAt)
			}
			return strings.Compare(a.ID, b.ID) < 0
		})

		// Pick top candidate and display details + suggestions
		best := ready[0]

		fmt.Println("— Next Suggested Task —")
		fmt.Printf("ID:        %s\n", best.ID)
		fmt.Printf("Title:     %s\n", best.Title)
		fmt.Printf("Status:    %s\n", best.Status)
		fmt.Printf("Priority:  %s\n", best.Priority)
		if best.Description != "" {
			fmt.Printf("Desc:      %s\n", best.Description)
		}
		if best.AcceptanceCriteria != "" {
			fmt.Println("Acceptance:")
			for _, line := range strings.Split(best.AcceptanceCriteria, "\n") {
				fmt.Printf("  %s\n", line)
			}
		}

		// Dependencies with status indicator
		if len(best.Dependencies) > 0 {
			fmt.Println("Dependencies:")
			for _, depID := range best.Dependencies {
				dep := byID[depID]
				mark := "⏱️"
				if dep.Status == models.StatusDone {
					mark = "✅"
				}
				fmt.Printf("  %s %s (%s)\n", mark, dep.Title, dep.ID)
			}
		} else {
			fmt.Println("Dependencies: none")
		}

		// Subtasks list
		if len(best.SubtaskIDs) > 0 {
			fmt.Println("Subtasks:")
			for _, sid := range best.SubtaskIDs {
				if sub, ok := byID[sid]; ok {
					fmt.Printf("  - %s (%s, %s)\n", sub.Title, sub.Status, sid)
				}
			}
		}

		fmt.Println()
		fmt.Println("Suggested actions:")
		fmt.Printf("  • Start:    taskwing start %s\n", best.ID)
		fmt.Printf("  • Show:     taskwing show %s\n", best.ID)
		fmt.Printf("  • Subtasks: taskwing list --parent %s\n", best.ID)
		fmt.Printf("  • Finish:   taskwing finish %s\n", best.ID)

		return nil
	},
}

func init() {
	rootCmd.AddCommand(nextCmd)
}
