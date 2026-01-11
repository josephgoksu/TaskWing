package codeintel

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/josephgoksu/TaskWing/internal/memory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestDB creates a temporary SQLite database for testing.
func setupTestDB(t *testing.T) (*memory.SQLiteStore, *SQLiteRepository, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "tw-codeintel-repo-*")
	require.NoError(t, err)

	store, err := memory.NewSQLiteStore(filepath.Join(tmpDir, "memory"))
	require.NoError(t, err)

	// Get the underlying DB handle for the repository
	repo := NewRepository(store.DB())

	cleanup := func() {
		_ = store.Close()
		_ = os.RemoveAll(tmpDir)
	}

	return store, repo, cleanup
}

func TestRepository_UpsertAndGetSymbol(t *testing.T) {
	_, repo, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	// Create a symbol
	symbol := &Symbol{
		Name:       "TestFunction",
		Kind:       SymbolFunction,
		FilePath:   "internal/test/test.go",
		StartLine:  10,
		EndLine:    25,
		Signature:  "func TestFunction(ctx context.Context) error",
		DocComment: "TestFunction is a test function for unit tests.",
		ModulePath: "internal/test",
		Visibility: "public",
		Language:   "go",
	}

	// Insert
	id, err := repo.UpsertSymbol(ctx, symbol)
	require.NoError(t, err)
	assert.NotZero(t, id)

	// Retrieve
	retrieved, err := repo.GetSymbol(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, symbol.Name, retrieved.Name)
	assert.Equal(t, symbol.Kind, retrieved.Kind)
	assert.Equal(t, symbol.FilePath, retrieved.FilePath)
	assert.Equal(t, symbol.StartLine, retrieved.StartLine)
	assert.Equal(t, symbol.EndLine, retrieved.EndLine)
	assert.Equal(t, symbol.Signature, retrieved.Signature)
	assert.Equal(t, symbol.DocComment, retrieved.DocComment)
	assert.Equal(t, symbol.ModulePath, retrieved.ModulePath)
	assert.Equal(t, symbol.Visibility, retrieved.Visibility)
	assert.Equal(t, symbol.Language, retrieved.Language)
}

func TestRepository_UpsertSymbol_Update(t *testing.T) {
	_, repo, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	// Create initial symbol
	symbol := &Symbol{
		Name:      "MyFunc",
		Kind:      SymbolFunction,
		FilePath:  "test.go",
		StartLine: 1,
		EndLine:   10,
		Signature: "func MyFunc()",
		Language:  "go",
	}

	id1, err := repo.UpsertSymbol(ctx, symbol)
	require.NoError(t, err)

	// Update same symbol (same name, file, line)
	symbol.EndLine = 20
	symbol.Signature = "func MyFunc(x int)"

	id2, err := repo.UpsertSymbol(ctx, symbol)
	require.NoError(t, err)
	assert.Equal(t, id1, id2, "Should return same ID on upsert")

	// Verify update
	retrieved, err := repo.GetSymbol(ctx, id1)
	require.NoError(t, err)
	assert.Equal(t, 20, retrieved.EndLine)
	assert.Equal(t, "func MyFunc(x int)", retrieved.Signature)
}

