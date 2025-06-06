/*
Copyright © 2025 NAME HERE josephgoksu@gmail.com
*/
package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
		relativeTasksDir := cfg.Project.TasksDir
		absoluteTasksDir := filepath.Join(projectRootDir, relativeTasksDir)

		// Create the project root directory (e.g., .taskwing)
		if err := os.MkdirAll(projectRootDir, 0755); err != nil {
			HandleError(fmt.Sprintf("Error: Could not create project directory '%s'.", projectRootDir), err)
		}

		// Create the tasks directory within the project root (e.g., .taskwing/tasks)
		if err := os.MkdirAll(absoluteTasksDir, 0755); err != nil {
			HandleError(fmt.Sprintf("Error: Could not create tasks directory '%s'.", absoluteTasksDir), err)
		}

		// Construct the expected full path to the task data file for messaging.
		taskFileName := viper.GetString("data.file")
		taskFileFormat := viper.GetString("data.format")
		ext := filepath.Ext(taskFileName)
		desiredExt := "." + taskFileFormat
		if taskFileFormat != "" && ext != desiredExt {
			baseName := strings.TrimSuffix(taskFileName, ext)
			taskFileName = baseName + desiredExt
		}
		if taskFileName == "" {
			taskFileName = "tasks." + taskFileFormat // Fallback, though viper should provide default
		}
		fullTaskDataPath := filepath.Join(absoluteTasksDir, taskFileName)

		// Attempt to get the store, which will initialize the data file if it doesn't exist.
		_, err := getStore()
		if err != nil {
			HandleError(fmt.Sprintf("Error: Could not initialize task store at '%s'.", fullTaskDataPath), err)
		}

		// Create default config file if it doesn't exist
		defaultConfigFileName := fmt.Sprintf("%s.yaml", projectConfigName)
		defaultConfigFilePath := filepath.Join(projectRootDir, defaultConfigFileName)
		configCreated := false
		configExisted := false

		if _, statErr := os.Stat(defaultConfigFilePath); os.IsNotExist(statErr) {
			fmt.Printf("Creating default configuration file: %s\n", defaultConfigFilePath)

			projectTasksDir := viper.GetString("project.tasksDir")
			projectTemplatesDir := viper.GetString("project.templatesDir")
			projectOutputLogPath := viper.GetString("project.outputLogPath") // Relative default like "logs/taskwing.log"
			dataFile := viper.GetString("data.file")
			dataFormat := viper.GetString("data.format")
			greeting := viper.GetString("greeting")

			llmProvider := viper.GetString("llm.provider")
			llmModelName := viper.GetString("llm.modelName")
			// llmApiKey is sensitive, so we don't write a default value to the file.
			// We will however show where to set it (ENV var).
			llmProjectId := viper.GetString("llm.projectId")
			llmMaxOutputTokens := viper.GetInt("llm.maxOutputTokens")
			llmTemperature := viper.GetFloat64("llm.temperature")

			// verbose is a flag, not typically a persisted project config default unless explicitly set by user in file
			// For initial file, we comment it out or show the current flag's effect.
			verboseCurrent := viper.GetBool("verbose")

			configContentStr := fmt.Sprintf(
				`# TaskWing Project-Specific Configuration
# File: %s
# This file allows you to override default TaskWing settings for this project.

project:
  # rootDir: "%s"
  #   Description: The root directory for all project-specific files (e.g., tasks, templates, logs).
  #   Default: ".taskwing" (This is typically the directory containing this config file).
  #   Note: If you move this config file, ensure TaskWing can still find the project root.

  tasksDir: "%s"
  #   Description: Directory for task data files, relative to project.rootDir.
  #   Default: "tasks" (e.g., %s)

  templatesDir: "%s"
  #   Description: Directory for templates, relative to project.rootDir.
  #   Default: "templates" (e.g., %s)

  outputLogPath: "%s"
  #   Description: Path for output logs, relative to project.rootDir.
  #   Default: "logs/taskwing.log" (e.g., %s)

data:
  file: "%s"
  #   Description: Name of the task data file.
  #   Default: "tasks.json"

  format: "%s"
  #   Description: Format of the task data file. Supported: json, yaml, toml.
  #   Default: "json"

# --- Optional configurations ---\n# Uncomment and customize as needed.\n# TaskWing uses built-in defaults if these are not specified.

# --- LLM Configuration for Task Generation ---
# Uncomment and configure to use the 'taskwing generate tasks' command.
# llm:
#   provider: "%s"  # Supported: "openai", "google"
#   modelName: "%s" # e.g., "gpt-4o-mini", "gpt-4o" for OpenAI; "gemini-1.5-pro-latest" for Google
#   # apiKey: "YOUR_API_KEY"
#   #   It's STRONGLY recommended to set this via an environment variable:
#   #   - For OpenAI: TASKWING_LLM_APIKEY or OPENAI_API_KEY
#   #   - For Google: TASKWING_LLM_APIKEY or GOOGLE_API_KEY (or use application default credentials)
#   # projectId: "%s" # Required for Google Cloud provider, if llm.provider is "google".
#   # maxOutputTokens: %d
#   # temperature: %.1f

# greeting: "%s"

# verbose: %t # Overrides the --verbose flag if set to true or false here.
              # Current effective verbose value (from flag/ENV): %t
`,
				filepath.ToSlash(defaultConfigFilePath),
				projectRootDir, // For project.rootDir comment

				projectTasksDir,
				filepath.ToSlash(filepath.Join(projectRootDir, projectTasksDir)),

				projectTemplatesDir,
				filepath.ToSlash(filepath.Join(projectRootDir, projectTemplatesDir)),

				projectOutputLogPath,
				filepath.ToSlash(filepath.Join(projectRootDir, projectOutputLogPath)),

				dataFile,
				dataFormat,

				llmProvider,
				llmModelName,
				llmProjectId, // Value will be empty if not set, appropriately commented
				llmMaxOutputTokens,
				llmTemperature,

				greeting,
				verboseCurrent, // For the # verbose: line
				verboseCurrent, // For the # Current effective ... line
			)

			if errWrite := os.WriteFile(defaultConfigFilePath, []byte(configContentStr), 0644); errWrite != nil {
				// This is a non-critical warning, not a fatal error.
				fmt.Fprintf(os.Stderr, "Warning: Failed to create default config file at %s: %v\n", defaultConfigFilePath, errWrite)
			} else {
				configCreated = true
			}
		} else if statErr == nil {
			configExisted = true
		} else {
			// This is a non-critical warning.
			fmt.Fprintf(os.Stderr, "Warning: Could not check for existing config file at %s: %v\n", defaultConfigFilePath, statErr)
		}

		fmt.Printf("TaskWing project initialized successfully.\n")
		fmt.Printf("Project root directory: %s\n", projectRootDir)
		fmt.Printf("Tasks directory: %s\n", absoluteTasksDir)
		fmt.Printf("Task data file will be managed at: %s\n", fullTaskDataPath)

		if configCreated {
			fmt.Printf("Default configuration file created at: %s\n", defaultConfigFilePath)
		} else if configExisted {
			fmt.Printf("Using existing configuration file: %s\n", defaultConfigFilePath)
		} else {
			// This case should ideally not be hit if stat error was handled, but as a fallback.
			fmt.Printf("No project-specific configuration file found at %s. TaskWing will use global defaults and environment variables.\n", defaultConfigFilePath)
			fmt.Printf("You can create %s to override defaults for this project.\n", defaultConfigFileName)
		}
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
