package cmd

import (
	"fmt"

	"github.com/josephgoksu/taskwing.app/models"
	"github.com/spf13/cobra"
)

var startCmd = &cobra.Command{
	Use:     "start [task_id]",
	Short:   "Start working on a task (moves to 'doing' status)",
	Long:    `Start working on a task by moving it from 'todo' to 'doing' status. If no task ID is provided, you'll be prompted to select from available tasks.`,
	Example: `  taskwing start abc123  # Start specific task
  taskwing start         # Interactive selection`,
	RunE: func(cmd *cobra.Command, args []string) error {
		taskStore, err := GetStore()
		if err != nil {
			return fmt.Errorf("failed to get task store: %w", err)
		}

		var taskID string

		if len(args) > 0 {
			taskID = args[0]
		} else {
			// Interactive selection from todo tasks
			task, err := selectTaskInteractive(taskStore, func(t models.Task) bool {
				return t.Status == models.StatusTodo
			}, "Select a task to start")
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
		if !models.ValidateStatusTransition(task.Status, models.StatusDoing) {
			return fmt.Errorf("cannot start task with status '%s' - only 'todo' tasks can be started", task.Status)
		}

		// Update status to doing
		updates := map[string]interface{}{
			"status": string(models.StatusDoing),
		}

		updatedTask, err := taskStore.UpdateTask(taskID, updates)
		if err != nil {
			return fmt.Errorf("failed to start task: %w", err)
		}

		// Set as current task
		if err := SetCurrentTask(taskID); err != nil {
			fmt.Printf("Warning: failed to set as current task: %v\n", err)
		}

		fmt.Printf("✅ Started task: %s\n", updatedTask.Title)
		fmt.Printf("📌 Set as current task\n")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(startCmd)
}