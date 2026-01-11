package codeintel

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/josephgoksu/TaskWing/internal/memory"
)

// TestIndexer_IndexDirectory tests basic directory indexing.
func TestIndexer_IndexDirectory(t *testing.T) {
	// Create temp directory with Go files
	tmpDir := t.TempDir()

	files := map[string]string{
		"main.go": `package main

// Main is the entry point.
func main() {
	helper()
}
`,
		"util.go": `package main

// Helper does helper things.
func helper() int {
	return 42
}
`,
	}

	for name, content := range files {
		if err := os.WriteFile(filepath.Join(tmpDir, name), []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write %s: %v", name, err)
		}
	}

	// Create repository
	store, err := memory.NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	repo := NewRepository(store.DB())

	// Create indexer
	config := DefaultIndexerConfig()
	config.Workers = 2
	indexer := NewIndexer(repo, config)

	// Index directory
	ctx := context.Background()
	stats, err := indexer.IndexDirectory(ctx, tmpDir)
	if err != nil {
		t.Fatalf("IndexDirectory failed: %v", err)
	}

	// Verify stats
	if stats.FilesScanned != 2 {
		t.Errorf("Expected 2 files scanned, got %d", stats.FilesScanned)
	}
	if stats.FilesIndexed != 2 {
		t.Errorf("Expected 2 files indexed, got %d", stats.FilesIndexed)
	}
	if stats.SymbolsFound < 4 {
		t.Errorf("Expected at least 4 symbols (2 packages + 2 functions), got %d", stats.SymbolsFound)
	}
}

// TestIndexer_SkipsTestFiles tests that test files are skipped by default.
func TestIndexer_SkipsTestFiles(t *testing.T) {
	tmpDir := t.TempDir()

	files := map[string]string{
		"main.go": `package main

func main() {}
`,
		"main_test.go": `package main

import "testing"

func TestMain(t *testing.T) {}
`,
	}

	for name, content := range files {
		if err := os.WriteFile(filepath.Join(tmpDir, name), []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write %s: %v", name, err)
		}
	}

	store, err := memory.NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	repo := NewRepository(store.DB())
	config := DefaultIndexerConfig()
	indexer := NewIndexer(repo, config)

	ctx := context.Background()
	stats, err := indexer.IndexDirectory(ctx, tmpDir)
	if err != nil {
		t.Fatalf("IndexDirectory failed: %v", err)
	}

	// Should only have indexed main.go, not main_test.go
	if stats.FilesIndexed != 1 {
		t.Errorf("Expected 1 file indexed (excluding test), got %d", stats.FilesIndexed)
	}
}

// TestIndexer_IncludesTestFiles tests including test files when configured.
func TestIndexer_IncludesTestFiles(t *testing.T) {
	tmpDir := t.TempDir()

	files := map[string]string{
		"main.go": `package main

func main() {}
`,
		"main_test.go": `package main

import "testing"

func TestMain(t *testing.T) {}
`,
	}

	for name, content := range files {
		if err := os.WriteFile(filepath.Join(tmpDir, name), []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write %s: %v", name, err)
		}
	}

	store, err := memory.NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	repo := NewRepository(store.DB())
	config := DefaultIndexerConfig()
	config.IncludeTests = true // Enable test file indexing
	indexer := NewIndexer(repo, config)

	ctx := context.Background()
	stats, err := indexer.IndexDirectory(ctx, tmpDir)
	if err != nil {
		t.Fatalf("IndexDirectory failed: %v", err)
	}

	// Should have indexed both files
	if stats.FilesIndexed != 2 {
		t.Errorf("Expected 2 files indexed (including test), got %d", stats.FilesIndexed)
	}
}

// TestIndexer_SkipsExcludedDirs tests that excluded directories are skipped.
func TestIndexer_SkipsExcludedDirs(t *testing.T) {
	tmpDir := t.TempDir()

	// Create files in various directories
	files := map[string]string{
		"main.go": `package main

func main() {}
`,
		"vendor/lib/lib.go": `package lib

func VendorFunc() {}
`,
		"node_modules/pkg/pkg.go": `package pkg

func NodeFunc() {}
`,
	}

	for path, content := range files {
		fullPath := filepath.Join(tmpDir, path)
		os.MkdirAll(filepath.Dir(fullPath), 0755)
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write %s: %v", path, err)
		}
	}

	store, err := memory.NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	repo := NewRepository(store.DB())
	config := DefaultIndexerConfig()
	indexer := NewIndexer(repo, config)

	ctx := context.Background()
	stats, err := indexer.IndexDirectory(ctx, tmpDir)
	if err != nil {
		t.Fatalf("IndexDirectory failed: %v", err)
	}

	// Should only index main.go
	if stats.FilesIndexed != 1 {
		t.Errorf("Expected 1 file indexed (excluding vendor/node_modules), got %d", stats.FilesIndexed)
	}
}

