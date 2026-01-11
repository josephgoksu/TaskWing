package codeintel

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/josephgoksu/TaskWing/internal/llm"
	"github.com/josephgoksu/TaskWing/internal/memory"
)

// TestQueryService_HybridSearch tests the hybrid search functionality.
func TestQueryService_HybridSearch(t *testing.T) {
	ctx := context.Background()

	// Create in-memory store
	store, err := memory.NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	repo := NewRepository(store.DB())

	// Insert test symbols
	symbols := []Symbol{
		{Name: "CreateUser", Kind: SymbolFunction, FilePath: "user.go", StartLine: 10, EndLine: 20, Signature: "func(ctx context.Context, name string) error", DocComment: "CreateUser creates a new user in the database", ModulePath: "internal/user", Visibility: "public", Language: "go"},
		{Name: "DeleteUser", Kind: SymbolFunction, FilePath: "user.go", StartLine: 25, EndLine: 35, Signature: "func(ctx context.Context, id string) error", DocComment: "DeleteUser removes a user from the database", ModulePath: "internal/user", Visibility: "public", Language: "go"},
		{Name: "User", Kind: SymbolStruct, FilePath: "user.go", StartLine: 5, EndLine: 8, DocComment: "User represents a user entity", ModulePath: "internal/user", Visibility: "public", Language: "go"},
		{Name: "handleRequest", Kind: SymbolFunction, FilePath: "handler.go", StartLine: 10, EndLine: 30, Signature: "func(w http.ResponseWriter, r *http.Request)", DocComment: "handleRequest processes incoming HTTP requests", ModulePath: "internal/api", Visibility: "private", Language: "go"},
	}

	for i := range symbols {
		_, err := repo.UpsertSymbol(ctx, &symbols[i])
		if err != nil {
			t.Fatalf("Failed to insert symbol: %v", err)
		}
	}

	// Rebuild FTS index
	if err := repo.RebuildSymbolsFTS(ctx); err != nil {
		t.Fatalf("Failed to rebuild FTS: %v", err)
	}

	// Create query service (no LLM config needed for FTS-only test)
	qs := NewQueryService(repo, emptyllmConfig())

	// Test FTS search
	results, err := qs.HybridSearch(ctx, "CreateUser", 10)
	if err != nil {
		t.Fatalf("HybridSearch failed: %v", err)
	}

	if len(results) == 0 {
		t.Error("Expected at least one result for 'CreateUser'")
	}

	// First result should be CreateUser (exact match)
	if len(results) > 0 && results[0].Symbol.Name != "CreateUser" {
		t.Errorf("Expected first result to be CreateUser, got %s", results[0].Symbol.Name)
	}
}

// TestQueryService_SearchByKind tests filtering by symbol kind.
func TestQueryService_SearchByKind(t *testing.T) {
	ctx := context.Background()

	store, err := memory.NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	repo := NewRepository(store.DB())

	// Insert symbols of different kinds
	symbols := []Symbol{
		{Name: "CreateUser", Kind: SymbolFunction, FilePath: "user.go", StartLine: 10, EndLine: 20, Language: "go", Visibility: "public"},
		{Name: "User", Kind: SymbolStruct, FilePath: "user.go", StartLine: 5, EndLine: 8, Language: "go", Visibility: "public"},
		{Name: "UserService", Kind: SymbolInterface, FilePath: "user.go", StartLine: 1, EndLine: 4, Language: "go", Visibility: "public"},
	}

	for i := range symbols {
		_, err := repo.UpsertSymbol(ctx, &symbols[i])
		if err != nil {
			t.Fatalf("Failed to insert symbol: %v", err)
		}
	}

	if err := repo.RebuildSymbolsFTS(ctx); err != nil {
		t.Fatalf("Failed to rebuild FTS: %v", err)
	}

	qs := NewQueryService(repo, emptyllmConfig())

	// Search for functions only
	results, err := qs.SearchByKind(ctx, "User", SymbolFunction, 10)
	if err != nil {
		t.Fatalf("SearchByKind failed: %v", err)
	}

	for _, r := range results {
		if r.Symbol.Kind != SymbolFunction {
			t.Errorf("Expected only functions, got %s", r.Symbol.Kind)
		}
	}
}

// TestQueryService_SearchByFile tests filtering by file path.
func TestQueryService_SearchByFile(t *testing.T) {
	ctx := context.Background()

	store, err := memory.NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	repo := NewRepository(store.DB())

	symbols := []Symbol{
		{Name: "CreateUser", Kind: SymbolFunction, FilePath: "user.go", StartLine: 10, EndLine: 20, Language: "go", Visibility: "public"},
		{Name: "CreateOrder", Kind: SymbolFunction, FilePath: "order.go", StartLine: 10, EndLine: 20, Language: "go", Visibility: "public"},
	}

	for i := range symbols {
		_, err := repo.UpsertSymbol(ctx, &symbols[i])
		if err != nil {
			t.Fatalf("Failed to insert symbol: %v", err)
		}
	}

	if err := repo.RebuildSymbolsFTS(ctx); err != nil {
		t.Fatalf("Failed to rebuild FTS: %v", err)
	}

	qs := NewQueryService(repo, emptyllmConfig())

	results, err := qs.SearchByFile(ctx, "Create", "user.go", 10)
	if err != nil {
		t.Fatalf("SearchByFile failed: %v", err)
	}

	for _, r := range results {
		if r.Symbol.FilePath != "user.go" {
			t.Errorf("Expected only user.go results, got %s", r.Symbol.FilePath)
		}
	}
}

