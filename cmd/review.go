package cmd

import (
	"fmt"

	"github.com/josephgoksu/TaskWing/models"
	"github.com/spf13/cobra"
)

var reviewCmd = &cobra.Command{
	Use:   "review [task_id]",
	Short: "Move task to review status (ready for validation)",
	Long:  `Move a task to 'review' status when work is complete and it needs validation. If no task ID is provided, you'll be prompted to select from 'doing' tasks.`,
	Example: `  taskwing review abc123  # Move specific task to review
  taskwing review         # Interactive selection from doing tasks`,
	RunE: func(cmd *cobra.Command, args []string) error {
		taskStore, err := GetStore()
		if err != nil {
			return fmt.Errorf("failed to get task store: %w", err)
		}

		var taskID string

		if len(args) > 0 {
			taskID = args[0]
		} else {
			// Interactive selection from doing tasks
			task, err := selectTaskInteractive(taskStore, func(t models.Task) bool {
				return t.Status == models.StatusDoing
			}, "Select a task to move to review")
			if err != nil {
				return err
			}
			taskID = task.ID
		}

		// Get the task
		task, err := taskStore.GetTask(taskID)
		if err != nil {
			return fmt.Errorf("failed to get task: %w", err)
		}

		// Validate transition
		if !models.ValidateStatusTransition(task.Status, models.StatusReview) {
			return fmt.Errorf("cannot move task with status '%s' to review - only 'doing' tasks can be moved to review", task.Status)
		}

		// Update status to review
		updates := map[string]interface{}{
			"status": string(models.StatusReview),
		}

		updatedTask, err := taskStore.UpdateTask(taskID, updates)
		if err != nil {
			return fmt.Errorf("failed to move task to review: %w", err)
		}

		fmt.Printf("🔍 Moved task to review: %s\n", updatedTask.Title)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(reviewCmd)
}
