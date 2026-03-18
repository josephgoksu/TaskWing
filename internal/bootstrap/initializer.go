package bootstrap

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/josephgoksu/TaskWing/internal/safepath"
	"github.com/josephgoksu/TaskWing/skills"
)

// Initializer handles the setup of TaskWing project structure and integrations.
type Initializer struct {
	basePath string
	// Version is the CLI version to stamp in .taskwing/version.
	// If empty, no version file is written.
	Version string
}

func NewInitializer(basePath string) *Initializer {
	return &Initializer{basePath: basePath}
}

// ValidAINames returns the list of supported AI assistant names.
func ValidAINames() []string {
	names := make([]string, 0, len(aiCatalog))
	for _, ai := range aiCatalog {
		names = append(names, ai.name)
	}
	return names
}

// AIDisplayNames returns AI display names keyed by id, in canonical catalog form.
func AIDisplayNames() map[string]string {
	display := make(map[string]string, len(aiCatalog))
	for _, ai := range aiCatalog {
		display[ai.name] = ai.displayName
	}
	return display
}

// Run executes the initialization process.
func (i *Initializer) Run(verbose bool, selectedAIs []string) error {
	// 1. Create directory structure
	if err := i.createStructure(verbose); err != nil {
		return err
	}

	if len(selectedAIs) == 0 {
		return nil
	}

	// 2. Setup AI integrations
	return i.setupAIIntegrations(verbose, selectedAIs, true)
}

// RegenerateConfigs regenerates AI configurations without creating directory structure.
// Used in repair mode when project structure is healthy but AI configs need repair.
func (i *Initializer) RegenerateConfigs(verbose bool, targetAIs []string) error {
	if len(targetAIs) == 0 {
		return nil
	}
	return i.setupAIIntegrations(verbose, targetAIs, false)
}

// AdoptionResult contains backup metadata for an unmanaged adoption operation.
type AdoptionResult struct {
	AI           string   `json:"ai"`
	BackupDir    string   `json:"backup_dir"`
	ManifestPath string   `json:"manifest_path"`
	BackedUp     []string `json:"backed_up"`
}

// AdoptAIConfig claims TaskWing-like unmanaged AI config safely by backing up artifacts,
// adding ownership markers, and preparing the AI for canonical regeneration.
func (i *Initializer) AdoptAIConfig(aiName string, verbose bool) (*AdoptionResult, error) {
	cfg, ok := aiHelpers[aiName]
	if !ok {
		return nil, fmt.Errorf("unsupported AI: %s", aiName)
	}

	ts := time.Now().UTC().Format("20060102T150405Z")
	backupDir := filepath.Join(i.basePath, ".taskwing", "backups", "ai-configs", ts, aiName)
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return nil, fmt.Errorf("create backup dir: %w", err)
	}

	paths := adoptionCandidatePaths(i.basePath, aiName, cfg)
	backedUp := make([]string, 0, len(paths))
	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			continue
		}
		rel, relErr := filepath.Rel(i.basePath, p)
		if relErr != nil {
			rel = filepath.Base(p)
		}
		dest := filepath.Join(backupDir, rel)
		if info.IsDir() {
			if err := copyDir(p, dest); err != nil {
				return nil, fmt.Errorf("backup %s: %w", p, err)
			}
		} else {
			if err := copyFile(p, dest); err != nil {
				return nil, fmt.Errorf("backup %s: %w", p, err)
			}
		}
		backedUp = append(backedUp, p)
	}

	if cfg.singleFile {
		if err := i.claimSingleFileOwnership(aiName, cfg); err != nil {
			return nil, err
		}
	} else {
		commandsDir := filepath.Join(i.basePath, cfg.commandsDir)
		if _, err := os.Stat(commandsDir); err == nil {
			if err := os.MkdirAll(commandsDir, 0755); err != nil {
				return nil, fmt.Errorf("ensure commands dir: %w", err)
			}
			markerPath := filepath.Join(commandsDir, TaskWingManagedFile)
			marker := fmt.Sprintf("# This directory is managed by TaskWing\n# Adopted: %s\n# AI: %s\n# Version: %s\n",
				time.Now().UTC().Format(time.RFC3339), aiName, AIToolConfigVersion(aiName))
			if err := os.WriteFile(markerPath, []byte(marker), 0644); err != nil {
				return nil, fmt.Errorf("write marker file: %w", err)
			}
		}
	}

	manifest := map[string]any{
		"ai":        aiName,
		"timestamp": ts,
		"backed_up": backedUp,
	}
	manifestPath := filepath.Join(backupDir, "manifest.json")
	blob, _ := json.MarshalIndent(manifest, "", "  ")
	if err := os.WriteFile(manifestPath, blob, 0644); err != nil {
		return nil, fmt.Errorf("write adoption manifest: %w", err)
	}

	if verbose {
		fmt.Printf("  ✓ Adopted unmanaged config for %s (backup: %s)\n", aiName, backupDir)
	}

	return &AdoptionResult{
		AI:           aiName,
		BackupDir:    backupDir,
		ManifestPath: manifestPath,
		BackedUp:     backedUp,
	}, nil
}

// setupAIIntegrations creates slash commands and hooks for selected AIs.
// If showHeader is true, prints the "Setting up AI integrations" message.
func (i *Initializer) setupAIIntegrations(verbose bool, selectedAIs []string, showHeader bool) error {
	// Validate AI names and filter unknown ones
	var validAIs []string
	for _, ai := range selectedAIs {
		if _, ok := aiHelpers[ai]; ok {
			validAIs = append(validAIs, ai)
		} else if verbose {
			fmt.Fprintf(os.Stderr, "⚠️  Unknown AI assistant '%s' (skipping)\n", ai)
		}
	}

	if len(validAIs) == 0 {
		if verbose {
			fmt.Println("⚠️  No valid AI assistants specified")
		}
		return nil
	}

	if showHeader {
		fmt.Printf("🔧 Setting up AI integrations for: %s\n", strings.Join(validAIs, ", "))
	}

	for _, ai := range validAIs {
		// Create slash commands
		if err := i.CreateSlashCommands(ai, verbose); err != nil {
			return err
		}

		// Install hooks config
		if err := i.InstallHooksConfig(ai, verbose); err != nil {
			fmt.Fprintf(os.Stderr, "⚠️  Failed to install hooks for %s: %v\n", ai, err)
		}

		if showHeader {
			fmt.Printf("   ✓ Created local config for %s\n", ai)
		}
	}

	// Update agent docs once (applies to all: CLAUDE.md, GEMINI.md, AGENTS.md)
	if err := i.updateAgentDocs(verbose); err != nil {
		fmt.Fprintf(os.Stderr, "⚠️  Failed to update agent docs: %v\n", err)
	}

	return nil
}

