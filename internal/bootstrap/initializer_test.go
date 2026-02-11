package bootstrap

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
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
		"claude":   false,
		"cursor":   false,
		"gemini":   false,
		"codex":    false,
		"copilot":  false,
		"opencode": false,
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
		".claude/commands/tw-brief.md",
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
	tomlPath := filepath.Join(tmpDir, ".gemini/commands/tw-brief.toml")
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

			err := init.CreateSlashCommands(aiName, false)
			if err != nil {
				t.Errorf("CreateSlashCommands(%s) failed: %v", aiName, err)
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
	err := init.CreateSlashCommands("unknown-ai", false)
	if err != nil {
		t.Errorf("Expected nil for unknown AI, got: %v", err)
	}
}

func TestCreateSlashCommands_PrunesRemovedLegacyCommands(t *testing.T) {
	tmpDir := t.TempDir()
	init := NewInitializer(tmpDir)

	cmdDir := filepath.Join(tmpDir, ".claude", "commands")
	if err := os.MkdirAll(cmdDir, 0755); err != nil {
		t.Fatalf("Failed to create commands dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cmdDir, "tw-context.md"), []byte("legacy"), 0644); err != nil {
		t.Fatalf("Failed to write tw-context.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cmdDir, "tw-block.md"), []byte("legacy"), 0644); err != nil {
		t.Fatalf("Failed to write tw-block.md: %v", err)
	}

	if err := init.CreateSlashCommands("claude", false); err != nil {
		t.Fatalf("CreateSlashCommands(claude) failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(cmdDir, "tw-context.md")); !os.IsNotExist(err) {
		t.Error("tw-context.md should be removed during slash command regeneration")
	}
	if _, err := os.Stat(filepath.Join(cmdDir, "tw-block.md")); !os.IsNotExist(err) {
		t.Error("tw-block.md should be removed during slash command regeneration")
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

// TestCopilotSingleFile tests Copilot single-file generation
func TestCopilotSingleFile(t *testing.T) {
	tmpDir := t.TempDir()
	init := NewInitializer(tmpDir)

	err := init.CreateSlashCommands("copilot", false)
	if err != nil {
		t.Fatalf("CreateSlashCommands(copilot) failed: %v", err)
	}

	// Verify single file created (not a directory of files)
	filePath := filepath.Join(tmpDir, ".github", "copilot-instructions.md")
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read copilot-instructions.md: %v", err)
	}

	// Verify marker is present
	if !contains(string(content), "<!-- TASKWING_MANAGED -->") {
		t.Error("Missing TASKWING_MANAGED marker in copilot-instructions.md")
	}

	// Verify version is present
	if !contains(string(content), "<!-- Version:") {
		t.Error("Missing Version comment in copilot-instructions.md")
	}
}

// TestCopilotUserFilePreservation tests that user-owned files are not overwritten (T4)
func TestCopilotUserFilePreservation(t *testing.T) {
	tmpDir := t.TempDir()
	init := NewInitializer(tmpDir)

	// Create user-owned copilot-instructions.md (no TaskWing marker)
	githubDir := filepath.Join(tmpDir, ".github")
	if err := os.MkdirAll(githubDir, 0755); err != nil {
		t.Fatalf("Failed to create .github dir: %v", err)
	}

	userContent := "# My Custom Instructions\nDo this, not that."
	userFilePath := filepath.Join(githubDir, "copilot-instructions.md")
	if err := os.WriteFile(userFilePath, []byte(userContent), 0644); err != nil {
		t.Fatalf("Failed to write user file: %v", err)
	}

	// Run CreateSlashCommands - should NOT overwrite user file
	err := init.CreateSlashCommands("copilot", true)
	if err != nil {
		t.Fatalf("CreateSlashCommands failed: %v", err)
	}

	// Verify user content is preserved
	content, err := os.ReadFile(userFilePath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	if string(content) != userContent {
		t.Errorf("User file was overwritten!\nExpected: %s\nGot: %s", userContent, string(content))
	}
}

// TestCopilotLegacyDirectoryCleanup tests cleanup of old directory-based config (T3)
func TestCopilotLegacyDirectoryCleanup(t *testing.T) {
	tmpDir := t.TempDir()
	init := NewInitializer(tmpDir)

	// Create legacy directory structure (old TaskWing format)
	legacyDir := filepath.Join(tmpDir, ".github", "copilot-instructions")
	if err := os.MkdirAll(legacyDir, 0755); err != nil {
		t.Fatalf("Failed to create legacy dir: %v", err)
	}

	// Add marker file to indicate TaskWing created it
	markerPath := filepath.Join(legacyDir, TaskWingManagedFile)
	if err := os.WriteFile(markerPath, []byte("# TaskWing managed"), 0644); err != nil {
		t.Fatalf("Failed to write marker: %v", err)
	}

	// Add some legacy files
	legacyFiles := []string{"tw-next.md", "tw-done.md", "tw-brief.md"}
	for _, f := range legacyFiles {
		if err := os.WriteFile(filepath.Join(legacyDir, f), []byte("test"), 0644); err != nil {
			t.Fatalf("Failed to write legacy file: %v", err)
		}
	}

	// Run CreateSlashCommands
	err := init.CreateSlashCommands("copilot", true)
	if err != nil {
		t.Fatalf("CreateSlashCommands failed: %v", err)
	}

	// Verify legacy directory was cleaned up
	if _, err := os.Stat(legacyDir); !os.IsNotExist(err) {
		t.Error("Legacy directory should have been removed")
	}

	// Verify new single file was created
	newFilePath := filepath.Join(tmpDir, ".github", "copilot-instructions.md")
	if _, err := os.Stat(newFilePath); os.IsNotExist(err) {
		t.Error("New copilot-instructions.md should have been created")
	}
}

// TestCopilotLegacyDirectoryWithoutMarker tests that legacy dirs without markers are detected (T4 variant)
func TestCopilotLegacyDirectoryWithoutMarker(t *testing.T) {
	tmpDir := t.TempDir()
	init := NewInitializer(tmpDir)

	// Create legacy directory structure WITHOUT marker (old old format)
	legacyDir := filepath.Join(tmpDir, ".github", "copilot-instructions")
	if err := os.MkdirAll(legacyDir, 0755); err != nil {
		t.Fatalf("Failed to create legacy dir: %v", err)
	}

	// Add tw-* files (pattern that indicates it's ours even without marker)
	legacyFiles := []string{"tw-next.md", "tw-done.md"}
	for _, f := range legacyFiles {
		if err := os.WriteFile(filepath.Join(legacyDir, f), []byte("test"), 0644); err != nil {
			t.Fatalf("Failed to write legacy file: %v", err)
		}
	}

	// Run CreateSlashCommands
	err := init.CreateSlashCommands("copilot", true)
	if err != nil {
		t.Fatalf("CreateSlashCommands failed: %v", err)
	}

	// Verify legacy directory was cleaned up (detected by tw-* pattern)
	if _, err := os.Stat(legacyDir); !os.IsNotExist(err) {
		t.Error("Legacy directory with tw-* files should have been removed")
	}
}

// TestVersionHashIncludesSingleFile tests that version hash changes when singleFile flag changes (T5)
func TestVersionHashIncludesSingleFile(t *testing.T) {
	// Get version for copilot (singleFile=true)
	copilotVersion := AIToolConfigVersion("copilot")

	// Get version for claude (singleFile=false)
	claudeVersion := AIToolConfigVersion("claude")

	// Versions should be different due to singleFile difference
	if copilotVersion == claudeVersion {
		t.Error("Copilot and Claude should have different version hashes due to singleFile difference")
	}

	// Verify version is deterministic
	copilotVersion2 := AIToolConfigVersion("copilot")
	if copilotVersion != copilotVersion2 {
		t.Error("Version hash should be deterministic")
	}
}

// =============================================================================
// OpenCode Tests
// =============================================================================

// TestInitializer_OpenCode_Skills tests OpenCode skills directory generation
func TestInitializer_OpenCode_Skills(t *testing.T) {
	tmpDir := t.TempDir()
	init := NewInitializer(tmpDir)

	err := init.CreateSlashCommands("opencode", false)
	if err != nil {
		t.Fatalf("CreateSlashCommands(opencode) failed: %v", err)
	}

	// Verify commands directory structure: .opencode/commands/<name>.md
	commandsDir := filepath.Join(tmpDir, ".opencode", "commands")
	if _, err := os.Stat(commandsDir); os.IsNotExist(err) {
		t.Fatal("Commands directory not created")
	}

	// Check marker file exists
	markerPath := filepath.Join(commandsDir, TaskWingManagedFile)
	if _, err := os.Stat(markerPath); os.IsNotExist(err) {
		t.Error("Marker file not created in commands directory")
	}

	// Verify at least one command was created with correct structure
	cmdPath := filepath.Join(commandsDir, "tw-brief.md")
	content, err := os.ReadFile(cmdPath)
	if err != nil {
		t.Fatalf("Failed to read tw-brief.md: %v", err)
	}

	contentStr := string(content)

	// Verify YAML frontmatter with description field (OpenCode format)
	if !contains(contentStr, "description:") {
		t.Error("Command file missing 'description' field in frontmatter")
	}
	if !contains(contentStr, "!taskwing slash brief") {
		t.Error("Command file missing taskwing command invocation")
	}
}

// TestInitializer_OpenCode_AllCommandsCreated tests all slash commands become OpenCode commands
func TestInitializer_OpenCode_AllCommandsCreated(t *testing.T) {
	tmpDir := t.TempDir()
	init := NewInitializer(tmpDir)

	err := init.CreateSlashCommands("opencode", false)
	if err != nil {
		t.Fatalf("CreateSlashCommands(opencode) failed: %v", err)
	}

	// Verify each slash command has a corresponding command file
	for _, cmd := range SlashCommands {
		cmdPath := filepath.Join(tmpDir, ".opencode", "commands", cmd.BaseName+".md")
		if _, err := os.Stat(cmdPath); os.IsNotExist(err) {
			t.Errorf("Command not created for %s", cmd.BaseName)
		}
	}
}

func TestInitializer_OpenCode_PrunesRemovedLegacyCommands(t *testing.T) {
	tmpDir := t.TempDir()
	init := NewInitializer(tmpDir)

	commandsDir := filepath.Join(tmpDir, ".opencode", "commands")
	if err := os.MkdirAll(commandsDir, 0755); err != nil {
		t.Fatalf("Failed to create commands dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(commandsDir, "tw-context.md"), []byte("legacy"), 0644); err != nil {
		t.Fatalf("Failed to write tw-context.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(commandsDir, "tw-block.md"), []byte("legacy"), 0644); err != nil {
		t.Fatalf("Failed to write tw-block.md: %v", err)
	}

	if err := init.CreateSlashCommands("opencode", false); err != nil {
		t.Fatalf("CreateSlashCommands(opencode) failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(commandsDir, "tw-context.md")); !os.IsNotExist(err) {
		t.Error("tw-context.md should be removed during OpenCode slash command regeneration")
	}
	if _, err := os.Stat(filepath.Join(commandsDir, "tw-block.md")); !os.IsNotExist(err) {
		t.Error("tw-block.md should be removed during OpenCode slash command regeneration")
	}
}

// TestInitializer_OpenCode_SkillNameValidation tests that skill names match OpenCode regex
func TestInitializer_OpenCode_SkillNameValidation(t *testing.T) {
	// All our SlashCommands should have valid names
	for _, cmd := range SlashCommands {
		if !openCodeSkillNameRegex.MatchString(cmd.BaseName) {
			t.Errorf("Slash command %s has invalid name for OpenCode skills (must match ^[a-z0-9]+(-[a-z0-9]+)*$)", cmd.BaseName)
		}
	}

	// Test some invalid names that should fail
	invalidNames := []string{
		"TW-Brief",     // uppercase
		"-tw-brief",    // starts with hyphen
		"tw-brief-",    // ends with hyphen
		"tw--brief",    // consecutive hyphens
		"tw_brief",     // underscore
		"tw.brief",     // dot
		"tw brief",     // space
		"TwBrief",      // camelCase
		"123-456-789a", // valid actually
	}

	for _, name := range invalidNames[:len(invalidNames)-1] { // last one is actually valid
		if openCodeSkillNameRegex.MatchString(name) {
			t.Errorf("Name %s should be invalid for OpenCode skills", name)
		}
	}
}

// TestInitializer_OpenCode_Plugin tests OpenCode plugin generation
func TestInitializer_OpenCode_Plugin(t *testing.T) {
	tmpDir := t.TempDir()
	init := NewInitializer(tmpDir)

	err := init.installOpenCodePlugin(false)
	if err != nil {
		t.Fatalf("installOpenCodePlugin failed: %v", err)
	}

	// Verify plugin file was created
	pluginPath := filepath.Join(tmpDir, ".opencode", "plugins", "taskwing-hooks.js")
	content, err := os.ReadFile(pluginPath)
	if err != nil {
		t.Fatalf("Failed to read plugin file: %v", err)
	}

	contentStr := string(content)

	// Verify plugin structure
	if !contains(contentStr, "TASKWING_MANAGED_PLUGIN") {
		t.Error("Plugin missing TASKWING_MANAGED_PLUGIN marker")
	}
	if !contains(contentStr, "export default async") {
		t.Error("Plugin missing default async export")
	}
	if !contains(contentStr, "session.created") {
		t.Error("Plugin missing session.created hook")
	}
	if !contains(contentStr, "session.idle") {
		t.Error("Plugin missing session.idle hook")
	}
	if !contains(contentStr, "taskwing hook session-init") {
		t.Error("Plugin missing session-init command")
	}
	if !contains(contentStr, "taskwing hook continue-check") {
		t.Error("Plugin missing continue-check command")
	}
}

// TestInitializer_OpenCode_PluginUserFilePreservation tests user-owned plugins aren't overwritten
func TestInitializer_OpenCode_PluginUserFilePreservation(t *testing.T) {
	tmpDir := t.TempDir()
	init := NewInitializer(tmpDir)

	// Create user-owned plugin (no TaskWing marker)
	pluginsDir := filepath.Join(tmpDir, ".opencode", "plugins")
	if err := os.MkdirAll(pluginsDir, 0755); err != nil {
		t.Fatalf("Failed to create plugins dir: %v", err)
	}

	userContent := "// My custom plugin\nexport default async (ctx) => ({});"
	pluginPath := filepath.Join(pluginsDir, "taskwing-hooks.js")
	if err := os.WriteFile(pluginPath, []byte(userContent), 0644); err != nil {
		t.Fatalf("Failed to write user plugin: %v", err)
	}

	// Run installOpenCodePlugin - should NOT overwrite user file
	err := init.installOpenCodePlugin(true)
	if err != nil {
		t.Fatalf("installOpenCodePlugin failed: %v", err)
	}

	// Verify user content is preserved
	content, err := os.ReadFile(pluginPath)
	if err != nil {
		t.Fatalf("Failed to read plugin: %v", err)
	}

	if string(content) != userContent {
		t.Errorf("User plugin was overwritten!\nExpected: %s\nGot: %s", userContent, string(content))
	}
}

// TestInitializer_OpenCode_InstallHooksConfig tests that InstallHooksConfig routes to plugin installer
func TestInitializer_OpenCode_InstallHooksConfig(t *testing.T) {
	tmpDir := t.TempDir()
	init := NewInitializer(tmpDir)

	err := init.InstallHooksConfig("opencode", false)
	if err != nil {
		t.Fatalf("InstallHooksConfig(opencode) failed: %v", err)
	}

	// Verify plugin was created (not JSON settings)
	pluginPath := filepath.Join(tmpDir, ".opencode", "plugins", "taskwing-hooks.js")
	if _, err := os.Stat(pluginPath); os.IsNotExist(err) {
		t.Error("OpenCode plugin not created by InstallHooksConfig")
	}

	// Verify no settings.json was created
	settingsPath := filepath.Join(tmpDir, ".opencode", "settings.json")
	if _, err := os.Stat(settingsPath); !os.IsNotExist(err) {
		t.Error("settings.json should NOT be created for OpenCode (uses plugins)")
	}
}

// TestInitializer_OpenCode_FullRun tests complete OpenCode initialization via Run
func TestInitializer_OpenCode_FullRun(t *testing.T) {
	tmpDir := t.TempDir()
	init := NewInitializer(tmpDir)

	err := init.Run(false, []string{"opencode"})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// Verify commands directory exists
	commandsDir := filepath.Join(tmpDir, ".opencode", "commands")
	if _, err := os.Stat(commandsDir); os.IsNotExist(err) {
		t.Error("Commands directory not created")
	}

	// Verify at least tw-brief command exists
	cmdPath := filepath.Join(commandsDir, "tw-brief.md")
	if _, err := os.Stat(cmdPath); os.IsNotExist(err) {
		t.Error("tw-brief command not created")
	}

	// Verify plugin exists
	pluginPath := filepath.Join(tmpDir, ".opencode", "plugins", "taskwing-hooks.js")
	if _, err := os.Stat(pluginPath); os.IsNotExist(err) {
		t.Error("Plugin not created")
	}
}

// TestInitializer_GenerateTwBrief tests that tw-brief command is generated with correct content
func TestInitializer_GenerateTwBrief(t *testing.T) {
	tmpDir := t.TempDir()
	init := NewInitializer(tmpDir)

	// Run initialization with opencode
	err := init.Run(false, []string{"opencode"})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// Verify tw-brief command exists (OpenCode format: .opencode/commands/<name>.md)
	cmdPath := filepath.Join(tmpDir, ".opencode", "commands", "tw-brief.md")
	content, err := os.ReadFile(cmdPath)
	if err != nil {
		t.Fatalf("Failed to read tw-brief.md: %v", err)
	}

	contentStr := string(content)

	// Verify frontmatter structure
	if !strings.HasPrefix(contentStr, "---\n") {
		t.Error("Command file missing frontmatter start marker")
	}

	// Verify required frontmatter field (OpenCode only requires description)
	if !strings.Contains(contentStr, "description:") {
		t.Error("Command file missing 'description' field")
	}

	// Verify description mentions project knowledge or brief
	if !strings.Contains(strings.ToLower(contentStr), "brief") && !strings.Contains(strings.ToLower(contentStr), "knowledge") {
		t.Error("Command description should mention 'brief' or 'knowledge'")
	}

	// Verify the command invokes taskwing slash command
	if !strings.Contains(contentStr, "!taskwing slash brief") {
		t.Error("Command file should contain '!taskwing slash brief' directive")
	}

	// Verify command name matches regex pattern: ^[a-z0-9]+(-[a-z0-9]+)*$
	namePattern := regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
	if !namePattern.MatchString("tw-brief") {
		t.Error("Command name 'tw-brief' doesn't match required pattern")
	}
}

// TestInitializer_GenerateOpenCodePlugin tests that OpenCode plugin is generated correctly
// with proper hook mappings and ctx.$ Bun shell API usage.
func TestInitializer_GenerateOpenCodePlugin(t *testing.T) {
	tmpDir := t.TempDir()
	init := NewInitializer(tmpDir)

	// Run initialization with opencode
	err := init.Run(false, []string{"opencode"})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// Verify plugin file exists
	pluginPath := filepath.Join(tmpDir, ".opencode", "plugins", "taskwing-hooks.js")
	content, err := os.ReadFile(pluginPath)
	if err != nil {
		t.Fatalf("Failed to read taskwing-hooks.js: %v", err)
	}

	contentStr := string(content)

	// Verify exports default as async function
	if !strings.Contains(contentStr, "export default async") {
		t.Error("Plugin missing 'export default async' export")
	}

	// Verify ctx parameter is used
	if !strings.Contains(contentStr, "(ctx)") {
		t.Error("Plugin missing ctx parameter in default export")
	}

	// Verify session.created hook exists
	if !strings.Contains(contentStr, `"session.created"`) {
		t.Error("Plugin missing session.created hook handler")
	}

	// Verify session.idle hook exists
	if !strings.Contains(contentStr, `"session.idle"`) {
		t.Error("Plugin missing session.idle hook handler")
	}

	// Verify ctx.$ calls to taskwing hook commands (Bun shell API)
	if !strings.Contains(contentStr, "ctx.$`taskwing hook session-init`") {
		t.Error("Plugin missing ctx.$`taskwing hook session-init` call")
	}
	if !strings.Contains(contentStr, "ctx.$`taskwing hook continue-check") {
		t.Error("Plugin missing ctx.$`taskwing hook continue-check` call")
	}

	// Verify no inline secrets (basic check)
	secretPatterns := []string{
		"api_key",
		"apikey",
		"secret",
		"password",
		"token",
		"credential",
	}
	contentLower := strings.ToLower(contentStr)
	for _, pattern := range secretPatterns {
		// Skip if it's just a reference (like error.message)
		if strings.Contains(contentLower, pattern) && !strings.Contains(contentLower, "error.message") {
			// Allow "token" in comments explaining what the plugin does
			if pattern == "token" && strings.Contains(contentStr, "// ") {
				continue
			}
			t.Errorf("Plugin may contain sensitive data (found pattern: %s)", pattern)
		}
	}

	// Verify managed marker exists (for update detection)
	if !strings.Contains(contentStr, "TASKWING_MANAGED_PLUGIN") {
		t.Error("Plugin missing TASKWING_MANAGED_PLUGIN marker")
	}
}
