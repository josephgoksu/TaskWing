// Package parser provides TypeScript/JavaScript source code parsing for symbol extraction.
// This implementation uses regex-based pattern matching for CGO-free operation.
package parser

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// TypeScriptParser extracts symbols from TypeScript and JavaScript files.
// It uses regex-based extraction for CGO-free operation.
type TypeScriptParser struct {
	basePath string
}

// NewTypeScriptParser creates a new TypeScript/JavaScript parser instance.
func NewTypeScriptParser(basePath string) *TypeScriptParser {
	return &TypeScriptParser{
		basePath: basePath,
	}
}

// SupportedExtensions returns the file extensions this parser can handle.
func (p *TypeScriptParser) SupportedExtensions() []string {
	return []string{".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs"}
}

// Language returns the language identifier for this parser.
func (p *TypeScriptParser) Language() string {
	return "typescript"
}

// CanParse returns true if this parser can handle the given file path.
func (p *TypeScriptParser) CanParse(filePath string) bool {
	ext := strings.ToLower(filepath.Ext(filePath))
	for _, supported := range p.SupportedExtensions() {
		if ext == supported {
			return true
		}
	}
	return false
}

// TypeScript regex patterns for symbol extraction
var (
	// Function declarations: function name(...) or async function name(...)
	tsFuncDeclPattern = regexp.MustCompile(`(?m)^\s*(?:export\s+)?(?:async\s+)?function\s+(\w+)\s*(<[^>]*>)?\s*\(([^)]*)\)(?:\s*:\s*([^{]+))?`)

	// Arrow functions with type annotations: const name = (...) => or const name: Type = (...) =>
	tsArrowFuncPattern = regexp.MustCompile(`(?m)^\s*(?:export\s+)?(?:const|let|var)\s+(\w+)\s*(?::\s*[^=]+)?\s*=\s*(?:async\s+)?(?:<[^>]*>\s*)?\(([^)]*)\)\s*(?::\s*([^=]+))?\s*=>`)

	// Class declarations: class Name or export class Name
	tsClassPattern = regexp.MustCompile(`(?m)^\s*(?:export\s+)?(?:abstract\s+)?class\s+(\w+)(?:\s*<[^>]*>)?(?:\s+extends\s+(\w+))?(?:\s+implements\s+([^{]+))?`)

	// Interface declarations: interface Name or export interface Name
	tsInterfacePattern = regexp.MustCompile(`(?m)^\s*(?:export\s+)?interface\s+(\w+)(?:\s*<[^>]*>)?(?:\s+extends\s+([^{]+))?`)

	// Type alias declarations: type Name = ...
	tsTypeAliasPattern = regexp.MustCompile(`(?m)^\s*(?:export\s+)?type\s+(\w+)(?:\s*<[^>]*>)?\s*=`)

	// Enum declarations: enum Name or export enum Name
	tsEnumPattern = regexp.MustCompile(`(?m)^\s*(?:export\s+)?(?:const\s+)?enum\s+(\w+)`)

	// Method declarations inside classes: name(...) or async name(...)
	tsMethodPattern = regexp.MustCompile(`(?m)^\s*(?:public|private|protected)?\s*(?:static\s+)?(?:async\s+)?(\w+)\s*(<[^>]*>)?\s*\(([^)]*)\)(?:\s*:\s*([^{]+))?`)

	// Property declarations in classes: name: type or name = value
	tsPropertyPattern = regexp.MustCompile(`(?m)^\s*(?:readonly\s+)?(?:public|private|protected)?\s*(?:static\s+)?(?:readonly\s+)?(\w+)\s*(?:\?\s*)?:\s*([^;=]+)`)

	// Const/let/var declarations: export const name = or const name: type =
	tsConstPattern = regexp.MustCompile(`(?m)^\s*(?:export\s+)?(?:const|let|var)\s+(\w+)\s*(?::\s*([^=]+))?\s*=`)

	// JSDoc comment pattern
	tsJSDocPattern = regexp.MustCompile(`(?s)/\*\*(.+?)\*/`)

	// Decorator pattern: @DecoratorName or @DecoratorName(...) - captures decorator name and args
	tsDecoratorPattern = regexp.MustCompile(`(?m)^\s*@(\w+)(?:\s*\(([^)]*)\))?`)

	// React hook patterns - both built-in and custom hooks (use* convention)
	// Matches: const [x, setX] = useState(...) OR const x = useMemo(...) OR useEffect(...)
	tsReactHookCallWithVarPattern = regexp.MustCompile(`(?m)^\s*(?:const|let)\s+(\[[^\]]+\]|\w+)\s*=\s*(use\w+)\s*[(<]`)
	// Matches standalone hook calls: useEffect(...), useLayoutEffect(...)
	tsReactHookCallStandalonePattern = regexp.MustCompile(`(?m)^\s*(useEffect|useLayoutEffect|useContext)\s*\(`)

	// Custom hook definitions: function useXxx or const useXxx =
	tsCustomHookDefPattern = regexp.MustCompile(`(?m)^\s*(?:export\s+)?(?:function\s+(use[A-Z]\w*)|(?:const|let)\s+(use[A-Z]\w*)\s*=)`)
)