func (i *Initializer) createStructure(verbose bool) error {
	fmt.Println("📁 Creating .taskwing/ structure...")
	dirs := []string{
		".taskwing",
		".taskwing/memory",
		".taskwing/plans",
	}
	for _, dir := range dirs {
		fullPath := filepath.Join(i.basePath, dir)
		if err := os.MkdirAll(fullPath, 0755); err != nil {
			return fmt.Errorf("create %s: %w", dir, err)
		}
		if verbose {
			fmt.Printf("  ✓ Created %s\n", dir)
		}
	}

	// Track CLI version for post-upgrade migration detection
	if i.Version != "" {
		versionPath := filepath.Join(i.basePath, ".taskwing", "version")
		_ = os.WriteFile(versionPath, []byte(i.Version), 0644)
	}

	return nil
}

// AI Config Definitions (single source of truth for AI integrations)
type aiHelperConfig struct {
	name           string
	displayName    string
	commandsDir    string
	fileExt        string
	singleFile     bool   // If true, generate a single file instead of directory with multiple files
	singleFileName string // Filename for single-file mode (e.g., "copilot-instructions.md")
	skillsDir      bool   // If true, use OpenCode-style skills directory structure
	claudeSkills   bool   // If true, generate .claude/commands/taskwing/ with embedded content
}

var aiCatalog = []aiHelperConfig{
	{name: "claude", displayName: "Claude Code", commandsDir: ".claude/commands", fileExt: ".md", claudeSkills: true},
	{name: "cursor", displayName: "Cursor", commandsDir: ".cursor/rules", fileExt: ".md", singleFile: false},
	{name: "gemini", displayName: "Gemini CLI", commandsDir: ".gemini/commands", fileExt: ".toml", singleFile: false},
	{name: "codex", displayName: "OpenAI Codex", commandsDir: ".codex/commands", fileExt: ".md", singleFile: false},
	{name: "copilot", displayName: "GitHub Copilot", commandsDir: ".github", fileExt: ".md", singleFile: true, singleFileName: "copilot-instructions.md"},
	{name: "opencode", displayName: "OpenCode", commandsDir: ".opencode/commands", fileExt: ".md", singleFile: false, skillsDir: true},
}

// Map AI name to config for O(1) lookups.
var aiHelpers = func() map[string]aiHelperConfig {
	cfg := make(map[string]aiHelperConfig, len(aiCatalog))
	for _, ai := range aiCatalog {
		cfg[ai.name] = ai
	}
	return cfg
}()

// AIHelperInfo exposes read-only AI config fields needed by external packages.
type AIHelperInfo struct {
	CommandsDir    string
	SingleFile     bool
	SingleFileName string
}

// AIHelperByName returns exported config info for the named AI, if it exists.
func AIHelperByName(name string) (AIHelperInfo, bool) {
	cfg, ok := aiHelpers[name]
	if !ok {
		return AIHelperInfo{}, false
	}
	return AIHelperInfo{
		CommandsDir:    cfg.commandsDir,
		SingleFile:     cfg.singleFile,
		SingleFileName: cfg.singleFileName,
	}, true
}

// TaskWingManagedFile is the marker file name written to directories managed by TaskWing.
// This file indicates that TaskWing created and owns the directory, preventing false positives
// when users have similarly named directories for other purposes.
const TaskWingManagedFile = ".taskwing-managed"

// SlashCommand defines a single slash command configuration.
type SlashCommand struct {
	BaseName    string `json:"base_name"`
	SlashCmd    string `json:"slash_cmd"`
	Description string `json:"description"`
}

// SlashCommands is the canonical list of slash commands generated by TaskWing.
// When this list changes, the version hash changes, triggering updates on next bootstrap.
var SlashCommands = []SlashCommand{
	{"taskwing:plan", "plan", "Use when you need to clarify a goal and build an approved execution plan."},
	{"taskwing:next", "next", "Use when you are ready to start the next approved TaskWing task with full context."},
	{"taskwing:done", "done", "Use when implementation is verified and you are ready to complete the current task."},
	{"taskwing:context", "context", "Use when you need the full project knowledge dump for complete architectural context."},
}

// SlashCommandNames returns slash command short names (e.g., "plan", "next", "done"), in canonical order.
func SlashCommandNames() []string {
	names := make([]string, 0, len(SlashCommands))
	for _, cmd := range SlashCommands {
		names = append(names, cmd.SlashCmd)
	}
	return names
}

// MCPTool describes a single MCP tool for documentation generation.
type MCPTool struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// MCPTools is the canonical list of MCP tools exposed by the TaskWing MCP server.
var MCPTools = []MCPTool{
	{"ask", "Search project knowledge (decisions, patterns, constraints)"},
	{"task", "Unified task lifecycle (next, current, start, complete)"},
	{"plan", "Plan management (clarify, decompose, expand, generate, finalize, audit)"},
	{"code", "Code intelligence (find, search, explain, callers, impact, simplify)"},
	{"debug", "Diagnose issues systematically with AI-powered analysis"},
	{"remember", "Store knowledge in project memory"},
}

// MCPToolNames returns MCP tool names in canonical order.
func MCPToolNames() []string {
	names := make([]string, 0, len(MCPTools))
	for _, tool := range MCPTools {
		names = append(names, tool.Name)
	}
	return names
}

// CoreCommand describes a CLI command included in documentation.
type CoreCommand struct {
	Display string `json:"display"` // e.g. "taskwing bootstrap"
}

// CoreCommands is the curated list of CLI commands shown in documentation.
var CoreCommands = []CoreCommand{
	{"taskwing bootstrap"},
	{"taskwing ask \"<query>\""},
	{"taskwing task"},
	{"taskwing mcp"},
	{"taskwing doctor"},
	{"taskwing config"},
	{"taskwing start"},
}

