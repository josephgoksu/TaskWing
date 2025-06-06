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

// resetCmd represents the reset command
var resetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Resets the TaskWing project by removing its configuration and data.",
	Long: `Resets the current TaskWing project.
This command will remove the project's root directory (e.g., '.taskwing/'),
which includes all task data, logs, and any project-specific configuration files stored within it.
It allows you to start over by running 'taskwing init' again.
This command does NOT delete global configuration files (e.g., in your home directory)
or configuration files specified explicitly via the --config flag if they are outside
the project's root directory.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Ensure configuration is loaded to get project paths
		// initConfig() is called by cobra.OnInitialize in root.go, so GlobalAppConfig should be populated.
		cfg := GetConfig()
		projectRootDir := cfg.Project.RootDir

		// Check if the project root directory exists
		if _, err := os.Stat(projectRootDir); os.IsNotExist(err) {
			fmt.Printf("No TaskWing project found at '%s' to reset. Nothing to do.\n", projectRootDir)
			// If viper is using a config file, mention it.
			configFileUsed := viper.ConfigFileUsed()
			if configFileUsed != "" {
				fmt.Printf("Note: A configuration file was loaded from '%s'. This file was not touched.\n", configFileUsed)
			}
			return
		}

		fmt.Printf("The following directory and all its contents will be PERMANENTLY DELETED:\n")
		fmt.Printf("- %s\n\n", projectRootDir)
		fmt.Printf("This includes all tasks, logs, and project-specific configurations within this directory.\n")

		prompt := promptui.Prompt{
			Label:     fmt.Sprintf("Are you sure you want to reset the project by deleting '%s'?", projectRootDir),
			IsConfirm: true,
		}

		_, err := prompt.Run()
		if err != nil {
			// Handles 'no' (promptui.ErrAbort) and actual errors
			if err == promptui.ErrAbort {
				fmt.Println("Project reset cancelled.")
				os.Exit(0)
			}
			HandleError("Error: Could not get confirmation for project reset.", err)
			return // Unreachable, for clarity
		}

		fmt.Printf("Deleting project directory '%s'...\n", projectRootDir)
		err = os.RemoveAll(projectRootDir)
		if err != nil {
			HandleError(fmt.Sprintf("Error: Failed to delete project directory '%s'. Check permissions or delete it manually.", projectRootDir), err)
		}

		fmt.Printf("TaskWing project at '%s' has been reset successfully.\n", projectRootDir)
		fmt.Println("You can now run 'taskwing init' to start a new project.")

		// Advise about potentially loaded config file if it wasn't inside projectRootDir
		configFileUsed := viper.ConfigFileUsed()
		if configFileUsed != "" {
			absProjectRootDir, absErr := filepath.Abs(projectRootDir)
			if absErr == nil {
				absConfigFileUsed, absConfErr := filepath.Abs(configFileUsed)
				if absConfErr == nil {
					if !strings.HasPrefix(strings.ToLower(absConfigFileUsed), strings.ToLower(absProjectRootDir)) && absConfigFileUsed != absProjectRootDir {
						fmt.Printf("Note: The configuration file loaded from '%s' was not part of the deleted project directory and has not been removed.\n", configFileUsed)
					}
				}
			} else if !strings.Contains(strings.ToLower(configFileUsed), strings.ToLower(projectRootDir)) { // Fallback to simple string check if absolute path resolution fails
				fmt.Printf("Note: The configuration file loaded from '%s' may not have been part of the deleted project directory and might not have been removed.\n", configFileUsed)
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(resetCmd)
}
