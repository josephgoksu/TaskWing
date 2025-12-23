/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/josephgoksu/TaskWing/internal/llm"
	"github.com/josephgoksu/TaskWing/internal/ui"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// AI assistant configurations
type aiConfig struct {
	name        string
	displayName string
	commandsDir string
}

// Ordered list for consistent display
var aiConfigOrder = []string{"claude", "cursor", "copilot", "gemini"}

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
}

var initCmd = &cobra.Command{
	Use:   "init [path]",
	Short: "Initialize TaskWing in a project",
	Long: `Initialize TaskWing in a project directory.

Creates the .taskwing/ structure and sets up slash commands
for your AI assistant.

Examples:
  tw init                    # Initialize in current directory
  tw init .                  # Same as above
  tw init my-project         # Create and initialize new directory
  tw init . --ai claude      # Skip AI selection prompt`,
	Args: cobra.MaximumNArgs(1),
	RunE: runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)
	initCmd.Flags().String("ai", "", "AI assistant preset (claude, cursor, copilot, gemini)")
}

func runInit(cmd *cobra.Command, args []string) error {
	// Determine target directory
	targetDir := "."
	if len(args) > 0 {
		targetDir = args[0]
	}

	// Resolve absolute path
	absPath, err := filepath.Abs(targetDir)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %w", err)
	}

	// Print banner
	printInitBanner()

	// Check if directory exists
	if targetDir != "." {
		if _, err := os.Stat(absPath); os.IsNotExist(err) {
			fmt.Printf("Creating directory: %s\n", absPath)
			if err := os.MkdirAll(absPath, 0755); err != nil {
				return fmt.Errorf("failed to create directory: %w", err)
			}
		}
	}

	// Check for existing .taskwing
	taskwingDir := filepath.Join(absPath, ".taskwing")
	if _, err := os.Stat(taskwingDir); err == nil {
		fmt.Println("⚠️  TaskWing already initialized in this directory")
		fmt.Print("Continue and overwrite? [y/N]: ")
		reader := bufio.NewReader(os.Stdin)
		response, _ := reader.ReadString('\n')
		response = strings.TrimSpace(strings.ToLower(response))
		if response != "y" && response != "yes" {
			fmt.Println("Aborted.")
			return nil
		}
	}

	// Get AI selection
	aiFlag, _ := cmd.Flags().GetString("ai")
	var selectedAIs []string
	if aiFlag != "" {
		// Single AI from flag
		selectedAIs = []string{aiFlag}
	} else {
		selectedAIs = promptAISelection()
		if len(selectedAIs) == 0 {
			fmt.Println("Cancelled.")
			return nil
		}
	}

	// Validate all selections
	for _, ai := range selectedAIs {
		if _, ok := aiConfigs[ai]; !ok {
			return fmt.Errorf("unknown AI assistant: %s (available: claude, cursor, copilot, gemini)", ai)
		}
	}

	verbose := viper.GetBool("verbose")

	// Create .taskwing structure
	fmt.Println("\n📁 Creating .taskwing/ structure...")
	if err := createTaskwingStructure(absPath, verbose); err != nil {
		return err
	}

	// Create AI-specific commands for each selected AI
	for _, ai := range selectedAIs {
		aiCfg := aiConfigs[ai]
		fmt.Printf("📝 Creating %s slash commands...\n", aiCfg.displayName)
		if err := createSlashCommands(absPath, aiCfg, verbose); err != nil {
			return err
		}
	}

	// Print success message
	printSuccessMessageMulti(absPath, selectedAIs)

	return nil
}

func printInitBanner() {
	fmt.Println(`
████████╗ █████╗ ███████╗██╗  ██╗██╗    ██╗██╗███╗   ██╗ ██████╗
╚══██╔══╝██╔══██╗██╔════╝██║ ██╔╝██║    ██║██║████╗  ██║██╔════╝
   ██║   ███████║███████╗█████╔╝ ██║ █╗ ██║██║██╔██╗ ██║██║  ███╗
   ██║   ██╔══██║╚════██║██╔═██╗ ██║███╗██║██║██║╚██╗██║██║   ██║
   ██║   ██║  ██║███████║██║  ██╗╚███╔███╔╝██║██║ ╚████║╚██████╔╝
   ╚═╝   ╚═╝  ╚═╝╚══════╝╚═╝  ╚═╝ ╚══╝╚══╝ ╚═╝╚═╝  ╚═══╝ ╚═════╝
                    Project Initialization`)
}

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

func createTaskwingStructure(basePath string, verbose bool) error {
	dirs := []string{
		".taskwing",
		".taskwing/memory",
	}

	for _, dir := range dirs {
		fullPath := filepath.Join(basePath, dir)
		if err := os.MkdirAll(fullPath, 0755); err != nil {
			return fmt.Errorf("failed to create %s: %w", dir, err)
		}
		if verbose {
			fmt.Printf("  ✓ Created %s\n", dir)
		}
	}

	// Create config.yaml
	configPath := filepath.Join(basePath, ".taskwing", "config.yaml")
	configContent := fmt.Sprintf(`# TaskWing Configuration
# https://taskwing.app/docs/config

version: "1"

# LLM settings for bootstrap
llm:
  provider: openai
  model: %s

# Memory settings
memory:
  path: .taskwing/memory
`, llm.DefaultOpenAIModel)
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		return fmt.Errorf("failed to create config.yaml: %w", err)
	}
	if verbose {
		fmt.Println("  ✓ Created .taskwing/config.yaml")
	}

	return nil
}

