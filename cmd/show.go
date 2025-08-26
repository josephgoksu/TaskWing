/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/josephgoksu/TaskWing/models"
	"github.com/josephgoksu/TaskWing/store"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

// showCmd represents the show command
var showCmd = &cobra.Command{
	Use:   "show [task_id]",
	Short: "Show details for a specific task",
	Long: `Displays detailed information about a single task, including its description,
status, priority, dependencies, parent, and subtasks.

If a task_id is provided, it will show details for that specific task.
Otherwise, it will present an interactive menu to select a task.`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		taskStore, err := GetStore()
		if err != nil {
			HandleError("Error: could not get the task store", err)
		}
		defer func() {
			if err := taskStore.Close(); err != nil {
				HandleError("Failed to close task store", err)
			}
		}()

		var taskToShow models.Task
		var selectedTaskID string

		if len(args) > 0 {
			selectedTaskID = args[0]
		} else {
			// No ID provided, use interactive selector
			allTasks, err := taskStore.ListTasks(nil, nil)
			if err != nil {
				HandleError("Error: Could not list tasks for selection.", err)
			}
			if len(allTasks) == 0 {
				fmt.Println("No tasks found.")
				return
			}

			// Using the existing selectTaskInteractive function from root.go
			selectedTask, err := selectTaskInteractive(taskStore, nil, "Select a task to view its details")
			if err != nil {
				if err == promptui.ErrInterrupt {
					fmt.Println("Operation cancelled.")
					return
				}
				HandleError("Error: Could not select a task.", err)
			}
			selectedTaskID = selectedTask.ID
		}

		taskToShow, err = taskStore.GetTask(selectedTaskID)
		if err != nil {
			HandleError(fmt.Sprintf("Error: Could not retrieve task with ID '%s'.", selectedTaskID), err)
		}

		displayTaskDetails(taskToShow, taskStore)
	},
}

// displayTaskDetails prints a formatted view of a single task's details.
func displayTaskDetails(task models.Task, store store.TaskStore) {
	fmt.Println(strings.Repeat("-", 40))
	fmt.Printf("Task Details: %s\n", task.Title)
	fmt.Println(strings.Repeat("-", 40))

	fmt.Printf("  %-20s %s\n", "ID:", task.ID)
	fmt.Printf("  %-20s %s\n", "Status:", task.Status)
	fmt.Printf("  %-20s %s\n", "Priority:", task.Priority)
	fmt.Printf("  %-20s %s\n", "Created At:", task.CreatedAt.Format(time.RFC1123))
	fmt.Printf("  %-20s %s\n", "Last Updated:", task.UpdatedAt.Format(time.RFC1123))
	if task.CompletedAt != nil {
		fmt.Printf("  %-20s %s\n", "Completed At:", task.CompletedAt.Format(time.RFC1123))
	}

	fmt.Println(strings.Repeat("-", 40))

	// Description
	if task.Description != "" {
		fmt.Println("Description:")
		fmt.Printf("  %s\n", task.Description)
		fmt.Println(strings.Repeat("-", 40))
	}

	// Acceptance Criteria
	if task.AcceptanceCriteria != "" {
		fmt.Println("Acceptance Criteria:")
		// Split by newline and indent each line for readability
		for _, line := range strings.Split(task.AcceptanceCriteria, "\n") {
			fmt.Printf("  %s\n", line)
		}
		fmt.Println(strings.Repeat("-", 40))
	}

	// Relationships
	// Parent Task
	if task.ParentID != nil && *task.ParentID != "" {
		parentTask, err := store.GetTask(*task.ParentID)
		if err == nil {
			fmt.Printf("Parent Task: %s (ID: %s)\n", parentTask.Title, *task.ParentID)
		} else {
			fmt.Printf("Parent Task ID: %s (Could not fetch full details)\n", *task.ParentID)
		}
	} else {
		fmt.Println("Parent Task: None")
	}

	// Subtasks
	if len(task.SubtaskIDs) > 0 {
		fmt.Println("Subtasks:")
		for _, subID := range task.SubtaskIDs {
			subTask, err := store.GetTask(subID)
			if err == nil {
				fmt.Printf("  - %s (ID: %s, Status: %s)\n", subTask.Title, subID, subTask.Status)
			} else {
				fmt.Printf("  - (ID: %s - Could not fetch details)\n", subID)
			}
		}
	} else {
		fmt.Println("Subtasks: None")
	}

	// Dependencies with status icons
	if len(task.Dependencies) > 0 {
		fmt.Println("Depends On (Dependencies):")
		for _, depID := range task.Dependencies {
			depTask, err := store.GetTask(depID)
			if err == nil {
				icon := "⏱️"
				if depTask.Status == models.StatusDone {
					icon = "✅"
				}
				fmt.Printf("  %s %s (ID: %s, Status: %s)\n", icon, depTask.Title, depID, depTask.Status)
			} else {
				fmt.Printf("  - (ID: %s - Could not fetch details)\n", depID)
			}
		}
	} else {
		fmt.Println("Depends On: None")
	}

	// Dependents
	if len(task.Dependents) > 0 {
		fmt.Println("Is a Dependency For (Dependents):")
		for _, depID := range task.Dependents {
			depTask, err := store.GetTask(depID)
			if err == nil {
				fmt.Printf("  - %s (ID: %s, Status: %s)\n", depTask.Title, depID, depTask.Status)
			} else {
				fmt.Printf("  - (ID: %s - Could not fetch details)\n", depID)
			}
		}
	} else {
		fmt.Println("Is a Dependency For: None")
	}

	fmt.Println(strings.Repeat("-", 40))
}

func init() {
	rootCmd.AddCommand(showCmd)
}
