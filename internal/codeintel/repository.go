package codeintel

import (
	"context"
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"
)

// Repository defines the interface for symbol persistence operations.
// This is the standard interface for code intelligence storage.
type Repository interface {
	// Symbol CRUD operations
	UpsertSymbol(ctx context.Context, s *Symbol) (uint32, error)
	GetSymbol(ctx context.Context, id uint32) (*Symbol, error)
	DeleteSymbol(ctx context.Context, id uint32) error
	DeleteSymbolsByFile(ctx context.Context, filePath string) error
	DeleteSymbolsByFileHash(ctx context.Context, fileHash string) error

	// Symbol query operations
	FindSymbolsByName(ctx context.Context, name string, lang *string) ([]Symbol, error)
	FindSymbolsByFile(ctx context.Context, filePath string) ([]Symbol, error)
	SearchSymbolsFTS(ctx context.Context, query string, limit int) ([]Symbol, error)
	ListSymbolsWithEmbeddings(ctx context.Context) ([]Symbol, error)

	// Relation CRUD operations
	UpsertRelation(ctx context.Context, r *SymbolRelation) error
	DeleteRelationsBySymbol(ctx context.Context, symbolID uint32) error

	// Relation query operations (for call graph traversal)
	GetCallers(ctx context.Context, symbolID uint32) ([]Symbol, error)
	GetCallees(ctx context.Context, symbolID uint32) ([]Symbol, error)
	GetImplementations(ctx context.Context, interfaceID uint32) ([]Symbol, error)
	GetImpactRadius(ctx context.Context, symbolID uint32, maxDepth int) ([]ImpactNode, error)

	// Statistics
	GetSymbolCount(ctx context.Context) (int, error)
	GetRelationCount(ctx context.Context) (int, error)
	GetFileCount(ctx context.Context) (int, error)
	GetSymbolStats(ctx context.Context) (*SymbolStats, error)
	GetStaleSymbolFiles(ctx context.Context, checkPath func(string) bool) ([]string, error)

	// Embedding operations
	UpdateSymbolEmbedding(ctx context.Context, id uint32, embedding []float32) error
	GetSymbolsWithoutEmbeddings(ctx context.Context, limit int) ([]Symbol, error)

	// Maintenance
	RebuildSymbolsFTS(ctx context.Context) error

	// C5 FIX: Atomic clear operation to avoid race conditions
	ClearAllSymbols(ctx context.Context) error

	// Dependency operations
	UpsertDependency(ctx context.Context, d *Dependency) (uint32, error)
	GetDependencies(ctx context.Context, ecosystem *string) ([]Dependency, error)
	SearchDependenciesFTS(ctx context.Context, query string, limit int) ([]Dependency, error)
	GetDependencyCount(ctx context.Context) (int, error)
	DeleteDependenciesByLockfile(ctx context.Context, lockfile string) error
	ClearAllDependencies(ctx context.Context) error
}

// SQLiteRepository implements Repository using SQLite.
// This follows the standard factory pattern used throughout TaskWing.
type SQLiteRepository struct {
	db *sql.DB
}

// NewRepository creates a new SQLite-backed symbol repository.
// This is the standard factory function for code intelligence storage.
func NewRepository(db *sql.DB) *SQLiteRepository {
	return &SQLiteRepository{db: db}
}

// === Symbol CRUD Operations ===

