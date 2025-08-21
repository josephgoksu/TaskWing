/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

import (
	"fmt"
	"os"
	"path/filepath"

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
	Short: "Initializes a new TaskWing project or reinitializes an existing one.",
	Long: `The init command sets up the necessary TaskWing structures in the current directory.
This includes creating the project root directory (e.g., '.taskwing'),
the tasks directory within it (e.g., '.taskwing/tasks'),
and ensuring the task data file (e.g., 'tasks.json') can be initialized.`,
	Run: func(cmd *cobra.Command, args []string) {
		cfg := GetConfig() // Get the application configuration

		projectRootDir := cfg.Project.RootDir
		projectTasksDir := filepath.Join(projectRootDir, cfg.Project.TasksDir)

		// Create the project root and tasks directories
		if err := os.MkdirAll(projectTasksDir, 0755); err != nil {
			HandleError(fmt.Sprintf("Error: Could not create project directories at '%s'.", projectTasksDir), err)
		}

		// Attempt to get the store, which will initialize the data file if it doesn't exist.
		store, err := GetStore()
		if err != nil {
			HandleError("Error: Could not initialize task store.", err)
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

# --- LLM Configuration for 'taskwing generate tasks' ---
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
#   estimationTemperature: %.1f
#   estimationMaxOutputTokens: %d
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
				viper.GetFloat64("llm.estimationTemperature"),
				viper.GetInt("llm.estimationMaxOutputTokens"),
				viper.GetFloat64("llm.improvementTemperature"),
				viper.GetInt("llm.improvementMaxOutputTokens"),
				viper.GetBool("verbose"),
			)

			// Write the config file
			err = os.WriteFile(projectConfigFilePath, []byte(defaultConfigContent), 0644)
			if err != nil {
				HandleError(fmt.Sprintf("Error: Could not write configuration file at '%s'.", projectConfigFilePath), err)
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

		fmt.Println("You can now use 'taskwing add' to create your first task!")
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
