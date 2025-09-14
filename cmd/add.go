/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/josephgoksu/TaskWing/llm"
	"github.com/josephgoksu/TaskWing/models"
	"github.com/josephgoksu/TaskWing/prompts"
	"github.com/josephgoksu/TaskWing/store"
	"github.com/josephgoksu/TaskWing/types"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

// addCmd represents the add command
var addCmd = &cobra.Command{
	Use:     "add [task description]",
	Aliases: []string{"mk", "create", "new", "t"},
	Short:   "Add a new task with AI enhancement",
	Long:    `Add a new task with AI-powered enhancement. Automatically improves title, description, acceptance criteria and priority. Use --no-ai to disable AI enhancement.`,
	Example: `  # Quick add with AI enhancement
  taskwing add "Fix login bug"
  
  # Add and immediately start working
  taskwing add "Implement auth" --start
  
  # Add with specific priority
  taskwing add "Deploy to production" --priority urgent
  
  # Add without AI (faster)
  taskwing add "Simple task" --no-ai
  
  # Add and generate subtasks
  taskwing add "Build dashboard" --plan`,
	Run: func(cmd *cobra.Command, args []string) {
		ctx := context.Background()

		// Initialize task store
		taskStore, err := GetStore()
		if err != nil {
			HandleError("Error: Could not initialize the task store.", err)
		}
		defer func() {
			if err := taskStore.Close(); err != nil {
				HandleError("Failed to close task store", err)
			}
		}()

		// Check flags
		noAI, _ := cmd.Flags().GetBool("no-ai")
		nonInteractive, _ := cmd.Flags().GetBool("non-interactive")
		breakdown, _ := cmd.Flags().GetBool("breakdown")

		// Get task input from args, flag, or prompt
		var taskInput string
		if title, _ := cmd.Flags().GetString("title"); title != "" {
			taskInput = title
		} else if len(args) > 0 {
			taskInput = strings.Join(args, " ")
		} else {
			if nonInteractive {
				HandleError("Task input is required in non-interactive mode. Use positional arguments or --title flag.", nil)
				return
			}
			// Interactive prompt for task input
			inputPrompt := promptui.Prompt{
				Label: "Task description",
				Validate: func(input string) error {
					if len(strings.TrimSpace(input)) < 3 {
						return fmt.Errorf("task description must be at least 3 characters long")
					}
					return nil
				},
			}
			taskInput, err = inputPrompt.Run()
			if err != nil {
				if err == promptui.ErrInterrupt {
					fmt.Println("Task addition cancelled.")
					os.Exit(0)
				}
				HandleError("Error: Failed to read task input.", err)
			}
		}

		var newTask models.Task

		// AI Enhancement (default behavior)
		if !noAI {
			fmt.Print("🔄 Enhancing task with AI... ")
			enhanced, err := enhanceTaskWithAI(ctx, taskInput, taskStore)
			if err != nil {
				fmt.Printf("\r⚠️  AI enhancement failed: %v\n", err)
				fmt.Println("Creating task with basic details...")
				newTask = models.Task{
					Title:       taskInput,
					Description: taskInput,
					Status:      models.StatusTodo,
					Priority:    models.PriorityMedium,
				}
			} else {
				fmt.Print("\r") // Clear the loading message
				newTask = models.Task{
					Title:              enhanced.Title,
					Description:        enhanced.Description,
					AcceptanceCriteria: enhanced.AcceptanceCriteria,
					Status:             models.StatusTodo,
					Priority:           models.TaskPriority(enhanced.Priority),
				}
				fmt.Printf("✨ AI enhanced task:\n")
				fmt.Printf("   Title: %s\n", enhanced.Title)
				fmt.Printf("   Priority: %s\n", enhanced.Priority)
				if enhanced.AcceptanceCriteria != "" {
					fmt.Printf("   Acceptance Criteria:\n%s\n", enhanced.AcceptanceCriteria)
				}
			}
		} else {
			// Manual mode - use basic task creation
			newTask = models.Task{
				Title:       taskInput,
				Description: taskInput,
				Status:      models.StatusTodo,
				Priority:    models.PriorityMedium,
			}
		}

		// Override with manual flags if provided
		if priority, _ := cmd.Flags().GetString("priority"); priority != "" {
			switch strings.ToLower(priority) {
			case "low":
				newTask.Priority = models.PriorityLow
			case "medium":
				newTask.Priority = models.PriorityMedium
			case "high":
				newTask.Priority = models.PriorityHigh
			case "urgent":
				newTask.Priority = models.PriorityUrgent
			default:
				fmt.Printf("⚠️  Invalid priority '%s', using medium\n", priority)
				newTask.Priority = models.PriorityMedium
			}
		}

		if description, _ := cmd.Flags().GetString("description"); description != "" {
			newTask.Description = description
		}

		if acceptance, _ := cmd.Flags().GetString("acceptance"); acceptance != "" {
			newTask.AcceptanceCriteria = acceptance
		}

		// Handle dependencies and parent ID from flags if provided
		if dependenciesStr, _ := cmd.Flags().GetString("dependencies"); dependenciesStr != "" {
			dependencies := strings.Split(dependenciesStr, ",")
			for i, dep := range dependencies {
				dependencies[i] = strings.TrimSpace(dep)
			}
			newTask.Dependencies = dependencies
		}

		if parentIDStr, _ := cmd.Flags().GetString("parentID"); parentIDStr != "" {
			newTask.ParentID = &parentIDStr
		}

		// Create the task
		createdTask, err := taskStore.CreateTask(newTask)
		if err != nil {
			HandleError("Error: Could not create the new task.", err)
		}

		fmt.Printf("✅ Task added successfully!\n")
		fmt.Printf("ID: %s\n", createdTask.ID[:8]) // Show short ID

		// Smart Task Breakdown if requested and AI is enabled
		if breakdown && !noAI {
			subtasks, err := suggestSubtasks(ctx, createdTask, taskStore)
			if err != nil {
				fmt.Printf("⚠️  Failed to generate subtask suggestions: %v\n", err)
			} else if len(subtasks) > 0 {
				handleSubtaskSuggestions(subtasks, createdTask.ID, taskStore, nonInteractive)
			} else {
				fmt.Printf("💡 This task seems simple enough - no subtasks suggested.\n")
			}
		}

		// Smart Dependencies Detection if requested and AI is enabled
		detectDeps, _ := cmd.Flags().GetBool("detect-deps")
		if detectDeps && !noAI {
			dependencySuggestions, err := suggestDependencies(ctx, createdTask, taskStore)
			if err != nil {
				fmt.Printf("⚠️  Failed to analyze task dependencies: %v\n", err)
			} else if len(dependencySuggestions) > 0 {
				handleDependencySuggestions(dependencySuggestions, createdTask.ID, taskStore, nonInteractive)
			} else {
				fmt.Printf("💡 No dependency suggestions found - this task appears independent.\n")
			}
		}

		// Handle --start flag: start working on the task immediately
		startFlag, _ := cmd.Flags().GetBool("start")
		if startFlag {
			fmt.Printf("\n🚀 Starting task...\n")
			_ = runCommand("start", []string{createdTask.ID})
		}

		// Handle --plan flag: generate subtasks immediately
		planFlag, _ := cmd.Flags().GetBool("plan")
		if planFlag {
			fmt.Printf("\n📋 Generating plan...\n")
			_ = runCommand("plan", []string{"--task", createdTask.ID, "--confirm"})
		} else if !nonInteractive && !startFlag {
			// Offer immediate planning flow only if not already handled
			prompt := promptui.Prompt{Label: "Generate a plan now?", IsConfirm: true, Default: "y"}
			if _, perr := prompt.Run(); perr == nil {
				// Preview first
				fmt.Print("\n🔄 Generating plan preview... ")
				if err := runCommand("plan", []string{"--task", createdTask.ID}); err != nil {
					fmt.Printf("\r❌ Failed to generate plan: %v\n", err)
					if strings.Contains(err.Error(), "API key") {
						fmt.Println("💡 To use AI planning features, set up your LLM API key:")
						fmt.Println("   export OPENAI_API_KEY=your_key_here")
						fmt.Println("   Or run: taskwing init --setup-llm")
					}
				} else {
					// Ask to apply
					confirm := promptui.Prompt{Label: "Apply this plan now? (This will create the subtasks)", IsConfirm: true}
					if _, aerr := confirm.Run(); aerr == nil {
						fmt.Print("🔄 Creating subtasks... ")
						if err := runCommand("plan", []string{"--task", createdTask.ID, "--confirm"}); err != nil {
							fmt.Printf("\r❌ Failed to apply plan: %v\n", err)
						} else {
							fmt.Print("\r✅ Plan applied successfully!\n")
						}
					}
				}
			}
		}

		// Command discovery hints (only if not already handled by flags)
		if !startFlag && !planFlag {
			fmt.Printf("\n💡 What's next?\n")
			fmt.Printf("   • Plan work:     taskwing plan %s\n", createdTask.ID[:8])
			fmt.Printf("   • Start working: taskwing start %s\n", createdTask.ID[:8])
			fmt.Printf("   • View details:  taskwing show %s\n", createdTask.ID[:8])
			fmt.Printf("   • List all:      taskwing ls\n")
		}
	},
}