func createSlashCommands(basePath string, aiCfg aiConfig, verbose bool) error {
	commandsDir := filepath.Join(basePath, aiCfg.commandsDir)
	if err := os.MkdirAll(commandsDir, 0755); err != nil {
		return fmt.Errorf("failed to create commands dir: %w", err)
	}

	// Define slash commands
	commands := map[string]string{
		"taskwing.bootstrap.md": getBootstrapCommand(),
		"taskwing.context.md":   getContextCommand(),
		"taskwing.specify.md":   getSpecifyCommand(),
	}

	for filename, content := range commands {
		filePath := filepath.Join(commandsDir, filename)
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to create %s: %w", filename, err)
		}
		if verbose {
			fmt.Printf("  ✓ Created %s/%s\n", aiCfg.commandsDir, filename)
		}
	}

	return nil
}

func printSuccessMessageMulti(basePath string, selectedAIs []string) {
	projectName := filepath.Base(basePath)

	fmt.Println("\n╭─────────────────────────────────────────────╮")
	fmt.Println("│              ✅ Project Ready                │")
	fmt.Println("╰─────────────────────────────────────────────╯")
	fmt.Printf("\n  Project: %s\n", projectName)

	// List all selected AIs
	var aiNames []string
	for _, ai := range selectedAIs {
		aiNames = append(aiNames, aiConfigs[ai].displayName)
	}
	fmt.Printf("  AI(s):   %s\n", strings.Join(aiNames, ", "))

	fmt.Println("\n╭─────────────────────────────────────────────╮")
	fmt.Println("│                 Next Steps                   │")
	fmt.Println("╰─────────────────────────────────────────────╯")
	fmt.Println("")
	fmt.Println("  1. Bootstrap your codebase:")
	fmt.Println("     tw bootstrap")
	fmt.Println("")
	fmt.Println("  2. Start the MCP server:")
	fmt.Println("     tw start")
	fmt.Println("")
	fmt.Println("  3. Use slash commands in your AI assistant:")
	fmt.Println("     /taskwing.bootstrap  - Analyze codebase")
	fmt.Println("     /taskwing.context    - Get project context")
	fmt.Println("     /taskwing.specify    - Create feature spec")
}

// Slash command content generators

func getBootstrapCommand() string {
	return `# /taskwing.bootstrap

Analyze the codebase and build the TaskWing knowledge graph.

## Two Ways to Bootstrap

### Option 1: Let me (your AI assistant) analyze directly
I can analyze your codebase right now without any API key. Just ask me to:
- Scan your project structure
- Identify architectural decisions
- Document key features and their rationale
- Save findings to .taskwing/memory/

**No API key required** - I'll do the analysis myself.

### Option 2: Use the CLI (requires OPENAI_API_KEY)
` + "```bash" + `
export OPENAI_API_KEY="your-key"
tw bootstrap
` + "```" + `

This runs TaskWing's own LLM analysis pipeline.

## What gets captured
- Architectural decisions and WHY they were made
- Key features and their rationale
- Dependencies and their purposes
- Code patterns and conventions

## Storage
Knowledge is stored in .taskwing/memory/memory.db (SQLite)
`
}

func getContextCommand() string {
	return `# /taskwing.context

Get relevant project context for your current task.

## Two Ways to Get Context

### Option 1: Ask me directly
Just describe what you're working on, and I'll search the TaskWing knowledge graph:

"I'm working on the authentication flow. What decisions were made about session handling?"

I'll query .taskwing/memory/ and give you relevant context.

### Option 2: Use the CLI
` + "```bash" + `
tw context "authentication flow"
tw context "why did we choose PostgreSQL"
` + "```" + `

## What I return
- Related architectural decisions
- Similar features already built
- Relevant code patterns to follow
- Trade-offs that were considered
`
}

func getSpecifyCommand() string {
	return `# /taskwing.specify

Create a feature specification using multi-persona analysis.

## Two Ways to Create Specs

### Option 1: Let me create the spec
Describe your feature, and I'll analyze it from multiple perspectives:
- **Product Manager**: User stories and acceptance criteria
- **Architect**: Technical design and integration points
- **Engineer**: Implementation tasks and estimates
- **QA**: Test strategy and edge cases

Just say: "Create a spec for adding OAuth2 authentication with Google"

### Option 2: Use the CLI (requires OPENAI_API_KEY)
` + "```bash" + `
tw spec create "add OAuth2 authentication with Google"
` + "```" + `

## Output
Specs are saved to .taskwing/specs/<feature-slug>/
- spec.json (structured data)
- spec.md (human-readable)
- tasks.md (implementation checklist)
`
}
