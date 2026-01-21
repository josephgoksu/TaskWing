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
	"strings"

	"github.com/josephgoksu/TaskWing/internal/codeintel"
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
  taskwing memory export              # Generate comprehensive ARCHITECTURE.md
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

		basePath, err := config.GetMemoryBasePath()
		if err != nil {
			return fmt.Errorf("get memory path: %w", err)
		}
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
  • Index cache staleness
  • Embedding dimension consistency
  • Symbol index health (language breakdown, stale files)`,
	RunE: func(cmd *cobra.Command, args []string) error {
		memoryPath, err := config.GetMemoryBasePath()
		if err != nil {
			return fmt.Errorf("get memory path: %w", err)
		}
		repo, err := memory.NewDefaultRepository(memoryPath)
		if err != nil {
			return fmt.Errorf("open memory repo: %w", err)
		}
		defer func() { _ = repo.Close() }()

		issues, err := repo.Check()
		if err != nil {
			return fmt.Errorf("check integrity: %w", err)
		}

		// Check embedding stats
		embStats, embErr := repo.GetEmbeddingStats()

		// Check symbol index stats
		ctx := context.Background()
		var symbolStats *codeintel.SymbolStats
		var staleFiles []string
		if db := repo.GetDB(); db != nil {
			if sqlDB := db.DB(); sqlDB != nil {
				codeRepo := codeintel.NewRepository(sqlDB)
				symbolStats, _ = codeRepo.GetSymbolStats(ctx)

				// Check for stale files (files that no longer exist)
				if symbolStats != nil && symbolStats.TotalFiles > 0 {
					cwd, _ := os.Getwd()
					staleFiles, _ = codeRepo.GetStaleSymbolFiles(ctx, func(path string) bool {
						fullPath := filepath.Join(cwd, path)
						_, err := os.Stat(fullPath)
						return err == nil
					})
					if symbolStats != nil {
						symbolStats.StaleFiles = len(staleFiles)
					}
				}
			}
		}

		if viper.GetBool("json") {
			output, _ := json.MarshalIndent(map[string]any{
				"issues":          issues,
				"count":           len(issues),
				"embedding_stats": embStats,
				"symbol_stats":    symbolStats,
				"stale_files":     staleFiles,
			}, "", "  ")
			fmt.Println(string(output))
			return nil
		}

		// Show embedding stats first
		if embErr == nil && embStats != nil {
			fmt.Println("📊 Knowledge Embeddings:")
			fmt.Printf("  Total nodes:     %d\n", embStats.TotalNodes)
			fmt.Printf("  With embeddings: %d\n", embStats.NodesWithEmbeddings)
			fmt.Printf("  Missing:         %d\n", embStats.NodesWithoutEmbeddings)
			if embStats.EmbeddingDimension > 0 {
				fmt.Printf("  Dimension:       %d\n", embStats.EmbeddingDimension)
			}
			fmt.Println()

			// Warn about missing embeddings
			if embStats.NodesWithoutEmbeddings > 0 {
				fmt.Printf("⚠  %d nodes are missing embeddings.\n", embStats.NodesWithoutEmbeddings)
				fmt.Println("   Run 'tw memory generate-embeddings' to backfill.")
				fmt.Println()
			}

			// Warn about mixed dimensions
			if embStats.MixedDimensions {
				fmt.Println("⚠  WARNING: Mixed embedding dimensions detected!")
				fmt.Println("   This can happen when switching between different embedding models.")
				fmt.Println("   Run 'tw memory rebuild-embeddings' to regenerate all embeddings.")
				fmt.Println()
			}
		}

		// Show symbol index stats
		if symbolStats != nil && symbolStats.TotalSymbols > 0 {
			fmt.Println("💻 Code Symbol Index:")
			fmt.Printf("  Total symbols:   %d\n", symbolStats.TotalSymbols)
			fmt.Printf("  Indexed files:   %d\n", symbolStats.TotalFiles)
			fmt.Printf("  Relations:       %d\n", symbolStats.TotalRelations)
			if symbolStats.TotalDeps > 0 {
				fmt.Printf("  Dependencies:    %d\n", symbolStats.TotalDeps)
			}
			if symbolStats.WithEmbeddings > 0 {
				fmt.Printf("  With embeddings: %d\n", symbolStats.WithEmbeddings)
			}
			fmt.Println()

			// Show language breakdown
			if len(symbolStats.ByLanguage) > 0 {
				fmt.Println("  Languages:")
				for lang, count := range symbolStats.ByLanguage {
					fmt.Printf("    %-12s %d symbols\n", lang+":", count)
				}
				fmt.Println()
			}

			// Warn about stale files
			if len(staleFiles) > 0 {
				fmt.Printf("⚠  %d indexed files no longer exist:\n", len(staleFiles))
				maxShow := 5
				for i, f := range staleFiles {
					if i >= maxShow {
						fmt.Printf("     ... and %d more\n", len(staleFiles)-maxShow)
						break
					}
					fmt.Printf("     %s\n", f)
				}
				fmt.Println("   Run 'tw bootstrap --force' to re-index the codebase.")
				fmt.Println()
			}
		} else if symbolStats != nil {
			fmt.Println("💻 Code Symbol Index: (empty)")
			fmt.Println("   Run 'tw bootstrap' to index your codebase.")
			fmt.Println()
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
		memoryPath, err := config.GetMemoryBasePath()
		if err != nil {
			return fmt.Errorf("get memory path: %w", err)
		}
		repo, err := memory.NewDefaultRepository(memoryPath)
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
	Use:   "rebuild-index",
	Short: "Rebuild the index cache",
	Long: `Regenerate the index.json cache from SQLite data.