// enhanceTaskWithAI uses AI to improve a basic task input into a well-structured task.
func enhanceTaskWithAI(ctx context.Context, taskInput string, taskStore store.TaskStore) (types.EnhancedTask, error) {
	// Get app config
	appCfg := GetConfig()

	// Create LLM provider with environment variable resolution
	provider, err := createLLMProvider(&appCfg.LLM)
	if err != nil {
		return types.EnhancedTask{}, fmt.Errorf("failed to create LLM provider: %w", err)
	}

	// Get system prompt
	templatesDir := filepath.Join(appCfg.Project.RootDir, appCfg.Project.TemplatesDir)
	systemPrompt, err := prompts.GetPrompt(prompts.KeyEnhanceTask, templatesDir)
	if err != nil {
		return types.EnhancedTask{}, fmt.Errorf("failed to load enhancement prompt: %w", err)
	}

	// Build simple context info
	contextInfo := buildTaskContext(taskStore)

	// Call AI enhancement
	enhanced, err := provider.EnhanceTask(
		ctx,
		systemPrompt,
		taskInput,
		contextInfo,
		appCfg.LLM.ModelName,
		appCfg.LLM.APIKey,
		appCfg.LLM.ProjectID,
		1024, // Max tokens for single task enhancement
		0.3,  // Lower temperature for consistency
	)
	if err != nil {
		return types.EnhancedTask{}, fmt.Errorf("AI enhancement failed: %w", err)
	}

	return enhanced, nil
}