// TestQueryService_AnalyzeImpact tests the impact analysis functionality.
func TestQueryService_AnalyzeImpact(t *testing.T) {
	ctx := context.Background()

	store, err := memory.NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	repo := NewRepository(store.DB())

	// Create a call chain: main -> processRequest -> validateInput
	symbols := []Symbol{
		{Name: "main", Kind: SymbolFunction, FilePath: "main.go", StartLine: 1, EndLine: 10, Language: "go", Visibility: "public"},
		{Name: "processRequest", Kind: SymbolFunction, FilePath: "handler.go", StartLine: 1, EndLine: 20, Language: "go", Visibility: "public"},
		{Name: "validateInput", Kind: SymbolFunction, FilePath: "validator.go", StartLine: 1, EndLine: 15, Language: "go", Visibility: "public"},
	}

	var ids []uint32
	for i := range symbols {
		id, err := repo.UpsertSymbol(ctx, &symbols[i])
		if err != nil {
			t.Fatalf("Failed to insert symbol: %v", err)
		}
		ids = append(ids, id)
	}

	// Create relations: main calls processRequest, processRequest calls validateInput
	relations := []SymbolRelation{
		{FromSymbolID: ids[0], ToSymbolID: ids[1], RelationType: RelationCalls}, // main -> processRequest
		{FromSymbolID: ids[1], ToSymbolID: ids[2], RelationType: RelationCalls}, // processRequest -> validateInput
	}

	for _, rel := range relations {
		if err := repo.UpsertRelation(ctx, &rel); err != nil {
			t.Fatalf("Failed to insert relation: %v", err)
		}
	}

	qs := NewQueryService(repo, emptyllmConfig())

	// Analyze impact of changing validateInput
	analysis, err := qs.AnalyzeImpact(ctx, ids[2], 5)
	if err != nil {
		t.Fatalf("AnalyzeImpact failed: %v", err)
	}

	// validateInput is called by processRequest (depth 1) and main (depth 2)
	if analysis.AffectedCount != 2 {
		t.Errorf("Expected 2 affected symbols, got %d", analysis.AffectedCount)
	}

	// Check depth 1 includes processRequest
	depth1 := analysis.ByDepth[1]
	found := false
	for _, s := range depth1 {
		if s.Name == "processRequest" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected processRequest at depth 1")
	}

	// Check depth 2 includes main
	depth2 := analysis.ByDepth[2]
	found = false
	for _, s := range depth2 {
		if s.Name == "main" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected main at depth 2")
	}
}

// TestQueryService_FindSymbol tests symbol lookup.
func TestQueryService_FindSymbol(t *testing.T) {
	ctx := context.Background()

	store, err := memory.NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	repo := NewRepository(store.DB())

	sym := Symbol{
		Name:      "TestFunction",
		Kind:      SymbolFunction,
		FilePath:  "test.go",
		StartLine: 1,
		EndLine:   10,
		Language:  "go",
		Visibility: "public",
	}

	id, err := repo.UpsertSymbol(ctx, &sym)
	if err != nil {
		t.Fatalf("Failed to insert symbol: %v", err)
	}

	qs := NewQueryService(repo, emptyllmConfig())

	found, err := qs.FindSymbol(ctx, id)
	if err != nil {
		t.Fatalf("FindSymbol failed: %v", err)
	}

	if found.Name != "TestFunction" {
		t.Errorf("Expected TestFunction, got %s", found.Name)
	}
}

