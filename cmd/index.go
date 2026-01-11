/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/josephgoksu/TaskWing/internal/codeintel"
	"github.com/josephgoksu/TaskWing/internal/ui"
	"github.com/spf13/cobra"
)

// indexCmd represents the index command
var indexCmd = &cobra.Command{
	Use:   "index [path]",
	Short: "Index codebase for code intelligence",
	Long: `Index the codebase to enable code intelligence features.

This command scans Go source files, extracts symbols (functions, structs,
interfaces, methods, etc.), and stores them in the database for fast lookup.

The index enables:
  - Symbol search (tw find)
  - Impact analysis (tw impact)
  - MCP tools (find_symbol, get_callers, analyze_impact)

Examples:
  taskwing index              # Index current directory
  taskwing index ./src        # Index specific directory
  taskwing index --clear      # Clear existing index before reindexing
  taskwing index --incremental # Only re-index changed files`,
	Args: cobra.MaximumNArgs(1),
	RunE: runIndex,
}

var (
	indexClear       bool
	indexIncremental bool
	indexWorkers     int
	indexIncludeTests bool
)

func init() {
	rootCmd.AddCommand(indexCmd)
	indexCmd.Flags().BoolVar(&indexClear, "clear", false, "Clear existing index before reindexing")
	indexCmd.Flags().BoolVar(&indexIncremental, "incremental", false, "Only re-index changed files")
	indexCmd.Flags().IntVar(&indexWorkers, "workers", 0, "Number of parallel workers (default: CPU count)")
	indexCmd.Flags().BoolVar(&indexIncludeTests, "include-tests", false, "Include test files in the index")
}

func runIndex(cmd *cobra.Command, args []string) error {
	// Determine path to index
	rootPath := "."
	if len(args) > 0 {
		rootPath = args[0]
	}

	// Verify path exists
	if _, err := os.Stat(rootPath); os.IsNotExist(err) {
		return fmt.Errorf("path does not exist: %s", rootPath)
	}

	// Render header
	if !isJSON() && !isQuiet() {
		ui.RenderPageHeader("TaskWing Code Index", fmt.Sprintf("Path: %s", rootPath))
	}

	// Initialize repository
	repo, err := openRepo()
	if err != nil {
		return fmt.Errorf("open memory repo: %w", err)
	}
	defer func() { _ = repo.Close() }()

	// Get database handle
	store := repo.GetDB()
	if store == nil {
		return fmt.Errorf("database store not available")
	}
	db := store.DB()
	if db == nil {
		return fmt.Errorf("database not available")
	}

	// Create code intelligence repository and indexer
	codeRepo := codeintel.NewRepository(db)
	config := codeintel.DefaultIndexerConfig()
	if indexWorkers > 0 {
		config.Workers = indexWorkers
	}
	config.IncludeTests = indexIncludeTests

	// Progress callback
	if !isQuiet() {
		config.OnProgress = func(stats codeintel.IndexStats) {
			fmt.Fprintf(os.Stderr, "\r📊 Indexed %d files, %d symbols...", stats.FilesIndexed, stats.SymbolsFound)
		}
	}

	indexer := codeintel.NewIndexer(codeRepo, config)
	ctx := context.Background()

	// Clear index if requested
	if indexClear {
		if !isQuiet() {
			fmt.Fprint(os.Stderr, "🗑️  Clearing existing index...")
		}
		if err := indexer.ClearIndex(ctx); err != nil {
			if !isQuiet() {
				fmt.Fprintln(os.Stderr, " failed")
			}
			return fmt.Errorf("clear index: %w", err)
		}
		if !isQuiet() {
			fmt.Fprintln(os.Stderr, " done")
		}
	}

	// Run indexing
	if !isQuiet() {
		if indexIncremental {
			fmt.Fprint(os.Stderr, "🔄 Running incremental index...")
		} else {
			fmt.Fprint(os.Stderr, "📇 Indexing codebase...")
		}
	}

	start := time.Now()
	var stats *codeintel.IndexStats

	if indexIncremental {
		stats, err = indexer.IncrementalIndex(ctx, rootPath)
	} else {
		stats, err = indexer.IndexDirectory(ctx, rootPath)
	}

	if err != nil {
		if !isQuiet() {
			fmt.Fprintln(os.Stderr, " failed")
		}
		return fmt.Errorf("indexing failed: %w", err)
	}

	// Clear progress line and show completion
	if !isQuiet() {
		fmt.Fprintf(os.Stderr, "\r%s\n", "                                                  ") // Clear line
	}

	// Output results
	if isJSON() {
		return printJSON(map[string]any{
			"success":         true,
			"path":            rootPath,
			"files_scanned":   stats.FilesScanned,
			"files_indexed":   stats.FilesIndexed,
			"files_skipped":   stats.FilesSkipped,
			"symbols_found":   stats.SymbolsFound,
			"relations_found": stats.RelationsFound,
			"duration_ms":     stats.Duration.Milliseconds(),
			"errors":          stats.Errors,
		})
	}

	// Render summary
	duration := time.Since(start)
	fmt.Printf("✅ Indexing complete in %v\n", duration.Round(time.Millisecond))
	fmt.Println()
	fmt.Printf("   📁 Files scanned:  %d\n", stats.FilesScanned)
	fmt.Printf("   📝 Files indexed:  %d\n", stats.FilesIndexed)
	if stats.FilesSkipped > 0 {
		fmt.Printf("   ⏭️  Files skipped:  %d\n", stats.FilesSkipped)
	}
	fmt.Printf("   🔤 Symbols found:  %d\n", stats.SymbolsFound)
	fmt.Printf("   🔗 Relations:      %d\n", stats.RelationsFound)

	if len(stats.Errors) > 0 {
		fmt.Println()
		fmt.Printf("⚠️  %d errors occurred:\n", len(stats.Errors))
		for _, e := range stats.Errors {
			fmt.Printf("   • %s\n", e)
		}
	}

	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  • Run 'tw find <query>' to search symbols")
	fmt.Println("  • Run 'tw impact <symbol>' to analyze impact")

	return nil
}
