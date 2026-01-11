// Package parser provides Rust source code parsing for symbol extraction.
// This implementation uses regex-based pattern matching for CGO-free operation.
package parser

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// RustParser extracts symbols from Rust source files.
// It uses regex-based extraction for CGO-free operation.
type RustParser struct {
	basePath string
}

// NewRustParser creates a new Rust parser instance.
func NewRustParser(basePath string) *RustParser {
	return &RustParser{
		basePath: basePath,
	}
}

// SupportedExtensions returns the file extensions this parser can handle.
func (p *RustParser) SupportedExtensions() []string {
	return []string{".rs"}
}

// Language returns the language identifier for this parser.
func (p *RustParser) Language() string {
	return "rust"
}

// CanParse returns true if this parser can handle the given file path.
func (p *RustParser) CanParse(filePath string) bool {
	ext := strings.ToLower(filepath.Ext(filePath))
	return ext == ".rs"
}

// Rust regex patterns for symbol extraction
var (
	// Derive attribute extraction: #[derive(Trait1, Trait2)]
	rsDerivePattern = regexp.MustCompile(`derive\s*\(([^)]+)\)`)

	// Function definitions: fn name(...) or pub fn name(...) or async fn name(...)
	// Note: Using [ \t]* instead of \s* to avoid capturing newlines in the indent group
	rsFuncPattern = regexp.MustCompile(`(?m)^([ \t]*)(?:pub(?:\s*\([^)]*\))?\s+)?(?:async\s+)?(?:unsafe\s+)?(?:extern\s+"[^"]*"\s+)?fn\s+(\w+)\s*(?:<[^>]*>)?\s*\(([^)]*)\)(?:\s*->\s*([^{;]+))?`)

	// Struct definitions: struct Name or pub struct Name
	rsStructPattern = regexp.MustCompile(`(?m)^(\s*)(?:pub(?:\s*\([^)]*\))?\s+)?struct\s+(\w+)(?:\s*<[^>]*>)?`)

	// Enum definitions: enum Name or pub enum Name
	rsEnumPattern = regexp.MustCompile(`(?m)^(\s*)(?:pub(?:\s*\([^)]*\))?\s+)?enum\s+(\w+)(?:\s*<[^>]*>)?`)

	// Trait definitions: trait Name or pub trait Name
	rsTraitPattern = regexp.MustCompile(`(?m)^(\s*)(?:pub(?:\s*\([^)]*\))?\s+)?(?:unsafe\s+)?trait\s+(\w+)(?:\s*<[^>]*>)?(?:\s*:\s*([^{]+))?`)

	// Type alias: type Name = ... or pub type Name = ...
	rsTypeAliasPattern = regexp.MustCompile(`(?m)^(\s*)(?:pub(?:\s*\([^)]*\))?\s+)?type\s+(\w+)(?:\s*<[^>]*>)?\s*=`)

	// Impl blocks: impl Name or impl Trait for Name
	// Note: Using [ \t]* instead of \s* to avoid capturing newlines in the indent group
	rsImplPattern = regexp.MustCompile(`(?m)^([ \t]*)impl(?:\s*<[^>]*>)?\s+(?:(\w+(?:<[^>]*>)?)\s+for\s+)?(\w+)(?:<[^>]*>)?`)

	// Const definitions: const NAME: Type = value
	rsConstPattern = regexp.MustCompile(`(?m)^(\s*)(?:pub(?:\s*\([^)]*\))?\s+)?const\s+(\w+)\s*:\s*([^=]+)\s*=`)

	// Static definitions: static NAME: Type = value
	rsStaticPattern = regexp.MustCompile(`(?m)^(\s*)(?:pub(?:\s*\([^)]*\))?\s+)?static\s+(?:mut\s+)?(\w+)\s*:\s*([^=]+)\s*=`)

	// Module definitions: mod name or pub mod name
	rsModPattern = regexp.MustCompile(`(?m)^(\s*)(?:pub(?:\s*\([^)]*\))?\s+)?mod\s+(\w+)`)

	// Macro definitions: macro_rules! name { ... } or pub macro name { ... }
	rsMacroRulesPattern = regexp.MustCompile(`(?m)^(\s*)(?:#\[macro_export\]\s*)?macro_rules!\s+(\w+)`)

	// Procedural macro attributes for functions: #[proc_macro], #[proc_macro_derive], #[proc_macro_attribute]
	rsProcMacroPattern = regexp.MustCompile(`(?m)^(\s*)#\[(proc_macro(?:_derive|_attribute)?(?:\([^)]*\))?)\]\s*\n\s*(?:pub(?:\s*\([^)]*\))?\s+)?fn\s+(\w+)`)

	// Block doc comment pattern (/** */ style)
	rsBlockDocPattern = regexp.MustCompile(`(?s)/\*[*!](.+?)\*/`)
)

