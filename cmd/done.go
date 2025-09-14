package cmd

import (
	"fmt"
	"strings"

	"github.com/josephgoksu/TaskWing/models"
	"github.com/josephgoksu/TaskWing/store"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

var (
	doneArchive   bool
	doneLessons   string
	doneTags      string
	doneAISuggest bool
	doneAIFix     bool
	doneAIAuto    bool
)

// doneCmd represents the done command
var doneCmd = &cobra.Command{
	Use:     "done [task_id]",
	Aliases: []string{"finish", "complete", "d"},
	Short:   "Mark a task as done",
	Long:    `Mark a task as completed. If task_id is provided, it attempts to mark that task directly. Otherwise, it presents an interactive list to choose a task.`,
	Example: `  # Interactive mode
  taskwing done

  # Complete specific task
  taskwing done abc123

  # Using alias
  taskwing d abc123`,
	Args: cobra.MaximumNArgs(1),
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

		var taskToMarkDone models.Task

		if len(args) > 0 {
			taskID := args[0]
			taskPtr, err := resolveTaskReference(taskStore, taskID)
			if err != nil {
				HandleError(fmt.Sprintf("Error: Could not find task with ID '%s'.", taskID), err)
			}
			taskToMarkDone = *taskPtr
		} else {
			// Filter for tasks that are not yet completed
			notDoneFilter := func(t models.Task) bool {
				return t.Status != models.StatusDone
			}
			taskToMarkDone, err = selectTaskInteractive(taskStore, notDoneFilter, "Select task to mark as done")
			if err != nil {
				if err == promptui.ErrInterrupt {
					fmt.Println("Operation cancelled.")
					return
				}
				if err == ErrNoTasksFound {
					fmt.Println("No active tasks available to mark as done.")
					return
				}
				HandleError("Error: Could not select a task.", err)
			}
		}

		if taskToMarkDone.Status == models.StatusDone {
			fmt.Printf("Task '%s' (ID: %s) is already completed.\n", taskToMarkDone.Title, taskToMarkDone.ID)
			return
		}

		updatedTask, err := taskStore.MarkTaskDone(taskToMarkDone.ID)
		if err != nil {
			HandleError(fmt.Sprintf("Error: Failed to mark task '%s' as done.", taskToMarkDone.Title), err)
		}

		// Clear current task if this was the current one
		currentTaskID := GetCurrentTask()
		if currentTaskID == taskToMarkDone.ID {
			if err := ClearCurrentTask(); err != nil {
				fmt.Printf("Warning: failed to clear current task: %v\n", err)
			}
		}

		fmt.Printf("🎉 Task '%s' (ID: %s) marked as done successfully!\n", updatedTask.Title, updatedTask.ID)

		// Prompt for lessons learned and archival (or use flags for non-interactive)
		fmt.Println()
		lessons := strings.TrimSpace(doneLessons)
		if lessons == "" {
			lessons = gatherLessonsInteractive(updatedTask, doneAISuggest, doneAIAuto, doneAIFix)
		}

		archiveNow := doneArchive
		if !doneArchive {
			confirmArchive := promptui.Select{Label: "Archive this task now?", Items: []string{"Yes", "No"}}
			if _, choice, err := confirmArchive.Run(); err == nil && choice == "Yes" {
				archiveNow = true
			}
		}

		if archiveNow {
			// Tags from flag or prompt
			tags := []string{}
			tagsInput := strings.TrimSpace(doneTags)
			if tagsInput == "" {
				tagsInput, _ = promptInput("Tags (comma-separated, optional)")
			}
			if tagsInput != "" {
				for _, t := range strings.Split(tagsInput, ",") {
					tt := strings.TrimSpace(t)
					if tt != "" {
						tags = append(tags, tt)
					}
				}
			}
			arch := store.NewFileArchiveStore()
			if err := arch.Initialize(map[string]string{"archiveDir": getArchiveDir()}); err != nil {
				fmt.Printf("Warning: failed to init archive store: %v\n", err)
			} else {
				defer arch.Close()
				if doneAIFix && strings.TrimSpace(lessons) != "" {
					if polished, ok := aiPolishLessons(lessons); ok {
						lessons = polished
					}
				}
				entry, err := arch.CreateFromTask(updatedTask, lessons, tags)
				if err != nil {
					fmt.Printf("Warning: failed to create archive entry: %v\n", err)
				} else {
					// friendly summary
					_, path, _ := arch.GetByID(entry.ID)
					short := entry.ID
					if len(short) > 8 {
						short = short[:8]
					}
					fmt.Printf("🗄️  Archived: %s (archive-id: %s)\n", entry.Title, short)
					if path != "" {
						fmt.Printf("     ↳ %s\n", path)
					}
					fmt.Printf("     View:   taskwing archive view %s\n", short)
					fmt.Printf("     Search: taskwing archive search \"%s\"\n", entry.Title)
				}
			}
		}

		// Command discovery hints
		fmt.Printf("\n💡 What's next?\n")
		fmt.Printf("   • Add new task:   taskwing add \"Your next task\"\n")
		fmt.Printf("   • Find next task: taskwing next\n")
		fmt.Printf("   • View all tasks: taskwing list\n")
	},
}

func init() {
	rootCmd.AddCommand(doneCmd)
	doneCmd.Flags().BoolVar(&doneArchive, "archive", false, "Archive task immediately (non-interactive)")
	doneCmd.Flags().StringVar(&doneLessons, "lessons", "", "Lessons learned text (non-interactive)")
	doneCmd.Flags().StringVar(&doneTags, "tags", "", "Comma-separated tags for the archive (non-interactive)")
	doneCmd.Flags().BoolVar(&doneAISuggest, "ai-suggest", true, "Use AI to propose lessons learned suggestions (default: true)")
	doneCmd.Flags().BoolVar(&doneAIFix, "ai-fix", true, "Use AI to polish/grammar-fix lessons text (default: true)")
	doneCmd.Flags().BoolVar(&doneAIAuto, "ai-auto", false, "Auto-pick the first AI suggestion without prompting")
}
