/*
Copyright © 2025 NAME HERE josephgoksu@gmail.com
*/
package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/josephgoksu/taskwing.app/models"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

// addCmd represents the add command
var addCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a new task",
	Long:  `Add a new task to the task manager. Prompts for title, description, priority, tags, dependencies, and optional parent task ID.`,
	Run: func(cmd *cobra.Command, args []string) {
		taskStore, err := getStore()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to get store: %v\n", err)
			os.Exit(1)
		}
		defer taskStore.Close()

		titlePrompt := promptui.Prompt{
			Label: "Task Title",
			Validate: func(input string) error {
				if len(input) < 3 {
					return fmt.Errorf("title must be at least 3 characters long")
				}
				return nil
			},
		}
		title, err := titlePrompt.Run()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Title prompt failed %v\n", err)
			os.Exit(1)
		}

		descriptionPrompt := promptui.Prompt{
			Label: "Task Description (optional)",
		}
		description, err := descriptionPrompt.Run()
		if err != nil && err != promptui.ErrInterrupt {
			fmt.Fprintf(os.Stderr, "Description prompt failed %v\n", err)
			os.Exit(1)
		}

		prioritySelect := promptui.Select{
			Label: "Task Priority",
			Items: []models.TaskPriority{models.PriorityLow, models.PriorityMedium, models.PriorityHigh, models.PriorityUrgent},
		}
		_, priorityStr, err := prioritySelect.Run()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Priority selection failed %v\n", err)
			os.Exit(1)
		}

		// Prompt for Dependencies
		depsPrompt := promptui.Prompt{
			Label: "Task Dependencies (optional, comma-separated IDs)",
		}
		depsStr, err := depsPrompt.Run()
		var dependencyIDs []string
		if err != nil && err != promptui.ErrInterrupt {
			fmt.Fprintf(os.Stderr, "Dependencies prompt failed %v\n", err)
			os.Exit(1)
		} else if err == nil && strings.TrimSpace(depsStr) != "" {
			dependencyIDs = strings.Split(strings.ReplaceAll(depsStr, " ", ""), ",")
		}

		// Prompt for Parent Task ID
		var parentIDPtr *string
		parentIDPrompt := promptui.Prompt{
			Label: "Parent Task ID (optional, leave empty if none)",
			Validate: func(input string) error {
				if strings.TrimSpace(input) == "" {
					return nil // Allow empty for no parent
				}
				// Basic format check could be done here, but existence is checked by store
				// For example, ensure it's not outrageously long or invalid chars if needed.
				return nil
			},
		}
		parentIDStr, err := parentIDPrompt.Run()
		if err != nil && err != promptui.ErrInterrupt {
			fmt.Fprintf(os.Stderr, "Parent ID prompt failed %v\n", err)
			os.Exit(1)
		} else if err == nil && strings.TrimSpace(parentIDStr) != "" {
			cleanParentIDStr := strings.TrimSpace(parentIDStr)
			parentIDPtr = &cleanParentIDStr
		}

		newTask := models.Task{
			Title:        title,
			Description:  description,
			Priority:     models.TaskPriority(priorityStr),
			Dependencies: dependencyIDs,
			ParentID:     parentIDPtr,
		}

		createdTask, err := taskStore.CreateTask(newTask)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create task: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Task '%s' added successfully! ID: %s\n", createdTask.Title, createdTask.ID)
		if createdTask.ParentID != nil && *createdTask.ParentID != "" {
			fmt.Printf("Parent Task ID: %s\n", *createdTask.ParentID)
		}
		if len(createdTask.Dependencies) > 0 {
			fmt.Printf("Dependencies: %v\n", createdTask.Dependencies)
		}
	},
}

func init() {
	rootCmd.AddCommand(addCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// addCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// addCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

// getStore was moved to root.go or a central cmd utility file
/*
func getStore() (store.TaskStore, error) {
	// For now, using FileTaskStore with default config.
	// This should ideally come from a config loader (e.g., Viper in config.go)
	s := store.NewFileTaskStore()
	// Config can be expanded or loaded from viper
	err := s.Initialize(map[string]string{
		"dataFile":       "tasks.json",
		"dataFileFormat": "json",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize store: %w", err)
	}
	return s, nil
}
*/