This is useful if the cache is out of sync with the database.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		memoryPath, err := config.GetMemoryBasePath()
		if err != nil {
			return fmt.Errorf("get memory path: %w", err)
		}
		repo, err := memory.NewDefaultRepository(memoryPath)
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
	Use:   "generate-embeddings",
	Short: "Generate embeddings for nodes without them",
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

		memoryPath, err := config.GetMemoryBasePath()
		if err != nil {
			return fmt.Errorf("get memory path: %w", err)
		}
		repo, err := memory.NewDefaultRepository(memoryPath)
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
		// Preflight embedding generation to avoid repeated failures
		if _, err := knowledge.GenerateEmbedding(ctx, "taskwing-embedding-healthcheck", llmCfg); err != nil {
			embeddingProvider := llmCfg.EmbeddingProvider
			if embeddingProvider == "" {
				embeddingProvider = llmCfg.Provider
			}
			if embeddingProvider == llm.ProviderOllama {
				baseURL := llmCfg.EmbeddingBaseURL
				if baseURL == "" {
					baseURL = llmCfg.BaseURL
				}
				if baseURL == "" {
					baseURL = llm.DefaultOllamaURL
				}
				return fmt.Errorf("embedding generation failed: Ollama not reachable at %s (start Ollama or set llm.provider/llm.embedding_provider to openai/gemini)", baseURL)
			}
			return fmt.Errorf("embedding generation failed: %w", err)
		}

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

