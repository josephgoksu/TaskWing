package codeintel

import (
	"context"
	"database/sql"
	"testing"

	"github.com/josephgoksu/TaskWing/internal/llm"
	_ "modernc.org/sqlite"
)

// setupTestDB creates an in-memory SQLite database with the required schema.
func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}

	// Create required tables
	schema := `
		CREATE TABLE IF NOT EXISTS symbols (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			kind TEXT NOT NULL,
			file_path TEXT NOT NULL,
			start_line INTEGER NOT NULL,
			end_line INTEGER NOT NULL,
			signature TEXT,
			doc_comment TEXT,
			module_path TEXT,
			visibility TEXT NOT NULL DEFAULT 'private',
			language TEXT NOT NULL DEFAULT 'unknown',
			file_hash TEXT,
			embedding BLOB,
			last_modified TEXT NOT NULL DEFAULT (datetime('now')),
			UNIQUE(name, file_path, start_line)
		);

		CREATE TABLE IF NOT EXISTS symbol_relations (
			from_symbol_id INTEGER NOT NULL,
			to_symbol_id INTEGER NOT NULL,
			relation_type TEXT NOT NULL,
			call_site_line INTEGER,
			metadata TEXT,
			PRIMARY KEY (from_symbol_id, to_symbol_id, relation_type)
		);

		CREATE VIRTUAL TABLE IF NOT EXISTS symbols_fts USING fts5(
			name, signature, doc_comment, module_path,
			content='',
			content_rowid='id'
		);

		CREATE TABLE IF NOT EXISTS dependencies (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			version TEXT NOT NULL,
			ecosystem TEXT NOT NULL,
			lockfile_ref TEXT NOT NULL,
			resolved TEXT,
			integrity TEXT,
			is_dev INTEGER DEFAULT 0,
			source TEXT,
			extras TEXT,
			last_modified TEXT NOT NULL DEFAULT (datetime('now')),
			UNIQUE(name, version, lockfile_ref)
		);

		CREATE VIRTUAL TABLE IF NOT EXISTS dependencies_fts USING fts5(
			name, version, ecosystem,
			content='',
			content_rowid='id'
		);
	`

	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("create schema: %v", err)
	}

	return db
}

// TestQueryService_FindSymbol tests symbol lookup by ID.
func TestQueryService_FindSymbol(t *testing.T) {
	db := setupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := NewRepository(db)
	qs := NewQueryService(repo, llm.Config{})
	ctx := context.Background()

	// Create a test symbol
	sym := &Symbol{
		Name:       "TestFunc",
		Kind:       SymbolFunction,
		FilePath:   "internal/app/service.go",
		StartLine:  10,
		EndLine:    20,
		Signature:  "func TestFunc(ctx context.Context) error",
		Visibility: "public",
		Language:   "go",
	}

	id, err := repo.UpsertSymbol(ctx, sym)
	if err != nil {
		t.Fatalf("upsert symbol: %v", err)
	}

	t.Run("successful lookup", func(t *testing.T) {
		found, err := qs.FindSymbol(ctx, id)
		if err != nil {
			t.Errorf("FindSymbol() error = %v", err)
			return
		}

		if found.Name != "TestFunc" {
			t.Errorf("FindSymbol() name = %q, want %q", found.Name, "TestFunc")
		}
		if found.Kind != SymbolFunction {
			t.Errorf("FindSymbol() kind = %q, want %q", found.Kind, SymbolFunction)
		}
		if found.FilePath != "internal/app/service.go" {
			t.Errorf("FindSymbol() filepath = %q, want %q", found.FilePath, "internal/app/service.go")
		}
	})

	t.Run("not found returns error", func(t *testing.T) {
		_, err := qs.FindSymbol(ctx, 99999)
		if err == nil {
			t.Errorf("FindSymbol() expected error for non-existent ID, got nil")
		}
	})
}