// AIToolConfigVersion computes a version hash for the AI tool configuration.
// The hash includes: command names, slash commands, descriptions, file extension, and config mode.
// When any of these change, the version hash changes, triggering an update.
func AIToolConfigVersion(aiName string) string {
	cfg, ok := aiHelpers[aiName]
	if !ok {
		return ""
	}

	// Create a deterministic representation of the config
	var parts []string
	parts = append(parts, fmt.Sprintf("dir:%s", cfg.commandsDir))
	parts = append(parts, fmt.Sprintf("ext:%s", cfg.fileExt))
	parts = append(parts, fmt.Sprintf("singleFile:%t", cfg.singleFile))
	parts = append(parts, fmt.Sprintf("singleFileName:%s", cfg.singleFileName))
	// Generation format marker: bump this to force regeneration when
	// the content generation method changes (e.g., shell-out to embedded).
	parts = append(parts, "gen:embedded-v1")

	for _, cmd := range SlashCommands {
		parts = append(parts, fmt.Sprintf("cmd:%s:%s:%s", cmd.BaseName, cmd.SlashCmd, cmd.Description))
	}

	for _, tool := range MCPTools {
		parts = append(parts, fmt.Sprintf("mcp:%s:%s", tool.Name, tool.Description))
	}

	for _, cc := range CoreCommands {
		parts = append(parts, fmt.Sprintf("corecmd:%s", cc.Display))
	}

	// Sort for determinism
	sort.Strings(parts)

	// Compute SHA256 hash
	h := sha256.New()
	for _, p := range parts {
		h.Write([]byte(p))
	}

	// Return first 12 chars of hex hash (short but unique enough)
	return hex.EncodeToString(h.Sum(nil))[:12]
}

// ExpectedCommandCount returns the number of expected slash command files.
func ExpectedCommandCount() int {
	return len(SlashCommands)
}

// slashCommandNamespace is the subdirectory name used for namespaced slash commands.
// e.g., .claude/commands/taskwing/plan.md → /taskwing:plan
const slashCommandNamespace = "taskwing"

func expectedSlashCommandFiles(ext string) map[string]struct{} {
	expected := make(map[string]struct{}, len(SlashCommands))
	for _, cmd := range SlashCommands {
		// Files live in the taskwing/ subdirectory, keyed by SlashCmd
		expected[cmd.SlashCmd+ext] = struct{}{}
	}
	return expected
}

// deprecatedSlashCommands lists command names that were removed but whose files
// may still exist from older bootstrap runs. These are always pruned.
var deprecatedSlashCommands = []string{"debug", "simplify", "brief", "ask", "remember", "status", "explain"}

func managedSlashCommandBases() map[string]struct{} {
	managed := make(map[string]struct{}, len(SlashCommands)*2+len(deprecatedSlashCommands)*2)
	for _, cmd := range SlashCommands {
		managed[cmd.SlashCmd] = struct{}{}
		managed["tw-"+cmd.SlashCmd] = struct{}{}
	}
	for _, name := range deprecatedSlashCommands {
		managed[name] = struct{}{}
		managed["tw-"+name] = struct{}{}
	}
	return managed
}

func pruneStaleSlashCommands(commandsDir, ext string, verbose bool) error {
	managedBases := managedSlashCommandBases()

	// Prune legacy flat tw-* files from the commands directory root
	entries, err := os.ReadDir(commandsDir)
	if err != nil {
		return fmt.Errorf("read commands dir: %w", err)
	}

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if filepath.Ext(name) != ext {
			continue
		}
		base := strings.TrimSuffix(name, ext)
		// Only prune files we recognize as managed (including legacy tw-* names)
		if _, known := managedBases[base]; !known {
			continue
		}
		// Legacy tw-* files should always be removed (replaced by taskwing/ subdir)
		if strings.HasPrefix(base, "tw-") {
			fullPath := filepath.Join(commandsDir, name)
			if err := os.Remove(fullPath); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("remove legacy slash command %s: %w", name, err)
			}
			if verbose {
				fmt.Printf("  ✓ Removed legacy command %s\n", name)
			}
		}
	}

	// Prune stale files inside the taskwing/ namespace subdirectory
	nsDir := filepath.Join(commandsDir, slashCommandNamespace)
	nsEntries, err := os.ReadDir(nsDir)
	if err != nil {
		// Subdirectory may not exist yet (first install) — not an error
		return nil
	}

	expected := expectedSlashCommandFiles(ext)
	for _, e := range nsEntries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if filepath.Ext(name) != ext {
			continue
		}
		if _, keep := expected[name]; keep {
			continue
		}
		base := strings.TrimSuffix(name, ext)
		if _, known := managedBases[base]; !known {
			continue
		}

		fullPath := filepath.Join(nsDir, name)
		if err := os.Remove(fullPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove stale slash command %s: %w", name, err)
		}
		if verbose {
			fmt.Printf("  ✓ Removed stale command %s/%s\n", slashCommandNamespace, name)
		}
	}

	return nil
}

func (i *Initializer) CreateSlashCommands(aiName string, verbose bool) error {
	cfg, ok := aiHelpers[aiName]
	if !ok {
		// Unknown AI - skip silently (user may have specified an unsupported AI)
		return nil
	}

	// Handle single-file mode (e.g., GitHub Copilot)
	if cfg.singleFile {
		return i.createSingleFileInstructions(aiName, verbose)
	}

	// Handle Claude Code: .claude/commands/taskwing/<name>.md with embedded content
	if cfg.claudeSkills {
		return i.createClaudeSkills(verbose)
	}

	// Handle OpenCode commands directory structure
	// OpenCode commands: .opencode/commands/<name>.md (flat structure)
	// See: https://opencode.ai/docs/commands/
	if cfg.skillsDir {
		return i.createOpenCodeCommands(aiName, verbose)
	}

	commandsDir := filepath.Join(i.basePath, cfg.commandsDir)
	if err := os.MkdirAll(commandsDir, 0755); err != nil {
		return fmt.Errorf("create commands dir: %w", err)
	}

	// Write marker file to indicate TaskWing manages this directory
	// Include the config version for update detection
	configVersion := AIToolConfigVersion(aiName)
	markerPath := filepath.Join(commandsDir, TaskWingManagedFile)
	markerContent := fmt.Sprintf("# This directory is managed by TaskWing\n# Created: %s\n# AI: %s\n# Version: %s\n",
		time.Now().UTC().Format(time.RFC3339), aiName, configVersion)
	if err := os.WriteFile(markerPath, []byte(markerContent), 0644); err != nil {
		return fmt.Errorf("create marker file: %w", err)
	}

	isTOML := cfg.fileExt == ".toml"

	// Create namespace subdirectory (e.g., .claude/commands/taskwing/)
	// This produces namespaced commands like /taskwing:plan, /taskwing:next
	nsDir := filepath.Join(commandsDir, slashCommandNamespace)
	if err := os.MkdirAll(nsDir, 0755); err != nil {
		return fmt.Errorf("create namespace dir %s: %w", slashCommandNamespace, err)
	}

	for _, cmd := range SlashCommands {
		// Embed skill content directly from the skills package
		body, err := skills.GetBody(cmd.SlashCmd)
		if err != nil {
			return fmt.Errorf("load skill content for %s: %w", cmd.SlashCmd, err)
		}

		var content, fileName string

		if isTOML {
			// Escape triple quotes in body for TOML
			escapedBody := strings.ReplaceAll(body, `"""`, `\"\"\"`)
			fileName = cmd.SlashCmd + ".toml"
			content = fmt.Sprintf("description = %q\n\nprompt = \"\"\"%s\"\"\"\n", cmd.Description, escapedBody)
		} else {
			fileName = cmd.SlashCmd + ".md"
			content = fmt.Sprintf("---\ndescription: %s\n---\n%s\n", cmd.Description, body)
		}

		filePath := filepath.Join(nsDir, fileName)
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			return fmt.Errorf("create %s: %w", fileName, err)
		}
		if verbose {
			fmt.Printf("  ✓ Created %s/%s/%s\n", cfg.commandsDir, slashCommandNamespace, fileName)
		}
	}

	if err := pruneStaleSlashCommands(commandsDir, cfg.fileExt, verbose); err != nil {
		return err
	}

	return nil
}

