// Package app provides the application layer for codeintel operations.
package app

import (
	"context"
	"fmt"

	"github.com/josephgoksu/TaskWing/internal/codeintel"
)

// CodeIntelApp provides code intelligence operations through the app layer.
// This follows the same pattern as RecallApp, TaskApp, etc.
type CodeIntelApp struct {
	ctx *Context
}

// NewCodeIntelApp creates a new code intelligence app.
func NewCodeIntelApp(ctx *Context) *CodeIntelApp {
	return &CodeIntelApp{ctx: ctx}
}

// === Result Types ===

// FindSymbolResult is the result of a find_symbol operation.
type FindSymbolResult struct {
	Success bool              `json:"success"`
	Symbols []codeintel.Symbol `json:"symbols,omitempty"`
	Count   int               `json:"count"`
	Message string            `json:"message,omitempty"`
}

// SearchCodeResult is the result of a semantic_search_code operation.
type SearchCodeResult struct {
	Success bool                          `json:"success"`
	Results []codeintel.SymbolSearchResult `json:"results,omitempty"`
	Count   int                           `json:"count"`
	Query   string                        `json:"query"`
	Message string                        `json:"message,omitempty"`
}

// GetCallersResult is the result of a get_callers operation.
type GetCallersResult struct {
	Success  bool               `json:"success"`
	Symbol   *codeintel.Symbol  `json:"symbol,omitempty"`   // The target symbol
	Callers  []codeintel.Symbol `json:"callers,omitempty"`  // Who calls this symbol
	Callees  []codeintel.Symbol `json:"callees,omitempty"`  // Who this symbol calls
	Count    int                `json:"count"`
	Message  string             `json:"message,omitempty"`
}

// AnalyzeImpactResult is the result of an analyze_impact operation.
type AnalyzeImpactResult struct {
	Success       bool                      `json:"success"`
	Source        *codeintel.Symbol         `json:"source,omitempty"`    // The symbol being analyzed
	Affected      []codeintel.ImpactNode    `json:"affected,omitempty"`  // All affected symbols
	AffectedCount int                       `json:"affected_count"`
	AffectedFiles int                       `json:"affected_files"`
	MaxDepth      int                       `json:"max_depth"`
	ByDepth       map[int][]codeintel.Symbol `json:"by_depth,omitempty"` // Grouped by depth
	Message       string                    `json:"message,omitempty"`
}

// IndexStatsResult is the result of getting index statistics.
type IndexStatsResult struct {
	Success        bool   `json:"success"`
	SymbolsFound   int    `json:"symbols_found"`
	RelationsFound int    `json:"relations_found"`
	FilesIndexed   int    `json:"files_indexed"`
	Message        string `json:"message,omitempty"`
}

// === Options Types ===

// FindSymbolOptions configures the find_symbol operation.
type FindSymbolOptions struct {
	Name     string `json:"name,omitempty"`     // Symbol name to find (exact match)
	ID       uint32 `json:"id,omitempty"`       // Symbol ID for direct lookup
	FilePath string `json:"file_path,omitempty"` // File path to search in
	Language string `json:"language,omitempty"` // Language filter (e.g., "go")
}

// SearchCodeOptions configures the semantic_search_code operation.
type SearchCodeOptions struct {
	Query    string               `json:"query"`              // Required: search query
	Limit    int                  `json:"limit,omitempty"`    // Max results (default 20)
	Kind     codeintel.SymbolKind `json:"kind,omitempty"`     // Filter by symbol kind
	FilePath string               `json:"file_path,omitempty"` // Filter by file path
}

// GetCallersOptions configures the get_callers operation.
type GetCallersOptions struct {
	SymbolID   uint32 `json:"symbol_id,omitempty"`   // Symbol ID to get callers for
	SymbolName string `json:"symbol_name,omitempty"` // Symbol name (if ID not provided)
	Direction  string `json:"direction,omitempty"`   // "callers", "callees", or "both" (default: "both")
}

// AnalyzeImpactOptions configures the analyze_impact operation.
type AnalyzeImpactOptions struct {
	SymbolID   uint32 `json:"symbol_id,omitempty"`   // Symbol ID to analyze
	SymbolName string `json:"symbol_name,omitempty"` // Symbol name (if ID not provided)
	MaxDepth   int    `json:"max_depth,omitempty"`   // Max recursion depth (default 5)
}

// === App Methods ===

