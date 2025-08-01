package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	configName = ".taskwing"
	envPrefix  = "TASKWING"
)

// LLMConfig holds configuration for Large Language Model interactions.
type LLMConfig struct {
	Provider                   string  `mapstructure:"provider" validate:"omitempty,oneof=openai google"`
	ModelName                  string  `mapstructure:"modelName" validate:"omitempty,min=1"`
	APIKey                     string  `mapstructure:"apiKey" validate:"omitempty,min=1"`    // Sensitive, primarily for ENV var loading
	ProjectID                  string  `mapstructure:"projectId" validate:"omitempty,min=1"` // For Google Cloud
	MaxOutputTokens            int     `mapstructure:"maxOutputTokens" validate:"omitempty,min=1"`
	Temperature                float64 `mapstructure:"temperature" validate:"omitempty,min=0,max=2"`
	EstimationTemperature      float64 `mapstructure:"estimationTemperature" validate:"omitempty,min=0,max=2"`
	EstimationMaxOutputTokens  int     `mapstructure:"estimationMaxOutputTokens" validate:"omitempty,min=1"`
	ImprovementTemperature     float64 `mapstructure:"improvementTemperature" validate:"omitempty,min=0,max=2"`
	ImprovementMaxOutputTokens int     `mapstructure:"improvementMaxOutputTokens" validate:"omitempty,min=1"`
}

// ProjectConfig holds project path related configuration
type ProjectConfig struct {
	RootDir       string `mapstructure:"rootDir" validate:"required"`       // Base directory for all project-specific files
	TasksDir      string `mapstructure:"tasksDir" validate:"required"`      // Relative to RootDir
	TemplatesDir  string `mapstructure:"templatesDir" validate:"required"`  // Relative to RootDir
	OutputLogPath string `mapstructure:"outputLogPath" validate:"required"` // Can be relative to RootDir or absolute
}

// AppConfig holds the application's entire configuration
type AppConfig struct {
	Greeting string        `mapstructure:"greeting"`
	Verbose  bool          `mapstructure:"verbose"`
	Config   string        `mapstructure:"config"`
	Project  ProjectConfig `mapstructure:"project" validate:"required"`
	LLM      LLMConfig     `mapstructure:"llm" validate:"omitempty"` // LLM config is optional overall
}

// GlobalAppConfig holds the global application configuration instance.
var GlobalAppConfig AppConfig

// validate is a single instance of Translate, it caches struct info
var validate *validator.Validate

func init() {
	validate = validator.New()
}

// validateAppConfig performs validation on the AppConfig struct.
func validateAppConfig(config *AppConfig) error {
	errs := validate.Struct(config)
	if errs != nil {
		// Optionally, you can iterate through errs to get more specific error messages.
		// For now, just return the full error object.
		return errs
	}
	return nil
}

