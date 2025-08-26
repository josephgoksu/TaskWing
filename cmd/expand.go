package cmd

import (
	"fmt"

	"github.com/josephgoksu/TaskWing/models"
	"github.com/josephgoksu/TaskWing/store"
	"github.com/spf13/cobra"
)

var expandCmd = &cobra.Command{
	Use:   "expand",
	Short: "Expand tasks into subtasks using recommendations or overrides",
	Long: `Expands a task (or all tasks) into subtasks.

Examples:
  taskwing expand --id=<task-id>
  taskwing expand --all
  taskwing expand --id=<task-id> --count=4 --dry-run`,
	RunE: func(cmd *cobra.Command, args []string) error {
		taskID, _ := cmd.Flags().GetString("id")
		allFlag, _ := cmd.Flags().GetBool("all")
		countOverride, _ := cmd.Flags().GetInt("count")
		dryRun, _ := cmd.Flags().GetBool("dry-run")

		if taskID == "" && !allFlag {
			return fmt.Errorf("provide --id or --all")
		}
		if taskID != "" && allFlag {
			return fmt.Errorf("use either --id or --all, not both")
		}

		st, err := GetStore()
		if err != nil {
			return fmt.Errorf("failed to init store: %w", err)
		}
		defer func() { _ = st.Close() }()

		// Build list of targets
		var targets []models.Task
		if taskID != "" {
			t, err := st.GetTask(taskID)
			if err != nil {
				return fmt.Errorf("task '%s' not found: %w", taskID, err)
			}
			targets = []models.Task{t}
		} else {
			all, err := st.ListTasks(nil, nil)
			if err != nil {
				return fmt.Errorf("failed to list tasks: %w", err)
			}
			targets = append(targets, all...)
		}

		// Execute expansion
		created := 0
		for _, parent := range targets {
			// Decide count
			n := countOverride
			if n == 0 {
				n = 3
			}

			// Dry-run preview
			if dryRun {
				fmt.Printf("Would create %d subtasks under %s - %s\n", n, parent.ID, parent.Title)
				continue
			}

			c, err := createUniformSubtasks(st, parent, n)
			if err != nil {
				return err
			}
			created += c
		}

		if !dryRun {
			fmt.Printf("Created %d subtasks in total.\n", created)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(expandCmd)
	expandCmd.Flags().String("id", "", "Task ID to expand")
	expandCmd.Flags().Bool("all", false, "Expand all tasks")
	expandCmd.Flags().Int("count", 0, "Override subtask count (default: 3)")
	expandCmd.Flags().Bool("dry-run", false, "Preview without creating subtasks")
}

func createUniformSubtasks(st store.TaskStore, parent models.Task, n int) (int, error) {
	count := 0
	for i := 1; i <= n; i++ {
		title := fmt.Sprintf("%s — subtask %d", parent.Title, i)
		desc := fmt.Sprintf("Subtask %d for '%s'. Reference parent: %s.", i, parent.Title, parent.ID)
		t := models.Task{
			ID:          "",
			Title:       title,
			Description: desc,
			Status:      models.StatusTodo,
			Priority:    parent.Priority,
			ParentID:    &parent.ID,
		}
		created, err := st.CreateTask(t)
		if err != nil {
			return count, fmt.Errorf("failed creating subtask %d under %s: %w", i, parent.ID, err)
		}
		// Link to parent handled by store
		_ = created
		count++
	}
	return count, nil
}