// ParseFile parses a TypeScript/JavaScript file and extracts symbols.
func (p *TypeScriptParser) ParseFile(filePath string) (*ParseResult, error) {
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
	modulePath := extractTSModulePath(relPath)
	now := time.Now()

	// Build line number map for position tracking
	lineStarts := buildLineStartMap(content)

	// Extract JSDoc comments for doc lookup
	docComments := extractJSDocComments(content, lineStarts)

	// Extract symbols
	p.extractFunctions(content, relPath, fileHash, modulePath, now, lineStarts, docComments, result)
	p.extractArrowFunctions(content, relPath, fileHash, modulePath, now, lineStarts, docComments, result)
	p.extractClasses(content, relPath, fileHash, modulePath, now, lineStarts, docComments, result)
	p.extractInterfaces(content, relPath, fileHash, modulePath, now, lineStarts, docComments, result)
	p.extractTypeAliases(content, relPath, fileHash, modulePath, now, lineStarts, docComments, result)
	p.extractEnums(content, relPath, fileHash, modulePath, now, lineStarts, docComments, result)
	p.extractConstants(content, relPath, fileHash, modulePath, now, lineStarts, docComments, result)
	p.extractReactHooks(content, relPath, fileHash, modulePath, now, lineStarts, docComments, result)

	return result, nil
}

