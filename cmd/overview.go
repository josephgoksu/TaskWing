/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/josephgoksu/TaskWing/internal/bootstrap"
	"github.com/josephgoksu/TaskWing/internal/memory"
	"github.com/josephgoksu/TaskWing/internal/ui"
	"github.com/spf13/cobra"
)

var overviewCmd = &cobra.Command{
	Use:   "overview",
	Short: "Manage project overview",
	Long: `View, edit, or regenerate the project overview.

The project overview provides a high-level description of your project
that is included in every MCP recall response for AI assistants.

Examples:
  taskwing overview             # Show current overview
  taskwing overview show        # Show current overview
  taskwing overview edit        # Edit overview in $EDITOR
  taskwing overview regenerate  # Re-analyze project and regenerate`,
	Args: cobra.NoArgs,
	RunE: runOverviewShow, // Default to show
}

var overviewShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Display the project overview",
	Long:  `Display the current project overview stored in the database.`,
	Args:  cobra.NoArgs,
	RunE:  runOverviewShow,
}

var overviewEditCmd = &cobra.Command{
	Use:   "edit",
	Short: "Edit the project overview",
	Long: `Open the project overview in your default editor ($EDITOR).

If no overview exists, creates a template for you to fill in.
After saving, the overview is stored in the database.`,
	Args: cobra.NoArgs,
	RunE: runOverviewEdit,
}

var overviewRegenerateCmd = &cobra.Command{
	Use:   "regenerate",
	Short: "Regenerate the project overview using AI",
	Long: `Re-analyze the project files (README, manifests, docs) and
generate a new overview using AI.

This will overwrite the existing overview.`,
	Args: cobra.NoArgs,
	RunE: runOverviewRegenerate,
}

func init() {
	rootCmd.AddCommand(overviewCmd)
	overviewCmd.AddCommand(overviewShowCmd)
	overviewCmd.AddCommand(overviewEditCmd)
	overviewCmd.AddCommand(overviewRegenerateCmd)
}

func runOverviewShow(cmd *cobra.Command, args []string) error {
	repo, err := openRepo()
	if err != nil {
		return fmt.Errorf("open memory repo: %w", err)
	}
	defer func() { _ = repo.Close() }()

	overview, err := repo.GetProjectOverview()
	if err != nil {
		return fmt.Errorf("get overview: %w", err)
	}

	if overview == nil {
		if isJSON() {
			return printJSON(map[string]any{"overview": nil, "message": "No project overview found"})
		}
		cmd.Println("No project overview found.")
		cmd.Println("Generate one with: taskwing overview regenerate")
		cmd.Println("Or create manually: taskwing overview edit")
		return nil
	}

	if isJSON() {
		return printJSON(overview)
	}

	renderOverview(overview)
	return nil
}

func renderOverview(overview *memory.ProjectOverview) {
	fmt.Println()
	fmt.Println(ui.StyleHeader.Render("📋 Project Overview"))
	fmt.Println(ui.StyleSubtle.Render(strings.Repeat("─", 50)))
	fmt.Println()

	// Short description
	fmt.Println(ui.StyleTitle.Render("Summary"))
	fmt.Printf("  %s\n\n", overview.ShortDescription)

	// Long description
	fmt.Println(ui.StyleTitle.Render("Description"))
	// Indent each line of the long description
	for _, line := range strings.Split(overview.LongDescription, "\n") {
		if line == "" {
			fmt.Println()
		} else {
			fmt.Printf("  %s\n", line)
		}
	}
	fmt.Println()

	// Metadata
	fmt.Println(ui.StyleSubtle.Render(strings.Repeat("─", 50)))
	meta := fmt.Sprintf("Generated: %s", overview.GeneratedAt.Format("Jan 02, 2006 15:04"))
	if !overview.LastEditedAt.IsZero() {
		meta += fmt.Sprintf(" | Last edited: %s", overview.LastEditedAt.Format("Jan 02, 2006 15:04"))
	}
	fmt.Println(ui.StyleSubtle.Render(meta))
	fmt.Println()
}

