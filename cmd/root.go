/*
Copyright © 2025 NAME HERE josephgoksu@gmail.com
*/
package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/josephgoksu/taskwing.app/store"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	// cfgFile is the path to the configuration file.
	cfgFile string
	// verbose enables verbose output.
	verbose bool
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "taskwing",
	Short: "TaskWing CLI helps you manage your tasks efficiently.",
	Long: `TaskWing CLI is a comprehensive tool to manage your tasks from the command line.
It allows you to initialize a task repository, add, list, update, and delete tasks.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Add your logic here
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "config file (default is $HOME/.taskwing.yaml or ./.taskwing.yaml)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose output")

	// Bind persistent flags to Viper
	viper.BindPFlag("config", rootCmd.PersistentFlags().Lookup("config"))
	viper.BindPFlag("verbose", rootCmd.PersistentFlags().Lookup("verbose"))

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	// Example:
	// rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

// initConfig is defined in config.go

// getStore initializes and returns the task store.
func getStore() (store.TaskStore, error) {
	s := store.NewFileTaskStore()

	cfg := GetConfig() // Get the global AppConfig

	projectRoot := cfg.Project.RootDir       // e.g., ".taskwing"
	relativeTasksDir := cfg.Project.TasksDir // e.g., "tasks"

	// Construct the absolute path to the directory where tasks.json will reside
	absoluteTasksDir := filepath.Join(projectRoot, relativeTasksDir) // e.g., ".taskwing/tasks"

	taskFileName := viper.GetString("data.file")     // e.g., "tasks.json"
	taskFileFormat := viper.GetString("data.format") // e.g., "json"

	// Ensure taskFileName has the correct extension based on taskFileFormat
	ext := filepath.Ext(taskFileName)
	desiredExt := "." + taskFileFormat
	if taskFileFormat != "" && ext != desiredExt {
		baseName := strings.TrimSuffix(taskFileName, ext)
		taskFileName = baseName + desiredExt
	}
	if taskFileName == "" { // Should be caught by viper default, but as a safeguard
		taskFileName = "tasks." + taskFileFormat
	}

	// The final full path to the data file itself
	fullPath := filepath.Join(absoluteTasksDir, taskFileName) // e.g., ".taskwing/tasks/tasks.json"

	err := s.Initialize(map[string]string{
		"dataFile":       fullPath,
		"dataFileFormat": taskFileFormat,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize store at %s: %w", fullPath, err)
	}
	return s, nil
}
