/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// configCmd represents the config command
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage TaskWing configuration",
	Long: `Manage TaskWing configuration with simple commands.

Configuration lookup order:
1. ./.taskwing/.taskwing.yaml (project-specific)
2. ./.taskwing.yaml (legacy project root)
3. ~/.taskwing.yaml (global)
4. Environment variables (TASKWING_*)
5. Built-in defaults

Examples:
  taskwing config show                    # Show current configuration
  taskwing config path                    # Show config file location`,
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current configuration",
	Long:  `Display the current TaskWing configuration values and their sources.`,
	Run: func(cmd *cobra.Command, args []string) {
		config := GetConfig()

		fmt.Println("---")
		fmt.Printf("Loaded from: %s\n\n", viper.ConfigFileUsed())

		fmt.Println("[Project]")
		fmt.Printf("  Root Directory:     %s\n", config.Project.RootDir)
		fmt.Printf("  Tasks Directory:    %s\n", config.Project.TasksDir)
		fmt.Printf("  Templates Directory: %s\n", config.Project.TemplatesDir)
		fmt.Printf("  Log Path:           %s\n", config.Project.OutputLogPath)

		fmt.Println("\n[Data]")
		fmt.Printf("  File Name:          %s\n", config.Data.File)
		fmt.Printf("  Format:             %s\n", config.Data.Format)
		fmt.Printf("  Full Task Path:     %s\n", GetTaskFilePath())

		fmt.Println("\n[LLM]")
		fmt.Printf("  Provider:           %s\n", config.LLM.Provider)
		fmt.Printf("  Model:              %s\n", config.LLM.ModelName)
		fmt.Printf("  Max Tokens:         %d\n", config.LLM.MaxOutputTokens)
		fmt.Printf("  Temperature:        %.1f\n", config.LLM.Temperature)

		if config.LLM.APIKey != "" {
			fmt.Printf("  API Key:            [SET]\n")
		} else {
			fmt.Printf("  API Key:            [NOT SET]\n")
		}

		if config.LLM.ProjectID != "" {
			fmt.Printf("  Project ID:         %s\n", config.LLM.ProjectID)
		}
	},
}

var configPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Show configuration file location",
	Long:  `Show the location of the configuration file being used by TaskWing.`,
	Run: func(cmd *cobra.Command, args []string) {
		configFile := viper.ConfigFileUsed()
		if configFile != "" {
			fmt.Printf("TaskWing is using the configuration file at: %s\n", configFile)
		} else {
			fmt.Println("TaskWing is not using a configuration file.")
			fmt.Println("It is running on defaults and/or environment variables.")
			fmt.Println("You can create one by running 'taskwing init'.")
		}
	},
}

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configPathCmd)
}
