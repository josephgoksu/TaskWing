/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/josephgoksu/TaskWing/models"
	"github.com/josephgoksu/TaskWing/store"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

var (
	recursive bool
)

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
			HandleError("Error getting task store", err)
		}
		defer func() {
			if err := taskStore.Close(); err != nil {
				HandleError("Failed to close task store", err)
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
	// Resolve the task ID (handle partial IDs)
	resolvedTask, err := resolveTaskReference(taskStore, taskID)
	if err != nil {
		HandleError(fmt.Sprintf("Error: Could not find task with reference '%s'.", taskID), err)
	}

	task := *resolvedTask

	confirmPrompt := promptui.Prompt{
		Label:     fmt.Sprintf("Are you sure you want to delete task '%s' (ID: %s)?", task.Title, task.ID),
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

	err = taskStore.DeleteTask(task.ID)
	if err != nil {
		HandleError(fmt.Sprintf("Error: Failed to delete task '%s'.", task.Title), err)
	}

	fmt.Printf("Task '%s' (ID: %s) deleted successfully.\n", task.Title, task.ID)
}

func handleRecursiveDelete(taskStore store.TaskStore, rootTaskID string) {
	// Resolve the task ID first
	resolvedTask, err := resolveTaskReference(taskStore, rootTaskID)
	if err != nil {
		HandleError(fmt.Sprintf("Error: Could not find task with reference '%s' to begin recursive delete.", rootTaskID), err)
	}

	// Use the resolved full UUID for the recursive deletion
	fullTaskID := resolvedTask.ID
	tasksToDelete, err := taskStore.GetTaskWithDescendants(fullTaskID)
	if err != nil {
		HandleError(fmt.Sprintf("Error: Could not get descendant tasks for '%s'.", resolvedTask.Title), err)
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

	// Try partial ID match (minimum 8 characters for meaningful UUID portion)
	if len(reference) >= 8 {
		for _, task := range tasks {
			if strings.HasPrefix(strings.ToLower(task.ID), strings.ToLower(reference)) {
				return &task, nil
			}
		}
	}

	// Try fuzzy title matching
	type match struct {
		task  models.Task
		score float64
	}

	var matches []match
	refLower := strings.ToLower(reference)

	for _, task := range tasks {
		titleLower := strings.ToLower(task.Title)

		// Exact title match
		if titleLower == refLower {
			return &task, nil
		}

		// Substring match in title
		if strings.Contains(titleLower, refLower) {
			score := 0.9 - (float64(len(titleLower)-len(refLower)) / float64(len(titleLower)) * 0.3)
			matches = append(matches, match{task: task, score: score})
		}
	}

	// Sort matches by score and return best match if confidence is high enough
	if len(matches) > 0 {
		sort.SliceStable(matches, func(i, j int) bool {
			return matches[i].score > matches[j].score
		})

		// If we have a high confidence match (>80%) and it's the only good match
		if matches[0].score > 0.8 && (len(matches) == 1 || matches[0].score > matches[1].score+0.2) {
			return &matches[0].task, nil
		}

		// If we have multiple similar matches, show suggestions
		if len(matches) > 1 {
			var suggestions []string
			for i, m := range matches {
				if i >= 3 { // Limit to top 3 suggestions
					break
				}
				suggestions = append(suggestions, fmt.Sprintf("  %s - %s",
					m.task.ID[:8], m.task.Title))
			}

			return nil, fmt.Errorf("multiple matches found for '%s'. Did you mean:\n%s\n\nUse a more specific reference or full task ID",
				reference, strings.Join(suggestions, "\n"))
		}
	}

	return nil, fmt.Errorf("no task found matching '%s'. Use 'taskwing list' to see available tasks", reference)
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