// TestQueryService_FindSymbolByName tests symbol lookup by name.
func TestQueryService_FindSymbolByName(t *testing.T) {
	db := setupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := NewRepository(db)
	qs := NewQueryService(repo, llm.Config{})
	ctx := context.Background()

	// Create test symbols with same name in different files
	symbols := []*Symbol{
		{
			Name:       "Handler",
			Kind:       SymbolFunction,
			FilePath:   "cmd/api/handler.go",
			StartLine:  15,
			EndLine:    30,
			Visibility: "public",
			Language:   "go",
		},
		{
			Name:       "Handler",
			Kind:       SymbolFunction,
			FilePath:   "cmd/web/handler.go",
			StartLine:  10,
			EndLine:    25,
			Visibility: "public",
			Language:   "go",
		},
		{
			Name:       "Handler",
			Kind:       SymbolFunction,
			FilePath:   "pkg/handler.ts",
			StartLine:  5,
			EndLine:    20,
			Visibility: "public",
			Language:   "typescript",
		},
	}

	for _, sym := range symbols {
		if _, err := repo.UpsertSymbol(ctx, sym); err != nil {
			t.Fatalf("upsert symbol: %v", err)
		}
	}

	t.Run("finds all symbols with name", func(t *testing.T) {
		found, err := qs.FindSymbolByName(ctx, "Handler")
		if err != nil {
			t.Errorf("FindSymbolByName() error = %v", err)
			return
		}

		if len(found) != 3 {
			t.Errorf("FindSymbolByName() count = %d, want 3", len(found))
		}
	})

	t.Run("finds symbols filtered by language", func(t *testing.T) {
		found, err := qs.FindSymbolByNameAndLang(ctx, "Handler", "go")
		if err != nil {
			t.Errorf("FindSymbolByNameAndLang() error = %v", err)
			return
		}

		if len(found) != 2 {
			t.Errorf("FindSymbolByNameAndLang() count = %d, want 2", len(found))
		}

		for _, s := range found {
			if s.Language != "go" {
				t.Errorf("FindSymbolByNameAndLang() found language %q, want go", s.Language)
			}
		}
	})

	t.Run("not found returns empty slice", func(t *testing.T) {
		found, err := qs.FindSymbolByName(ctx, "NonExistentFunction")
		if err != nil {
			t.Errorf("FindSymbolByName() error = %v", err)
			return
		}

		if len(found) != 0 {
			t.Errorf("FindSymbolByName() count = %d, want 0", len(found))
		}
	})
}