// createClaudeSkills generates .claude/commands/taskwing/<name>.md with embedded content.
// Embeds the full prompt content directly from the skills package.
// Uses the commands namespace system: .claude/commands/taskwing/next.md -> /taskwing:next
func (i *Initializer) createClaudeSkills(verbose bool) error {
	cfg := aiHelpers["claude"]
	commandsDir := filepath.Join(i.basePath, cfg.commandsDir)
	if err := os.MkdirAll(commandsDir, 0755); err != nil {
		return fmt.Errorf("create commands dir: %w", err)
	}

	// Write marker file
	configVersion := AIToolConfigVersion("claude")
	markerPath := filepath.Join(commandsDir, TaskWingManagedFile)
	markerContent := fmt.Sprintf("# This directory is managed by TaskWing\n# Created: %s\n# AI: claude\n# Version: %s\n",
		time.Now().UTC().Format(time.RFC3339), configVersion)
	if err := os.WriteFile(markerPath, []byte(markerContent), 0644); err != nil {
		return fmt.Errorf("create marker file: %w", err)
	}

	// Create namespace subdirectory: .claude/commands/taskwing/
	// This produces /taskwing:plan, /taskwing:next, etc.
	nsDir := filepath.Join(commandsDir, slashCommandNamespace)
	if err := os.MkdirAll(nsDir, 0755); err != nil {
		return fmt.Errorf("create namespace dir %s: %w", slashCommandNamespace, err)
	}

	for _, cmd := range SlashCommands {
		// Read embedded content from the skills package
		body, err := skills.GetBody(cmd.SlashCmd)
		if err != nil {
			return fmt.Errorf("read embedded skill %s: %w", cmd.SlashCmd, err)
		}

		// Write as command file with frontmatter (description for Claude Code discovery)
		fileName := cmd.SlashCmd + ".md"
		content := fmt.Sprintf("---\ndescription: %s\n---\n%s", cmd.Description, body)

		filePath := filepath.Join(nsDir, fileName)
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			return fmt.Errorf("create %s: %w", fileName, err)
		}
		if verbose {
			fmt.Printf("  ✓ Created %s/%s/%s\n", cfg.commandsDir, slashCommandNamespace, fileName)
		}
	}

	if err := pruneStaleSlashCommands(commandsDir, cfg.fileExt, verbose); err != nil {
		return err
	}

	// Clean up intermediate .claude/skills/tw-*/ directories (from development builds)
	skillsDir := filepath.Join(i.basePath, ".claude", "skills")
	if entries, err := os.ReadDir(skillsDir); err == nil {
		for _, e := range entries {
			if e.IsDir() && strings.HasPrefix(e.Name(), "tw-") {
				p, err := safepath.SafeJoin(skillsDir, e.Name())
				if err != nil {
					continue
				}
				_ = os.RemoveAll(p)
				if verbose {
					fmt.Printf("  ✓ Removed intermediate skill %s\n", e.Name())
				}
			}
		}
		// Remove marker from skills dir if present
		_ = os.Remove(filepath.Join(skillsDir, TaskWingManagedFile))
	}

	return nil
}

