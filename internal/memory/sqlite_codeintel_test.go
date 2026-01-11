package memory

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCodeIntelSchemaCreation verifies that the symbols and symbol_relations tables
// are created with the correct schema during SQLiteStore initialization.
func TestCodeIntelSchemaCreation(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tw-codeintel-test-*")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	store, err := NewSQLiteStore(filepath.Join(tmpDir, "memory"))
	require.NoError(t, err)
	defer func() { _ = store.Close() }()

	// Verify symbols table exists with correct columns
	t.Run("symbols_table_columns", func(t *testing.T) {
		rows, err := store.db.Query("PRAGMA table_info(symbols)")
		require.NoError(t, err)
		defer func() { _ = rows.Close() }()

		expectedColumns := map[string]bool{
			"id":            false,
			"name":          false,
			"kind":          false,
			"file_path":     false,
			"start_line":    false,
			"end_line":      false,
			"signature":     false,
			"doc_comment":   false,
			"module_path":   false,
			"visibility":    false,
			"language":      false,
			"file_hash":     false,
			"embedding":     false,
			"last_modified": false,
		}

		for rows.Next() {
			var cid int
			var name, ctype string
			var notnull, pk int
			var dflt interface{}
			err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk)
			require.NoError(t, err)

			if _, exists := expectedColumns[name]; exists {
				expectedColumns[name] = true
			}
		}

		for col, found := range expectedColumns {
			assert.True(t, found, "Column %s should exist in symbols table", col)
		}
	})

	// Verify symbol_relations table exists with correct columns
	t.Run("symbol_relations_table_columns", func(t *testing.T) {
		rows, err := store.db.Query("PRAGMA table_info(symbol_relations)")
		require.NoError(t, err)
		defer func() { _ = rows.Close() }()

		expectedColumns := map[string]bool{
			"from_symbol_id": false,
			"to_symbol_id":   false,
			"relation_type":  false,
			"call_site_line": false,
			"metadata":       false,
		}

		for rows.Next() {
			var cid int
			var name, ctype string
			var notnull, pk int
			var dflt interface{}
			err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk)
			require.NoError(t, err)

			if _, exists := expectedColumns[name]; exists {
				expectedColumns[name] = true
			}
		}

		for col, found := range expectedColumns {
			assert.True(t, found, "Column %s should exist in symbol_relations table", col)
		}
	})

	// Verify symbols_fts virtual table exists
	t.Run("symbols_fts_exists", func(t *testing.T) {
		var count int
		err := store.db.QueryRow(`
			SELECT COUNT(*) FROM sqlite_master
			WHERE type='table' AND name='symbols_fts'
		`).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 1, count, "symbols_fts virtual table should exist")
	})

	// Verify FTS triggers exist
	t.Run("symbols_fts_triggers_exist", func(t *testing.T) {
		triggers := []string{"symbols_fts_ai", "symbols_fts_ad", "symbols_fts_au"}
		for _, trigger := range triggers {
			var count int
			err := store.db.QueryRow(`
				SELECT COUNT(*) FROM sqlite_master
				WHERE type='trigger' AND name=?
			`, trigger).Scan(&count)
			require.NoError(t, err)
			assert.Equal(t, 1, count, "Trigger %s should exist", trigger)
		}
	})

	// Verify indexes exist
	t.Run("symbols_indexes_exist", func(t *testing.T) {
		indexes := []string{
			"idx_symbols_name",
			"idx_symbols_file",
			"idx_symbols_kind",
			"idx_symbols_language",
			"idx_symbols_module",
			"idx_symbols_file_hash",
		}
		for _, idx := range indexes {
			var count int
			err := store.db.QueryRow(`
				SELECT COUNT(*) FROM sqlite_master
				WHERE type='index' AND name=?
			`, idx).Scan(&count)
			require.NoError(t, err)
			assert.Equal(t, 1, count, "Index %s should exist", idx)
		}
	})

	// Verify symbol_relations indexes exist
	t.Run("symbol_relations_indexes_exist", func(t *testing.T) {
		indexes := []string{
			"idx_symbol_relations_from",
			"idx_symbol_relations_to",
			"idx_symbol_relations_type",
		}
		for _, idx := range indexes {
			var count int
			err := store.db.QueryRow(`
				SELECT COUNT(*) FROM sqlite_master
				WHERE type='index' AND name=?
			`, idx).Scan(&count)
			require.NoError(t, err)
			assert.Equal(t, 1, count, "Index %s should exist", idx)
		}
	})
}

