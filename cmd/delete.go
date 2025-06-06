/*
Copyright © 2025 NAME HERE josephgoksu@gmail.com
*/
package cmd

import (
	"fmt"
	"os"

	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

// deleteCmd represents the delete command
var deleteCmd = &cobra.Command{
	Use:   "delete [task_id]",
	Short: "Delete a task",
	Long:  `Delete a task by its ID. If no ID is provided, an interactive list is shown. A confirmation prompt is always displayed before deletion.`,
	Args:  cobra.MaximumNArgs(1), // Allow 0 or 1 argument
	Run: func(cmd *cobra.Command, args []string) {
		taskStore, err := getStore() // Assumes getStore() is defined (e.g., in add.go or a shared util)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to get store: %v\\n", err)
			os.Exit(1)
		}
		defer taskStore.Close()

		var taskIDToDelete string
		var taskTitle string // For confirmation message

		if len(args) > 0 {
			taskIDToDelete = args[0]
			// Validate if task exists and get title
			task, err := taskStore.GetTask(taskIDToDelete)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to retrieve task %s for deletion: %v\\n", taskIDToDelete, err)
				os.Exit(1)
			}
			taskTitle = task.Title
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
				fmt.Fprintf(os.Stderr, "Task selection failed: %v\n", err)
				os.Exit(1)
			}
			taskIDToDelete = selectedTask.ID
			taskTitle = selectedTask.Title
		}

		// Confirmation prompt
		confirmPrompt := promptui.Prompt{
			Label:     fmt.Sprintf("Are you sure you want to delete task '%s' (ID: %s)?", taskTitle, taskIDToDelete),
			IsConfirm: true,
		}
		_, err = confirmPrompt.Run()
		if err != nil {
			// Handles both 'no' (promptui.ErrAbort) and actual errors
			if err == promptui.ErrAbort {
				fmt.Println("Deletion cancelled.")
			} else {
				fmt.Fprintf(os.Stderr, "Confirmation prompt failed: %v\\n", err)
			}
			os.Exit(1)
		}

		err = taskStore.DeleteTask(taskIDToDelete)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to delete task %s: %v\\n", taskIDToDelete, err)
			os.Exit(1)
		}

		fmt.Printf("Task ID %s deleted successfully.\\n", taskIDToDelete)
	},
}

func init() {
	rootCmd.AddCommand(deleteCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// deleteCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// deleteCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