// ParseFile parses a Rust file and extracts symbols.
func (p *RustParser) ParseFile(filePath string) (*ParseResult, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading file: %w", err)
	}

	result := &ParseResult{
		Symbols:   make([]Symbol, 0),
		Relations: make([]SymbolRelation, 0),
	}

	fileHash := ComputeHash(content)
	relPath := p.relativePath(filePath)
	modulePath := extractRsModulePath(relPath)
	now := time.Now()

	// Build line number map for position tracking
	lineStarts := buildRsLineStartMap(content)

	// Extract doc comments for documentation lookup
	docComments := extractRsDocComments(content, lineStarts)

	// Extract symbols
	p.extractFunctions(content, relPath, fileHash, modulePath, now, lineStarts, docComments, result)
	p.extractStructs(content, relPath, fileHash, modulePath, now, lineStarts, docComments, result)
	p.extractEnums(content, relPath, fileHash, modulePath, now, lineStarts, docComments, result)
	p.extractTraits(content, relPath, fileHash, modulePath, now, lineStarts, docComments, result)
	p.extractTypeAliases(content, relPath, fileHash, modulePath, now, lineStarts, docComments, result)
	p.extractImpls(content, relPath, fileHash, modulePath, now, lineStarts, docComments, result)
	p.extractConstants(content, relPath, fileHash, modulePath, now, lineStarts, docComments, result)
	p.extractModules(content, relPath, fileHash, modulePath, now, lineStarts, docComments, result)
	p.extractMacros(content, relPath, fileHash, modulePath, now, lineStarts, docComments, result)

	return result, nil
}

// ParseDirectory parses all Rust files in a directory recursively.
func (p *RustParser) ParseDirectory(dirPath string) (*ParseResult, error) {
	combined := &ParseResult{
		Symbols:   make([]Symbol, 0),
		Relations: make([]SymbolRelation, 0),
	}

	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip hidden directories and common non-source dirs
		if info.IsDir() {
			name := info.Name()
			if strings.HasPrefix(name, ".") || name == "target" || name == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}

		if !p.CanParse(path) {
			return nil
		}

		result, err := p.ParseFile(path)
		if err != nil {
			combined.Errors = append(combined.Errors, fmt.Errorf("%s: %w", path, err))
			return nil
		}

		combined.Symbols = append(combined.Symbols, result.Symbols...)
		combined.Relations = append(combined.Relations, result.Relations...)
		return nil
	})

	if err != nil {
		return nil, err
	}

	return combined, nil
}

// extractFunctions extracts top-level function definitions.
// Methods inside impl blocks are handled by extractImplMethods.
func (p *RustParser) extractFunctions(content []byte, filePath, fileHash, modulePath string, now time.Time, lineStarts []int, docComments map[int]string, result *ParseResult) {
	matches := rsFuncPattern.FindAllSubmatchIndex(content, -1)

	for _, match := range matches {
		if len(match) < 6 {
			continue
		}

		indent := string(content[match[2]:match[3]])
		// Skip indented functions - they're methods inside impl blocks
		// and will be extracted by extractImplMethods with proper type-qualified signatures
		if len(indent) > 0 {
			continue
		}

		name := string(content[match[4]:match[5]])
		line := findLineNumber(match[0], lineStarts)

		var params string
		if match[6] != -1 && match[7] != -1 {
			params = string(content[match[6]:match[7]])
		}

		var returnType string
		if len(match) >= 10 && match[8] != -1 && match[9] != -1 {
			returnType = strings.TrimSpace(string(content[match[8]:match[9]]))
		}

		sig := fmt.Sprintf("fn %s(%s)", name, cleanRsParams(params))
		if returnType != "" {
			sig += " -> " + returnType
		}

		declaration := string(content[match[0]:match[1]])

		// Extract attributes (for #[test], #[inline], #[cfg], etc.)
		attrs, derives := extractRsAttributes(content, line)
		attrDoc := formatAttributesForDoc(attrs, derives)
		docComment := combineDocWithAttrs(findRsDocComment(line, docComments), attrDoc)

		sym := Symbol{
			Name:         name,
			Kind:         SymbolFunction,
			FilePath:     filePath,
			StartLine:    line,
			EndLine:      findRsBlockEnd(content, match[1], lineStarts),
			Signature:    sig,
			DocComment:   docComment,
			ModulePath:   modulePath,
			Visibility:   rsVisibility(declaration),
			Language:     "rust",
			FileHash:     fileHash,
			LastModified: now,
		}

		result.Symbols = append(result.Symbols, sym)
	}
}