// TestQueryService_AnalyzeImpact tests impact analysis with dependency graph traversal.
func TestQueryService_AnalyzeImpact(t *testing.T) {
	db := setupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := NewRepository(db)
	qs := NewQueryService(repo, llm.Config{})
	ctx := context.Background()

	// Create a call graph:
	//   main() -> handler() -> service() -> repository()
	//                      -> cache() -> repository()
	//
	// Changing repository() should impact: service (depth 1), cache (depth 1),
	// handler (depth 2), main (depth 3)
	//
	// Note: repository appears at multiple depths via different paths,
	// but should be deduplicated to show only at lowest depth.

	symbols := map[string]*Symbol{
		"main": {
			Name: "main", Kind: SymbolFunction, FilePath: "cmd/main.go",
			StartLine: 10, EndLine: 20, Visibility: "private", Language: "go",
		},
		"handler": {
			Name: "handler", Kind: SymbolFunction, FilePath: "internal/api/handler.go",
			StartLine: 15, EndLine: 30, Visibility: "public", Language: "go",
		},
		"service": {
			Name: "service", Kind: SymbolFunction, FilePath: "internal/app/service.go",
			StartLine: 20, EndLine: 40, Visibility: "public", Language: "go",
		},
		"cache": {
			Name: "cache", Kind: SymbolFunction, FilePath: "internal/cache/cache.go",
			StartLine: 10, EndLine: 25, Visibility: "public", Language: "go",
		},
		"repository": {
			Name: "repository", Kind: SymbolFunction, FilePath: "internal/store/repo.go",
			StartLine: 30, EndLine: 50, Visibility: "public", Language: "go",
		},
	}

	// Insert symbols and collect IDs
	ids := make(map[string]uint32)
	for name, sym := range symbols {
		id, err := repo.UpsertSymbol(ctx, sym)
		if err != nil {
			t.Fatalf("upsert symbol %s: %v", name, err)
		}
		ids[name] = id
	}

	// Create call relationships
	relations := []struct {
		from, to string
	}{
		{"main", "handler"},
		{"handler", "service"},
		{"handler", "cache"},
		{"service", "repository"},
		{"cache", "repository"},
	}

	for _, rel := range relations {
		err := repo.UpsertRelation(ctx, &SymbolRelation{
			FromSymbolID: ids[rel.from],
			ToSymbolID:   ids[rel.to],
			RelationType: RelationCalls,
		})
		if err != nil {
			t.Fatalf("upsert relation %s->%s: %v", rel.from, rel.to, err)
		}
	}

	t.Run("finds all affected symbols", func(t *testing.T) {
		analysis, err := qs.AnalyzeImpact(ctx, ids["repository"], 5)
		if err != nil {
			t.Errorf("AnalyzeImpact() error = %v", err)
			return
		}

		if analysis.Source.Name != "repository" {
			t.Errorf("AnalyzeImpact() source = %q, want repository", analysis.Source.Name)
		}

		// Should find: service (d1), cache (d1), handler (d2), main (d3)
		if analysis.AffectedCount != 4 {
			t.Errorf("AnalyzeImpact() count = %d, want 4", analysis.AffectedCount)
			t.Logf("Affected symbols:")
			for _, node := range analysis.Affected {
				t.Logf("  - %s (depth %d)", node.Symbol.Name, node.Depth)
			}
		}

		// Verify depth grouping
		if len(analysis.ByDepth[1]) != 2 {
			t.Errorf("AnalyzeImpact() depth 1 count = %d, want 2", len(analysis.ByDepth[1]))
		}
		if len(analysis.ByDepth[2]) != 1 {
			t.Errorf("AnalyzeImpact() depth 2 count = %d, want 1", len(analysis.ByDepth[2]))
		}
		if len(analysis.ByDepth[3]) != 1 {
			t.Errorf("AnalyzeImpact() depth 3 count = %d, want 1", len(analysis.ByDepth[3]))
		}
	})

	t.Run("respects max depth limit", func(t *testing.T) {
		analysis, err := qs.AnalyzeImpact(ctx, ids["repository"], 1)
		if err != nil {
			t.Errorf("AnalyzeImpact() error = %v", err)
			return
		}

		// With maxDepth=1, should only find: service (d1), cache (d1)
		if analysis.AffectedCount != 2 {
			t.Errorf("AnalyzeImpact() with maxDepth=1 count = %d, want 2", analysis.AffectedCount)
		}
	})

	t.Run("handles symbol with no callers", func(t *testing.T) {
		analysis, err := qs.AnalyzeImpact(ctx, ids["main"], 5)
		if err != nil {
			t.Errorf("AnalyzeImpact() error = %v", err)
			return
		}

		// main has no callers
		if analysis.AffectedCount != 0 {
			t.Errorf("AnalyzeImpact() for main count = %d, want 0", analysis.AffectedCount)
		}
	})
}

// TestQueryService_AnalyzeImpact_CyclicGraph tests impact analysis handles cycles correctly.
func TestQueryService_AnalyzeImpact_CyclicGraph(t *testing.T) {
	db := setupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := NewRepository(db)
	qs := NewQueryService(repo, llm.Config{})
	ctx := context.Background()

	// Create a cyclic call graph:
	//   A -> B -> C -> A (cycle)
	//
	// When analyzing C, we should find B (d1), A (d2)
	// The cycle should be handled without infinite recursion.

	symbols := []*Symbol{
		{Name: "A", Kind: SymbolFunction, FilePath: "a.go", StartLine: 1, EndLine: 10, Visibility: "public", Language: "go"},
		{Name: "B", Kind: SymbolFunction, FilePath: "b.go", StartLine: 1, EndLine: 10, Visibility: "public", Language: "go"},
		{Name: "C", Kind: SymbolFunction, FilePath: "c.go", StartLine: 1, EndLine: 10, Visibility: "public", Language: "go"},
	}

	ids := make(map[string]uint32)
	for _, sym := range symbols {
		id, err := repo.UpsertSymbol(ctx, sym)
		if err != nil {
			t.Fatalf("upsert symbol %s: %v", sym.Name, err)
		}
		ids[sym.Name] = id
	}

	// Create cyclic relationships: A->B, B->C, C->A
	relations := []struct{ from, to string }{
		{"A", "B"},
		{"B", "C"},
		{"C", "A"},
	}

	for _, rel := range relations {
		err := repo.UpsertRelation(ctx, &SymbolRelation{
			FromSymbolID: ids[rel.from],
			ToSymbolID:   ids[rel.to],
			RelationType: RelationCalls,
		})
		if err != nil {
			t.Fatalf("upsert relation: %v", err)
		}
	}

	t.Run("handles cyclic graph without infinite loop", func(t *testing.T) {
		analysis, err := qs.AnalyzeImpact(ctx, ids["C"], 10)
		if err != nil {
			t.Errorf("AnalyzeImpact() error = %v", err)
			return
		}

		// Should complete without timeout or stack overflow
		// Due to cycle, we expect B (d1), A (d2), and then the cycle continues
		// but depth limit should prevent infinite recursion
		if analysis.Source.Name != "C" {
			t.Errorf("AnalyzeImpact() source = %q, want C", analysis.Source.Name)
		}

		// B directly calls C, A calls B (which calls C)
		// After that, C calls A which starts the cycle again
		// The deduplication should ensure each symbol appears only once
		t.Logf("Found %d affected symbols in cyclic graph", analysis.AffectedCount)
		for _, node := range analysis.Affected {
			t.Logf("  - %s (depth %d)", node.Symbol.Name, node.Depth)
		}
	})
}

