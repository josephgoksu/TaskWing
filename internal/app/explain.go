// Package app provides the ExplainApp for deep symbol explanation.
package app

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/cloudwego/eino/schema"
	"github.com/josephgoksu/TaskWing/internal/codeintel"
	"github.com/josephgoksu/TaskWing/internal/knowledge"
	"github.com/josephgoksu/TaskWing/internal/llm"
)

// ExplainRequest configures what to explain.
type ExplainRequest struct {
	Query        string    // Symbol name or search query
	SymbolID     uint32    // Direct symbol ID (optional, overrides Query)
	Depth        int       // Call graph traversal depth (1-5, default: 2)
	IncludeCode  bool      // Include source snippets (default: true)
	StreamWriter io.Writer // For streaming output (optional)
}

// ExplainResult contains the full explanation.
type ExplainResult struct {
	// Target symbol
	Symbol SymbolResponse `json:"symbol"`

	// Call graph context
	Callers []CallNode `json:"callers"`
	Callees []CallNode `json:"callees"`

	// Impact analysis
	ImpactStats ImpactStats `json:"impact_stats"`

	// Related knowledge
	Decisions []knowledge.NodeResponse `json:"decisions,omitempty"`
	Patterns  []knowledge.NodeResponse `json:"patterns,omitempty"`

	// Source context
	SourceCode []CodeSnippet `json:"source_code,omitempty"`

	// Synthesized explanation
	Explanation string `json:"explanation"`
}

// CallNode represents a node in the call graph.
type CallNode struct {
	Symbol   SymbolResponse `json:"symbol"`
	Depth    int            `json:"depth"`
	Relation string         `json:"relation"` // "calls" or "called_by"
	CallSite string         `json:"call_site,omitempty"`
}

// ImpactStats summarizes dependency impact.
type ImpactStats struct {
	DirectCallers        int `json:"direct_callers"`
	DirectCallees        int `json:"direct_callees"`
	TransitiveDependents int `json:"transitive_dependents"`
	AffectedFiles        int `json:"affected_files"`
	MaxDepthReached      int `json:"max_depth_reached"`
}

// ExplainApp provides deep symbol explanation.
type ExplainApp struct {
	ctx          *Context
	queryService *codeintel.QueryService
}

// NewExplainApp creates a new explain application service.
func NewExplainApp(ctx *Context) *ExplainApp {
	// Get database handle for codeintel
	var queryService *codeintel.QueryService
	if ctx.Repo != nil {
		store := ctx.Repo.GetDB()
		if store != nil && store.DB() != nil {
			codeRepo := codeintel.NewRepository(store.DB())
			queryService = codeintel.NewQueryService(codeRepo, ctx.LLMCfg)
		}
	}

	return &ExplainApp{
		ctx:          ctx,
		queryService: queryService,
	}
}

// Explain generates a comprehensive explanation for a symbol.
func (a *ExplainApp) Explain(ctx context.Context, req ExplainRequest) (*ExplainResult, error) {
	if a.queryService == nil {
		return nil, fmt.Errorf("code intelligence not available (run 'tw bootstrap' first)")
	}

	// Set defaults
	if req.Depth <= 0 {
		req.Depth = 2
	}
	if req.Depth > 5 {
		req.Depth = 5
	}

	// 1. Resolve symbol
	symbol, err := a.resolveSymbol(ctx, req)
	if err != nil {
		return nil, err
	}

	// 2. Build call graph context
	callers, err := a.queryService.GetCallers(ctx, symbol.ID)
	if err != nil {
		callers = nil // Non-fatal
	}

	callees, err := a.queryService.GetCallees(ctx, symbol.ID)
	if err != nil {
		callees = nil // Non-fatal
	}

	// 3. Run impact analysis
	impact, err := a.queryService.AnalyzeImpact(ctx, symbol.ID, req.Depth)
	if err != nil {
		impact = nil // Non-fatal
	}

	// 4. Build result structure
	result := &ExplainResult{
		Symbol:  symbolToResponse(*symbol),
		Callers: symbolsToCallNodes(callers, "called_by"),
		Callees: symbolsToCallNodes(callees, "calls"),
	}

	// Build impact stats
	result.ImpactStats = ImpactStats{
		DirectCallers:   len(callers),
		DirectCallees:   len(callees),
		MaxDepthReached: req.Depth,
	}
	if impact != nil {
		result.ImpactStats.TransitiveDependents = impact.AffectedCount
		result.ImpactStats.AffectedFiles = impact.AffectedFiles
	}

	// 5. Fetch source code context
	if req.IncludeCode && a.ctx.BasePath != "" {
		result.SourceCode = a.fetchSourceContext(symbol, callers, callees)
	}

	// 6. Find relevant knowledge (decisions/patterns)
	result.Decisions = a.findRelevantKnowledge(ctx, symbol, "decision")
	result.Patterns = a.findRelevantKnowledge(ctx, symbol, "pattern")

	// 7. Generate narrative explanation
	explanation, err := a.generateExplanation(ctx, result, req.StreamWriter)
	if err != nil {
		// Non-fatal: still return structured data
		result.Explanation = fmt.Sprintf("(Explanation unavailable: %v)", err)
	} else {
		result.Explanation = explanation
	}

	return result, nil
}

