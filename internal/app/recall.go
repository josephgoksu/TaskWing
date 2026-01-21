package app

import (
	"context"
	"fmt"
	"io"
	"log"
	"sort"
	"strings"

	"github.com/cloudwego/eino/schema"
	"github.com/josephgoksu/TaskWing/internal/codeintel"
	"github.com/josephgoksu/TaskWing/internal/knowledge"
	"github.com/josephgoksu/TaskWing/internal/llm"
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
	Limit          int       // Maximum number of knowledge results (default: 5)
	SymbolLimit    int       // Maximum number of symbol results (default: 5)
	GenerateAnswer bool      // Whether to generate a RAG answer
	IncludeSymbols bool      // Whether to include code symbols in search (default: true)
	NoRewrite      bool      // Disable LLM query rewriting (faster, no API call)
	DisableVector  bool      // Disable vector search (FTS-only, no embeddings)
	DisableRerank  bool      // Disable reranking (skip TEI reranker)
	StreamWriter   io.Writer // If set, stream RAG answer tokens to this writer

	// Workspace filtering for monorepo support
	Workspace   string // Filter by workspace ('root' for global, or service name like 'osprey')
	IncludeRoot bool   // When Workspace is set, also include 'root' workspace nodes (default: true)
}

// DefaultRecallOptions returns sensible defaults for recall queries.
func DefaultRecallOptions() RecallOptions {
	return RecallOptions{
		Limit:          5,
		SymbolLimit:    5,
		GenerateAnswer: false,
		IncludeSymbols: true,
		NoRewrite:      false,
		DisableVector:  false,
		DisableRerank:  false,
		StreamWriter:   nil,
		Workspace:      "",   // Empty means all workspaces
		IncludeRoot:    true, // Always include root/global knowledge by default
	}
}

