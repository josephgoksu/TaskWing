package bootstrap

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestNewInitializer(t *testing.T) {
	basePath := "/test/path"
	init := NewInitializer(basePath)

	if init == nil {
		t.Fatal("NewInitializer returned nil")
	}
	if init.basePath != basePath {
		t.Errorf("basePath = %q, want %q", init.basePath, basePath)
	}
}

func TestValidAINames(t *testing.T) {
	names := ValidAINames()

	// Should return all keys from aiHelpers map
	if len(names) == 0 {
		t.Error("ValidAINames returned empty slice")
	}

	// Check that known AI names are present
	expectedNames := map[string]bool{
		"claude":  false,
		"cursor":  false,
		"gemini":  false,
		"codex":   false,
		"copilot": false,
	}

	for _, name := range names {
		if _, ok := expectedNames[name]; ok {
			expectedNames[name] = true
		}
	}

	for name, found := range expectedNames {
		if !found {
			t.Errorf("Expected AI name %q not found in ValidAINames()", name)
		}
	}
}

func TestInitializer_Run_EmptyAIs(t *testing.T) {
	tmpDir := t.TempDir()
	init := NewInitializer(tmpDir)

	// Should not error with empty AIs
	err := init.Run(false, []string{})
	if err != nil {
		t.Errorf("Run with empty AIs failed: %v", err)
	}

	// Should create .taskwing directory
	if _, err := os.Stat(filepath.Join(tmpDir, ".taskwing")); os.IsNotExist(err) {
		t.Error(".taskwing directory was not created")
	}
}

func TestInitializer_Run_InvalidAI(t *testing.T) {
	tmpDir := t.TempDir()
	init := NewInitializer(tmpDir)

	// Should handle invalid AI names gracefully
	err := init.Run(true, []string{"invalid-ai-name"})
	if err != nil {
		t.Errorf("Run with invalid AI failed: %v", err)
	}
}

func TestInitializer_Run_CreateSlashCommands(t *testing.T) {
	tmpDir := t.TempDir()
	init := NewInitializer(tmpDir)

	// Test with claude
	err := init.Run(false, []string{"claude"})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// Check slash command files were created
	expectedFiles := []string{
		".claude/commands/taskwing.md",
		".claude/commands/tw-next.md",
		".claude/commands/tw-done.md",
	}

	for _, file := range expectedFiles {
		path := filepath.Join(tmpDir, file)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("Expected file %s was not created", file)
		}
	}
}

func TestInitializer_Run_GeminiTOML(t *testing.T) {
	tmpDir := t.TempDir()
	init := NewInitializer(tmpDir)

	err := init.Run(false, []string{"gemini"})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// Check TOML files were created for Gemini
	tomlPath := filepath.Join(tmpDir, ".gemini/commands/taskwing.toml")
	content, err := os.ReadFile(tomlPath)
	if err != nil {
		t.Fatalf("Failed to read TOML file: %v", err)
	}

	// Verify TOML content has expected fields
	contentStr := string(content)
	if !contains(contentStr, "description =") {
		t.Error("TOML file missing description field")
	}
	if !contains(contentStr, "prompt =") {
		t.Error("TOML file missing prompt field")
	}
}

func TestInitializer_InstallHooksConfig(t *testing.T) {
	tmpDir := t.TempDir()
	init := NewInitializer(tmpDir)

	err := init.InstallHooksConfig("claude", false)
	if err != nil {
		t.Fatalf("InstallHooksConfig failed: %v", err)
	}

	// Read the created settings.json
	settingsPath := filepath.Join(tmpDir, ".claude/settings.json")
	content, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("Failed to read settings.json: %v", err)
	}

	// Parse and verify JSON structure
	var config HooksConfig
	if err := json.Unmarshal(content, &config); err != nil {
		t.Fatalf("Invalid JSON in settings.json: %v", err)
	}

	if config.Hooks == nil {
		t.Error("Hooks config is nil")
	}
	if _, ok := config.Hooks["SessionStart"]; !ok {
		t.Error("Missing SessionStart hook")
	}
	if _, ok := config.Hooks["Stop"]; !ok {
		t.Error("Missing Stop hook")
	}
}

