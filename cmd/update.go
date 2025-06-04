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

// updateCmd represents the update command
var updateCmd = &cobra.Command{
	Use:   "update [task_id]",
	Short: "Update an existing task",
	Long:  `Update an existing task. If task_id is provided, it attempts to update that task directly. Otherwise, it presents an interactive list to choose a task. Allows updating title, description, priority, status, tags, and dependencies.`,
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		taskStore, err := getStore()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to get store: %v\n", err)
			os.Exit(1)
		}
		defer taskStore.Close()

		var taskToUpdate models.Task

		if len(args) > 0 {
			taskID := args[0]
			taskToUpdate, err = taskStore.GetTask(taskID)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to get task %s: %v\n", taskID, err)
				os.Exit(1)
			}
		} else {
			tasks, err := taskStore.ListTasks(nil, nil)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to list tasks for selection: %v\n", err)
				os.Exit(1)
			}
			if len(tasks) == 0 {
				fmt.Println("No tasks available to update.")
				return
			}

			templates := &promptui.SelectTemplates{
				Label:    "{{ . }}?",
				Active:   `> {{ .Title | cyan }} ({{ .ID | red }})`,
				Inactive: `  {{ .Title | faint }} ({{ .ID | faint }})`,
				Selected: `{{ "✔" | green }} {{ .Title | faint }}`,
				Details: `
--------- Task Details ----------
{{ "ID:	" | faint }} {{ .ID }}
{{ "Title:	" | faint }} {{ .Title }}
{{ "Status:	" | faint }} {{ .Status }}
{{ "Priority:	" | faint }} {{ .Priority }}
{{ "Description:	" | faint }} {{ .Description }}
{{ "Tags:	" | faint }} {{ if .Tags }}{{ Join .Tags ", " }}{{ else }}None{{ end }}`,
			}

			searcher := func(input string, index int) bool {
				task := tasks[index]
				name := strings.ToLower(task.Title)
				input = strings.ToLower(input)
				return strings.Contains(name, input) || strings.Contains(task.ID, input)
			}

			prompt := promptui.Select{
				Label:     "Select task to update",
				Items:     tasks,
				Templates: templates,
				Searcher:  searcher,
			}

			i, _, err := prompt.Run()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Task selection failed %v\n", err)
				os.Exit(1)
			}
			taskToUpdate = tasks[i]
		}

		fmt.Printf("Updating task: %s (ID: %s)\n", taskToUpdate.Title, taskToUpdate.ID)
		updates := make(map[string]interface{})

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
		if err != nil && err != promptui.ErrInterrupt {
			fmt.Fprintf(os.Stderr, "Title prompt failed: %v\n", err)
			os.Exit(1)
		} else if err == nil && newTitle != taskToUpdate.Title {
			updates["title"] = newTitle
		}

		// Prompt for Description (existing)
		descPrompt := promptui.Prompt{
			Label:   fmt.Sprintf("New Description (current: %s, press Enter to keep)", taskToUpdate.Description),
			Default: taskToUpdate.Description,
		}
		newDesc, err := descPrompt.Run()
		if err != nil && err != promptui.ErrInterrupt {
			fmt.Fprintf(os.Stderr, "Description prompt failed: %v\n", err)
			os.Exit(1)
		} else if err == nil && newDesc != taskToUpdate.Description {
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
		if err != nil && err != promptui.ErrInterrupt {
			fmt.Fprintf(os.Stderr, "Priority selection failed: %v\n", err)
			os.Exit(1)
		} else if err == nil && models.TaskPriority(newPriorityStr) != taskToUpdate.Priority {
			updates["priority"] = newPriorityStr
		}

		// Prompt for Status (existing)
		currentStatusIndex := 0
		statusItems := []models.TaskStatus{models.StatusPending, models.StatusInProgress, models.StatusOnHold, models.StatusBlocked, models.StatusNeedsReview, models.StatusCompleted, models.StatusCancelled}
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
		if err != nil && err != promptui.ErrInterrupt {
			fmt.Fprintf(os.Stderr, "Status selection failed: %v\n", err)
			os.Exit(1)
		} else if err == nil && models.TaskStatus(newStatusStr) != taskToUpdate.Status {
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
			fmt.Fprintf(os.Stderr, "Parent task management choice failed: %v\n", err)
			os.Exit(1)
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
				fmt.Fprintf(os.Stderr, "Parent ID prompt failed: %v\n", err)
				os.Exit(1)
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
			fmt.Fprintf(os.Stderr, "Dependency management choice failed: %v\n", err)
			os.Exit(1)
		}

		if err == nil && manageDepsChoice == "Yes" {
			currentDepsStr := strings.Join(taskToUpdate.Dependencies, ", ")
			depsPrompt := promptui.Prompt{
				Label:   fmt.Sprintf("New Dependencies (comma-separated IDs, current: [%s], Enter to keep, 'none' to clear)", currentDepsStr),
				Default: currentDepsStr,
			}
			newDepsStr, err := depsPrompt.Run()
			if err != nil && err != promptui.ErrInterrupt {
				fmt.Fprintf(os.Stderr, "Dependencies prompt failed: %v\n", err)
				os.Exit(1)
			} else if err == nil && newDepsStr != currentDepsStr {
				if strings.ToLower(newDepsStr) == "none" || strings.TrimSpace(newDepsStr) == "" {
					updates["dependencies"] = []string{}
				} else {
					updates["dependencies"] = strings.Split(strings.ReplaceAll(newDepsStr, " ", ""), ",")
				}
			}
		}

		if len(updates) == 0 {
			fmt.Println("No changes detected. Update cancelled.")
			return
		}

		updatedTask, err := taskStore.UpdateTask(taskToUpdate.ID, updates)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to update task %s: %v\n", taskToUpdate.ID, err)
			os.Exit(1)
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
}
