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
	Long: ` ████████╗ █████╗ ███████╗██╗  ██╗██╗    ██╗██╗███╗   ██╗ ██████╗ 
 ╚══██╔══╝██╔══██╗██╔════╝██║ ██╔╝██║    ██║██║████╗  ██║██╔════╝ 
    ██║   ███████║███████╗█████╔╝ ██║ █╗ ██║██║██╔██╗ ██║██║  ███╗
    ██║   ██╔══██║╚════██║██╔═██╗ ██║███╗██║██║██║╚██╗██║██║   ██║
    ██║   ██║  ██║███████║██║  ██╗╚███╔███╔╝██║██║ ╚████║╚██████╔╝
    ╚═╝   ╚═╝  ╚═╝╚══════╝╚═╝  ╚═╝ ╚══╝╚══╝ ╚═╝╚═╝  ╚═══╝ ╚═════╝ 

TaskWing CLI is a comprehensive tool to manage your tasks from the command line.
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

// Command categories for organized help display
var commandCategories = map[string][]string{
	"Getting Started":      {"quickstart", "interactive"},
	"Core Tasks":           {"add", "list", "show", "update", "delete"},
	"Workflow":             {"start", "review", "done", "current"},
	"Discovery & Planning": {"search", "next", "expand", "clear"},
	"Project Setup":        {"init", "reset", "config"},
	"System & Utilities":   {"mcp", "generate", "completion", "version", "help"},
}

// getGroupedHelpTemplate returns a custom help template with grouped commands
func getGroupedHelpTemplate() string {
	return `{{.Long | trimTrailingWhitespaces}}

Usage:{{if .Runnable}}
  {{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}
  {{.CommandPath}} [command]{{end}}

Common Workflows:
  {{.CommandPath}} quickstart                    # Interactive getting started guide
  {{.CommandPath}} add "Fix login bug"           # Create a new task  
  {{.CommandPath}} ls                            # List all tasks
  {{.CommandPath}} start <task-id>               # Begin working on a task
  {{.CommandPath}} done <task-id>                # Mark task complete
  {{.CommandPath}} add "Task" && {{.CommandPath}} start $({{.CommandPath}} ls --format=id --status=todo | head -1)  # Add and start

Available Commands:{{range $category, $commands := getCommandsByCategory .}}
{{$category}}:{{range $commands}}{{if .IsAvailableCommand}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}
{{end}}
{{if .HasAvailableLocalFlags}}Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}

Global Flags:
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasHelpSubCommands}}

Additional help topics:{{range .Commands}}{{if .IsAdditionalHelpTopicCommand}}
  {{rpad .CommandPath .CommandPathPadding}} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableSubCommands}}

Use "{{.CommandPath}} [command] --help" for more information about a command.{{end}}
`
}

// getCommandsByCategory organizes commands into categories for the help template
func getCommandsByCategory(cmd *cobra.Command) map[string][]*cobra.Command {
	result := make(map[string][]*cobra.Command)

	// Create a map of command names to commands for quick lookup
	cmdMap := make(map[string]*cobra.Command)
	for _, subCmd := range cmd.Commands() {
		cmdMap[subCmd.Name()] = subCmd
	}

	// Organize commands by category
	for category, cmdNames := range commandCategories {
		for _, cmdName := range cmdNames {
			if subCmd, exists := cmdMap[cmdName]; exists && subCmd.IsAvailableCommand() {
				result[category] = append(result[category], subCmd)
			}
		}
	}

	return result
}

func init() {
	cobra.OnInitialize(InitConfig)

	// Register custom template function
	cobra.AddTemplateFunc("getCommandsByCategory", getCommandsByCategory)

	// Set custom help template with grouped commands
	rootCmd.SetHelpTemplate(getGroupedHelpTemplate())

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