// TestIndexer_IncrementalIndex tests incremental indexing.
func TestIndexer_IncrementalIndex(t *testing.T) {
	tmpDir := t.TempDir()

	// Initial file
	mainPath := filepath.Join(tmpDir, "main.go")
	if err := os.WriteFile(mainPath, []byte(`package main

func main() {}
`), 0644); err != nil {
		t.Fatalf("Failed to write main.go: %v", err)
	}

	store, err := memory.NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	repo := NewRepository(store.DB())
	config := DefaultIndexerConfig()
	indexer := NewIndexer(repo, config)

	ctx := context.Background()

	// Initial full index
	stats1, err := indexer.IndexDirectory(ctx, tmpDir)
	if err != nil {
		t.Fatalf("Initial IndexDirectory failed: %v", err)
	}
	if stats1.FilesIndexed != 1 {
		t.Errorf("Expected 1 file indexed initially, got %d", stats1.FilesIndexed)
	}

	// Incremental index without changes
	stats2, err := indexer.IncrementalIndex(ctx, tmpDir)
	if err != nil {
		t.Fatalf("IncrementalIndex failed: %v", err)
	}
	if stats2.FilesIndexed != 0 {
		t.Errorf("Expected 0 files indexed (no changes), got %d", stats2.FilesIndexed)
	}

	// Modify file and run incremental
	if err := os.WriteFile(mainPath, []byte(`package main

func main() {
	println("updated")
}
`), 0644); err != nil {
		t.Fatalf("Failed to update main.go: %v", err)
	}

	stats3, err := indexer.IncrementalIndex(ctx, tmpDir)
	if err != nil {
		t.Fatalf("IncrementalIndex after update failed: %v", err)
	}
	if stats3.FilesIndexed != 1 {
		t.Errorf("Expected 1 file indexed (after update), got %d", stats3.FilesIndexed)
	}
}

// TestIndexer_GetStats tests getting index statistics.
func TestIndexer_GetStats(t *testing.T) {
	tmpDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(`package main

func main() {}

func helper() {}
`), 0644); err != nil {
		t.Fatalf("Failed to write main.go: %v", err)
	}

	store, err := memory.NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	repo := NewRepository(store.DB())
	config := DefaultIndexerConfig()
	indexer := NewIndexer(repo, config)

	ctx := context.Background()

	// Index first
	_, err = indexer.IndexDirectory(ctx, tmpDir)
	if err != nil {
		t.Fatalf("IndexDirectory failed: %v", err)
	}

	// Get stats
	stats, err := indexer.GetStats(ctx)
	if err != nil {
		t.Fatalf("GetStats failed: %v", err)
	}

	if stats.SymbolsFound < 3 {
		t.Errorf("Expected at least 3 symbols, got %d", stats.SymbolsFound)
	}
	if stats.FilesIndexed != 1 {
		t.Errorf("Expected 1 file indexed, got %d", stats.FilesIndexed)
	}
}

// TestIndexer_ClearIndex tests clearing the index.
func TestIndexer_ClearIndex(t *testing.T) {
	tmpDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(`package main

func main() {}
`), 0644); err != nil {
		t.Fatalf("Failed to write main.go: %v", err)
	}

	store, err := memory.NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	repo := NewRepository(store.DB())
	config := DefaultIndexerConfig()
	indexer := NewIndexer(repo, config)

	ctx := context.Background()

	// Index first
	_, err = indexer.IndexDirectory(ctx, tmpDir)
	if err != nil {
		t.Fatalf("IndexDirectory failed: %v", err)
	}

	// Verify symbols exist
	count, _ := repo.GetSymbolCount(ctx)
	if count == 0 {
		t.Error("Expected symbols after indexing")
	}

	// Clear index
	if err := indexer.ClearIndex(ctx); err != nil {
		t.Fatalf("ClearIndex failed: %v", err)
	}

	// Verify symbols cleared
	count, _ = repo.GetSymbolCount(ctx)
	if count != 0 {
		t.Errorf("Expected 0 symbols after clear, got %d", count)
	}
}