// createSingleFileInstructions generates a single instructions file for AIs that use this format
// (like GitHub Copilot's .github/copilot-instructions.md) instead of a directory of slash command files.
func (i *Initializer) createSingleFileInstructions(aiName string, verbose bool) error {
	cfg := aiHelpers[aiName]

	// Ensure parent directory exists
	parentDir := filepath.Join(i.basePath, cfg.commandsDir)
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		return fmt.Errorf("create %s dir: %w", cfg.commandsDir, err)
	}

	// Clean up legacy directory-based config with rollback protection
	// (from older TaskWing versions that incorrectly created directory instead of file)
	legacyDirName := strings.TrimSuffix(cfg.singleFileName, filepath.Ext(cfg.singleFileName))
	legacyDir := filepath.Join(parentDir, legacyDirName)
	var legacyBackup string

	if info, err := os.Stat(legacyDir); err == nil && info.IsDir() {
		// Check if it's TaskWing-managed (has marker) or looks like our old format (has .md files)
		markerPath := filepath.Join(legacyDir, TaskWingManagedFile)
		hasMarker := false
		if _, err := os.Stat(markerPath); err == nil {
			hasMarker = true
		}

		// Also check for old TaskWing versions without marker - look for our command files
		looksLikeOurs := false
		if !hasMarker {
			if entries, err := os.ReadDir(legacyDir); err == nil {
				for _, e := range entries {
					if (strings.HasPrefix(e.Name(), "tw-") || e.Name() == slashCommandNamespace) && (e.IsDir() || strings.HasSuffix(e.Name(), ".md")) {
						looksLikeOurs = true
						break
					}
				}
			}
		}

		if hasMarker || looksLikeOurs {
			// Rename to backup instead of delete for rollback safety
			legacyBackup = legacyDir + ".taskwing-backup"
			if err := os.Rename(legacyDir, legacyBackup); err != nil {
				return fmt.Errorf("backup legacy directory: %w", err)
			}
			if verbose {
				fmt.Printf("  ✓ Backed up legacy %s/ directory\n", legacyDirName)
			}
		}
	}

	// Check if file already exists and is user-managed (C2: don't overwrite user files)
	filePath := filepath.Join(parentDir, cfg.singleFileName)
	if existingContent, err := os.ReadFile(filePath); err == nil {
		if !strings.Contains(string(existingContent), "<!-- TASKWING_MANAGED -->") {
			// User owns this file - do not overwrite
			if verbose {
				fmt.Printf("  ⚠️  Skipping %s - file exists and is user-managed\n", cfg.singleFileName)
			}
			// Clean up backup since we're not proceeding
			if legacyBackup != "" {
				_ = os.Rename(legacyBackup, legacyDir) // Restore backup
			}
			return nil
		}
	}

	configVersion := AIToolConfigVersion(aiName)

	// Build instructions content
	var sb strings.Builder
	sb.WriteString("# Project Instructions for GitHub Copilot\n\n")
	sb.WriteString("<!-- TASKWING_MANAGED -->\n")
	fmt.Fprintf(&sb, "<!-- Version: %s -->\n\n", configVersion)

	sb.WriteString("## TaskWing Integration\n\n")
	sb.WriteString("This project uses TaskWing for AI-assisted development with project memory.\n\n")

	sb.WriteString("### Available Commands\n\n")
	sb.WriteString("The following TaskWing commands are available via MCP (if configured in `.vscode/mcp.json`):\n\n")

	for _, cmd := range SlashCommands {
		fmt.Fprintf(&sb, "- **%s**: %s\n", cmd.BaseName, cmd.Description)
	}

	sb.WriteString("\n### MCP Server Configuration\n\n")
	sb.WriteString("To enable TaskWing MCP tools in VS Code with Copilot, add to `.vscode/mcp.json`:\n\n")
	sb.WriteString("```json\n")
	sb.WriteString(`{
  "servers": {
    "taskwing": {
      "command": "taskwing",
      "args": ["mcp"]
    }
  }
}
`)
	sb.WriteString("```\n\n")

	sb.WriteString("### Usage\n\n")
	sb.WriteString("With MCP configured, you can use TaskWing tools via:\n")
	sb.WriteString("- `@mcp taskwing ask \"query\"` - Search project knowledge\n")
	sb.WriteString("- `@mcp taskwing task {\\\"action\\\":\\\"next\\\"}` - Get next task from plan (session_id auto-derived in MCP session)\n")
	sb.WriteString("- `@mcp taskwing remember \"content\"` - Store knowledge\n")

	if err := os.WriteFile(filePath, []byte(sb.String()), 0644); err != nil {
		// Rollback: restore legacy backup if write fails
		if legacyBackup != "" {
			_ = os.Rename(legacyBackup, legacyDir)
		}
		return fmt.Errorf("create %s: %w", cfg.singleFileName, err)
	}

	// Success - now safe to remove backup
	if legacyBackup != "" {
		_ = os.RemoveAll(legacyBackup)
		if verbose {
			fmt.Printf("  ✓ Removed legacy %s/ directory\n", legacyDirName)
		}
	}

	if verbose {
		fmt.Printf("  ✓ Created %s/%s\n", cfg.commandsDir, cfg.singleFileName)
	}

	return nil
}

// openCodeSkillNameRegex validates OpenCode skill names.
// OpenCode requires skill names to match: ^[a-z0-9]+(-[a-z0-9]+)*$
// Names cannot start/end with hyphens or contain consecutive hyphens.
var openCodeSkillNameRegex = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// createOpenCodeCommands generates OpenCode commands in the flat directory structure.
// OpenCode commands use: .opencode/commands/<name>.md
// Each command file contains YAML frontmatter with "description" field.
// The filename (without .md) becomes the slash command name.
// See: https://opencode.ai/docs/commands/
//
// Command name validation rules (from OpenCode docs):
// - Must match regex: ^[a-z0-9]+(-[a-z0-9]+)*$
// - Lowercase alphanumeric with hyphens as separators
// - Cannot start/end with hyphens
// - Cannot have consecutive hyphens (--)
func (i *Initializer) createOpenCodeCommands(aiName string, verbose bool) error {
	cfg := aiHelpers[aiName]

	// Commands directory: .opencode/commands
	commandsDir := filepath.Join(i.basePath, cfg.commandsDir)
	if err := os.MkdirAll(commandsDir, 0755); err != nil {
		return fmt.Errorf("create commands dir: %w", err)
	}

	// Write marker file to indicate TaskWing manages this directory
	configVersion := AIToolConfigVersion(aiName)
	markerPath := filepath.Join(commandsDir, TaskWingManagedFile)
	markerContent := fmt.Sprintf("# This directory is managed by TaskWing\n# Created: %s\n# AI: %s\n# Version: %s\n",
		time.Now().UTC().Format(time.RFC3339), aiName, configVersion)
	if err := os.WriteFile(markerPath, []byte(markerContent), 0644); err != nil {
		return fmt.Errorf("create marker file: %w", err)
	}

	// Generate a command for each slash command
	// OpenCode format: .opencode/commands/<name>.md with description frontmatter
	// OpenCode uses flat filenames (no subdirectory namespace), so we use SlashCmd directly
	for _, cmd := range SlashCommands {
		// Validate command name against OpenCode requirements (SlashCmd, not BaseName)
		if !openCodeSkillNameRegex.MatchString(cmd.SlashCmd) {
			return fmt.Errorf("invalid OpenCode command name '%s': must match ^[a-z0-9]+(-[a-z0-9]+)*$ (lowercase alphanumeric with hyphens)", cmd.SlashCmd)
		}

		// OpenCode command format: YAML frontmatter + embedded content
		// See: https://opencode.ai/docs/commands/
		body, err := skills.GetBody(cmd.SlashCmd)
		if err != nil {
			return fmt.Errorf("load skill content for %s: %w", cmd.SlashCmd, err)
		}
		content := fmt.Sprintf("---\ndescription: %s\n---\n%s\n", cmd.Description, body)

		// Write <name>.md file directly in commands directory
		filePath := filepath.Join(commandsDir, cmd.SlashCmd+".md")
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			return fmt.Errorf("create %s.md: %w", cmd.SlashCmd, err)
		}

		if verbose {
			fmt.Printf("  ✓ Created %s/%s.md\n", cfg.commandsDir, cmd.SlashCmd)
		}
	}

	if err := pruneStaleSlashCommands(commandsDir, ".md", verbose); err != nil {
		return err
	}

	return nil
}

// Hooks Logic (Moved from cmd/bootstrap.go)
type HooksConfig struct {
	Hooks map[string][]HookMatcher `json:"hooks"`
}
type HookMatcher struct {
	Matcher *HookMatcherConfig `json:"matcher,omitempty"`
	Hooks   []HookCommand      `json:"hooks"`
}
type HookMatcherConfig struct {
	Tools []string `json:"tools,omitempty"`
}
type HookCommand struct {
	Type    string `json:"type"`
	Command string `json:"command"`
	Timeout int    `json:"timeout,omitempty"`
}

