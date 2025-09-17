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

// quickstartCmd represents the quickstart command
var quickstartCmd = &cobra.Command{
	Use:   "quickstart",
	Short: "Interactive guide for getting started with TaskWing",
	Long: `Quickstart provides an interactive guide to help you get started with TaskWing.
This command will walk you through:
- Setting up your first TaskWing project
- Creating your first task
- Starting work on a task
- Marking tasks as complete

Perfect for first-time users who want to learn the basic workflow.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("🚀 Welcome to TaskWing!")
		fmt.Println("Let's get you started with a quick interactive tour.")
		fmt.Println()

		// Step 1: Check if project is initialized
		if !isProjectInitialized() {
			if !confirmStep("Would you like to initialize a TaskWing project in the current directory?") {
				fmt.Println("👋 No problem! Run 'taskwing init' when you're ready to get started.")
				return
			}

			fmt.Println("📁 Initializing TaskWing project...")
			if err := runCommand("init", []string{}); err != nil {
				HandleFatalError("Failed to initialize project", err)
				return
			}
			fmt.Println("✅ Project initialized!")
			fmt.Println()
		} else {
			fmt.Println("✅ TaskWing project already initialized.")
			fmt.Println()
		}

		// Step 2: Create first task
		if !confirmStep("Would you like to create your first task?") {
			showNextSteps("create")
			return
		}

		taskTitle := promptForInput("What task would you like to work on?", "Fix the login bug")
		fmt.Printf("📝 Creating task: %s\n", taskTitle)

		if err := runCommand("add", []string{taskTitle, "--no-ai"}); err != nil {
			HandleFatalError("Failed to create task", err)
			return
		}
		fmt.Println("✅ Task created!")
		fmt.Println()

		// Step 3: Show the task list
		fmt.Println("📋 Here are your current tasks:")
		if err := runCommand("list", []string{}); err != nil {
			fmt.Println("⚠️  Could not display tasks")
		}
		fmt.Println()

		// Step 4: Start working on the task
		if !confirmStep("Would you like to start working on this task?") {
			showNextSteps("start")
			return
		}

		// Get the most recently created task
		taskStore, err := GetStore()
		if err != nil {
			HandleFatalError("Could not access tasks", err)
			return
		}
		defer func() {
			if err := taskStore.Close(); err != nil {
				HandleFatalError("Failed to close task store", err)
			}
		}()

		tasks, err := taskStore.ListTasks(func(t models.Task) bool {
			return t.Status == models.StatusTodo
		}, nil)
		if err != nil || len(tasks) == 0 {
			fmt.Println("⚠️  No tasks found to start")
			showNextSteps("general")
			return
		}

		// Start the first todo task
		firstTask := tasks[0]
		fmt.Printf("🏃 Starting work on: %s\n", firstTask.Title)
		if err := runCommand("start", []string{firstTask.ID[:8]}); err != nil {
			HandleFatalError("Failed to start task", err)
			return
		}
		fmt.Println("✅ Task started! Status changed to 'doing'")
		fmt.Println()

		// Step 5: Complete the workflow
		if !confirmStep("Ready to mark this task as complete? (This is just for demo purposes)") {
			showNextSteps("finish")
			return
		}

		fmt.Printf("✅ Marking task as done: %s\n", firstTask.Title)
		if err := runCommand("done", []string{firstTask.ID[:8]}); err != nil {
			HandleFatalError("Failed to complete task", err)
			return
		}
		fmt.Println("🎉 Congratulations! You've completed your first TaskWing workflow!")
		fmt.Println()

		// Final guidance
		showCompletionMessage()
	},
}

func init() {
	rootCmd.AddCommand(quickstartCmd)
}

// isProjectInitialized checks if a TaskWing project exists in the current directory
func isProjectInitialized() bool {
	config := GetConfig()

	// Check if config exists and tasks file is accessible
	if config.Project.RootDir == "" {
		return false
	}

	// Try to initialize store to see if project is properly set up
	store, err := GetStore()
	if err != nil {
		return false
	}
	defer func() {
		if err := store.Close(); err != nil {
			fmt.Printf("Warning: Failed to close task store: %v\n", err)
		}
	}()

	return true
}

// confirmStep prompts user for confirmation on each step
func confirmStep(message string) bool {
	prompt := promptui.Prompt{
		Label:     message + " (y/n)",
		IsConfirm: true,
		Default:   "y",
	}

	result, err := prompt.Run()
	if err != nil {
		if err == promptui.ErrAbort {
			return false
		}
		return false
	}

	return strings.ToLower(result) == "y" || strings.ToLower(result) == "yes" || result == ""
}

// promptForInput prompts user for text input with validation
func promptForInput(label, defaultValue string) string {
	prompt := promptui.Prompt{
		Label:   label,
		Default: defaultValue,
		Validate: func(input string) error {
			if len(strings.TrimSpace(input)) < 3 {
				return fmt.Errorf("input must be at least 3 characters long")
			}
			return nil
		},
	}

	result, err := prompt.Run()
	if err != nil {
		if err == promptui.ErrInterrupt {
			fmt.Println("\n👋 Quickstart cancelled.")
			os.Exit(0)
		}
		return defaultValue
	}

	return result
}

// runCommand executes a TaskWing command programmatically
func runCommand(cmdName string, args []string) error {
	// Create a new root command instance to avoid state pollution
	cmd := &cobra.Command{Use: "taskwing"}

	// Add all the same subcommands that rootCmd has
	for _, subCmd := range rootCmd.Commands() {
		cmd.AddCommand(subCmd)
	}

	// Construct the full command line
	fullArgs := append([]string{cmdName}, args...)

	// Set the arguments and execute
	cmd.SetArgs(fullArgs)

	return cmd.Execute()
}

// showNextSteps shows what the user should do next based on where they stopped
func showNextSteps(step string) {
	fmt.Println("📚 Next Steps:")

	switch step {
	case "create":
		fmt.Println("  • Create a task: taskwing add \"Your task description\"")
		fmt.Println("  • List tasks: taskwing list")
	case "start":
		fmt.Println("  • Start a task: taskwing start <task-id>")
		fmt.Println("  • View task details: taskwing show <task-id>")
	case "finish":
		fmt.Println("  • Mark task complete: taskwing done <task-id>")
		fmt.Println("  • Review workflow: taskwing current")
	default:
		fmt.Println("  • Get help: taskwing --help")
		fmt.Println("  • Create tasks: taskwing add \"Task description\"")
		fmt.Println("  • Manage workflow: taskwing start, taskwing done")
		fmt.Println("  • Discover features: taskwing search, taskwing next")
	}

	fmt.Println()
	fmt.Println("💡 Tip: Use 'taskwing [command] --help' for detailed help on any command.")
}

// showCompletionMessage displays the final success message
func showCompletionMessage() {
	fmt.Println("🎯 You've learned the core TaskWing workflow:")
	fmt.Println("   1. taskwing add     → Create tasks")
	fmt.Println("   2. taskwing start   → Begin work")
	fmt.Println("   3. taskwing done    → Complete tasks")
	fmt.Println()
	fmt.Println("🔍 Explore more features:")
	fmt.Println("   • taskwing search   → Find specific tasks")
	fmt.Println("   • taskwing next     → Get AI task suggestions")
	fmt.Println("   • taskwing current  → Manage active work")
	fmt.Println("   • taskwing plan     → Generate a concise plan (subtasks)")
	fmt.Println("   • taskwing iterate  → Refine or split a specific step")
	fmt.Println()
	fmt.Println("🤖 Pro tip: Try AI-enhanced task creation by running 'taskwing add' without --no-ai!")
	fmt.Println("Happy task managing! 🚀")
}