// TestIndexer_ParallelWorkers tests that parallel workers work correctly.
func TestIndexer_ParallelWorkers(t *testing.T) {
	tmpDir := t.TempDir()

	// Create many files to ensure parallel processing
	for i := 0; i < 10; i++ {
		content := `package main

func func` + string(rune('A'+i)) + `() {}
`
		if err := os.WriteFile(filepath.Join(tmpDir, "file"+string(rune('0'+i))+".go"), []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write file: %v", err)
		}
	}

	store, err := memory.NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	repo := NewRepository(store.DB())
	config := DefaultIndexerConfig()
	config.Workers = 4 // Use multiple workers
	indexer := NewIndexer(repo, config)

	ctx := context.Background()
	stats, err := indexer.IndexDirectory(ctx, tmpDir)
	if err != nil {
		t.Fatalf("IndexDirectory failed: %v", err)
	}

	if stats.FilesIndexed != 10 {
		t.Errorf("Expected 10 files indexed, got %d", stats.FilesIndexed)
	}
	if stats.SymbolsFound < 20 {
		t.Errorf("Expected at least 20 symbols (10 packages + 10 functions), got %d", stats.SymbolsFound)
	}
}

// TestIndexer_EmptyDirectory tests indexing an empty directory.
func TestIndexer_EmptyDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	store, err := memory.NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	repo := NewRepository(store.DB())
	config := DefaultIndexerConfig()
	indexer := NewIndexer(repo, config)

	ctx := context.Background()
	stats, err := indexer.IndexDirectory(ctx, tmpDir)
	if err != nil {
		t.Fatalf("IndexDirectory failed: %v", err)
	}

	if stats.FilesScanned != 0 {
		t.Errorf("Expected 0 files scanned, got %d", stats.FilesScanned)
	}
	if stats.FilesIndexed != 0 {
		t.Errorf("Expected 0 files indexed, got %d", stats.FilesIndexed)
	}
}

// TestIndexer_DefaultConfig tests default configuration.
func TestIndexer_DefaultConfig(t *testing.T) {
	config := DefaultIndexerConfig()

	if config.Workers <= 0 {
		t.Error("Workers should be > 0")
	}
	if config.BatchSize <= 0 {
		t.Error("BatchSize should be > 0")
	}
	if len(config.ExcludePatterns) == 0 {
		t.Error("ExcludePatterns should not be empty")
	}
	if config.IncludeTests {
		t.Error("IncludeTests should be false by default")
	}
}

// TestIndexer_CountSupportedFiles tests file counting for safety checks.
func TestIndexer_CountSupportedFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test files
	files := map[string]string{
		"main.go":      "package main\nfunc main() {}\n",
		"util.go":      "package main\nfunc helper() {}\n",
		"app.ts":       "export class App {}\n",
		"service.py":   "class Service: pass\n",
		"lib.rs":       "fn main() {}\n",
		"README.md":    "# Test\n",       // Should not be counted
		"data.json":    "{}",             // Should not be counted
		"main_test.go": "package main\n", // Test file, excluded by default
	}

	for name, content := range files {
		if err := os.WriteFile(filepath.Join(tmpDir, name), []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write %s: %v", name, err)
		}
	}

	// Create repository
	store, err := memory.NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	repo := NewRepository(store.DB())

	// Create indexer with default config (excludes test files)
	config := DefaultIndexerConfig()
	indexer := NewIndexer(repo, config)

	// Count files
	count, err := indexer.CountSupportedFiles(tmpDir)
	if err != nil {
		t.Fatalf("CountSupportedFiles failed: %v", err)
	}

	// Should count: main.go, util.go, app.ts, service.py, lib.rs = 5
	// Should NOT count: README.md, data.json, main_test.go
	expectedCount := 5
	if count != expectedCount {
		t.Errorf("Expected %d files, got %d", expectedCount, count)
	}

	// Test with IncludeTests=true
	config.IncludeTests = true
	indexer = NewIndexer(repo, config)
	count, err = indexer.CountSupportedFiles(tmpDir)
	if err != nil {
		t.Fatalf("CountSupportedFiles with IncludeTests failed: %v", err)
	}

	// Now should include main_test.go = 6
	expectedWithTests := 6
	if count != expectedWithTests {
		t.Errorf("Expected %d files with tests, got %d", expectedWithTests, count)
	}
}