// InitConfig reads in config file and ENV variables if set.
func InitConfig() {
	// Load .env file first if present
	if err := godotenv.Load(); err != nil {
		// It's okay if .env file doesn't exist.
		// If verbose, we could print a notice, but it's not critical.
	}

	// Environment variable handling must be set up BEFORE reading the config file
	// or checking for cfgFile, so that env vars can influence config loading if needed
	// (e.g. an env var pointing to a config directory, though not used here directly).
	viper.SetEnvPrefix(envPrefix)                          // e.g., TASKWING_VERBOSE
	viper.AutomaticEnv()                                   // Read in environment variables that match
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_")) // Replace dots with underscores in env var names

	cfgFileFlag := viper.GetString("config") // Value from --config flag

	// Determine project root directory for config search path priority
	// Use default here, as GlobalAppConfig might not be unmarshaled yet.
	// We need project.rootDir *before* full unmarshal to locate the config file itself.
	// Viper provides a way to get a string with a default if not set:
	// However, viper.GetString("project.rootDir") might try to load config if it hasn't been told where to look yet.
	// So, we will assume a default like ".taskwing" for the purpose of *finding* the config file.
	// The actual value from config will be used once loaded.
	potentialProjectConfigDir := viper.GetString("project.rootDir")
	if potentialProjectConfigDir == "" { // If not set by ENV or previous (unlikely) viper.ReadInConfig
		potentialProjectConfigDir = ".taskwing" // Default directory name to check for project-specific config
	}

	if cfgFileFlag != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFileFlag)
	} else {
		// Check if potentialProjectConfigDir (e.g., ./.taskwing) exists
		if _, err := os.Stat(potentialProjectConfigDir); !os.IsNotExist(err) {
			// Project-specific config directory exists. Prioritize it.
			viper.AddConfigPath(potentialProjectConfigDir) // e.g., look in ./.taskwing/
			viper.SetConfigName(configName)                // configName is ".taskwing" -> ./.taskwing/.taskwing.yaml
		} else {
			// Project-specific config dir not found, fallback to home and current directory for global/legacy config
			home, err := os.UserHomeDir()
			cobra.CheckErr(err)
			viper.AddConfigPath(home)       // $HOME/.taskwing.yaml
			viper.AddConfigPath(".")        // ./.taskwing.yaml (legacy project root config)
			viper.SetConfigName(configName) // Still looking for a file named ".taskwing"
		}
	}

	// Attempt to read the configuration file.
	if err := viper.ReadInConfig(); err == nil {
		if viper.GetBool("verbose") {
			fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
		}
	} else {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			if cfgFileFlag != "" {
				// If a specific config file was provided by flag but not found, it's an error to report.
				fmt.Fprintln(os.Stderr, "Error: Specified config file not found:", cfgFileFlag)
				// os.Exit(1) // Or handle more gracefully depending on requirements
			} else if viper.GetBool("verbose") {
				// Config file not found by search paths, which is fine.
				fmt.Fprintln(os.Stderr, "No config file found. Using defaults and environment variables.")
			}
		} else {
			// Config file was found but another error was produced (e.g., parsing error).
			fmt.Fprintln(os.Stderr, "Error reading config file:", viper.ConfigFileUsed(), "-", err)
			// os.Exit(1) // Or handle more gracefully
		}
	}

	// Set default values
	viper.SetDefault("greeting", "Hello from TaskWing!")

	viper.SetDefault("project.rootDir", ".taskwing")
	viper.SetDefault("project.tasksDir", "tasks")
	viper.SetDefault("project.templatesDir", "templates")
	viper.SetDefault("project.outputLogPath", "logs/taskwing.log")
	viper.SetDefault("data.file", "tasks.json")
	viper.SetDefault("data.format", "json")

	// Defaults for LLMConfig
	viper.SetDefault("llm.provider", "openai")
	viper.SetDefault("llm.modelName", "")
	viper.SetDefault("llm.apiKey", "")
	viper.SetDefault("llm.projectId", "")
	viper.SetDefault("llm.maxOutputTokens", 16384)
	viper.SetDefault("llm.temperature", 0.7)

	// After all sources are configured, unmarshal into GlobalAppConfig
	if err := viper.Unmarshal(&GlobalAppConfig); err != nil {
		fmt.Fprintf(os.Stderr, "Error unmarshaling config: %s\n", err)
		os.Exit(1) // Exit if unmarshaling fails
	}

	// Ensure critical project paths are set, falling back to Viper's defaults if empty after unmarshal.
	// This handles cases where a config file might exist but be missing these specific nested keys.
	if GlobalAppConfig.Project.RootDir == "" {
		GlobalAppConfig.Project.RootDir = viper.GetString("project.rootDir")
	}
	if GlobalAppConfig.Project.TasksDir == "" {
		GlobalAppConfig.Project.TasksDir = viper.GetString("project.tasksDir")
	}
	// Ensure outputLogPath is also sensible, potentially making it relative to RootDir if not absolute
	if GlobalAppConfig.Project.OutputLogPath == "" {
		GlobalAppConfig.Project.OutputLogPath = viper.GetString("project.outputLogPath")
	}
	if GlobalAppConfig.Project.RootDir != "" && GlobalAppConfig.Project.OutputLogPath != "" && !filepath.IsAbs(GlobalAppConfig.Project.OutputLogPath) {
		GlobalAppConfig.Project.OutputLogPath = filepath.Join(GlobalAppConfig.Project.RootDir, GlobalAppConfig.Project.OutputLogPath)
	}

	// Validate the populated configuration
	if err := validateAppConfig(&GlobalAppConfig); err != nil {
		fmt.Fprintf(os.Stderr, "Configuration validation error: %s\n", err)
		// You can iterate through err.(validator.ValidationErrors) for detailed messages
		// Example:
		// for _, err := range err.(validator.ValidationErrors) {
		// 	 fmt.Println(err.Namespace())
		// 	 fmt.Println(err.Field())
		// 	 fmt.Println(err.StructNamespace())
		// 	 fmt.Println(err.StructField())
		// 	 fmt.Println(err.Tag())
		// 	 fmt.Println(err.ActualTag())
		// 	 fmt.Println(err.Kind())
		// 	 fmt.Println(err.Type())
		// 	 fmt.Println(err.Value())
		// 	 fmt.Println(err.Param())
		// 	 fmt.Println()
		// }
		os.Exit(1) // Exit if validation fails
	}

	// The verbose and config values are bound from flags directly to Viper.
	// When Unmarshal runs, it will populate GlobalAppConfig.Verbose and GlobalAppConfig.Config
	// if those fields exist in the struct and are mapped.
	// We've added them to AppConfig struct with mapstructure tags.
}

// GetConfig returns a pointer to the global AppConfig instance.
func GetConfig() *AppConfig {
	return &GlobalAppConfig
}

// Example of how to get a config value
// func GetGreeting() string { // This can now be GetConfig().Greeting
// 	return viper.GetString("greeting")
// }