// TestQueryService_GetCallers tests the GetCallers function.
func TestQueryService_GetCallers(t *testing.T) {
	db := setupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := NewRepository(db)
	qs := NewQueryService(repo, llm.Config{})
	ctx := context.Background()

	// Create symbols
	target := &Symbol{Name: "target", Kind: SymbolFunction, FilePath: "target.go", StartLine: 1, EndLine: 10, Visibility: "public", Language: "go"}
	caller1 := &Symbol{Name: "caller1", Kind: SymbolFunction, FilePath: "caller1.go", StartLine: 1, EndLine: 10, Visibility: "public", Language: "go"}
	caller2 := &Symbol{Name: "caller2", Kind: SymbolFunction, FilePath: "caller2.go", StartLine: 1, EndLine: 10, Visibility: "public", Language: "go"}

	targetID, _ := repo.UpsertSymbol(ctx, target)
	caller1ID, _ := repo.UpsertSymbol(ctx, caller1)
	caller2ID, _ := repo.UpsertSymbol(ctx, caller2)

	// caller1 and caller2 both call target
	_ = repo.UpsertRelation(ctx, &SymbolRelation{FromSymbolID: caller1ID, ToSymbolID: targetID, RelationType: RelationCalls})
	_ = repo.UpsertRelation(ctx, &SymbolRelation{FromSymbolID: caller2ID, ToSymbolID: targetID, RelationType: RelationCalls})

	t.Run("returns all callers", func(t *testing.T) {
		callers, err := qs.GetCallers(ctx, targetID)
		if err != nil {
			t.Errorf("GetCallers() error = %v", err)
			return
		}

		if len(callers) != 2 {
			t.Errorf("GetCallers() count = %d, want 2", len(callers))
		}
	})

	t.Run("returns empty for symbol with no callers", func(t *testing.T) {
		callers, err := qs.GetCallers(ctx, caller1ID)
		if err != nil {
			t.Errorf("GetCallers() error = %v", err)
			return
		}

		if len(callers) != 0 {
			t.Errorf("GetCallers() count = %d, want 0", len(callers))
		}
	})
}