// buildTaskContext creates simple context information for AI task enhancement.
func buildTaskContext(taskStore store.TaskStore) string {
	context := "Project context:\n"

	// Get current task
	if currentTaskID := GetCurrentTask(); currentTaskID != "" {
		if currentTask, err := taskStore.GetTask(currentTaskID); err == nil {
			context += fmt.Sprintf("- Current task: %s\n", currentTask.Title)
		}
	}

	// Get recent tasks count
	if tasks, err := taskStore.ListTasks(nil, nil); err == nil {
		todoCount := 0
		doingCount := 0
		for _, task := range tasks {
			switch task.Status {
			case models.StatusTodo:
				todoCount++
			case models.StatusDoing:
				doingCount++
			}
		}
		context += fmt.Sprintf("- Project has %d todo tasks and %d in-progress tasks\n", todoCount, doingCount)
	}

	return context
}

// suggestSubtasks uses AI to analyze a task and suggest relevant subtasks
func suggestSubtasks(ctx context.Context, parentTask models.Task, taskStore store.TaskStore) ([]types.EnhancedTask, error) {
	// Get app config and prepare LLM config
	appCfg := GetConfig()

	// Resolve LLM configuration
	resolvedLLMConfig := types.LLMConfig{
		Provider:                   appCfg.LLM.Provider,
		ModelName:                  appCfg.LLM.ModelName,
		APIKey:                     appCfg.LLM.APIKey,
		ProjectID:                  appCfg.LLM.ProjectID,
		MaxOutputTokens:            appCfg.LLM.MaxOutputTokens,
		Temperature:                appCfg.LLM.Temperature,
		ImprovementTemperature:     appCfg.LLM.ImprovementTemperature,
		ImprovementMaxOutputTokens: appCfg.LLM.ImprovementMaxOutputTokens,
	}

	// Resolve API key from environment if needed
	if resolvedLLMConfig.APIKey == "" && resolvedLLMConfig.Provider == "openai" {
		if apiKeyEnv := os.Getenv("OPENAI_API_KEY"); apiKeyEnv != "" {
			resolvedLLMConfig.APIKey = apiKeyEnv
		} else if apiKeyEnv := os.Getenv(envPrefix + "_LLM_APIKEY"); apiKeyEnv != "" {
			resolvedLLMConfig.APIKey = apiKeyEnv
		}
	}

	// Validate LLM config
	if err := validateAndGuideLLMConfig(&resolvedLLMConfig); err != nil {
		return nil, err
	}

	// Create LLM provider
	provider, err := llm.NewProvider(&resolvedLLMConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create LLM provider: %w", err)
	}

	// Get system prompt for task breakdown
	templatesDir := filepath.Join(appCfg.Project.RootDir, appCfg.Project.TemplatesDir)
	systemPrompt, err := prompts.GetPrompt(prompts.KeyBreakdownTask, templatesDir)
	if err != nil {
		return nil, fmt.Errorf("failed to load breakdown prompt: %w", err)
	}

	// Build context for the parent task
	contextInfo := buildTaskBreakdownContext(parentTask, taskStore)

	// Call AI to suggest subtasks
	subtasks, err := provider.BreakdownTask(
		ctx,
		systemPrompt,
		parentTask.Title,
		parentTask.Description,
		parentTask.AcceptanceCriteria,
		contextInfo,
		resolvedLLMConfig.ModelName,
		resolvedLLMConfig.APIKey,
		resolvedLLMConfig.ProjectID,
		2048, // Max tokens for subtask generation
		0.7,  // Higher temperature for creativity
	)
	if err != nil {
		return nil, fmt.Errorf("AI subtask generation failed: %w", err)
	}

	return subtasks, nil
}

