package cmd

import (
	"fmt"

	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

// flowCmd provides guided workflows for common TaskWing operations
var flowCmd = &cobra.Command{
	Use:     "flow",
	Aliases: []string{"workflow", "guide"},
	Short:   "Interactive guided workflows for common tasks",
	Long:    `Provides step-by-step guided workflows to help you use TaskWing effectively.`,
	Example: `  # Start guided workflow
  taskwing flow
  
  # Quick aliases
  taskwing guide`,
	RunE: func(cmd *cobra.Command, args []string) error {
		workflows := []string{
			"🚀 Quick Start: Add and start a new task",
			"📋 Planning: Break down a complex task",
			"✅ Progress: Update and complete tasks",
			"🔍 Review: Check status and priorities",
			"🧹 Cleanup: Archive completed tasks",
			"⚡ Sprint: Plan a 6-day development cycle",
		}

		prompt := promptui.Select{
			Label: "Choose a workflow",
			Items: workflows,
		}

		idx, _, err := prompt.Run()
		if err != nil {
			if err == promptui.ErrInterrupt {
				fmt.Println("Workflow cancelled.")
				return nil
			}
			return err
		}

		switch idx {
		case 0: // Quick Start
			return quickStartFlow()
		case 1: // Planning
			return planningFlow()
		case 2: // Progress
			return progressFlow()
		case 3: // Review
			return reviewFlow()
		case 4: // Cleanup
			return cleanupFlow()
		case 5: // Sprint
			return sprintFlow()
		}

		return nil
	},
}

func quickStartFlow() error {
	fmt.Println("\n🚀 Quick Start Workflow")
	fmt.Println("========================")
	fmt.Println("Let's add a new task and start working on it immediately.")
	fmt.Println()

	// Step 1: Add task
	fmt.Println("Step 1: Creating your task...")
	prompt := promptui.Prompt{
		Label: "What do you want to work on?",
		Validate: func(input string) error {
			if len(input) < 3 {
				return fmt.Errorf("task must be at least 3 characters")
			}
			return nil
		},
	}

	taskDesc, err := prompt.Run()
	if err != nil {
		return err
	}

	// Add with AI and start
	fmt.Println("\n📝 Creating and enhancing task with AI...")
	if err := runCommand("add", []string{taskDesc, "--start"}); err != nil {
		return fmt.Errorf("failed to add task: %w", err)
	}

	fmt.Println("\n✨ Great! You're now working on your task.")
	fmt.Println("When you're done, run: taskwing done")

	return nil
}

func planningFlow() error {
	fmt.Println("\n📋 Planning Workflow")
	fmt.Println("====================")
	fmt.Println("Let's break down a complex task into manageable steps.")
	fmt.Println()

	// Select or create parent task
	confirmPrompt := promptui.Select{
		Label: "Do you have an existing task to plan, or create a new one?",
		Items: []string{"Select existing task", "Create new task"},
	}

	choice, _, err := confirmPrompt.Run()
	if err != nil {
		return err
	}

	if choice == 1 {
		// Create new task first
		prompt := promptui.Prompt{
			Label: "Describe the complex task",
		}
		taskDesc, err := prompt.Run()
		if err != nil {
			return err
		}

		fmt.Println("\n📝 Creating task...")
		if err := runCommand("add", []string{taskDesc, "--no-ai"}); err != nil {
			return fmt.Errorf("failed to add task: %w", err)
		}
	}

	// Generate plan
	fmt.Println("\n🤖 Generating subtasks with AI...")
	if err := runCommand("plan", []string{}); err != nil {
		return fmt.Errorf("failed to plan: %w", err)
	}

	fmt.Println("\n✨ Planning complete! Use 'taskwing ls' to see your subtasks.")

	return nil
}

func progressFlow() error {
	fmt.Println("\n✅ Progress Workflow")
	fmt.Println("=====================")
	fmt.Println("Let's update your task progress.")
	fmt.Println()

	actions := []string{
		"Start a new task",
		"Mark task as done",
		"Review task (move to review)",
		"Update task details",
		"View current task",
	}

	prompt := promptui.Select{
		Label: "What would you like to do?",
		Items: actions,
	}

	idx, _, err := prompt.Run()
	if err != nil {
		return err
	}

	switch idx {
	case 0:
		return runCommand("start", []string{})
	case 1:
		return runCommand("done", []string{})
	case 2:
		return runCommand("review", []string{})
	case 3:
		return runCommand("update", []string{})
	case 4:
		return runCommand("current", []string{})
	}

	return nil
}

func reviewFlow() error {
	fmt.Println("\n🔍 Review Workflow")
	fmt.Println("===================")
	fmt.Println("Let's review your tasks and priorities.")
	fmt.Println()

	views := []string{
		"Show all todo tasks",
		"Show high priority tasks",
		"Show tasks in progress",
		"Show blocked tasks",
		"Show ready tasks (no blockers)",
		"Show everything",
	}

	prompt := promptui.Select{
		Label: "What would you like to see?",
		Items: views,
	}

	idx, _, err := prompt.Run()
	if err != nil {
		return err
	}

	switch idx {
	case 0:
		return runCommand("list", []string{"--status", "todo"})
	case 1:
		return runCommand("list", []string{"--priority", "high"})
	case 2:
		return runCommand("list", []string{"--status", "doing"})
	case 3:
		return runCommand("list", []string{"--blocked"})
	case 4:
		return runCommand("list", []string{"--ready"})
	case 5:
		return runCommand("list", []string{"--all"})
	}

	return nil
}

func cleanupFlow() error {
	fmt.Println("\n🧹 Cleanup Workflow")
	fmt.Println("====================")
	fmt.Println("Let's clean up completed tasks.")
	fmt.Println()

	// Show completed tasks first
	fmt.Println("Showing completed tasks...")
	_ = runCommand("list", []string{"--status", "done"})

	confirmPrompt := promptui.Select{
		Label: "\nWhat would you like to do?",
		Items: []string{
			"Clear all completed tasks",
			"Clear completed tasks older than 7 days",
			"Keep everything (cancel)",
		},
	}

	choice, _, err := confirmPrompt.Run()
	if err != nil {
		return err
	}

	switch choice {
	case 0:
		return runCommand("clear", []string{"--completed"})
	case 1:
		// This would need date filtering support
		fmt.Println("Date-based clearing coming soon!")
		return runCommand("clear", []string{"--completed"})
	case 2:
		fmt.Println("Cleanup cancelled.")
	}

	return nil
}

func sprintFlow() error {
	fmt.Println("\n⚡ Sprint Planning Workflow")
	fmt.Println("============================")
	fmt.Println("Let's plan your 6-day development sprint.")
	fmt.Println()

	// Step 1: Review current state
	fmt.Println("📊 Current Status:")
	_ = runCommand("list", []string{"--status", "todo", "--priority", "high"})

	// Step 2: Add sprint goal
	prompt := promptui.Prompt{
		Label: "\nWhat's the main goal for this sprint?",
	}

	goal, err := prompt.Run()
	if err != nil {
		return err
	}

	// Create sprint parent task
	fmt.Println("\n📝 Creating sprint goal...")
	if err := runCommand("add", []string{goal, "--priority", "high"}); err != nil {
		return err
	}

	// Generate sprint plan
	fmt.Println("\n🤖 Generating 6-day sprint plan...")
	if err := runCommand("plan", []string{"--count", "6", "--confirm"}); err != nil {
		return err
	}

	fmt.Println("\n✨ Sprint planned! Each subtask represents roughly one day of work.")
	fmt.Println("Start with: taskwing start")
	fmt.Println("Track progress: taskwing current")

	return nil
}

// runCommand is defined in quickstart.go and reused here

func init() {
	rootCmd.AddCommand(flowCmd)
}
