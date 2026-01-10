package app

import (
	"context"
	"fmt"

	"github.com/josephgoksu/TaskWing/internal/knowledge"
)

// RecallResult contains the complete result of a knowledge search.
// This is the canonical response type used by both CLI and MCP.
type RecallResult struct {
	Query          string                   `json:"query"`
	RewrittenQuery string                   `json:"rewritten_query,omitempty"`
	Pipeline       string                   `json:"pipeline"`
	Results        []knowledge.NodeResponse `json:"results"`
	Total          int                      `json:"total"`
	Answer         string                   `json:"answer,omitempty"`
	Warning        string                   `json:"warning,omitempty"`
}

// RecallOptions configures the behavior of a recall query.
type RecallOptions struct {
	Limit          int  // Maximum number of results (default: 5)
	GenerateAnswer bool // Whether to generate a RAG answer
}

// DefaultRecallOptions returns sensible defaults for recall queries.
func DefaultRecallOptions() RecallOptions {
	return RecallOptions{
		Limit:          5,
		GenerateAnswer: false,
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
// 2. Hybrid search (FTS + Vector)
// 3. Reranking (if enabled)
// 4. Graph expansion (if enabled)
// 5. Answer generation (if requested)
func (a *RecallApp) Query(ctx context.Context, query string, opts RecallOptions) (*RecallResult, error) {
	if opts.Limit <= 0 {
		opts.Limit = 5
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

	// 3. Execute search (hybrid + rerank + graph expansion)
	scored, err := ks.Search(ctx, searchQuery, opts.Limit)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	// 4. Convert results to response format (strips embeddings)
	results := make([]knowledge.NodeResponse, 0, len(scored))
	for _, sn := range scored {
		results = append(results, knowledge.ScoredNodeToResponse(sn))
	}

	// 5. Generate RAG answer if requested
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
		Total:          len(results),
		Answer:         answer,
		Warning:        warning,
	}, nil
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