// buildTaskBreakdownContext creates context information for AI subtask generation
func buildTaskBreakdownContext(parentTask models.Task, taskStore store.TaskStore) string {
	context := "Parent Task Analysis:\n"
	context += fmt.Sprintf("Title: %s\n", parentTask.Title)
	context += fmt.Sprintf("Description: %s\n", parentTask.Description)
	context += fmt.Sprintf("Priority: %s\n", parentTask.Priority)
	if parentTask.AcceptanceCriteria != "" {
		context += fmt.Sprintf("Acceptance Criteria: %s\n", parentTask.AcceptanceCriteria)
	}

	// Add project context
	if tasks, err := taskStore.ListTasks(nil, nil); err == nil {
		context += fmt.Sprintf("\nProject Context: %d total tasks\n", len(tasks))

		// Find similar tasks for pattern recognition
		similarTasks := findSimilarTasks(parentTask, tasks)
		if len(similarTasks) > 0 {
			context += "Similar existing tasks:\n"
			for i, similar := range similarTasks {
				if i >= 3 { // Limit to top 3
					break
				}
				context += fmt.Sprintf("- %s\n", similar.Title)
			}
		}
	}

	return context
}

// findSimilarTasks finds tasks with similar titles or descriptions for pattern recognition
func findSimilarTasks(target models.Task, allTasks []models.Task) []models.Task {
	var similar []models.Task
	targetWords := strings.Fields(strings.ToLower(target.Title + " " + target.Description))

	for _, task := range allTasks {
		if task.ID == target.ID {
			continue // Skip the target task itself
		}

		taskWords := strings.Fields(strings.ToLower(task.Title + " " + task.Description))
		matchScore := calculateWordMatchScore(targetWords, taskWords)

		if matchScore > 0.3 { // 30% word overlap threshold
			similar = append(similar, task)
		}
	}

	return similar
}

// calculateWordMatchScore calculates similarity score between two word sets
func calculateWordMatchScore(words1, words2 []string) float64 {
	if len(words1) == 0 || len(words2) == 0 {
		return 0
	}

	matches := 0
	for _, w1 := range words1 {
		for _, w2 := range words2 {
			if w1 == w2 && len(w1) > 2 { // Only count meaningful words
				matches++
				break
			}
		}
	}

	return float64(matches) / float64(len(words1))
}

// handleSubtaskSuggestions presents subtask suggestions to the user and handles creation
func handleSubtaskSuggestions(subtasks []types.EnhancedTask, parentID string, taskStore store.TaskStore, nonInteractive bool) {
	fmt.Printf("\n🤖 AI suggested %d subtasks:\n\n", len(subtasks))

	// Display suggested subtasks
	for i, subtask := range subtasks {
		fmt.Printf("%d. 📝 %s\n", i+1, subtask.Title)
		if subtask.Description != "" && subtask.Description != subtask.Title {
			fmt.Printf("   %s\n", subtask.Description)
		}
		if subtask.Priority != "" {
			fmt.Printf("   Priority: %s\n", subtask.Priority)
		}
		fmt.Println()
	}

	if nonInteractive {
		fmt.Printf("💡 Use 'taskwing add --breakdown' in interactive mode to approve/reject these subtasks.\n")
		return
	}

	// Interactive approval
	prompt := promptui.Prompt{
		Label:     "Create these subtasks? (y/n)",
		Default:   "y",
		IsConfirm: true,
	}

	result, err := prompt.Run()
	if err != nil {
		if err == promptui.ErrAbort {
			fmt.Println("🚫 Subtask creation cancelled.")
			return
		}
		HandleError("Error reading confirmation", err)
		return
	}

	if strings.ToLower(result) == "y" {
		createSubtasks(subtasks, parentID, taskStore)
	} else {
		fmt.Println("🚫 Subtask creation cancelled.")
	}
}