// memory export command
var memoryExportCmd = &cobra.Command{
	Use:   "export",
	Short: "Generate comprehensive ARCHITECTURE.md",
	Long: `Generate a comprehensive ARCHITECTURE.md file that consolidates all project knowledge.

The generated file includes:
  • Architectural Constraints (mandatory rules)
  • Features & Components (with their decisions)
  • Design Patterns (recurring workflows)
  • Key Decisions (cross-cutting decisions by source)

The file is written to .taskwing/memory/ARCHITECTURE.md

Examples:
  taskwing memory export                    # Generate with project name from cwd
  taskwing memory export --name "My App"    # Generate with custom project name`,
	RunE: func(cmd *cobra.Command, args []string) error {
		memoryPath, err := config.GetMemoryBasePath()
		if err != nil {
			return fmt.Errorf("get memory path: %w", err)
		}
		repo, err := memory.NewDefaultRepository(memoryPath)
		if err != nil {
			return fmt.Errorf("open memory repo: %w", err)
		}
		defer func() { _ = repo.Close() }()

		// Get project name from flag or use current directory name
		projectName, _ := cmd.Flags().GetString("name")
		if projectName == "" {
			cwd, _ := os.Getwd()
			projectName = filepath.Base(cwd)
		}

		if err := repo.GenerateArchitectureMD(projectName); err != nil {
			return fmt.Errorf("generate architecture.md: %w", err)
		}

		archPath := filepath.Join(memoryPath, "ARCHITECTURE.md")
		fmt.Printf("✓ Generated %s\n", archPath)
		return nil
	},
}

