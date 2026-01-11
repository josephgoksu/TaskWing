package codeintel

import (
	"context"
	"fmt"
	"math"
	"sort"

	"github.com/josephgoksu/TaskWing/internal/knowledge"
	"github.com/josephgoksu/TaskWing/internal/llm"
)

// QueryConfig holds configuration for the query service.
type QueryConfig struct {
	// FTSWeight is the weight for FTS5 keyword matches (default 0.3).
	FTSWeight float32

	// VectorWeight is the weight for vector similarity matches (default 0.7).
	VectorWeight float32

	// VectorThreshold is the minimum vector similarity to include (default 0.5).
	VectorThreshold float32

	// MinResultThreshold is the minimum combined score to include (default 0.1).
	MinResultThreshold float32

	// DefaultLimit is the default number of results to return.
	DefaultLimit int

	// MaxImpactDepth is the maximum depth for impact analysis (default 5).
	MaxImpactDepth int
}

// DefaultQueryConfig returns sensible defaults for query configuration.
func DefaultQueryConfig() QueryConfig {
	return QueryConfig{
		FTSWeight:          0.3,
		VectorWeight:       0.7,
		VectorThreshold:    0.5,
		MinResultThreshold: 0.1,
		DefaultLimit:       20,
		MaxImpactDepth:     5,
	}
}

// QueryService provides hybrid search and impact analysis for code symbols.
// It combines FTS5 lexical search with vector similarity for best results.
type QueryService struct {
	repo   Repository
	llmCfg llm.Config
	config QueryConfig
}

// NewQueryService creates a new query service with default configuration.
func NewQueryService(repo Repository, llmCfg llm.Config) *QueryService {
	return &QueryService{
		repo:   repo,
		llmCfg: llmCfg,
		config: DefaultQueryConfig(),
	}
}

// NewQueryServiceWithConfig creates a new query service with custom configuration.
func NewQueryServiceWithConfig(repo Repository, llmCfg llm.Config, config QueryConfig) *QueryService {
	return &QueryService{
		repo:   repo,
		llmCfg: llmCfg,
		config: config,
	}
}

// HybridSearch performs a combined FTS5 and vector similarity search.
// Returns ranked results combining text match (FTS5) and semantic meaning (embeddings).
//
// The hybrid approach ensures:
// 1. Exact name matches rank highly (via FTS5)
// 2. Semantically similar code is found even with different naming (via vectors)
// 3. Combined scoring balances precision and recall
func (qs *QueryService) HybridSearch(ctx context.Context, query string, limit int) ([]SymbolSearchResult, error) {
	if limit <= 0 {
		limit = qs.config.DefaultLimit
	}

	// Collect candidates from both search methods
	scoreByID := make(map[uint32]float32)
	symbolByID := make(map[uint32]*Symbol)
	sourceByID := make(map[uint32]string)

	// 1. FTS5 keyword search (fast, no API call)
	ftsResults, err := qs.repo.SearchSymbolsFTS(ctx, query, limit*2)
	if err != nil {
		// FTS errors are non-fatal - vector search can still work
		// This allows graceful degradation if FTS5 index is unavailable
	} else {
		for i := range ftsResults {
			sym := &ftsResults[i]
			// FTS5 returns results ordered by BM25, so we assign decreasing scores
			// based on position (first result gets highest score)
			ftsScore := float32(1.0) - float32(i)/float32(len(ftsResults)+1)
			scoreByID[sym.ID] = ftsScore * qs.config.FTSWeight
			symbolByID[sym.ID] = sym
			sourceByID[sym.ID] = "fts"
		}
	}

	// 2. Vector similarity search (semantic)
	queryEmbedding, embErr := knowledge.GenerateEmbedding(ctx, query, qs.llmCfg)
	if embErr == nil && len(queryEmbedding) > 0 {
		symbolsWithEmb, err := qs.repo.ListSymbolsWithEmbeddings(ctx)
		if err == nil {
			for i := range symbolsWithEmb {
				sym := &symbolsWithEmb[i]
				if len(sym.Embedding) == 0 {
					continue
				}

				vectorScore := cosineSimilarity(queryEmbedding, sym.Embedding)
				if vectorScore < qs.config.VectorThreshold {
					continue // Skip low-relevance results
				}

				if _, exists := symbolByID[sym.ID]; !exists {
					symbolByID[sym.ID] = sym
					scoreByID[sym.ID] = 0
					sourceByID[sym.ID] = "vector"
				} else {
					// Found in both FTS and vector - mark as hybrid
					sourceByID[sym.ID] = "hybrid"
				}
				scoreByID[sym.ID] += vectorScore * qs.config.VectorWeight
			}
		}
	}

	// 3. Merge, filter low-confidence, and sort by combined score
	var results []SymbolSearchResult
	for id, score := range scoreByID {
		// Filter out noise: only include results above minimum threshold
		if score < qs.config.MinResultThreshold {
			continue
		}
		if sym, ok := symbolByID[id]; ok {
			results = append(results, SymbolSearchResult{
				Symbol: *sym,
				Score:  score,
				Source: sourceByID[id],
			})
		}
	}

	// Sort by score descending
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	// Limit results
	if len(results) > limit {
		results = results[:limit]
	}

	return results, nil
}

