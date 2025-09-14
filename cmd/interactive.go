/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

import (
	"fmt"
	"strings"

	"github.com/josephgoksu/TaskWing/models"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

// interactiveCmd represents the interactive command
var interactiveCmd = &cobra.Command{
	Use:   "interactive",
	Short: "Interactive menu for common TaskWing operations",
	Long: `Interactive mode provides a guided menu interface for TaskWing operations.
Perfect for users who prefer menu-driven workflows or are new to command-line tools.

This mode allows you to:
- Create new tasks
- View and manage existing tasks
- Start, complete, and update tasks
- Navigate through your task workflow

Use arrow keys to navigate and Enter to select.`,
	Aliases: []string{"menu", "ui"},
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("🎯 Welcome to TaskWing Interactive Mode!")
		fmt.Println("Use arrow keys to navigate, Enter to select, and Ctrl+C to exit.")
		fmt.Println()

		for runInteractiveMenu() {
			// Continue running the interactive menu until user chooses to exit
		}

		fmt.Println("👋 Goodbye!")
	},
}

func init() {
	rootCmd.AddCommand(interactiveCmd)
}

// MenuItem represents a menu option
type MenuItem struct {
	Label       string
	Description string
	Action      func() error
}

// runInteractiveMenu displays the main menu and handles user selection
func runInteractiveMenu() bool {
	taskStore, err := GetStore()
	if err != nil {
		fmt.Printf("❌ Error: Could not initialize task store: %v\n", err)
		return false
	}
	defer func() {
		if err := taskStore.Close(); err != nil {
			fmt.Printf("Warning: Failed to close task store: %v\n", err)
		}
	}()

	// Get task counts for context
	tasks, _ := taskStore.ListTasks(nil, nil)
	todoCount := 0
	doingCount := 0
	doneCount := 0

	for _, task := range tasks {
		switch task.Status {
		case models.StatusTodo:
			todoCount++
		case models.StatusDoing:
			doingCount++
		case models.StatusDone:
			doneCount++
		}
	}

	// Create menu items
	menuItems := []MenuItem{
		{
			Label:       "📝 Create New Task",
			Description: "Add a new task to your list",
			Action: func() error {
				return handleCreateTask()
			},
		},
		{
			Label:       fmt.Sprintf("📋 View Tasks (%d total)", len(tasks)),
			Description: "Browse and manage your existing tasks",
			Action: func() error {
				return handleViewTasks()
			},
		},
		{
			Label:       fmt.Sprintf("🚀 Start Working (%d todo)", todoCount),
			Description: "Select a task to begin working on",
			Action: func() error {
				return handleStartTask()
			},
		},
		{
			Label:       fmt.Sprintf("✅ Complete Task (%d in progress)", doingCount),
			Description: "Mark a task as finished",
			Action: func() error {
				return handleCompleteTask()
			},
		},
		{
			Label:       "🔍 Search Tasks",
			Description: "Find specific tasks by title or description",
			Action: func() error {
				return handleSearchTasks()
			},
		},
		{
			Label:       "⚙️ Task Management",
			Description: "Advanced operations (update, delete, clear)",
			Action: func() error {
				return handleTaskManagement()
			},
		},
		{
			Label:       "📊 Project Status",
			Description: fmt.Sprintf("Overview - Todo: %d, Doing: %d, Done: %d", todoCount, doingCount, doneCount),
			Action: func() error {
				return handleProjectStatus()
			},
		},
		{
			Label:       "🚪 Exit",
			Description: "Leave interactive mode",
			Action: func() error {
				return fmt.Errorf("exit")
			},
		},
	}

	// Create menu prompt
	templates := &promptui.SelectTemplates{
		Label:    "{{ . }}",
		Active:   "▶ {{ .Label | cyan }}",
		Inactive: "  {{ .Label | white }}",
		Selected: "▶ {{ .Label | green | bold }}",
		Details: `
{{ "Description:" | faint }} {{ .Description }}`,
	}

	prompt := promptui.Select{
		Label:     "What would you like to do?",
		Items:     menuItems,
		Templates: templates,
		Size:      10,
	}

	i, _, err := prompt.Run()
	if err != nil {
		if err == promptui.ErrInterrupt {
			return false
		}
		fmt.Printf("❌ Selection error: %v\n", err)
		return false
	}

	// Execute selected action
	err = menuItems[i].Action()
	if err != nil {
		if err.Error() == "exit" {
			return false
		}
		fmt.Printf("❌ Error: %v\n", err)
		pressAnyKey()
	}

	return true
}

