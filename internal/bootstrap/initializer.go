package bootstrap

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Initializer handles the setup of TaskWing project structure and integrations.
type Initializer struct {
	basePath string
}

func NewInitializer(basePath string) *Initializer {
	return &Initializer{basePath: basePath}
}

// ValidAINames returns the list of supported AI assistant names.
func ValidAINames() []string {
	names := make([]string, 0, len(aiHelpers))
	for name := range aiHelpers {
		names = append(names, name)
	}
	return names
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
		if err := i.createSlashCommands(ai, verbose); err != nil {
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
	return nil
}

// AI Config Definitions (Moved from cmd/bootstrap.go)
type aiHelperConfig struct {
	commandsDir string
	fileExt     string
}

// Map AI name to config
var aiHelpers = map[string]aiHelperConfig{
	"claude":  {".claude/commands", ".md"},
	"cursor":  {".cursor/rules", ".md"},
	"gemini":  {".gemini/commands", ".toml"},
	"codex":   {".codex/commands", ".md"},
	"copilot": {".github/copilot-instructions", ".md"}, // Simplified mapping
}

func (i *Initializer) createSlashCommands(aiName string, verbose bool) error {
	cfg, ok := aiHelpers[aiName]
	if !ok {
		// Unknown AI - skip silently (user may have specified an unsupported AI)
		return nil
	}

	commandsDir := filepath.Join(i.basePath, cfg.commandsDir)
	if err := os.MkdirAll(commandsDir, 0755); err != nil {
		return fmt.Errorf("create commands dir: %w", err)
	}

	commands := []struct {
		baseName    string
		slashCmd    string
		description string
	}{
		{"taskwing", "taskwing", "Fetch project architecture context from TaskWing"},
		{"tw-next", "next", "Start next TaskWing task with full context"},
		{"tw-done", "done", "Complete current task with architecture-aware summary"},
		{"tw-context", "context", "Fetch additional context for current task"},
		{"tw-status", "status", "Show current task status"},
		{"tw-block", "block", "Mark current task as blocked"},
		{"tw-plan", "plan", "Create development plan with goal"},
	}

	isTOML := cfg.fileExt == ".toml"

	for _, cmd := range commands {
		var content, fileName string

		if isTOML {
			fileName = cmd.baseName + ".toml"
			content = fmt.Sprintf(`description = "%s"

prompt = """!{taskwing slash %s}"""
`, cmd.description, cmd.slashCmd)
		} else {
			fileName = cmd.baseName + ".md"
			content = fmt.Sprintf(`---
description: %s
---
!taskwing slash %s
`, cmd.description, cmd.slashCmd)
		}

		filePath := filepath.Join(commandsDir, fileName)
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			return fmt.Errorf("create %s: %w", fileName, err)
		}
		if verbose {
			fmt.Printf("  ✓ Created %s/%s\n", cfg.commandsDir, fileName)
		}
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

func (i *Initializer) InstallHooksConfig(aiName string, verbose bool) error {
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

	if content, err := os.ReadFile(settingsPath); err == nil {
		var existing map[string]any
		if err := json.Unmarshal(content, &existing); err != nil {
			// File exists but contains invalid JSON - don't overwrite, warn user
			return fmt.Errorf("existing %s contains invalid JSON (please fix manually): %w", settingsPath, err)
		}
		if _, hasHooks := existing["hooks"]; hasHooks {
			if verbose {
				fmt.Printf("  ℹ️  Hooks already configured in %s\n", settingsPath)
			}
			return nil
		}
	}

	// Hook timeout values (in seconds):
	// - SessionStart (10s): Quick initialization, only creates session file
	// - Stop (15s): May need to query plan state, fetch next task context
	// Users can adjust these in the generated settings.json if needed.
	config := HooksConfig{
		Hooks: map[string][]HookMatcher{
			"SessionStart": {
				{
					Hooks: []HookCommand{
						{
							Type:    "command",
							Command: "taskwing hook session-init",
							Timeout: 10,
						},
					},
				},
			},
			"Stop": {
				{
					Hooks: []HookCommand{
						{
							Type:    "command",
							Command: "taskwing hook continue-check --max-tasks=5 --max-minutes=30",
							Timeout: 15,
						},
					},
				},
			},
		},
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
	}
	return nil
}

// Markers for TaskWing-managed documentation section (HTML comments, invisible when rendered)
const (
	taskwingDocMarkerStart = "<!-- TASKWING_DOCS_START -->"
	taskwingDocMarkerEnd   = "<!-- TASKWING_DOCS_END -->"
)

// taskwingDocSection is the complete TaskWing documentation block with markers
const taskwingDocSection = taskwingDocMarkerStart + `

## TaskWing Integration

TaskWing provides project memory for AI assistants via MCP tools and slash commands.

### Slash Commands
- ` + "`/taskwing`" + ` - Fetch full project context (decisions, patterns, constraints)
- ` + "`/tw-next`" + ` - Start next task with architecture context
- ` + "`/tw-done`" + ` - Complete current task with summary
- ` + "`/tw-plan`" + ` - Create development plan from goal
- ` + "`/tw-status`" + ` - Show current task status

### MCP Tools
| Tool | Description |
|------|-------------|
| ` + "`recall`" + ` | Retrieve project knowledge (decisions, patterns, constraints) |
| ` + "`task_next`" + ` | Get next pending task from plan |
| ` + "`task_start`" + ` | Claim and start a specific task |
| ` + "`task_complete`" + ` | Mark task as completed |
| ` + "`plan_clarify`" + ` | Refine goal with clarifying questions |
| ` + "`plan_generate`" + ` | Generate plan with tasks |
| ` + "`remember`" + ` | Store knowledge in project memory |

### CLI Commands
` + "```bash" + `
tw bootstrap        # Initialize project memory (first-time setup)
tw context "query"  # Search knowledge semantically
tw add "content"    # Add knowledge to memory
tw plan new "goal"  # Create development plan
tw task list        # List tasks from active plan
` + "```" + `

### Autonomous Task Execution (Hooks)

TaskWing integrates with Claude Code's hook system for autonomous plan execution:

` + "```bash" + `
taskwing hook session-init      # Initialize session tracking (SessionStart hook)
taskwing hook continue-check    # Check if should continue to next task (Stop hook)
taskwing hook session-end       # Cleanup session (SessionEnd hook)
taskwing hook status            # View current session state
` + "```" + `

**Circuit breakers** prevent runaway execution:
- ` + "`--max-tasks=5`" + ` - Stop after N tasks for human review
- ` + "`--max-minutes=30`" + ` - Stop after N minutes

Configuration in ` + "`.claude/settings.json`" + ` enables auto-continuation through plans.

` + taskwingDocMarkerEnd

func (i *Initializer) updateAgentDocs(verbose bool) error {
	// Always update all three agent doc files: CLAUDE.md, GEMINI.md, AGENTS.md
	filesToUpdate := []string{"CLAUDE.md", "GEMINI.md", "AGENTS.md"}

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
			newContent = before + taskwingDocSection + after
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
			newContent = strings.TrimRight(before, "\n") + "\n" + taskwingDocSection + after
			action = "migrated"
		} else {
			// No existing TaskWing content - append
			newContent = strings.TrimRight(contentStr, "\n") + "\n" + taskwingDocSection
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