// ParseDirectory parses all TypeScript/JavaScript files in a directory recursively.
func (p *TypeScriptParser) ParseDirectory(dirPath string) (*ParseResult, error) {
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
			if strings.HasPrefix(name, ".") || name == "node_modules" || name == "dist" || name == "build" || name == "coverage" {
				return filepath.SkipDir
			}
			return nil
		}

		if !p.CanParse(path) {
			return nil
		}

		// Skip test and spec files
		baseName := strings.ToLower(filepath.Base(path))
		if strings.Contains(baseName, ".test.") || strings.Contains(baseName, ".spec.") {
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

// extractFunctions extracts regular function declarations.
func (p *TypeScriptParser) extractFunctions(content []byte, filePath, fileHash, modulePath string, now time.Time, lineStarts []int, docComments map[int]string, result *ParseResult) {
	matches := tsFuncDeclPattern.FindAllSubmatchIndex(content, -1)
	for _, match := range matches {
		if len(match) < 4 {
			continue
		}

		name := string(content[match[2]:match[3]])
		line := findLineNumber(match[0], lineStarts)

		var params string
		if match[6] != -1 && match[7] != -1 {
			params = string(content[match[6]:match[7]])
		}

		var returnType string
		if len(match) >= 10 && match[8] != -1 && match[9] != -1 {
			returnType = strings.TrimSpace(string(content[match[8]:match[9]]))
		}

		sig := fmt.Sprintf("function %s(%s)", name, params)
		if returnType != "" {
			sig += ": " + returnType
		}

		sym := Symbol{
			Name:         name,
			Kind:         SymbolFunction,
			FilePath:     filePath,
			StartLine:    line,
			EndLine:      line, // Will be refined if we can find the closing brace
			Signature:    sig,
			DocComment:   findDocComment(line, docComments),
			ModulePath:   modulePath,
			Visibility:   tsVisibility(string(content[match[0]:match[1]])),
			Language:     "typescript",
			FileHash:     fileHash,
			LastModified: now,
		}

		result.Symbols = append(result.Symbols, sym)
	}
}

// extractArrowFunctions extracts arrow function declarations.
func (p *TypeScriptParser) extractArrowFunctions(content []byte, filePath, fileHash, modulePath string, now time.Time, lineStarts []int, docComments map[int]string, result *ParseResult) {
	matches := tsArrowFuncPattern.FindAllSubmatchIndex(content, -1)
	for _, match := range matches {
		if len(match) < 4 {
			continue
		}

		name := string(content[match[2]:match[3]])
		line := findLineNumber(match[0], lineStarts)

		var params string
		if match[4] != -1 && match[5] != -1 {
			params = string(content[match[4]:match[5]])
		}

		var returnType string
		if len(match) >= 8 && match[6] != -1 && match[7] != -1 {
			returnType = strings.TrimSpace(string(content[match[6]:match[7]]))
		}

		sig := fmt.Sprintf("const %s = (%s) =>", name, params)
		if returnType != "" {
			sig = fmt.Sprintf("const %s = (%s): %s =>", name, params, returnType)
		}

		sym := Symbol{
			Name:         name,
			Kind:         SymbolFunction,
			FilePath:     filePath,
			StartLine:    line,
			EndLine:      line,
			Signature:    sig,
			DocComment:   findDocComment(line, docComments),
			ModulePath:   modulePath,
			Visibility:   tsVisibility(string(content[match[0]:match[1]])),
			Language:     "typescript",
			FileHash:     fileHash,
			LastModified: now,
		}

		result.Symbols = append(result.Symbols, sym)
	}
}

// extractClasses extracts class declarations.
func (p *TypeScriptParser) extractClasses(content []byte, filePath, fileHash, modulePath string, now time.Time, lineStarts []int, docComments map[int]string, result *ParseResult) {
	matches := tsClassPattern.FindAllSubmatchIndex(content, -1)
	for _, match := range matches {
		if len(match) < 4 {
			continue
		}

		name := string(content[match[2]:match[3]])
		line := findLineNumber(match[0], lineStarts)

		var sig strings.Builder
		sig.WriteString("class ")
		sig.WriteString(name)

		if len(match) >= 6 && match[4] != -1 && match[5] != -1 {
			sig.WriteString(" extends ")
			sig.WriteString(string(content[match[4]:match[5]]))
		}

		if len(match) >= 8 && match[6] != -1 && match[7] != -1 {
			sig.WriteString(" implements ")
			sig.WriteString(strings.TrimSpace(string(content[match[6]:match[7]])))
		}

		// Extract decorators for this class
		decorators, decoratorStartPos := extractDecoratorsForPosition(content, match[0])
		docComment := findDocComment(line, docComments)

		// If no JSDoc found at class line but decorators exist, check for JSDoc at decorator line
		if docComment == "" && decoratorStartPos >= 0 {
			decoratorLine := findLineNumber(decoratorStartPos, lineStarts)
			docComment = findDocComment(decoratorLine, docComments)
		}

		if decorators != "" {
			if docComment != "" {
				docComment = decorators + "\n" + docComment
			} else {
				docComment = decorators
			}
		}

		sym := Symbol{
			Name:         name,
			Kind:         SymbolStruct, // Using struct for classes
			FilePath:     filePath,
			StartLine:    line,
			EndLine:      findBlockEnd(content, match[1], lineStarts),
			Signature:    sig.String(),
			DocComment:   docComment,
			ModulePath:   modulePath,
			Visibility:   tsVisibility(string(content[match[0]:match[1]])),
			Language:     "typescript",
			FileHash:     fileHash,
			LastModified: now,
		}

		result.Symbols = append(result.Symbols, sym)

		// Extract class members
		p.extractClassMembers(content, match[1], name, filePath, fileHash, modulePath, now, lineStarts, docComments, result)
	}
}

// extractClassMembers extracts methods and properties from a class body.
func (p *TypeScriptParser) extractClassMembers(content []byte, classStart int, className, filePath, fileHash, modulePath string, now time.Time, lineStarts []int, docComments map[int]string, result *ParseResult) {
	// Find the class body (between { and })
	braceStart := bytes.IndexByte(content[classStart:], '{')
	if braceStart == -1 {
		return
	}
	braceStart += classStart

	braceEnd := findMatchingBrace(content, braceStart)
	if braceEnd == -1 {
		return
	}

	classBody := content[braceStart+1 : braceEnd]
	bodyOffset := braceStart + 1

	// Extract methods
	methodMatches := tsMethodPattern.FindAllSubmatchIndex(classBody, -1)
	for _, match := range methodMatches {
		if len(match) < 4 {
			continue
		}

		name := string(classBody[match[2]:match[3]])
		if name == "constructor" {
			continue // Skip constructor for now
		}

		line := findLineNumber(bodyOffset+match[0], lineStarts)

		var params string
		if match[6] != -1 && match[7] != -1 {
			params = string(classBody[match[6]:match[7]])
		}

		var returnType string
		if len(match) >= 10 && match[8] != -1 && match[9] != -1 {
			returnType = strings.TrimSpace(string(classBody[match[8]:match[9]]))
		}

		sig := fmt.Sprintf("%s.%s(%s)", className, name, params)
		if returnType != "" {
			sig += ": " + returnType
		}

		sym := Symbol{
			Name:         name,
			Kind:         SymbolMethod,
			FilePath:     filePath,
			StartLine:    line,
			EndLine:      line,
			Signature:    sig,
			DocComment:   findDocComment(line, docComments),
			ModulePath:   modulePath,
			Visibility:   tsMethodVisibility(string(classBody[match[0]:match[1]])),
			Language:     "typescript",
			FileHash:     fileHash,
			LastModified: now,
		}

		result.Symbols = append(result.Symbols, sym)
	}

	// Extract properties
	propMatches := tsPropertyPattern.FindAllSubmatchIndex(classBody, -1)
	for _, match := range propMatches {
		if len(match) < 4 {
			continue
		}

		name := string(classBody[match[2]:match[3]])
		line := findLineNumber(bodyOffset+match[0], lineStarts)

		var propType string
		if match[4] != -1 && match[5] != -1 {
			propType = strings.TrimSpace(string(classBody[match[4]:match[5]]))
		}

		sig := fmt.Sprintf("%s.%s: %s", className, name, propType)

		sym := Symbol{
			Name:         name,
			Kind:         SymbolField,
			FilePath:     filePath,
			StartLine:    line,
			EndLine:      line,
			Signature:    sig,
			DocComment:   findDocComment(line, docComments),
			ModulePath:   modulePath,
			Visibility:   tsMethodVisibility(string(classBody[match[0]:match[1]])),
			Language:     "typescript",
			FileHash:     fileHash,
			LastModified: now,
		}

		result.Symbols = append(result.Symbols, sym)
	}
}

// extractInterfaces extracts interface declarations.
func (p *TypeScriptParser) extractInterfaces(content []byte, filePath, fileHash, modulePath string, now time.Time, lineStarts []int, docComments map[int]string, result *ParseResult) {
	matches := tsInterfacePattern.FindAllSubmatchIndex(content, -1)
	for _, match := range matches {
		if len(match) < 4 {
			continue
		}

		name := string(content[match[2]:match[3]])
		line := findLineNumber(match[0], lineStarts)

		var sig strings.Builder
		sig.WriteString("interface ")
		sig.WriteString(name)

		if len(match) >= 6 && match[4] != -1 && match[5] != -1 {
			sig.WriteString(" extends ")
			sig.WriteString(strings.TrimSpace(string(content[match[4]:match[5]])))
		}

		sym := Symbol{
			Name:         name,
			Kind:         SymbolInterface,
			FilePath:     filePath,
			StartLine:    line,
			EndLine:      findBlockEnd(content, match[1], lineStarts),
			Signature:    sig.String(),
			DocComment:   findDocComment(line, docComments),
			ModulePath:   modulePath,
			Visibility:   tsVisibility(string(content[match[0]:match[1]])),
			Language:     "typescript",
			FileHash:     fileHash,
			LastModified: now,
		}

		result.Symbols = append(result.Symbols, sym)
	}
}

// extractTypeAliases extracts type alias declarations.
func (p *TypeScriptParser) extractTypeAliases(content []byte, filePath, fileHash, modulePath string, now time.Time, lineStarts []int, docComments map[int]string, result *ParseResult) {
	matches := tsTypeAliasPattern.FindAllSubmatchIndex(content, -1)
	for _, match := range matches {
		if len(match) < 4 {
			continue
		}

		name := string(content[match[2]:match[3]])
		line := findLineNumber(match[0], lineStarts)

		sym := Symbol{
			Name:         name,
			Kind:         SymbolType,
			FilePath:     filePath,
			StartLine:    line,
			EndLine:      line,
			Signature:    fmt.Sprintf("type %s", name),
			DocComment:   findDocComment(line, docComments),
			ModulePath:   modulePath,
			Visibility:   tsVisibility(string(content[match[0]:match[1]])),
			Language:     "typescript",
			FileHash:     fileHash,
			LastModified: now,
		}

		result.Symbols = append(result.Symbols, sym)
	}
}

// extractEnums extracts enum declarations.
func (p *TypeScriptParser) extractEnums(content []byte, filePath, fileHash, modulePath string, now time.Time, lineStarts []int, docComments map[int]string, result *ParseResult) {
	matches := tsEnumPattern.FindAllSubmatchIndex(content, -1)
	for _, match := range matches {
		if len(match) < 4 {
			continue
		}

		name := string(content[match[2]:match[3]])
		line := findLineNumber(match[0], lineStarts)

		sym := Symbol{
			Name:         name,
			Kind:         SymbolType,
			FilePath:     filePath,
			StartLine:    line,
			EndLine:      findBlockEnd(content, match[1], lineStarts),
			Signature:    fmt.Sprintf("enum %s", name),
			DocComment:   findDocComment(line, docComments),
			ModulePath:   modulePath,
			Visibility:   tsVisibility(string(content[match[0]:match[1]])),
			Language:     "typescript",
			FileHash:     fileHash,
			LastModified: now,
		}

		result.Symbols = append(result.Symbols, sym)
	}
}

// extractConstants extracts const/let/var declarations (excluding arrow functions).
func (p *TypeScriptParser) extractConstants(content []byte, filePath, fileHash, modulePath string, now time.Time, lineStarts []int, docComments map[int]string, result *ParseResult) {
	matches := tsConstPattern.FindAllSubmatchIndex(content, -1)
	for _, match := range matches {
		if len(match) < 4 {
			continue
		}

		name := string(content[match[2]:match[3]])
		line := findLineNumber(match[0], lineStarts)

		// Skip if this is likely an arrow function (already extracted)
		endIdx := match[1]
		if endIdx < len(content) {
			rest := content[endIdx:]
			if bytes.HasPrefix(bytes.TrimSpace(rest), []byte("(")) ||
				bytes.HasPrefix(bytes.TrimSpace(rest), []byte("async")) {
				continue
			}
		}

		var constType string
		if match[4] != -1 && match[5] != -1 {
			constType = strings.TrimSpace(string(content[match[4]:match[5]]))
		}

		sig := fmt.Sprintf("const %s", name)
		if constType != "" {
			sig += ": " + constType
		}

		sym := Symbol{
			Name:         name,
			Kind:         SymbolConstant,
			FilePath:     filePath,
			StartLine:    line,
			EndLine:      line,
			Signature:    sig,
			DocComment:   findDocComment(line, docComments),
			ModulePath:   modulePath,
			Visibility:   tsVisibility(string(content[match[0]:match[1]])),
			Language:     "typescript",
			FileHash:     fileHash,
			LastModified: now,
		}

		result.Symbols = append(result.Symbols, sym)
	}
}

// extractDecoratorsForPosition finds decorators that appear immediately before a given position.
// Handles multi-line decorators like @Component({ selector: 'app' }).
// Returns a formatted string with all decorators (or empty if none), and the byte position where
// the first decorator starts (for JSDoc lookup). Returns -1 if no decorators found.
func extractDecoratorsForPosition(content []byte, position int) (string, int) {
	if position <= 0 {
		return "", -1
	}

	// Strategy: Scan backwards from position, collecting decorators.
	// Stop when we hit: }, ;, or { that's not inside a decorator's parens.

	var decorators []string
	var decoratorStartPos int = -1 // Track where the first decorator starts
	i := position - 1

	// Skip whitespace at the end
	for i >= 0 && (content[i] == ' ' || content[i] == '\t' || content[i] == '\n') {
		i--
	}

	// Now scan backwards looking for decorators
	for i >= 0 {
		// Skip whitespace
		for i >= 0 && (content[i] == ' ' || content[i] == '\t' || content[i] == '\n') {
			i--
		}
		if i < 0 {
			break
		}

		// Check if we're at the end of a decorator (either ) or identifier)
		if content[i] == ')' {
			// Find matching ( - this is decorator arguments
			parenEnd := i
			parenDepth := 1
			i-- // Move past )
			for i >= 0 && parenDepth > 0 {
				switch content[i] {
				case ')':
					parenDepth++
				case '(':
					parenDepth--
				}
				if parenDepth > 0 {
					i--
				}
			}
			if i < 0 {
				break
			}
			i-- // Move past (

			// Skip whitespace between identifier and (
			for i >= 0 && (content[i] == ' ' || content[i] == '\t' || content[i] == '\n') {
				i--
			}

			// Read identifier backwards
			for i >= 0 && (isAlphanumeric(content[i]) || content[i] == '_') {
				i--
			}
			identStart := i + 1

			// Check for @
			if i >= 0 && content[i] == '@' {
				decoratorStartPos = i // Track the first decorator position (since we're going backwards)
				decoratorText := string(content[i : parenEnd+1])
				decoratorText = strings.Join(strings.Fields(decoratorText), " ")
				decorators = append([]string{decoratorText}, decorators...)
				i-- // Move past @
			} else {
				// Not a decorator - put back position and stop
				i = identStart - 1
				break
			}
		} else if isAlphanumeric(content[i]) || content[i] == '_' {
			// Could be decorator without parens: @Decorator
			identEnd := i + 1
			for i >= 0 && (isAlphanumeric(content[i]) || content[i] == '_') {
				i--
			}

			// Check for @
			if i >= 0 && content[i] == '@' {
				decoratorStartPos = i // Track the first decorator position (since we're going backwards)
				decoratorText := string(content[i:identEnd])
				decorators = append([]string{decoratorText}, decorators...)
				i-- // Move past @
			} else {
				// Not a decorator - stop (hit a keyword like 'export')
				break
			}
		} else if content[i] == '/' && i > 0 && content[i-1] == '*' {
			// End of JSDoc comment - skip it backwards
			i -= 2
			for i > 0 && !(content[i] == '/' && content[i+1] == '*') {
				i--
			}
			if i > 0 {
				i-- // Move before /*
			}
		} else if content[i] == '}' || content[i] == ';' || content[i] == '{' {
			// Hit boundary - stop
			break
		} else {
			// Unknown - stop
			break
		}
	}

	if len(decorators) == 0 {
		return "", -1
	}

	return strings.Join(decorators, " "), decoratorStartPos
}

// isAlphanumeric checks if a byte is a letter or digit
func isAlphanumeric(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
}

// extractReactHooks extracts React hook usage and custom hook definitions.
// It identifies both built-in hooks (useState, useEffect, etc.) and custom hooks (use* naming convention).
func (p *TypeScriptParser) extractReactHooks(content []byte, filePath, fileHash, modulePath string, now time.Time, lineStarts []int, docComments map[int]string, result *ParseResult) {
	// Find custom hook definitions and update existing symbols with [React Hook] prefix
	matches := tsCustomHookDefPattern.FindAllSubmatchIndex(content, -1)
	customHookNames := make(map[string]bool)

	for _, match := range matches {
		if len(match) < 4 {
			continue
		}

		var name string
		// Check function definition capture group first (match[2]:match[3])
		if match[2] != -1 && match[3] != -1 {
			name = string(content[match[2]:match[3]])
		} else if len(match) >= 6 && match[4] != -1 && match[5] != -1 {
			// Then check const/let definition capture group (match[4]:match[5])
			name = string(content[match[4]:match[5]])
		}

		if name != "" {
			customHookNames[name] = true
		}
	}

	// Update existing symbols that are custom hooks with [React Hook] prefix
	for i := range result.Symbols {
		if customHookNames[result.Symbols[i].Name] {
			if !strings.HasPrefix(result.Symbols[i].DocComment, "[React Hook]") {
				if result.Symbols[i].DocComment != "" {
					result.Symbols[i].DocComment = "[React Hook] " + result.Symbols[i].DocComment
				} else {
					result.Symbols[i].DocComment = "[React Hook] Custom hook definition"
				}
			}
			// Update signature to show it's a hook
			if !strings.HasPrefix(result.Symbols[i].Signature, "hook ") {
				result.Symbols[i].Signature = "hook " + result.Symbols[i].Name
			}
		}
	}

	// Extract React hook usage with variable assignment (const [x, setX] = useState(...))
	hookMatches := tsReactHookCallWithVarPattern.FindAllSubmatchIndex(content, -1)
	for _, match := range hookMatches {
		if len(match) < 6 {
			continue
		}

		var hookName string
		if match[4] != -1 && match[5] != -1 {
			hookName = string(content[match[4]:match[5]])
		}

		if hookName == "" {
			continue
		}

		line := findLineNumber(match[0], lineStarts)
		hookCategory := classifyReactHook(hookName)

		if hookCategory != "" {
			rel := SymbolRelation{
				FromSymbolID: 0,
				ToSymbolID:   0,
				RelationType: RelationCalls,
				CallSiteLine: line,
				Metadata: map[string]any{
					"hookName":     hookName,
					"hookCategory": hookCategory,
					"filePath":     filePath,
				},
			}
			result.Relations = append(result.Relations, rel)
		}
	}

	// Extract standalone hook calls (useEffect, useLayoutEffect, etc.)
	standaloneMatches := tsReactHookCallStandalonePattern.FindAllSubmatchIndex(content, -1)
	for _, match := range standaloneMatches {
		if len(match) < 4 {
			continue
		}

		var hookName string
		if match[2] != -1 && match[3] != -1 {
			hookName = string(content[match[2]:match[3]])
		}

		if hookName == "" {
			continue
		}

		line := findLineNumber(match[0], lineStarts)
		hookCategory := classifyReactHook(hookName)

		if hookCategory != "" {
			rel := SymbolRelation{
				FromSymbolID: 0,
				ToSymbolID:   0,
				RelationType: RelationCalls,
				CallSiteLine: line,
				Metadata: map[string]any{
					"hookName":     hookName,
					"hookCategory": hookCategory,
					"filePath":     filePath,
				},
			}
			result.Relations = append(result.Relations, rel)
		}
	}
}

// classifyReactHook returns the category of a React hook.
func classifyReactHook(hookName string) string {
	switch hookName {
	case "useState", "useReducer":
		return "state-management"
	case "useEffect", "useLayoutEffect":
		return "side-effect"
	case "useMemo", "useCallback":
		return "memoization"
	case "useContext":
		return "context"
	case "useRef":
		return "ref"
	default:
		if strings.HasPrefix(hookName, "use") {
			return "custom-hook"
		}
		return ""
	}
}

// Helper functions

func (p *TypeScriptParser) relativePath(absPath string) string {
	if p.basePath == "" {
		return absPath
	}
	rel, err := filepath.Rel(p.basePath, absPath)
	if err != nil {
		return absPath
	}
	return rel
}

func extractTSModulePath(filePath string) string {
	dir := filepath.Dir(filePath)
	if dir == "." || dir == "" {
		return ""
	}
	return dir
}

func buildLineStartMap(content []byte) []int {
	lineStarts := []int{0}
	for i, b := range content {
		if b == '\n' {
			lineStarts = append(lineStarts, i+1)
		}
	}
	return lineStarts
}

func findLineNumber(offset int, lineStarts []int) int {
	for i := len(lineStarts) - 1; i >= 0; i-- {
		if lineStarts[i] <= offset {
			return i + 1
		}
	}
	return 1
}

func extractJSDocComments(content []byte, lineStarts []int) map[int]string {
	comments := make(map[int]string)
	matches := tsJSDocPattern.FindAllSubmatchIndex(content, -1)

	for _, match := range matches {
		if len(match) < 4 {
			continue
		}

		commentEnd := match[1]
		// Find the line after the comment
		nextLine := findLineNumber(commentEnd, lineStarts) + 1

		commentText := string(content[match[2]:match[3]])
		commentText = cleanJSDocComment(commentText)

		comments[nextLine] = commentText
	}

	return comments
}

func cleanJSDocComment(comment string) string {
	var lines []string
	scanner := bufio.NewScanner(strings.NewReader(comment))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		line = strings.TrimPrefix(line, "*")
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "@") {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, " ")
}