// SearchByKind performs hybrid search filtered to a specific symbol kind.
// Useful for finding only functions, only structs, etc.
func (qs *QueryService) SearchByKind(ctx context.Context, query string, kind SymbolKind, limit int) ([]SymbolSearchResult, error) {
	// Get all results then filter by kind
	// This is simpler than adding kind filtering to both FTS and vector search
	allResults, err := qs.HybridSearch(ctx, query, limit*3) // Fetch more to allow for filtering
	if err != nil {
		return nil, err
	}

	var filtered []SymbolSearchResult
	for _, r := range allResults {
		if r.Symbol.Kind == kind {
			filtered = append(filtered, r)
			if len(filtered) >= limit {
				break
			}
		}
	}

	return filtered, nil
}

// SearchByFile performs hybrid search filtered to a specific file.
func (qs *QueryService) SearchByFile(ctx context.Context, query string, filePath string, limit int) ([]SymbolSearchResult, error) {
	allResults, err := qs.HybridSearch(ctx, query, limit*3)
	if err != nil {
		return nil, err
	}

	var filtered []SymbolSearchResult
	for _, r := range allResults {
		if r.Symbol.FilePath == filePath {
			filtered = append(filtered, r)
			if len(filtered) >= limit {
				break
			}
		}
	}

	return filtered, nil
}

// AnalyzeImpact finds all symbols that would be affected by changing a given symbol.
// Uses recursive CTEs to traverse the call graph and find all downstream consumers.
//
// This is critical for understanding the blast radius of code changes:
// - Who calls this function?
// - Who calls those callers? (and so on, up to maxDepth)
// - What interfaces does this type implement?
//
// H2 FIX: Deduplicates symbols that appear at multiple depths in cyclic graphs,
// keeping only the first occurrence (lowest depth) for each symbol.
func (qs *QueryService) AnalyzeImpact(ctx context.Context, symbolID uint32, maxDepth int) (*ImpactAnalysis, error) {
	if maxDepth <= 0 {
		maxDepth = qs.config.MaxImpactDepth
	}

	// Get the source symbol for context
	sourceSymbol, err := qs.repo.GetSymbol(ctx, symbolID)
	if err != nil {
		return nil, fmt.Errorf("get source symbol: %w", err)
	}

	// Use repository's recursive CTE-based impact analysis
	impactNodes, err := qs.repo.GetImpactRadius(ctx, symbolID, maxDepth)
	if err != nil {
		return nil, fmt.Errorf("get impact radius: %w", err)
	}

	// H2 FIX: Deduplicate symbols that appear at multiple depths (cycles in call graph)
	// Keep only the first occurrence (lowest depth) for each symbol ID
	seenSymbols := make(map[uint32]bool)
	var dedupedNodes []ImpactNode
	for _, node := range impactNodes {
		if seenSymbols[node.Symbol.ID] {
			continue // Skip duplicate - already seen at a lower depth
		}
		seenSymbols[node.Symbol.ID] = true
		dedupedNodes = append(dedupedNodes, node)
	}

	// Build the analysis result
	analysis := &ImpactAnalysis{
		Source:        *sourceSymbol,
		AffectedCount: len(dedupedNodes),
		MaxDepth:      maxDepth,
	}

	// Group affected symbols by depth for clarity
	analysis.ByDepth = make(map[int][]Symbol)
	uniqueFiles := make(map[string]bool)

	for _, node := range dedupedNodes {
		analysis.Affected = append(analysis.Affected, node)
		analysis.ByDepth[node.Depth] = append(analysis.ByDepth[node.Depth], node.Symbol)
		uniqueFiles[node.Symbol.FilePath] = true
	}

	analysis.AffectedFiles = len(uniqueFiles)

	return analysis, nil
}

