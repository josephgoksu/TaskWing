/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/josephgoksu/TaskWing/models"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

// updateCmd represents the update command
var updateCmd = &cobra.Command{
	Use:   "update [task_id]",
	Short: "Update an existing task",
	Long:  `Update an existing task. If task_id is provided, it attempts to update that task directly. Otherwise, it presents an interactive list to choose a task. Allows updating title, description, priority, status, tags, and dependencies.`,
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		taskStore, err := GetStore()
		if err != nil {
			HandleError("Error: Could not initialize the task store.", err)
		}
		defer func() {
			if err := taskStore.Close(); err != nil {
				HandleError("Failed to close task store", err)
			}
		}()

		var taskToUpdate models.Task

		if len(args) > 0 {
			taskID := args[0]
			taskToUpdate, err = taskStore.GetTask(taskID)
			if err != nil {
				HandleError(fmt.Sprintf("Error: Could not find task with ID '%s'.", taskID), err)
			}
		} else {
			taskToUpdate, err = selectTaskInteractive(taskStore, nil, "Select task to update")
			if err != nil {
				if err == promptui.ErrInterrupt {
					fmt.Println("Update cancelled.")
					return
				}
				if err == ErrNoTasksFound {
					fmt.Println("No tasks available to update.")
					return
				}
				HandleError("Error: Could not select a task for updating.", err)
			}
		}

		fmt.Printf("Updating task: %s (ID: %s)\n", taskToUpdate.Title, taskToUpdate.ID)
		updates := make(map[string]interface{})
		interactive := true

		if cmd.Flags().Changed("title") {
			newTitle, _ := cmd.Flags().GetString("title")
			updates["title"] = newTitle
			interactive = false
		}
		if cmd.Flags().Changed("description") {
			newDesc, _ := cmd.Flags().GetString("description")
			updates["description"] = newDesc
			interactive = false
		}
		if cmd.Flags().Changed("priority") {
			newPrio, _ := cmd.Flags().GetString("priority")
			updates["priority"] = newPrio
			interactive = false
		}
		if cmd.Flags().Changed("status") {
			newStatus, _ := cmd.Flags().GetString("status")
			updates["status"] = newStatus
			interactive = false
		}
		if cmd.Flags().Changed("parentID") {
			newParentID, _ := cmd.Flags().GetString("parentID")
			if strings.ToLower(newParentID) == "none" || newParentID == "" {
				updates["parentId"] = nil
			} else {
				updates["parentId"] = newParentID
			}
			interactive = false
		}
		if cmd.Flags().Changed("dependencies") {
			newDeps, _ := cmd.Flags().GetString("dependencies")
			if strings.ToLower(newDeps) == "none" || strings.TrimSpace(newDeps) == "" {
				updates["dependencies"] = []string{}
			} else {
				updates["dependencies"] = strings.Split(strings.ReplaceAll(newDeps, " ", ""), ",")
			}
			interactive = false
		}

		if !interactive {
			if len(updates) == 0 {
				fmt.Println("No update flags provided. Use flags like --title, --description, etc. to update.")
				return
			}
		} else {
			// Fallback to original interactive prompts if no flags were used
			// Prompt for Title (existing)
			titlePrompt := promptui.Prompt{
				Label:   fmt.Sprintf("New Title (current: %s, press Enter to keep)", taskToUpdate.Title),
				Default: taskToUpdate.Title,
				Validate: func(input string) error {
					if len(input) < 3 && input != taskToUpdate.Title {
						return fmt.Errorf("title must be at least 3 characters long")
					}
					return nil
				},
			}
			newTitle, err := titlePrompt.Run()
			if err != nil {
				if err == promptui.ErrInterrupt {
					fmt.Println("Update cancelled.")
					os.Exit(0)
				}
				HandleError("Error: Failed to read new title.", err)
			} else if newTitle != taskToUpdate.Title {
				updates["title"] = newTitle
			}

			// Prompt for Description (existing)
			descPrompt := promptui.Prompt{
				Label:   fmt.Sprintf("New Description (current: %s, press Enter to keep)", taskToUpdate.Description),
				Default: taskToUpdate.Description,
			}
			newDesc, err := descPrompt.Run()
			if err != nil && err != promptui.ErrInterrupt {
				HandleError("Error: Failed to read new description.", err)
			} else if newDesc != taskToUpdate.Description {
				updates["description"] = newDesc
			}

			// Prompt for Priority (existing)
			currentPriorityIndex := 0
			priorityItems := []models.TaskPriority{models.PriorityLow, models.PriorityMedium, models.PriorityHigh, models.PriorityUrgent}
			for i, p := range priorityItems {
				if p == taskToUpdate.Priority {
					currentPriorityIndex = i
					break
				}
			}
			prioSelect := promptui.Select{
				Label:     fmt.Sprintf("New Priority (current: %s)", taskToUpdate.Priority),
				Items:     priorityItems,
				CursorPos: currentPriorityIndex,
			}
			_, newPriorityStr, err := prioSelect.Run()
			if err != nil {
				if err == promptui.ErrInterrupt {
					fmt.Println("Update cancelled.")
					os.Exit(0)
				}
				HandleError("Error: Could not select a new priority.", err)
			} else if models.TaskPriority(newPriorityStr) != taskToUpdate.Priority {
				updates["priority"] = newPriorityStr
			}

			// Prompt for Status (existing)
			currentStatusIndex := 0
			statusItems := []models.TaskStatus{models.StatusTodo, models.StatusDoing, models.StatusReview, models.StatusDone}
			for i, s := range statusItems {
				if s == taskToUpdate.Status {
					currentStatusIndex = i
					break
				}
			}
			statusSelect := promptui.Select{
				Label:     fmt.Sprintf("New Status (current: %s)", taskToUpdate.Status),
				Items:     statusItems,
				CursorPos: currentStatusIndex,
			}
			_, newStatusStr, err := statusSelect.Run()
			if err != nil {
				if err == promptui.ErrInterrupt {
					fmt.Println("Update cancelled.")
					os.Exit(0)
				}
				HandleError("Error: Could not select a new status.", err)
			} else if models.TaskStatus(newStatusStr) != taskToUpdate.Status {
				updates["status"] = newStatusStr
			}

			// Prompt for Managing Parent Task
			manageParentPrompt := promptui.Select{
				Label:     "Manage Parent Task?",
				Items:     []string{"Yes", "No"},
				CursorPos: 1, // Default to No
			}
			_, manageParentChoice, err := manageParentPrompt.Run()
			if err != nil && err != promptui.ErrInterrupt {
				HandleError("Error: Could not read parent management choice.", err)
			}

			if err == nil && manageParentChoice == "Yes" {
				currentParentIDStr := "none"
				if taskToUpdate.ParentID != nil && *taskToUpdate.ParentID != "" {
					currentParentIDStr = *taskToUpdate.ParentID
				}
				parentIDPrompt := promptui.Prompt{
					Label:   fmt.Sprintf("New Parent Task ID (current: %s, leave empty or 'none' to clear, Enter to keep current)", currentParentIDStr),
					Default: currentParentIDStr, // Allows user to press Enter to keep current
					// No complex validation here, store will validate existence and self-parenting
				}
				newParentIDStr, err := parentIDPrompt.Run()
				if err != nil && err != promptui.ErrInterrupt {
					HandleError("Error: Could not read new parent ID.", err)
				} else if err == nil && newParentIDStr != currentParentIDStr {
					trimmedNewParentID := strings.TrimSpace(newParentIDStr)
					if strings.ToLower(trimmedNewParentID) == "none" || trimmedNewParentID == "" {
						updates["parentId"] = nil // Signal to clear parent
					} else {
						updates["parentId"] = trimmedNewParentID
					}
				}
			}

			// Prompt for Managing Dependencies (existing)
			manageDepsPrompt := promptui.Select{
				Label:     "Manage Dependencies?",
				Items:     []string{"Yes", "No"},
				CursorPos: 1,
			}
			_, manageDepsChoice, err := manageDepsPrompt.Run()
			if err != nil && err != promptui.ErrInterrupt {
				HandleError("Error: Could not read dependency management choice.", err)
			}

			if err == nil && manageDepsChoice == "Yes" {
				currentDepsStr := strings.Join(taskToUpdate.Dependencies, ", ")
				depsPrompt := promptui.Prompt{
					Label:   fmt.Sprintf("New Dependencies (comma-separated IDs, current: [%s], Enter to keep, 'none' to clear)", currentDepsStr),
					Default: currentDepsStr,
				}
				newDepsStr, err := depsPrompt.Run()
				if err != nil && err != promptui.ErrInterrupt {
					HandleError("Error: Could not read new dependencies.", err)
				} else if err == nil && newDepsStr != currentDepsStr {
					if strings.ToLower(newDepsStr) == "none" || strings.TrimSpace(newDepsStr) == "" {
						updates["dependencies"] = []string{}
					} else {
						updates["dependencies"] = strings.Split(strings.ReplaceAll(newDepsStr, " ", ""), ",")
					}
				}
			}
		}

		if len(updates) == 0 {
			fmt.Println("No changes detected. Update cancelled.")
			return
		}

		updatedTask, err := taskStore.UpdateTask(taskToUpdate.ID, updates)
		if err != nil {
			HandleError(fmt.Sprintf("Error: Could not update task '%s'.", taskToUpdate.Title), err)
		}

		fmt.Printf("Task %s updated successfully! ID: %s\n", updatedTask.Title, updatedTask.ID)
		if _, ok := updates["parentId"]; ok {
			if updatedTask.ParentID != nil && *updatedTask.ParentID != "" {
				fmt.Printf("Updated Parent Task ID: %s\n", *updatedTask.ParentID)
			} else {
				fmt.Println("Parent task association has been removed.")
			}
		}
		if _, ok := updates["dependencies"]; ok {
			fmt.Printf("Updated Dependencies: %v\n", updatedTask.Dependencies)
		}
	},
}

func init() {
	rootCmd.AddCommand(updateCmd)
	updateCmd.Flags().String("title", "", "New title for the task")
	updateCmd.Flags().String("description", "", "New description for the task")
	updateCmd.Flags().String("priority", "", "New priority for the task (low, medium, high, urgent)")
	updateCmd.Flags().String("status", "", "New status for the task (e.g., pending, in-progress)")
	updateCmd.Flags().String("parentID", "", "New parent task ID. Use 'none' to remove parent.")
	updateCmd.Flags().String("dependencies", "", "New comma-separated list of dependency IDs. Use 'none' to clear.")
}