// memory rebuild-embeddings command
var memoryRebuildEmbeddingsCmd = &cobra.Command{
	Use:   "rebuild-embeddings",
	Short: "Regenerate ALL embeddings",
	Long: `Regenerate embeddings for ALL nodes in the memory database.

This is useful when:
  • Switching to a different embedding model
  • Mixed embedding dimensions detected
  • Upgrading to a better embedding model (e.g., Qwen3)

Unlike 'generate-embeddings' (which only backfills missing), this command
regenerates embeddings for ALL nodes, ensuring consistency.

WARNING: This can be expensive if you have many nodes and are using a paid API.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ui.RenderPageHeader("TaskWing Embeddings", "Regenerating all vectors")

		force, _ := cmd.Flags().GetBool("force")
		if !force {
			fmt.Print("⚠  This will regenerate ALL embeddings. Are you sure? [y/N]: ")
			var response string
			_, _ = fmt.Scanln(&response)
			if response != "y" && response != "Y" {
				fmt.Println("Rebuild cancelled.")
				return nil
			}
		}

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

		memoryPath, err := config.GetMemoryBasePath()
		if err != nil {
			return fmt.Errorf("get memory path: %w", err)
		}
		repo, err := memory.NewDefaultRepository(memoryPath)
		if err != nil {
			return fmt.Errorf("open memory repo: %w", err)
		}
		defer func() { _ = repo.Close() }()

		nodes, err := repo.ListNodes("")
		if err != nil {
			return fmt.Errorf("list nodes: %w", err)
		}

		if len(nodes) == 0 {
			fmt.Println("No nodes to process.")
			return nil
		}

		fmt.Printf("Regenerating embeddings for %d nodes...\n\n", len(nodes))

		ctx := context.Background()
		generated := 0
		failed := 0

		for _, n := range nodes {
			fullNode, err := repo.GetNode(n.ID)
			if err != nil {
				failed++
				continue
			}

			embedding, err := knowledge.GenerateEmbedding(ctx, fullNode.Content, llmCfg)
			if err != nil {
				fmt.Printf("  ✗ %s: %v\n", n.ID, err)
				failed++
				continue
			}

			if err := repo.UpdateNodeEmbedding(n.ID, embedding); err != nil {
				fmt.Printf("  ✗ %s: save failed\n", n.ID)
				failed++
				continue
			}

			generated++
			if !viper.GetBool("quiet") {
				fmt.Printf("  ✓ %s (dim: %d)\n", fullNode.Summary, len(embedding))
			}
		}

		fmt.Printf("\n✓ Regenerated %d/%d embeddings", generated, len(nodes))
		if failed > 0 {
			fmt.Printf(" (%d failed)", failed)
		}
		fmt.Println()

		return nil
	},
}

// memory inspect command
var memoryInspectCmd = &cobra.Command{
	Use:   "inspect <query>",
	Short: "Inspect retrieval results with debug info",
	Long: `Debug the semantic search pipeline by showing raw retrieval data.

Shows detailed information about how the search works:
  • Chunk IDs and source file paths
  • Individual scores (FTS, Vector, Combined, Rerank)
  • Search pipeline stages used (ExactMatch, FTS, Vector, Rerank, Graph)
  • Timing for each stage

This is useful for understanding why certain results rank higher than others.

Examples:
  taskwing memory inspect "authentication"       # Search for auth-related knowledge
  taskwing memory inspect "task-abc123"          # Direct ID lookup
  taskwing memory inspect "JWT" --verbose        # Show embedding dimensions
  taskwing memory inspect "error handling" -n 20 # Show more results`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		query := args[0]
		limit, _ := cmd.Flags().GetInt("limit")
		verbose, _ := cmd.Flags().GetBool("verbose")

		if limit <= 0 {
			limit = 10
		}

		memoryPath, err := config.GetMemoryBasePath()
		if err != nil {
			return fmt.Errorf("get memory path: %w", err)
		}
		repo, err := memory.NewDefaultRepository(memoryPath)
		if err != nil {
			return fmt.Errorf("open memory repo: %w", err)
		}
		defer func() { _ = repo.Close() }()

		llmCfg, _ := config.LoadLLMConfig()
		svc := knowledge.NewService(repo, llmCfg)

		ctx := context.Background()
		result, err := svc.SearchDebug(ctx, query, limit)
		if err != nil {
			return fmt.Errorf("search failed: %w", err)
		}

		if viper.GetBool("json") {
			output, _ := json.MarshalIndent(result, "", "  ")
			fmt.Println(string(output))
			return nil
		}

		// Header
		ui.RenderPageHeader("TaskWing Memory Inspect", fmt.Sprintf("Query: %q", query))

		// Pipeline info
		fmt.Printf("📊 Pipeline: %s\n", formatPipeline(result.Pipeline))
		fmt.Printf("🔍 Total candidates: %d\n", result.TotalCandidates)

		// Timings
		if verbose {
			fmt.Printf("⏱  Timings: ")
			first := true
			for stage, ms := range result.Timings {
				if ms > 0 {
					if !first {
						fmt.Print(", ")
					}
					fmt.Printf("%s=%dms", stage, ms)
					first = false
				}
			}
			fmt.Println()
		}
		fmt.Println()

		if len(result.Results) == 0 {
			fmt.Println("No results found.")
			return nil
		}

		// Build table
		table := ui.Table{
			MaxWidth: 40,
		}

		if verbose {
			table.Headers = []string{"ID", "Type", "Score", "FTS", "Vec", "Dim", "Source"}
		} else {
			table.Headers = []string{"ID", "Type", "Score", "Source/Summary"}
		}

		for _, r := range result.Results {
			// Format score with indicator
			scoreStr := fmt.Sprintf("%.3f", r.CombinedScore)
			if r.IsExactMatch {
				scoreStr += " ★"
			} else if r.IsGraphExpanded {
				scoreStr += " ◆"
			}

			// Source or summary
			source := r.SourceFilePath
			if source == "" {
				source = r.Summary
			}
			if len(source) > 40 {
				source = "..." + source[len(source)-37:]
			}

			if verbose {
				ftsStr := "-"
				if r.FTSScore > 0 {
					ftsStr = fmt.Sprintf("%.2f", r.FTSScore)
				}
				vecStr := "-"
				if r.VectorScore > 0 {
					vecStr = fmt.Sprintf("%.2f", r.VectorScore)
				}
				dimStr := "-"
				if r.EmbeddingDimension > 0 {
					dimStr = fmt.Sprintf("%d", r.EmbeddingDimension)
				}

				table.Rows = append(table.Rows, []string{
					ui.TruncateID(r.ID),
					r.NodeType,
					scoreStr,
					ftsStr,
					vecStr,
					dimStr,
					source,
				})
			} else {
				table.Rows = append(table.Rows, []string{
					ui.TruncateID(r.ID),
					r.NodeType,
					scoreStr,
					source,
				})
			}
		}

		fmt.Println(table.Render())

		// Legend
		fmt.Println()
		fmt.Println("Legend: ★ = exact ID match, ◆ = graph expanded")

		return nil
	},
}

