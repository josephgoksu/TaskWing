// Package app provides source code fetching for RAG.
package app

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/josephgoksu/TaskWing/internal/codeintel"
	"github.com/josephgoksu/TaskWing/internal/llm"
)

// CodeSnippet represents extracted source code for a symbol.
type CodeSnippet struct {
	SymbolID   uint32 `json:"symbol_id"`
	SymbolName string `json:"symbol_name"`
	Kind       string `json:"kind"`
	FilePath   string `json:"file_path"`
	StartLine  int    `json:"start_line"`
	EndLine    int    `json:"end_line"`
	Content    string `json:"content"`
	DocComment string `json:"doc_comment,omitempty"`
	Signature  string `json:"signature,omitempty"`
	TokenCount int    `json:"token_count"`
}

// SourceFetcherConfig configures source code extraction.
type SourceFetcherConfig struct {
	// MaxTokens is the total token budget for all snippets combined.
	MaxTokens int

	// ContextLinesBefore is the number of lines to include before the symbol.
	ContextLinesBefore int

	// ContextLinesAfter is the number of lines to include after the symbol.
	ContextLinesAfter int

	// MaxLinesPerSymbol caps the extracted lines per symbol.
	MaxLinesPerSymbol int

	// PrioritizePublic puts public symbols first in token budget allocation.
	PrioritizePublic bool
}

// DefaultSourceFetcherConfig returns sensible defaults.
func DefaultSourceFetcherConfig() SourceFetcherConfig {
	return SourceFetcherConfig{
		MaxTokens:          4000, // ~4k tokens for source context
		ContextLinesBefore: 3,
		ContextLinesAfter:  2,
		MaxLinesPerSymbol:  100,
		PrioritizePublic:   true,
	}
}

// SourceFetcher reads source code for symbols at query time.
type SourceFetcher struct {
	basePath string
	config   SourceFetcherConfig
}

// NewSourceFetcher creates a new source code fetcher.
func NewSourceFetcher(basePath string) *SourceFetcher {
	return &SourceFetcher{
		basePath: basePath,
		config:   DefaultSourceFetcherConfig(),
	}
}

// SetConfig updates the fetcher configuration.
func (f *SourceFetcher) SetConfig(cfg SourceFetcherConfig) {
	f.config = cfg
}

// FetchContext retrieves source code for the given symbols.
// It respects the token budget and prioritizes symbols accordingly.
func (f *SourceFetcher) FetchContext(symbols []codeintel.Symbol) []CodeSnippet {
	if len(symbols) == 0 {
		return nil
	}

	// Make a copy to avoid mutating caller's slice
	sortedSymbols := make([]codeintel.Symbol, len(symbols))
	copy(sortedSymbols, symbols)

	// Sort symbols: public first if configured, then by relevance (assumed order)
	if f.config.PrioritizePublic {
		sort.SliceStable(sortedSymbols, func(i, j int) bool {
			// Public > private
			if sortedSymbols[i].Visibility != sortedSymbols[j].Visibility {
				return sortedSymbols[i].Visibility == "public"
			}
			// Functions > types > variables (more actionable code)
			return symbolKindPriority(sortedSymbols[i].Kind) > symbolKindPriority(sortedSymbols[j].Kind)
		})
	}

	var snippets []CodeSnippet
	usedTokens := 0

	// Group symbols by file to read each file only once
	fileSymbols := make(map[string][]codeintel.Symbol)
	for _, sym := range sortedSymbols {
		fileSymbols[sym.FilePath] = append(fileSymbols[sym.FilePath], sym)
	}

	// Sort file paths for deterministic iteration order
	filePaths := make([]string, 0, len(fileSymbols))
	for fp := range fileSymbols {
		filePaths = append(filePaths, fp)
	}
	sort.Strings(filePaths)

	// Read files and extract snippets (deterministic order)
	for _, filePath := range filePaths {
		if usedTokens >= f.config.MaxTokens {
			break
		}

		syms := fileSymbols[filePath]
		fileSnippets := f.extractFromFile(filePath, syms, f.config.MaxTokens-usedTokens)
		for _, snippet := range fileSnippets {
			if usedTokens+snippet.TokenCount > f.config.MaxTokens {
				continue
			}
			snippets = append(snippets, snippet)
			usedTokens += snippet.TokenCount
		}
	}

	return snippets
}

// extractFromFile reads a file and extracts snippets for the given symbols.
func (f *SourceFetcher) extractFromFile(relPath string, symbols []codeintel.Symbol, remainingTokens int) []CodeSnippet {
	// Construct absolute path
	absPath := filepath.Join(f.basePath, relPath)

	// Read file content
	content, err := os.ReadFile(absPath)
	if err != nil {
		return nil // File may have been deleted
	}

	lines := strings.Split(string(content), "\n")
	var snippets []CodeSnippet

	for _, sym := range symbols {
		if remainingTokens <= 0 {
			break
		}

		snippet := f.extractSymbolSnippet(lines, sym, relPath)
		if snippet.TokenCount > 0 && snippet.TokenCount <= remainingTokens {
			snippets = append(snippets, snippet)
			remainingTokens -= snippet.TokenCount
		}
	}

	return snippets
}

// extractSymbolSnippet extracts source code for a single symbol.
func (f *SourceFetcher) extractSymbolSnippet(lines []string, sym codeintel.Symbol, filePath string) CodeSnippet {
	// Calculate line range with context
	startLine := sym.StartLine - f.config.ContextLinesBefore
	if startLine < 1 {
		startLine = 1
	}

	endLine := sym.EndLine + f.config.ContextLinesAfter
	if endLine > len(lines) {
		endLine = len(lines)
	}

	// Apply max lines limit
	if endLine-startLine+1 > f.config.MaxLinesPerSymbol {
		endLine = startLine + f.config.MaxLinesPerSymbol - 1
	}

	// Extract lines (adjust for 0-based indexing)
	if startLine < 1 || endLine > len(lines) || startLine > endLine {
		return CodeSnippet{}
	}

	extractedLines := lines[startLine-1 : endLine]

	// Format with line numbers for context
	var formatted strings.Builder
	formatted.WriteString(fmt.Sprintf("// %s:%d-%d\n", filePath, startLine, endLine))
	for i, line := range extractedLines {
		formatted.WriteString(fmt.Sprintf("%d: %s\n", startLine+i, line))
	}

	tokenCount := llm.EstimateTokens(formatted.String())

	return CodeSnippet{
		SymbolID:   sym.ID,
		SymbolName: sym.Name,
		Kind:       string(sym.Kind),
		FilePath:   filePath,
		StartLine:  startLine,
		EndLine:    endLine,
		Content:    formatted.String(),
		DocComment: sym.DocComment,
		Signature:  sym.Signature,
		TokenCount: tokenCount,
	}
}

// symbolKindPriority returns a priority for symbol kinds.
// Higher priority = more likely to be useful in context.
func symbolKindPriority(kind codeintel.SymbolKind) int {
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
		return 5
	case codeintel.SymbolConstant:
		return 4
	case codeintel.SymbolVariable:
		return 3
	default:
		return 0
	}
}