func findDocComment(line int, docComments map[int]string) string {
	if doc, ok := docComments[line]; ok {
		return doc
	}
	return ""
}

func tsVisibility(declaration string) string {
	if strings.Contains(declaration, "export") {
		return "public"
	}
	return "private"
}

func tsMethodVisibility(declaration string) string {
	if strings.Contains(declaration, "private") {
		return "private"
	}
	if strings.Contains(declaration, "protected") {
		return "protected"
	}
	return "public"
}

func findBlockEnd(content []byte, startOffset int, lineStarts []int) int {
	braceStart := bytes.IndexByte(content[startOffset:], '{')
	if braceStart == -1 {
		return findLineNumber(startOffset, lineStarts)
	}

	braceEnd := findMatchingBrace(content, startOffset+braceStart)
	if braceEnd == -1 {
		return findLineNumber(startOffset, lineStarts)
	}

	return findLineNumber(braceEnd, lineStarts)
}

func findMatchingBrace(content []byte, openBracePos int) int {
	if openBracePos >= len(content) || content[openBracePos] != '{' {
		return -1
	}

	depth := 1
	inString := false
	stringChar := byte(0)
	escaped := false

	for i := openBracePos + 1; i < len(content); i++ {
		c := content[i]

		if escaped {
			escaped = false
			continue
		}

		if c == '\\' && inString {
			escaped = true
			continue
		}

		if !inString && (c == '"' || c == '\'' || c == '`') {
			inString = true
			stringChar = c
			continue
		}

		if inString && c == stringChar {
			inString = false
			continue
		}

		if inString {
			continue
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

// Ensure TypeScriptParser implements LanguageParser at compile time.
var _ LanguageParser = (*TypeScriptParser)(nil)
