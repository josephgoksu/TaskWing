package prompts

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// PromptKey is a type for identifying specific prompts.
type PromptKey string

const (
	// KeyGenerateTasks is the key for the main task generation prompt.
	KeyGenerateTasks PromptKey = "GenerateTasks"
	// KeyImprovePRD is the key for the PRD improvement prompt.
	KeyImprovePRD PromptKey = "ImprovePRD"
	// KeyEnhanceTask is the key for the single task enhancement prompt.
	KeyEnhanceTask PromptKey = "EnhanceTask"
)

// promptConfig defines the default content and filename for a prompt.
type promptConfig struct {
	defaultContent string
	filename       string
}

// promptRegistry maps a PromptKey to its configuration.
var promptRegistry = map[PromptKey]promptConfig{
	KeyGenerateTasks: {
		defaultContent: GenerateTasksSystemPrompt,
		filename:       "generate_tasks_prompt.txt",
	},
	KeyImprovePRD: {
		defaultContent: ImprovePRDSystemPrompt,
		filename:       "improve_prd_prompt.txt",
	},
	KeyEnhanceTask: {
		defaultContent: EnhanceTaskSystemPrompt,
		filename:       "enhance_task_prompt.txt",
	},
}

// GetPrompt searches for a user-provided prompt file in the project's templates
// directory. If found, it returns the content of that file. Otherwise, it returns
// the hardcoded default prompt content.
func GetPrompt(key PromptKey, templatesDir string) (string, error) {
	config, ok := promptRegistry[key]
	if !ok {
		return "", fmt.Errorf("unrecognized prompt key: %s", key)
	}

	// If templatesDir is not configured or is empty, always use default.
	if strings.TrimSpace(templatesDir) == "" {
		return config.defaultContent, nil
	}

	customPromptPath := filepath.Join(templatesDir, config.filename)

	// Check if the custom prompt file exists.
	if _, err := os.Stat(customPromptPath); err == nil {
		// File exists, read its content.
		content, readErr := os.ReadFile(customPromptPath)
		if readErr != nil {
			return "", fmt.Errorf("failed to read custom prompt file at %s: %w", customPromptPath, readErr)
		}
		// Use the custom prompt.
		fmt.Printf("Using custom prompt from: %s\n", customPromptPath) // Inform user
		return string(content), nil
	} else if !os.IsNotExist(err) {
		// Some other error occurred when checking for the file (e.g., permissions).
		return "", fmt.Errorf("error checking for custom prompt file at %s: %w", customPromptPath, err)
	}

	// File does not exist, so return the default content.
	return config.defaultContent, nil
}
