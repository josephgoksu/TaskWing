package tools

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewCodeChunker(t *testing.T) {
	chunker := NewCodeChunker("/test/path")
	if chunker == nil {
		t.Fatal("NewCodeChunker returned nil")
	}
	if chunker.basePath != "/test/path" {
		t.Errorf("basePath = %q, want %q", chunker.basePath, "/test/path")
	}
}

func TestDefaultChunkConfig(t *testing.T) {
	cfg := DefaultChunkConfig()
	if cfg.MaxTokensPerChunk != 30000 {
		t.Errorf("MaxTokensPerChunk = %d, want 30000", cfg.MaxTokensPerChunk)
	}
	if cfg.MaxFilesPerChunk != 50 {
		t.Errorf("MaxFilesPerChunk = %d, want 50", cfg.MaxFilesPerChunk)
	}
	if !cfg.IncludeLineNumbers {
		t.Error("IncludeLineNumbers should be true by default")
	}
}

func TestCodeChunker_ChunkSourceCode(t *testing.T) {
	// Create temp directory with test files
	tmpDir := t.TempDir()

	// Create some source files
	files := map[string]string{
		"main.go":           "package main\n\nfunc main() {\n\tprintln(\"hello\")\n}\n",
		"handler.go":        "package main\n\nfunc handleRequest() {}\n",
		"internal/util.go":  "package internal\n\nfunc Util() {}\n",
		"internal/store.go": "package internal\n\nfunc Store() {}\n",
	}

	for path, content := range files {
		fullPath := filepath.Join(tmpDir, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatalf("Failed to create dir: %v", err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write file: %v", err)
		}
	}

	chunker := NewCodeChunker(tmpDir)
	chunks, err := chunker.ChunkSourceCode()
	if err != nil {
		t.Fatalf("ChunkSourceCode failed: %v", err)
	}

	if len(chunks) == 0 {
		t.Fatal("Expected at least one chunk")
	}

	// Verify all files were read
	coverage := chunker.GetCoverage()
	if len(coverage.FilesRead) != 4 {
		t.Errorf("Expected 4 files read, got %d", len(coverage.FilesRead))
	}
}

func TestCodeChunker_ChunkSourceCode_MultipleChunks(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a chunk config with very small limits to force multiple chunks
	chunker := NewCodeChunker(tmpDir)
	chunker.SetConfig(ChunkConfig{
		MaxTokensPerChunk:  100, // Very small to force chunking
		MaxFilesPerChunk:   2,   // Only 2 files per chunk
		IncludeLineNumbers: true,
	})

	// Create 5 files
	for i := 0; i < 5; i++ {
		filename := filepath.Join(tmpDir, "file"+string(rune('a'+i))+".go")
		content := "package main\n\nfunc Test" + string(rune('A'+i)) + "() {}\n"
		if err := os.WriteFile(filename, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write file: %v", err)
		}
	}

	chunks, err := chunker.ChunkSourceCode()
	if err != nil {
		t.Fatalf("ChunkSourceCode failed: %v", err)
	}

	// With 5 files and max 2 per chunk, should have at least 3 chunks
	if len(chunks) < 3 {
		t.Errorf("Expected at least 3 chunks, got %d", len(chunks))
	}

	// Verify chunk indices are sequential
	for i, chunk := range chunks {
		if chunk.Index != i {
			t.Errorf("Chunk %d has index %d", i, chunk.Index)
		}
	}
}

func TestCodeChunker_SkipsTestFiles(t *testing.T) {
	tmpDir := t.TempDir()

	files := map[string]string{
		"main.go":      "package main\nfunc main() {}\n",
		"main_test.go": "package main\nfunc TestMain() {}\n",
		"util.spec.ts": "describe('util', () => {})",
		"app.go":       "package main\nfunc app() {}\n",
	}

	for path, content := range files {
		if err := os.WriteFile(filepath.Join(tmpDir, path), []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write file: %v", err)
		}
	}

	chunker := NewCodeChunker(tmpDir)
	_, err := chunker.ChunkSourceCode()
	if err != nil {
		t.Fatalf("ChunkSourceCode failed: %v", err)
	}

	coverage := chunker.GetCoverage()

	// Should read main.go and app.go, skip test files
	if len(coverage.FilesRead) != 2 {
		t.Errorf("Expected 2 files read, got %d", len(coverage.FilesRead))
	}

	// Test files should be in skipped
	hasSkippedTest := false
	for _, skip := range coverage.FilesSkipped {
		if skip.Reason == "test file" {
			hasSkippedTest = true
			break
		}
	}
	if !hasSkippedTest {
		t.Error("Expected test files to be skipped")
	}
}

func TestCodeChunker_PriorityOrdering(t *testing.T) {
	tmpDir := t.TempDir()

	// Create files with different priorities
	// main.go should be higher priority than random.go
	files := map[string]string{
		"zzz_random.go":       "package main\nfunc random() {}\n",
		"main.go":             "package main\nfunc main() {}\n",
		"internal/handler.go": "package internal\nfunc handler() {}\n",
	}

	for path, content := range files {
		fullPath := filepath.Join(tmpDir, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatalf("Failed to create dir: %v", err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write file: %v", err)
		}
	}

	chunker := NewCodeChunker(tmpDir)
	chunks, err := chunker.ChunkSourceCode()
	if err != nil {
		t.Fatalf("ChunkSourceCode failed: %v", err)
	}

	if len(chunks) == 0 || len(chunks[0].Files) == 0 {
		t.Fatal("Expected at least one chunk with files")
	}

	// First file should be main.go (priority 1) not zzz_random.go
	firstFile := chunks[0].Files[0].RelPath
	if firstFile != "main.go" {
		t.Errorf("Expected main.go as first file (highest priority), got %s", firstFile)
	}
}

func TestCodeChunker_EmptyDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	chunker := NewCodeChunker(tmpDir)
	_, err := chunker.ChunkSourceCode()

	if err == nil {
		t.Error("Expected error for empty directory")
	}
}
