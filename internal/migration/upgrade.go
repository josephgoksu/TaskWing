package migration

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/josephgoksu/TaskWing/internal/bootstrap"
	"github.com/josephgoksu/TaskWing/internal/config"
)

// CheckAndMigrate runs a post-upgrade migration if the CLI version has changed
// since the last run in this project. It silently regenerates local configs and
// returns warnings for issues that require manual intervention (e.g., global MCP).
//
// This is designed to be called from PersistentPreRunE and must be:
//   - Sub-millisecond on the happy path (version matches)
//   - Non-fatal on all error paths (never blocks user commands)
func CheckAndMigrate(projectDir, currentVersion string) (warnings []string, err error) {
	taskwingDir := filepath.Join(projectDir, ".taskwing")
	versionFile := filepath.Join(taskwingDir, "version")

	// Not bootstrapped or inaccessible — nothing to migrate
	if _, err := os.Stat(taskwingDir); err != nil {
		return nil, nil
	}

	stored, err := os.ReadFile(versionFile)
	if err != nil {
		// Version file missing (pre-migration bootstrap). Write current and return.
		if werr := os.WriteFile(versionFile, []byte(currentVersion), 0644); werr != nil {
			fmt.Fprintf(os.Stderr, "⚠️  taskwing: could not write version stamp (%v); migration will re-run next time\n", werr)
		}
		return nil, nil
	}

	storedVersion := strings.TrimSpace(string(stored))

	// Happy path: version matches — no-op
	if storedVersion == currentVersion {
		return nil, nil
	}

	// Skip dev builds to avoid constant re-runs during development
	if currentVersion == "dev" {
		return nil, nil
	}

	// --- Version mismatch: run migration ---

	// 1. Silent local migration: regenerate slash commands for managed AIs
	migrateLocalConfigs(projectDir)

	// 2. Global MCP check: warn about legacy server names
	warnings = checkGlobalMCPLegacy()

	// 3. Write current version
	if werr := os.WriteFile(versionFile, []byte(currentVersion), 0644); werr != nil {
		fmt.Fprintf(os.Stderr, "⚠️  taskwing: could not write version stamp (%v); migration will re-run next time\n", werr)
	}

	return warnings, nil
}

// migrateLocalConfigs detects which AIs have managed markers and regenerates
// their slash commands/skills (which internally prunes stale files).
func migrateLocalConfigs(projectDir string) {
	for _, aiName := range bootstrap.ValidAINames() {
		cfg, ok := bootstrap.AIHelperByName(aiName)
		if !ok {
			continue
		}

		// Check if this AI has a managed marker
		managed := false
		if cfg.SingleFile {
			// Single-file AIs (e.g., Copilot) embed the marker in file content.
			filePath := filepath.Join(projectDir, cfg.CommandsDir, cfg.SingleFileName)
			content, err := os.ReadFile(filePath)
			if err == nil && strings.Contains(string(content), "<!-- TASKWING_MANAGED -->") {
				managed = true
			}
		} else {
			markerPath := filepath.Join(projectDir, cfg.CommandsDir, bootstrap.TaskWingManagedFile)
			if _, err := os.Stat(markerPath); err == nil {
				managed = true
			}
		}

		if !managed {
			continue
		}

		// Regenerate (this prunes stale files and creates new ones)
		initializer := bootstrap.NewInitializer(projectDir)
		if err := initializer.CreateSlashCommands(aiName, false); err != nil {
			fmt.Fprintf(os.Stderr, "⚠️  taskwing: could not regenerate %s commands: %v\n", aiName, err)
		}
	}
}

// checkGlobalMCPLegacy reads Claude's global MCP config and warns if legacy
// server names are present.
func checkGlobalMCPLegacy() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	configPath := filepath.Join(home, ".claude", "claude_desktop_config.json")
	return checkGlobalMCPLegacyAt(configPath)
}

// checkGlobalMCPLegacyAt checks a specific config file path for legacy server names.
func checkGlobalMCPLegacyAt(configPath string) []string {
	content, err := os.ReadFile(configPath)
	if err != nil {
		return nil
	}

	var mcpCfg struct {
		MCPServers map[string]json.RawMessage `json:"mcpServers"`
	}
	if err := json.Unmarshal(content, &mcpCfg); err != nil {
		return nil
	}

	var warnings []string
	for name := range mcpCfg.MCPServers {
		if config.IsLegacyServerName(name) {
			warnings = append(warnings, fmt.Sprintf("Global MCP config has legacy server name %q. Run: taskwing doctor --fix --yes", name))
		}
	}

	return warnings
}
