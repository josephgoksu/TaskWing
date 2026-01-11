// Package codeintel provides code intelligence capabilities.
package codeintel

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/josephgoksu/TaskWing/internal/codeintel/parser"
	"github.com/josephgoksu/TaskWing/internal/knowledge"
	"github.com/josephgoksu/TaskWing/internal/llm"
)

// IndexerConfig holds configuration for the indexer.
type IndexerConfig struct {
	// Workers is the number of parallel workers. Defaults to runtime.NumCPU().
	Workers int

	// GenerateEmbeddings enables embedding generation for symbols.
	GenerateEmbeddings bool

	// LLMConfig is required when GenerateEmbeddings is true.
	LLMConfig llm.Config

	// ExcludePatterns are glob patterns for directories to skip.
	ExcludePatterns []string

	// IncludeTests controls whether test files are indexed.
	IncludeTests bool

	// BatchSize is the number of symbols to insert in a single transaction.
	BatchSize int

	// OnProgress is called with progress updates.
	OnProgress func(stats IndexStats)
}

// DefaultIndexerConfig returns sensible defaults for indexing.
func DefaultIndexerConfig() IndexerConfig {
	return IndexerConfig{
		Workers:   runtime.NumCPU(),
		BatchSize: 100,
		ExcludePatterns: []string{
			"vendor",
			"node_modules",
			".git",
			".taskwing",
			"testdata",
		},
		IncludeTests: false,
	}
}

// Indexer processes source files and populates the symbol database.
type Indexer struct {
	repo     Repository
	config   IndexerConfig
	registry *parser.ParserRegistry
}

// NewIndexer creates a new indexer with the given repository and config.
func NewIndexer(repo Repository, config IndexerConfig) *Indexer {
	if config.Workers <= 0 {
		config.Workers = runtime.NumCPU()
	}
	if config.BatchSize <= 0 {
		config.BatchSize = 100
	}
	return &Indexer{
		repo:   repo,
		config: config,
	}
}

// fileJob represents a file to be parsed.
type fileJob struct {
	path string
}

// parseResult contains the result of parsing a file.
type parseResult struct {
	path      string
	symbols   []Symbol
	relations []SymbolRelation
	err       error
}

// IndexDirectory indexes all supported source files in the given directory.
// Supports Go, TypeScript, Python, and Rust files.
func (idx *Indexer) IndexDirectory(ctx context.Context, rootPath string) (*IndexStats, error) {
	start := time.Now()
	stats := &IndexStats{}

	// Create parser registry with all language parsers
	idx.registry = parser.NewDefaultRegistry(rootPath)

	// Find all supported source files
	files, err := idx.findSupportedFiles(rootPath)
	if err != nil {
		return nil, fmt.Errorf("finding files: %w", err)
	}
	stats.FilesScanned = len(files)

	if len(files) == 0 {
		stats.Duration = time.Since(start)
		return stats, nil
	}

	// Create channels for work distribution
	jobs := make(chan fileJob, len(files))
	results := make(chan parseResult, len(files))

	// Start worker pool
	var wg sync.WaitGroup
	for i := 0; i < idx.config.Workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			idx.worker(ctx, jobs, results)
		}()
	}

	// Send jobs
	for _, file := range files {
		jobs <- fileJob{path: file}
	}
	close(jobs)

	// Wait for workers and close results
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results and build symbol map for relation resolution
	allSymbols := make([]Symbol, 0)
	allRelations := make([]SymbolRelation, 0)
	symbolMap := make(map[string]uint32) // key -> symbol ID

	var filesIndexed, filesSkipped int32
	var parseErrors []string

	for result := range results {
		if result.err != nil {
			atomic.AddInt32(&filesSkipped, 1)
			parseErrors = append(parseErrors, fmt.Sprintf("%s: %v", result.path, result.err))
			continue
		}

		atomic.AddInt32(&filesIndexed, 1)
		allSymbols = append(allSymbols, result.symbols...)
		allRelations = append(allRelations, result.relations...)
	}

	stats.FilesIndexed = int(filesIndexed)
	stats.FilesSkipped = int(filesSkipped)
	stats.Errors = parseErrors

	// Insert symbols into database and build ID map
	for i := range allSymbols {
		id, err := idx.repo.UpsertSymbol(ctx, &allSymbols[i])
		if err != nil {
			stats.Errors = append(stats.Errors, fmt.Sprintf("insert symbol %s: %v", allSymbols[i].Name, err))
			continue
		}
		allSymbols[i].ID = id
		stats.SymbolsFound++

		// Build symbol key for relation resolution
		key := buildSymbolKeyForIndexer(allSymbols[i].ModulePath, allSymbols[i].Name, allSymbols[i].Kind)
		symbolMap[key] = id
	}

	// Resolve and insert relations
	for _, rel := range allRelations {
		// Try to resolve target symbol from metadata
		if meta := rel.Metadata; meta != nil {
			if calleeName, ok := meta["calleeName"].(string); ok {
				// Look for target in any module
				for symKey, symID := range symbolMap {
					if strings.HasSuffix(symKey, ":"+calleeName) {
						rel.ToSymbolID = symID
						break
					}
				}
			}
		}

		// Only insert if we have valid source (from parser index) and resolved target
		if rel.ToSymbolID > 0 {
			// FromSymbolID from parser is an index, need to map to actual ID
			if int(rel.FromSymbolID) < len(allSymbols) {
				rel.FromSymbolID = allSymbols[rel.FromSymbolID].ID
				if err := idx.repo.UpsertRelation(ctx, &rel); err != nil {
					stats.Errors = append(stats.Errors, fmt.Sprintf("insert relation: %v", err))
					continue
				}
				stats.RelationsFound++
			}
		}
	}

	// Generate embeddings if enabled
	if idx.config.GenerateEmbeddings {
		embeddingsGenerated, embeddingErrors := idx.generateEmbeddings(ctx, allSymbols)
		stats.EmbeddingsGen = embeddingsGenerated
		stats.Errors = append(stats.Errors, embeddingErrors...)
	}

	stats.Duration = time.Since(start)

	// Call progress callback
	if idx.config.OnProgress != nil {
		idx.config.OnProgress(*stats)
	}

	return stats, nil
}