func runOverviewEdit(cmd *cobra.Command, args []string) error {
	repo, err := openRepo()
	if err != nil {
		return fmt.Errorf("open memory repo: %w", err)
	}
	defer func() { _ = repo.Close() }()

	// Get existing overview or create template
	overview, err := repo.GetProjectOverview()
	if err != nil {
		return fmt.Errorf("get overview: %w", err)
	}

	var shortDesc, longDesc string
	if overview != nil {
		shortDesc = overview.ShortDescription
		longDesc = overview.LongDescription
	} else {
		shortDesc = "A one-sentence summary of your project."
		longDesc = "A detailed description of your project.\n\nExplain what it does, key features, and target users."
	}

	// Create temp file with content
	tmpFile, err := os.CreateTemp("", "taskwing-overview-*.md")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	template := fmt.Sprintf(`# Project Overview

## Short Description (one sentence)
%s

## Long Description (2-3 paragraphs)
%s

---
Instructions: Edit the text above, then save and close the editor.
Lines starting with # are section headers and will be parsed.
`, shortDesc, longDesc)

	if _, err := tmpFile.WriteString(template); err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}
	tmpFile.Close()

	// Open editor
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vim"
	}

	editorCmd := exec.Command(editor, tmpFile.Name())
	editorCmd.Stdin = os.Stdin
	editorCmd.Stdout = os.Stdout
	editorCmd.Stderr = os.Stderr

	if err := editorCmd.Run(); err != nil {
		return fmt.Errorf("run editor: %w", err)
	}

	// Read edited content
	content, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		return fmt.Errorf("read edited file: %w", err)
	}

	// Parse the content
	newShort, newLong := parseOverviewTemplate(string(content))

	// Validate both fields are present (SQLite layer requires non-empty values)
	if newShort == "" || newLong == "" {
		// Determine what's missing for a clear error message
		if newShort == "" && newLong == "" {
			// Check if user wrote content but we failed to parse sections
			// Compare against empty template to detect real changes
			emptyTemplate := fmt.Sprintf(`# Project Overview

## Short Description (one sentence)
%s

## Long Description (2-3 paragraphs)
%s

---
Instructions: Edit the text above, then save and close the editor.
Lines starting with # are section headers and will be parsed.
`, shortDesc, longDesc)
			if strings.TrimSpace(string(content)) != strings.TrimSpace(emptyTemplate) {
				cmd.Println("Warning: Could not parse content. Make sure you have:")
				cmd.Println("  - A '## Short Description' section with content below it")
				cmd.Println("  - A '## Long Description' section with content below it")
				cmd.Println("Overview not updated. Please try again with the correct format.")
				return nil
			}
			cmd.Println("No changes detected. Overview not updated.")
			return nil
		}
		// One field is missing
		if newShort == "" {
			cmd.Println("Error: Short description is empty.")
			cmd.Println("Please add content below the '## Short Description' section.")
			return nil
		}
		cmd.Println("Error: Long description is empty.")
		cmd.Println("Please add content below the '## Long Description' section.")
		return nil
	}

	// Save to database
	now := time.Now().UTC()
	newOverview := &memory.ProjectOverview{
		ShortDescription: newShort,
		LongDescription:  newLong,
		LastEditedAt:     now,
	}

	// Preserve GeneratedAt if updating existing
	if overview != nil {
		newOverview.GeneratedAt = overview.GeneratedAt
	} else {
		newOverview.GeneratedAt = now
	}

	if err := repo.SaveProjectOverview(newOverview); err != nil {
		return fmt.Errorf("save overview: %w", err)
	}

	if !isQuiet() {
		cmd.Println(ui.StyleSuccess.Render("✓ Project overview updated"))
	}

	return nil
}

// parseOverviewTemplate extracts short and long descriptions from the editor template
func parseOverviewTemplate(content string) (short, long string) {
	lines := strings.Split(content, "\n")

	var shortLines, longLines []string
	section := ""

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Skip instruction block
		if strings.HasPrefix(trimmed, "---") {
			break
		}

		// Detect sections
		if strings.HasPrefix(trimmed, "## Short Description") {
			section = "short"
			continue
		}
		if strings.HasPrefix(trimmed, "## Long Description") {
			section = "long"
			continue
		}
		if strings.HasPrefix(trimmed, "#") {
			// Skip other headers like the title
			continue
		}

		// Collect content
		switch section {
		case "short":
			if trimmed != "" {
				shortLines = append(shortLines, trimmed)
			}
		case "long":
			longLines = append(longLines, line) // Preserve whitespace in long description
		}
	}

	short = strings.Join(shortLines, " ")
	long = strings.TrimSpace(strings.Join(longLines, "\n"))

	return short, long
}

func runOverviewRegenerate(cmd *cobra.Command, args []string) error {
	repo, err := openRepo()
	if err != nil {
		return fmt.Errorf("open memory repo: %w", err)
	}
	defer func() { _ = repo.Close() }()

	// Get LLM config
	cfg, err := getLLMConfig(cmd)
	if err != nil {
		return fmt.Errorf("get LLM config: %w", err)
	}

	// Get project path
	projectPath, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	if !isQuiet() {
		cmd.Println("Analyzing project files...")
	}

	// Run analyzer
	ctx := context.Background()
	analyzer := bootstrap.NewOverviewAnalyzer(cfg, projectPath)
	overview, err := analyzer.Analyze(ctx)
	if err != nil {
		return fmt.Errorf("analyze project: %w", err)
	}

	// Save to database
	if err := repo.SaveProjectOverview(overview); err != nil {
		return fmt.Errorf("save overview: %w", err)
	}

	if isJSON() {
		return printJSON(overview)
	}

	if !isQuiet() {
		cmd.Println(ui.StyleSuccess.Render("✓ Project overview regenerated"))
		fmt.Println()
		renderOverview(overview)
	}

	return nil
}