// formatPipeline formats the pipeline stages for display
func formatPipeline(stages []string) string {
	if len(stages) == 0 {
		return "(none)"
	}
	result := ""
	for i, s := range stages {
		if i > 0 {
			result += " → "
		}
		result += s
	}
	return result
}

// memoryBackfillWorkspaceCmd backfills workspace values for existing nodes
var memoryBackfillWorkspaceCmd = &cobra.Command{
	Use:   "backfill-workspace",
	Short: "Backfill workspace values for existing nodes",
	Long: `Infer and populate workspace values for nodes that don't have them.

This is useful for:
  • Migrating existing knowledge to workspace-aware storage
  • Fixing nodes created before workspace support was added
  • Ensuring monorepo knowledge is properly scoped

Workspace inference uses:
  1. SourceAgent metadata (if agent is service-specific)
  2. File paths in node content (e.g., paths containing service names)
  3. Defaults to 'root' if no workspace can be inferred

Examples:
  taskwing memory backfill-workspace --dry-run     # Preview changes without writing
  taskwing memory backfill-workspace               # Apply workspace assignments
  taskwing memory backfill-workspace --limit 10    # Process only first 10 nodes`,
	RunE: func(cmd *cobra.Command, args []string) error {
		dryRun, _ := cmd.Flags().GetBool("dry-run")
		limit, _ := cmd.Flags().GetInt("limit")

		if dryRun {
			ui.RenderPageHeader("TaskWing Workspace Backfill", "Dry-run mode (no changes)")
		} else {
			ui.RenderPageHeader("TaskWing Workspace Backfill", "Inferring workspace values")
		}

		memoryPath, err := config.GetMemoryBasePath()
		if err != nil {
			return fmt.Errorf("get memory path: %w", err)
		}
		repo, err := memory.NewDefaultRepository(memoryPath)
		if err != nil {
			return fmt.Errorf("open memory repo: %w", err)
		}
		defer func() { _ = repo.Close() }()

		// Get all nodes
		nodes, err := repo.ListNodes("")
		if err != nil {
			return fmt.Errorf("list nodes: %w", err)
		}

		// Apply limit if specified
		if limit > 0 && limit < len(nodes) {
			nodes = nodes[:limit]
		}

		// Detect workspace structure for inference
		cwd, _ := os.Getwd()
		wsInfo, _ := detectWorkspaceForBackfill(cwd)

		if viper.GetBool("json") {
			return runBackfillJSON(repo, nodes, wsInfo, dryRun)
		}

		// Show workspace detection results
		if wsInfo != nil && len(wsInfo.Services) > 0 {
			fmt.Printf("📂 Detected workspace: %s (%d services)\n", wsInfo.Type.String(), len(wsInfo.Services))
			fmt.Printf("   Services: %v\n\n", wsInfo.Services)
		} else {
			fmt.Println("📂 No monorepo detected (single workspace mode)")
			fmt.Println()
		}

		updated := 0
		skipped := 0
		unchanged := 0

		for _, n := range nodes {
			fullNode, err := repo.GetNode(n.ID)
			if err != nil {
				skipped++
				continue
			}

			// Skip if workspace is already set and not 'root'
			if fullNode.Workspace != "" && fullNode.Workspace != "root" {
				unchanged++
				continue
			}

			// Infer workspace from node content and metadata
			inferredWS := inferWorkspace(fullNode, wsInfo)

			// Skip if inference results in same value
			if inferredWS == fullNode.Workspace {
				unchanged++
				continue
			}

			if dryRun {
				fmt.Printf("PLANNED: %s → workspace=%q (was %q)\n", fullNode.ID, inferredWS, fullNode.Workspace)
				updated++
			} else {
				// Update the node's workspace
				if err := repo.UpdateNodeWorkspace(fullNode.ID, inferredWS); err != nil {
					fmt.Printf("  ✗ %s: %v\n", fullNode.ID, err)
					skipped++
					continue
				}
				if !viper.GetBool("quiet") {
					fmt.Printf("  ✓ %s → workspace=%q\n", fullNode.ID, inferredWS)
				}
				updated++
			}
		}

		fmt.Println()
		if dryRun {
			fmt.Printf("📊 Dry-run summary: %d would be updated, %d unchanged, %d skipped\n", updated, unchanged, skipped)
			fmt.Println("\nRun without --dry-run to apply changes.")
		} else {
			fmt.Printf("✓ Backfill complete: %d updated, %d unchanged, %d skipped\n", updated, unchanged, skipped)

			// Regenerate markdown mirror if changes were made
			if updated > 0 {
				fmt.Println("Regenerating markdown mirror...")
				if err := repo.RebuildFiles(); err != nil {
					fmt.Printf("⚠  Warning: failed to rebuild markdown files: %v\n", err)
				} else {
					fmt.Println("✓ Markdown mirror updated")
				}
			}
		}

		return nil
	},
}

