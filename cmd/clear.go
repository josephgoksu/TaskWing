/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"encoding/json"
	
	"github.com/josephgoksu/taskwing.app/models"
	"github.com/josephgoksu/taskwing.app/store"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

var (
	clearForce      bool
	clearStatus     string
	clearPriority   string
	clearCompleted  bool
	clearAll        bool
	clearBackup     bool
)

// clearCmd represents the clear command
var clearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Clear tasks from the board with safety options",
	Long: `Clear tasks from your TaskWing board with flexible filtering and safety features.

By default, this command clears only completed tasks. You can specify different criteria:
- Clear by status: --status=todo,doing,review
- Clear by priority: --priority=low,medium
- Clear completed tasks: --completed (default behavior)
- Clear everything: --all (requires confirmation)

Safety features:
- Interactive confirmation (unless --force is used)
- Automatic backup creation (unless --no-backup)
- Shows preview of tasks to be cleared

Examples:
  taskwing clear                    # Clear completed tasks (safe default)
  taskwing clear --status=todo      # Clear only todo tasks
  taskwing clear --priority=low     # Clear only low priority tasks  
  taskwing clear --all              # Clear all tasks (with confirmation)
  taskwing clear --all --force      # Clear all tasks without confirmation`,
	Run: func(cmd *cobra.Command, args []string) {
		taskStore, err := GetStore()
		if err != nil {
			HandleError("Error getting task store", err)
		}
		defer taskStore.Close()

		// Determine what to clear based on flags
		filterFn := buildClearFilter(cmd)
		
		// Get tasks to be cleared
		tasksToDelete, err := taskStore.ListTasks(filterFn, nil)
		if err != nil {
			HandleError("Error listing tasks", err)
		}

		if len(tasksToDelete) == 0 {
			fmt.Println("No tasks match the clearing criteria.")
			return
		}

		// Show preview of what will be cleared
		showClearPreview(tasksToDelete)

		// Get confirmation unless --force is used
		if !clearForce {
			if err := confirmClear(tasksToDelete); err != nil {
				fmt.Println("Clear operation cancelled.")
				return
			}
		}

		// Create backup unless disabled
		if clearBackup {
			if err := createClearBackup(taskStore, tasksToDelete); err != nil {
				fmt.Printf("Warning: Failed to create backup: %v\n", err)
				if !clearForce {
					fmt.Println("Clear operation cancelled for safety.")
					return
				}
			}
		}

		// Perform the clearing
		cleared, failed := performClear(taskStore, tasksToDelete)
		
		// Report results
		fmt.Printf("\n✅ Successfully cleared %d tasks\n", cleared)
		if failed > 0 {
			fmt.Printf("⚠️  Failed to clear %d tasks\n", failed)
		}

		// Clear current task if it was deleted
		clearCurrentTaskIfDeleted(tasksToDelete)
	},
}

func buildClearFilter(cmd *cobra.Command) func(models.Task) bool {
	return func(task models.Task) bool {
		// If --all is specified, match everything
		if clearAll {
			return true
		}

		// If --completed is specified or no other filters, match completed tasks
		if clearCompleted || (!cmd.Flag("status").Changed && !cmd.Flag("priority").Changed) {
			return task.Status == models.StatusDone
		}

		// Check status filter
		if clearStatus != "" {
			statusList := strings.Split(strings.ToLower(clearStatus), ",")
			statusMatch := false
			for _, status := range statusList {
				status = strings.TrimSpace(status)
				switch status {
				case "todo":
					if task.Status == models.StatusTodo {
						statusMatch = true
					}
				case "doing":
					if task.Status == models.StatusDoing {
						statusMatch = true
					}
				case "review":
					if task.Status == models.StatusReview {
						statusMatch = true
					}
				case "done":
					if task.Status == models.StatusDone {
						statusMatch = true
					}
				}
			}
			if !statusMatch {
				return false
			}
		}

		// Check priority filter
		if clearPriority != "" {
			priorityList := strings.Split(strings.ToLower(clearPriority), ",")
			priorityMatch := false
			for _, priority := range priorityList {
				priority = strings.TrimSpace(priority)
				switch priority {
				case "low":
					if task.Priority == models.PriorityLow {
						priorityMatch = true
					}
				case "medium":
					if task.Priority == models.PriorityMedium {
						priorityMatch = true
					}
				case "high":
					if task.Priority == models.PriorityHigh {
						priorityMatch = true
					}
				case "urgent":
					if task.Priority == models.PriorityUrgent {
						priorityMatch = true
					}
				}
			}
			if !priorityMatch {
				return false
			}
		}

		return true
	}
}