// createSubtasks creates the approved subtasks with proper parent-child relationships
func createSubtasks(subtasks []types.EnhancedTask, parentID string, taskStore store.TaskStore) {
	createdCount := 0
	failedCount := 0

	fmt.Printf("\n🗺️ Creating %d subtasks...\n", len(subtasks))

	for i, subtask := range subtasks {
		newTask := models.Task{
			Title:              subtask.Title,
			Description:        subtask.Description,
			AcceptanceCriteria: subtask.AcceptanceCriteria,
			Status:             models.StatusTodo,
			Priority:           models.TaskPriority(subtask.Priority),
			ParentID:           &parentID,
		}

		// Set default priority if empty
		if newTask.Priority == "" {
			newTask.Priority = models.PriorityMedium
		}

		createdTask, err := taskStore.CreateTask(newTask)
		if err != nil {
			fmt.Printf("⚠️  Failed to create subtask %d: %v\n", i+1, err)
			failedCount++
		} else {
			fmt.Printf("✅ Created: %s (ID: %s)\n", createdTask.Title, createdTask.ID[:8])
			createdCount++
		}
	}

	fmt.Printf("\n🎉 Subtask creation complete: %d created, %d failed\n", createdCount, failedCount)

	if createdCount > 0 {
		fmt.Printf("💡 Use 'taskwing list --parent %s' to view all subtasks\n", parentID[:8])
	}
}

// suggestDependencies calls the AI provider to analyze dependencies for a task
func suggestDependencies(ctx context.Context, task models.Task, taskStore store.TaskStore) ([]types.DependencySuggestion, error) {
	config := GetConfig()

	// Create LLM provider
	provider, err := createLLMProvider(&config.LLM)
	if err != nil {
		return nil, fmt.Errorf("failed to create LLM provider: %w", err)
	}

	// Get system prompt
	systemPrompt, err := prompts.GetPrompt(prompts.KeyDetectDependencies, config.Project.TemplatesDir)
	if err != nil {
		return nil, fmt.Errorf("failed to get system prompt: %w", err)
	}

	// Get all tasks for context
	allTasks, err := taskStore.ListTasks(nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get tasks for context: %w", err)
	}

	// Build task info
	taskInfo := fmt.Sprintf("Task ID: %s\nTitle: %s\nDescription: %s\nAcceptance Criteria: %s\nPriority: %s",
		task.ID, task.Title, task.Description, task.AcceptanceCriteria, task.Priority)

	// Build context from all tasks
	contextInfo := buildDependencyContext(task, allTasks)

	// Call AI provider
	suggestions, err := provider.DetectDependencies(
		ctx,
		systemPrompt,
		taskInfo,
		contextInfo,
		config.LLM.ModelName,
		config.LLM.APIKey,
		config.LLM.ProjectID,
		config.LLM.MaxOutputTokens,
		config.LLM.Temperature,
	)
	if err != nil {
		return nil, fmt.Errorf("AI dependency detection failed: %w", err)
	}

	return suggestions, nil
}

// buildDependencyContext creates context information for dependency analysis
func buildDependencyContext(targetTask models.Task, allTasks []models.Task) string {
	var context strings.Builder

	context.WriteString("=== ALL PROJECT TASKS ===\n")
	for _, task := range allTasks {
		if task.ID == targetTask.ID {
			continue // Skip the target task
		}

		context.WriteString(fmt.Sprintf("Task ID: %s\n", task.ID))
		context.WriteString(fmt.Sprintf("Title: %s\n", task.Title))
		context.WriteString(fmt.Sprintf("Status: %s\n", task.Status))
		context.WriteString(fmt.Sprintf("Priority: %s\n", task.Priority))
		if task.Description != "" {
			context.WriteString(fmt.Sprintf("Description: %s\n", task.Description))
		}
		if task.AcceptanceCriteria != "" {
			context.WriteString(fmt.Sprintf("Acceptance Criteria: %s\n", task.AcceptanceCriteria))
		}

		// Show existing dependencies
		if len(task.Dependencies) > 0 {
			context.WriteString("Dependencies: ")
			for i, depID := range task.Dependencies {
				if i > 0 {
					context.WriteString(", ")
				}
				// Find dependency name
				for _, dep := range allTasks {
					if dep.ID == depID {
						context.WriteString(fmt.Sprintf("%s (%s)", dep.Title, depID))
						break
					}
				}
			}
			context.WriteString("\n")
		}

		context.WriteString("\n")
	}

	return context.String()
}