// detectWorkspaceForBackfill wraps project.DetectWorkspace to import it
func detectWorkspaceForBackfill(basePath string) (*workspaceInfoCompat, error) {
	// Import the project package's workspace detection
	// For now, we'll do a simple inline detection
	entries, err := os.ReadDir(basePath)
	if err != nil {
		return nil, err
	}

	var services []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if name[0] == '.' || isSkippableBackfillDir(name) {
			continue
		}
		dirPath := filepath.Join(basePath, name)
		if isBackfillProjectDir(dirPath) {
			services = append(services, name)
		}
	}

	return &workspaceInfoCompat{
		Services: services,
		Type:     func() workspaceTypeCompat {
			if len(services) > 1 {
				return wsTypeMonorepo
			}
			return wsTypeSingle
		}(),
	}, nil
}

type workspaceTypeCompat int

const (
	wsTypeSingle   workspaceTypeCompat = 0
	wsTypeMonorepo workspaceTypeCompat = 1
)

func (t workspaceTypeCompat) String() string {
	if t == wsTypeMonorepo {
		return "monorepo"
	}
	return "single"
}

type workspaceInfoCompat struct {
	Type     workspaceTypeCompat
	Services []string
}

func isSkippableBackfillDir(name string) bool {
	skip := map[string]bool{
		"node_modules": true, "vendor": true, "dist": true, "build": true,
		"out": true, "target": true, "bin": true, "__pycache__": true,
		".next": true, "coverage": true, "test-results": true,
	}
	return skip[name]
}

func isBackfillProjectDir(path string) bool {
	markers := []string{".git", "package.json", "go.mod", "pom.xml", "build.gradle", "Cargo.toml", "Dockerfile"}
	for _, m := range markers {
		if _, err := os.Stat(filepath.Join(path, m)); err == nil {
			return true
		}
	}
	return false
}

// inferWorkspace determines the workspace for a node based on content and metadata
func inferWorkspace(n *memory.Node, wsInfo *workspaceInfoCompat) string {
	if wsInfo == nil || len(wsInfo.Services) == 0 {
		return "root"
	}

	// Check if node content contains service-specific paths
	content := n.Content + " " + n.Summary
	for _, svc := range wsInfo.Services {
		// Look for patterns like "osprey/", "studio/src", etc.
		patterns := []string{
			svc + "/",
			svc + "\\",
			"/" + svc + "/",
			"\\" + svc + "\\",
		}
		for _, p := range patterns {
			if containsCaseInsensitive(content, p) {
				return svc
			}
		}
	}

	// Check SourceAgent for service hints
	if n.SourceAgent != "" {
		for _, svc := range wsInfo.Services {
			if containsCaseInsensitive(n.SourceAgent, svc) {
				return svc
			}
		}
	}

	// Default to root (global workspace)
	return "root"
}

