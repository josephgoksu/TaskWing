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

	"github.com/josephgoksu/taskwing.app/llm"
	"github.com/josephgoksu/taskwing.app/models"
	"github.com/josephgoksu/taskwing.app/prompts"
	"github.com/josephgoksu/taskwing.app/store"
	"github.com/josephgoksu/taskwing.app/types"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

// addCmd represents the add command
var addCmd = &cobra.Command{
	Use:   "add [task description]",
	Short: "Add a new task with AI enhancement",
	Long:  `Add a new task with AI-powered enhancement. Automatically improves title, description, acceptance criteria and priority. Use --no-ai to disable AI enhancement.`,
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
			enhanced, err := enhanceTaskWithAI(ctx, taskInput, taskStore)
			if err != nil {
				fmt.Printf("⚠️  AI enhancement failed: %v\n", err)
				fmt.Println("Creating task with basic details...")
				newTask = models.Task{
					Title:       taskInput,
					Description: taskInput,
					Status:      models.StatusTodo,
					Priority:    models.PriorityMedium,
				}
			} else {
				newTask = models.Task{
					Title:              enhanced.Title,
					Description:        enhanced.Description,
					AcceptanceCriteria: enhanced.AcceptanceCriteria,
					Status:             models.StatusTodo,
					Priority:           models.TaskPriority(enhanced.Priority),
				}
				fmt.Printf("🤖 AI enhanced task:\n")
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
	},
}

// enhanceTaskWithAI uses AI to improve a basic task input into a well-structured task.
func enhanceTaskWithAI(ctx context.Context, taskInput string, taskStore store.TaskStore) (types.EnhancedTask, error) {
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
		EstimationTemperature:      appCfg.LLM.EstimationTemperature,
		EstimationMaxOutputTokens:  appCfg.LLM.EstimationMaxOutputTokens,
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
	if resolvedLLMConfig.Provider == "" || resolvedLLMConfig.ModelName == "" {
		return types.EnhancedTask{}, fmt.Errorf("LLM provider or model not configured")
	}
	if resolvedLLMConfig.Provider == "openai" && resolvedLLMConfig.APIKey == "" {
		return types.EnhancedTask{}, fmt.Errorf("OpenAI API key not configured")
	}

	// Create LLM provider
	provider, err := llm.NewProvider(&resolvedLLMConfig)
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
		resolvedLLMConfig.ModelName,
		resolvedLLMConfig.APIKey,
		resolvedLLMConfig.ProjectID,
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
			if task.Status == models.StatusTodo {
				todoCount++
			} else if task.Status == models.StatusDoing {
				doingCount++
			}
		}
		context += fmt.Sprintf("- Project has %d todo tasks and %d in-progress tasks\n", todoCount, doingCount)
	}

	return context
}

func init() {
	rootCmd.AddCommand(addCmd)

	// AI-native flags
	addCmd.Flags().Bool("no-ai", false, "Disable AI enhancement (create basic task)")
	addCmd.Flags().String("title", "", "Task input (alternative to positional arguments)")
	addCmd.Flags().String("dependencies", "", "Comma-separated task IDs that this task depends on")
	addCmd.Flags().String("parentID", "", "ID of the parent task")
	addCmd.Flags().Bool("non-interactive", false, "Run in non-interactive mode (requires task input)")
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