// TestCodeIntelSymbolCRUD tests basic symbol insert and query operations.
func TestCodeIntelSymbolCRUD(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tw-codeintel-crud-*")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	store, err := NewSQLiteStore(filepath.Join(tmpDir, "memory"))
	require.NoError(t, err)
	defer func() { _ = store.Close() }()

	// Insert a symbol
	t.Run("insert_symbol", func(t *testing.T) {
		_, err := store.db.Exec(`
			INSERT INTO symbols (name, kind, file_path, start_line, end_line, signature, doc_comment, module_path, visibility, language, last_modified)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, "NewSQLiteStore", "function", "internal/memory/sqlite.go", 25, 62,
			"func NewSQLiteStore(basePath string) (*SQLiteStore, error)",
			"NewSQLiteStore creates a new SQLite-backed memory store.",
			"internal/memory", "public", "go", "2025-01-01T00:00:00Z")
		require.NoError(t, err)
	})

	// Query the symbol via FTS
	t.Run("fts_search_symbol", func(t *testing.T) {
		rows, err := store.db.Query(`
			SELECT name FROM symbols_fts WHERE symbols_fts MATCH 'SQLite'
		`)
		require.NoError(t, err)
		defer func() { _ = rows.Close() }()

		var names []string
		for rows.Next() {
			var name string
			err := rows.Scan(&name)
			require.NoError(t, err)
			names = append(names, name)
		}
		assert.Contains(t, names, "NewSQLiteStore", "FTS should find symbol by partial match")
	})

	// Insert a relation
	t.Run("insert_relation", func(t *testing.T) {
		// First insert a second symbol
		result, err := store.db.Exec(`
			INSERT INTO symbols (name, kind, file_path, start_line, end_line, signature, module_path, visibility, language, last_modified)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, "initSchema", "method", "internal/memory/sqlite.go", 65, 400,
			"func (s *SQLiteStore) initSchema() error",
			"internal/memory", "private", "go", "2025-01-01T00:00:00Z")
		require.NoError(t, err)

		toID, err := result.LastInsertId()
		require.NoError(t, err)

		// Insert a call relationship
		_, err = store.db.Exec(`
			INSERT INTO symbol_relations (from_symbol_id, to_symbol_id, relation_type, call_site_line)
			VALUES (?, ?, ?, ?)
		`, 1, toID, "calls", 56)
		require.NoError(t, err)
	})

	// Query relations (simulating impact analysis)
	t.Run("query_callers", func(t *testing.T) {
		rows, err := store.db.Query(`
			SELECT s.name, sr.call_site_line
			FROM symbol_relations sr
			JOIN symbols s ON sr.from_symbol_id = s.id
			WHERE sr.to_symbol_id = 2 AND sr.relation_type = 'calls'
		`)
		require.NoError(t, err)
		defer func() { _ = rows.Close() }()

		var callers []string
		for rows.Next() {
			var name string
			var line sql.NullInt64
			err := rows.Scan(&name, &line)
			require.NoError(t, err)
			callers = append(callers, name)
		}
		assert.Contains(t, callers, "NewSQLiteStore", "Should find caller of initSchema")
	})
}

// TestCodeIntelRecursiveImpactAnalysis tests the recursive CTE for impact analysis.
func TestCodeIntelRecursiveImpactAnalysis(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tw-codeintel-impact-*")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	store, err := NewSQLiteStore(filepath.Join(tmpDir, "memory"))
	require.NoError(t, err)
	defer func() { _ = store.Close() }()

	// Create a chain of calls: A -> B -> C -> D
	symbols := []struct {
		name string
		kind string
	}{
		{"FuncA", "function"},
		{"FuncB", "function"},
		{"FuncC", "function"},
		{"FuncD", "function"},
	}

	for _, s := range symbols {
		_, err := store.db.Exec(`
			INSERT INTO symbols (name, kind, file_path, start_line, end_line, visibility, language, last_modified)
			VALUES (?, ?, 'test.go', 1, 10, 'public', 'go', '2025-01-01T00:00:00Z')
		`, s.name, s.kind)
		require.NoError(t, err)
	}

	// Create call chain: A->B, B->C, C->D
	relations := []struct {
		from, to int
	}{
		{1, 2}, // A calls B
		{2, 3}, // B calls C
		{3, 4}, // C calls D
	}

	for _, r := range relations {
		_, err := store.db.Exec(`
			INSERT INTO symbol_relations (from_symbol_id, to_symbol_id, relation_type)
			VALUES (?, ?, 'calls')
		`, r.from, r.to)
		require.NoError(t, err)
	}

	// Test recursive CTE to find all symbols affected by changing FuncD (id=4)
	t.Run("recursive_impact_analysis", func(t *testing.T) {
		rows, err := store.db.Query(`
			WITH RECURSIVE impact AS (
				-- Base case: direct callers of the target symbol
				SELECT from_symbol_id as id, 1 as depth
				FROM symbol_relations
				WHERE to_symbol_id = 4 AND relation_type = 'calls'

				UNION ALL

				-- Recursive case: callers of callers
				SELECT sr.from_symbol_id, i.depth + 1
				FROM symbol_relations sr
				JOIN impact i ON sr.to_symbol_id = i.id
				WHERE sr.relation_type = 'calls' AND i.depth < 10
			)
			SELECT s.name, i.depth
			FROM impact i
			JOIN symbols s ON s.id = i.id
			ORDER BY i.depth
		`)
		require.NoError(t, err)
		defer func() { _ = rows.Close() }()

		type impactResult struct {
			name  string
			depth int
		}
		var results []impactResult
		for rows.Next() {
			var r impactResult
			err := rows.Scan(&r.name, &r.depth)
			require.NoError(t, err)
			results = append(results, r)
		}

		// FuncD is changed, so FuncC (direct caller), FuncB (2 hops), FuncA (3 hops) are affected
		require.Len(t, results, 3)
		assert.Equal(t, "FuncC", results[0].name)
		assert.Equal(t, 1, results[0].depth)
		assert.Equal(t, "FuncB", results[1].name)
		assert.Equal(t, 2, results[1].depth)
		assert.Equal(t, "FuncA", results[2].name)
		assert.Equal(t, 3, results[2].depth)
	})
}