func defaultTaskWingHooks() map[string][]HookMatcher {
	// Claude hook docs recommend referencing project scripts via CLAUDE_PROJECT_DIR
	// and note command hooks default to a long timeout when unset.
	// We intentionally avoid short custom timeouts here because Stop hooks may need
	// repo + policy checks and can exceed aggressive limits on larger projects.
	return map[string][]HookMatcher{
		"SessionStart": {
			{
				Hooks: []HookCommand{
					{
						Type:    "command",
						Command: taskWingHookCommand("session-init"),
					},
				},
			},
		},
		"Stop": {
			{
				Hooks: []HookCommand{
					{
						Type:    "command",
						Command: taskWingHookCommand("continue-check --max-tasks=5 --max-minutes=30"),
					},
				},
			},
		},
		"SessionEnd": {
			{
				Hooks: []HookCommand{
					{
						Type:    "command",
						Command: taskWingHookCommand("session-end"),
					},
				},
			},
		},
	}
}

func taskWingHookCommand(args string) string {
	// Prefer project-local binary when present, fall back to PATH binary.
	// Quoted $CLAUDE_PROJECT_DIR follows Claude hook docs for path safety.
	return fmt.Sprintf(`if [ -x "$CLAUDE_PROJECT_DIR/bin/taskwing" ]; then "$CLAUDE_PROJECT_DIR/bin/taskwing" hook %s; else taskwing hook %s; fi`, args, args)
}

func requiredHookCommandSubstr(hookName string) string {
	switch hookName {
	case "SessionStart":
		return "hook session-init"
	case "Stop":
		return "hook continue-check"
	case "SessionEnd":
		return "hook session-end"
	default:
		return ""
	}
}

func hookEventHasRequiredCommand(raw any, required string) bool {
	eventEntries, ok := raw.([]any)
	if !ok {
		return false
	}
	req := strings.ToLower(strings.TrimSpace(required))
	if req == "" {
		return false
	}
	for _, entry := range eventEntries {
		entryMap, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		rawHooks, ok := entryMap["hooks"]
		if !ok {
			continue
		}
		hookCommands, ok := rawHooks.([]any)
		if !ok {
			continue
		}
		for _, cmdEntry := range hookCommands {
			cmdMap, ok := cmdEntry.(map[string]any)
			if !ok {
				continue
			}
			cmdStr, _ := cmdMap["command"].(string)
			if strings.Contains(strings.ToLower(cmdStr), req) {
				return true
			}
		}
	}
	return false
}