// extractStructs extracts struct definitions.
func (p *RustParser) extractStructs(content []byte, filePath, fileHash, modulePath string, now time.Time, lineStarts []int, docComments map[int]string, result *ParseResult) {
	matches := rsStructPattern.FindAllSubmatchIndex(content, -1)

	for _, match := range matches {
		if len(match) < 6 {
			continue
		}

		indent := string(content[match[2]:match[3]])
		// Skip nested structs
		if len(indent) > 0 {
			continue
		}

		name := string(content[match[4]:match[5]])
		line := findLineNumber(match[0], lineStarts)

		declaration := string(content[match[0]:match[1]])

		// Extract attributes
		attrs, derives := extractRsAttributes(content, line)
		attrDoc := formatAttributesForDoc(attrs, derives)
		docComment := combineDocWithAttrs(findRsDocComment(line, docComments), attrDoc)

		sym := Symbol{
			Name:         name,
			Kind:         SymbolStruct,
			FilePath:     filePath,
			StartLine:    line,
			EndLine:      findRsBlockEnd(content, match[1], lineStarts),
			Signature:    fmt.Sprintf("struct %s", name),
			DocComment:   docComment,
			ModulePath:   modulePath,
			Visibility:   rsVisibility(declaration),
			Language:     "rust",
			FileHash:     fileHash,
			LastModified: now,
		}

		result.Symbols = append(result.Symbols, sym)

		// Extract struct fields
		p.extractStructFields(content, match[1], name, filePath, fileHash, modulePath, now, lineStarts, docComments, result)
	}
}

// extractStructFields extracts fields from a struct definition.
func (p *RustParser) extractStructFields(content []byte, structStart int, structName, filePath, fileHash, modulePath string, now time.Time, lineStarts []int, docComments map[int]string, result *ParseResult) {
	// Find the struct body
	braceStart := bytes.IndexByte(content[structStart:], '{')
	if braceStart == -1 {
		return // Tuple struct or unit struct
	}
	braceStart += structStart

	braceEnd := findRsMatchingBrace(content, braceStart)
	if braceEnd == -1 {
		return
	}

	// Simple field extraction pattern
	fieldPattern := regexp.MustCompile(`(?m)^\s*(?:pub(?:\s*\([^)]*\))?\s+)?(\w+)\s*:\s*([^,}]+)`)
	structBody := content[braceStart+1 : braceEnd]
	bodyOffset := braceStart + 1

	fieldMatches := fieldPattern.FindAllSubmatchIndex(structBody, -1)
	for _, match := range fieldMatches {
		if len(match) < 6 {
			continue
		}

		fieldName := string(structBody[match[2]:match[3]])
		fieldType := strings.TrimSpace(string(structBody[match[4]:match[5]]))
		line := findLineNumber(bodyOffset+match[0], lineStarts)

		sym := Symbol{
			Name:         fieldName,
			Kind:         SymbolField,
			FilePath:     filePath,
			StartLine:    line,
			EndLine:      line,
			Signature:    fmt.Sprintf("%s.%s: %s", structName, fieldName, fieldType),
			DocComment:   findRsDocComment(line, docComments),
			ModulePath:   modulePath,
			Visibility:   rsFieldVisibility(string(structBody[match[0]:match[1]])),
			Language:     "rust",
			FileHash:     fileHash,
			LastModified: now,
		}

		result.Symbols = append(result.Symbols, sym)
	}
}

// extractEnums extracts enum definitions.
func (p *RustParser) extractEnums(content []byte, filePath, fileHash, modulePath string, now time.Time, lineStarts []int, docComments map[int]string, result *ParseResult) {
	matches := rsEnumPattern.FindAllSubmatchIndex(content, -1)

	for _, match := range matches {
		if len(match) < 6 {
			continue
		}

		indent := string(content[match[2]:match[3]])
		if len(indent) > 0 {
			continue
		}

		name := string(content[match[4]:match[5]])
		line := findLineNumber(match[0], lineStarts)

		declaration := string(content[match[0]:match[1]])

		// Extract attributes
		attrs, derives := extractRsAttributes(content, line)
		attrDoc := formatAttributesForDoc(attrs, derives)
		docComment := combineDocWithAttrs(findRsDocComment(line, docComments), attrDoc)

		sym := Symbol{
			Name:         name,
			Kind:         SymbolType, // Enums are types in Rust
			FilePath:     filePath,
			StartLine:    line,
			EndLine:      findRsBlockEnd(content, match[1], lineStarts),
			Signature:    fmt.Sprintf("enum %s", name),
			DocComment:   docComment,
			ModulePath:   modulePath,
			Visibility:   rsVisibility(declaration),
			Language:     "rust",
			FileHash:     fileHash,
			LastModified: now,
		}

		result.Symbols = append(result.Symbols, sym)
	}
}

