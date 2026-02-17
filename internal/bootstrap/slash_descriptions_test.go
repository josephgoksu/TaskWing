package bootstrap

import (
	"strings"
	"testing"
)

func TestSlashCommandsDescriptions_AreTriggerFocused(t *testing.T) {
	for _, cmd := range SlashCommands {
		if !strings.HasPrefix(cmd.Description, "Use when ") {
			t.Fatalf("slash command %q description must start with 'Use when ': %q", cmd.BaseName, cmd.Description)
		}
	}
}
