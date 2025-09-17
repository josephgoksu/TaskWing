package prompts

import (
	"os"
	"strings"
	"testing"
)

func TestGetPrompt(t *testing.T) {
	templatesDir := os.TempDir() // Use temp dir for testing

	tests := []struct {
		name      string
		promptKey PromptKey
		wantError bool
		contains  []string
	}{
		{
			name:      "generate tasks prompt",
			promptKey: KeyGenerateTasks,
			wantError: false,
			contains:  []string{"task"},
		},
		{
			name:      "enhance task prompt",
			promptKey: KeyEnhanceTask,
			wantError: false,
			contains:  []string{"task"},
		},
		{
			name:      "breakdown task prompt",
			promptKey: KeyBreakdownTask,
			wantError: false,
			contains:  []string{"subtask"},
		},
		{
			name:      "suggest next task prompt",
			promptKey: KeySuggestNextTask,
			wantError: false,
			contains:  []string{"next"},
		},
		{
			name:      "detect dependencies prompt",
			promptKey: KeyDetectDependencies,
			wantError: false,
			contains:  []string{"dependen"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prompt, err := GetPrompt(tt.promptKey, templatesDir)
			if (err != nil) != tt.wantError {
				t.Errorf("GetPrompt() error = %v, wantError %v", err, tt.wantError)
				return
			}
			if !tt.wantError {
				promptLower := strings.ToLower(prompt)
				for _, expected := range tt.contains {
					if !strings.Contains(promptLower, strings.ToLower(expected)) {
						t.Errorf("GetPrompt(%v) missing expected content %q in prompt", tt.promptKey, expected)
					}
				}
			}
		})
	}
}
