package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
	"github.com/josephgoksu/TaskWing/internal/config"
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

	// Layered config loading: global first, then project merges on top.
	// Resolution order: project > profile > global > env vars > defaults
	viper.SetConfigType("yaml")

	// 1. Load global config as base layer
	globalConfigFile := filepath.Join(home, ".taskwing", "config.yaml")
	if _, err := os.Stat(globalConfigFile); err == nil {
		viper.SetConfigFile(globalConfigFile)
		if err := viper.ReadInConfig(); err == nil {
			if viper.GetBool("verbose") && !viper.GetBool("json") {
				fmt.Fprintln(os.Stderr, "Loaded global config:", globalConfigFile)
			}
		}
	}

	// 2. Load profile config (merges on top of global)
	// Check env var first, then scan os.Args for --profile flag
	// (Cobra hasn't parsed flags yet when initConfig runs)
	profileName := os.Getenv("TASKWING_PROFILE")
	if profileName == "" {
		profileName = viper.GetString("profile") // from global config.yaml
	}
	if profileName == "" {
		profileName = scanFlagFromArgs("profile")
	}
	if profileName != "" && filepath.Base(profileName) == profileName && !strings.Contains(profileName, "..") {
		profileFile := filepath.Join(home, ".taskwing", "profiles", profileName+".yaml")
		if _, err := os.Stat(profileFile); err == nil {
			profileViper := viper.New()
			profileViper.SetConfigFile(profileFile)
			if err := profileViper.ReadInConfig(); err == nil {
				if err := viper.MergeConfigMap(profileViper.AllSettings()); err == nil {
					if viper.GetBool("verbose") && !viper.GetBool("json") {
						fmt.Fprintln(os.Stderr, "Loaded profile config:", profileFile)
					}
				}
			}
		}
	}

	// 3. Load project config from global store (merges on top, highest file-based priority)
	if projectCtx != nil && projectCtx.RootPath != "" {
		if storePath, err := config.GetProjectStorePath(projectCtx.RootPath); err == nil {
			projectConfigFile := filepath.Join(storePath, "config.yaml")
			if _, err := os.Stat(projectConfigFile); err == nil {
				projectViper := viper.New()
				projectViper.SetConfigFile(projectConfigFile)
				if err := projectViper.ReadInConfig(); err == nil {
					if err := viper.MergeConfigMap(projectViper.AllSettings()); err == nil {
						if viper.GetBool("verbose") && !viper.GetBool("json") {
							fmt.Fprintln(os.Stderr, "Loaded project config:", projectConfigFile)
						}
					}
				}
			}
		}
	}

	// Set defaults for v2
	viper.SetDefault("verbose", false)
	viper.SetDefault("json", false)
	viper.SetDefault("quiet", false)
	viper.SetDefault("preview", false)

	// Memory store path: do NOT set a default here.
	// GetMemoryBasePath() resolves to ~/.taskwing/projects/<slug>/ via GetProjectStorePath.
	// For non-project commands (help, version), GetMemoryBasePathOrGlobal() provides fallback.

	// LLM defaults (for bootstrap scanner)
	// Do NOT set defaults for llm.provider, llm.apiKey, or llm.model
	// We want interactive selection and provider-specific model defaults.
	// Do not set llm.baseURL default globally, it leaks into non-Ollama providers.
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
		// This is expected for non-project directories (e.g., running `taskwing help` from home)
		if viper.GetBool("verbose") {
			fmt.Fprintf(os.Stderr, "Project detection: %v (using global fallback for non-project commands)\n", err)
		}
		return nil
	}

	// Store in config package for GetMemoryBasePath and other consumers
	if err := config.SetProjectContext(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to set project context: %v\n", err)
		return nil
	}

	// Log in verbose mode
	if viper.GetBool("verbose") && ctx.RootPath != cwd {
		fmt.Fprintf(os.Stderr, "Detected project root: %s (via %s)\n", ctx.RootPath, ctx.MarkerType)
	}

	return ctx
}

// scanFlagFromArgs extracts a flag value from os.Args before Cobra parses them.
// Supports --flag=value and --flag value forms.
func scanFlagFromArgs(name string) string {
	prefix := "--" + name
	for i, arg := range os.Args {
		if arg == prefix && i+1 < len(os.Args) {
			return os.Args[i+1]
		}
		if val, ok := strings.CutPrefix(arg, prefix+"="); ok {
			return val
		}
	}
	return ""
}