// worker processes file jobs from the jobs channel.
// H3 FIX: Added panic recovery to prevent single-file failures from crashing the entire indexer.
func (idx *Indexer) worker(ctx context.Context, jobs <-chan fileJob, results chan<- parseResult) {
	for job := range jobs {
		select {
		case <-ctx.Done():
			results <- parseResult{path: job.path, err: ctx.Err()}
			return
		default:
		}

		// H3 FIX: Recover from panics in parser to prevent entire indexer crash
		func() {
			defer func() {
				if r := recover(); r != nil {
					results <- parseResult{
						path: job.path,
						err:  fmt.Errorf("parser panic: %v", r),
					}
				}
			}()

			// Use the parser registry to get the appropriate parser for this file
			result, err := idx.registry.ParseFile(job.path)
			if err != nil {
				results <- parseResult{path: job.path, err: err}
				return
			}

			// Convert parser types to codeintel types
			symbols := convertSymbols(result.Symbols)
			relations := convertRelations(result.Relations)

			results <- parseResult{
				path:      job.path,
				symbols:   symbols,
				relations: relations,
			}
		}()
	}
}

// convertSymbols converts parser.Symbol to codeintel.Symbol
func convertSymbols(parserSymbols []parser.Symbol) []Symbol {
	symbols := make([]Symbol, len(parserSymbols))
	for i, ps := range parserSymbols {
		symbols[i] = Symbol{
			ID:           ps.ID,
			Name:         ps.Name,
			Kind:         SymbolKind(ps.Kind),
			FilePath:     ps.FilePath,
			StartLine:    ps.StartLine,
			EndLine:      ps.EndLine,
			Signature:    ps.Signature,
			DocComment:   ps.DocComment,
			ModulePath:   ps.ModulePath,
			Visibility:   ps.Visibility,
			Language:     ps.Language,
			FileHash:     ps.FileHash,
			Embedding:    ps.Embedding,
			LastModified: ps.LastModified,
		}
	}
	return symbols
}

// convertRelations converts parser.SymbolRelation to codeintel.SymbolRelation
func convertRelations(parserRelations []parser.SymbolRelation) []SymbolRelation {
	relations := make([]SymbolRelation, len(parserRelations))
	for i, pr := range parserRelations {
		relations[i] = SymbolRelation{
			FromSymbolID: pr.FromSymbolID,
			ToSymbolID:   pr.ToSymbolID,
			RelationType: RelationType(pr.RelationType),
			CallSiteLine: pr.CallSiteLine,
			Metadata:     pr.Metadata,
		}
	}
	return relations
}