// ValidateWorkspace checks if a workspace string is valid.
// Valid workspaces are: empty string (all), "root", or alphanumeric service names.
func ValidateWorkspace(workspace string) error {
	if workspace == "" || workspace == "root" {
		return nil
	}
	// Allow alphanumeric, hyphens, underscores (common service naming conventions)
	for _, r := range workspace {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || r == '-' || r == '_') {
			return fmt.Errorf("invalid workspace name %q: only alphanumeric characters, hyphens, and underscores allowed", workspace)
		}
	}
	return nil
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

	retrievalCfg := knowledge.LoadRetrievalConfig()
	if opts.DisableVector {
		retrievalCfg.VectorWeight = 0
		retrievalCfg.FTSWeight = 1.0
	}
	if opts.DisableRerank {
		retrievalCfg.RerankingEnabled = false
	}
	embeddingConfigWarning := ""
	if !opts.DisableVector && retrievalCfg.VectorWeight > 0 {
		embeddingProvider := a.ctx.LLMCfg.EmbeddingProvider
		if embeddingProvider == "" {
			embeddingProvider = a.ctx.LLMCfg.Provider
		}
		embeddingAPIKey := a.ctx.LLMCfg.EmbeddingAPIKey
		if embeddingAPIKey == "" {
			embeddingAPIKey = a.ctx.LLMCfg.APIKey
		}
		supportsEmbeddings := embeddingProvider == llm.ProviderOpenAI ||
			embeddingProvider == llm.ProviderOllama ||
			embeddingProvider == llm.ProviderGemini ||
			embeddingProvider == llm.ProviderTEI
		if !supportsEmbeddings {
			retrievalCfg.VectorWeight = 0
			retrievalCfg.FTSWeight = 1.0
			if embeddingProvider == "" {
				embeddingConfigWarning = "Embeddings disabled: no embedding provider configured"
			} else {
				embeddingConfigWarning = fmt.Sprintf("Embeddings disabled: provider %s does not support embeddings", embeddingProvider)
			}
		} else if (embeddingProvider == llm.ProviderOpenAI || embeddingProvider == llm.ProviderGemini) && embeddingAPIKey == "" {
			retrievalCfg.VectorWeight = 0
			retrievalCfg.FTSWeight = 1.0
			embeddingConfigWarning = fmt.Sprintf("Embeddings disabled: missing API key for %s", embeddingProvider)
		}
	}
	var embeddingStatsChecked bool
	var embeddingStatsMessage string
	if !opts.DisableVector && retrievalCfg.VectorWeight > 0 {
		if stats, err := a.ctx.Repo.GetEmbeddingStats(); err == nil && stats != nil {
			embeddingStatsChecked = true
			if stats.TotalNodes > 0 && stats.NodesWithEmbeddings == 0 {
				// No embeddings exist - skip vector search to avoid wasted LLM calls
				retrievalCfg.VectorWeight = 0
				retrievalCfg.FTSWeight = 1.0
			}
			if stats.TotalNodes > 0 {
				if stats.MixedDimensions {
					msg := fmt.Sprintf("Embedding issues: mixed embedding dimensions detected (found %d-dim, but others exist)", stats.EmbeddingDimension)
					if stats.NodesWithoutEmbeddings > 0 {
						msg += fmt.Sprintf("; %d nodes missing embeddings", stats.NodesWithoutEmbeddings)
					}
					msg += ". Run 'tw memory rebuild-embeddings' to fix."
					embeddingStatsMessage = msg
				} else if stats.NodesWithoutEmbeddings > 0 {
					embeddingStatsMessage = fmt.Sprintf("%d nodes missing embeddings. Run 'tw memory generate-embeddings' to backfill.", stats.NodesWithoutEmbeddings)
				}
			}
		}
	}
	ks := knowledge.NewServiceWithConfig(a.ctx.Repo, a.ctx.LLMCfg, retrievalCfg)
	cfg := ks.GetRetrievalConfig()

	// 1. Query rewriting (skip if NoRewrite option is set)
	searchQuery := query
	rewrittenQuery := ""
	if cfg.QueryRewriteEnabled && !opts.NoRewrite {
		if rewritten, err := ks.RewriteQuery(ctx, query); err == nil && rewritten != query {
			searchQuery = rewritten
			rewrittenQuery = rewritten
		}
	}

	// 2. Build pipeline description for transparency
	var pipelineParts []string
	if cfg.FTSWeight > 0 {
		pipelineParts = append(pipelineParts, "FTS")
	}
	if cfg.VectorWeight > 0 {
		pipelineParts = append(pipelineParts, "Vector")
	}
	if cfg.QueryRewriteEnabled && !opts.NoRewrite {
		pipelineParts = append(pipelineParts, "Rewrite")
	}
	if cfg.RerankingEnabled {
		pipelineParts = append(pipelineParts, "Rerank")
	}
	if cfg.GraphExpansionEnabled {
		pipelineParts = append(pipelineParts, "Graph")
	}
	if opts.IncludeSymbols {
		pipelineParts = append(pipelineParts, "Symbols")
	}
	pipeline := strings.Join(pipelineParts, " + ")
	if pipeline == "" {
		pipeline = "None"
	}

	// 2b. Embedding consistency warning (surface missing/mixed embeddings)
	var warnings []string
	if embeddingConfigWarning != "" {
		warnings = append(warnings, embeddingConfigWarning)
	}
	if embeddingStatsChecked && embeddingStatsMessage != "" {
		warnings = append(warnings, embeddingStatsMessage)
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

	// 6. Generate RAG answer if requested (Code-Based RAG)
	var answer string
	if opts.GenerateAnswer {
		// Fetch actual source code for symbols to ground the answer
		// Use same search as UI symbols to ensure consistency (what you see = what RAG uses)
		var codeSnippets []CodeSnippet
		if a.ctx.BasePath != "" && len(symbols) > 0 {
			// Convert SymbolResponse back to raw symbols for source fetching
			// This ensures RAG uses the SAME symbols shown in the UI
			rawSymbols := a.getRawSymbols(ctx, searchQuery, len(symbols))
			if len(rawSymbols) > 0 {
				fetcher := NewSourceFetcher(a.ctx.BasePath)
				codeSnippets = fetcher.FetchContext(rawSymbols)
			}
		}

		// Generate answer with both knowledge nodes and code snippets
		ans, err := a.generateRAGAnswer(ctx, query, scored, codeSnippets, opts.StreamWriter)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("Answer unavailable: %v", err))
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
		Warning:        strings.Join(warnings, " "),
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

// getRawSymbols retrieves raw codeintel.Symbol objects for source code fetching.
// This is the core symbol retrieval - searchSymbols wraps it with response conversion.
func (a *RecallApp) getRawSymbols(ctx context.Context, query string, limit int) []codeintel.Symbol {
	store := a.ctx.Repo.GetDB()
	if store == nil {
		return nil
	}
	db := store.DB()
	if db == nil {
		return nil
	}

	codeRepo := codeintel.NewRepository(db)
	symbols, err := codeRepo.SearchSymbolsFTS(ctx, query, limit)
	if err != nil {
		return nil
	}
	return symbols
}

// generateRAGAnswer creates an answer using both knowledge nodes and code snippets.
// This is the core of Code-Based RAG: answers are grounded in actual source code.
// If streamWriter is provided, tokens are streamed as they arrive.
func (a *RecallApp) generateRAGAnswer(ctx context.Context, query string, nodes []knowledge.ScoredNode, snippets []CodeSnippet, streamWriter io.Writer) (string, error) {
	// Build context from both sources
	var contextParts []string

	// Add knowledge nodes (docs, decisions, patterns)
	if len(nodes) > 0 {
		contextParts = append(contextParts, "## Project Knowledge\n")
		for _, sn := range nodes {
			nodeContext := fmt.Sprintf("### [%s] %s\n%s", sn.Node.Type, sn.Node.Summary, sn.Node.Content)
			contextParts = append(contextParts, nodeContext)
		}
	}

	// Add actual source code snippets
	if len(snippets) > 0 {
		contextParts = append(contextParts, "\n## Relevant Source Code\n")
		for _, snippet := range snippets {
			var sb strings.Builder
			sb.WriteString(fmt.Sprintf("### %s `%s` (%s)\n", snippet.Kind, snippet.SymbolName, snippet.FilePath))
			if snippet.DocComment != "" {
				sb.WriteString(fmt.Sprintf("> %s\n", strings.ReplaceAll(snippet.DocComment, "\n", "\n> ")))
			}
			sb.WriteString("```\n")
			sb.WriteString(snippet.Content)
			sb.WriteString("```\n")
			contextParts = append(contextParts, sb.String())
		}
	}

	if len(contextParts) == 0 {
		return "I found no relevant information to answer your question.", nil
	}

	retrievedContext := strings.Join(contextParts, "\n\n")

	prompt := fmt.Sprintf(`You are an expert on this codebase. Answer the user's question using ONLY the context below.
The context includes both project documentation/decisions AND actual source code.
When referencing code, cite the file and line numbers.
Be concise and direct.

%s

## Question:
%s

## Answer:`, retrievedContext, query)

	chatModel, err := llm.NewCloseableChatModel(ctx, a.ctx.LLMCfg)
	if err != nil {
		return "", fmt.Errorf("create chat model: %w", err)
	}
	defer func() { _ = chatModel.Close() }()
	if a.ctx.LLMCfg.Provider == llm.ProviderGemini {
		restore := suppressStdLogger()
		defer restore()
	}

	messages := []*schema.Message{
		schema.UserMessage(prompt),
	}

	// Use streaming if a writer is provided
	if streamWriter != nil {
		stream, err := chatModel.Stream(ctx, messages)
		if err != nil {
			return "", fmt.Errorf("stream answer: %w", err)
		}
		defer stream.Close()

		var fullAnswer strings.Builder
		for {
			chunk, err := stream.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				return "", fmt.Errorf("recv stream: %w", err)
			}
			// Write to stream writer (CLI output)
			_, _ = streamWriter.Write([]byte(chunk.Content))
			// Also accumulate for return value
			fullAnswer.WriteString(chunk.Content)
		}
		if fullAnswer.Len() == 0 {
			return "", fmt.Errorf("empty response from model")
		}
		return fullAnswer.String(), nil
	}

	// Non-streaming fallback
	resp, err := chatModel.Generate(ctx, messages)
	if err != nil {
		return "", fmt.Errorf("generate answer: %w", err)
	}

	return resp.Content, nil
}

func suppressStdLogger() func() {
	prev := log.Writer()
	log.SetOutput(io.Discard)
	return func() {
		log.SetOutput(prev)
	}
}