// handleCreateTask prompts for a new task
func handleCreateTask() error {
	fmt.Println("\n📝 Create New Task")

	titlePrompt := promptui.Prompt{
		Label: "Task title",
		Validate: func(input string) error {
			if len(strings.TrimSpace(input)) < 3 {
				return fmt.Errorf("task title must be at least 3 characters")
			}
			return nil
		},
	}

	title, err := titlePrompt.Run()
	if err != nil {
		if err == promptui.ErrInterrupt {
			return nil
		}
		return err
	}

	descPrompt := promptui.Prompt{
		Label:   "Description (optional)",
		Default: title,
	}

	description, err := descPrompt.Run()
	if err != nil {
		if err == promptui.ErrInterrupt {
			return nil
		}
		description = title
	}

	// Priority selection
	priorities := []string{"low", "medium", "high", "urgent"}
	priorityPrompt := promptui.Select{
		Label: "Priority",
		Items: priorities,
		Size:  4,
	}

	_, priority, err := priorityPrompt.Run()
	if err != nil {
		if err == promptui.ErrInterrupt {
			return nil
		}
		priority = "medium"
	}

	// Create the task
	taskStore, err := GetStore()
	if err != nil {
		return err
	}
	defer func() {
		if err := taskStore.Close(); err != nil {
			fmt.Printf("Warning: Failed to close task store: %v\n", err)
		}
	}()

	newTask := models.Task{
		Title:       title,
		Description: description,
		Status:      models.StatusTodo,
		Priority:    models.TaskPriority(priority),
	}

	createdTask, err := taskStore.CreateTask(newTask)
	if err != nil {
		return err
	}

	fmt.Printf("✅ Task created: %s (ID: %s)\n", createdTask.Title, createdTask.ID[:8])
	pressAnyKey()
	return nil
}

// handleViewTasks shows the task list
func handleViewTasks() error {
	fmt.Println("\n📋 Task List")

	taskStore, err := GetStore()
	if err != nil {
		return err
	}
	defer func() {
		if err := taskStore.Close(); err != nil {
			fmt.Printf("Warning: Failed to close task store: %v\n", err)
		}
	}()

	tasks, err := taskStore.ListTasks(nil, nil)
	if err != nil {
		return err
	}

	if len(tasks) == 0 {
		fmt.Println("No tasks found. Create your first task!")
		pressAnyKey()
		return nil
	}

	// Format tasks for selection
	templates := &promptui.SelectTemplates{
		Label:    "{{ . }}",
		Active:   "▶ {{ .Title | cyan }} ({{ .Status | yellow }}) [{{ .ID | faint }}]",
		Inactive: "  {{ .Title | white }} ({{ .Status | faint }}) [{{ .ID | faint }}]",
		Selected: "▶ {{ .Title | green }}",
		Details: `
{{ "ID:" | faint }} {{ .ID }}
{{ "Status:" | faint }} {{ .Status }}
{{ "Priority:" | faint }} {{ .Priority }}
{{ "Description:" | faint }} {{ .Description }}`,
	}

	prompt := promptui.Select{
		Label:     "Select task to view details (press Enter or Ctrl+C to go back)",
		Items:     tasks,
		Templates: templates,
		Size:      10,
	}

	_, _, err = prompt.Run()
	if err != nil && err != promptui.ErrInterrupt {
		return err
	}

	return nil
}

