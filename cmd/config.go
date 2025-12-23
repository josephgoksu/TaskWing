package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
	"github.com/josephgoksu/TaskWing/internal/config"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	configName = ".taskwing"
	envPrefix  = "TASKWING"
)

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

	// Memory store defaults
	viper.SetDefault("memory.path", ".taskwing/memory")

	// LLM defaults (for bootstrap scanner)
	// Do NOT set defaults for llm.provider or llm.apiKey - we want interactive selection if not set
	viper.SetDefault("llm.model", config.DefaultOpenAIModel)
	viper.SetDefault("llm.baseURL", config.DefaultOllamaURL)
	viper.SetDefault("llm.maxOutputTokens", 0)
	viper.SetDefault("llm.temperature", 0.7)
}