func hookMatchersToAny(matchers []HookMatcher) ([]any, error) {
	data, err := json.Marshal(matchers)
	if err != nil {
		return nil, err
	}
	var out []any
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (i *Initializer) InstallHooksConfig(aiName string, verbose bool) error {
	// OpenCode uses JavaScript plugins instead of JSON hooks config
	if aiName == "opencode" {
		return i.installOpenCodePlugin(verbose)
	}

	var settingsPath string
	switch aiName {
	case "claude":
		settingsPath = filepath.Join(i.basePath, ".claude", "settings.json")
	case "codex":
		settingsPath = filepath.Join(i.basePath, ".codex", "settings.json")
	default:
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(settingsPath), 0755); err != nil {
		return fmt.Errorf("create settings dir: %w", err)
	}

	desiredHooks := defaultTaskWingHooks()

	config := map[string]any{
		"hooks": desiredHooks,
	}
	changed := true

	if content, err := os.ReadFile(settingsPath); err == nil {
		changed = false
		var existing map[string]any
		if err := json.Unmarshal(content, &existing); err != nil {
			// File exists but contains invalid JSON - don't overwrite, warn user
			return fmt.Errorf("existing %s contains invalid JSON (please fix manually): %w", settingsPath, err)
		}
		config = existing

		hooksRaw, hasHooks := config["hooks"]
		if !hasHooks {
			config["hooks"] = desiredHooks
			changed = true
		} else {
			hooksMap, ok := hooksRaw.(map[string]any)
			if !ok {
				return fmt.Errorf("existing %s has invalid hooks format (expected object)", settingsPath)
			}
			for hookName, hookConfig := range desiredHooks {
				existingHook, exists := hooksMap[hookName]
				if !exists {
					hooksMap[hookName] = hookConfig
					changed = true
					continue
				}

				requiredSubstr := requiredHookCommandSubstr(hookName)
				if requiredSubstr == "" || hookEventHasRequiredCommand(existingHook, requiredSubstr) {
					continue
				}

				existingList, ok := existingHook.([]any)
				if !ok {
					return fmt.Errorf("existing %s has invalid %s hook format (expected array)", settingsPath, hookName)
				}
				desiredList, err := hookMatchersToAny(hookConfig)
				if err != nil {
					return fmt.Errorf("convert desired %s hook config: %w", hookName, err)
				}
				hooksMap[hookName] = append(existingList, desiredList...)
				changed = true
			}
		}
	}

	if !changed {
		if verbose {
			fmt.Printf("  ℹ️  Hooks already configured in %s\n", settingsPath)
		}
		return nil
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal hooks config: %w", err)
	}

	if err := os.WriteFile(settingsPath, data, 0644); err != nil {
		return fmt.Errorf("write hooks config: %w", err)
	}

	if verbose {
		fmt.Printf("  ✓ Created hooks config: %s\n", settingsPath)
		fmt.Println("  ℹ️  If Claude Code is already running, review/reload hooks from /hooks for changes to take effect.")
	}
	return nil
}

// openCodePluginTemplate is the JavaScript plugin template for OpenCode hooks.
// OpenCode uses Bun-based plugins in .opencode/plugins/ that export hook handlers.
//
// Available hooks from OpenCode docs:
// - session.created: Called when a new session starts (like Claude's SessionStart)
// - session.compacted: Called when session context is summarized
// - session.idle: Called when session becomes idle (like Claude's Stop hook)
//
// The plugin uses ctx.$ (Bun shell API) to execute taskwing CLI commands.
const openCodePluginTemplate = `// TaskWing Plugin for OpenCode
// This plugin integrates TaskWing's autonomous task execution with OpenCode.
// Generated by TaskWing - do not edit manually (will be overwritten on bootstrap).
//
// TASKWING_MANAGED_PLUGIN
// Version: %s

export default async (ctx) => ({
  // session.created: Called when a new OpenCode session starts
  // Equivalent to Claude Code's SessionStart hook
  "session.created": async (event) => {
    try {
      await ctx.$` + "`taskwing hook session-init`" + `;
      ctx.client.app.log("info", "TaskWing session initialized");
    } catch (error) {
      ctx.client.app.log("warn", ` + "`TaskWing session-init failed: ${error.message}`" + `);
    }
  },

  // session.idle: Called when the session becomes idle (task completed)
  // Equivalent to Claude Code's Stop hook - checks if should continue to next task
  "session.idle": async (event) => {
    try {
      const result = await ctx.$` + "`taskwing hook continue-check --max-tasks=5 --max-minutes=30`" + `;
      if (result.exitCode === 0 && result.stdout.includes("CONTINUE")) {
        ctx.client.app.log("info", "TaskWing: Continuing to next task");
        // OpenCode will pick up the next task context from stdout
      }
    } catch (error) {
      ctx.client.app.log("debug", ` + "`TaskWing continue-check: ${error.message}`" + `);
    }
  },

  // session.compacted: Called when session context is being summarized
  // Can be used to preserve important TaskWing state during compaction
  "session.compacted": async (event) => {
    ctx.client.app.log("debug", "TaskWing: Session compacted");
  }
});
`

// installOpenCodePlugin creates the TaskWing hooks plugin for OpenCode.
// OpenCode plugins are JavaScript files in .opencode/plugins/ that export hook handlers.
// Unlike Claude/Codex which use JSON settings, OpenCode requires actual JS code.
func (i *Initializer) installOpenCodePlugin(verbose bool) error {
	pluginsDir := filepath.Join(i.basePath, ".opencode", "plugins")
	if err := os.MkdirAll(pluginsDir, 0755); err != nil {
		return fmt.Errorf("create plugins dir: %w", err)
	}

	pluginPath := filepath.Join(pluginsDir, "taskwing-hooks.js")
	configVersion := AIToolConfigVersion("opencode")

	// Check if plugin already exists and is user-managed
	if existingContent, err := os.ReadFile(pluginPath); err == nil {
		if !strings.Contains(string(existingContent), "TASKWING_MANAGED_PLUGIN") {
			// User owns this file - do not overwrite
			if verbose {
				fmt.Printf("  ⚠️  Skipping taskwing-hooks.js - file exists and is user-managed\n")
			}
			return nil
		}
	}

	// Generate plugin content with version
	content := fmt.Sprintf(openCodePluginTemplate, configVersion)

	if err := os.WriteFile(pluginPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("write plugin: %w", err)
	}

	if verbose {
		fmt.Printf("  ✓ Created OpenCode plugin: .opencode/plugins/taskwing-hooks.js\n")
	}
	return nil
}

// Markers for TaskWing-managed documentation section (HTML comments, invisible when rendered)
const (
	taskwingDocMarkerStart = "<!-- TASKWING_DOCS_START -->"
	taskwingDocMarkerEnd   = "<!-- TASKWING_DOCS_END -->"
)

// taskwingDocSectionHeader is the static top portion of the documentation block.
// The behavioral instructions at the top tell AI tools WHEN to use TaskWing MCP,
// not just what's available. This is what makes the AI proactively use the tools.
const taskwingDocSectionHeader = `

## TaskWing Integration

This project uses TaskWing for architectural knowledge management. You have access to TaskWing MCP tools.

### TaskWing Workflow Contract v1
1. No implementation before a clarified and approved plan/task checkpoint.
2. No completion claim without fresh verification evidence.
3. No debug fix proposal without root-cause evidence.

### MCP Tools (use directly, no skill needed)
- ` + "`ask`" + ` -- Search project knowledge before modifying unfamiliar code.
- ` + "`remember`" + ` -- Persist a decision or pattern for future sessions.
- ` + "`code`" + ` -- Find symbols, explain call graphs, analyze impact, simplify code.
- ` + "`debug`" + ` -- Diagnose issues with root-cause analysis.
- ` + "`task`" + ` with action=current -- Check current task status.

**When to use TaskWing MCP tools:**
- Before modifying unfamiliar code: call ` + "`ask`" + ` to check for relevant decisions, constraints, and patterns
- Before planning multi-step work: call ` + "`plan`" + ` with action=clarify to get a structured plan
- When asked about architecture, tech stack, or "why" questions: call ` + "`ask`" + ` with answer=true
- After making an architectural decision: call ` + "`remember`" + ` to persist it for future sessions
- To understand a symbol's role and callers: call ` + "`code`" + ` with action=explain

**Do not** grep or read files to answer architecture questions when TaskWing MCP is available. The knowledge graph has pre-extracted, verified decisions with evidence.

`

// taskwingDocSectionFooter is the static bottom portion of the documentation block.
const taskwingDocSectionFooter = `
### Autonomous Task Execution (Hooks)

TaskWing integrates with Claude Code's hook system for autonomous plan execution:

~~~bash
taskwing hook session-init      # Initialize session tracking (SessionStart hook)
taskwing hook continue-check    # Check if should continue to next task (Stop hook)
taskwing hook session-end       # Cleanup session (SessionEnd hook)
taskwing hook status            # View current session state
~~~

Circuit breakers prevent runaway execution:
- --max-tasks=5 stops after N tasks for human review.
- --max-minutes=30 stops after N minutes.

Configuration in .claude/settings.json enables auto-continuation through plans.
Hook commands prefer $CLAUDE_PROJECT_DIR/bin/taskwing and fall back to taskwing in PATH.
If Claude Code is already running, use /hooks to review or reload hook changes.

`

// buildTaskwingDocSection assembles the complete TaskWing documentation block
// from the three registries (SlashCommands, CoreCommands, MCPTools).
// This is the single source of truth for documentation stamped into CLAUDE.md,
// AGENTS.md, and GEMINI.md during bootstrap.
func buildTaskwingDocSection() string {
	var sb strings.Builder

	sb.WriteString(taskwingDocMarkerStart)
	sb.WriteString(taskwingDocSectionHeader)

	// Slash Commands — generated from SlashCommands registry
	sb.WriteString("### Slash Commands\n")
	for _, cmd := range SlashCommands {
		fmt.Fprintf(&sb, "- /%s - %s\n", cmd.BaseName, cmd.Description)
	}

	// Core Commands — generated from CoreCommands registry
	sb.WriteString("\n### Core Commands\n\n")
	sb.WriteString("<!-- TASKWING_COMMANDS_START -->\n")
	for _, cc := range CoreCommands {
		fmt.Fprintf(&sb, "- %s\n", cc.Display)
	}
	sb.WriteString("<!-- TASKWING_COMMANDS_END -->\n")

	// MCP Tools — generated from MCPTools registry
	sb.WriteString("\n### MCP Tools (Canonical Contract)\n\n")
	sb.WriteString("<!-- TASKWING_MCP_TOOLS_START -->\n")
	sb.WriteString("| Tool | Description |\n")
	sb.WriteString("|------|-------------|\n")
	for _, tool := range MCPTools {
		fmt.Fprintf(&sb, "| %s | %s |\n", tool.Name, tool.Description)
	}
	sb.WriteString("<!-- TASKWING_MCP_TOOLS_END -->\n")

	sb.WriteString(taskwingDocSectionFooter)
	sb.WriteString(taskwingDocMarkerEnd)

	return sb.String()
}

func (i *Initializer) updateAgentDocs(verbose bool) error {
	// Always update all three agent doc files: CLAUDE.md, GEMINI.md, AGENTS.md
	filesToUpdate := []string{"CLAUDE.md", "GEMINI.md", "AGENTS.md"}

	// Build doc section once from registries (single source of truth)
	docSection := buildTaskwingDocSection()

	for _, fileName := range filesToUpdate {
		filePath := filepath.Join(i.basePath, fileName)
		content, err := os.ReadFile(filePath)
		if err != nil {
			// File doesn't exist - skip silently
			continue
		}

		contentStr := string(content)
		var newContent string
		action := ""

		// Check if markers exist (previous TaskWing installation with markers)
		startIdx := strings.Index(contentStr, taskwingDocMarkerStart)
		endIdx := strings.Index(contentStr, taskwingDocMarkerEnd)

		// Validate marker state
		hasStartMarker := startIdx != -1
		hasEndMarker := endIdx != -1

		if hasStartMarker && hasEndMarker && endIdx > startIdx {
			// Valid markers - replace content between them
			before := contentStr[:startIdx]
			after := contentStr[endIdx+len(taskwingDocMarkerEnd):]
			newContent = before + docSection + after
			action = "updated"
		} else if hasStartMarker != hasEndMarker {
			// Partial markers - warn and skip to avoid corruption
			fmt.Fprintf(os.Stderr, "  ⚠️  %s has incomplete TaskWing markers - skipping (please fix manually)\n", fileName)
			continue
		} else if legacyStart, legacyEnd := findLegacyTaskWingSection(contentStr); legacyStart != -1 {
			// Legacy content without markers - replace with new marked section
			before := contentStr[:legacyStart]
			after := ""
			if legacyEnd < len(contentStr) {
				after = contentStr[legacyEnd:]
			}
			newContent = strings.TrimRight(before, "\n") + "\n" + docSection + after
			action = "migrated"
		} else {
			// No existing TaskWing content - append
			newContent = strings.TrimRight(contentStr, "\n") + "\n" + docSection
			action = "added"
		}

		if action != "" && newContent != contentStr {
			if err := os.WriteFile(filePath, []byte(newContent), 0644); err != nil {
				return fmt.Errorf("update %s: %w", fileName, err)
			}
			if verbose {
				fmt.Printf("  ✓ TaskWing docs %s in %s\n", action, fileName)
			}
		} else if verbose {
			fmt.Printf("  ℹ️  TaskWing docs unchanged in %s\n", fileName)
		}
	}
	return nil
}

// findLegacyTaskWingSection finds legacy TaskWing content without markers.
// Returns (startIndex, endIndex) or (-1, -1) if not found.
// Uses case-insensitive matching and handles multiple heading levels.
func findLegacyTaskWingSection(content string) (int, int) {
	contentLower := strings.ToLower(content)

	// Find "## taskwing integration" case-insensitively
	legacyStart := strings.Index(contentLower, "## taskwing integration")
	if legacyStart == -1 {
		return -1, -1
	}

	// Find the end of TaskWing section by looking for next heading at same or higher level
	// This handles ## headings and # headings
	afterSection := content[legacyStart+len("## taskwing integration"):]

	// Look for next heading (# or ##) that would end our section
	legacyEnd := len(content) // Default to end of file
	lines := strings.Split(afterSection, "\n")
	offset := legacyStart + len("## taskwing integration")

	for _, line := range lines {
		offset += len(line) + 1 // +1 for newline
		trimmed := strings.TrimLeft(line, " \t")
		// Stop at # or ## headings (but not ### which are subsections)
		if strings.HasPrefix(trimmed, "## ") || (strings.HasPrefix(trimmed, "# ") && !strings.HasPrefix(trimmed, "## ")) {
			legacyEnd = offset - len(line) - 1 // Point to before the newline
			break
		}
	}

	return legacyStart, legacyEnd
}

func adoptionCandidatePaths(basePath, aiName string, cfg aiHelperConfig) []string {
	paths := make([]string, 0, 5)
	if cfg.singleFile {
		paths = append(paths, filepath.Join(basePath, cfg.commandsDir, cfg.singleFileName))
	} else {
		paths = append(paths, filepath.Join(basePath, cfg.commandsDir))
	}
	switch aiName {
	case "claude", "codex":
		paths = append(paths, filepath.Join(basePath, "."+aiName, "settings.json"))
	case "opencode":
		paths = append(paths,
			filepath.Join(basePath, ".opencode", "plugins", "taskwing-hooks.js"),
			filepath.Join(basePath, "opencode.json"),
		)
	case "gemini":
		paths = append(paths, filepath.Join(basePath, ".gemini", "settings.json"))
	case "cursor":
		paths = append(paths, filepath.Join(basePath, ".cursor", "mcp.json"))
	case "copilot":
		paths = append(paths, filepath.Join(basePath, ".vscode", "mcp.json"))
	}
	return paths
}

func (i *Initializer) claimSingleFileOwnership(aiName string, cfg aiHelperConfig) error {
	filePath := filepath.Join(i.basePath, cfg.commandsDir, cfg.singleFileName)
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil
	}
	if strings.Contains(string(content), "<!-- TASKWING_MANAGED -->") {
		return nil
	}
	version := AIToolConfigVersion(aiName)
	prefix := fmt.Sprintf("<!-- TASKWING_MANAGED -->\n<!-- Version: %s -->\n", version)
	newContent := prefix + string(content)
	if err := os.WriteFile(filePath, []byte(newContent), 0644); err != nil {
		return fmt.Errorf("claim ownership for %s: %w", filePath, err)
	}
	return nil
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}

func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(src, path)
		if relErr != nil {
			return relErr
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		return copyFile(path, target)
	})
}