// handleDependencySuggestions presents dependency suggestions to the user
func handleDependencySuggestions(suggestions []types.DependencySuggestion, taskID string, taskStore store.TaskStore, nonInteractive bool) {
	fmt.Printf("\n🔗 AI suggested %d dependencies:\n\n", len(suggestions))

	// Display suggested dependencies
	for i, suggestion := range suggestions {
		// Get task names for display
		sourceTask, _ := taskStore.GetTask(suggestion.SourceTaskID)
		targetTask, _ := taskStore.GetTask(suggestion.TargetTaskID)

		fmt.Printf("%d. %s (%s)\n", i+1, suggestion.DependencyType, suggestion.Reasoning)
		fmt.Printf("   Task: %s\n", sourceTask.Title)
		fmt.Printf("   Depends on: %s\n", targetTask.Title)
		fmt.Printf("   Confidence: %.0f%%\n\n", suggestion.ConfidenceScore*100)
	}

	if nonInteractive {
		fmt.Printf("⚠️  Non-interactive mode: skipping dependency suggestions\n")
		return
	}

	// Ask user if they want to apply suggestions
	fmt.Printf("Apply these dependency suggestions? (y/N): ")
	var response string
	_, _ = fmt.Scanln(&response)

	if strings.ToLower(strings.TrimSpace(response)) == "y" {
		applied := 0
		for _, suggestion := range suggestions {
			if suggestion.SourceTaskID == taskID {
				// Add dependency to the current task
				task, err := taskStore.GetTask(taskID)
				if err != nil {
					fmt.Printf("⚠️  Failed to get task %s: %v\n", taskID, err)
					continue
				}

				// Check if dependency already exists
				alreadyExists := false
				for _, existingDep := range task.Dependencies {
					if existingDep == suggestion.TargetTaskID {
						alreadyExists = true
						break
					}
				}

				if !alreadyExists {
					updatedDependencies := append(task.Dependencies, suggestion.TargetTaskID)
					updates := map[string]interface{}{
						"dependencies": updatedDependencies,
					}

					_, err := taskStore.UpdateTask(taskID, updates)
					if err != nil {
						fmt.Printf("⚠️  Failed to add dependency: %v\n", err)
					} else {
						applied++
						fmt.Printf("✅ Added dependency: %s\n", suggestion.TargetTaskID[:8])
					}
				}
			}
		}

		if applied > 0 {
			fmt.Printf("\n🎉 Applied %d dependency suggestions!\n", applied)
		} else {
			fmt.Printf("\n💡 No dependencies were applied.\n")
		}
	}
}

func init() {
	rootCmd.AddCommand(addCmd)

	// AI-native flags
	addCmd.Flags().Bool("no-ai", false, "Disable AI enhancement (create basic task)")
	addCmd.Flags().Bool("breakdown", false, "AI suggests subtasks for complex tasks")
	addCmd.Flags().Bool("detect-deps", false, "AI analyzes and suggests task dependencies")
	addCmd.Flags().String("title", "", "Task input (alternative to positional arguments)")
	addCmd.Flags().String("dependencies", "", "Comma-separated task IDs that this task depends on")
	addCmd.Flags().String("parentID", "", "ID of the parent task")
	addCmd.Flags().Bool("non-interactive", false, "Run in non-interactive mode (requires task input)")

	// Developer-friendly flags
	addCmd.Flags().BoolP("start", "s", false, "Start working on the task immediately after creation")
	addCmd.Flags().BoolP("plan", "p", false, "Generate subtasks immediately after creation")
	addCmd.Flags().StringP("priority", "P", "", "Set priority (low/medium/high/urgent)")
	addCmd.Flags().StringP("description", "d", "", "Task description")
	addCmd.Flags().StringP("acceptance", "a", "", "Acceptance criteria")
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
