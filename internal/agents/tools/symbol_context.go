// Package tools provides utilities for agent context gathering.
package tools

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/josephgoksu/TaskWing/internal/codeintel"
	"github.com/josephgoksu/TaskWing/internal/llm"
	"github.com/spf13/viper"

	_ "modernc.org/sqlite" // SQLite driver
)

// SymbolContextConfig configures symbol-based context gathering.
type SymbolContextConfig struct {
	MaxTokens      int  // Maximum tokens to use for symbol context
	IncludeCallees bool // Include functions called by each symbol
	IncludeCallers bool // Include functions that call each symbol
	PreferPublic   bool // Prioritize exported/public symbols
}

// DefaultSymbolContextConfig returns sensible defaults.
func DefaultSymbolContextConfig() SymbolContextConfig {
	return SymbolContextConfig{
		MaxTokens:      50000, // ~50k tokens leaves room for prompt overhead
		IncludeCallees: false, // Keep context compact
		IncludeCallers: false,
		PreferPublic:   true,
	}
}

// SymbolContext provides symbol-based context gathering from the code index.
type SymbolContext struct {
	db     *sql.DB // Database handle for cleanup
	repo   codeintel.Repository
	query  *codeintel.QueryService
	config SymbolContextConfig
}

// NewSymbolContext creates a new symbol context gatherer.
// Returns nil if the index is not available (fallback to raw files).
func NewSymbolContext(basePath string, llmCfg llm.Config) (*SymbolContext, error) {
	// Try to open the database
	dbPath := filepath.Join(basePath, ".taskwing", "memory", "memory.db")
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("index not available: %s", dbPath)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// Verify connection is valid
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("database ping failed: %w", err)
	}

	// Enrich config with embedding settings from viper
	// This ensures HybridSearch uses the correct embedding provider (e.g., Ollama)
	embeddingCfg := llmCfg
	if embProvider := viper.GetString("llm.embedding_provider"); embProvider != "" {
		embeddingCfg.EmbeddingProvider = llm.Provider(embProvider)
	}
	if embModel := viper.GetString("llm.embedding_model"); embModel != "" {
		embeddingCfg.EmbeddingModel = embModel
	}
	if embBaseURL := viper.GetString("llm.embedding_base_url"); embBaseURL != "" {
		embeddingCfg.EmbeddingBaseURL = embBaseURL
	}

	repo := codeintel.NewRepository(db)
	query := codeintel.NewQueryService(repo, embeddingCfg)

	return &SymbolContext{
		db:     db,
		repo:   repo,
		query:  query,
		config: DefaultSymbolContextConfig(),
	}, nil
}

// SetConfig updates the configuration.
func (sc *SymbolContext) SetConfig(cfg SymbolContextConfig) {
	sc.config = cfg
}

// GatherArchitecturalContext retrieves key architectural symbols and formats them for LLM.
// Returns a compact representation of the codebase structure.
func (sc *SymbolContext) GatherArchitecturalContext(ctx context.Context) (string, error) {
	var sb strings.Builder
	usedTokens := 0

	// Query architectural patterns
	queries := []struct {
		label string
		query string
		limit int
	}{
		{"Entry Points", "main init server app handler", 30},
		{"Middleware & Auth", "middleware auth authentication authorization cors", 20},
		{"Handlers & Controllers", "handler controller route endpoint api", 30},
		{"Services & Business Logic", "service usecase repository store", 30},
		{"Models & Types", "model struct type config schema", 20},
		{"Error Handling", "error exception panic recover", 10},
	}

	for _, q := range queries {
		if usedTokens >= sc.config.MaxTokens {
			break
		}

		results, err := sc.query.HybridSearch(ctx, q.query, q.limit)
		if err != nil {
			continue
		}

		if len(results) == 0 {
			continue
		}

		section := sc.formatSection(q.label, results, sc.config.MaxTokens-usedTokens)
		sectionTokens := llm.EstimateTokens(section)

		if usedTokens+sectionTokens > sc.config.MaxTokens {
			// Truncate section to fit
			remainingTokens := sc.config.MaxTokens - usedTokens
			if remainingTokens > 100 {
				section = sc.formatSection(q.label, results[:min(len(results), 5)], remainingTokens)
			} else {
				break
			}
		}

		sb.WriteString(section)
		usedTokens += llm.EstimateTokens(section)
	}

	if sb.Len() == 0 {
		return "", fmt.Errorf("no symbols found in index")
	}

	return sb.String(), nil
}

// formatSection formats a group of symbols into a compact section.
func (sc *SymbolContext) formatSection(label string, results []codeintel.SymbolSearchResult, maxTokens int) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## %s\n\n", label))

	usedTokens := llm.EstimateTokens(sb.String())

	for _, r := range results {
		sym := r.Symbol

		// Skip private symbols if preferring public
		if sc.config.PreferPublic && sym.Visibility == "private" {
			continue
		}

		entry := sc.formatSymbol(sym)
		entryTokens := llm.EstimateTokens(entry)

		if usedTokens+entryTokens > maxTokens {
			break
		}

		sb.WriteString(entry)
		usedTokens += entryTokens
	}

	sb.WriteString("\n")
	return sb.String()
}

// formatSymbol formats a single symbol compactly.
func (sc *SymbolContext) formatSymbol(sym codeintel.Symbol) string {
	var sb strings.Builder

	// Format: ### FunctionName (file.go:123)
	sb.WriteString(fmt.Sprintf("### %s (%s:%d)\n", sym.Name, filepath.Base(sym.FilePath), sym.StartLine))

	// Signature
	if sym.Signature != "" {
		sb.WriteString(fmt.Sprintf("```\n%s\n```\n", sym.Signature))
	}

	// Doc comment (truncated)
	if sym.DocComment != "" {
		doc := sym.DocComment
		if len(doc) > 200 {
			doc = doc[:200] + "..."
		}
		sb.WriteString(fmt.Sprintf("> %s\n", strings.ReplaceAll(doc, "\n", "\n> ")))
	}

	sb.WriteString("\n")
	return sb.String()
}

// GetStats returns index statistics for diagnostics.
func (sc *SymbolContext) GetStats(ctx context.Context) (*codeintel.IndexStats, error) {
	return sc.query.GetStats(ctx)
}

// Close releases resources.
func (sc *SymbolContext) Close() error {
	if sc.db != nil {
		return sc.db.Close()
	}
	return nil
}