// findSupportedFiles walks the directory and returns all supported source files to index.
// Supports: Go (.go), TypeScript (.ts, .tsx), JavaScript (.js, .jsx, .mjs, .cjs),
// Python (.py), and Rust (.rs) files.
func (idx *Indexer) findSupportedFiles(rootPath string) ([]string, error) {
	var files []string

	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Check if directory should be excluded
		if info.IsDir() {
			name := info.Name()

			// Skip hidden directories
			if strings.HasPrefix(name, ".") && name != "." {
				return filepath.SkipDir
			}

			// Check exclude patterns
			for _, pattern := range idx.config.ExcludePatterns {
				if matched, _ := filepath.Match(pattern, name); matched {
					return filepath.SkipDir
				}
			}

			return nil
		}

		// Check if the file can be parsed by any registered parser
		if idx.registry != nil && !idx.registry.CanParse(path) {
			return nil
		}

		// Skip test files unless configured to include them
		if !idx.config.IncludeTests {
			fileName := info.Name()
			// Go test files
			if strings.HasSuffix(fileName, "_test.go") {
				return nil
			}
			// TypeScript/JavaScript test files
			if strings.HasSuffix(fileName, ".test.ts") ||
				strings.HasSuffix(fileName, ".test.tsx") ||
				strings.HasSuffix(fileName, ".test.js") ||
				strings.HasSuffix(fileName, ".spec.ts") ||
				strings.HasSuffix(fileName, ".spec.tsx") ||
				strings.HasSuffix(fileName, ".spec.js") {
				return nil
			}
			// Python test files
			if strings.HasPrefix(fileName, "test_") && strings.HasSuffix(fileName, ".py") {
				return nil
			}
			// Rust test files
			if strings.HasSuffix(fileName, "_test.rs") {
				return nil
			}
		}

		files = append(files, path)
		return nil
	})

	return files, err
}

// generateEmbeddings creates embeddings for symbols without them.
func (idx *Indexer) generateEmbeddings(ctx context.Context, symbols []Symbol) (int, []string) {
	generated := 0
	var errors []string

	for _, sym := range symbols {
		// Skip if already has embedding
		if len(sym.Embedding) > 0 {
			continue
		}

		// Build text for embedding: name + signature + doc
		text := sym.Name
		if sym.Signature != "" {
			text += " " + sym.Signature
		}
		if sym.DocComment != "" {
			text += " " + sym.DocComment
		}

		embedding, err := knowledge.GenerateEmbedding(ctx, text, idx.config.LLMConfig)
		if err != nil {
			errors = append(errors, fmt.Sprintf("embedding for %s: %v", sym.Name, err))
			continue
		}

		if err := idx.repo.UpdateSymbolEmbedding(ctx, sym.ID, embedding); err != nil {
			errors = append(errors, fmt.Sprintf("store embedding for %s: %v", sym.Name, err))
			continue
		}

		generated++
	}

	return generated, errors
}