// ImpactAnalysis holds the result of an impact analysis.
type ImpactAnalysis struct {
	Source        Symbol            `json:"source"`        // The symbol being analyzed
	Affected      []ImpactNode      `json:"affected"`      // All affected symbols with depth
	AffectedCount int               `json:"affectedCount"` // Total count of affected symbols
	AffectedFiles int               `json:"affectedFiles"` // Number of files affected
	MaxDepth      int               `json:"maxDepth"`      // Maximum traversal depth used
	ByDepth       map[int][]Symbol  `json:"byDepth"`       // Symbols grouped by distance
}

// FindSymbol looks up a symbol by ID.
func (qs *QueryService) FindSymbol(ctx context.Context, id uint32) (*Symbol, error) {
	return qs.repo.GetSymbol(ctx, id)
}

// FindSymbolByName finds symbols with a specific name.
// Returns all matches across all files/modules.
func (qs *QueryService) FindSymbolByName(ctx context.Context, name string) ([]Symbol, error) {
	return qs.repo.FindSymbolsByName(ctx, name, nil)
}

// FindSymbolByNameAndLang finds symbols with a specific name in a specific language.
func (qs *QueryService) FindSymbolByNameAndLang(ctx context.Context, name, lang string) ([]Symbol, error) {
	return qs.repo.FindSymbolsByName(ctx, name, &lang)
}

// GetCallers returns all symbols that call the given symbol.
func (qs *QueryService) GetCallers(ctx context.Context, symbolID uint32) ([]Symbol, error) {
	return qs.repo.GetCallers(ctx, symbolID)
}

// GetCallees returns all symbols called by the given symbol.
func (qs *QueryService) GetCallees(ctx context.Context, symbolID uint32) ([]Symbol, error) {
	return qs.repo.GetCallees(ctx, symbolID)
}

// GetImplementations returns all types that implement a given interface.
func (qs *QueryService) GetImplementations(ctx context.Context, interfaceID uint32) ([]Symbol, error) {
	return qs.repo.GetImplementations(ctx, interfaceID)
}

// GetSymbolsInFile returns all symbols defined in a file.
func (qs *QueryService) GetSymbolsInFile(ctx context.Context, filePath string) ([]Symbol, error) {
	return qs.repo.FindSymbolsByFile(ctx, filePath)
}

// GetStats returns current index statistics.
func (qs *QueryService) GetStats(ctx context.Context) (*IndexStats, error) {
	symbolCount, err := qs.repo.GetSymbolCount(ctx)
	if err != nil {
		return nil, err
	}

	relationCount, err := qs.repo.GetRelationCount(ctx)
	if err != nil {
		return nil, err
	}

	fileCount, err := qs.repo.GetFileCount(ctx)
	if err != nil {
		return nil, err
	}

	return &IndexStats{
		SymbolsFound:   symbolCount,
		RelationsFound: relationCount,
		FilesIndexed:   fileCount,
	}, nil
}

// cosineSimilarity computes the cosine similarity between two vectors.
// Returns a value between -1 and 1, where 1 means identical.
func cosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}

	var dot, normA, normB float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return float32(dot / (math.Sqrt(normA) * math.Sqrt(normB)))
}