// extractTraits extracts trait definitions.
func (p *RustParser) extractTraits(content []byte, filePath, fileHash, modulePath string, now time.Time, lineStarts []int, docComments map[int]string, result *ParseResult) {
	matches := rsTraitPattern.FindAllSubmatchIndex(content, -1)

	for _, match := range matches {
		if len(match) < 6 {
			continue
		}

		indent := string(content[match[2]:match[3]])
		if len(indent) > 0 {
			continue
		}

		name := string(content[match[4]:match[5]])
		line := findLineNumber(match[0], lineStarts)

		var sig strings.Builder
		sig.WriteString("trait ")
		sig.WriteString(name)

		if len(match) >= 8 && match[6] != -1 && match[7] != -1 {
			sig.WriteString(": ")
			sig.WriteString(strings.TrimSpace(string(content[match[6]:match[7]])))
		}

		declaration := string(content[match[0]:match[1]])

		sym := Symbol{
			Name:         name,
			Kind:         SymbolInterface, // Traits are like interfaces
			FilePath:     filePath,
			StartLine:    line,
			EndLine:      findRsBlockEnd(content, match[1], lineStarts),
			Signature:    sig.String(),
			DocComment:   findRsDocComment(line, docComments),
			ModulePath:   modulePath,
			Visibility:   rsVisibility(declaration),
			Language:     "rust",
			FileHash:     fileHash,
			LastModified: now,
		}

		result.Symbols = append(result.Symbols, sym)
	}
}

// extractTypeAliases extracts type alias definitions.
func (p *RustParser) extractTypeAliases(content []byte, filePath, fileHash, modulePath string, now time.Time, lineStarts []int, docComments map[int]string, result *ParseResult) {
	matches := rsTypeAliasPattern.FindAllSubmatchIndex(content, -1)

	for _, match := range matches {
		if len(match) < 6 {
			continue
		}

		indent := string(content[match[2]:match[3]])
		if len(indent) > 0 {
			continue
		}

		name := string(content[match[4]:match[5]])
		line := findLineNumber(match[0], lineStarts)

		declaration := string(content[match[0]:match[1]])

		sym := Symbol{
			Name:         name,
			Kind:         SymbolType,
			FilePath:     filePath,
			StartLine:    line,
			EndLine:      line,
			Signature:    fmt.Sprintf("type %s", name),
			DocComment:   findRsDocComment(line, docComments),
			ModulePath:   modulePath,
			Visibility:   rsVisibility(declaration),
			Language:     "rust",
			FileHash:     fileHash,
			LastModified: now,
		}

		result.Symbols = append(result.Symbols, sym)
	}
}

// extractImpls extracts impl blocks and their methods.
func (p *RustParser) extractImpls(content []byte, filePath, fileHash, modulePath string, now time.Time, lineStarts []int, docComments map[int]string, result *ParseResult) {
	matches := rsImplPattern.FindAllSubmatchIndex(content, -1)

	for _, match := range matches {
		if len(match) < 8 {
			continue
		}

		indent := string(content[match[2]:match[3]])
		if len(indent) > 0 {
			continue
		}

		var traitName string
		if match[4] != -1 && match[5] != -1 {
			traitName = string(content[match[4]:match[5]])
		}

		typeName := string(content[match[6]:match[7]])
		line := findLineNumber(match[0], lineStarts)

		// Find the impl block end
		endLine := findRsBlockEnd(content, match[1], lineStarts)

		// Extract methods from the impl block
		p.extractImplMethods(content, match[1], typeName, filePath, fileHash, modulePath, now, lineStarts, docComments, result)

		if traitName != "" {
			// Create implements relation for trait impl: impl Trait for Type
			rel := SymbolRelation{
				RelationType: RelationImplements,
				Metadata: map[string]any{
					"implementor": typeName,
					"trait":       traitName,
					"startLine":   line,
					"endLine":     endLine,
					"implType":    "trait",
				},
			}
			result.Relations = append(result.Relations, rel)
		} else {
			// Create association relation for inherent impl: impl Type
			rel := SymbolRelation{
				RelationType: RelationCalls, // Using Calls as a generic association
				Metadata: map[string]any{
					"target":    typeName,
					"startLine": line,
					"endLine":   endLine,
					"implType":  "inherent",
				},
			}
			result.Relations = append(result.Relations, rel)
		}
	}
}

