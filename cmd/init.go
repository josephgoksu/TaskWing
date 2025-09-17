/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	// projectConfigName is the base name of the config file (e.g., .taskwing)
	// It will be used to create .taskwing.yaml
	projectConfigName = ".taskwing"
)

// initCmd represents the init command
var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new TaskWing project with optional AI setup",
	Long: `Initialize a complete TaskWing project in the current directory.
This sets up all necessary project structure and optionally configures AI features.

The init command:
• Creates the project directory structure (.taskwing/, tasks/, etc.)
• Generates a configuration file with sensible defaults
• Optionally walks you through AI setup (models, API keys)
• Gets you ready to start managing tasks immediately

Perfect for getting started - everything you need in one command!`,
	Run: func(cmd *cobra.Command, args []string) {
		cfg := GetConfig() // Get the application configuration

		projectRootDir := cfg.Project.RootDir
		projectTasksDir := filepath.Join(projectRootDir, cfg.Project.TasksDir)

		// Create the project root and tasks directories
		if err := os.MkdirAll(projectTasksDir, 0o755); err != nil {
			HandleFatalError(fmt.Sprintf("Error: Could not create project directories at '%s'.", projectTasksDir), err)
		}

		// Attempt to get the store, which will initialize the data file if it doesn't exist.
		store, err := GetStore()
		if err != nil {
			HandleFatalError("Error: Could not initialize task store.", err)
		}
		if err := store.Close(); err != nil {
			// Log the error but continue with initialization
			fmt.Fprintf(os.Stderr, "Warning: Failed to close task store: %v\n", err)
		}

		// Create default config file if it doesn't exist inside the project root dir
		projectConfigFilePath := filepath.Join(projectRootDir, fmt.Sprintf("%s.yaml", projectConfigName))
		configCreated := false
		configExisted := false

		if _, statErr := os.Stat(projectConfigFilePath); os.IsNotExist(statErr) {
			fmt.Printf("Creating default configuration file: %s\n", projectConfigFilePath)

			// Use viper to get the default values as strings/ints/etc.
			defaultConfigContent := fmt.Sprintf(
				`# TaskWing Project-Specific Configuration
# File: %s
# This file allows you to override default TaskWing settings for this project.

project:
  rootDir: "%s"
  tasksDir: "%s"
  templatesDir: "%s"
  outputLogPath: "%s"

data:
  file: "%s"
  format: "%s"

# --- Optional configurations ---
# Uncomment and customize as needed.

# --- LLM Configuration for 'taskwing plan' and 'taskwing iterate' ---
# llm:
#   provider: "%s"
#   modelName: "%s"
#   # It's STRONGLY recommended to set API keys via an environment variable:
#   # - For OpenAI: TASKWING_LLM_APIKEY or OPENAI_API_KEY
#   # - For Google: TASKWING_LLM_APIKEY or GOOGLE_API_KEY
#   apiKey: ""
#   # Required for Google Cloud provider if not using Application Default Credentials
#   projectId: "%s"
#   maxOutputTokens: %d
#   temperature: %.1f
#   improvementTemperature: %.1f
#   improvementMaxOutputTokens: %d

# verbose: %t
`,
				filepath.ToSlash(projectConfigFilePath),
				viper.GetString("project.rootDir"),
				viper.GetString("project.tasksDir"),
				viper.GetString("project.templatesDir"),
				viper.GetString("project.outputLogPath"),
				viper.GetString("data.file"),
				viper.GetString("data.format"),
				viper.GetString("llm.provider"),
				viper.GetString("llm.modelName"),
				viper.GetString("llm.projectId"),
				viper.GetInt("llm.maxOutputTokens"),
				viper.GetFloat64("llm.temperature"),
				viper.GetFloat64("llm.improvementTemperature"),
				viper.GetInt("llm.improvementMaxOutputTokens"),
				viper.GetBool("verbose"),
			)

			// Write the config file
			err = os.WriteFile(projectConfigFilePath, []byte(defaultConfigContent), 0o644)
			if err != nil {
				HandleFatalError(fmt.Sprintf("Error: Could not write configuration file at '%s'.", projectConfigFilePath), err)
			}
			configCreated = true
		} else {
			configExisted = true
		}

		// Summary
		fmt.Printf("TaskWing has been initialized in the current directory.\n")
		fmt.Printf("Project root directory: %s\n", projectRootDir)
		fmt.Printf("Tasks directory: %s\n", projectTasksDir)

		if configCreated {
			fmt.Printf("Configuration file created: %s\n", projectConfigFilePath)
		} else if configExisted {
			fmt.Printf("Configuration file already exists: %s\n", projectConfigFilePath)
		}

		// Step: Optional AI Setup
		fmt.Println("\n🤖 AI Setup (Optional)")
		fmt.Println("─────────────────────")
		fmt.Println("TaskWing includes AI-powered features:")
		fmt.Println("  • AI-enhanced task creation and management")
		fmt.Println("  • Generate tasks from documents (PRDs, specs, etc.)")
		fmt.Println("  • Intelligent task suggestions and planning")

		// Ask if user wants to configure AI now
		aiSetup, err := promptForAISetup()
		if err != nil {
			// If there's an error with the prompt, just continue without AI setup
			fmt.Println("\n⚠️  Skipping AI setup. You can configure it later if needed.")
		} else if aiSetup {
			// Run AI setup inline
			fmt.Println("\n🚀 Setting up AI features...")
			err = runAISetup(projectConfigFilePath)
			if err != nil {
				fmt.Printf("\n⚠️  AI setup failed: %v\n", err)
				fmt.Println("Don't worry - you can try again later or configure manually.")
			}
		}

		fmt.Println("\n🎉 TaskWing initialization complete!")
		fmt.Println("──────────────────────────────────────────")
		fmt.Println("\n📝 Ready to go:")
		fmt.Println("  • Create your first task: taskwing add \"Your task description\"")
		fmt.Println("  • Get started interactively: taskwing quickstart")
		fmt.Println("  • Explore all features: taskwing interactive")

		fmt.Println("\n💡 Pro tip: AI features are completely optional - TaskWing works great without them!")

		// Show different message based on whether AI was configured
		if aiSetup {
			fmt.Println("    Your AI features are ready to use!")
		} else {
			fmt.Println("    Run 'taskwing init' again to configure AI anytime!")
		}
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}

