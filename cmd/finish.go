package cmd

import (
	"fmt"
	"time"

	"github.com/josephgoksu/TaskWing/models"
	"github.com/spf13/cobra"
)

var finishCmd = &cobra.Command{
	Use:   "finish [task_id]",
	Short: "Finish a task (moves to 'done' status)",
	Long:  `Complete a task by moving it to 'done' status. Can be used on tasks in any status. If no task ID is provided, you'll be prompted to select from non-done tasks.`,
	Example: `  taskwing finish abc123  # Finish specific task
  taskwing finish         # Interactive selection`,
	RunE: func(cmd *cobra.Command, args []string) error {
		taskStore, err := GetStore()
		if err != nil {
			return fmt.Errorf("failed to get task store: %w", err)
		}

		var taskID string

		if len(args) > 0 {
			taskID = args[0]
		} else {
			// Interactive selection from non-done tasks
			task, err := selectTaskInteractive(taskStore, func(t models.Task) bool {
				return t.Status != models.StatusDone
			}, "Select a task to finish")
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

		// Check if already done
		if task.Status == models.StatusDone {
			fmt.Printf("Task '%s' is already done\n", task.Title)
			return nil
		}

		// Update status to done and set completion time
		now := time.Now()
		updates := map[string]interface{}{
			"status":      string(models.StatusDone),
			"completedAt": &now,
		}

		updatedTask, err := taskStore.UpdateTask(taskID, updates)
		if err != nil {
			return fmt.Errorf("failed to finish task: %w", err)
		}

		// Clear current task if this was the current one
		currentTaskID := GetCurrentTask()
		if currentTaskID == taskID {
			if err := ClearCurrentTask(); err != nil {
				fmt.Printf("Warning: failed to clear current task: %v\n", err)
			}
		}

		fmt.Printf("🎉 Finished task: %s\n", updatedTask.Title)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(finishCmd)
}
