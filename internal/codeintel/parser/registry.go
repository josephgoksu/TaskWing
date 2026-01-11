// Package parser provides multi-language source code parsing for symbol extraction.
// This file implements the ParserRegistry for managing language-specific parsers.
package parser

import (
	"path/filepath"
	"strings"
	"sync"
)

// ParserRegistry manages language-specific parsers and routes files to the appropriate parser.
// It follows the factory pattern and is safe for concurrent use.
type ParserRegistry struct {
	mu      sync.RWMutex
	parsers map[string]LanguageParser // extension (with dot) -> parser
}

// NewParserRegistry creates a new empty parser registry.
// Use Register() to add language parsers.
func NewParserRegistry() *ParserRegistry {
	return &ParserRegistry{
		parsers: make(map[string]LanguageParser),
	}
}

// NewDefaultRegistry creates a registry pre-populated with all available parsers.
// The basePath is used for computing relative paths in parsed symbols.
func NewDefaultRegistry(basePath string) *ParserRegistry {
	r := NewParserRegistry()

	// Register Go parser (uses native go/ast, most accurate)
	goParser := NewGoParser(basePath)
	r.Register(goParser)

	// Register TypeScript/JavaScript parser (regex-based, CGO-free)
	tsParser := NewTypeScriptParser(basePath)
	r.Register(tsParser)

	// Register Python parser (regex-based, CGO-free)
	pyParser := NewPythonParser(basePath)
	r.Register(pyParser)

	// Register Rust parser (regex-based, CGO-free)
	rustParser := NewRustParser(basePath)
	r.Register(rustParser)

	return r
}

// Register adds a parser to the registry for all its supported extensions.
// If a parser is already registered for an extension, it will be replaced.
func (r *ParserRegistry) Register(parser LanguageParser) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, ext := range parser.SupportedExtensions() {
		// Normalize extension to lowercase with leading dot
		normalizedExt := normalizeExtension(ext)
		r.parsers[normalizedExt] = parser
	}
}

// Unregister removes a parser from the registry for all its supported extensions.
func (r *ParserRegistry) Unregister(parser LanguageParser) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, ext := range parser.SupportedExtensions() {
		normalizedExt := normalizeExtension(ext)
		if existing, ok := r.parsers[normalizedExt]; ok {
			// Only remove if it's the same parser (by language)
			if existing.Language() == parser.Language() {
				delete(r.parsers, normalizedExt)
			}
		}
	}
}

// GetParserForFile returns the appropriate parser for a given file path.
// Returns nil if no parser is registered for the file's extension.
func (r *ParserRegistry) GetParserForFile(filePath string) LanguageParser {
	ext := filepath.Ext(filePath)
	return r.GetParserByExtension(ext)
}

// GetParserByExtension returns the parser registered for a specific extension.
// The extension can be with or without the leading dot.
// Returns nil if no parser is registered for the extension.
func (r *ParserRegistry) GetParserByExtension(ext string) LanguageParser {
	r.mu.RLock()
	defer r.mu.RUnlock()

	normalizedExt := normalizeExtension(ext)
	return r.parsers[normalizedExt]
}

// CanParse returns true if the registry has a parser for the given file.
func (r *ParserRegistry) CanParse(filePath string) bool {
	return r.GetParserForFile(filePath) != nil
}

// SupportedExtensions returns all file extensions that have registered parsers.
func (r *ParserRegistry) SupportedExtensions() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	extensions := make([]string, 0, len(r.parsers))
	for ext := range r.parsers {
		extensions = append(extensions, ext)
	}
	return extensions
}

// RegisteredLanguages returns the list of unique languages with registered parsers.
func (r *ParserRegistry) RegisteredLanguages() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	seen := make(map[string]bool)
	languages := make([]string, 0)

	for _, parser := range r.parsers {
		lang := parser.Language()
		if !seen[lang] {
			seen[lang] = true
			languages = append(languages, lang)
		}
	}
	return languages
}

// ParseFile parses a file using the appropriate registered parser.
// Returns an error if no parser is available for the file type.
func (r *ParserRegistry) ParseFile(filePath string) (*ParseResult, error) {
	parser := r.GetParserForFile(filePath)
	if parser == nil {
		return nil, &UnsupportedFileError{FilePath: filePath, Extension: filepath.Ext(filePath)}
	}
	return parser.ParseFile(filePath)
}

// UnsupportedFileError is returned when attempting to parse a file with no registered parser.
type UnsupportedFileError struct {
	FilePath  string
	Extension string
}

func (e *UnsupportedFileError) Error() string {
	return "no parser registered for extension: " + e.Extension + " (file: " + e.FilePath + ")"
}

// normalizeExtension ensures the extension is lowercase with a leading dot.
func normalizeExtension(ext string) string {
	ext = strings.ToLower(ext)
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	return ext
}