// extractImplMethods extracts methods from an impl block.
func (p *RustParser) extractImplMethods(content []byte, implStart int, typeName, filePath, fileHash, modulePath string, now time.Time, lineStarts []int, docComments map[int]string, result *ParseResult) {
	braceStart := bytes.IndexByte(content[implStart:], '{')
	if braceStart == -1 {
		return
	}
	braceStart += implStart

	braceEnd := findRsMatchingBrace(content, braceStart)
	if braceEnd == -1 {
		return
	}

	implBody := content[braceStart+1 : braceEnd]
	bodyOffset := braceStart + 1

	// Find function definitions within the impl block
	funcMatches := rsFuncPattern.FindAllSubmatchIndex(implBody, -1)

	for _, match := range funcMatches {
		if len(match) < 6 {
			continue
		}

		name := string(implBody[match[4]:match[5]])
		line := findLineNumber(bodyOffset+match[0], lineStarts)

		var params string
		if match[6] != -1 && match[7] != -1 {
			params = string(implBody[match[6]:match[7]])
		}

		var returnType string
		if len(match) >= 10 && match[8] != -1 && match[9] != -1 {
			returnType = strings.TrimSpace(string(implBody[match[8]:match[9]]))
		}

		sig := fmt.Sprintf("%s::%s(%s)", typeName, name, cleanRsParams(params))
		if returnType != "" {
			sig += " -> " + returnType
		}

		declaration := string(implBody[match[0]:match[1]])

		// Extract attributes for the method
		attrs, derives := extractRsAttributes(content, line)
		attrDoc := formatAttributesForDoc(attrs, derives)
		docComment := combineDocWithAttrs(findRsDocComment(line, docComments), attrDoc)

		sym := Symbol{
			Name:         name,
			Kind:         SymbolMethod,
			FilePath:     filePath,
			StartLine:    line,
			EndLine:      findRsBlockEnd(content, bodyOffset+match[1], lineStarts),
			Signature:    sig,
			DocComment:   docComment,
			ModulePath:   modulePath,
			Visibility:   rsVisibility(declaration),
			Language:     "rust",
			FileHash:     fileHash,
			LastModified: now,
		}

		result.Symbols = append(result.Symbols, sym)
	}
}

// extractConstants extracts const and static definitions.
func (p *RustParser) extractConstants(content []byte, filePath, fileHash, modulePath string, now time.Time, lineStarts []int, docComments map[int]string, result *ParseResult) {
	// Extract const
	constMatches := rsConstPattern.FindAllSubmatchIndex(content, -1)
	for _, match := range constMatches {
		if len(match) < 8 {
			continue
		}

		indent := string(content[match[2]:match[3]])
		if len(indent) > 0 {
			continue
		}

		name := string(content[match[4]:match[5]])
		constType := strings.TrimSpace(string(content[match[6]:match[7]]))
		line := findLineNumber(match[0], lineStarts)

		declaration := string(content[match[0]:match[1]])

		sym := Symbol{
			Name:         name,
			Kind:         SymbolConstant,
			FilePath:     filePath,
			StartLine:    line,
			EndLine:      line,
			Signature:    fmt.Sprintf("const %s: %s", name, constType),
			DocComment:   findRsDocComment(line, docComments),
			ModulePath:   modulePath,
			Visibility:   rsVisibility(declaration),
			Language:     "rust",
			FileHash:     fileHash,
			LastModified: now,
		}

		result.Symbols = append(result.Symbols, sym)
	}

	// Extract static
	staticMatches := rsStaticPattern.FindAllSubmatchIndex(content, -1)
	for _, match := range staticMatches {
		if len(match) < 8 {
			continue
		}

		indent := string(content[match[2]:match[3]])
		if len(indent) > 0 {
			continue
		}

		name := string(content[match[4]:match[5]])
		staticType := strings.TrimSpace(string(content[match[6]:match[7]]))
		line := findLineNumber(match[0], lineStarts)

		declaration := string(content[match[0]:match[1]])

		sym := Symbol{
			Name:         name,
			Kind:         SymbolVariable,
			FilePath:     filePath,
			StartLine:    line,
			EndLine:      line,
			Signature:    fmt.Sprintf("static %s: %s", name, staticType),
			DocComment:   findRsDocComment(line, docComments),
			ModulePath:   modulePath,
			Visibility:   rsVisibility(declaration),
			Language:     "rust",
			FileHash:     fileHash,
			LastModified: now,
		}

		result.Symbols = append(result.Symbols, sym)
	}
}

// extractModules extracts module definitions.
func (p *RustParser) extractModules(content []byte, filePath, fileHash, modulePath string, now time.Time, lineStarts []int, docComments map[int]string, result *ParseResult) {
	matches := rsModPattern.FindAllSubmatchIndex(content, -1)

	for _, match := range matches {
		if len(match) < 6 {
			continue
		}

		indent := string(content[match[2]:match[3]])
		if len(indent) > 0 {
			continue
		}

		name := string(content[match[4]:match[5]])
		line := findLineNumber(match[0], lineStarts)

		declaration := string(content[match[0]:match[1]])

		sym := Symbol{
			Name:         name,
			Kind:         SymbolPackage,
			FilePath:     filePath,
			StartLine:    line,
			EndLine:      line,
			Signature:    fmt.Sprintf("mod %s", name),
			DocComment:   findRsDocComment(line, docComments),
			ModulePath:   modulePath,
			Visibility:   rsVisibility(declaration),
			Language:     "rust",
			FileHash:     fileHash,
			LastModified: now,
		}

		result.Symbols = append(result.Symbols, sym)
	}
}