// promptForAISetup asks user if they want to set up AI features during init
func promptForAISetup() (bool, error) {
	prompt := promptui.Prompt{
		Label:     "Would you like to set up AI features now? (y/n)",
		IsConfirm: true,
		Default:   "y",
	}

	result, err := prompt.Run()
	if err != nil {
		if err == promptui.ErrAbort {
			return false, nil
		}
		return false, err
	}

	return strings.ToLower(result) == "y" || strings.ToLower(result) == "yes" || result == "", nil
}

// runAISetup performs the AI setup inline during init
func runAISetup(configPath string) error {
	// Step 1: Select Model (simplified for init - just pick best defaults)
	modelPrompt := promptui.Select{
		Label: "Select AI model",
		Items: []string{
			"gpt-5-mini (Recommended - Fast & efficient)",
			"gpt-5-nano (Ultra-fast)",
			"gpt-5 (Most capable)",
			"Skip AI setup",
		},
	}

	modelIdx, _, err := modelPrompt.Run()
	if err != nil {
		return fmt.Errorf("model selection failed: %w", err)
	}

	if modelIdx == 3 {
		return nil // Skip setup
	}

	var modelName string
	switch modelIdx {
	case 0:
		modelName = "gpt-5-mini"
	case 1:
		modelName = "gpt-5-nano"
	case 2:
		modelName = "gpt-5"
	}

	// Step 2: API Key Setup (simplified)
	apiKeyPrompt := promptui.Select{
		Label: "API Key setup",
		Items: []string{
			"I'll set OPENAI_API_KEY environment variable (Recommended)",
			"Enter API key now (saved to config file)",
			"Skip - configure later",
		},
	}

	keyIdx, _, err := apiKeyPrompt.Run()
	if err != nil {
		return fmt.Errorf("API key setup failed: %w", err)
	}

	var apiKey string
	if keyIdx == 1 {
		// Enter API key now
		keyPrompt := promptui.Prompt{
			Label: "Enter your OpenAI API key",
			Mask:  '*',
			Validate: func(input string) error {
				if len(input) < 20 {
					return fmt.Errorf("API key seems too short")
				}
				return nil
			},
		}
		apiKey, err = keyPrompt.Run()
		if err != nil {
			return fmt.Errorf("API key input failed: %w", err)
		}
	}

	// Step 3: Update config file
	err = updateLLMConfigInFile(configPath, modelName, apiKey)
	if err != nil {
		return fmt.Errorf("failed to update config: %w", err)
	}

	// Step 4: Show success message
	fmt.Printf("✅ AI configured: %s\n", modelName)
	switch keyIdx {
	case 0:
		fmt.Println("💡 Remember to set OPENAI_API_KEY environment variable")
	case 1:
		fmt.Println("🔑 API key saved to config file")
	}

	return nil
}