// resolveSymbol finds the symbol, disambiguating if multiple matches.
func (a *ExplainApp) resolveSymbol(ctx context.Context, req ExplainRequest) (*codeintel.Symbol, error) {
	// Direct ID lookup
	if req.SymbolID > 0 {
		return a.queryService.FindSymbol(ctx, req.SymbolID)
	}

	// Search by name
	if req.Query == "" {
		return nil, fmt.Errorf("no symbol specified")
	}

	// First try exact name match
	symbols, err := a.queryService.FindSymbolByName(ctx, req.Query)
	if err == nil && len(symbols) == 1 {
		return &symbols[0], nil
	}
	if err == nil && len(symbols) > 1 {
		// Multiple exact matches - prefer exported, then by kind priority
		return disambiguateSymbols(symbols, req.Query), nil
	}

	// Fall back to hybrid search
	results, err := a.queryService.HybridSearch(ctx, req.Query, 5)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("symbol not found: %s", req.Query)
	}

	// Return best match
	return &results[0].Symbol, nil
}

// disambiguateSymbols picks the best symbol when multiple match.
func disambiguateSymbols(symbols []codeintel.Symbol, query string) *codeintel.Symbol {
	// Sort by priority: exact name match > exported > kind priority
	sort.Slice(symbols, func(i, j int) bool {
		// Exact name match wins
		if symbols[i].Name == query && symbols[j].Name != query {
			return true
		}
		if symbols[j].Name == query && symbols[i].Name != query {
			return false
		}

		// Exported wins
		if symbols[i].IsExported() && !symbols[j].IsExported() {
			return true
		}
		if symbols[j].IsExported() && !symbols[i].IsExported() {
			return false
		}

		// Kind priority: function > method > struct > interface > type
		return kindPriority(symbols[i].Kind) > kindPriority(symbols[j].Kind)
	})

	return &symbols[0]
}

func kindPriority(kind codeintel.SymbolKind) int {
	switch kind {
	case codeintel.SymbolFunction:
		return 10
	case codeintel.SymbolMethod:
		return 9
	case codeintel.SymbolStruct:
		return 8
	case codeintel.SymbolInterface:
		return 7
	case codeintel.SymbolType:
		return 6
	case codeintel.SymbolConstant:
		return 5
	case codeintel.SymbolVariable:
		return 4
	default:
		return 0
	}
}

// fetchSourceContext fetches source code for the symbol and related symbols.
func (a *ExplainApp) fetchSourceContext(symbol *codeintel.Symbol, callers, callees []codeintel.Symbol) []CodeSnippet {
	if a.ctx.BasePath == "" {
		return nil
	}

	fetcher := NewSourceFetcher(a.ctx.BasePath)

	// Collect symbols to fetch: main symbol + top callers + top callees
	var toFetch []codeintel.Symbol
	toFetch = append(toFetch, *symbol)

	// Add top 2 callers
	for i := 0; i < min(2, len(callers)); i++ {
		toFetch = append(toFetch, callers[i])
	}

	// Add top 2 callees
	for i := 0; i < min(2, len(callees)); i++ {
		toFetch = append(toFetch, callees[i])
	}

	return fetcher.FetchContext(toFetch)
}

// findRelevantKnowledge searches for knowledge nodes related to the symbol.
func (a *ExplainApp) findRelevantKnowledge(ctx context.Context, symbol *codeintel.Symbol, nodeType string) []knowledge.NodeResponse {
	ks := knowledge.NewService(a.ctx.Repo, a.ctx.LLMCfg)

	// Search for knowledge mentioning this symbol or its file
	query := fmt.Sprintf("%s %s", symbol.Name, symbol.FilePath)
	scored, err := ks.SearchByType(ctx, query, nodeType, 3)
	if err != nil || len(scored) == 0 {
		return nil
	}

	var results []knowledge.NodeResponse
	for _, sn := range scored {
		results = append(results, knowledge.ScoredNodeToResponse(sn))
	}
	return results
}

// generateExplanation uses LLM to synthesize a narrative explanation.
func (a *ExplainApp) generateExplanation(ctx context.Context, result *ExplainResult, streamWriter io.Writer) (string, error) {
	prompt := buildExplainPrompt(result)

	chatModel, err := llm.NewCloseableChatModel(ctx, a.ctx.LLMCfg)
	if err != nil {
		return "", fmt.Errorf("create chat model: %w", err)
	}
	defer func() { _ = chatModel.Close() }()

	messages := []*schema.Message{
		schema.UserMessage(prompt),
	}

	// Use streaming if writer provided
	if streamWriter != nil {
		stream, err := chatModel.Stream(ctx, messages)
		if err != nil {
			return "", fmt.Errorf("stream: %w", err)
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
			_, _ = streamWriter.Write([]byte(chunk.Content))
			fullAnswer.WriteString(chunk.Content)
		}
		return fullAnswer.String(), nil
	}

	// Non-streaming
	resp, err := chatModel.Generate(ctx, messages)
	if err != nil {
		return "", fmt.Errorf("generate: %w", err)
	}

	return resp.Content, nil
}

