/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/josephgoksu/TaskWing/internal/config"
	"github.com/josephgoksu/TaskWing/internal/knowledge"
	"github.com/josephgoksu/TaskWing/internal/llm"
	"github.com/josephgoksu/TaskWing/internal/memory"
	"github.com/josephgoksu/TaskWing/internal/ui"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// memoryCmd represents the memory command
var memoryCmd = &cobra.Command{
	Use:   "memory",
	Short: "Manage project memory integrity",
	Long: `Manage the integrity of your project memory database.

Commands for checking, repairing, and rebuilding the memory store.

Examples:
  taskwing memory check               # Check for integrity issues
  taskwing memory repair              # Fix integrity issues
  taskwing memory rebuild             # Rebuild the index cache
  taskwing memory generate-embeddings # Backfill missing embeddings
  taskwing memory reset               # Wipe all project memory and start fresh`,
}

// memory reset command
var memoryResetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Wipe all project memory",
	Long: `Completely delete the project memory database and index.

This action is irreversible. It will delete all nodes, edges, features,
and decisions from the current project's memory store.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ui.RenderPageHeader("TaskWing Memory Reset", "Wiping all project context")
		force, _ := cmd.Flags().GetBool("force")
		if !force {
			fmt.Print("⚠️  This will delete ALL project memory. Are you sure? [y/N]: ")
			var response string
			_, _ = fmt.Scanln(&response)
			if response != "y" && response != "Y" {
				fmt.Println("Reset cancelled.")
				return nil
			}
		}

		basePath := config.GetMemoryBasePath()
		fmt.Printf("Wiping memory in %s...\n", basePath)

		// Close any open connections by not creating a store, or we can just delete files
		dbPath := filepath.Join(basePath, "memory.db")
		indexPath := filepath.Join(basePath, "index.json")
		featuresDir := filepath.Join(basePath, "features")

		_ = os.Remove(dbPath)
		_ = os.Remove(indexPath)
		_ = os.RemoveAll(featuresDir)

		fmt.Println("✓ Project memory wiped successfully.")
		return nil
	},
}

// memory check command
var memoryCheckCmd = &cobra.Command{
	Use:   "check",
	Short: "Check memory integrity",
	Long: `Validate the integrity of the project memory.

Checks for:
  • Missing markdown files
  • Orphan edges (relationships to non-existent features)
  • Index cache staleness`,
	RunE: func(cmd *cobra.Command, args []string) error {
		repo, err := memory.NewDefaultRepository(config.GetMemoryBasePath())
		if err != nil {
			return fmt.Errorf("open memory repo: %w", err)
		}
		defer func() { _ = repo.Close() }()

		issues, err := repo.Check()
		if err != nil {
			return fmt.Errorf("check integrity: %w", err)
		}

		if viper.GetBool("json") {
			output, _ := json.MarshalIndent(map[string]interface{}{
				"issues": issues,
				"count":  len(issues),
			}, "", "  ")
			fmt.Println(string(output))
			return nil
		}

		if len(issues) == 0 {
			fmt.Println("✓ No integrity issues found")
			return nil
		}

		fmt.Printf("Found %d issues:\n\n", len(issues))
		for i, issue := range issues {
			fmt.Printf("%d. [%s] %s\n", i+1, issue.Type, issue.Message)
			if issue.FeatureID != "" {
				fmt.Printf("   Feature: %s\n", issue.FeatureID)
			}
		}

		fmt.Println("\nRun 'taskwing memory repair' to fix these issues.")
		return nil
	},
}

// memory repair command
var memoryRepairCmd = &cobra.Command{
	Use:   "repair",
	Short: "Repair integrity issues",
	Long: `Attempt to fix integrity issues in project memory.

Actions:
  • Regenerate missing markdown files from SQLite data
  • Remove orphan edges
  • Rebuild the index cache`,
	RunE: func(cmd *cobra.Command, args []string) error {
		repo, err := memory.NewDefaultRepository(config.GetMemoryBasePath())
		if err != nil {
			return fmt.Errorf("open memory repo: %w", err)
		}
		defer func() { _ = repo.Close() }()

		// First check what needs repair
		issues, _ := repo.Check()
		if len(issues) == 0 {
			fmt.Println("✓ No issues to repair")
			return nil
		}

		fmt.Printf("Repairing %d issues...\n", len(issues))

		if err := repo.Repair(); err != nil {
			return fmt.Errorf("repair: %w", err)
		}

		// Verify repair
		remaining, _ := repo.Check()
		if len(remaining) == 0 {
			fmt.Println("✓ All issues repaired")
		} else {
			fmt.Printf("⚠ %d issues remain after repair\n", len(remaining))
		}

		return nil
	},
}

// memory rebuild command
var memoryRebuildCmd = &cobra.Command{
	Use:     "rebuild-index",
	Aliases: []string{"rebuild"},
	Short:   "Rebuild the index cache",
	Long: `Regenerate the index.json cache from SQLite data.

