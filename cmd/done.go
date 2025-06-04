package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/josephgoksu/taskwing.app/models"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

// doneCmd represents the done command
var doneCmd = &cobra.Command{
	Use:   "done [task_id]",
	Short: "Mark a task as done",
	Long:  `Mark a task as completed. If task_id is provided, it attempts to mark that task directly. Otherwise, it presents an interactive list to choose a task.`,
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		taskStore, err := getStore()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to get store: %v\\n", err)
			os.Exit(1)
		}
		defer taskStore.Close()

		var taskToMarkDone models.Task

		if len(args) > 0 {
			taskID := args[0]
			taskToMarkDone, err = taskStore.GetTask(taskID)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to get task %s: %v\\n", taskID, err)
				os.Exit(1)
			}
		} else {
			// Filter for tasks that are not yet completed
			notDoneFilter := func(t models.Task) bool {
				return t.Status != models.StatusCompleted
			}
			tasks, err := taskStore.ListTasks(notDoneFilter, nil)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to list tasks for selection: %v\\n", err)
				os.Exit(1)
			}
			if len(tasks) == 0 {
				fmt.Println("No active tasks available to mark as done.")
				return
			}

			templates := &promptui.SelectTemplates{
				Label:    "{{ . }}?",
				Active:   `> {{ .Title | cyan }} (ID: {{ .ID }}, Status: {{ .Status }})`,
				Inactive: `  {{ .Title | faint }} (ID: {{ .ID }}, Status: {{ .Status }})`,
				Selected: `{{ "✔" | green }} {{ .Title | faint }} (ID: {{ .ID }})`,
				Details: `
--------- Task Details ----------
{{ "ID:	" | faint }} {{ .ID }}
{{ "Title:	" | faint }} {{ .Title }}
{{ "Status:	" | faint }} {{ .Status }}
{{ "Priority:	" | faint }} {{ .Priority }}`,
			}

			searcher := func(input string, index int) bool {
				task := tasks[index]
				name := strings.ToLower(task.Title)
				input = strings.ToLower(input)
				return strings.Contains(name, input) || strings.Contains(task.ID, input)
			}

			prompt := promptui.Select{
				Label:     "Select task to mark as done",
				Items:     tasks,
				Templates: templates,
				Searcher:  searcher,
			}

			i, _, err := prompt.Run()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Task selection failed %v\\n", err)
				os.Exit(1)
			}
			taskToMarkDone = tasks[i]
		}

		if taskToMarkDone.Status == models.StatusCompleted {
			fmt.Printf("Task '%s' (ID: %s) is already completed.\\n", taskToMarkDone.Title, taskToMarkDone.ID)
			return
		}

		updatedTask, err := taskStore.MarkTaskDone(taskToMarkDone.ID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to mark task %s as done: %v\\n", taskToMarkDone.ID, err)
			os.Exit(1)
		}

		fmt.Printf("Task '%s' (ID: %s) marked as done successfully!\\n", updatedTask.Title, updatedTask.ID)
	},
}

func init() {
	rootCmd.AddCommand(doneCmd)
}