// TestIndexer_MultiLanguageSupport tests indexing of TypeScript, Python, and Rust files.
func TestIndexer_MultiLanguageSupport(t *testing.T) {
	tmpDir := t.TempDir()

	files := map[string]string{
		// Go file
		"main.go": `package main

// Main is the entry point.
func main() {}
`,
		// TypeScript file
		"app.ts": `/**
 * UserService handles user operations.
 */
export class UserService {
    constructor(private db: Database) {}

    async getUser(id: string): Promise<User> {
        return this.db.findUser(id);
    }
}

export interface User {
    id: string;
    name: string;
}
`,
		// Python file
		"utils.py": `"""Utility functions for data processing."""

def calculate_sum(numbers: list[int]) -> int:
    """Calculate the sum of a list of numbers.

    Args:
        numbers: List of integers to sum.

    Returns:
        The sum of all numbers.
    """
    return sum(numbers)

class DataProcessor:
    """Processes data records."""

    def __init__(self, config: dict):
        self.config = config

    def process(self, data: list) -> list:
        """Process a list of data records."""
        return [self.transform(item) for item in data]
`,
		// Rust file
		"lib.rs": `//! Library for handling user data.

/// Represents a user in the system.
#[derive(Debug, Clone)]
pub struct User {
    pub id: u64,
    pub name: String,
    email: String,
}

impl User {
    /// Creates a new user.
    pub fn new(id: u64, name: String, email: String) -> Self {
        User { id, name, email }
    }
}

/// Trait for displayable items.
pub trait Displayable {
    fn display(&self) -> String;
}
`,
	}

	for name, content := range files {
		if err := os.WriteFile(filepath.Join(tmpDir, name), []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write %s: %v", name, err)
		}
	}

	// Create repository
	store, err := memory.NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	repo := NewRepository(store.DB())

	// Create indexer
	config := DefaultIndexerConfig()
	indexer := NewIndexer(repo, config)

	// Index directory
	ctx := context.Background()
	stats, err := indexer.IndexDirectory(ctx, tmpDir)
	if err != nil {
		t.Fatalf("IndexDirectory failed: %v", err)
	}

	// Verify we indexed all 4 files
	if stats.FilesScanned != 4 {
		t.Errorf("Expected 4 files scanned, got %d", stats.FilesScanned)
	}
	if stats.FilesIndexed != 4 {
		t.Errorf("Expected 4 files indexed, got %d", stats.FilesIndexed)
	}

	// Verify symbols from each language were extracted
	// Go: main package + main func = 2 symbols
	// TypeScript: UserService class + getUser method + User interface = 3 symbols
	// Python: calculate_sum func + DataProcessor class + process method = 3+ symbols
	// Rust: User struct + new method + Displayable trait = 3+ symbols
	if stats.SymbolsFound < 8 {
		t.Errorf("Expected at least 8 symbols from 4 languages, got %d", stats.SymbolsFound)
	}

	// Verify symbols are in database with correct languages
	langCount := make(map[string]int)
	symbols, _ := repo.FindSymbolsByFile(ctx, "main.go")
	langCount["go"] = len(symbols)

	symbols, _ = repo.FindSymbolsByFile(ctx, "app.ts")
	langCount["typescript"] = len(symbols)

	symbols, _ = repo.FindSymbolsByFile(ctx, "utils.py")
	langCount["python"] = len(symbols)

	symbols, _ = repo.FindSymbolsByFile(ctx, "lib.rs")
	langCount["rust"] = len(symbols)

	if langCount["go"] == 0 {
		t.Error("No Go symbols found")
	}
	if langCount["typescript"] == 0 {
		t.Error("No TypeScript symbols found")
	}
	if langCount["python"] == 0 {
		t.Error("No Python symbols found")
	}
	if langCount["rust"] == 0 {
		t.Error("No Rust symbols found")
	}

	t.Logf("Symbols by language: Go=%d, TypeScript=%d, Python=%d, Rust=%d",
		langCount["go"], langCount["typescript"], langCount["python"], langCount["rust"])
}