// TestQueryService_GetCallees tests the GetCallees function.
func TestQueryService_GetCallees(t *testing.T) {
	db := setupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := NewRepository(db)
	qs := NewQueryService(repo, llm.Config{})
	ctx := context.Background()

	// Create symbols
	caller := &Symbol{Name: "caller", Kind: SymbolFunction, FilePath: "caller.go", StartLine: 1, EndLine: 10, Visibility: "public", Language: "go"}
	target1 := &Symbol{Name: "target1", Kind: SymbolFunction, FilePath: "target1.go", StartLine: 1, EndLine: 10, Visibility: "public", Language: "go"}
	target2 := &Symbol{Name: "target2", Kind: SymbolFunction, FilePath: "target2.go", StartLine: 1, EndLine: 10, Visibility: "public", Language: "go"}

	callerID, _ := repo.UpsertSymbol(ctx, caller)
	target1ID, _ := repo.UpsertSymbol(ctx, target1)
	target2ID, _ := repo.UpsertSymbol(ctx, target2)

	// caller calls both target1 and target2
	_ = repo.UpsertRelation(ctx, &SymbolRelation{FromSymbolID: callerID, ToSymbolID: target1ID, RelationType: RelationCalls})
	_ = repo.UpsertRelation(ctx, &SymbolRelation{FromSymbolID: callerID, ToSymbolID: target2ID, RelationType: RelationCalls})

	t.Run("returns all callees", func(t *testing.T) {
		callees, err := qs.GetCallees(ctx, callerID)
		if err != nil {
			t.Errorf("GetCallees() error = %v", err)
			return
		}

		if len(callees) != 2 {
			t.Errorf("GetCallees() count = %d, want 2", len(callees))
		}
	})

	t.Run("returns empty for symbol with no callees", func(t *testing.T) {
		callees, err := qs.GetCallees(ctx, target1ID)
		if err != nil {
			t.Errorf("GetCallees() error = %v", err)
			return
		}

		if len(callees) != 0 {
			t.Errorf("GetCallees() count = %d, want 0", len(callees))
		}
	})
}

// TestQueryService_NotFoundScenarios tests various not-found scenarios.
func TestQueryService_NotFoundScenarios(t *testing.T) {
	db := setupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := NewRepository(db)
	qs := NewQueryService(repo, llm.Config{})
	ctx := context.Background()

	t.Run("FindSymbol with invalid ID returns error", func(t *testing.T) {
		_, err := qs.FindSymbol(ctx, 12345)
		if err == nil {
			t.Error("FindSymbol() expected error for non-existent ID")
		}
	})

	t.Run("FindSymbolByName with no matches returns empty", func(t *testing.T) {
		result, err := qs.FindSymbolByName(ctx, "NonExistent")
		if err != nil {
			t.Errorf("FindSymbolByName() unexpected error: %v", err)
		}
		if len(result) != 0 {
			t.Errorf("FindSymbolByName() expected empty slice, got %d items", len(result))
		}
	})

	t.Run("AnalyzeImpact with invalid ID returns error", func(t *testing.T) {
		_, err := qs.AnalyzeImpact(ctx, 99999, 5)
		if err == nil {
			t.Error("AnalyzeImpact() expected error for non-existent ID")
		}
	})

	t.Run("GetCallers with non-existent ID returns empty", func(t *testing.T) {
		callers, err := qs.GetCallers(ctx, 99999)
		if err != nil {
			t.Errorf("GetCallers() unexpected error: %v", err)
		}
		if len(callers) != 0 {
			t.Errorf("GetCallers() expected empty slice, got %d items", len(callers))
		}
	})

	t.Run("GetCallees with non-existent ID returns empty", func(t *testing.T) {
		callees, err := qs.GetCallees(ctx, 99999)
		if err != nil {
			t.Errorf("GetCallees() unexpected error: %v", err)
		}
		if len(callees) != 0 {
			t.Errorf("GetCallees() expected empty slice, got %d items", len(callees))
		}
	})
}

// TestCosineSimilarity tests the cosine similarity function.
func TestCosineSimilarity(t *testing.T) {
	tests := []struct {
		name string
		a, b []float32
		want float32
	}{
		{
			name: "identical vectors",
			a:    []float32{1, 0, 0},
			b:    []float32{1, 0, 0},
			want: 1.0,
		},
		{
			name: "orthogonal vectors",
			a:    []float32{1, 0, 0},
			b:    []float32{0, 1, 0},
			want: 0.0,
		},
		{
			name: "opposite vectors",
			a:    []float32{1, 0, 0},
			b:    []float32{-1, 0, 0},
			want: -1.0,
		},
		{
			name: "empty vectors",
			a:    []float32{},
			b:    []float32{},
			want: 0.0,
		},
		{
			name: "different length vectors",
			a:    []float32{1, 2},
			b:    []float32{1, 2, 3},
			want: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cosineSimilarity(tt.a, tt.b)
			// Allow small floating point error
			if diff := got - tt.want; diff > 0.0001 || diff < -0.0001 {
				t.Errorf("cosineSimilarity() = %v, want %v", got, tt.want)
			}
		})
	}
}
