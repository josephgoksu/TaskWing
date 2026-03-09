package migration

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/josephgoksu/TaskWing/internal/bootstrap"
)

func TestNoMigrationWhenNotBootstrapped(t *testing.T) {
	dir := t.TempDir()
	warnings, err := CheckAndMigrate(dir, "1.22.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}
	// Version file should not be created
	if _, err := os.Stat(filepath.Join(dir, ".taskwing", "version")); !os.IsNotExist(err) {
		t.Fatal("version file should not exist when not bootstrapped")
	}
}

func TestNoMigrationWhenVersionMatches(t *testing.T) {
	dir := t.TempDir()
	twDir := filepath.Join(dir, ".taskwing")
	if err := os.MkdirAll(twDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(twDir, "version"), []byte("1.22.0"), 0644); err != nil {
		t.Fatal(err)
	}

	warnings, err := CheckAndMigrate(dir, "1.22.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}
}

func TestNoMigrationForDevVersion(t *testing.T) {
	dir := t.TempDir()
	twDir := filepath.Join(dir, ".taskwing")
	if err := os.MkdirAll(twDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(twDir, "version"), []byte("1.21.4"), 0644); err != nil {
		t.Fatal(err)
	}

	warnings, err := CheckAndMigrate(dir, "dev")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}

	// Version file should NOT be updated to "dev"
	stored, _ := os.ReadFile(filepath.Join(twDir, "version"))
	if string(stored) != "1.21.4" {
		t.Fatalf("version should remain 1.21.4, got %q", string(stored))
	}
}

func TestMigrationRunsOnVersionChange(t *testing.T) {
	dir := t.TempDir()
	twDir := filepath.Join(dir, ".taskwing")
	if err := os.MkdirAll(twDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(twDir, "version"), []byte("1.21.4"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a managed Claude commands directory with a legacy tw-ask.md file
	claudeDir := filepath.Join(dir, ".claude", "commands")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatal(err)
	}
	markerContent := "# This directory is managed by TaskWing\n# AI: claude\n# Version: old\n"
	if err := os.WriteFile(filepath.Join(claudeDir, bootstrap.TaskWingManagedFile), []byte(markerContent), 0644); err != nil {
		t.Fatal(err)
	}
	// Write a legacy tw-ask.md file that should get pruned
	if err := os.WriteFile(filepath.Join(claudeDir, "tw-ask.md"), []byte("legacy"), 0644); err != nil {
		t.Fatal(err)
	}

	warnings, err := CheckAndMigrate(dir, "1.22.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// No global MCP warnings expected in test (no real home dir config)
	_ = warnings

	// Version file should be updated
	stored, _ := os.ReadFile(filepath.Join(twDir, "version"))
	if string(stored) != "1.22.0" {
		t.Fatalf("version should be 1.22.0, got %q", string(stored))
	}

	// Legacy tw-ask.md should be removed
	if _, err := os.Stat(filepath.Join(claudeDir, "tw-ask.md")); !os.IsNotExist(err) {
		t.Fatal("legacy tw-ask.md should have been pruned")
	}

	// New namespace directory should exist with regenerated commands
	nsDir := filepath.Join(claudeDir, "taskwing")
	if _, err := os.Stat(nsDir); os.IsNotExist(err) {
		t.Fatal("taskwing/ namespace directory should have been created")
	}
}

func TestMigrationIdempotent(t *testing.T) {
	dir := t.TempDir()
	twDir := filepath.Join(dir, ".taskwing")
	if err := os.MkdirAll(twDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(twDir, "version"), []byte("1.21.4"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create managed Claude config
	claudeDir := filepath.Join(dir, ".claude", "commands")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatal(err)
	}
	markerContent := "# This directory is managed by TaskWing\n# AI: claude\n# Version: old\n"
	if err := os.WriteFile(filepath.Join(claudeDir, bootstrap.TaskWingManagedFile), []byte(markerContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Run migration twice
	_, err := CheckAndMigrate(dir, "1.22.0")
	if err != nil {
		t.Fatalf("first run: %v", err)
	}

	// Second run should be a no-op (version now matches)
	warnings, err := CheckAndMigrate(dir, "1.22.0")
	if err != nil {
		t.Fatalf("second run: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("second run should produce no warnings, got %v", warnings)
	}
}

func TestMigrationWritesVersionOnFirstEncounter(t *testing.T) {
	dir := t.TempDir()
	twDir := filepath.Join(dir, ".taskwing")
	if err := os.MkdirAll(twDir, 0755); err != nil {
		t.Fatal(err)
	}
	// No version file exists (pre-migration bootstrap)

	warnings, err := CheckAndMigrate(dir, "1.22.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}

	// Version file should be stamped
	stored, err := os.ReadFile(filepath.Join(twDir, "version"))
	if err != nil {
		t.Fatal("version file should exist after first encounter")
	}
	if string(stored) != "1.22.0" {
		t.Fatalf("version should be 1.22.0, got %q", string(stored))
	}
}

func TestGlobalMCPWarning(t *testing.T) {
	// checkGlobalMCPLegacy reads from the real home dir, which we can't
	// easily mock. Instead, test the function directly with a temp config.
	dir := t.TempDir()
	configDir := filepath.Join(dir, ".claude")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatal(err)
	}

	config := `{"mcpServers":{"taskwing-mcp":{"command":"taskwing","args":["mcp"]}}}`
	configPath := filepath.Join(configDir, "claude_desktop_config.json")
	if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
		t.Fatal(err)
	}

	// Call the internal function with overridden home
	warnings := checkGlobalMCPLegacyAt(configPath)
	if len(warnings) == 0 {
		t.Fatal("expected warning about legacy server name")
	}
	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d", len(warnings))
	}
}
