/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

import (
	"fmt"

	"github.com/josephgoksu/TaskWing/internal/ui"
)

// AI assistant configurations
type aiConfig struct {
	name        string
	displayName string
	commandsDir string
	fileExt     string // ".md" or ".toml"
}

// Ordered list for consistent display
var aiConfigOrder = []string{"claude", "cursor", "copilot", "gemini", "codex"}

var aiConfigs = map[string]aiConfig{
	"claude": {
		name:        "claude",
		displayName: "Claude Code",
		commandsDir: ".claude/commands",
		fileExt:     ".md",
	},
	"cursor": {
		name:        "cursor",
		displayName: "Cursor",
		commandsDir: ".cursor/rules",
		fileExt:     ".md",
	},
	"copilot": {
		name:        "copilot",
		displayName: "GitHub Copilot",
		commandsDir: ".github/copilot-instructions",
		fileExt:     ".md",
	},
	"gemini": {
		name:        "gemini",
		displayName: "Gemini CLI",
		commandsDir: ".gemini/commands",
		fileExt:     ".toml",
	},
	"codex": {
		name:        "codex",
		displayName: "OpenAI Codex",
		commandsDir: ".codex/commands",
		fileExt:     ".md",
	},
}

func promptAISelection() []string {
	descriptions := make(map[string]string)
	for _, id := range aiConfigOrder {
		descriptions[id] = aiConfigs[id].displayName
	}

	selected, err := ui.PromptAISelection(aiConfigOrder, descriptions)
	if err != nil {
		fmt.Printf("Error running selection: %v\n", err)
		return nil
	}
	return selected
}
