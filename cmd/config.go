package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
	"github.com/josephgoksu/TaskWing/internal/config"
	"github.com/josephgoksu/TaskWing/internal/llm"
	"github.com/josephgoksu/TaskWing/internal/project"
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

	// Detect project root using zero-config detection
	// This finds the nearest .taskwing, go.mod, package.json, etc.
	projectCtx := detectProjectRoot()

	// ALWAYS add global config path first (highest priority for user settings)
	viper.AddConfigPath(filepath.Join(home, ".taskwing"))

	// Add detected project root's .taskwing directory for project-specific config
	if projectCtx != nil && projectCtx.RootPath != "" {
		projectConfigPath := filepath.Join(projectCtx.RootPath, ".taskwing")
		if info, err := os.Stat(projectConfigPath); err == nil && info.IsDir() {
			viper.AddConfigPath(projectConfigPath)
		}
	}

	// Legacy: Also check CWD's .taskwing directory (for backwards compatibility)
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
	// GetMemoryBasePath() has FAIL-FAST semantics:
	// 1. If user sets memory.path in config → use that
	// 2. Detected project root → use {project_root}/.taskwing/memory
	// 3. Otherwise → return error (no silent fallbacks)
	// For non-project commands (help, version), GetMemoryBasePathOrGlobal() provides ~/.taskwing fallback.

	// LLM defaults (for bootstrap scanner)
	// Do NOT set defaults for llm.provider, llm.apiKey, or llm.model
	// We want interactive selection and provider-specific model defaults
	viper.SetDefault("llm.baseURL", llm.DefaultOllamaURL)
	viper.SetDefault("llm.maxOutputTokens", 0)
	viper.SetDefault("llm.temperature", 0.7)
}

// detectProjectRoot uses the project package to detect the project boundary.
// It stores the result in the config package for downstream consumption.
// FAIL-FAST: Errors are logged to stderr for debugging but nil is returned
// to allow non-project commands (help, version) to proceed.
func detectProjectRoot() *project.Context {
	cwd, err := os.Getwd()
	if err != nil {
		// Log error for debugging - this is a system-level failure
		fmt.Fprintf(os.Stderr, "Warning: cannot get working directory: %v\n", err)
		return nil
	}

	ctx, err := project.Detect(cwd)
	if err != nil {
		// Log detection failure for debugging
		// This is expected for non-project directories (e.g., running `tw help` from home)
		if viper.GetBool("verbose") {
			fmt.Fprintf(os.Stderr, "Project detection: %v (using global fallback for non-project commands)\n", err)
		}
		return nil
	}

	// Store in config package for GetMemoryBasePath and other consumers
	config.SetProjectContext(ctx)

	// Log in verbose mode
	if viper.GetBool("verbose") && ctx.RootPath != cwd {
		fmt.Fprintf(os.Stderr, "Detected project root: %s (via %s)\n", ctx.RootPath, ctx.MarkerType)
	}

	return ctx
}