// TestQueryService_FindSymbolByName tests name-based symbol lookup.
func TestQueryService_FindSymbolByName(t *testing.T) {
	ctx := context.Background()

	store, err := memory.NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	repo := NewRepository(store.DB())

	// Insert symbols with same name in different files
	symbols := []Symbol{
		{Name: "Handle", Kind: SymbolFunction, FilePath: "user.go", StartLine: 1, EndLine: 10, Language: "go", Visibility: "public"},
		{Name: "Handle", Kind: SymbolFunction, FilePath: "order.go", StartLine: 1, EndLine: 10, Language: "go", Visibility: "public"},
	}

	for i := range symbols {
		_, err := repo.UpsertSymbol(ctx, &symbols[i])
		if err != nil {
			t.Fatalf("Failed to insert symbol: %v", err)
		}
	}

	qs := NewQueryService(repo, emptyllmConfig())

	results, err := qs.FindSymbolByName(ctx, "Handle")
	if err != nil {
		t.Fatalf("FindSymbolByName failed: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("Expected 2 symbols named Handle, got %d", len(results))
	}
}

// TestQueryService_GetCallersAndCallees tests call graph traversal.
func TestQueryService_GetCallersAndCallees(t *testing.T) {
	ctx := context.Background()

	store, err := memory.NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	repo := NewRepository(store.DB())

	// Create symbols
	symbols := []Symbol{
		{Name: "caller", Kind: SymbolFunction, FilePath: "a.go", StartLine: 1, EndLine: 5, Language: "go", Visibility: "public"},
		{Name: "target", Kind: SymbolFunction, FilePath: "b.go", StartLine: 1, EndLine: 5, Language: "go", Visibility: "public"},
		{Name: "callee", Kind: SymbolFunction, FilePath: "c.go", StartLine: 1, EndLine: 5, Language: "go", Visibility: "public"},
	}

	var ids []uint32
	for i := range symbols {
		id, err := repo.UpsertSymbol(ctx, &symbols[i])
		if err != nil {
			t.Fatalf("Failed to insert symbol: %v", err)
		}
		ids = append(ids, id)
	}

	// caller -> target -> callee
	relations := []SymbolRelation{
		{FromSymbolID: ids[0], ToSymbolID: ids[1], RelationType: RelationCalls},
		{FromSymbolID: ids[1], ToSymbolID: ids[2], RelationType: RelationCalls},
	}

	for _, rel := range relations {
		if err := repo.UpsertRelation(ctx, &rel); err != nil {
			t.Fatalf("Failed to insert relation: %v", err)
		}
	}

	qs := NewQueryService(repo, emptyllmConfig())

	// Test GetCallers for target
	callers, err := qs.GetCallers(ctx, ids[1])
	if err != nil {
		t.Fatalf("GetCallers failed: %v", err)
	}
	if len(callers) != 1 || callers[0].Name != "caller" {
		t.Errorf("Expected caller, got %v", callers)
	}

	// Test GetCallees for target
	callees, err := qs.GetCallees(ctx, ids[1])
	if err != nil {
		t.Fatalf("GetCallees failed: %v", err)
	}
	if len(callees) != 1 || callees[0].Name != "callee" {
		t.Errorf("Expected callee, got %v", callees)
	}
}

// TestQueryService_GetSymbolsInFile tests file-based symbol retrieval.
func TestQueryService_GetSymbolsInFile(t *testing.T) {
	ctx := context.Background()

	store, err := memory.NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	repo := NewRepository(store.DB())

	symbols := []Symbol{
		{Name: "funcA", Kind: SymbolFunction, FilePath: "test.go", StartLine: 1, EndLine: 5, Language: "go", Visibility: "public"},
		{Name: "funcB", Kind: SymbolFunction, FilePath: "test.go", StartLine: 10, EndLine: 15, Language: "go", Visibility: "public"},
		{Name: "funcC", Kind: SymbolFunction, FilePath: "other.go", StartLine: 1, EndLine: 5, Language: "go", Visibility: "public"},
	}

	for i := range symbols {
		_, err := repo.UpsertSymbol(ctx, &symbols[i])
		if err != nil {
			t.Fatalf("Failed to insert symbol: %v", err)
		}
	}

	qs := NewQueryService(repo, emptyllmConfig())

	results, err := qs.GetSymbolsInFile(ctx, "test.go")
	if err != nil {
		t.Fatalf("GetSymbolsInFile failed: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("Expected 2 symbols in test.go, got %d", len(results))
	}
}

// TestQueryService_GetStats tests statistics retrieval.
func TestQueryService_GetStats(t *testing.T) {
	ctx := context.Background()

	store, err := memory.NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	repo := NewRepository(store.DB())

	// Insert symbols and relations
	sym1 := Symbol{Name: "func1", Kind: SymbolFunction, FilePath: "a.go", StartLine: 1, EndLine: 5, Language: "go", Visibility: "public"}
	sym2 := Symbol{Name: "func2", Kind: SymbolFunction, FilePath: "b.go", StartLine: 1, EndLine: 5, Language: "go", Visibility: "public"}

	id1, _ := repo.UpsertSymbol(ctx, &sym1)
	id2, _ := repo.UpsertSymbol(ctx, &sym2)

	rel := SymbolRelation{FromSymbolID: id1, ToSymbolID: id2, RelationType: RelationCalls}
	_ = repo.UpsertRelation(ctx, &rel)

	qs := NewQueryService(repo, emptyllmConfig())

	stats, err := qs.GetStats(ctx)
	if err != nil {
		t.Fatalf("GetStats failed: %v", err)
	}

	if stats.SymbolsFound != 2 {
		t.Errorf("Expected 2 symbols, got %d", stats.SymbolsFound)
	}
	if stats.RelationsFound != 1 {
		t.Errorf("Expected 1 relation, got %d", stats.RelationsFound)
	}
	if stats.FilesIndexed != 2 {
		t.Errorf("Expected 2 files, got %d", stats.FilesIndexed)
	}
}

// TestQueryService_DefaultConfig tests default configuration.
func TestQueryService_DefaultConfig(t *testing.T) {
	config := DefaultQueryConfig()

	if config.FTSWeight <= 0 {
		t.Error("FTSWeight should be > 0")
	}
	if config.VectorWeight <= 0 {
		t.Error("VectorWeight should be > 0")
	}
	if config.VectorThreshold < 0 || config.VectorThreshold > 1 {
		t.Error("VectorThreshold should be between 0 and 1")
	}
	if config.DefaultLimit <= 0 {
		t.Error("DefaultLimit should be > 0")
	}
	if config.MaxImpactDepth <= 0 {
		t.Error("MaxImpactDepth should be > 0")
	}
}

// TestCosineSimilarity tests the cosine similarity function.
func TestCosineSimilarity(t *testing.T) {
	// Test identical vectors
	a := []float32{1, 0, 0}
	b := []float32{1, 0, 0}
	sim := cosineSimilarity(a, b)
	if sim < 0.99 {
		t.Errorf("Expected ~1.0 for identical vectors, got %f", sim)
	}

	// Test orthogonal vectors
	c := []float32{1, 0, 0}
	d := []float32{0, 1, 0}
	sim = cosineSimilarity(c, d)
	if sim > 0.01 {
		t.Errorf("Expected ~0.0 for orthogonal vectors, got %f", sim)
	}

	// Test opposite vectors
	e := []float32{1, 0, 0}
	f := []float32{-1, 0, 0}
	sim = cosineSimilarity(e, f)
	if sim > -0.99 {
		t.Errorf("Expected ~-1.0 for opposite vectors, got %f", sim)
	}

	// Test empty vectors
	g := []float32{}
	h := []float32{}
	sim = cosineSimilarity(g, h)
	if sim != 0 {
		t.Errorf("Expected 0 for empty vectors, got %f", sim)
	}

	// Test different length vectors
	i := []float32{1, 2, 3}
	j := []float32{1, 2}
	sim = cosineSimilarity(i, j)
	if sim != 0 {
		t.Errorf("Expected 0 for different length vectors, got %f", sim)
	}
}

// TestQueryService_ImpactAnalysisPerformance tests impact analysis performance.
func TestQueryService_ImpactAnalysisPerformance(t *testing.T) {
	ctx := context.Background()

	store, err := memory.NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	repo := NewRepository(store.DB())

	// Create a larger graph with 1000+ symbols
	const numSymbols = 1000
	ids := make([]uint32, numSymbols)

	for i := 0; i < numSymbols; i++ {
		sym := Symbol{
			Name:       "func" + itoa(i),
			Kind:       SymbolFunction,
			FilePath:   "file" + itoa(i%100) + ".go", // 100 files
			StartLine:  i * 10,
			EndLine:    i*10 + 5,
			Language:   "go",
			Visibility: "public",
		}
		id, err := repo.UpsertSymbol(ctx, &sym)
		if err != nil {
			t.Fatalf("Failed to insert symbol %d: %v", i, err)
		}
		ids[i] = id
	}

	// Create relations (each symbol calls the next 3)
	for i := 0; i < numSymbols-3; i++ {
		for j := 1; j <= 3; j++ {
			rel := SymbolRelation{
				FromSymbolID: ids[i],
				ToSymbolID:   ids[i+j],
				RelationType: RelationCalls,
			}
			if err := repo.UpsertRelation(ctx, &rel); err != nil {
				t.Fatalf("Failed to insert relation: %v", err)
			}
		}
	}

	qs := NewQueryService(repo, emptyllmConfig())

	// Test that impact analysis completes in reasonable time (< 200ms)
	start := time.Now()
	analysis, err := qs.AnalyzeImpact(ctx, ids[numSymbols-1], 5)
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("AnalyzeImpact failed: %v", err)
	}

	if duration > 200*time.Millisecond {
		t.Errorf("Impact analysis took too long: %v (expected < 200ms)", duration)
	}

	t.Logf("Impact analysis for %d symbols: %v, found %d affected", numSymbols, duration, analysis.AffectedCount)
}

// emptyllmConfig returns an empty LLM config (no API key = no vector search).
func emptyllmConfig() llm.Config {
	return llm.Config{}
}

// === MANDATORY KNOWLEDGE/MEMORY UNIT TESTS ===
// These tests ensure the symbol database remains consistent and
// impact analysis logic is deterministic, per CLAUDE.md requirements.

// TestQueryService_NewQueryServiceWithConfig tests custom configuration.
func TestQueryService_NewQueryServiceWithConfig(t *testing.T) {
	store, err := memory.NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	repo := NewRepository(store.DB())

	customConfig := QueryConfig{
		FTSWeight:          0.5,
		VectorWeight:       0.5,
		VectorThreshold:    0.6,
		MinResultThreshold: 0.2,
		DefaultLimit:       50,
		MaxImpactDepth:     10,
	}

	qs := NewQueryServiceWithConfig(repo, emptyllmConfig(), customConfig)

	// Verify config was applied by checking behavior
	if qs.config.DefaultLimit != 50 {
		t.Errorf("Expected DefaultLimit 50, got %d", qs.config.DefaultLimit)
	}
	if qs.config.MaxImpactDepth != 10 {
		t.Errorf("Expected MaxImpactDepth 10, got %d", qs.config.MaxImpactDepth)
	}
}

// TestQueryService_FindSymbolByNameAndLang tests language-filtered name lookup.
func TestQueryService_FindSymbolByNameAndLang(t *testing.T) {
	ctx := context.Background()

	store, err := memory.NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	repo := NewRepository(store.DB())

	// Insert symbols with different languages
	symbols := []Symbol{
		{Name: "Handle", Kind: SymbolFunction, FilePath: "handler.go", StartLine: 1, EndLine: 10, Language: "go", Visibility: "public"},
		{Name: "Handle", Kind: SymbolFunction, FilePath: "handler.ts", StartLine: 1, EndLine: 10, Language: "typescript", Visibility: "public"},
	}

	for i := range symbols {
		_, err := repo.UpsertSymbol(ctx, &symbols[i])
		if err != nil {
			t.Fatalf("Failed to insert symbol: %v", err)
		}
	}

	qs := NewQueryService(repo, emptyllmConfig())

	// Search for Go only
	results, err := qs.FindSymbolByNameAndLang(ctx, "Handle", "go")
	if err != nil {
		t.Fatalf("FindSymbolByNameAndLang failed: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("Expected 1 Go symbol named Handle, got %d", len(results))
	}

	if len(results) > 0 && results[0].Language != "go" {
		t.Errorf("Expected Go language, got %s", results[0].Language)
	}
}

// TestQueryService_ImpactAnalysisCycleHandling tests that cycles don't cause infinite loops.
// Regression test: Call graphs can have cycles (A calls B, B calls A).
func TestQueryService_ImpactAnalysisCycleHandling(t *testing.T) {
	ctx := context.Background()

	store, err := memory.NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	repo := NewRepository(store.DB())

	// Create a cycle: A -> B -> C -> A
	symbols := []Symbol{
		{Name: "A", Kind: SymbolFunction, FilePath: "cycle.go", StartLine: 1, EndLine: 5, Language: "go", Visibility: "public"},
		{Name: "B", Kind: SymbolFunction, FilePath: "cycle.go", StartLine: 10, EndLine: 15, Language: "go", Visibility: "public"},
		{Name: "C", Kind: SymbolFunction, FilePath: "cycle.go", StartLine: 20, EndLine: 25, Language: "go", Visibility: "public"},
	}

	var ids []uint32
	for i := range symbols {
		id, err := repo.UpsertSymbol(ctx, &symbols[i])
		if err != nil {
			t.Fatalf("Failed to insert symbol: %v", err)
		}
		ids = append(ids, id)
	}

	// Create cyclic relations: A -> B -> C -> A
	relations := []SymbolRelation{
		{FromSymbolID: ids[0], ToSymbolID: ids[1], RelationType: RelationCalls},
		{FromSymbolID: ids[1], ToSymbolID: ids[2], RelationType: RelationCalls},
		{FromSymbolID: ids[2], ToSymbolID: ids[0], RelationType: RelationCalls}, // Cycle!
	}

	for _, rel := range relations {
		if err := repo.UpsertRelation(ctx, &rel); err != nil {
			t.Fatalf("Failed to insert relation: %v", err)
		}
	}

	qs := NewQueryService(repo, emptyllmConfig())

	// This should complete without hanging (cycle handling)
	done := make(chan struct{})
	var analysis *ImpactAnalysis
	var analysisErr error

	go func() {
		analysis, analysisErr = qs.AnalyzeImpact(ctx, ids[2], 10)
		close(done)
	}()

	// Wait with timeout - the key test is that it COMPLETES without hanging
	select {
	case <-done:
		// Success - completed without hanging
	case <-time.After(5 * time.Second):
		t.Fatal("Impact analysis timed out - possible infinite loop due to cycle")
	}

	if analysisErr != nil {
		t.Fatalf("AnalyzeImpact failed: %v", analysisErr)
	}

	// Should find some affected symbols (the cycle participants)
	if analysis.AffectedCount == 0 {
		t.Error("Expected some affected symbols")
	}

	// The key assertion is that the analysis completed in finite time
	// and found the expected callers (B calls C, so B is affected)
	t.Logf("Cycle handling: completed successfully with %d affected symbols", analysis.AffectedCount)
}

// TestQueryService_ImpactAnalysisDiamond tests diamond dependency patterns.
// Regression test: A -> B, A -> C, B -> D, C -> D (diamond shape).
func TestQueryService_ImpactAnalysisDiamond(t *testing.T) {
	ctx := context.Background()

	store, err := memory.NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	repo := NewRepository(store.DB())

	// Diamond: A -> B, A -> C, B -> D, C -> D
	symbols := []Symbol{
		{Name: "A", Kind: SymbolFunction, FilePath: "diamond.go", StartLine: 1, EndLine: 5, Language: "go", Visibility: "public"},
		{Name: "B", Kind: SymbolFunction, FilePath: "diamond.go", StartLine: 10, EndLine: 15, Language: "go", Visibility: "public"},
		{Name: "C", Kind: SymbolFunction, FilePath: "diamond.go", StartLine: 20, EndLine: 25, Language: "go", Visibility: "public"},
		{Name: "D", Kind: SymbolFunction, FilePath: "diamond.go", StartLine: 30, EndLine: 35, Language: "go", Visibility: "public"},
	}

	var ids []uint32
	for i := range symbols {
		id, err := repo.UpsertSymbol(ctx, &symbols[i])
		if err != nil {
			t.Fatalf("Failed to insert symbol: %v", err)
		}
		ids = append(ids, id)
	}

	// Diamond relations
	relations := []SymbolRelation{
		{FromSymbolID: ids[0], ToSymbolID: ids[1], RelationType: RelationCalls}, // A -> B
		{FromSymbolID: ids[0], ToSymbolID: ids[2], RelationType: RelationCalls}, // A -> C
		{FromSymbolID: ids[1], ToSymbolID: ids[3], RelationType: RelationCalls}, // B -> D
		{FromSymbolID: ids[2], ToSymbolID: ids[3], RelationType: RelationCalls}, // C -> D
	}

	for _, rel := range relations {
		if err := repo.UpsertRelation(ctx, &rel); err != nil {
			t.Fatalf("Failed to insert relation: %v", err)
		}
	}

	qs := NewQueryService(repo, emptyllmConfig())

	// Analyze impact of D (called by both B and C)
	analysis, err := qs.AnalyzeImpact(ctx, ids[3], 5)
	if err != nil {
		t.Fatalf("AnalyzeImpact failed: %v", err)
	}

	// D is called by B and C (depth 1), both called by A (depth 2)
	// Should find: B, C at depth 1, A at depth 2 = 3 affected
	if analysis.AffectedCount != 3 {
		t.Errorf("Expected 3 affected symbols in diamond, got %d", analysis.AffectedCount)
	}

	// Verify depths are correct
	depth1 := analysis.ByDepth[1]
	if len(depth1) != 2 {
		t.Errorf("Expected 2 symbols at depth 1, got %d", len(depth1))
	}

	depth2 := analysis.ByDepth[2]
	if len(depth2) != 1 {
		t.Errorf("Expected 1 symbol at depth 2 (A), got %d", len(depth2))
	}
}

// TestQueryService_ImpactAnalysisDeepChain tests deep call chains.
// Regression test: Ensure maxDepth limit is respected.
func TestQueryService_ImpactAnalysisDeepChain(t *testing.T) {
	ctx := context.Background()

	store, err := memory.NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	repo := NewRepository(store.DB())

	// Create a deep chain: func0 -> func1 -> func2 -> ... -> func10
	const depth = 10
	var ids []uint32

	for i := 0; i <= depth; i++ {
		sym := Symbol{
			Name:       "func" + itoa(i),
			Kind:       SymbolFunction,
			FilePath:   "deep.go",
			StartLine:  i * 10,
			EndLine:    i*10 + 5,
			Language:   "go",
			Visibility: "public",
		}
		id, err := repo.UpsertSymbol(ctx, &sym)
		if err != nil {
			t.Fatalf("Failed to insert symbol: %v", err)
		}
		ids = append(ids, id)
	}

	// Create chain relations
	for i := 0; i < depth; i++ {
		rel := SymbolRelation{
			FromSymbolID: ids[i],
			ToSymbolID:   ids[i+1],
			RelationType: RelationCalls,
		}
		if err := repo.UpsertRelation(ctx, &rel); err != nil {
			t.Fatalf("Failed to insert relation: %v", err)
		}
	}

	qs := NewQueryService(repo, emptyllmConfig())

	// Analyze with maxDepth=3 (should stop early)
	analysis, err := qs.AnalyzeImpact(ctx, ids[depth], 3)
	if err != nil {
		t.Fatalf("AnalyzeImpact failed: %v", err)
	}

	// With maxDepth=3, should find func9, func8, func7 (depths 1,2,3)
	if analysis.AffectedCount > 3 {
		t.Errorf("Expected at most 3 affected with maxDepth=3, got %d", analysis.AffectedCount)
	}

	// Verify no depth exceeds maxDepth
	for depthLevel := range analysis.ByDepth {
		if depthLevel > 3 {
			t.Errorf("Found depth %d exceeding maxDepth 3", depthLevel)
		}
	}
}

// TestQueryService_ImpactAnalysisDeterministic ensures results are deterministic.
// Regression test: Same input should always produce same output.
func TestQueryService_ImpactAnalysisDeterministic(t *testing.T) {
	ctx := context.Background()

	store, err := memory.NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	repo := NewRepository(store.DB())

	// Create a graph
	symbols := []Symbol{
		{Name: "root", Kind: SymbolFunction, FilePath: "det.go", StartLine: 1, EndLine: 5, Language: "go", Visibility: "public"},
		{Name: "child1", Kind: SymbolFunction, FilePath: "det.go", StartLine: 10, EndLine: 15, Language: "go", Visibility: "public"},
		{Name: "child2", Kind: SymbolFunction, FilePath: "det.go", StartLine: 20, EndLine: 25, Language: "go", Visibility: "public"},
	}

	var ids []uint32
	for i := range symbols {
		id, err := repo.UpsertSymbol(ctx, &symbols[i])
		if err != nil {
			t.Fatalf("Failed to insert symbol: %v", err)
		}
		ids = append(ids, id)
	}

	relations := []SymbolRelation{
		{FromSymbolID: ids[0], ToSymbolID: ids[1], RelationType: RelationCalls},
		{FromSymbolID: ids[0], ToSymbolID: ids[2], RelationType: RelationCalls},
	}

	for _, rel := range relations {
		if err := repo.UpsertRelation(ctx, &rel); err != nil {
			t.Fatalf("Failed to insert relation: %v", err)
		}
	}

	qs := NewQueryService(repo, emptyllmConfig())

	// Run impact analysis multiple times
	var results []int
	for i := 0; i < 5; i++ {
		analysis, err := qs.AnalyzeImpact(ctx, ids[1], 5)
		if err != nil {
			t.Fatalf("AnalyzeImpact failed on iteration %d: %v", i, err)
		}
		results = append(results, analysis.AffectedCount)
	}

	// All results should be identical
	for i := 1; i < len(results); i++ {
		if results[i] != results[0] {
			t.Errorf("Non-deterministic result: iteration 0=%d, iteration %d=%d",
				results[0], i, results[i])
		}
	}
}

// TestQueryService_GetImplementations tests interface implementation lookup.
func TestQueryService_GetImplementations(t *testing.T) {
	ctx := context.Background()

	store, err := memory.NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	repo := NewRepository(store.DB())

	// Create interface and implementations
	symbols := []Symbol{
		{Name: "Reader", Kind: SymbolInterface, FilePath: "io.go", StartLine: 1, EndLine: 5, Language: "go", Visibility: "public"},
		{Name: "FileReader", Kind: SymbolStruct, FilePath: "file.go", StartLine: 1, EndLine: 10, Language: "go", Visibility: "public"},
		{Name: "BufferReader", Kind: SymbolStruct, FilePath: "buffer.go", StartLine: 1, EndLine: 10, Language: "go", Visibility: "public"},
	}

	var ids []uint32
	for i := range symbols {
		id, err := repo.UpsertSymbol(ctx, &symbols[i])
		if err != nil {
			t.Fatalf("Failed to insert symbol: %v", err)
		}
		ids = append(ids, id)
	}

	// FileReader and BufferReader implement Reader
	relations := []SymbolRelation{
		{FromSymbolID: ids[1], ToSymbolID: ids[0], RelationType: RelationImplements},
		{FromSymbolID: ids[2], ToSymbolID: ids[0], RelationType: RelationImplements},
	}

	for _, rel := range relations {
		if err := repo.UpsertRelation(ctx, &rel); err != nil {
			t.Fatalf("Failed to insert relation: %v", err)
		}
	}

	qs := NewQueryService(repo, emptyllmConfig())

	impls, err := qs.GetImplementations(ctx, ids[0])
	if err != nil {
		t.Fatalf("GetImplementations failed: %v", err)
	}

	if len(impls) != 2 {
		t.Errorf("Expected 2 implementations, got %d", len(impls))
	}
}

// TestQueryService_SymbolDatabaseConsistency tests that symbol operations maintain consistency.
// This ensures the database remains in a valid state after various operations.
func TestQueryService_SymbolDatabaseConsistency(t *testing.T) {
	ctx := context.Background()

	store, err := memory.NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	repo := NewRepository(store.DB())
	qs := NewQueryService(repo, emptyllmConfig())

	// Insert symbols
	sym1 := Symbol{Name: "Func1", Kind: SymbolFunction, FilePath: "a.go", StartLine: 1, EndLine: 10, Language: "go", Visibility: "public"}
	sym2 := Symbol{Name: "Func2", Kind: SymbolFunction, FilePath: "b.go", StartLine: 1, EndLine: 10, Language: "go", Visibility: "public"}

	id1, _ := repo.UpsertSymbol(ctx, &sym1)
	id2, _ := repo.UpsertSymbol(ctx, &sym2)

	// Add relation
	rel := SymbolRelation{FromSymbolID: id1, ToSymbolID: id2, RelationType: RelationCalls}
	_ = repo.UpsertRelation(ctx, &rel)

	// Verify stats consistency
	stats, err := qs.GetStats(ctx)
	if err != nil {
		t.Fatalf("GetStats failed: %v", err)
	}

	if stats.SymbolsFound != 2 {
		t.Errorf("Expected 2 symbols, got %d", stats.SymbolsFound)
	}
	if stats.RelationsFound != 1 {
		t.Errorf("Expected 1 relation, got %d", stats.RelationsFound)
	}

	// Delete one symbol - relations should cascade
	if err := repo.DeleteSymbol(ctx, id2); err != nil {
		t.Fatalf("DeleteSymbol failed: %v", err)
	}

	// Verify consistency after delete
	stats, err = qs.GetStats(ctx)
	if err != nil {
		t.Fatalf("GetStats after delete failed: %v", err)
	}

	if stats.SymbolsFound != 1 {
		t.Errorf("Expected 1 symbol after delete, got %d", stats.SymbolsFound)
	}

	// Relations should be cleaned up due to foreign key cascade
	if stats.RelationsFound != 0 {
		t.Errorf("Expected 0 relations after cascade delete, got %d", stats.RelationsFound)
	}
}

// TestQueryService_HybridSearchDefaultLimit tests that default limit is applied.
func TestQueryService_HybridSearchDefaultLimit(t *testing.T) {
	ctx := context.Background()

	store, err := memory.NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	repo := NewRepository(store.DB())

	// Insert many symbols
	for i := 0; i < 50; i++ {
		sym := Symbol{
			Name:       "TestFunc" + itoa(i),
			Kind:       SymbolFunction,
			FilePath:   "test.go",
			StartLine:  i * 10,
			EndLine:    i*10 + 5,
			Language:   "go",
			Visibility: "public",
		}
		_, _ = repo.UpsertSymbol(ctx, &sym)
	}
	_ = repo.RebuildSymbolsFTS(ctx)

	qs := NewQueryService(repo, emptyllmConfig())

	// Search with limit=0 should use default (20)
	results, err := qs.HybridSearch(ctx, "TestFunc", 0)
	if err != nil {
		t.Fatalf("HybridSearch failed: %v", err)
	}

	if len(results) > 20 {
		t.Errorf("Expected at most 20 results (default limit), got %d", len(results))
	}
}

// TestQueryService_AnalyzeImpactDefaultDepth tests that default depth is applied.
func TestQueryService_AnalyzeImpactDefaultDepth(t *testing.T) {
	ctx := context.Background()

	store, err := memory.NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	repo := NewRepository(store.DB())

	sym := Symbol{Name: "test", Kind: SymbolFunction, FilePath: "test.go", StartLine: 1, EndLine: 5, Language: "go", Visibility: "public"}
	id, _ := repo.UpsertSymbol(ctx, &sym)

	qs := NewQueryService(repo, emptyllmConfig())

	// Analyze with depth=0 should use default
	analysis, err := qs.AnalyzeImpact(ctx, id, 0)
	if err != nil {
		t.Fatalf("AnalyzeImpact failed: %v", err)
	}

	// MaxDepth should be set to default (5)
	if analysis.MaxDepth != 5 {
		t.Errorf("Expected default maxDepth 5, got %d", analysis.MaxDepth)
	}
}

// TestCodeIntel_NoMarkdownLeakage verifies that symbol data is NOT written to markdown files.
// CRITICAL: Per CLAUDE.md, symbol data must remain in SQLite only to avoid filesystem bloat.
// Unlike architectural decisions, symbols are transient and should not be persisted to markdown.
func TestCodeIntel_NoMarkdownLeakage(t *testing.T) {
	ctx := context.Background()

	// Create temp directory for test
	tmpDir, err := os.MkdirTemp("", "codeintel-md-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create memory path structure
	memDir := filepath.Join(tmpDir, ".taskwing", "memory")
	if err := os.MkdirAll(memDir, 0755); err != nil {
		t.Fatalf("Failed to create memory dir: %v", err)
	}

	// Create SQLite store in the memory directory
	dbPath := filepath.Join(memDir, "memory.db")
	store, err := memory.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	repo := NewRepository(store.DB())

	// Insert symbols
	symbols := []Symbol{
		{Name: "TestFunction", Kind: SymbolFunction, FilePath: "test.go", StartLine: 1, EndLine: 10, Language: "go", Visibility: "public", DocComment: "Test function doc"},
		{Name: "TestStruct", Kind: SymbolStruct, FilePath: "test.go", StartLine: 15, EndLine: 25, Language: "go", Visibility: "public", DocComment: "Test struct doc"},
		{Name: "TestInterface", Kind: SymbolInterface, FilePath: "test.go", StartLine: 30, EndLine: 40, Language: "go", Visibility: "public", DocComment: "Test interface doc"},
	}

	for i := range symbols {
		_, err := repo.UpsertSymbol(ctx, &symbols[i])
		if err != nil {
			t.Fatalf("Failed to insert symbol: %v", err)
		}
	}

	// Add relations
	ids := []uint32{1, 2, 3}
	rel := SymbolRelation{FromSymbolID: ids[0], ToSymbolID: ids[1], RelationType: RelationCalls}
	if err := repo.UpsertRelation(ctx, &rel); err != nil {
		t.Fatalf("Failed to insert relation: %v", err)
	}

	// Verify symbols are in SQLite
	count, err := repo.GetSymbolCount(ctx)
	if err != nil {
		t.Fatalf("GetSymbolCount failed: %v", err)
	}
	if count != 3 {
		t.Errorf("Expected 3 symbols in SQLite, got %d", count)
	}

	// CRITICAL CHECK: Verify NO markdown files were created for symbols
	mdFiles, err := filepath.Glob(filepath.Join(memDir, "*.md"))
	if err != nil {
		t.Fatalf("Failed to glob markdown files: %v", err)
	}

	// Check for any markdown files with symbol-related content
	for _, mdFile := range mdFiles {
		content, err := os.ReadFile(mdFile)
		if err != nil {
			continue
		}
		contentStr := string(content)

		// Symbol-specific markers that should NOT exist
		symbolMarkers := []string{
			"TestFunction",
			"TestStruct",
			"TestInterface",
			"SymbolFunction",
			"SymbolStruct",
			"SymbolInterface",
			"symbol_relations",
		}

		for _, marker := range symbolMarkers {
			if strings.Contains(contentStr, marker) {
				t.Errorf("Markdown file %s contains symbol data: found '%s'. "+
					"Symbol data should stay in SQLite only!", mdFile, marker)
			}
		}
	}

	// Also check features subdirectory (if exists) for leakage
	featuresDir := filepath.Join(memDir, "features")
	if _, err := os.Stat(featuresDir); err == nil {
		entries, _ := os.ReadDir(featuresDir)
		for _, entry := range entries {
			if strings.HasSuffix(entry.Name(), ".md") {
				content, err := os.ReadFile(filepath.Join(featuresDir, entry.Name()))
				if err != nil {
					continue
				}
				for _, marker := range []string{"TestFunction", "TestStruct", "TestInterface"} {
					if strings.Contains(string(content), marker) {
						t.Errorf("Feature file %s contains symbol data: found '%s'. "+
							"Symbol data should stay in SQLite only!", entry.Name(), marker)
					}
				}
			}
		}
	}

	// Verify the SQLite tables exist but no markdown references
	qs := NewQueryService(repo, emptyllmConfig())

	// Search should work (data is in SQLite)
	if err := repo.RebuildSymbolsFTS(ctx); err != nil {
		t.Fatalf("Failed to rebuild FTS: %v", err)
	}

	results, err := qs.HybridSearch(ctx, "Test", 10)
	if err != nil {
		t.Fatalf("HybridSearch failed: %v", err)
	}

	if len(results) == 0 {
		t.Error("Expected symbols to be searchable from SQLite")
	}

	t.Log("Verified: Symbol data correctly stored in SQLite only, no markdown leakage")
}