// TestCodeIntelFTSTriggers verifies that FTS is automatically updated on insert/update/delete.
func TestCodeIntelFTSTriggers(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tw-codeintel-fts-*")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	store, err := NewSQLiteStore(filepath.Join(tmpDir, "memory"))
	require.NoError(t, err)
	defer func() { _ = store.Close() }()

	// Insert a symbol
	t.Run("fts_auto_insert", func(t *testing.T) {
		_, err := store.db.Exec(`
			INSERT INTO symbols (name, kind, file_path, start_line, end_line, signature, doc_comment, visibility, language, last_modified)
			VALUES ('HandleRequest', 'function', 'api/handler.go', 10, 50, 'func HandleRequest(w http.ResponseWriter, r *http.Request)', 'HandleRequest processes incoming HTTP requests', 'public', 'go', '2025-01-01T00:00:00Z')
		`)
		require.NoError(t, err)

		// Verify FTS can find it
		var count int
		err = store.db.QueryRow(`SELECT COUNT(*) FROM symbols_fts WHERE symbols_fts MATCH 'HTTP'`).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 1, count, "FTS should find symbol by doc_comment match")
	})

	// Update the symbol
	t.Run("fts_auto_update", func(t *testing.T) {
		_, err := store.db.Exec(`
			UPDATE symbols SET doc_comment = 'HandleRequest processes incoming GraphQL requests' WHERE name = 'HandleRequest'
		`)
		require.NoError(t, err)

		// New term should match (GraphQL was added)
		var countGraphQL int
		err = store.db.QueryRow(`SELECT COUNT(*) FROM symbols_fts WHERE symbols_fts MATCH 'GraphQL'`).Scan(&countGraphQL)
		require.NoError(t, err)
		assert.Equal(t, 1, countGraphQL, "FTS should find new term after update")

		// Signature still contains HTTP so that's fine
		var countHTTP int
		err = store.db.QueryRow(`SELECT COUNT(*) FROM symbols_fts WHERE symbols_fts MATCH 'HTTP'`).Scan(&countHTTP)
		require.NoError(t, err)
		assert.Equal(t, 1, countHTTP, "FTS should still find HTTP (in signature)")
	})

	// Delete the symbol
	t.Run("fts_auto_delete", func(t *testing.T) {
		_, err := store.db.Exec(`DELETE FROM symbols WHERE name = 'HandleRequest'`)
		require.NoError(t, err)

		var count int
		err = store.db.QueryRow(`SELECT COUNT(*) FROM symbols_fts WHERE symbols_fts MATCH 'HandleRequest'`).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 0, count, "FTS should not find deleted symbol")
	})
}

// TestCodeIntelForeignKeyConstraints verifies cascading deletes work correctly.
func TestCodeIntelForeignKeyConstraints(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tw-codeintel-fk-*")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	store, err := NewSQLiteStore(filepath.Join(tmpDir, "memory"))
	require.NoError(t, err)
	defer func() { _ = store.Close() }()

	// Insert two symbols and a relation
	_, err = store.db.Exec(`
		INSERT INTO symbols (name, kind, file_path, start_line, end_line, visibility, language, last_modified)
		VALUES ('Caller', 'function', 'a.go', 1, 10, 'public', 'go', '2025-01-01T00:00:00Z')
	`)
	require.NoError(t, err)

	_, err = store.db.Exec(`
		INSERT INTO symbols (name, kind, file_path, start_line, end_line, visibility, language, last_modified)
		VALUES ('Callee', 'function', 'b.go', 1, 10, 'public', 'go', '2025-01-01T00:00:00Z')
	`)
	require.NoError(t, err)

	_, err = store.db.Exec(`
		INSERT INTO symbol_relations (from_symbol_id, to_symbol_id, relation_type)
		VALUES (1, 2, 'calls')
	`)
	require.NoError(t, err)

	// Verify relation exists
	var relationCount int
	err = store.db.QueryRow(`SELECT COUNT(*) FROM symbol_relations`).Scan(&relationCount)
	require.NoError(t, err)
	assert.Equal(t, 1, relationCount)

	// Delete the callee symbol - should cascade delete the relation
	_, err = store.db.Exec(`DELETE FROM symbols WHERE id = 2`)
	require.NoError(t, err)

	// Verify relation was cascade deleted
	err = store.db.QueryRow(`SELECT COUNT(*) FROM symbol_relations`).Scan(&relationCount)
	require.NoError(t, err)
	assert.Equal(t, 0, relationCount, "Relation should be cascade deleted when symbol is deleted")
}
