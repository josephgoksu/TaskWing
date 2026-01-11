// Package parser provides multi-language source code parsing for symbol extraction.
// This file defines the common interface that all language parsers must implement.
package parser

// LanguageParser defines the interface for language-specific code parsers.
// All language implementations (Go, TypeScript, Python, Rust) must satisfy this interface
// to be used by the indexer and query services.
type LanguageParser interface {
	// ParseFile parses a single source file and extracts symbols and relations.
	// The filePath must be an absolute path to an existing file.
	// Returns a ParseResult containing all extracted symbols and their relationships.
	ParseFile(filePath string) (*ParseResult, error)

	// ParseDirectory parses all supported files in a directory recursively.
	// Skips hidden directories, vendor folders, and other common non-source directories.
	ParseDirectory(dirPath string) (*ParseResult, error)

	// SupportedExtensions returns the file extensions this parser can handle.
	// Extensions should include the leading dot (e.g., ".go", ".ts", ".py").
	SupportedExtensions() []string

	// Language returns the language identifier for this parser.
	// This is used for tagging symbols and filtering queries.
	// Examples: "go", "typescript", "python", "rust"
	Language() string

	// CanParse returns true if this parser can handle the given file path.
	// This is typically based on file extension, but parsers may use other heuristics.
	CanParse(filePath string) bool
}

// Ensure all parsers implement LanguageParser at compile time.
// These will fail compilation if any parser doesn't satisfy the interface.
var (
	_ LanguageParser = (*GoParser)(nil)
	_ LanguageParser = (*TypeScriptParser)(nil)
	_ LanguageParser = (*PythonParser)(nil)
	_ LanguageParser = (*RustParser)(nil)
)
