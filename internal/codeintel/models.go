// Package codeintel provides code intelligence capabilities including
// symbol extraction, call graph analysis, and semantic code search.
package codeintel

import "time"

// SymbolKind represents the type of code symbol.
type SymbolKind string

const (
	SymbolFunction  SymbolKind = "function"
	SymbolMethod    SymbolKind = "method"
	SymbolStruct    SymbolKind = "struct"
	SymbolInterface SymbolKind = "interface"
	SymbolType      SymbolKind = "type"
	SymbolVariable  SymbolKind = "variable"
	SymbolConstant  SymbolKind = "constant"
	SymbolField     SymbolKind = "field"
	SymbolPackage   SymbolKind = "package"
)

// Symbol represents a code symbol extracted from parsing.
// Symbols are the atomic units of code intelligence - functions, types, etc.
type Symbol struct {
	ID           uint32     `json:"id"`                     // Auto-increment primary key
	Name         string     `json:"name"`                   // Symbol name (e.g., "NewSQLiteStore")
	Kind         SymbolKind `json:"kind"`                   // function, method, struct, etc.
	FilePath     string     `json:"filePath"`               // Relative path to source file
	StartLine    int        `json:"startLine"`              // 1-indexed start line
	EndLine      int        `json:"endLine"`                // 1-indexed end line
	Signature    string     `json:"signature,omitempty"`    // e.g., "func(ctx context.Context, cfg Config) error"
	DocComment   string     `json:"docComment,omitempty"`   // Extracted documentation
	ModulePath   string     `json:"modulePath,omitempty"`   // e.g., "internal/memory"
	Visibility   string     `json:"visibility"`             // public, private
	Language     string     `json:"language"`               // go, typescript, python, etc.
	FileHash     string     `json:"fileHash,omitempty"`     // SHA256 of file for incremental updates
	Embedding    []float32  `json:"embedding,omitempty"`    // Semantic vector for similarity search
	LastModified time.Time  `json:"lastModified"`           // When the symbol was last indexed
}

// SymbolRelation represents a relationship between two symbols.
// Relations enable call graph traversal and impact analysis.
type SymbolRelation struct {
	FromSymbolID uint32       `json:"fromSymbolId"`          // Source symbol
	ToSymbolID   uint32       `json:"toSymbolId"`            // Target symbol
	RelationType RelationType `json:"relationType"`          // calls, implements, etc.
	CallSiteLine int          `json:"callSiteLine,omitempty"` // Line where the call occurs
	Metadata     map[string]any `json:"metadata,omitempty"`  // Additional context (JSON)
}

// RelationType defines the kind of relationship between symbols.
type RelationType string

const (
	RelationCalls      RelationType = "calls"       // Function A calls function B
	RelationCalledBy   RelationType = "called_by"   // Function A is called by function B (inverse of calls)
	RelationImplements RelationType = "implements"  // Type A implements interface B
	RelationExtends    RelationType = "extends"     // Type A embeds/extends type B
	RelationUses       RelationType = "uses"        // Symbol A uses type B
	RelationDefines    RelationType = "defines"     // Package/struct A defines symbol B
	RelationReferences RelationType = "references"  // Symbol A references symbol B
)

// IndexStats holds statistics from an indexing operation.
type IndexStats struct {
	FilesScanned   int           `json:"filesScanned"`
	FilesIndexed   int           `json:"filesIndexed"`
	FilesSkipped   int           `json:"filesSkipped"`
	SymbolsFound   int           `json:"symbolsFound"`
	RelationsFound int           `json:"relationsFound"`
	EmbeddingsGen  int           `json:"embeddingsGenerated"`
	Duration       time.Duration `json:"duration"`
	Errors         []string      `json:"errors,omitempty"`
}

// SymbolSearchResult represents a search result with relevance score.
type SymbolSearchResult struct {
	Symbol Symbol  `json:"symbol"`
	Score  float32 `json:"score"` // Combined FTS + vector score
	Source string  `json:"source"` // "fts", "vector", or "hybrid"
}

// ImpactNode represents a node in the impact analysis graph.
type ImpactNode struct {
	Symbol   Symbol `json:"symbol"`
	Depth    int    `json:"depth"`    // Distance from the changed symbol
	Relation string `json:"relation"` // How it's related (calls, implements, etc.)
}

// SymbolStats holds symbol index statistics for health checks.
type SymbolStats struct {
	TotalSymbols    int            `json:"totalSymbols"`
	TotalFiles      int            `json:"totalFiles"`
	TotalRelations  int            `json:"totalRelations"`
	TotalDeps       int            `json:"totalDeps"`
	ByLanguage      map[string]int `json:"byLanguage"`
	ByKind          map[string]int `json:"byKind"`
	WithEmbeddings  int            `json:"withEmbeddings"`
	StaleFiles      int            `json:"staleFiles"` // Files that no longer exist
}

// IsExported returns true if the symbol is publicly visible.
func (s *Symbol) IsExported() bool {
	return s.Visibility == "public"
}

// FullName returns the fully qualified name (module.name).
func (s *Symbol) FullName() string {
	if s.ModulePath != "" {
		return s.ModulePath + "." + s.Name
	}
	return s.Name
}

// Location returns a human-readable location string.
func (s *Symbol) Location() string {
	if s.StartLine == s.EndLine {
		return s.FilePath + ":" + itoa(s.StartLine)
	}
	return s.FilePath + ":" + itoa(s.StartLine) + "-" + itoa(s.EndLine)
}

// itoa converts int to string without importing strconv.
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b [20]byte
	pos := len(b)
	neg := i < 0
	if neg {
		i = -i
	}
	for i > 0 {
		pos--
		b[pos] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		pos--
		b[pos] = '-'
	}
	return string(b[pos:])
}