// handleStartTask helps user start working on a task
func handleStartTask() error {
	fmt.Println("\n🚀 Start Working on Task")

	taskStore, err := GetStore()
	if err != nil {
		return err
	}
	defer func() {
		if err := taskStore.Close(); err != nil {
			fmt.Printf("Warning: Failed to close task store: %v\n", err)
		}
	}()

	// Filter for todo tasks
	tasks, err := taskStore.ListTasks(func(t models.Task) bool {
		return t.Status == models.StatusTodo
	}, nil)
	if err != nil {
		return err
	}

	if len(tasks) == 0 {
		fmt.Println("No tasks available to start. Create a task first!")
		pressAnyKey()
		return nil
	}

	task, err := selectTaskInteractive(taskStore, func(t models.Task) bool {
		return t.Status == models.StatusTodo
	}, "Select task to start")
	if err != nil {
		if err == promptui.ErrInterrupt {
			return nil
		}
		return err
	}

	// Update task status
	_, err = taskStore.UpdateTask(task.ID, map[string]interface{}{
		"status": string(models.StatusDoing),
	})
	if err != nil {
		return err
	}

	// Set as current task
	if err := SetCurrentTask(task.ID); err != nil {
		fmt.Printf("Warning: Could not set as current task: %v\n", err)
	}

	fmt.Printf("🎯 Started working on: %s\n", task.Title)
	pressAnyKey()
	return nil
}

// handleCompleteTask helps user complete a task
func handleCompleteTask() error {
	fmt.Println("\n✅ Complete Task")

	taskStore, err := GetStore()
	if err != nil {
		return err
	}
	defer func() {
		if err := taskStore.Close(); err != nil {
			fmt.Printf("Warning: Failed to close task store: %v\n", err)
		}
	}()

	// Filter for non-done tasks
	tasks, err := taskStore.ListTasks(func(t models.Task) bool {
		return t.Status != models.StatusDone
	}, nil)
	if err != nil {
		return err
	}

	if len(tasks) == 0 {
		fmt.Println("No active tasks to complete!")
		pressAnyKey()
		return nil
	}

	task, err := selectTaskInteractive(taskStore, func(t models.Task) bool {
		return t.Status != models.StatusDone
	}, "Select task to complete")
	if err != nil {
		if err == promptui.ErrInterrupt {
			return nil
		}
		return err
	}

	// Mark as done
	_, err = taskStore.MarkTaskDone(task.ID)
	if err != nil {
		return err
	}

	// Clear current task if this was the current one
	currentTaskID := GetCurrentTask()
	if currentTaskID == task.ID {
		if err := ClearCurrentTask(); err != nil {
			fmt.Printf("Warning: failed to clear current task: %v\n", err)
		}
	}

	fmt.Printf("🎉 Completed: %s\n", task.Title)
	pressAnyKey()
	return nil
}

// handleSearchTasks provides search functionality
func handleSearchTasks() error {
	fmt.Println("\n🔍 Search Tasks")

	searchPrompt := promptui.Prompt{
		Label: "Enter search term",
	}

	searchTerm, err := searchPrompt.Run()
	if err != nil {
		if err == promptui.ErrInterrupt {
			return nil
		}
		return err
	}

	if strings.TrimSpace(searchTerm) == "" {
		return nil
	}

	taskStore, err := GetStore()
	if err != nil {
		return err
	}
	defer func() {
		if err := taskStore.Close(); err != nil {
			fmt.Printf("Warning: Failed to close task store: %v\n", err)
		}
	}()

	// Simple search in title and description
	tasks, err := taskStore.ListTasks(func(t models.Task) bool {
		term := strings.ToLower(searchTerm)
		return strings.Contains(strings.ToLower(t.Title), term) ||
			strings.Contains(strings.ToLower(t.Description), term)
	}, nil)
	if err != nil {
		return err
	}

	if len(tasks) == 0 {
		fmt.Printf("No tasks found matching '%s'\n", searchTerm)
		pressAnyKey()
		return nil
	}

	fmt.Printf("Found %d task(s) matching '%s':\n", len(tasks), searchTerm)
	for _, task := range tasks {
		fmt.Printf("  • %s (ID: %s, Status: %s)\n", task.Title, task.ID[:8], task.Status)
	}
	pressAnyKey()
	return nil
}

