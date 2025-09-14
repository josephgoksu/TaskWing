/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package types

// AppConfig represents the complete application configuration
type AppConfig struct {
	Greeting string        `mapstructure:"greeting"`
	Verbose  bool          `mapstructure:"verbose"`
	Config   string        `mapstructure:"config"`
	Project  ProjectConfig `mapstructure:"project" validate:"required"`
	Data     DataConfig    `mapstructure:"data" validate:"required"`
	LLM      LLMConfig     `mapstructure:"llm" validate:"omitempty"`
}

// ProjectConfig holds project-related settings
type ProjectConfig struct {
	RootDir       string `mapstructure:"rootDir" validate:"required"`
	TasksDir      string `mapstructure:"tasksDir" validate:"required"`
	TemplatesDir  string `mapstructure:"templatesDir" validate:"required"`
	OutputLogPath string `mapstructure:"outputLogPath" validate:"required"`
	CurrentTaskID string `mapstructure:"currentTaskId" validate:"omitempty,uuid4"`
}

// DataConfig holds data storage configuration
type DataConfig struct {
	File   string `mapstructure:"file" validate:"required"`
	Format string `mapstructure:"format" validate:"required,oneof=json yaml toml"`
}

// LLMConfig holds configuration for LLM integration
type LLMConfig struct {
	Provider                   string  `mapstructure:"provider" validate:"omitempty,oneof=openai"`
	ModelName                  string  `mapstructure:"modelName" validate:"omitempty,min=1"`
	APIKey                     string  `mapstructure:"apiKey" validate:"omitempty,min=1"`
	ProjectID                  string  `mapstructure:"projectId" validate:"omitempty,min=1"`
	MaxOutputTokens            int     `mapstructure:"maxOutputTokens" validate:"omitempty,min=1"`
	Temperature                float64 `mapstructure:"temperature" validate:"omitempty,min=0,max=2"`
	ImprovementTemperature     float64 `mapstructure:"improvementTemperature" validate:"omitempty,min=0,max=2"`
	ImprovementMaxOutputTokens int     `mapstructure:"improvementMaxOutputTokens" validate:"omitempty,min=1"`
	// RequestTimeoutSeconds controls the HTTP client timeout for LLM calls
	RequestTimeoutSeconds int `mapstructure:"requestTimeoutSeconds" validate:"omitempty,min=5,max=600"`
	// MaxRetries controls how many automatic retries on recoverable errors (timeouts, temp rejection)
	MaxRetries int `mapstructure:"maxRetries" validate:"omitempty,min=0,max=3"`
	// Debug enables extra request/response logging within the LLM provider (generally tied to --verbose)
	Debug bool `mapstructure:"debug"`
}
