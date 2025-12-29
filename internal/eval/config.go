package eval

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	"gopkg.in/yaml.v3"
)

// LoadConfig reads and parses a tasks.yaml file.
func LoadConfig(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read tasks: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse tasks: %w", err)
	}
	return cfg, nil
}

// WriteFileIfMissing writes content to path if it doesn't exist (or force is true).
func WriteFileIfMissing(path string, content string, force bool) error {
	if !force {
		if _, err := os.Stat(path); err == nil {
			return nil
		}
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

// ParseModel splits a model string into provider and model name.
// Input: "openai:gpt-4o" -> ("openai", "gpt-4o")
// Input: "gpt-4o" -> ("", "gpt-4o")
func ParseModel(input string) (provider, model string) {
	if strings.Contains(input, ":") {
		parts := strings.SplitN(input, ":", 2)
		return parts[0], parts[1]
	}
	return "", input
}

// SafeName converts a model name to a filesystem-safe name.
func SafeName(input string) string {
	safe := strings.ReplaceAll(input, "/", "_")
	safe = strings.ReplaceAll(safe, " ", "_")
	safe = strings.ReplaceAll(safe, ":", "_")
	if runtime.GOOS == "windows" {
		safe = strings.ReplaceAll(safe, "\\", "_")
	}
	return safe
}

// Contains checks if a string slice contains a specific item.
func Contains(list []string, item string) bool {
	for _, v := range list {
		if v == item {
			return true
		}
	}
	return false
}