// handleTaskManagement provides advanced operations
func handleTaskManagement() error {
	fmt.Println("\n⚙️ Task Management")

	operations := []string{
		"Update task details",
		"Delete a task",
		"Clear completed tasks",
		"Back to main menu",
	}

	opPrompt := promptui.Select{
		Label: "Select operation",
		Items: operations,
		Size:  4,
	}

	i, _, err := opPrompt.Run()
	if err != nil {
		return err
	}

	switch i {
	case 0:
		return handleUpdateTask()
	case 1:
		return handleDeleteTask()
	case 2:
		return handleClearTasks()
	case 3:
		return nil
	}

	return nil
}

// handleUpdateTask provides task update functionality
func handleUpdateTask() error {
	taskStore, err := GetStore()
	if err != nil {
		return err
	}
	defer func() {
		if err := taskStore.Close(); err != nil {
			fmt.Printf("Warning: Failed to close task store: %v\n", err)
		}
	}()

	task, err := selectTaskInteractive(taskStore, func(t models.Task) bool {
		return true
	}, "Select task to update")
	if err != nil {
		if err == promptui.ErrInterrupt {
			return nil
		}
		return err
	}

	fmt.Printf("\nUpdating: %s\n", task.Title)

	fields := []string{"title", "description", "priority", "status", "cancel"}
	fieldPrompt := promptui.Select{
		Label: "What would you like to update?",
		Items: fields,
		Size:  5,
	}

	i, _, err := fieldPrompt.Run()
	if err != nil || i == 4 {
		return err
	}

	updates := make(map[string]interface{})

	switch i {
	case 0: // title
		titlePrompt := promptui.Prompt{
			Label:   "New title",
			Default: task.Title,
		}
		newTitle, err := titlePrompt.Run()
		if err == nil {
			updates["title"] = newTitle
		}
	case 1: // description
		descPrompt := promptui.Prompt{
			Label:   "New description",
			Default: task.Description,
		}
		newDesc, err := descPrompt.Run()
		if err == nil {
			updates["description"] = newDesc
		}
	case 2: // priority
		priorities := []string{"low", "medium", "high", "urgent"}
		priorityPrompt := promptui.Select{
			Label: "New priority",
			Items: priorities,
		}
		_, newPriority, err := priorityPrompt.Run()
		if err == nil {
			updates["priority"] = newPriority
		}
	case 3: // status
		statuses := []string{"todo", "doing", "review", "done"}
		statusPrompt := promptui.Select{
			Label: "New status",
			Items: statuses,
		}
		_, newStatus, err := statusPrompt.Run()
		if err == nil {
			updates["status"] = newStatus
		}
	}

	if len(updates) > 0 {
		_, err = taskStore.UpdateTask(task.ID, updates)
		if err != nil {
			return err
		}
		fmt.Println("✅ Task updated successfully!")
	}

	pressAnyKey()
	return nil
}

// handleDeleteTask provides task deletion
func handleDeleteTask() error {
	taskStore, err := GetStore()
	if err != nil {
		return err
	}
	defer func() {
		if err := taskStore.Close(); err != nil {
			fmt.Printf("Warning: Failed to close task store: %v\n", err)
		}
	}()

	task, err := selectTaskInteractive(taskStore, func(t models.Task) bool {
		return true
	}, "Select task to delete")
	if err != nil {
		if err == promptui.ErrInterrupt {
			return nil
		}
		return err
	}

	confirmPrompt := promptui.Prompt{
		Label:     fmt.Sprintf("Are you sure you want to delete '%s'? (type 'yes' to confirm)", task.Title),
		IsConfirm: false,
	}

	confirmation, err := confirmPrompt.Run()
	if err != nil || strings.ToLower(confirmation) != "yes" {
		fmt.Println("Delete cancelled.")
		pressAnyKey()
		return nil
	}

	err = taskStore.DeleteTask(task.ID)
	if err != nil {
		return err
	}

	fmt.Printf("✅ Deleted: %s\n", task.Title)
	pressAnyKey()
	return nil
}

