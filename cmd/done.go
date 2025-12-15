package cmd

import (
	"fmt"

	"github.com/josephgoksu/TaskWing/models"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

// doneCmd represents the done command
var doneCmd = &cobra.Command{
	Use:     "done [task_id]",
	Aliases: []string{"finish", "complete", "d"},
	Short:   "Mark a task as done",
	Long:    `Mark a task as completed. If task_id is provided, it attempts to mark that task directly. Otherwise, it presents an interactive list to choose a task.`,
	Example: `  # Interactive mode
  taskwing done

  # Complete specific task
  taskwing done abc123

  # Using alias
  taskwing d abc123`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		taskStore, err := GetStore()
		if err != nil {
			HandleFatalError("Error: Could not initialize the task store.", err)
		}
		defer func() {
			if err := taskStore.Close(); err != nil {
				HandleFatalError("Failed to close task store", err)
			}
		}()

		var taskToMarkDone models.Task

		if len(args) > 0 {
			taskID := args[0]
			taskPtr, err := resolveTaskReference(taskStore, taskID)
			if err != nil {
				HandleFatalError(fmt.Sprintf("Error: Could not find task with ID '%s'.", taskID), err)
			}
			taskToMarkDone = *taskPtr
		} else {
			// Filter for tasks that are not yet completed
			notDoneFilter := func(t models.Task) bool {
				return t.Status != models.StatusDone
			}
			taskToMarkDone, err = selectTaskInteractive(taskStore, notDoneFilter, "Select task to mark as done")
			if err != nil {
				if err == promptui.ErrInterrupt {
					fmt.Println("Operation cancelled.")
					return
				}
				if err == ErrNoTasksFound {
					fmt.Println("No active tasks available to mark as done.")
					return
				}
				HandleFatalError("Error: Could not select a task.", err)
			}
		}

		if taskToMarkDone.Status == models.StatusDone {
			fmt.Printf("Task '%s' (ID: %s) is already completed.\n", taskToMarkDone.Title, taskToMarkDone.ID)
			return
		}

		updatedTask, err := taskStore.MarkTaskDone(taskToMarkDone.ID)
		if err != nil {
			HandleFatalError(fmt.Sprintf("Error: Failed to mark task '%s' as done.", taskToMarkDone.Title), err)
		}

		// Clear current task if this was the current one
		currentTaskID := GetCurrentTask()
		if currentTaskID == taskToMarkDone.ID {
			if err := ClearCurrentTask(); err != nil {
				fmt.Printf("Warning: failed to clear current task: %v\n", err)
			}
		}

		fmt.Printf("🎉 Task '%s' (ID: %s) marked as done successfully!\n", updatedTask.Title, updatedTask.ID)

		// Command discovery hints
		fmt.Printf("\n💡 What's next?\n")
		fmt.Printf("   • Add new task:   taskwing add \"Your next task\"\n")
		fmt.Printf("   • Find next task: taskwing next\n")
		fmt.Printf("   • View all tasks: taskwing list\n")
	},
}

func init() {
	rootCmd.AddCommand(doneCmd)
}
