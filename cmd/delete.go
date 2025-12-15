/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

import (
	"fmt"

	"github.com/josephgoksu/TaskWing/internal/taskutil"
	"github.com/josephgoksu/TaskWing/models"
	"github.com/josephgoksu/TaskWing/store"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

var recursive bool

// deleteCmd represents the delete command
var deleteCmd = &cobra.Command{
	Use:     "delete [task_id]",
	Aliases: []string{"rm", "remove"},
	Short:   "Delete a task",
	Long:    `Delete a task by its ID. If no ID is provided, an interactive list is shown. A confirmation prompt is always displayed before deletion.`,
	Args:    cobra.MaximumNArgs(1), // Allow 0 or 1 argument
	Run: func(cmd *cobra.Command, args []string) {
		recursive, _ = cmd.Flags().GetBool("recursive")

		taskStore, err := GetStore()
		if err != nil {
			HandleFatalError("Error getting task store", err)
		}
		defer func() {
			if err := taskStore.Close(); err != nil {
				HandleFatalError("Failed to close task store", err)
			}
		}()

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
				HandleFatalError("Error: Could not select a task.", err)
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
	// Resolve the task ID (handle partial IDs)
	resolvedTask, err := resolveTaskReference(taskStore, taskID)
	if err != nil {
		HandleFatalError(fmt.Sprintf("Error: Could not find task with reference '%s'.", taskID), err)
	}

	task := *resolvedTask

	// Inspect relationships to provide a safer, more helpful flow
	// 1) Subtasks: if present, confirm recursive delete of descendants
	// 2) Dependents: ensure references are unlinked by using batch DeleteTasks

	// Fetch descendants (includes root)
	descendants, derr := taskStore.GetTaskWithDescendants(task.ID)
	if derr != nil {
		HandleFatalError("Error: Could not inspect subtasks for deletion.", derr)
	}
	hasSubtasks := len(descendants) > 1

	// Build confirmation message
	confirmLabel := fmt.Sprintf("Are you sure you want to delete task '%s' (ID: %s)?", task.Title, task.ID)
	if hasSubtasks {
		confirmLabel = fmt.Sprintf("This will delete '%s' and %d subtask(s). Proceed?", task.Title, len(descendants)-1)
	}

	confirmPrompt := promptui.Prompt{Label: confirmLabel, IsConfirm: true}
	if _, err = confirmPrompt.Run(); err != nil {
		if err == promptui.ErrAbort {
			fmt.Println("Deletion cancelled.")
			return
		}
		HandleFatalError("Error: Could not get confirmation for deletion.", err)
	}

	// Determine IDs to delete
	idsToDelete := []string{task.ID}
	if hasSubtasks {
		idsToDelete = make([]string, 0, len(descendants))
		for _, t := range descendants {
			idsToDelete = append(idsToDelete, t.ID)
		}
	}

	// Use batch delete which also cleans up dependency links in kept tasks
	deletedCount, err := taskStore.DeleteTasks(idsToDelete)
	if err != nil {
		HandleFatalError(fmt.Sprintf("Error: Failed to delete task '%s'.", task.Title), err)
	}

	if hasSubtasks {
		fmt.Printf("Deleted '%s' and %d subtask(s).\n", task.Title, len(descendants)-1)
	}
	if deletedCount == 0 {
		fmt.Printf("No tasks were deleted.\n")
	} else if !hasSubtasks {
		fmt.Printf("Task '%s' (ID: %s) deleted successfully.\n", task.Title, task.ID)
	}
}

func handleRecursiveDelete(taskStore store.TaskStore, rootTaskID string) {
	// Resolve the task ID first
	resolvedTask, err := resolveTaskReference(taskStore, rootTaskID)
	if err != nil {
		HandleFatalError(fmt.Sprintf("Error: Could not find task with reference '%s' to begin recursive delete.", rootTaskID), err)
	}

	// Use the resolved full UUID for the recursive deletion
	fullTaskID := resolvedTask.ID
	tasksToDelete, err := taskStore.GetTaskWithDescendants(fullTaskID)
	if err != nil {
		HandleFatalError(fmt.Sprintf("Error: Could not get descendant tasks for '%s'.", resolvedTask.Title), err)
	}

	if len(tasksToDelete) <= 1 {
		// If it's just one task, there are no descendants, so treat as a single delete.
		fmt.Println("No subtasks found. Proceeding with a single task delete.")
		handleSingleDelete(taskStore, fullTaskID)
		return
	}

	fmt.Printf("You are about to recursively delete the following %d tasks:\n", len(tasksToDelete))
	for _, t := range tasksToDelete {
		// Highlight the root of the deletion
		if t.ID == fullTaskID {
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
		HandleFatalError("Error: Could not get confirmation for recursive deletion.", err)
	}

	idsToDelete := make([]string, len(tasksToDelete))
	for i, t := range tasksToDelete {
		idsToDelete[i] = t.ID
	}

	deletedCount, err := taskStore.DeleteTasks(idsToDelete)
	if err != nil {
		HandleFatalError("Error: Failed to perform the recursive delete operation.", err)
	}

	fmt.Printf("Successfully deleted %d tasks.\n", deletedCount)
}

// resolveTaskReference resolves a partial task ID or reference to a full task
func resolveTaskReference(taskStore store.TaskStore, reference string) (*models.Task, error) {
	// First try exact match
	if task, err := taskStore.GetTask(reference); err == nil {
		return &task, nil
	}

	// Get all tasks for fuzzy matching
	tasks, err := taskStore.ListTasks(nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list tasks: %w", err)
	}

	return taskutil.ResolveTaskReference(reference, tasks)
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