// extractMacros extracts macro definitions from Rust source files.
// Handles both declarative macros (macro_rules!) and procedural macros (#[proc_macro*]).
func (p *RustParser) extractMacros(content []byte, filePath, fileHash, modulePath string, now time.Time, lineStarts []int, docComments map[int]string, result *ParseResult) {
	// Extract macro_rules! definitions
	matches := rsMacroRulesPattern.FindAllSubmatchIndex(content, -1)

	for _, match := range matches {
		if len(match) < 6 {
			continue
		}

		name := string(content[match[4]:match[5]])
		line := findLineNumber(match[0], lineStarts)

		// Check if it's exported
		declarationStart := match[0]
		visibility := "private"
		if declarationStart > 0 {
			// Look backwards for #[macro_export]
			searchStart := declarationStart - 1
			for searchStart >= 0 && (content[searchStart] == ' ' || content[searchStart] == '\t' || content[searchStart] == '\n') {
				searchStart--
			}
			if searchStart >= 14 {
				preceding := string(content[searchStart-14 : searchStart+1])
				if strings.Contains(preceding, "macro_export") {
					visibility = "public"
				}
			}
		}

		// Find the end of the macro (matching braces)
		macroEnd := findRsMacroEnd(content, match[1], lineStarts)

		// Add [Macro] marker to docComment for identification
		docComment := findRsDocComment(line, docComments)
		if docComment != "" {
			docComment = "[Macro] " + docComment
		} else {
			docComment = "[Macro]"
		}

		sym := Symbol{
			Name:         name,
			Kind:         SymbolFunction, // Using function kind for macros
			FilePath:     filePath,
			StartLine:    line,
			EndLine:      macroEnd,
			Signature:    fmt.Sprintf("macro_rules! %s", name),
			DocComment:   docComment,
			ModulePath:   modulePath,
			Visibility:   visibility,
			Language:     "rust",
			FileHash:     fileHash,
			LastModified: now,
		}

		result.Symbols = append(result.Symbols, sym)
	}

	// Extract procedural macros (functions with #[proc_macro*] attributes)
	procMatches := rsProcMacroPattern.FindAllSubmatchIndex(content, -1)

	for _, match := range procMatches {
		if len(match) < 8 {
			continue
		}

		macroAttr := string(content[match[4]:match[5]])
		name := string(content[match[6]:match[7]])
		line := findLineNumber(match[0], lineStarts)

		// Determine macro type from attribute
		macroType := "Proc Macro"
		if strings.Contains(macroAttr, "derive") {
			macroType = "Derive Macro"
		} else if strings.Contains(macroAttr, "attribute") {
			macroType = "Attribute Macro"
		}

		// Add macro type marker to docComment
		docComment := findRsDocComment(line, docComments)
		if docComment != "" {
			docComment = fmt.Sprintf("[%s] %s", macroType, docComment)
		} else {
			docComment = fmt.Sprintf("[%s]", macroType)
		}

		sym := Symbol{
			Name:         name,
			Kind:         SymbolFunction,
			FilePath:     filePath,
			StartLine:    line,
			EndLine:      line, // Could be improved to find function end
			Signature:    fmt.Sprintf("#[%s] fn %s", macroAttr, name),
			DocComment:   docComment,
			ModulePath:   modulePath,
			Visibility:   "public", // Proc macros are always pub
			Language:     "rust",
			FileHash:     fileHash,
			LastModified: now,
		}

		result.Symbols = append(result.Symbols, sym)
	}
}

// findRsMacroEnd finds the end line of a macro_rules! definition by matching braces.
func findRsMacroEnd(content []byte, startOffset int, lineStarts []int) int {
	// Find opening brace/paren/bracket
	bracePos := -1
	var openChar, closeChar byte
	for i := startOffset; i < len(content); i++ {
		switch content[i] {
		case '{':
			bracePos = i
			openChar = '{'
			closeChar = '}'
			goto foundBrace
		case '(':
			bracePos = i
			openChar = '('
			closeChar = ')'
			goto foundBrace
		case '[':
			bracePos = i
			openChar = '['
			closeChar = ']'
			goto foundBrace
		}
	}
	return findLineNumber(startOffset, lineStarts)

foundBrace:
	depth := 1
	for i := bracePos + 1; i < len(content); i++ {
		if content[i] == openChar {
			depth++
		} else if content[i] == closeChar {
			depth--
			if depth == 0 {
				return findLineNumber(i, lineStarts)
			}
		}
	}
	return findLineNumber(startOffset, lineStarts)
}