// buildExplainPrompt constructs the LLM prompt for explanation.
func buildExplainPrompt(result *ExplainResult) string {
	var sb strings.Builder

	sb.WriteString("You are explaining how a code symbol fits into a larger system.\n\n")

	// Target symbol
	sb.WriteString("## Target Symbol\n")
	sb.WriteString(fmt.Sprintf("Name: %s\n", result.Symbol.Name))
	sb.WriteString(fmt.Sprintf("Kind: %s\n", result.Symbol.Kind))
	sb.WriteString(fmt.Sprintf("File: %s:%d\n", result.Symbol.FilePath, result.Symbol.StartLine))
	if result.Symbol.Signature != "" {
		sb.WriteString(fmt.Sprintf("Signature: %s\n", result.Symbol.Signature))
	}
	if result.Symbol.DocComment != "" {
		sb.WriteString(fmt.Sprintf("Documentation: %s\n", result.Symbol.DocComment))
	}

	// Call graph context
	sb.WriteString("\n## Call Graph Context\n")

	sb.WriteString(fmt.Sprintf("\n### Who calls this (%d callers):\n", len(result.Callers)))
	for i, c := range result.Callers {
		if i >= 5 {
			sb.WriteString(fmt.Sprintf("... and %d more\n", len(result.Callers)-5))
			break
		}
		sb.WriteString(fmt.Sprintf("- %s (%s)\n", c.Symbol.Name, c.Symbol.Location))
	}

	sb.WriteString(fmt.Sprintf("\n### What this calls (%d callees):\n", len(result.Callees)))
	for i, c := range result.Callees {
		if i >= 5 {
			sb.WriteString(fmt.Sprintf("... and %d more\n", len(result.Callees)-5))
			break
		}
		sb.WriteString(fmt.Sprintf("- %s (%s)\n", c.Symbol.Name, c.Symbol.Location))
	}

	// Impact analysis
	sb.WriteString("\n### Impact Analysis\n")
	sb.WriteString(fmt.Sprintf("- Direct callers: %d\n", result.ImpactStats.DirectCallers))
	sb.WriteString(fmt.Sprintf("- Direct callees: %d\n", result.ImpactStats.DirectCallees))
	sb.WriteString(fmt.Sprintf("- Transitive dependents: %d (depth %d)\n",
		result.ImpactStats.TransitiveDependents, result.ImpactStats.MaxDepthReached))
	if result.ImpactStats.AffectedFiles > 0 {
		sb.WriteString(fmt.Sprintf("- Affected files: %d\n", result.ImpactStats.AffectedFiles))
	}

	// Related decisions
	if len(result.Decisions) > 0 {
		sb.WriteString("\n## Related Architectural Decisions\n")
		for _, d := range result.Decisions {
			sb.WriteString(fmt.Sprintf("- [%s] %s\n", d.Type, d.Summary))
		}
	}

	// Source code context
	if len(result.SourceCode) > 0 {
		sb.WriteString("\n## Source Code Context\n")
		for _, snippet := range result.SourceCode {
			sb.WriteString(fmt.Sprintf("\n### %s `%s` (%s)\n", snippet.Kind, snippet.SymbolName, snippet.FilePath))
			sb.WriteString("```\n")
			sb.WriteString(snippet.Content)
			sb.WriteString("```\n")
		}
	}

	// Task
	sb.WriteString(`
## Task
Write a concise explanation (2-3 paragraphs) that:
1. Describes what this symbol does and its purpose
2. Explains how it fits into the system (who uses it, what it depends on)
3. Notes any architectural significance
4. Mentions the impact of changes (who would be affected)

Be specific and reference actual code locations when relevant.
`)

	return sb.String()
}

// symbolToResponse converts a codeintel.Symbol to SymbolResponse.
func symbolToResponse(s codeintel.Symbol) SymbolResponse {
	return SymbolResponse{
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

// symbolsToCallNodes converts a list of symbols to CallNodes.
func symbolsToCallNodes(symbols []codeintel.Symbol, relation string) []CallNode {
	nodes := make([]CallNode, len(symbols))
	for i, s := range symbols {
		nodes[i] = CallNode{
			Symbol:   symbolToResponse(s),
			Depth:    1,
			Relation: relation,
			CallSite: fmt.Sprintf("%s:%d", s.FilePath, s.StartLine),
		}
	}
	return nodes
}