func containsCaseInsensitive(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr || len(substr) > 0 &&
			(containsLower(s, substr)))
}

func containsLower(s, substr string) bool {
	s = strings.ToLower(s)
	substr = strings.ToLower(substr)
	return strings.Contains(s, substr)
}

// runBackfillJSON outputs backfill results as JSON
func runBackfillJSON(repo *memory.Repository, nodes []memory.Node, wsInfo *workspaceInfoCompat, dryRun bool) error {
	type backfillResult struct {
		NodeID       string `json:"nodeId"`
		OldWorkspace string `json:"oldWorkspace"`
		NewWorkspace string `json:"newWorkspace"`
		Status       string `json:"status"` // "planned", "updated", "unchanged", "error"
	}

	var results []backfillResult

	for _, n := range nodes {
		fullNode, err := repo.GetNode(n.ID)
		if err != nil {
			results = append(results, backfillResult{
				NodeID: n.ID,
				Status: "error",
			})
			continue
		}

		if fullNode.Workspace != "" && fullNode.Workspace != "root" {
			results = append(results, backfillResult{
				NodeID:       fullNode.ID,
				OldWorkspace: fullNode.Workspace,
				NewWorkspace: fullNode.Workspace,
				Status:       "unchanged",
			})
			continue
		}

		inferredWS := inferWorkspace(fullNode, wsInfo)

		if inferredWS == fullNode.Workspace {
			results = append(results, backfillResult{
				NodeID:       fullNode.ID,
				OldWorkspace: fullNode.Workspace,
				NewWorkspace: inferredWS,
				Status:       "unchanged",
			})
			continue
		}

		if dryRun {
			results = append(results, backfillResult{
				NodeID:       fullNode.ID,
				OldWorkspace: fullNode.Workspace,
				NewWorkspace: inferredWS,
				Status:       "planned",
			})
		} else {
			if err := repo.UpdateNodeWorkspace(fullNode.ID, inferredWS); err != nil {
				results = append(results, backfillResult{
					NodeID:       fullNode.ID,
					OldWorkspace: fullNode.Workspace,
					NewWorkspace: inferredWS,
					Status:       "error",
				})
			} else {
				results = append(results, backfillResult{
					NodeID:       fullNode.ID,
					OldWorkspace: fullNode.Workspace,
					NewWorkspace: inferredWS,
					Status:       "updated",
				})
			}
		}
	}

	output, _ := json.MarshalIndent(map[string]any{
		"dryRun":   dryRun,
		"total":    len(nodes),
		"services": wsInfo.Services,
		"results":  results,
	}, "", "  ")
	fmt.Println(string(output))
	return nil
}

func init() {
	rootCmd.AddCommand(memoryCmd)

	// Add subcommands
	memoryCmd.AddCommand(memoryCheckCmd)
	memoryCmd.AddCommand(memoryRepairCmd)
	memoryCmd.AddCommand(memoryRebuildCmd)
	memoryCmd.AddCommand(memoryGenerateEmbeddingsCmd)
	memoryCmd.AddCommand(memoryRebuildEmbeddingsCmd)
	memoryCmd.AddCommand(memoryResetCmd)
	memoryCmd.AddCommand(memoryExportCmd)
	memoryCmd.AddCommand(memoryInspectCmd)
	memoryCmd.AddCommand(memoryBackfillWorkspaceCmd)

	memoryResetCmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")
	memoryRebuildEmbeddingsCmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")
	memoryExportCmd.Flags().StringP("name", "n", "", "Project name for the document header")
	memoryInspectCmd.Flags().IntP("limit", "n", 10, "Maximum number of results")
	memoryInspectCmd.Flags().BoolP("verbose", "v", false, "Show detailed scores and embedding dimensions")
	memoryBackfillWorkspaceCmd.Flags().Bool("dry-run", false, "Preview changes without writing to database")
	memoryBackfillWorkspaceCmd.Flags().IntP("limit", "n", 0, "Limit the number of nodes to process (0 = all)")
}