This is useful if the cache is out of sync with the database.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		repo, err := memory.NewDefaultRepository(config.GetMemoryBasePath())
		if err != nil {
			return fmt.Errorf("open memory repo: %w", err)
		}
		defer func() { _ = repo.Close() }()

		if err := repo.RebuildFiles(); err != nil {
			return fmt.Errorf("rebuild files: %w", err)
		}
		// Also rebuild index if repo has that method exposed or via internal db access
		if err := repo.GetDB().RebuildIndex(); err != nil {
			return fmt.Errorf("rebuild index: %w", err)
		}

		index, _ := repo.GetIndex()
		fmt.Printf("✓ Index rebuilt with %d features\n", len(index.Features))
		return nil
	},
}

// memory generate-embeddings command
var memoryGenerateEmbeddingsCmd = &cobra.Command{
	Use:     "generate-embeddings",
	Aliases: []string{"embed"},
	Short:   "Generate embeddings for nodes without them",
	Long: `Backfill embeddings for knowledge nodes that don't have them.

Requires an API key for the configured provider (OpenAI/Gemini) or a local Ollama setup. Useful after:
  • Importing data without embeddings
  • Running bootstrap without API key
  • Adding nodes with --skip-ai`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ui.RenderPageHeader("TaskWing Embeddings", "Generating missing vectors")
		llmCfg, err := config.LoadLLMConfig()
		if err != nil {
			return fmt.Errorf("load llm config: %w", err)
		}
		if llmCfg.Provider == llm.ProviderAnthropic {
			return fmt.Errorf("embedding generation is not supported for provider %q; use openai, gemini, or ollama", llmCfg.Provider)
		}
		if llmCfg.APIKey == "" && llmCfg.Provider != llm.ProviderOllama {
			return fmt.Errorf("API key required for embedding generation with provider %q", llmCfg.Provider)
		}

		repo, err := memory.NewDefaultRepository(config.GetMemoryBasePath())
		if err != nil {
			return fmt.Errorf("open memory repo: %w", err)
		}
		defer func() { _ = repo.Close() }()

		nodes, err := repo.ListNodes("")
		if err != nil {
			return fmt.Errorf("list nodes: %w", err)
		}

		// Find nodes without embeddings
		var toProcess []memory.Node
		for _, n := range nodes {
			fullNode, err := repo.GetNode(n.ID)
			if err != nil {
				continue
			}
			if len(fullNode.Embedding) == 0 {
				toProcess = append(toProcess, *fullNode)
			}
		}

		if len(toProcess) == 0 {
			fmt.Println("✓ All nodes already have embeddings")
			return nil
		}

		fmt.Printf("Generating embeddings for %d nodes...\n", len(toProcess))

		ctx := context.Background()
		generated := 0

		for _, n := range toProcess {
			embedding, err := knowledge.GenerateEmbedding(ctx, n.Content, llmCfg)
			if err != nil {
				fmt.Printf("  ✗ %s: %v\n", n.ID, err)
				continue
			}

			if err := repo.UpdateNodeEmbedding(n.ID, embedding); err != nil {
				fmt.Printf("  ✗ %s: save failed\n", n.ID)
				continue
			}

			generated++
			if !viper.GetBool("quiet") {
				fmt.Printf("  ✓ %s\n", n.Summary)
			}
		}

		fmt.Printf("\n✓ Generated %d/%d embeddings\n", generated, len(toProcess))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(memoryCmd)

	// Add subcommands
	memoryCmd.AddCommand(memoryCheckCmd)
	memoryCmd.AddCommand(memoryRepairCmd)
	memoryCmd.AddCommand(memoryRebuildCmd)
	memoryCmd.AddCommand(memoryGenerateEmbeddingsCmd)
	memoryCmd.AddCommand(memoryResetCmd)

	memoryResetCmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")
}
