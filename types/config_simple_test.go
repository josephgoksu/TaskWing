package types

import (
	"testing"
)

func TestAppConfig_Structure(t *testing.T) {
	config := AppConfig{
		Project: ProjectConfig{
			RootDir:       "/home/user/.taskwing",
			TasksDir:      "tasks",
			TemplatesDir:  "templates",
			OutputLogPath: "/tmp/taskwing.log",
		},
		Data: DataConfig{
			File:   "tasks.json",
			Format: "json",
		},
		LLM: LLMConfig{
			Provider:    "openai",
			ModelName:   "gpt-4",
			Temperature: 0.7,
		},
	}

	// Test basic structure
	if config.Project.RootDir != "/home/user/.taskwing" {
		t.Errorf("Project.RootDir mismatch: got %q, want %q", config.Project.RootDir, "/home/user/.taskwing")
	}
	if config.Data.Format != "json" {
		t.Errorf("Data.Format mismatch: got %q, want %q", config.Data.Format, "json")
	}
	if config.LLM.Provider != "openai" {
		t.Errorf("LLM.Provider mismatch: got %q, want %q", config.LLM.Provider, "openai")
	}
}

func TestProjectConfig_Structure(t *testing.T) {
	config := ProjectConfig{
		RootDir:       "/test/path",
		TasksDir:      "tasks",
		TemplatesDir:  "templates",
		OutputLogPath: "/test/log.txt",
		CurrentTaskID: "",
	}

	if config.RootDir != "/test/path" {
		t.Errorf("RootDir mismatch: got %q, want %q", config.RootDir, "/test/path")
	}
	if config.TasksDir != "tasks" {
		t.Errorf("TasksDir mismatch: got %q, want %q", config.TasksDir, "tasks")
	}
}

func TestDataConfig_Structure(t *testing.T) {
	config := DataConfig{
		File:   "tasks.yaml",
		Format: "yaml",
	}

	if config.File != "tasks.yaml" {
		t.Errorf("File mismatch: got %q, want %q", config.File, "tasks.yaml")
	}
	if config.Format != "yaml" {
		t.Errorf("Format mismatch: got %q, want %q", config.Format, "yaml")
	}
}

func TestLLMConfig_Structure(t *testing.T) {
	config := LLMConfig{
		Provider:                   "openai",
		ModelName:                  "gpt-4",
		APIKey:                     "test-key",
		MaxOutputTokens:            2048,
		Temperature:                0.7,
		ImprovementTemperature:     0.5,
		ImprovementMaxOutputTokens: 1024,
		RequestTimeoutSeconds:      30,
		MaxRetries:                 2,
		Debug:                      true,
	}

	if config.Provider != "openai" {
		t.Errorf("Provider mismatch: got %q, want %q", config.Provider, "openai")
	}
	if config.ModelName != "gpt-4" {
		t.Errorf("ModelName mismatch: got %q, want %q", config.ModelName, "gpt-4")
	}
	if config.MaxOutputTokens != 2048 {
		t.Errorf("MaxOutputTokens mismatch: got %d, want %d", config.MaxOutputTokens, 2048)
	}
	if config.Temperature != 0.7 {
		t.Errorf("Temperature mismatch: got %f, want %f", config.Temperature, 0.7)
	}
	if config.Debug != true {
		t.Errorf("Debug mismatch: got %v, want %v", config.Debug, true)
	}
}
