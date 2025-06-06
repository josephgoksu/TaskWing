/*
Copyright © 2025 NAME HERE josephgoksu@gmail.com
*/
package cmd

import (
	"fmt"

	"github.com/josephgoksu/taskwing.app/store"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

var (
	recursive bool
)

// deleteCmd represents the delete command
var deleteCmd = &cobra.Command{
	Use:   "delete [task_id]",
	Short: "Delete a task",
	Long:  `Delete a task by its ID. If no ID is provided, an interactive list is shown. A confirmation prompt is always displayed before deletion.`,
	Args:  cobra.MaximumNArgs(1), // Allow 0 or 1 argument
	Run: func(cmd *cobra.Command, args []string) {
		recursive, _ = cmd.Flags().GetBool("recursive")

		taskStore, err := getStore()
		if err != nil {
			HandleError("Error: Could not initialize the task store.", err)
		}
		defer taskStore.Close()

		var taskIDToDelete string

		if len(args) > 0 {
			taskIDToDelete = args[0]
			// Minimal validation, store will confirm existence.
		} else {
			selectedTask, err := selectTaskInteractive(taskStore, nil, "Select task to delete")
			if err != nil {
				if err == promptui.ErrInterrupt {
					fmt.Println("Deletion cancelled.")
					return
				}
				if err == ErrNoTasksFound {
					fmt.Println("No tasks available to delete.")
					return
				}
				HandleError("Error: Could not select a task.", err)
			}
			taskIDToDelete = selectedTask.ID
		}

		if recursive {
			handleRecursiveDelete(taskStore, taskIDToDelete)
		} else {
			handleSingleDelete(taskStore, taskIDToDelete)
		}
	},
}

func handleSingleDelete(taskStore store.TaskStore, taskID string) {
	// Get task details for a better confirmation message.
	task, err := taskStore.GetTask(taskID)
	if err != nil {
		HandleError(fmt.Sprintf("Error: Could not find task with ID '%s'.", taskID), err)
	}

	confirmPrompt := promptui.Prompt{
		Label:     fmt.Sprintf("Are you sure you want to delete task '%s' (ID: %s)?", task.Title, taskID),
		IsConfirm: true,
	}
	_, err = confirmPrompt.Run()
	if err != nil {
		if err == promptui.ErrAbort {
			fmt.Println("Deletion cancelled.")
			return
		}
		HandleError("Error: Could not get confirmation for deletion.", err)
	}

	err = taskStore.DeleteTask(taskID)
	if err != nil {
		HandleError(fmt.Sprintf("Error: Failed to delete task '%s'.", task.Title), err)
	}

	fmt.Printf("Task '%s' (ID: %s) deleted successfully.\n", task.Title, taskID)
}

func handleRecursiveDelete(taskStore store.TaskStore, rootTaskID string) {
	tasksToDelete, err := taskStore.GetTaskWithDescendants(rootTaskID)
	if err != nil {
		HandleError(fmt.Sprintf("Error: Could not find task with ID '%s' to begin recursive delete.", rootTaskID), err)
	}

	if len(tasksToDelete) <= 1 {
		// If it's just one task, there are no descendants, so treat as a single delete.
		fmt.Println("No subtasks found. Proceeding with a single task delete.")
		handleSingleDelete(taskStore, rootTaskID)
		return
	}

	fmt.Printf("You are about to recursively delete the following %d tasks:\n", len(tasksToDelete))
	for _, t := range tasksToDelete {
		// Highlight the root of the deletion
		if t.ID == rootTaskID {
			fmt.Printf("- %s (ID: %s) [ROOT]\n", t.Title, t.ID)
		} else {
			fmt.Printf("- %s (ID: %s)\n", t.Title, t.ID)
		}
	}

	confirmPrompt := promptui.Prompt{
		Label:     "This action is irreversible. Are you sure you want to continue?",
		IsConfirm: true,
	}
	_, err = confirmPrompt.Run()
	if err != nil {
		if err == promptui.ErrAbort {
			fmt.Println("Recursive deletion cancelled.")
			return
		}
		HandleError("Error: Could not get confirmation for recursive deletion.", err)
	}

	idsToDelete := make([]string, len(tasksToDelete))
	for i, t := range tasksToDelete {
		idsToDelete[i] = t.ID
	}

	deletedCount, err := taskStore.DeleteTasks(idsToDelete)
	if err != nil {
		HandleError("Error: Failed to perform the recursive delete operation.", err)
	}

	fmt.Printf("Successfully deleted %d tasks.\n", deletedCount)
}

func init() {
	rootCmd.AddCommand(deleteCmd)
	deleteCmd.Flags().BoolP("recursive", "r", false, "Recursively delete the task and all its subtasks")

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// deleteCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// deleteCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
