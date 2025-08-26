/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/josephgoksu/TaskWing/models"
	"github.com/josephgoksu/TaskWing/store"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	// cfgFile is the path to the configuration file.
	cfgFile string
	// verbose enables verbose output.
	verbose bool
	// ErrNoTasksFound is returned when an interactive selection is attempted but no tasks are available.
	ErrNoTasksFound = errors.New("no tasks found matching your criteria")
	// version is the application version.
	version = "0.2.0"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "taskwing",
	Short: "TaskWing CLI helps you manage your tasks efficiently.",
	Long: `TaskWing CLI is a comprehensive tool to manage your tasks from the command line.
It allows you to initialize a task repository, add, list, update, and delete tasks.`,
	Run: func(cmd *cobra.Command, args []string) {
		// return help if no args are provided
		if len(args) == 0 {
			_ = cmd.Help()
			os.Exit(0)
		}

		// otherwise, run the subcommand
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
	cobra.OnInitialize(InitConfig)

	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "config file (default is $HOME/.taskwing.yaml or ./.taskwing.yaml)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose output")

	// Bind persistent flags to Viper
	_ = viper.BindPFlag("config", rootCmd.PersistentFlags().Lookup("config"))
	_ = viper.BindPFlag("verbose", rootCmd.PersistentFlags().Lookup("verbose"))

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	// Example:
	// rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

// initConfig is defined in config.go

// GetTaskFilePath returns the full path to the tasks file
func GetTaskFilePath() string {
	config := GetConfig()
	return filepath.Join(config.Project.RootDir, config.Project.TasksDir, config.Data.File)
}

// GetStore initializes and returns the task store using the unified types.AppConfig.
func GetStore() (store.TaskStore, error) {
	s := store.NewFileTaskStore()
	config := GetConfig()

	taskFilePath := GetTaskFilePath()

	err := s.Initialize(map[string]string{
		"dataFile":       taskFilePath,
		"dataFileFormat": config.Data.Format,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize store at %s: %w", taskFilePath, err)
	}
	return s, nil
}

// selectTaskInteractive presents a prompt to the user to select a task from a list.
// It can be filtered using the provided filter function.
func selectTaskInteractive(taskStore store.TaskStore, filterFn func(models.Task) bool, label string) (models.Task, error) {
	tasks, err := taskStore.ListTasks(filterFn, nil)
	if err != nil {
		return models.Task{}, fmt.Errorf("failed to list tasks for selection: %w", err)
	}

	if len(tasks) == 0 {
		return models.Task{}, ErrNoTasksFound
	}

	templates := &promptui.SelectTemplates{
		Label:    "{{ . }}?",
		Active:   `> {{ .Title | cyan }} (ID: {{ .ID }}, Status: {{ .Status }})`,
		Inactive: `  {{ .Title | faint }} (ID: {{ .ID }}, Status: {{ .Status }})`,
		Selected: `{{ "✔" | green }} {{ .Title | faint }} (ID: {{ .ID }})`,
		Details: `
--------- Task Details ----------
{{ "ID:\t" | faint }} {{ .ID }}
{{ "Title:\t" | faint }} {{ .Title }}
{{ "Description:\t" | faint }} {{ .Description }}
{{ "Status:\t" | faint }} {{ .Status }}
{{ "Priority:\t" | faint }} {{ .Priority }}`,
	}

	searcher := func(input string, index int) bool {
		task := tasks[index]
		name := strings.ToLower(task.Title)
		id := task.ID
		input = strings.ToLower(input)
		return strings.Contains(name, input) || strings.Contains(id, input)
	}

	prompt := promptui.Select{
		Label:     label,
		Items:     tasks,
		Templates: templates,
		Searcher:  searcher,
	}

	i, _, err := prompt.Run()
	if err != nil {
		return models.Task{}, err // Return error as is (includes promptui.ErrInterrupt)
	}

	return tasks[i], nil
}
