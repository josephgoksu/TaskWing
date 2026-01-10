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
	// Note: We need access to aiConfigs from `install_helpers.go` (if we moved it)
	// OR we need to replicate that logic.
	// For now, assuming the caller passes the AI keys directly or we move helper logic here.
	// The CLI `cmd/mcp_install.go` has installation logic. The `cmd/bootstrap.go` has slash command creation.
	// We should move `createSlashCommands` and the `aiConfig` struct here or make them shared.

	// Since we can't easily move `aiConfig` from `cmd` without refactoring `cmd/mcp_install.go` too,
	// let's define the necessary config here for slash commands.

	for _, ai := range selectedAIs {
		// Create slash commands
		if err := i.createSlashCommands(ai, verbose); err != nil {
			return err
		}

		// Install hooks config
		if err := i.InstallHooksConfig(ai, verbose); err != nil {
			fmt.Fprintf(os.Stderr, "⚠️  Failed to install hooks for %s: %v\n", ai, err)
		}

		// Update agent docs
		if err := i.updateAgentDocs(ai, verbose); err != nil {
			fmt.Fprintf(os.Stderr, "⚠️  Failed to update docs for %s: %v\n", ai, err)
		}
	}

	// Note: Actual MCP installation (binary copy) is handled by `cmd/mcp_install.go`.
	// Bootstrap calls it via `installClaude`, etc.
	// Those are currently CLI helpers. A full refactor would move them to `internal/install`.
	// For this step, we'll focus on the config/file generation parts handled by bootstrap.

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
		// Fallback or skip if unknown (Copilot handles differently in old code?)
		if aiName == "copilot" {
			// Copilot logic was specific in bootstrap.go?
			// Checking previous file content... it was handled generally if in the map.
			return nil
		}
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
		if err := json.Unmarshal(content, &existing); err == nil {
			if _, hasHooks := existing["hooks"]; hasHooks {
				if verbose {
					fmt.Printf("  ℹ️  Hooks already configured in %s\n", settingsPath)
				}
				return nil
			}
		}
	}

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

const hooksDocSection = `

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
`

const taskwingUsageDocSection = `

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
`

func (i *Initializer) updateAgentDocs(aiName string, verbose bool) error {
	var filesToCheck []string
	switch aiName {
	case "claude":
		filesToCheck = []string{"CLAUDE.md", "AGENTS.md"}
	case "codex":
		filesToCheck = []string{"AGENTS.md", "CODEX.md"}
	case "gemini":
		filesToCheck = []string{"GEMINI.md", "AGENTS.md"}
	default:
		filesToCheck = []string{"AGENTS.md"}
	}

	for _, fileName := range filesToCheck {
		filePath := filepath.Join(i.basePath, fileName)
		content, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		contentStr := string(content)
		updated := false

		// Add TaskWing usage docs if not present
		if !strings.Contains(contentStr, "TaskWing Integration") &&
			!strings.Contains(contentStr, "### MCP Tools") {
			contentStr += taskwingUsageDocSection
			updated = true
			if verbose {
				fmt.Printf("  ✓ Added TaskWing usage docs to %s\n", fileName)
			}
		} else if verbose {
			fmt.Printf("  ℹ️  TaskWing usage docs already in %s\n", fileName)
		}

		// Add hooks docs if not present
		if !strings.Contains(contentStr, "Autonomous Task Execution") &&
			!strings.Contains(contentStr, "tw hook session-init") {
			contentStr += hooksDocSection
			updated = true
			if verbose {
				fmt.Printf("  ✓ Added hooks docs to %s\n", fileName)
			}
		} else if verbose {
			fmt.Printf("  ℹ️  Hooks docs already in %s\n", fileName)
		}

		if updated {
			if err := os.WriteFile(filePath, []byte(contentStr), 0644); err != nil {
				return fmt.Errorf("update %s: %w", fileName, err)
			}
		}

		// Only update the first found file
		break
	}
	return nil
}
