package app

import (
	"context"
	"fmt"
	"sort"

	"github.com/josephgoksu/TaskWing/internal/codeintel"
	"github.com/josephgoksu/TaskWing/internal/knowledge"
)

// SymbolResponse represents a code symbol in search results.
// This provides a JSON-safe representation of codeintel.Symbol.
type SymbolResponse struct {
	Name       string `json:"name"`
	Kind       string `json:"kind"`
	FilePath   string `json:"file_path"`
	StartLine  int    `json:"start_line"`
	EndLine    int    `json:"end_line"`
	Signature  string `json:"signature,omitempty"`
	DocComment string `json:"doc_comment,omitempty"`
	ModulePath string `json:"module_path,omitempty"`
	Visibility string `json:"visibility"`
	Language   string `json:"language"`
	Location   string `json:"location"` // "file:line" for easy navigation
}

// RecallResult contains the complete result of a knowledge search.
// This is the canonical response type used by both CLI and MCP.
type RecallResult struct {
	Query          string                   `json:"query"`
	RewrittenQuery string                   `json:"rewritten_query,omitempty"`
	Pipeline       string                   `json:"pipeline"`
	Results        []knowledge.NodeResponse `json:"results"`
	Symbols        []SymbolResponse         `json:"symbols,omitempty"`
	Total          int                      `json:"total"`
	TotalSymbols   int                      `json:"total_symbols,omitempty"`
	Answer         string                   `json:"answer,omitempty"`
	Warning        string                   `json:"warning,omitempty"`
}

// RecallOptions configures the behavior of a recall query.
type RecallOptions struct {
	Limit          int  // Maximum number of knowledge results (default: 5)
	SymbolLimit    int  // Maximum number of symbol results (default: 5)
	GenerateAnswer bool // Whether to generate a RAG answer
	IncludeSymbols bool // Whether to include code symbols in search (default: true)
}

// DefaultRecallOptions returns sensible defaults for recall queries.
func DefaultRecallOptions() RecallOptions {
	return RecallOptions{
		Limit:          5,
		SymbolLimit:    5,
		GenerateAnswer: false,
		IncludeSymbols: true,
	}
}

// RecallApp provides knowledge retrieval operations.
// This is THE implementation - CLI and MCP both call these methods.
type RecallApp struct {
	ctx *Context
}

// NewRecallApp creates a new recall application service.
func NewRecallApp(ctx *Context) *RecallApp {
	return &RecallApp{ctx: ctx}
}

// Query performs semantic search with optional RAG answer generation.
// This method encapsulates the entire search pipeline:
// 1. Query rewriting (if enabled)
// 2. Hybrid search (FTS + Vector) for knowledge
// 3. Symbol FTS search (if enabled)
// 4. Reranking (if enabled)
// 5. Graph expansion (if enabled)
// 6. Answer generation (if requested)
func (a *RecallApp) Query(ctx context.Context, query string, opts RecallOptions) (*RecallResult, error) {
	if opts.Limit <= 0 {
		opts.Limit = 5
	}
	if opts.SymbolLimit <= 0 {
		opts.SymbolLimit = 5
	}

	ks := knowledge.NewService(a.ctx.Repo, a.ctx.LLMCfg)
	cfg := ks.GetRetrievalConfig()

	// 1. Query rewriting
	searchQuery := query
	rewrittenQuery := ""
	if cfg.QueryRewriteEnabled {
		if rewritten, err := ks.RewriteQuery(ctx, query); err == nil && rewritten != query {
			searchQuery = rewritten
			rewrittenQuery = rewritten
		}
	}

	// 2. Build pipeline description for transparency
	pipeline := "FTS + Vector"
	if cfg.RerankingEnabled {
		pipeline += " + Rerank"
	}
	if cfg.GraphExpansionEnabled {
		pipeline += " + Graph"
	}
	if opts.IncludeSymbols {
		pipeline += " + Symbols"
	}

	// 3. Execute knowledge search (hybrid + rerank + graph expansion)
	scored, err := ks.Search(ctx, searchQuery, opts.Limit)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	// 4. Convert results to response format (strips embeddings)
	results := make([]knowledge.NodeResponse, 0, len(scored))
	for _, sn := range scored {
		results = append(results, knowledge.ScoredNodeToResponse(sn))
	}

	// 5. Search for code symbols (if enabled and database available)
	var symbols []SymbolResponse
	if opts.IncludeSymbols {
		symbols = a.searchSymbols(ctx, searchQuery, opts.SymbolLimit)
	}

	// 6. Generate RAG answer if requested
	var answer, warning string
	if opts.GenerateAnswer && len(scored) > 0 {
		// Use original query for answer generation (more natural)
		ans, err := ks.Ask(ctx, query, scored)
		if err != nil {
			warning = fmt.Sprintf("Answer unavailable: %v", err)
		} else {
			answer = ans
		}
	}

	return &RecallResult{
		Query:          query,
		RewrittenQuery: rewrittenQuery,
		Pipeline:       pipeline,
		Results:        results,
		Symbols:        symbols,
		Total:          len(results),
		TotalSymbols:   len(symbols),
		Answer:         answer,
		Warning:        warning,
	}, nil
}

// searchSymbols searches the code intelligence index for matching symbols.
// It prioritizes public symbols over private ones.
func (a *RecallApp) searchSymbols(ctx context.Context, query string, limit int) []SymbolResponse {
	// Get database handle from repository
	store := a.ctx.Repo.GetDB()
	if store == nil {
		return nil
	}
	db := store.DB()
	if db == nil {
		return nil
	}

	// Create codeintel repository and search
	codeRepo := codeintel.NewRepository(db)
	symbols, err := codeRepo.SearchSymbolsFTS(ctx, query, limit*2) // Get extra for sorting
	if err != nil {
		return nil // Silent failure - symbols are supplementary
	}

	// Sort: public symbols first, then by name
	sort.Slice(symbols, func(i, j int) bool {
		// Public > Private
		if symbols[i].Visibility != symbols[j].Visibility {
			return symbols[i].Visibility == "public"
		}
		// Then alphabetically
		return symbols[i].Name < symbols[j].Name
	})

	// Limit results
	if len(symbols) > limit {
		symbols = symbols[:limit]
	}

	// Convert to response format
	responses := make([]SymbolResponse, len(symbols))
	for i, s := range symbols {
		responses[i] = SymbolResponse{
			Name:       s.Name,
			Kind:       string(s.Kind),
			FilePath:   s.FilePath,
			StartLine:  s.StartLine,
			EndLine:    s.EndLine,
			Signature:  s.Signature,
			DocComment: s.DocComment,
			ModulePath: s.ModulePath,
			Visibility: s.Visibility,
			Language:   s.Language,
			Location:   fmt.Sprintf("%s:%d", s.FilePath, s.StartLine),
		}
	}

	return responses
}

// Summary returns a high-level overview of the project's knowledge base.
// Use this when no query is provided.
func (a *RecallApp) Summary(ctx context.Context) (*knowledge.ProjectSummary, error) {
	ks := knowledge.NewService(a.ctx.Repo, a.ctx.LLMCfg)
	summary, err := ks.GetProjectSummary(ctx)
	if err != nil {
		return nil, fmt.Errorf("get project summary: %w", err)
	}
	return &summary, nil
}