func TestInitializer_InstallHooksConfig_MalformedJSON(t *testing.T) {
	tmpDir := t.TempDir()
	init := NewInitializer(tmpDir)

	// Create malformed settings.json
	settingsDir := filepath.Join(tmpDir, ".claude")
	if err := os.MkdirAll(settingsDir, 0755); err != nil {
		t.Fatalf("Failed to create dir: %v", err)
	}
	settingsPath := filepath.Join(settingsDir, "settings.json")
	if err := os.WriteFile(settingsPath, []byte("not valid json{"), 0644); err != nil {
		t.Fatalf("Failed to write malformed JSON: %v", err)
	}

	// Should return error for malformed JSON
	err := init.InstallHooksConfig("claude", false)
	if err == nil {
		t.Error("Expected error for malformed JSON, got nil")
	}
}

func TestInitializer_InstallHooksConfig_ExistingHooks(t *testing.T) {
	tmpDir := t.TempDir()
	init := NewInitializer(tmpDir)

	// Create valid settings.json with hooks
	settingsDir := filepath.Join(tmpDir, ".claude")
	if err := os.MkdirAll(settingsDir, 0755); err != nil {
		t.Fatalf("Failed to create dir: %v", err)
	}
	settingsPath := filepath.Join(settingsDir, "settings.json")
	existingConfig := `{"hooks": {"Test": []}}`
	if err := os.WriteFile(settingsPath, []byte(existingConfig), 0644); err != nil {
		t.Fatalf("Failed to write existing config: %v", err)
	}

	// Should not overwrite existing hooks
	err := init.InstallHooksConfig("claude", false)
	if err != nil {
		t.Fatalf("InstallHooksConfig failed: %v", err)
	}

	// Read back and verify hooks weren't changed
	content, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("Failed to read settings.json: %v", err)
	}

	var config map[string]any
	if err := json.Unmarshal(content, &config); err != nil {
		t.Fatalf("Invalid JSON: %v", err)
	}

	hooks, ok := config["hooks"].(map[string]any)
	if !ok {
		t.Fatal("Hooks field missing or wrong type")
	}
	if _, ok := hooks["Test"]; !ok {
		t.Error("Existing Test hook was removed")
	}
}

func TestInitializer_InstallHooksConfig_UnsupportedAI(t *testing.T) {
	tmpDir := t.TempDir()
	init := NewInitializer(tmpDir)

	// Cursor doesn't support hooks
	err := init.InstallHooksConfig("cursor", false)
	if err != nil {
		t.Errorf("Expected nil for unsupported AI, got: %v", err)
	}
}

func TestCreateSlashCommands_AllAIs(t *testing.T) {
	for aiName := range aiHelpers {
		t.Run(aiName, func(t *testing.T) {
			tmpDir := t.TempDir()
			init := NewInitializer(tmpDir)

			err := init.createSlashCommands(aiName, false)
			if err != nil {
				t.Errorf("createSlashCommands(%s) failed: %v", aiName, err)
			}

			// Verify commands directory exists
			cfg := aiHelpers[aiName]
			cmdDir := filepath.Join(tmpDir, cfg.commandsDir)
			if _, err := os.Stat(cmdDir); os.IsNotExist(err) {
				t.Errorf("Commands directory not created for %s", aiName)
			}
		})
	}
}

func TestCreateSlashCommands_UnknownAI(t *testing.T) {
	tmpDir := t.TempDir()
	init := NewInitializer(tmpDir)

	// Unknown AI should return nil (no error)
	err := init.createSlashCommands("unknown-ai", false)
	if err != nil {
		t.Errorf("Expected nil for unknown AI, got: %v", err)
	}
}

// Helper function to check string containment
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