// getQueryService creates a QueryService with current context.
func (a *CodeIntelApp) getQueryService() (*codeintel.QueryService, error) {
	// Get the database from memory repository via SQLiteStore
	store := a.ctx.Repo.GetDB()
	if store == nil {
		return nil, fmt.Errorf("database store not available")
	}
	db := store.DB()
	if db == nil {
		return nil, fmt.Errorf("database not available")
	}

	// Create repository and query service
	repo := codeintel.NewRepository(db)
	return codeintel.NewQueryService(repo, a.ctx.LLMCfg), nil
}

// FindSymbol finds symbols by name, ID, or file.
func (a *CodeIntelApp) FindSymbol(ctx context.Context, opts FindSymbolOptions) (*FindSymbolResult, error) {
	qs, err := a.getQueryService()
	if err != nil {
		return &FindSymbolResult{
			Success: false,
			Message: fmt.Sprintf("failed to initialize query service: %v", err),
		}, nil
	}

	var symbols []codeintel.Symbol

	// Find by ID (most specific)
	if opts.ID > 0 {
		sym, err := qs.FindSymbol(ctx, opts.ID)
		if err != nil {
			return &FindSymbolResult{
				Success: false,
				Message: fmt.Sprintf("symbol not found: %v", err),
			}, nil
		}
		symbols = append(symbols, *sym)
	} else if opts.FilePath != "" {
		// Find by file path
		syms, err := qs.GetSymbolsInFile(ctx, opts.FilePath)
		if err != nil {
			return &FindSymbolResult{
				Success: false,
				Message: fmt.Sprintf("failed to get symbols in file: %v", err),
			}, nil
		}
		symbols = syms
	} else if opts.Name != "" {
		// Find by name
		if opts.Language != "" {
			syms, err := qs.FindSymbolByNameAndLang(ctx, opts.Name, opts.Language)
			if err != nil {
				return &FindSymbolResult{
					Success: false,
					Message: fmt.Sprintf("failed to find symbol: %v", err),
				}, nil
			}
			symbols = syms
		} else {
			syms, err := qs.FindSymbolByName(ctx, opts.Name)
			if err != nil {
				return &FindSymbolResult{
					Success: false,
					Message: fmt.Sprintf("failed to find symbol: %v", err),
				}, nil
			}
			symbols = syms
		}
	} else {
		return &FindSymbolResult{
			Success: false,
			Message: "at least one of name, id, or file_path is required",
		}, nil
	}

	if len(symbols) == 0 {
		return &FindSymbolResult{
			Success: true,
			Count:   0,
			Message: "no symbols found",
		}, nil
	}

	return &FindSymbolResult{
		Success: true,
		Symbols: symbols,
		Count:   len(symbols),
	}, nil
}

// SearchCode performs hybrid semantic + lexical search.
func (a *CodeIntelApp) SearchCode(ctx context.Context, opts SearchCodeOptions) (*SearchCodeResult, error) {
	if opts.Query == "" {
		return &SearchCodeResult{
			Success: false,
			Message: "query is required",
		}, nil
	}

	qs, err := a.getQueryService()
	if err != nil {
		return &SearchCodeResult{
			Success: false,
			Query:   opts.Query,
			Message: fmt.Sprintf("failed to initialize query service: %v", err),
		}, nil
	}

	limit := opts.Limit
	if limit <= 0 {
		limit = 20
	}

	var results []codeintel.SymbolSearchResult

	if opts.Kind != "" {
		// Filter by kind
		results, err = qs.SearchByKind(ctx, opts.Query, opts.Kind, limit)
	} else if opts.FilePath != "" {
		// Filter by file
		results, err = qs.SearchByFile(ctx, opts.Query, opts.FilePath, limit)
	} else {
		// Full hybrid search
		results, err = qs.HybridSearch(ctx, opts.Query, limit)
	}

	if err != nil {
		return &SearchCodeResult{
			Success: false,
			Query:   opts.Query,
			Message: fmt.Sprintf("search failed: %v", err),
		}, nil
	}

	return &SearchCodeResult{
		Success: true,
		Results: results,
		Count:   len(results),
		Query:   opts.Query,
	}, nil
}

