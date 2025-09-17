/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// currentCmd represents the current command
var currentCmd = &cobra.Command{
	Use:   "current",
	Short: "Manage the current active task",
	Long: `Manage the current active task that you're working on. This helps AI tools understand what you're currently focused on.

Examples:
  taskwing current set <task_id>    # Set current task
  taskwing current show             # Show current task
  taskwing current clear            # Clear current task`,
}

var currentSetCmd = &cobra.Command{
	Use:   "set <task_id>",
	Short: "Set the current active task",
	Long:  "Set the current active task that you're working on. This task will be included in context for AI tools.",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		taskID := args[0]

		// Initialize task store
		taskStore, err := GetStore()
		if err != nil {
			HandleFatalError("Error: Could not initialize the task store.", err)
		}
		defer func() {
			if err := taskStore.Close(); err != nil {
				HandleFatalError("Failed to close task store", err)
			}
		}()

		// Verify the task exists
		task, err := taskStore.GetTask(taskID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: Task '%s' not found: %v\n", taskID, err)
			os.Exit(1)
		}

		// Set the current task
		if err := SetCurrentTask(taskID); err != nil {
			fmt.Fprintf(os.Stderr, "Error setting current task: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("✓ Set current task: %s - %s\n", task.ID, task.Title)
	},
}

var currentShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show the current active task",
	Long:  "Display information about the current active task.",
	Run: func(cmd *cobra.Command, args []string) {
		currentTaskID := GetCurrentTask()

		if currentTaskID == "" {
			fmt.Println("No current task set.")
			fmt.Println("Use 'taskwing current set <task_id>' to set one.")
			return
		}

		// Initialize task store
		taskStore, err := GetStore()
		if err != nil {
			HandleFatalError("Error: Could not initialize the task store.", err)
		}
		defer func() {
			if err := taskStore.Close(); err != nil {
				HandleFatalError("Failed to close task store", err)
			}
		}()

		// Get the current task
		task, err := taskStore.GetTask(currentTaskID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Current task '%s' not found: %v\n", currentTaskID, err)
			fmt.Println("The current task may have been deleted. Use 'taskwing current clear' to reset.")
			return
		}

		// Display task information
		fmt.Printf("📌 Current Task: %s\n", task.ID)
		fmt.Printf("   Title: %s\n", task.Title)
		fmt.Printf("   Status: %s\n", task.Status)
		fmt.Printf("   Priority: %s\n", task.Priority)
		if task.Description != "" {
			fmt.Printf("   Description: %s\n", task.Description)
		}
		fmt.Printf("   Created: %s\n", task.CreatedAt.Format("2006-01-02 15:04"))
		if !task.UpdatedAt.IsZero() {
			fmt.Printf("   Updated: %s\n", task.UpdatedAt.Format("2006-01-02 15:04"))
		}
	},
}

var currentClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Clear the current active task",
	Long:  "Remove the current active task setting.",
	Run: func(cmd *cobra.Command, args []string) {
		currentTaskID := GetCurrentTask()

		if currentTaskID == "" {
			fmt.Println("No current task is set.")
			return
		}

		if err := ClearCurrentTask(); err != nil {
			fmt.Fprintf(os.Stderr, "Error clearing current task: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("✓ Cleared current task: %s\n", currentTaskID)
	},
}

func init() {
	rootCmd.AddCommand(currentCmd)
	currentCmd.AddCommand(currentSetCmd)
	currentCmd.AddCommand(currentShowCmd)
	currentCmd.AddCommand(currentClearCmd)
}