func showClearPreview(tasks []models.Task) {
	fmt.Printf("\n📋 Tasks to be cleared (%d total):\n\n", len(tasks))
	
	// Group by status for better overview
	statusGroups := make(map[models.TaskStatus][]models.Task)
	for _, task := range tasks {
		statusGroups[task.Status] = append(statusGroups[task.Status], task)
	}

	for status, groupTasks := range statusGroups {
		fmt.Printf("  %s (%d tasks):\n", strings.ToUpper(string(status)), len(groupTasks))
		for _, task := range groupTasks {
			priority := ""
			switch task.Priority {
			case models.PriorityHigh, models.PriorityUrgent:
				priority = " [HIGH]"
			}
			fmt.Printf("    • %s%s\n", task.Title, priority)
		}
		fmt.Println()
	}
}

func confirmClear(tasks []models.Task) error {
	prompt := promptui.Prompt{
		Label: fmt.Sprintf("Clear %d tasks permanently? This action cannot be undone", len(tasks)),
		IsConfirm: true,
	}

	_, err := prompt.Run()
	if err == promptui.ErrAbort {
		return fmt.Errorf("cancelled by user")
	}
	return err
}

func createClearBackup(taskStore store.TaskStore, tasksToDelete []models.Task) error {
	cfg := GetConfig()
	backupDir := filepath.Join(cfg.Project.RootDir, "backups")
	
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return fmt.Errorf("failed to create backup directory: %w", err)
	}

	timestamp := time.Now().Format("2006-01-02_15-04-05")
	backupFile := filepath.Join(backupDir, fmt.Sprintf("clear_backup_%s.json", timestamp))

	// Create backup data
	backupData := struct {
		Timestamp time.Time     `json:"timestamp"`
		Operation string        `json:"operation"`
		TaskCount int           `json:"task_count"`
		Tasks     []models.Task `json:"tasks"`
	}{
		Timestamp: time.Now(),
		Operation: "clear",
		TaskCount: len(tasksToDelete),
		Tasks:     tasksToDelete,
	}

	if err := writeJSONFile(backupFile, backupData); err != nil {
		return fmt.Errorf("failed to write backup file: %w", err)
	}

	fmt.Printf("📦 Backup created: %s\n", backupFile)
	return nil
}

func performClear(taskStore store.TaskStore, tasks []models.Task) (int, int) {
	cleared := 0
	failed := 0

	for _, task := range tasks {
		if err := taskStore.DeleteTask(task.ID); err != nil {
			fmt.Printf("❌ Failed to clear task '%s': %v\n", task.Title, err)
			failed++
		} else {
			cleared++
		}
	}

	return cleared, failed
}

func clearCurrentTaskIfDeleted(deletedTasks []models.Task) {
	currentTaskID := GetCurrentTask()
	if currentTaskID == "" {
		return
	}

	for _, task := range deletedTasks {
		if task.ID == currentTaskID {
			if err := ClearCurrentTask(); err != nil {
				fmt.Printf("⚠️  Warning: Could not clear current task reference: %v\n", err)
			} else {
				fmt.Println("ℹ️  Cleared current task reference (task was deleted)")
			}
			break
		}
	}
}

// writeJSONFile writes data as JSON to the specified file
func writeJSONFile(filename string, data interface{}) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}

func init() {
	rootCmd.AddCommand(clearCmd)
	
	clearCmd.Flags().BoolVarP(&clearForce, "force", "f", false, "Skip confirmation prompt")
	clearCmd.Flags().StringVar(&clearStatus, "status", "", "Clear tasks by status (comma-separated: todo,doing,review,done)")
	clearCmd.Flags().StringVar(&clearPriority, "priority", "", "Clear tasks by priority (comma-separated: low,medium,high,urgent)")
	clearCmd.Flags().BoolVar(&clearCompleted, "completed", false, "Clear only completed tasks (default behavior)")
	clearCmd.Flags().BoolVar(&clearAll, "all", false, "Clear all tasks (requires confirmation)")
	clearCmd.Flags().BoolVar(&clearBackup, "backup", true, "Create backup before clearing (default: true)")
	clearCmd.Flags().BoolVar(&clearBackup, "no-backup", false, "Skip backup creation")
	
	// Make --no-backup set backup to false
	clearCmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		if cmd.Flag("no-backup").Changed {
			clearBackup = false
		}
		return nil
	}
}