// GetCallers returns the callers and/or callees of a symbol.
func (a *CodeIntelApp) GetCallers(ctx context.Context, opts GetCallersOptions) (*GetCallersResult, error) {
	qs, err := a.getQueryService()
	if err != nil {
		return &GetCallersResult{
			Success: false,
			Message: fmt.Sprintf("failed to initialize query service: %v", err),
		}, nil
	}

	// Resolve symbol ID
	var symbolID uint32
	if opts.SymbolID > 0 {
		symbolID = opts.SymbolID
	} else if opts.SymbolName != "" {
		// Find symbol by name (use first match)
		symbols, err := qs.FindSymbolByName(ctx, opts.SymbolName)
		if err != nil || len(symbols) == 0 {
			return &GetCallersResult{
				Success: false,
				Message: fmt.Sprintf("symbol '%s' not found", opts.SymbolName),
			}, nil
		}
		symbolID = symbols[0].ID
	} else {
		return &GetCallersResult{
			Success: false,
			Message: "symbol_id or symbol_name is required",
		}, nil
	}

	// Get the symbol
	symbol, err := qs.FindSymbol(ctx, symbolID)
	if err != nil {
		return &GetCallersResult{
			Success: false,
			Message: fmt.Sprintf("symbol not found: %v", err),
		}, nil
	}

	result := &GetCallersResult{
		Success: true,
		Symbol:  symbol,
	}

	direction := opts.Direction
	if direction == "" {
		direction = "both"
	}

	// Get callers
	if direction == "callers" || direction == "both" {
		callers, err := qs.GetCallers(ctx, symbolID)
		if err == nil {
			result.Callers = callers
		}
	}

	// Get callees
	if direction == "callees" || direction == "both" {
		callees, err := qs.GetCallees(ctx, symbolID)
		if err == nil {
			result.Callees = callees
		}
	}

	result.Count = len(result.Callers) + len(result.Callees)
	return result, nil
}

// AnalyzeImpact finds all symbols affected by changing a given symbol.
func (a *CodeIntelApp) AnalyzeImpact(ctx context.Context, opts AnalyzeImpactOptions) (*AnalyzeImpactResult, error) {
	qs, err := a.getQueryService()
	if err != nil {
		return &AnalyzeImpactResult{
			Success: false,
			Message: fmt.Sprintf("failed to initialize query service: %v", err),
		}, nil
	}

	// Resolve symbol ID
	var symbolID uint32
	if opts.SymbolID > 0 {
		symbolID = opts.SymbolID
	} else if opts.SymbolName != "" {
		// Find symbol by name (use first match)
		symbols, err := qs.FindSymbolByName(ctx, opts.SymbolName)
		if err != nil || len(symbols) == 0 {
			return &AnalyzeImpactResult{
				Success: false,
				Message: fmt.Sprintf("symbol '%s' not found", opts.SymbolName),
			}, nil
		}
		symbolID = symbols[0].ID
	} else {
		return &AnalyzeImpactResult{
			Success: false,
			Message: "symbol_id or symbol_name is required",
		}, nil
	}

	maxDepth := opts.MaxDepth
	if maxDepth <= 0 {
		maxDepth = 5
	}

	// Run impact analysis
	analysis, err := qs.AnalyzeImpact(ctx, symbolID, maxDepth)
	if err != nil {
		return &AnalyzeImpactResult{
			Success: false,
			Message: fmt.Sprintf("impact analysis failed: %v", err),
		}, nil
	}

	return &AnalyzeImpactResult{
		Success:       true,
		Source:        &analysis.Source,
		Affected:      analysis.Affected,
		AffectedCount: analysis.AffectedCount,
		AffectedFiles: analysis.AffectedFiles,
		MaxDepth:      analysis.MaxDepth,
		ByDepth:       analysis.ByDepth,
	}, nil
}

// GetStats returns the current index statistics.
func (a *CodeIntelApp) GetStats(ctx context.Context) (*IndexStatsResult, error) {
	qs, err := a.getQueryService()
	if err != nil {
		return &IndexStatsResult{
			Success: false,
			Message: fmt.Sprintf("failed to initialize query service: %v", err),
		}, nil
	}

	stats, err := qs.GetStats(ctx)
	if err != nil {
		return &IndexStatsResult{
			Success: false,
			Message: fmt.Sprintf("failed to get stats: %v", err),
		}, nil
	}

	return &IndexStatsResult{
		Success:        true,
		SymbolsFound:   stats.SymbolsFound,
		RelationsFound: stats.RelationsFound,
		FilesIndexed:   stats.FilesIndexed,
	}, nil
}