// handleClearTasks clears completed tasks
func handleClearTasks() error {
	taskStore, err := GetStore()
	if err != nil {
		return err
	}
	defer func() {
		if err := taskStore.Close(); err != nil {
			fmt.Printf("Warning: Failed to close task store: %v\n", err)
		}
	}()

	// Count done tasks
	doneTasks, err := taskStore.ListTasks(func(t models.Task) bool {
		return t.Status == models.StatusDone
	}, nil)
	if err != nil {
		return err
	}

	if len(doneTasks) == 0 {
		fmt.Println("No completed tasks to clear.")
		pressAnyKey()
		return nil
	}

	confirmPrompt := promptui.Prompt{
		Label:     fmt.Sprintf("Clear %d completed tasks? (y/n)", len(doneTasks)),
		IsConfirm: true,
	}

	_, err = confirmPrompt.Run()
	if err != nil {
		fmt.Println("Clear cancelled.")
		pressAnyKey()
		return nil
	}

	// Delete all done tasks in batch for better relationship handling
	ids := make([]string, 0, len(doneTasks))
	for _, t := range doneTasks {
		ids = append(ids, t.ID)
	}
	deletedCount, err := taskStore.DeleteTasks(ids)
	if err != nil {
		fmt.Printf("Warning: Batch clear encountered an error: %v\n", err)
	}
	fmt.Printf("✅ Cleared %d completed tasks\n", deletedCount)
	pressAnyKey()
	return nil
}

// handleProjectStatus shows project overview
func handleProjectStatus() error {
	fmt.Println("\n📊 Project Status")

	taskStore, err := GetStore()
	if err != nil {
		return err
	}
	defer func() {
		if err := taskStore.Close(); err != nil {
			fmt.Printf("Warning: Failed to close task store: %v\n", err)
		}
	}()

	tasks, err := taskStore.ListTasks(nil, nil)
	if err != nil {
		return err
	}

	todoCount := 0
	doingCount := 0
	reviewCount := 0
	doneCount := 0

	for _, task := range tasks {
		switch task.Status {
		case models.StatusTodo:
			todoCount++
		case models.StatusDoing:
			doingCount++
		case models.StatusReview:
			reviewCount++
		case models.StatusDone:
			doneCount++
		}
	}

	fmt.Printf("Total Tasks: %d\n", len(tasks))
	fmt.Printf("  📝 Todo: %d\n", todoCount)
	fmt.Printf("  🚀 Doing: %d\n", doingCount)
	fmt.Printf("  👀 Review: %d\n", reviewCount)
	fmt.Printf("  ✅ Done: %d\n", doneCount)

	if len(tasks) > 0 {
		completionRate := float64(doneCount) / float64(len(tasks)) * 100
		fmt.Printf("\n📈 Completion Rate: %.1f%%\n", completionRate)
	}

	// Show current task
	currentTaskID := GetCurrentTask()
	if currentTaskID != "" {
		if currentTask, err := taskStore.GetTask(currentTaskID); err == nil {
			fmt.Printf("\n🎯 Current Task: %s\n", currentTask.Title)
		}
	}

	pressAnyKey()
	return nil
}

// pressAnyKey pauses until user presses enter
func pressAnyKey() {
	fmt.Print("\nPress Enter to continue...")
	_, _ = fmt.Scanln() // Ignore error as this is just for user interaction
	fmt.Println()
}
