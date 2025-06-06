package cmd

import (
	"fmt"
	"os"

	"github.com/josephgoksu/taskwing.app/models"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

// doneCmd represents the done command
var doneCmd = &cobra.Command{
	Use:   "done [task_id]",
	Short: "Mark a task as done",
	Long:  `Mark a task as completed. If task_id is provided, it attempts to mark that task directly. Otherwise, it presents an interactive list to choose a task.`,
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		taskStore, err := getStore()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to get store: %v\\n", err)
			os.Exit(1)
		}
		defer taskStore.Close()

		var taskToMarkDone models.Task

		if len(args) > 0 {
			taskID := args[0]
			taskToMarkDone, err = taskStore.GetTask(taskID)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to get task %s: %v\\n", taskID, err)
				os.Exit(1)
			}
		} else {
			// Filter for tasks that are not yet completed
			notDoneFilter := func(t models.Task) bool {
				return t.Status != models.StatusCompleted
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
				fmt.Fprintf(os.Stderr, "Task selection failed: %v\n", err)
				os.Exit(1)
			}
		}

		if taskToMarkDone.Status == models.StatusCompleted {
			fmt.Printf("Task '%s' (ID: %s) is already completed.\\n", taskToMarkDone.Title, taskToMarkDone.ID)
			return
		}

		updatedTask, err := taskStore.MarkTaskDone(taskToMarkDone.ID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to mark task %s as done: %v\\n", taskToMarkDone.ID, err)
			os.Exit(1)
		}

		fmt.Printf("Task '%s' (ID: %s) marked as done successfully!\\n", updatedTask.Title, updatedTask.ID)
	},
}

func init() {
	rootCmd.AddCommand(doneCmd)
}
