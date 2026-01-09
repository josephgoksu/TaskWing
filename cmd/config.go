package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
	"github.com/josephgoksu/TaskWing/internal/llm"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const envPrefix = "TASKWING"

// initConfig reads in config file and ENV variables if set.
// This is called by cobra.OnInitialize in root.go
func initConfig() {
	// Load .env file first if present (ignore error if file doesn't exist)
	_ = godotenv.Load()

	// Environment variable handling
	viper.SetEnvPrefix(envPrefix)                          // e.g., TASKWING_VERBOSE
	viper.AutomaticEnv()                                   // Read in environment variables that match
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_")) // Replace dots with underscores

	home, err := os.UserHomeDir()
	cobra.CheckErr(err)

	// ALWAYS add global config path first (highest priority for user settings)
	viper.AddConfigPath(filepath.Join(home, ".taskwing"))

	// Also check local .taskwing directory for project-specific overrides
	if _, err := os.Stat(".taskwing"); !os.IsNotExist(err) {
		viper.AddConfigPath(".taskwing")
	}

	viper.SetConfigName("config") // looks for config.yaml
	viper.SetConfigType("yaml")

	// Attempt to read the configuration file
	if err := viper.ReadInConfig(); err == nil {
		if viper.GetBool("verbose") {
			fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
		}
	}

	// Set defaults for v2
	viper.SetDefault("verbose", false)
	viper.SetDefault("json", false)
	viper.SetDefault("quiet", false)
	viper.SetDefault("preview", false)

	// Memory store path: do NOT set a default here.
	// The fallback logic in config.GetMemoryBasePath() handles defaults properly:
	// 1. If user sets memory.path in config → use that
	// 2. If XDG_DATA_HOME is set → use $XDG_DATA_HOME/taskwing/memory
	// 3. Otherwise → use ~/.taskwing/memory (global)
	// Setting a default here would bypass that fallback chain.

	// LLM defaults (for bootstrap scanner)
	// Do NOT set defaults for llm.provider, llm.apiKey, or llm.model
	// We want interactive selection and provider-specific model defaults
	viper.SetDefault("llm.baseURL", llm.DefaultOllamaURL)
	viper.SetDefault("llm.maxOutputTokens", 0)
	viper.SetDefault("llm.temperature", 0.7)
}
