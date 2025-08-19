/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
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
		taskStore, err := GetStore()
		if err != nil {
			HandleError("Error: Could not initialize the task store.", err)
		}
		defer taskStore.Close()

		// Check if running in non-interactive mode
		nonInteractive, _ := cmd.Flags().GetBool("non-interactive")

		// Get title from flag or prompt
		title, err := cmd.Flags().GetString("title")
		if err != nil {
			HandleError("Error getting title flag", err)
		}
		if title == "" {
			if nonInteractive {
				HandleError("Title is required in non-interactive mode. Use --title flag.", nil)
				return
			}
			// Interactive prompt for title
			titlePrompt := promptui.Prompt{
				Label: "Task Title",
				Validate: func(input string) error {
					if len(strings.TrimSpace(input)) < 3 {
						return fmt.Errorf("title must be at least 3 characters long")
					}
					return nil
				},
			}
			title, err = titlePrompt.Run()
			if err != nil {
				if err == promptui.ErrInterrupt {
					fmt.Println("Task addition cancelled.")
					os.Exit(0)
				}
				HandleError("Error: Failed to read task title.", err)
			}
		}

		// Get description from flag or prompt
		description, err := cmd.Flags().GetString("description")
		if err != nil {
			HandleError("Error getting description flag", err)
		}
		if description == "" && !nonInteractive {
			descriptionPrompt := promptui.Prompt{
				Label: "Task Description (optional)",
			}
			description, err = descriptionPrompt.Run()
			if err != nil && err != promptui.ErrInterrupt {
				HandleError("Error: Failed to read task description.", err)
			}
		}

		// ... (similar logic for priority, dependencies, parentID)

		// Get priority from flag or prompt
		priorityStr, err := cmd.Flags().GetString("priority")
		if err != nil {
			HandleError("Error getting priority flag", err)
		}
		if priorityStr == "" {
			if nonInteractive {
				priorityStr = "medium" // Default priority in non-interactive mode
			} else {
				priorityPrompt := promptui.Select{
					Label: "Select Priority",
					Items: []string{"low", "medium", "high", "urgent"},
				}
				_, priorityStr, err = priorityPrompt.Run()
				if err != nil && err != promptui.ErrInterrupt {
					HandleError("Error: Failed to select priority.", err)
				}
			}
		}

		// Get dependencies from flag or prompt
		dependenciesStr, err := cmd.Flags().GetString("dependencies")
		if err != nil {
			HandleError("Error getting dependencies flag", err)
		}
		if dependenciesStr == "" && !nonInteractive {
			dependenciesPrompt := promptui.Prompt{
				Label: "Dependencies (comma-separated task IDs, optional)",
			}
			dependenciesStr, err = dependenciesPrompt.Run()
			if err != nil && err != promptui.ErrInterrupt {
				HandleError("Error: Failed to read dependencies.", err)
			}
		}
		var dependencies []string
		if dependenciesStr != "" {
			dependencies = strings.Split(dependenciesStr, ",")
			for i, dep := range dependencies {
				dependencies[i] = strings.TrimSpace(dep)
			}
		}

		// Get Parent ID from flag or prompt
		parentIDStr, err := cmd.Flags().GetString("parentID")
		if err != nil {
			HandleError("Error getting parentID flag", err)
		}
		if parentIDStr == "" && !nonInteractive {
			parentSelectPrompt := promptui.Prompt{
				Label: "Parent Task ID (optional, press Enter to skip)",
			}
			parentIDStr, err = parentSelectPrompt.Run()
			if err != nil && err != promptui.ErrInterrupt {
				HandleError("Error: Failed to read parent task ID.", err)
			}
		}
		var parentID *string
		if parentIDStr != "" {
			parentID = &parentIDStr
		}

		// Create the new task
		newTask := models.Task{
			Title:        title,
			Description:  description,
			Status:       models.StatusTodo,
			Priority:     models.TaskPriority(priorityStr),
			Dependencies: dependencies,
			ParentID:     parentID,
		}

		createdTask, err := taskStore.CreateTask(newTask)
		if err != nil {
			HandleError("Error: Could not create the new task.", err)
		}

		fmt.Printf("✅ Task added successfully!\n")
		fmt.Printf("ID: %s\nTitle: %s\n", createdTask.ID, createdTask.Title)
	},
}

func init() {
	rootCmd.AddCommand(addCmd)

	// Here you will define your flags and configuration settings.
	addCmd.Flags().String("title", "", "Title of the task")
	addCmd.Flags().String("description", "", "Description of the task")
	addCmd.Flags().String("priority", "medium", "Priority of the task (low, medium, high, urgent)")
	addCmd.Flags().String("dependencies", "", "Comma-separated task IDs that this task depends on")
	addCmd.Flags().String("parentID", "", "ID of the parent task")
	addCmd.Flags().Bool("non-interactive", false, "Run in non-interactive mode (requires --title flag)")

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