// Helper functions

func (p *RustParser) relativePath(absPath string) string {
	if p.basePath == "" {
		return absPath
	}
	rel, err := filepath.Rel(p.basePath, absPath)
	if err != nil {
		return absPath
	}
	return rel
}

func extractRsModulePath(filePath string) string {
	dir := filepath.Dir(filePath)
	if dir == "." || dir == "" {
		return ""
	}
	// Convert path to Rust module path
	return strings.ReplaceAll(dir, string(filepath.Separator), "::")
}

func buildRsLineStartMap(content []byte) []int {
	lineStarts := []int{0}
	for i, b := range content {
		if b == '\n' {
			lineStarts = append(lineStarts, i+1)
		}
	}
	return lineStarts
}

// extractRsDocComments extracts doc comments and associates them with lines.
func extractRsDocComments(content []byte, lineStarts []int) map[int]string {
	comments := make(map[int]string)

	// Extract /// and //! style comments
	lines := bytes.Split(content, []byte("\n"))
	var docLines []string

	for i, line := range lines {
		trimmed := bytes.TrimSpace(line)

		if bytes.HasPrefix(trimmed, []byte("///")) || bytes.HasPrefix(trimmed, []byte("//!")) {
			text := string(bytes.TrimPrefix(bytes.TrimPrefix(trimmed, []byte("///")), []byte("//!")))
			text = strings.TrimSpace(text)
			docLines = append(docLines, text)
		} else {
			if len(docLines) > 0 {
				// Associate doc comment with the next non-empty line
				comments[i+1] = strings.Join(docLines, " ")
				docLines = nil
			}
		}
	}

	// Also extract /** */ style block comments
	blockMatches := rsBlockDocPattern.FindAllSubmatchIndex(content, -1)
	for _, match := range blockMatches {
		if len(match) < 4 {
			continue
		}

		commentEnd := match[1]
		nextLine := findLineNumber(commentEnd, lineStarts) + 1

		commentText := string(content[match[2]:match[3]])
		commentText = cleanRsBlockComment(commentText)

		comments[nextLine] = commentText
	}

	return comments
}

func cleanRsBlockComment(comment string) string {
	lines := strings.Split(comment, "\n")
	var cleaned []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		line = strings.TrimPrefix(line, "*")
		line = strings.TrimSpace(line)
		if line != "" {
			cleaned = append(cleaned, line)
		}
	}

	return strings.Join(cleaned, " ")
}

func findRsDocComment(line int, docComments map[int]string) string {
	if doc, ok := docComments[line]; ok {
		return doc
	}
	return ""
}

func rsVisibility(declaration string) string {
	if strings.Contains(declaration, "pub") {
		return "public"
	}
	return "private"
}

func rsFieldVisibility(declaration string) string {
	if strings.Contains(declaration, "pub") {
		return "public"
	}
	return "private"
}

func findRsBlockEnd(content []byte, startOffset int, lineStarts []int) int {
	braceStart := bytes.IndexByte(content[startOffset:], '{')
	if braceStart == -1 {
		// Might be a semicolon-terminated declaration
		semiPos := bytes.IndexByte(content[startOffset:], ';')
		if semiPos != -1 {
			return findLineNumber(startOffset+semiPos, lineStarts)
		}
		return findLineNumber(startOffset, lineStarts)
	}

	braceEnd := findRsMatchingBrace(content, startOffset+braceStart)
	if braceEnd == -1 {
		return findLineNumber(startOffset, lineStarts)
	}

	return findLineNumber(braceEnd, lineStarts)
}