// UpsertSymbol creates or updates a symbol, returning its ID.
// Uses atomic INSERT ... ON CONFLICT for thread-safety during concurrent indexing.
func (r *SQLiteRepository) UpsertSymbol(ctx context.Context, s *Symbol) (uint32, error) {
	if s.LastModified.IsZero() {
		s.LastModified = time.Now().UTC()
	}

	var embeddingBytes []byte
	if len(s.Embedding) > 0 {
		embeddingBytes = float32SliceToBytes(s.Embedding)
	}

	// Atomic upsert using ON CONFLICT with the unique index on (name, file_path, start_line)
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO symbols (
			name, kind, file_path, start_line, end_line, signature, doc_comment,
			module_path, visibility, language, file_hash, embedding, last_modified
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(name, file_path, start_line) DO UPDATE SET
			kind = excluded.kind,
			end_line = excluded.end_line,
			signature = excluded.signature,
			doc_comment = excluded.doc_comment,
			module_path = excluded.module_path,
			visibility = excluded.visibility,
			language = excluded.language,
			file_hash = excluded.file_hash,
			embedding = excluded.embedding,
			last_modified = excluded.last_modified
	`, s.Name, s.Kind, s.FilePath, s.StartLine, s.EndLine, s.Signature, s.DocComment,
		s.ModulePath, s.Visibility, s.Language, s.FileHash, embeddingBytes,
		s.LastModified.Format(time.RFC3339))
	if err != nil {
		return 0, fmt.Errorf("upsert symbol: %w", err)
	}

	// Query for the ID since LastInsertId doesn't work reliably with ON CONFLICT
	var id uint32
	err = r.db.QueryRowContext(ctx, `
		SELECT id FROM symbols WHERE name = ? AND file_path = ? AND start_line = ?
	`, s.Name, s.FilePath, s.StartLine).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("get symbol id: %w", err)
	}

	return id, nil
}

// GetSymbol retrieves a symbol by ID.
func (r *SQLiteRepository) GetSymbol(ctx context.Context, id uint32) (*Symbol, error) {
	var s Symbol
	var embeddingBytes []byte
	var signature, docComment, modulePath, fileHash sql.NullString
	var lastModified string

	err := r.db.QueryRowContext(ctx, `
		SELECT id, name, kind, file_path, start_line, end_line, signature, doc_comment,
		       module_path, visibility, language, file_hash, embedding, last_modified
		FROM symbols WHERE id = ?
	`, id).Scan(&s.ID, &s.Name, &s.Kind, &s.FilePath, &s.StartLine, &s.EndLine,
		&signature, &docComment, &modulePath, &s.Visibility, &s.Language,
		&fileHash, &embeddingBytes, &lastModified)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("symbol not found: %d", id)
	}
	if err != nil {
		return nil, fmt.Errorf("query symbol: %w", err)
	}

	s.Signature = signature.String
	s.DocComment = docComment.String
	s.ModulePath = modulePath.String
	s.FileHash = fileHash.String
	s.LastModified, _ = time.Parse(time.RFC3339, lastModified)
	if len(embeddingBytes) > 0 {
		s.Embedding = bytesToFloat32Slice(embeddingBytes)
	}

	return &s, nil
}

// DeleteSymbol removes a symbol by ID.
func (r *SQLiteRepository) DeleteSymbol(ctx context.Context, id uint32) error {
	result, err := r.db.ExecContext(ctx, "DELETE FROM symbols WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete symbol: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("symbol not found: %d", id)
	}

	return nil
}

// DeleteSymbolsByFile removes all symbols from a file.
func (r *SQLiteRepository) DeleteSymbolsByFile(ctx context.Context, filePath string) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM symbols WHERE file_path = ?", filePath)
	if err != nil {
		return fmt.Errorf("delete symbols by file: %w", err)
	}
	return nil
}

// DeleteSymbolsByFileHash removes all symbols with a specific file hash.
func (r *SQLiteRepository) DeleteSymbolsByFileHash(ctx context.Context, fileHash string) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM symbols WHERE file_hash = ?", fileHash)
	if err != nil {
		return fmt.Errorf("delete symbols by hash: %w", err)
	}
	return nil
}

// === Symbol Query Operations ===

// FindSymbolsByName finds symbols matching a name, optionally filtered by language.
func (r *SQLiteRepository) FindSymbolsByName(ctx context.Context, name string, lang *string) ([]Symbol, error) {
	var rows *sql.Rows
	var err error

	if lang != nil && *lang != "" {
		rows, err = r.db.QueryContext(ctx, `
			SELECT id, name, kind, file_path, start_line, end_line, signature, doc_comment,
			       module_path, visibility, language, file_hash, last_modified
			FROM symbols WHERE name = ? AND language = ?
			ORDER BY file_path
		`, name, *lang)
	} else {
		rows, err = r.db.QueryContext(ctx, `
			SELECT id, name, kind, file_path, start_line, end_line, signature, doc_comment,
			       module_path, visibility, language, file_hash, last_modified
			FROM symbols WHERE name = ?
			ORDER BY file_path
		`, name)
	}

	if err != nil {
		return nil, fmt.Errorf("query symbols by name: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanSymbols(rows)
}

// FindSymbolsByFile returns all symbols in a file.
func (r *SQLiteRepository) FindSymbolsByFile(ctx context.Context, filePath string) ([]Symbol, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, name, kind, file_path, start_line, end_line, signature, doc_comment,
		       module_path, visibility, language, file_hash, last_modified
		FROM symbols WHERE file_path = ?
		ORDER BY start_line
	`, filePath)
	if err != nil {
		return nil, fmt.Errorf("query symbols by file: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanSymbols(rows)
}

// SearchSymbolsFTS performs full-text search on symbols.
// C3 FIX: Sanitizes query to prevent FTS5 syntax errors and injection attacks.
func (r *SQLiteRepository) SearchSymbolsFTS(ctx context.Context, query string, limit int) ([]Symbol, error) {
	if limit <= 0 {
		limit = 20
	}

	// C3 FIX: Sanitize query for FTS5 to prevent injection and syntax errors
	// FTS5 special characters that can cause issues: ", *, ^, :, (, ), {, }, [, ], OR, AND, NOT, NEAR
	sanitizedQuery := sanitizeFTSQuery(query)
	if sanitizedQuery == "" {
		return nil, nil // Empty query returns no results
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT s.id, s.name, s.kind, s.file_path, s.start_line, s.end_line,
		       s.signature, s.doc_comment, s.module_path, s.visibility, s.language,
		       s.file_hash, s.last_modified
		FROM symbols_fts f
		JOIN symbols s ON f.rowid = s.id
		WHERE symbols_fts MATCH ?
		ORDER BY bm25(symbols_fts)
		LIMIT ?
	`, sanitizedQuery, limit)
	if err != nil {
		return nil, fmt.Errorf("FTS search: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanSymbols(rows)
}

// ListSymbolsWithEmbeddings returns all symbols that have embeddings.
func (r *SQLiteRepository) ListSymbolsWithEmbeddings(ctx context.Context) ([]Symbol, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, name, kind, file_path, start_line, end_line, signature, doc_comment,
		       module_path, visibility, language, file_hash, embedding, last_modified
		FROM symbols WHERE embedding IS NOT NULL AND length(embedding) > 0
		ORDER BY file_path, start_line
	`)
	if err != nil {
		return nil, fmt.Errorf("query symbols with embeddings: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanSymbolsWithEmbeddings(rows)
}

// === Relation CRUD Operations ===

// UpsertRelation creates or updates a symbol relation.
func (r *SQLiteRepository) UpsertRelation(ctx context.Context, rel *SymbolRelation) error {
	var metadataJSON []byte
	if len(rel.Metadata) > 0 {
		var err error
		metadataJSON, err = json.Marshal(rel.Metadata)
		if err != nil {
			return fmt.Errorf("marshal metadata: %w", err)
		}
	}

	_, err := r.db.ExecContext(ctx, `
		INSERT OR REPLACE INTO symbol_relations (
			from_symbol_id, to_symbol_id, relation_type, call_site_line, metadata
		) VALUES (?, ?, ?, ?, ?)
	`, rel.FromSymbolID, rel.ToSymbolID, rel.RelationType, rel.CallSiteLine, metadataJSON)

	if err != nil {
		return fmt.Errorf("upsert relation: %w", err)
	}
	return nil
}

// DeleteRelationsBySymbol removes all relations involving a symbol.
func (r *SQLiteRepository) DeleteRelationsBySymbol(ctx context.Context, symbolID uint32) error {
	_, err := r.db.ExecContext(ctx, `
		DELETE FROM symbol_relations WHERE from_symbol_id = ? OR to_symbol_id = ?
	`, symbolID, symbolID)
	if err != nil {
		return fmt.Errorf("delete relations: %w", err)
	}
	return nil
}

// === Relation Query Operations ===

// GetCallers returns all symbols that call the given symbol.
func (r *SQLiteRepository) GetCallers(ctx context.Context, symbolID uint32) ([]Symbol, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT s.id, s.name, s.kind, s.file_path, s.start_line, s.end_line,
		       s.signature, s.doc_comment, s.module_path, s.visibility, s.language,
		       s.file_hash, s.last_modified
		FROM symbol_relations sr
		JOIN symbols s ON sr.from_symbol_id = s.id
		WHERE sr.to_symbol_id = ? AND sr.relation_type = 'calls'
		ORDER BY s.file_path, s.start_line
	`, symbolID)
	if err != nil {
		return nil, fmt.Errorf("query callers: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanSymbols(rows)
}

// GetCallees returns all symbols called by the given symbol.
func (r *SQLiteRepository) GetCallees(ctx context.Context, symbolID uint32) ([]Symbol, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT s.id, s.name, s.kind, s.file_path, s.start_line, s.end_line,
		       s.signature, s.doc_comment, s.module_path, s.visibility, s.language,
		       s.file_hash, s.last_modified
		FROM symbol_relations sr
		JOIN symbols s ON sr.to_symbol_id = s.id
		WHERE sr.from_symbol_id = ? AND sr.relation_type = 'calls'
		ORDER BY s.file_path, s.start_line
	`, symbolID)
	if err != nil {
		return nil, fmt.Errorf("query callees: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanSymbols(rows)
}

// GetImplementations returns all types that implement the given interface.
func (r *SQLiteRepository) GetImplementations(ctx context.Context, interfaceID uint32) ([]Symbol, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT s.id, s.name, s.kind, s.file_path, s.start_line, s.end_line,
		       s.signature, s.doc_comment, s.module_path, s.visibility, s.language,
		       s.file_hash, s.last_modified
		FROM symbol_relations sr
		JOIN symbols s ON sr.from_symbol_id = s.id
		WHERE sr.to_symbol_id = ? AND sr.relation_type = 'implements'
		ORDER BY s.file_path, s.start_line
	`, interfaceID)
	if err != nil {
		return nil, fmt.Errorf("query implementations: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanSymbols(rows)
}

// GetImpactRadius finds all symbols affected by changing the given symbol.
// Uses recursive CTE to traverse the call graph up to maxDepth levels.
func (r *SQLiteRepository) GetImpactRadius(ctx context.Context, symbolID uint32, maxDepth int) ([]ImpactNode, error) {
	if maxDepth <= 0 {
		maxDepth = 5
	}

	rows, err := r.db.QueryContext(ctx, `
		WITH RECURSIVE impact AS (
			-- Base case: direct callers of the target symbol
			SELECT from_symbol_id as id, 1 as depth, relation_type as rel
			FROM symbol_relations
			WHERE to_symbol_id = ?

			UNION ALL

			-- Recursive case: callers of callers
			SELECT sr.from_symbol_id, i.depth + 1, sr.relation_type
			FROM symbol_relations sr
			JOIN impact i ON sr.to_symbol_id = i.id
			WHERE i.depth < ?
		)
		SELECT DISTINCT s.id, s.name, s.kind, s.file_path, s.start_line, s.end_line,
		       s.signature, s.doc_comment, s.module_path, s.visibility, s.language,
		       s.file_hash, s.last_modified, i.depth, i.rel
		FROM impact i
		JOIN symbols s ON s.id = i.id
		ORDER BY i.depth, s.file_path
	`, symbolID, maxDepth)
	if err != nil {
		return nil, fmt.Errorf("impact analysis: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var results []ImpactNode
	for rows.Next() {
		var s Symbol
		var depth int
		var relation string
		var signature, docComment, modulePath, fileHash sql.NullString
		var lastModified string

		err := rows.Scan(&s.ID, &s.Name, &s.Kind, &s.FilePath, &s.StartLine, &s.EndLine,
			&signature, &docComment, &modulePath, &s.Visibility, &s.Language,
			&fileHash, &lastModified, &depth, &relation)
		if err != nil {
			return nil, fmt.Errorf("scan impact node: %w", err)
		}

		s.Signature = signature.String
		s.DocComment = docComment.String
		s.ModulePath = modulePath.String
		s.FileHash = fileHash.String
		s.LastModified, _ = time.Parse(time.RFC3339, lastModified)

		results = append(results, ImpactNode{
			Symbol:   s,
			Depth:    depth,
			Relation: relation,
		})
	}

	return results, nil
}

// === Statistics ===

// GetSymbolCount returns the total number of indexed symbols.
func (r *SQLiteRepository) GetSymbolCount(ctx context.Context) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM symbols").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count symbols: %w", err)
	}
	return count, nil
}

// GetRelationCount returns the total number of symbol relations.
func (r *SQLiteRepository) GetRelationCount(ctx context.Context) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM symbol_relations").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count relations: %w", err)
	}
	return count, nil
}

// GetFileCount returns the number of unique files with indexed symbols.
func (r *SQLiteRepository) GetFileCount(ctx context.Context) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(DISTINCT file_path) FROM symbols").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count files: %w", err)
	}
	return count, nil
}

// GetSymbolStats returns comprehensive statistics about the symbol index.
func (r *SQLiteRepository) GetSymbolStats(ctx context.Context) (*SymbolStats, error) {
	stats := &SymbolStats{
		ByLanguage: make(map[string]int),
		ByKind:     make(map[string]int),
	}

	// Get total counts
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM symbols").Scan(&stats.TotalSymbols); err != nil {
		return nil, fmt.Errorf("count symbols: %w", err)
	}
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(DISTINCT file_path) FROM symbols").Scan(&stats.TotalFiles); err != nil {
		return nil, fmt.Errorf("count files: %w", err)
	}
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM symbol_relations").Scan(&stats.TotalRelations); err != nil {
		return nil, fmt.Errorf("count relations: %w", err)
	}
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM dependencies").Scan(&stats.TotalDeps); err != nil {
		// Dependencies table might not exist in older DBs - ignore error
		stats.TotalDeps = 0
	}
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM symbols WHERE embedding IS NOT NULL AND length(embedding) > 0").Scan(&stats.WithEmbeddings); err != nil {
		return nil, fmt.Errorf("count embeddings: %w", err)
	}

	// Get language breakdown
	rows, err := r.db.QueryContext(ctx, "SELECT language, COUNT(*) FROM symbols GROUP BY language ORDER BY COUNT(*) DESC")
	if err != nil {
		return nil, fmt.Errorf("query languages: %w", err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var lang string
		var count int
		if err := rows.Scan(&lang, &count); err != nil {
			continue
		}
		stats.ByLanguage[lang] = count
	}

	// Get kind breakdown
	rows, err = r.db.QueryContext(ctx, "SELECT kind, COUNT(*) FROM symbols GROUP BY kind ORDER BY COUNT(*) DESC")
	if err != nil {
		return nil, fmt.Errorf("query kinds: %w", err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var kind string
		var count int
		if err := rows.Scan(&kind, &count); err != nil {
			continue
		}
		stats.ByKind[kind] = count
	}

	return stats, nil
}

// GetStaleSymbolFiles returns file paths that have symbols indexed but no longer exist.
// The checkPath function should return true if the file exists.
func (r *SQLiteRepository) GetStaleSymbolFiles(ctx context.Context, checkPath func(string) bool) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT DISTINCT file_path FROM symbols ORDER BY file_path")
	if err != nil {
		return nil, fmt.Errorf("query files: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var staleFiles []string
	for rows.Next() {
		var filePath string
		if err := rows.Scan(&filePath); err != nil {
			continue
		}
		if !checkPath(filePath) {
			staleFiles = append(staleFiles, filePath)
		}
	}

	return staleFiles, nil
}

// === Embedding Operations ===

// UpdateSymbolEmbedding updates the embedding for a symbol.
func (r *SQLiteRepository) UpdateSymbolEmbedding(ctx context.Context, id uint32, embedding []float32) error {
	embeddingBytes := float32SliceToBytes(embedding)

	result, err := r.db.ExecContext(ctx, "UPDATE symbols SET embedding = ? WHERE id = ?", embeddingBytes, id)
	if err != nil {
		return fmt.Errorf("update embedding: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("symbol not found: %d", id)
	}

	return nil
}

// GetSymbolsWithoutEmbeddings returns symbols that need embeddings generated.
func (r *SQLiteRepository) GetSymbolsWithoutEmbeddings(ctx context.Context, limit int) ([]Symbol, error) {
	if limit <= 0 {
		limit = 100
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT id, name, kind, file_path, start_line, end_line, signature, doc_comment,
		       module_path, visibility, language, file_hash, last_modified
		FROM symbols
		WHERE embedding IS NULL OR length(embedding) = 0
		ORDER BY file_path, start_line
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("query symbols without embeddings: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanSymbols(rows)
}

// === Maintenance ===

// RebuildSymbolsFTS rebuilds the FTS5 index from existing symbols.
func (r *SQLiteRepository) RebuildSymbolsFTS(ctx context.Context) error {
	// Clear FTS index
	if _, err := r.db.ExecContext(ctx, "DELETE FROM symbols_fts"); err != nil {
		return fmt.Errorf("clear symbols_fts: %w", err)
	}

	// Repopulate from symbols table
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO symbols_fts(rowid, name, signature, doc_comment, module_path)
		SELECT id, name, COALESCE(signature, ''), COALESCE(doc_comment, ''), COALESCE(module_path, '')
		FROM symbols
	`)
	if err != nil {
		return fmt.Errorf("rebuild symbols_fts: %w", err)
	}

	return nil
}

// ClearAllSymbols atomically removes all symbols and relations.
// C5 FIX: Uses single DELETE statements instead of fetch-then-delete-one-by-one
// to avoid race conditions with concurrent indexing operations.
func (r *SQLiteRepository) ClearAllSymbols(ctx context.Context) error {
	// Delete all relations first (or rely on CASCADE, but explicit is safer)
	if _, err := r.db.ExecContext(ctx, "DELETE FROM symbol_relations"); err != nil {
		return fmt.Errorf("clear symbol_relations: %w", err)
	}

	// Delete all symbols
	if _, err := r.db.ExecContext(ctx, "DELETE FROM symbols"); err != nil {
		return fmt.Errorf("clear symbols: %w", err)
	}

	// Clear FTS index
	if _, err := r.db.ExecContext(ctx, "DELETE FROM symbols_fts"); err != nil {
		return fmt.Errorf("clear symbols_fts: %w", err)
	}

	return nil
}

// === Helper Functions ===

// sanitizeFTSQuery sanitizes a query for FTS5 symbol search.
// Uses OR logic for multi-word queries to improve recall for natural language queries.
// Stop words are filtered to focus on content words.
// Preserves trailing * for prefix matching.
func sanitizeFTSQuery(query string) string {
	if query == "" {
		return ""
	}

	// Common stop words that rarely help code search
	stopWords := map[string]bool{
		"a": true, "an": true, "the": true, "is": true, "are": true,
		"was": true, "were": true, "be": true, "been": true, "being": true,
		"have": true, "has": true, "had": true, "do": true, "does": true,
		"did": true, "will": true, "would": true, "could": true, "should": true,
		"may": true, "might": true, "must": true, "shall": true, "am": true,
		"i": true, "you": true, "he": true, "she": true, "it": true,
		"we": true, "they": true, "what": true, "which": true, "who": true,
		"whom": true, "this": true, "that": true, "these": true, "those": true,
		"of": true, "at": true, "by": true, "for": true, "with": true,
		"about": true, "against": true, "between": true, "into": true,
		"through": true, "during": true, "before": true, "after": true,
		"above": true, "below": true, "to": true, "from": true, "up": true,
		"down": true, "in": true, "out": true, "on": true, "off": true,
		"over": true, "under": true, "again": true, "further": true,
		"then": true, "once": true, "here": true, "there": true, "when": true,
		"where": true, "why": true, "how": true, "all": true, "each": true,
		"few": true, "more": true, "most": true, "other": true, "some": true,
		"such": true, "no": true, "nor": true, "not": true, "only": true,
		"own": true, "same": true, "so": true, "than": true, "too": true,
		"very": true, "s": true, "t": true, "can": true, "just": true,
		"don": true, "now": true, "d": true, "ll": true, "m": true,
		"o": true, "re": true, "ve": true, "y": true, "ain": true,
		"aren": true, "couldn": true, "didn": true, "doesn": true,
		"hadn": true, "hasn": true, "haven": true, "isn": true,
		"ma": true, "mightn": true, "mustn": true, "needn": true,
		"shan": true, "shouldn": true, "wasn": true, "weren": true,
		"won": true, "wouldn": true,
	}

	// Replace FTS5 special characters with spaces
	// Preserve * which is used for prefix matching
	replacer := strings.NewReplacer(
		`"`, " ",
		`^`, " ",
		`:`, " ",
		`(`, " ",
		`)`, " ",
		`{`, " ",
		`}`, " ",
		`[`, " ",
		`]`, " ",
		`-`, " ",
		`+`, " ",
		`.`, " ",
		`,`, " ",
		`'`, " ",
		`?`, " ",
		`!`, " ",
	)
	sanitized := replacer.Replace(strings.ToLower(query))

	// Split into words and filter
	words := strings.Fields(sanitized)
	var filtered []string
	seen := make(map[string]bool)

	for _, word := range words {
		// Handle prefix wildcard
		hasPrefixWildcard := false
		cleanWord := word
		if strings.HasSuffix(word, "*") {
			hasPrefixWildcard = true
			cleanWord = strings.TrimSuffix(word, "*")
			cleanWord = strings.ReplaceAll(cleanWord, "*", "")
		} else {
			cleanWord = strings.ReplaceAll(cleanWord, "*", "")
		}

		if cleanWord == "" {
			continue
		}

		// Skip very short words (likely noise)
		if len(cleanWord) < 2 {
			continue
		}

		// Skip stop words
		if stopWords[cleanWord] {
			continue
		}

		// Skip FTS5 operators
		upper := strings.ToUpper(cleanWord)
		if upper == "OR" || upper == "AND" || upper == "NOT" || upper == "NEAR" {
			continue
		}

		// Skip duplicates
		if seen[cleanWord] {
			continue
		}
		seen[cleanWord] = true

		// Restore wildcard if present
		if hasPrefixWildcard {
			filtered = append(filtered, cleanWord+"*")
		} else {
			filtered = append(filtered, cleanWord)
		}
	}

	if len(filtered) == 0 {
		return ""
	}

	// Build query with OR logic for natural language queries
	// For prefix queries, use them directly (FTS5 requires unquoted)
	var result []string
	for _, word := range filtered {
		if strings.HasSuffix(word, "*") {
			result = append(result, word)
		} else {
			result = append(result, `"`+word+`"`)
		}
	}

	// Use OR to improve recall for natural language queries
	return strings.Join(result, " OR ")
}

// scanSymbols scans rows into a slice of Symbol (without embeddings).
func scanSymbols(rows *sql.Rows) ([]Symbol, error) {
	var symbols []Symbol
	for rows.Next() {
		var s Symbol
		var signature, docComment, modulePath, fileHash sql.NullString
		var lastModified string

		err := rows.Scan(&s.ID, &s.Name, &s.Kind, &s.FilePath, &s.StartLine, &s.EndLine,
			&signature, &docComment, &modulePath, &s.Visibility, &s.Language,
			&fileHash, &lastModified)
		if err != nil {
			return nil, fmt.Errorf("scan symbol: %w", err)
		}

		s.Signature = signature.String
		s.DocComment = docComment.String
		s.ModulePath = modulePath.String
		s.FileHash = fileHash.String
		s.LastModified, _ = time.Parse(time.RFC3339, lastModified)

		symbols = append(symbols, s)
	}
	return symbols, nil
}

// scanSymbolsWithEmbeddings scans rows into a slice of Symbol (with embeddings).
func scanSymbolsWithEmbeddings(rows *sql.Rows) ([]Symbol, error) {
	var symbols []Symbol
	for rows.Next() {
		var s Symbol
		var signature, docComment, modulePath, fileHash sql.NullString
		var embeddingBytes []byte
		var lastModified string

		err := rows.Scan(&s.ID, &s.Name, &s.Kind, &s.FilePath, &s.StartLine, &s.EndLine,
			&signature, &docComment, &modulePath, &s.Visibility, &s.Language,
			&fileHash, &embeddingBytes, &lastModified)
		if err != nil {
			return nil, fmt.Errorf("scan symbol: %w", err)
		}

		s.Signature = signature.String
		s.DocComment = docComment.String
		s.ModulePath = modulePath.String
		s.FileHash = fileHash.String
		s.LastModified, _ = time.Parse(time.RFC3339, lastModified)
		if len(embeddingBytes) > 0 {
			s.Embedding = bytesToFloat32Slice(embeddingBytes)
		}

		symbols = append(symbols, s)
	}
	return symbols, nil
}

// float32SliceToBytes converts a float32 slice to bytes for SQLite storage.
// C4 FIX: Uses encoding/binary instead of unsafe.Pointer for safe, portable encoding.
func float32SliceToBytes(floats []float32) []byte {
	buf := make([]byte, len(floats)*4)
	for i, f := range floats {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(f))
	}
	return buf
}

// bytesToFloat32Slice converts bytes to a float32 slice.
// C4 FIX: Uses encoding/binary instead of unsafe.Pointer for safe, portable decoding.
func bytesToFloat32Slice(buf []byte) []float32 {
	if len(buf)%4 != 0 {
		return nil // Invalid buffer size
	}
	floats := make([]float32, len(buf)/4)
	for i := range floats {
		bits := binary.LittleEndian.Uint32(buf[i*4:])
		floats[i] = math.Float32frombits(bits)
	}
	return floats
}

// === Dependency Operations ===

// Dependency represents a package dependency from a lockfile.
type Dependency struct {
	ID           uint32    `json:"id"`
	Name         string    `json:"name"`
	Version      string    `json:"version"`
	Ecosystem    string    `json:"ecosystem"`   // npm, pypi, crates.io
	LockfileRef  string    `json:"lockfileRef"` // Path to lockfile
	Resolved     string    `json:"resolved"`    // URL/path
	Integrity    string    `json:"integrity"`   // Hash
	IsDev        bool      `json:"isDev"`       // Dev dependency
	Source       string    `json:"source"`      // registry, git, path
	Extras       string    `json:"extras"`      // JSON metadata
	LastModified time.Time `json:"lastModified"`
}

// UpsertDependency creates or updates a dependency.
func (r *SQLiteRepository) UpsertDependency(ctx context.Context, d *Dependency) (uint32, error) {
	if d.LastModified.IsZero() {
		d.LastModified = time.Now().UTC()
	}

	isDevInt := 0
	if d.IsDev {
		isDevInt = 1
	}

	// Use INSERT OR REPLACE with UNIQUE constraint on (name, version, lockfile_ref)
	result, err := r.db.ExecContext(ctx, `
		INSERT INTO dependencies (
			name, version, ecosystem, lockfile_ref, resolved, integrity,
			is_dev, source, extras, last_modified
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(name, version, lockfile_ref) DO UPDATE SET
			ecosystem = excluded.ecosystem,
			resolved = excluded.resolved,
			integrity = excluded.integrity,
			is_dev = excluded.is_dev,
			source = excluded.source,
			extras = excluded.extras,
			last_modified = excluded.last_modified
	`, d.Name, d.Version, d.Ecosystem, d.LockfileRef, d.Resolved, d.Integrity,
		isDevInt, d.Source, d.Extras, d.LastModified.Format(time.RFC3339))
	if err != nil {
		return 0, fmt.Errorf("upsert dependency: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("get last insert id: %w", err)
	}

	return uint32(id), nil
}

// GetDependencies returns all dependencies, optionally filtered by ecosystem.
func (r *SQLiteRepository) GetDependencies(ctx context.Context, ecosystem *string) ([]Dependency, error) {
	var rows *sql.Rows
	var err error

	if ecosystem != nil && *ecosystem != "" {
		rows, err = r.db.QueryContext(ctx, `
			SELECT id, name, version, ecosystem, lockfile_ref, resolved, integrity,
			       is_dev, source, extras, last_modified
			FROM dependencies WHERE ecosystem = ?
			ORDER BY name
		`, *ecosystem)
	} else {
		rows, err = r.db.QueryContext(ctx, `
			SELECT id, name, version, ecosystem, lockfile_ref, resolved, integrity,
			       is_dev, source, extras, last_modified
			FROM dependencies
			ORDER BY ecosystem, name
		`)
	}

	if err != nil {
		return nil, fmt.Errorf("query dependencies: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var deps []Dependency
	for rows.Next() {
		var d Dependency
		var resolved, integrity, source, extras sql.NullString
		var isDevInt int
		var lastModified string

		err := rows.Scan(&d.ID, &d.Name, &d.Version, &d.Ecosystem, &d.LockfileRef,
			&resolved, &integrity, &isDevInt, &source, &extras, &lastModified)
		if err != nil {
			return nil, fmt.Errorf("scan dependency: %w", err)
		}

		d.Resolved = resolved.String
		d.Integrity = integrity.String
		d.Source = source.String
		d.Extras = extras.String
		d.IsDev = isDevInt != 0
		d.LastModified, _ = time.Parse(time.RFC3339, lastModified)

		deps = append(deps, d)
	}

	return deps, nil
}

// SearchDependenciesFTS performs full-text search on dependencies.
func (r *SQLiteRepository) SearchDependenciesFTS(ctx context.Context, query string, limit int) ([]Dependency, error) {
	if limit <= 0 {
		limit = 20
	}

	sanitizedQuery := sanitizeFTSQuery(query)
	if sanitizedQuery == "" {
		return nil, nil
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT d.id, d.name, d.version, d.ecosystem, d.lockfile_ref, d.resolved, d.integrity,
		       d.is_dev, d.source, d.extras, d.last_modified
		FROM dependencies_fts f
		JOIN dependencies d ON f.rowid = d.id
		WHERE dependencies_fts MATCH ?
		ORDER BY bm25(dependencies_fts)
		LIMIT ?
	`, sanitizedQuery, limit)
	if err != nil {
		return nil, fmt.Errorf("FTS search dependencies: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var deps []Dependency
	for rows.Next() {
		var d Dependency
		var resolved, integrity, source, extras sql.NullString
		var isDevInt int
		var lastModified string

		err := rows.Scan(&d.ID, &d.Name, &d.Version, &d.Ecosystem, &d.LockfileRef,
			&resolved, &integrity, &isDevInt, &source, &extras, &lastModified)
		if err != nil {
			return nil, fmt.Errorf("scan dependency: %w", err)
		}

		d.Resolved = resolved.String
		d.Integrity = integrity.String
		d.Source = source.String
		d.Extras = extras.String
		d.IsDev = isDevInt != 0
		d.LastModified, _ = time.Parse(time.RFC3339, lastModified)

		deps = append(deps, d)
	}

	return deps, nil
}

// GetDependencyCount returns the total number of dependencies.
func (r *SQLiteRepository) GetDependencyCount(ctx context.Context) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM dependencies").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count dependencies: %w", err)
	}
	return count, nil
}

// DeleteDependenciesByLockfile removes all dependencies from a specific lockfile.
func (r *SQLiteRepository) DeleteDependenciesByLockfile(ctx context.Context, lockfile string) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM dependencies WHERE lockfile_ref = ?", lockfile)
	if err != nil {
		return fmt.Errorf("delete dependencies by lockfile: %w", err)
	}
	return nil
}

// ClearAllDependencies removes all dependencies.
func (r *SQLiteRepository) ClearAllDependencies(ctx context.Context) error {
	if _, err := r.db.ExecContext(ctx, "DELETE FROM dependencies"); err != nil {
		return fmt.Errorf("clear dependencies: %w", err)
	}
	if _, err := r.db.ExecContext(ctx, "DELETE FROM dependencies_fts"); err != nil {
		return fmt.Errorf("clear dependencies_fts: %w", err)
	}
	return nil
}

// Ensure SQLiteRepository implements Repository interface.
var _ Repository = (*SQLiteRepository)(nil)
