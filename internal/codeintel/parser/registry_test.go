package parser

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGoParserImplementsInterface(t *testing.T) {
	// This test verifies that GoParser satisfies the LanguageParser interface.
	// The compile-time check in interface.go also enforces this.
	var _ LanguageParser = (*GoParser)(nil)

	parser := NewGoParser("/test")
	assert.NotNil(t, parser)
	assert.Equal(t, "go", parser.Language())
	assert.Equal(t, []string{".go"}, parser.SupportedExtensions())
	assert.True(t, parser.CanParse("main.go"))
	assert.True(t, parser.CanParse("/path/to/file.go"))
	assert.False(t, parser.CanParse("main.ts"))
	assert.False(t, parser.CanParse("main.py"))
	assert.False(t, parser.CanParse("main.rs"))
}

func TestNewParserRegistry(t *testing.T) {
	registry := NewParserRegistry()
	assert.NotNil(t, registry)
	assert.Empty(t, registry.SupportedExtensions())
	assert.Empty(t, registry.RegisteredLanguages())
}

func TestParserRegistry_Register(t *testing.T) {
	registry := NewParserRegistry()
	goParser := NewGoParser("/test")

	registry.Register(goParser)

	// Verify registration
	assert.Contains(t, registry.SupportedExtensions(), ".go")
	assert.Contains(t, registry.RegisteredLanguages(), "go")

	// Verify retrieval by extension
	parser := registry.GetParserByExtension(".go")
	assert.NotNil(t, parser)
	assert.Equal(t, "go", parser.Language())

	// Test with and without leading dot
	parser = registry.GetParserByExtension("go")
	assert.NotNil(t, parser)
	assert.Equal(t, "go", parser.Language())
}

func TestParserRegistry_GetParserForFile(t *testing.T) {
	registry := NewDefaultRegistry("/test")

	tests := []struct {
		filePath string
		wantLang string
		wantNil  bool
	}{
		{"main.go", "go", false},
		{"/path/to/file.go", "go", false},
		{"internal/parser/go_parser.go", "go", false},
		{"main.ts", "typescript", false},
		{"main.tsx", "typescript", false},
		{"main.js", "typescript", false},
		{"main.jsx", "typescript", false},
		{"main.py", "python", false},
		{"main.pyi", "python", false},
		{"main.rs", "rust", false},
		{"main.txt", "", true}, // Not a source file
		{"Makefile", "", true}, // No extension
	}

	for _, tt := range tests {
		t.Run(tt.filePath, func(t *testing.T) {
			parser := registry.GetParserForFile(tt.filePath)
			if tt.wantNil {
				assert.Nil(t, parser)
			} else {
				require.NotNil(t, parser)
				assert.Equal(t, tt.wantLang, parser.Language())
			}
		})
	}
}

func TestParserRegistry_CanParse(t *testing.T) {
	registry := NewDefaultRegistry("/test")

	assert.True(t, registry.CanParse("main.go"))
	assert.True(t, registry.CanParse("/path/to/file.GO")) // Case insensitive
	assert.True(t, registry.CanParse("main.ts"))
	assert.True(t, registry.CanParse("main.tsx"))
	assert.True(t, registry.CanParse("main.js"))
	assert.True(t, registry.CanParse("main.py"))
	assert.True(t, registry.CanParse("main.rs"))
	assert.False(t, registry.CanParse("main.java")) // Not supported yet
	assert.False(t, registry.CanParse("main.cpp"))  // Not supported yet
}

func TestParserRegistry_Unregister(t *testing.T) {
	registry := NewParserRegistry()
	goParser := NewGoParser("/test")

	registry.Register(goParser)
	assert.True(t, registry.CanParse("main.go"))

	registry.Unregister(goParser)
	assert.False(t, registry.CanParse("main.go"))
	assert.Empty(t, registry.SupportedExtensions())
}

func TestParserRegistry_ParseFile(t *testing.T) {
	// Create a temporary Go file
	tmpDir := t.TempDir()
	goFile := filepath.Join(tmpDir, "test.go")
	err := os.WriteFile(goFile, []byte(`package test

// TestFunc is a test function.
func TestFunc() string {
	return "hello"
}
`), 0644)
	require.NoError(t, err)

	registry := NewDefaultRegistry(tmpDir)

	// Parse Go file
	result, err := registry.ParseFile(goFile)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotEmpty(t, result.Symbols)

	// Verify we found the function
	var found bool
	for _, sym := range result.Symbols {
		if sym.Name == "TestFunc" && sym.Kind == SymbolFunction {
			found = true
			assert.Equal(t, "go", sym.Language)
			assert.Equal(t, "public", sym.Visibility)
			assert.Contains(t, sym.DocComment, "test function")
			break
		}
	}
	assert.True(t, found, "TestFunc should be found in parsed symbols")

	// Try parsing TypeScript file (now supported)
	tsFile := filepath.Join(tmpDir, "test.ts")
	err = os.WriteFile(tsFile, []byte(`export const x = 1;`), 0644)
	require.NoError(t, err)

	tsResult, err := registry.ParseFile(tsFile)
	require.NoError(t, err)
	require.NotNil(t, tsResult)

	// Try parsing unsupported file
	unsupportedFile := filepath.Join(tmpDir, "test.java")
	err = os.WriteFile(unsupportedFile, []byte(`public class Test {}`), 0644)
	require.NoError(t, err)

	_, err = registry.ParseFile(unsupportedFile)
	assert.Error(t, err)

	var unsupportedErr *UnsupportedFileError
	assert.ErrorAs(t, err, &unsupportedErr)
	assert.Equal(t, ".java", unsupportedErr.Extension)
}

func TestUnsupportedFileError(t *testing.T) {
	err := &UnsupportedFileError{
		FilePath:  "/path/to/file.xyz",
		Extension: ".xyz",
	}
	assert.Contains(t, err.Error(), ".xyz")
	assert.Contains(t, err.Error(), "/path/to/file.xyz")
}

func TestParserRegistry_ConcurrentAccess(t *testing.T) {
	registry := NewDefaultRegistry("/test")

	// Test concurrent reads
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				_ = registry.GetParserForFile("main.go")
				_ = registry.CanParse("main.go")
				_ = registry.SupportedExtensions()
			}
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}