// updateLLMConfigInFile updates the LLM configuration in the YAML file by uncommenting and updating values
func updateLLMConfigInFile(configPath, modelName, apiKey string) error {
	content, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	lines := strings.Split(string(content), "\n")
	var newLines []string
	inLLMSection := false
	llmSectionFound := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Check if we're entering LLM section
		if strings.HasPrefix(trimmed, "# llm:") || strings.HasPrefix(trimmed, "llm:") {
			llmSectionFound = true
			inLLMSection = true
			// Add the uncommented llm: line
			newLines = append(newLines, "llm:")
			continue
		}

		// If we're in the LLM section, replace the entire section with clean values
		if inLLMSection {
			// Check if this line is still part of LLM section
			if (strings.HasPrefix(line, "#   ") || strings.HasPrefix(line, "   ") ||
				strings.HasPrefix(line, "  ") || strings.HasPrefix(line, "    ")) &&
				(strings.Contains(trimmed, ":") || strings.HasPrefix(trimmed, "#")) {

				// Skip all existing LLM config lines - we'll replace them
				continue
			} else if trimmed != "" && !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "#") {
				// We've left the LLM section, add our clean config now
				newLines = append(newLines, "  provider: \"openai\"")
				newLines = append(newLines, fmt.Sprintf("  modelName: \"%s\"", modelName))
				if apiKey != "" {
					newLines = append(newLines, fmt.Sprintf("  apiKey: \"%s\"", apiKey))
				}
				newLines = append(newLines, "  maxOutputTokens: 0")
				newLines = append(newLines, "  temperature: 0.7")
				newLines = append(newLines, "  improvementTemperature: 0.3")
				newLines = append(newLines, "  improvementMaxOutputTokens: 0")
				newLines = append(newLines, "")
				inLLMSection = false
			} else {
				// Skip empty lines or comments within LLM section
				continue
			}
		}

		newLines = append(newLines, line)
	}

	// If we ended while still in LLM section, close it out
	if inLLMSection {
		newLines = append(newLines, "  provider: \"openai\"")
		newLines = append(newLines, fmt.Sprintf("  modelName: \"%s\"", modelName))
		if apiKey != "" {
			newLines = append(newLines, fmt.Sprintf("  apiKey: \"%s\"", apiKey))
		}
		newLines = append(newLines, "  maxOutputTokens: 0")
		newLines = append(newLines, "  temperature: 0.7")
		newLines = append(newLines, "  improvementTemperature: 0.3")
		newLines = append(newLines, "  improvementMaxOutputTokens: 0")
	}

	// If no LLM section was found, add it
	if !llmSectionFound {
		newLines = append(newLines, "")
		newLines = append(newLines, "llm:")
		newLines = append(newLines, "  provider: \"openai\"")
		newLines = append(newLines, fmt.Sprintf("  modelName: \"%s\"", modelName))
		if apiKey != "" {
			newLines = append(newLines, fmt.Sprintf("  apiKey: \"%s\"", apiKey))
		}
		newLines = append(newLines, "  maxOutputTokens: 0")
		newLines = append(newLines, "  temperature: 0.7")
		newLines = append(newLines, "  improvementTemperature: 0.3")
		newLines = append(newLines, "  improvementMaxOutputTokens: 0")
	}

	// Write the updated content back to the file
	return os.WriteFile(configPath, []byte(strings.Join(newLines, "\n")), 0o644)
}