func findRsMatchingBrace(content []byte, openBracePos int) int {
	if openBracePos >= len(content) || content[openBracePos] != '{' {
		return -1
	}

	depth := 1
	inString := false
	inLineComment := false
	inBlockComment := false
	escaped := false

	for i := openBracePos + 1; i < len(content); i++ {
		c := content[i]

		// Handle line comments
		if inLineComment {
			if c == '\n' {
				inLineComment = false
			}
			continue
		}

		// Handle block comments
		if inBlockComment {
			if c == '*' && i+1 < len(content) && content[i+1] == '/' {
				inBlockComment = false
				i++ // Skip the '/'
			}
			continue
		}

		// Check for comment start
		if !inString && c == '/' && i+1 < len(content) {
			if content[i+1] == '/' {
				inLineComment = true
				i++
				continue
			}
			if content[i+1] == '*' {
				inBlockComment = true
				i++
				continue
			}
		}

		if escaped {
			escaped = false
			continue
		}

		if c == '\\' && inString {
			escaped = true
			continue
		}

		// Handle character literals - they're like 'c' or '\n' or '\''
		// We need to be careful not to treat apostrophes in comments as char literals
		// Since we now skip comments, we only handle actual char literals here
		if !inString && c == '\'' {
			// Look ahead to see if this is a char literal
			// Char literals are: 'x' or '\x' or '\xx' (max 4 chars for things like '\u{...}')
			if i+2 < len(content) {
				if content[i+1] == '\\' {
					// Escaped char like '\n' or '\''
					// Find the closing quote
					for j := i + 2; j < len(content) && j < i+8; j++ {
						if content[j] == '\'' {
							i = j
							break
						}
					}
				} else if content[i+2] == '\'' {
					// Simple char like 'c'
					i += 2
				}
				// Otherwise it's an apostrophe (like in "user's"), ignore it
			}
			continue
		}

		// Handle string literals
		if !inString && c == '"' {
			inString = true
			continue
		}
		if inString && c == '"' {
			inString = false
			continue
		}

		if inString {
			continue
		}

		// Handle raw strings r#"..."#
		if c == 'r' && i+1 < len(content) && content[i+1] == '#' {
			// Skip raw string
			hashCount := 0
			for j := i + 1; j < len(content) && content[j] == '#'; j++ {
				hashCount++
			}
			if i+hashCount+1 < len(content) && content[i+hashCount+1] == '"' {
				// Find closing
				closePattern := make([]byte, hashCount+1)
				closePattern[0] = '"'
				for j := 1; j <= hashCount; j++ {
					closePattern[j] = '#'
				}
				closeIdx := bytes.Index(content[i+hashCount+2:], closePattern)
				if closeIdx != -1 {
					i = i + hashCount + 2 + closeIdx + len(closePattern) - 1
					continue
				}
			}
		}

		if c == '{' {
			depth++
		} else if c == '}' {
			depth--
			if depth == 0 {
				return i
			}
		}
	}

	return -1
}

func cleanRsParams(params string) string {
	// Remove 'self' variants from parameters
	parts := strings.Split(params, ",")
	var cleaned []string

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "self" || part == "&self" || part == "&mut self" ||
			part == "mut self" || strings.HasPrefix(part, "self:") {
			continue
		}
		if part != "" {
			cleaned = append(cleaned, part)
		}
	}

	return strings.Join(cleaned, ", ")
}

// extractRsAttributes extracts attributes immediately preceding a given line.
// Returns a slice of attribute strings and derive traits.
func extractRsAttributes(content []byte, symbolLine int) (attrs []string, derives []string) {
	lines := bytes.Split(content, []byte("\n"))

	// Look backwards from the symbol line to find attributes
	for i := symbolLine - 2; i >= 0; i-- {
		if i >= len(lines) {
			continue
		}
		line := bytes.TrimSpace(lines[i])

		// Stop if we hit empty line or non-attribute content
		if len(line) == 0 {
			break
		}

		// Check if it's an attribute line
		if !bytes.HasPrefix(line, []byte("#[")) {
			// Allow doc comments to be skipped
			if bytes.HasPrefix(line, []byte("///")) || bytes.HasPrefix(line, []byte("//!")) {
				continue
			}
			break
		}

		attrContent := string(line)
		attrContent = strings.TrimPrefix(attrContent, "#[")
		attrContent = strings.TrimSuffix(attrContent, "]")
		attrs = append([]string{attrContent}, attrs...) // Prepend to maintain order

		// Extract derive traits
		if deriveMatch := rsDerivePattern.FindStringSubmatch(attrContent); len(deriveMatch) > 1 {
			traitList := strings.Split(deriveMatch[1], ",")
			for _, trait := range traitList {
				trait = strings.TrimSpace(trait)
				if trait != "" {
					derives = append(derives, trait)
				}
			}
		}
	}

	return attrs, derives
}

// formatAttributesForDoc formats attributes for inclusion in DocComment.
func formatAttributesForDoc(attrs []string, derives []string) string {
	if len(attrs) == 0 && len(derives) == 0 {
		return ""
	}

	var parts []string

	if len(derives) > 0 {
		parts = append(parts, "[Derives: "+strings.Join(derives, ", ")+"]")
	}

	// Include non-derive attributes
	var otherAttrs []string
	for _, attr := range attrs {
		if !strings.HasPrefix(attr, "derive(") {
			otherAttrs = append(otherAttrs, attr)
		}
	}

	if len(otherAttrs) > 0 {
		parts = append(parts, "[Attrs: "+strings.Join(otherAttrs, ", ")+"]")
	}

	return strings.Join(parts, " ")
}

// combineDocWithAttrs combines doc comment with attributes.
func combineDocWithAttrs(doc, attrDoc string) string {
	if attrDoc == "" {
		return doc
	}
	if doc == "" {
		return attrDoc
	}
	return attrDoc + " " + doc
}

// Ensure RustParser implements LanguageParser at compile time.
var _ LanguageParser = (*RustParser)(nil)