func TestRepository_DeleteSymbol(t *testing.T) {
	_, repo, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	// Create and delete
	symbol := &Symbol{
		Name:      "ToDelete",
		Kind:      SymbolFunction,
		FilePath:  "delete.go",
		StartLine: 1,
		EndLine:   5,
		Language:  "go",
	}

	id, err := repo.UpsertSymbol(ctx, symbol)
	require.NoError(t, err)

	err = repo.DeleteSymbol(ctx, id)
	require.NoError(t, err)

	// Verify deleted
	_, err = repo.GetSymbol(ctx, id)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestRepository_DeleteSymbolsByFile(t *testing.T) {
	_, repo, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	// Create multiple symbols in same file
	for i := 1; i <= 3; i++ {
		_, err := repo.UpsertSymbol(ctx, &Symbol{
			Name:      "Func" + itoa(i),
			Kind:      SymbolFunction,
			FilePath:  "target.go",
			StartLine: i * 10,
			EndLine:   i*10 + 5,
			Language:  "go",
		})
		require.NoError(t, err)
	}

	// Create symbol in different file
	_, err := repo.UpsertSymbol(ctx, &Symbol{
		Name:      "OtherFunc",
		Kind:      SymbolFunction,
		FilePath:  "other.go",
		StartLine: 1,
		EndLine:   5,
		Language:  "go",
	})
	require.NoError(t, err)

	// Verify 4 symbols total
	count, err := repo.GetSymbolCount(ctx)
	require.NoError(t, err)
	assert.Equal(t, 4, count)

	// Delete by file
	err = repo.DeleteSymbolsByFile(ctx, "target.go")
	require.NoError(t, err)

	// Verify only 1 remains
	count, err = repo.GetSymbolCount(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestRepository_FindSymbolsByName(t *testing.T) {
	_, repo, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	// Create symbols with same name in different files
	for _, file := range []string{"a.go", "b.go", "c.go"} {
		_, err := repo.UpsertSymbol(ctx, &Symbol{
			Name:      "CommonName",
			Kind:      SymbolFunction,
			FilePath:  file,
			StartLine: 1,
			EndLine:   10,
			Language:  "go",
		})
		require.NoError(t, err)
	}

	// Find all
	symbols, err := repo.FindSymbolsByName(ctx, "CommonName", nil)
	require.NoError(t, err)
	assert.Len(t, symbols, 3)

	// Find with language filter
	lang := "go"
	symbols, err = repo.FindSymbolsByName(ctx, "CommonName", &lang)
	require.NoError(t, err)
	assert.Len(t, symbols, 3)

	// No results for different language
	lang = "python"
	symbols, err = repo.FindSymbolsByName(ctx, "CommonName", &lang)
	require.NoError(t, err)
	assert.Len(t, symbols, 0)
}

func TestRepository_FindSymbolsByFile(t *testing.T) {
	_, repo, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	// Create multiple symbols in same file
	for i := 1; i <= 5; i++ {
		_, err := repo.UpsertSymbol(ctx, &Symbol{
			Name:      "Func" + itoa(i),
			Kind:      SymbolFunction,
			FilePath:  "target.go",
			StartLine: i * 10,
			EndLine:   i*10 + 5,
			Language:  "go",
		})
		require.NoError(t, err)
	}

	symbols, err := repo.FindSymbolsByFile(ctx, "target.go")
	require.NoError(t, err)
	assert.Len(t, symbols, 5)

	// Verify ordering by start_line
	for i := 0; i < len(symbols)-1; i++ {
		assert.Less(t, symbols[i].StartLine, symbols[i+1].StartLine)
	}
}

func TestRepository_SearchSymbolsFTS(t *testing.T) {
	_, repo, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	// Create symbols with searchable content
	_, err := repo.UpsertSymbol(ctx, &Symbol{
		Name:       "CreateUserHandler",
		Kind:       SymbolFunction,
		FilePath:   "handler.go",
		StartLine:  1,
		EndLine:    20,
		Signature:  "func CreateUserHandler(w http.ResponseWriter, r *http.Request)",
		DocComment: "CreateUserHandler handles user creation requests",
		Language:   "go",
	})
	require.NoError(t, err)

	_, err = repo.UpsertSymbol(ctx, &Symbol{
		Name:       "DeleteUserHandler",
		Kind:       SymbolFunction,
		FilePath:   "handler.go",
		StartLine:  25,
		EndLine:    40,
		Signature:  "func DeleteUserHandler(w http.ResponseWriter, r *http.Request)",
		DocComment: "DeleteUserHandler handles user deletion requests",
		Language:   "go",
	})
	require.NoError(t, err)

	// Search by exact name (FTS5 tokenizes on whitespace/punctuation)
	symbols, err := repo.SearchSymbolsFTS(ctx, "CreateUserHandler", 10)
	require.NoError(t, err)
	require.Len(t, symbols, 1, "Should find CreateUserHandler by exact name")
	assert.Equal(t, "CreateUserHandler", symbols[0].Name)

	// Search by doc content word
	symbols, err = repo.SearchSymbolsFTS(ctx, "deletion", 10)
	require.NoError(t, err)
	require.Len(t, symbols, 1, "Should find DeleteUserHandler by doc_comment word")
	assert.Equal(t, "DeleteUserHandler", symbols[0].Name)

	// Search matching multiple - both doc_comments have "handles"
	symbols, err = repo.SearchSymbolsFTS(ctx, "handles", 10)
	require.NoError(t, err)
	assert.Len(t, symbols, 2, "Should find both symbols with 'handles' in doc")

	// Prefix search with wildcard
	symbols, err = repo.SearchSymbolsFTS(ctx, "Create*", 10)
	require.NoError(t, err)
	assert.Len(t, symbols, 1, "Should find CreateUserHandler with prefix search")
}

func TestRepository_Relations(t *testing.T) {
	_, repo, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	// Create caller and callee
	callerID, err := repo.UpsertSymbol(ctx, &Symbol{
		Name:      "Caller",
		Kind:      SymbolFunction,
		FilePath:  "caller.go",
		StartLine: 1,
		EndLine:   10,
		Language:  "go",
	})
	require.NoError(t, err)

	calleeID, err := repo.UpsertSymbol(ctx, &Symbol{
		Name:      "Callee",
		Kind:      SymbolFunction,
		FilePath:  "callee.go",
		StartLine: 1,
		EndLine:   10,
		Language:  "go",
	})
	require.NoError(t, err)

	// Create relation
	rel := &SymbolRelation{
		FromSymbolID: callerID,
		ToSymbolID:   calleeID,
		RelationType: RelationCalls,
		CallSiteLine: 5,
	}
	err = repo.UpsertRelation(ctx, rel)
	require.NoError(t, err)

	// Verify relation count
	count, err := repo.GetRelationCount(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	// Get callers of callee
	callers, err := repo.GetCallers(ctx, calleeID)
	require.NoError(t, err)
	require.Len(t, callers, 1)
	assert.Equal(t, "Caller", callers[0].Name)

	// Get callees of caller
	callees, err := repo.GetCallees(ctx, callerID)
	require.NoError(t, err)
	require.Len(t, callees, 1)
	assert.Equal(t, "Callee", callees[0].Name)
}

func TestRepository_GetImpactRadius(t *testing.T) {
	_, repo, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	// Create call chain: A -> B -> C -> D
	ids := make([]uint32, 4)
	for i, name := range []string{"FuncA", "FuncB", "FuncC", "FuncD"} {
		id, err := repo.UpsertSymbol(ctx, &Symbol{
			Name:      name,
			Kind:      SymbolFunction,
			FilePath:  "chain.go",
			StartLine: i * 10,
			EndLine:   i*10 + 5,
			Language:  "go",
		})
		require.NoError(t, err)
		ids[i] = id
	}

	// Create relations: A->B, B->C, C->D
	for i := 0; i < 3; i++ {
		err := repo.UpsertRelation(ctx, &SymbolRelation{
			FromSymbolID: ids[i],
			ToSymbolID:   ids[i+1],
			RelationType: RelationCalls,
		})
		require.NoError(t, err)
	}

	// Impact of changing D (should affect C, B, A)
	impact, err := repo.GetImpactRadius(ctx, ids[3], 10)
	require.NoError(t, err)
	require.Len(t, impact, 3)

	// Verify depths
	depthMap := make(map[string]int)
	for _, node := range impact {
		depthMap[node.Symbol.Name] = node.Depth
	}
	assert.Equal(t, 1, depthMap["FuncC"])
	assert.Equal(t, 2, depthMap["FuncB"])
	assert.Equal(t, 3, depthMap["FuncA"])
}

func TestRepository_GetImplementations(t *testing.T) {
	_, repo, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	// Create interface
	interfaceID, err := repo.UpsertSymbol(ctx, &Symbol{
		Name:      "Reader",
		Kind:      SymbolInterface,
		FilePath:  "io.go",
		StartLine: 1,
		EndLine:   5,
		Language:  "go",
	})
	require.NoError(t, err)

	// Create implementing types
	for _, name := range []string{"FileReader", "StringReader", "BufferReader"} {
		implID, err := repo.UpsertSymbol(ctx, &Symbol{
			Name:      name,
			Kind:      SymbolStruct,
			FilePath:  "readers.go",
			StartLine: 1,
			EndLine:   10,
			Language:  "go",
		})
		require.NoError(t, err)

		err = repo.UpsertRelation(ctx, &SymbolRelation{
			FromSymbolID: implID,
			ToSymbolID:   interfaceID,
			RelationType: RelationImplements,
		})
		require.NoError(t, err)
	}

	// Get implementations
	impls, err := repo.GetImplementations(ctx, interfaceID)
	require.NoError(t, err)
	assert.Len(t, impls, 3)

	names := make([]string, len(impls))
	for i, impl := range impls {
		names[i] = impl.Name
	}
	assert.Contains(t, names, "FileReader")
	assert.Contains(t, names, "StringReader")
	assert.Contains(t, names, "BufferReader")
}

func TestRepository_Embeddings(t *testing.T) {
	_, repo, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	// Create symbol without embedding
	id, err := repo.UpsertSymbol(ctx, &Symbol{
		Name:      "NeedsEmbedding",
		Kind:      SymbolFunction,
		FilePath:  "test.go",
		StartLine: 1,
		EndLine:   10,
		Language:  "go",
	})
	require.NoError(t, err)

	// Get symbols without embeddings
	noEmbed, err := repo.GetSymbolsWithoutEmbeddings(ctx, 10)
	require.NoError(t, err)
	assert.Len(t, noEmbed, 1)

	// Update with embedding
	embedding := make([]float32, 384)
	for i := range embedding {
		embedding[i] = float32(i) * 0.001
	}
	err = repo.UpdateSymbolEmbedding(ctx, id, embedding)
	require.NoError(t, err)

	// Verify embedding stored
	withEmbed, err := repo.ListSymbolsWithEmbeddings(ctx)
	require.NoError(t, err)
	require.Len(t, withEmbed, 1)
	assert.Len(t, withEmbed[0].Embedding, 384)

	// No more symbols without embeddings
	noEmbed, err = repo.GetSymbolsWithoutEmbeddings(ctx, 10)
	require.NoError(t, err)
	assert.Len(t, noEmbed, 0)
}

func TestRepository_Statistics(t *testing.T) {
	_, repo, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	// Initially empty
	count, err := repo.GetSymbolCount(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, count)

	// Add symbols in different files
	for i, file := range []string{"a.go", "b.go", "c.go"} {
		_, err := repo.UpsertSymbol(ctx, &Symbol{
			Name:      "Func" + itoa(i),
			Kind:      SymbolFunction,
			FilePath:  file,
			StartLine: 1,
			EndLine:   10,
			Language:  "go",
		})
		require.NoError(t, err)
	}

	// Verify counts
	count, err = repo.GetSymbolCount(ctx)
	require.NoError(t, err)
	assert.Equal(t, 3, count)

	fileCount, err := repo.GetFileCount(ctx)
	require.NoError(t, err)
	assert.Equal(t, 3, fileCount)
}

func TestRepository_RebuildFTS(t *testing.T) {
	_, repo, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	// Create symbols
	_, err := repo.UpsertSymbol(ctx, &Symbol{
		Name:       "SearchMe",
		Kind:       SymbolFunction,
		FilePath:   "test.go",
		StartLine:  1,
		EndLine:    10,
		DocComment: "This function does something special",
		Language:   "go",
	})
	require.NoError(t, err)

	// Rebuild FTS
	err = repo.RebuildSymbolsFTS(ctx)
	require.NoError(t, err)

	// Search should still work
	results, err := repo.SearchSymbolsFTS(ctx, "special", 10)
	require.NoError(t, err)
	assert.Len(t, results, 1)
}

func TestNewRepository_ImplementsInterface(t *testing.T) {
	// This test verifies that SQLiteRepository implements Repository interface
	// at compile time (via the var _ Repository = (*SQLiteRepository)(nil) line)
	// but we can also verify it at runtime
	_, repo, cleanup := setupTestDB(t)
	defer cleanup()

	var _ Repository = repo
}
