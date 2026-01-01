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
}

// Ordered list for consistent display
var aiConfigOrder = []string{"claude", "cursor", "copilot", "gemini", "codex"}

var aiConfigs = map[string]aiConfig{
	"claude": {
		name:        "claude",
		displayName: "Claude Code",
		commandsDir: ".claude/commands",
	},
	"cursor": {
		name:        "cursor",
		displayName: "Cursor",
		commandsDir: ".cursor/rules",
	},
	"copilot": {
		name:        "copilot",
		displayName: "GitHub Copilot",
		commandsDir: ".github/copilot-instructions",
	},
	"gemini": {
		name:        "gemini",
		displayName: "Gemini CLI",
		commandsDir: ".gemini/commands",
	},
	"codex": {
		name:        "codex",
		displayName: "OpenAI Codex",
		commandsDir: ".codex/commands",
	},
}

// NOTE: initCmd is deprecated - use 'tw bootstrap' which auto-initializes
// The structs and functions below are kept for use by bootstrap.go

func promptAISelection() []string {
	// Build display map for UI
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