// IncrementalIndex re-indexes only files that have changed since last index.
// C1 FIX: Now properly inserts relations (was previously missing entirely).
func (idx *Indexer) IncrementalIndex(ctx context.Context, rootPath string) (*IndexStats, error) {
	start := time.Now()
	stats := &IndexStats{}

	// Create parser registry with all language parsers
	idx.registry = parser.NewDefaultRegistry(rootPath)

	// Find all supported source files
	allFiles, err := idx.findSupportedFiles(rootPath)
	if err != nil {
		return nil, fmt.Errorf("finding files: %w", err)
	}
	stats.FilesScanned = len(allFiles)

	// Filter to only changed files
	var changedFiles []string
	for _, file := range allFiles {
		content, err := os.ReadFile(file)
		if err != nil {
			continue
		}

		newHash := parser.ComputeHash(content)
		relPath, _ := filepath.Rel(rootPath, file)

		// Check if file hash matches existing symbols
		symbols, err := idx.repo.FindSymbolsByFile(ctx, relPath)
		if err != nil || len(symbols) == 0 {
			// File not indexed yet
			changedFiles = append(changedFiles, file)
			continue
		}

		// Check if hash changed
		if symbols[0].FileHash != newHash {
			// Delete old symbols for this file
			if err := idx.repo.DeleteSymbolsByFile(ctx, relPath); err != nil {
				stats.Errors = append(stats.Errors, fmt.Sprintf("delete old symbols for %s: %v", relPath, err))
			}
			changedFiles = append(changedFiles, file)
		}
	}

	if len(changedFiles) == 0 {
		stats.Duration = time.Since(start)
		return stats, nil
	}

	// Create channels for work distribution
	jobs := make(chan fileJob, len(changedFiles))
	results := make(chan parseResult, len(changedFiles))

	// Start worker pool
	var wg sync.WaitGroup
	for i := 0; i < idx.config.Workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			idx.worker(ctx, jobs, results)
		}()
	}

	// Send jobs
	for _, file := range changedFiles {
		jobs <- fileJob{path: file}
	}
	close(jobs)

	// Wait for workers and close results
	go func() {
		wg.Wait()
		close(results)
	}()

	// C1 FIX: Collect all symbols and relations first, then insert with proper ID mapping
	allSymbols := make([]Symbol, 0)
	allRelations := make([]SymbolRelation, 0)
	symbolMap := make(map[string]uint32) // key -> symbol ID

	// Collect results
	for result := range results {
		if result.err != nil {
			stats.FilesSkipped++
			stats.Errors = append(stats.Errors, fmt.Sprintf("%s: %v", result.path, result.err))
			continue
		}

		stats.FilesIndexed++
		allSymbols = append(allSymbols, result.symbols...)
		allRelations = append(allRelations, result.relations...)
	}

	// Insert symbols and build ID map
	for i := range allSymbols {
		id, err := idx.repo.UpsertSymbol(ctx, &allSymbols[i])
		if err != nil {
			stats.Errors = append(stats.Errors, fmt.Sprintf("insert symbol %s: %v", allSymbols[i].Name, err))
			continue
		}
		allSymbols[i].ID = id
		stats.SymbolsFound++

		// Build symbol key for relation resolution
		key := buildSymbolKeyForIndexer(allSymbols[i].ModulePath, allSymbols[i].Name, allSymbols[i].Kind)
		symbolMap[key] = id
	}

	// C1 FIX: Resolve and insert relations (was completely missing before!)
	for _, rel := range allRelations {
		// Try to resolve target symbol from metadata
		if meta := rel.Metadata; meta != nil {
			if calleeName, ok := meta["calleeName"].(string); ok {
				// Look for target in any module
				for symKey, symID := range symbolMap {
					if strings.HasSuffix(symKey, ":"+calleeName) {
						rel.ToSymbolID = symID
						break
					}
				}
			}
		}

		// Only insert if we have valid source and resolved target
		if rel.ToSymbolID > 0 {
			// FromSymbolID from parser is an index, need to map to actual ID
			if int(rel.FromSymbolID) < len(allSymbols) {
				rel.FromSymbolID = allSymbols[rel.FromSymbolID].ID
				if err := idx.repo.UpsertRelation(ctx, &rel); err != nil {
					stats.Errors = append(stats.Errors, fmt.Sprintf("insert relation: %v", err))
					continue
				}
				stats.RelationsFound++
			}
		}
	}

	stats.Duration = time.Since(start)
	return stats, nil
}

// ClearIndex removes all symbols and relations from the database.
// C5 FIX: Uses atomic ClearAllSymbols instead of fetch-then-delete-one-by-one
// which was prone to race conditions with concurrent indexing operations.
func (idx *Indexer) ClearIndex(ctx context.Context) error {
	return idx.repo.ClearAllSymbols(ctx)
}

// CountSupportedFiles returns the number of files that would be indexed.
// This is useful for safety checks before starting a potentially long index operation.
func (idx *Indexer) CountSupportedFiles(rootPath string) (int, error) {
	// Create parser registry to determine which files can be parsed
	idx.registry = parser.NewDefaultRegistry(rootPath)
	files, err := idx.findSupportedFiles(rootPath)
	if err != nil {
		return 0, err
	}
	return len(files), nil
}

// GetStats returns current index statistics.
func (idx *Indexer) GetStats(ctx context.Context) (*IndexStats, error) {
	symbolCount, err := idx.repo.GetSymbolCount(ctx)
	if err != nil {
		return nil, err
	}

	relationCount, err := idx.repo.GetRelationCount(ctx)
	if err != nil {
		return nil, err
	}

	fileCount, err := idx.repo.GetFileCount(ctx)
	if err != nil {
		return nil, err
	}

	return &IndexStats{
		SymbolsFound:   symbolCount,
		RelationsFound: relationCount,
		FilesIndexed:   fileCount,
	}, nil
}

// buildSymbolKeyForIndexer creates a unique key for symbol lookup.
func buildSymbolKeyForIndexer(modulePath, name string, kind SymbolKind) string {
	return fmt.Sprintf("%s:%s:%s", modulePath, kind, name)
}
