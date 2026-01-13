// Package parser provides Python source code parsing for symbol extraction.
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

// PythonParser extracts symbols from Python source files.
// It uses regex-based extraction for CGO-free operation.
type PythonParser struct {
	basePath string
}

// NewPythonParser creates a new Python parser instance.
func NewPythonParser(basePath string) *PythonParser {
	return &PythonParser{
		basePath: basePath,
	}
}

// SupportedExtensions returns the file extensions this parser can handle.
func (p *PythonParser) SupportedExtensions() []string {
	return []string{".py", ".pyi"}
}

// Language returns the language identifier for this parser.
func (p *PythonParser) Language() string {
	return "python"
}

// CanParse returns true if this parser can handle the given file path.
func (p *PythonParser) CanParse(filePath string) bool {
	ext := strings.ToLower(filepath.Ext(filePath))
	for _, supported := range p.SupportedExtensions() {
		if ext == supported {
			return true
		}
	}
	return false
}

// Python regex patterns for symbol extraction
var (
	// Function definitions: def name(...) or async def name(...)
	// Uses [ \t]* instead of \s* to avoid matching newlines as indent
	pyFuncPattern = regexp.MustCompile(`(?m)^([ \t]*)(?:async\s+)?def\s+(\w+)\s*\(([^)]*)\)(?:\s*->\s*([^:]+))?:`)

	// Class definitions: class Name or class Name(Base1, Base2)
	// Uses [ \t]* instead of \s* to avoid matching newlines as indent
	pyClassPattern = regexp.MustCompile(`(?m)^([ \t]*)class\s+(\w+)(?:\s*\(([^)]*)\))?:`)

	// Variable assignments with type annotations: name: Type = value
	pyTypedVarPattern = regexp.MustCompile(`(?m)^(\w+)\s*:\s*([^=\n]+)(?:\s*=)?`)

	// Module-level variable assignments: NAME = value (UPPER_CASE for constants)
	pyConstPattern = regexp.MustCompile(`(?m)^([A-Z][A-Z0-9_]*)\s*(?::\s*[^=\n]+)?\s*=`)

	// Decorator pattern: @decorator or @decorator(args)
	pyDecoratorPattern = regexp.MustCompile(`(?m)^(\s*)@(\w+(?:\.\w+)*)(?:\([^)]*\))?`)

	// String patterns for stripping (to avoid false positives)
	pyTripleDoubleQuote = regexp.MustCompile(`"""[\s\S]*?"""`)
	pyTripleSingleQuote = regexp.MustCompile(`'''[\s\S]*?'''`)
)

// ParseFile parses a Python file and extracts symbols.
func (p *PythonParser) ParseFile(filePath string) (*ParseResult, error) {
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
	modulePath := extractPyModulePath(relPath)
	now := time.Now()

	// Build line map for position tracking
	lines := bytes.Split(content, []byte("\n"))
	lineStarts := buildPyLineStartMap(content)

	// Extract docstrings for documentation lookup (before stripping strings)
	docstrings := extractDocstrings(lines)

	// Strip string literals to avoid false positives (replaces with spaces to preserve positions)
	strippedContent := stripPyStrings(content)
	strippedLines := bytes.Split(strippedContent, []byte("\n"))

	// Extract symbols using stripped content for pattern matching
	p.extractFunctions(strippedContent, strippedLines, relPath, fileHash, modulePath, now, lineStarts, docstrings, result)
	p.extractClasses(strippedContent, strippedLines, relPath, fileHash, modulePath, now, lineStarts, docstrings, result)
	p.extractConstants(strippedContent, relPath, fileHash, modulePath, now, lineStarts, docstrings, result)
	p.extractTypedVariables(strippedContent, relPath, fileHash, modulePath, now, lineStarts, docstrings, result)

	return result, nil
}

// ParseDirectory parses all Python files in a directory recursively.
func (p *PythonParser) ParseDirectory(dirPath string) (*ParseResult, error) {
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
			if strings.HasPrefix(name, ".") || name == "__pycache__" || name == "venv" ||
				name == ".venv" || name == "env" || name == ".env" ||
				name == "node_modules" || name == "dist" || name == "build" {
				return filepath.SkipDir
			}
			return nil
		}

		if !p.CanParse(path) {
			return nil
		}

		// Skip test files
		baseName := strings.ToLower(filepath.Base(path))
		if strings.HasPrefix(baseName, "test_") || strings.HasSuffix(baseName, "_test.py") {
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

// extractFunctions extracts function definitions.
func (p *PythonParser) extractFunctions(content []byte, lines [][]byte, filePath, fileHash, modulePath string, now time.Time, lineStarts []int, docstrings map[int]string, result *ParseResult) {
	matches := pyFuncPattern.FindAllSubmatchIndex(content, -1)

	for _, match := range matches {
		if len(match) < 6 {
			continue
		}

		indent := string(content[match[2]:match[3]])
		name := string(content[match[4]:match[5]])
		line := findLineNumber(match[0], lineStarts)

		// Skip nested functions (non-zero indent at module level or inside classes)
		// We'll extract class methods separately
		if len(indent) > 0 && !p.isClassMethod(content, match[0], indent) {
			continue
		}

		var params string
		if match[6] != -1 && match[7] != -1 {
			params = string(content[match[6]:match[7]])
		}

		var returnType string
		if len(match) >= 10 && match[8] != -1 && match[9] != -1 {
			returnType = strings.TrimSpace(string(content[match[8]:match[9]]))
		}

		sig := fmt.Sprintf("def %s(%s)", name, params)
		if returnType != "" {
			sig += " -> " + returnType
		}

		// Check for decorators
		decorators := p.findDecorators(content, match[0], lineStarts)

		kind := SymbolFunction
		if len(indent) > 0 {
			kind = SymbolMethod
		}

		sym := Symbol{
			Name:         name,
			Kind:         kind,
			FilePath:     filePath,
			StartLine:    line,
			EndLine:      findPyBlockEnd(lines, line-1, len(indent)),
			Signature:    sig,
			DocComment:   findPyDocstring(line, docstrings),
			ModulePath:   modulePath,
			Visibility:   pyVisibility(name, decorators),
			Language:     "python",
			FileHash:     fileHash,
			LastModified: now,
		}

		result.Symbols = append(result.Symbols, sym)
	}
}

// extractClasses extracts class definitions and their methods.
func (p *PythonParser) extractClasses(content []byte, lines [][]byte, filePath, fileHash, modulePath string, now time.Time, lineStarts []int, docstrings map[int]string, result *ParseResult) {
	matches := pyClassPattern.FindAllSubmatchIndex(content, -1)

	for _, match := range matches {
		if len(match) < 6 {
			continue
		}

		indent := string(content[match[2]:match[3]])
		name := string(content[match[4]:match[5]])
		line := findLineNumber(match[0], lineStarts)

		// Skip nested classes
		if len(indent) > 0 {
			continue
		}

		var bases string
		if match[6] != -1 && match[7] != -1 {
			bases = strings.TrimSpace(string(content[match[6]:match[7]]))
		}

		var sig strings.Builder
		sig.WriteString("class ")
		sig.WriteString(name)
		if bases != "" {
			sig.WriteString("(")
			sig.WriteString(bases)
			sig.WriteString(")")
		}

		endLine := findPyBlockEnd(lines, line-1, 0)

		sym := Symbol{
			Name:         name,
			Kind:         SymbolStruct, // Using struct for classes
			FilePath:     filePath,
			StartLine:    line,
			EndLine:      endLine,
			Signature:    sig.String(),
			DocComment:   findPyDocstring(line, docstrings),
			ModulePath:   modulePath,
			Visibility:   pyVisibility(name, nil),
			Language:     "python",
			FileHash:     fileHash,
			LastModified: now,
		}

		result.Symbols = append(result.Symbols, sym)

		// Extract class methods
		p.extractClassMethods(content, lines, line, endLine, name, filePath, fileHash, modulePath, now, lineStarts, docstrings, result)
	}
}

// extractClassMethods extracts methods from within a class body.
func (p *PythonParser) extractClassMethods(content []byte, lines [][]byte, classStartLine, classEndLine int, className, filePath, fileHash, modulePath string, now time.Time, lineStarts []int, docstrings map[int]string, result *ParseResult) {
	// Find method definitions within the class
	matches := pyFuncPattern.FindAllSubmatchIndex(content, -1)

	for _, match := range matches {
		if len(match) < 6 {
			continue
		}

		line := findLineNumber(match[0], lineStarts)

		// Only process methods within this class
		if line <= classStartLine || line > classEndLine {
			continue
		}

		indent := string(content[match[2]:match[3]])
		// Class methods should have exactly one level of indentation (4 spaces or 1 tab typically)
		if len(indent) == 0 || len(indent) > 8 {
			continue
		}

		name := string(content[match[4]:match[5]])

		var params string
		if match[6] != -1 && match[7] != -1 {
			params = string(content[match[6]:match[7]])
		}

		var returnType string
		if len(match) >= 10 && match[8] != -1 && match[9] != -1 {
			returnType = strings.TrimSpace(string(content[match[8]:match[9]]))
		}

		// Remove 'self' or 'cls' from params for signature
		cleanParams := cleanPyParams(params)

		sig := fmt.Sprintf("%s.%s(%s)", className, name, cleanParams)
		if returnType != "" {
			sig += " -> " + returnType
		}

		decorators := p.findDecorators(content, match[0], lineStarts)

		sym := Symbol{
			Name:         name,
			Kind:         SymbolMethod,
			FilePath:     filePath,
			StartLine:    line,
			EndLine:      findPyBlockEnd(lines, line-1, len(indent)),
			Signature:    sig,
			DocComment:   findPyDocstring(line, docstrings),
			ModulePath:   modulePath,
			Visibility:   pyVisibility(name, decorators),
			Language:     "python",
			FileHash:     fileHash,
			LastModified: now,
		}

		result.Symbols = append(result.Symbols, sym)
	}
}

// extractConstants extracts module-level constant assignments.
func (p *PythonParser) extractConstants(content []byte, filePath, fileHash, modulePath string, now time.Time, lineStarts []int, docstrings map[int]string, result *ParseResult) {
	matches := pyConstPattern.FindAllSubmatchIndex(content, -1)

	for _, match := range matches {
		if len(match) < 4 {
			continue
		}

		name := string(content[match[2]:match[3]])
		line := findLineNumber(match[0], lineStarts)

		sym := Symbol{
			Name:         name,
			Kind:         SymbolConstant,
			FilePath:     filePath,
			StartLine:    line,
			EndLine:      line,
			Signature:    name,
			DocComment:   findPyDocstring(line, docstrings),
			ModulePath:   modulePath,
			Visibility:   "public",
			Language:     "python",
			FileHash:     fileHash,
			LastModified: now,
		}

		result.Symbols = append(result.Symbols, sym)
	}
}

// extractTypedVariables extracts module-level typed variable declarations.
func (p *PythonParser) extractTypedVariables(content []byte, filePath, fileHash, modulePath string, now time.Time, lineStarts []int, docstrings map[int]string, result *ParseResult) {
	matches := pyTypedVarPattern.FindAllSubmatchIndex(content, -1)

	for _, match := range matches {
		if len(match) < 6 {
			continue
		}

		name := string(content[match[2]:match[3]])
		varType := strings.TrimSpace(string(content[match[4]:match[5]]))
		line := findLineNumber(match[0], lineStarts)

		// Skip if this is all uppercase (already captured as constant)
		if strings.ToUpper(name) == name {
			continue
		}

		// Skip private variables for now
		if strings.HasPrefix(name, "_") {
			continue
		}

		sig := fmt.Sprintf("%s: %s", name, varType)

		sym := Symbol{
			Name:         name,
			Kind:         SymbolVariable,
			FilePath:     filePath,
			StartLine:    line,
			EndLine:      line,
			Signature:    sig,
			DocComment:   findPyDocstring(line, docstrings),
			ModulePath:   modulePath,
			Visibility:   pyVisibility(name, nil),
			Language:     "python",
			FileHash:     fileHash,
			LastModified: now,
		}

		result.Symbols = append(result.Symbols, sym)
	}
}

// Helper functions

// stripPyStrings removes string literals from Python content to avoid false positives.
// It replaces string content with spaces to preserve byte positions for accurate line tracking.
func stripPyStrings(content []byte) []byte {
	result := make([]byte, len(content))
	copy(result, content)

	// Replace triple-quoted strings first (must be processed before regular quotes)
	result = pyTripleDoubleQuote.ReplaceAllFunc(result, func(match []byte) []byte {
		// Keep the first 3 quotes and replace the rest (except last 3) with spaces
		// This preserves docstrings that the parser needs to identify
		replacement := make([]byte, len(match))
		for i := range replacement {
			if match[i] == '\n' {
				replacement[i] = '\n' // Preserve newlines for line counting
			} else {
				replacement[i] = ' '
			}
		}
		return replacement
	})

	result = pyTripleSingleQuote.ReplaceAllFunc(result, func(match []byte) []byte {
		replacement := make([]byte, len(match))
		for i := range replacement {
			if match[i] == '\n' {
				replacement[i] = '\n'
			} else {
				replacement[i] = ' '
			}
		}
		return replacement
	})

	return result
}

func (p *PythonParser) relativePath(absPath string) string {
	if p.basePath == "" {
		return absPath
	}
	rel, err := filepath.Rel(p.basePath, absPath)
	if err != nil {
		return absPath
	}
	return rel
}

func extractPyModulePath(filePath string) string {
	dir := filepath.Dir(filePath)
	if dir == "." || dir == "" {
		return ""
	}
	// Convert path separators to Python module dots
	return strings.ReplaceAll(dir, string(filepath.Separator), ".")
}

func buildPyLineStartMap(content []byte) []int {
	lineStarts := []int{0}
	for i, b := range content {
		if b == '\n' {
			lineStarts = append(lineStarts, i+1)
		}
	}
	return lineStarts
}

// extractDocstrings finds docstrings that appear right after definitions.
func extractDocstrings(lines [][]byte) map[int]string {
	docstrings := make(map[int]string)

	for i := 0; i < len(lines)-1; i++ {
		line := strings.TrimSpace(string(lines[i]))

		// Check if this line starts a definition
		if strings.HasPrefix(line, "def ") || strings.HasPrefix(line, "async def ") ||
			strings.HasPrefix(line, "class ") {

			// Check next line for docstring
			if i+1 < len(lines) {
				nextLine := strings.TrimSpace(string(lines[i+1]))
				if strings.HasPrefix(nextLine, `"""`) || strings.HasPrefix(nextLine, `'''`) {
					docstring := extractMultiLineDocstring(lines, i+1)
					docstrings[i+2] = docstring // Line number is 1-indexed
				}
			}
		}
	}

	return docstrings
}

func extractMultiLineDocstring(lines [][]byte, startIdx int) string {
	if startIdx >= len(lines) {
		return ""
	}

	firstLine := string(lines[startIdx])
	trimmed := strings.TrimSpace(firstLine)

	// Determine quote style
	quoteStyle := `"""`
	if strings.HasPrefix(trimmed, `'''`) {
		quoteStyle = `'''`
	}

	// Check for single-line docstring
	content := strings.TrimPrefix(trimmed, quoteStyle)
	if strings.HasSuffix(content, quoteStyle) {
		return strings.TrimSuffix(content, quoteStyle)
	}

	// Multi-line docstring
	var sb strings.Builder
	sb.WriteString(content)

	for i := startIdx + 1; i < len(lines); i++ {
		line := string(lines[i])
		if strings.Contains(line, quoteStyle) {
			// End of docstring
			endIdx := strings.Index(line, quoteStyle)
			sb.WriteString(" ")
			sb.WriteString(strings.TrimSpace(line[:endIdx]))
			break
		}
		sb.WriteString(" ")
		sb.WriteString(strings.TrimSpace(line))
	}

	return strings.TrimSpace(sb.String())
}

func findPyDocstring(line int, docstrings map[int]string) string {
	// Check line after definition (where docstring would be)
	if doc, ok := docstrings[line+1]; ok {
		return parseStructuredDocstring(doc)
	}
	return ""
}

// parseStructuredDocstring parses Google, NumPy, and Sphinx style docstrings
// and returns a normalized representation with structured sections.
func parseStructuredDocstring(raw string) string {
	if raw == "" {
		return ""
	}

	// Detect the docstring format
	format := detectDocstringFormat(raw)

	switch format {
	case "google":
		return parseGoogleDocstring(raw)
	case "numpy":
		return parseNumpyDocstring(raw)
	case "sphinx":
		return parseSphinxDocstring(raw)
	default:
		// Plain docstring - just return the description
		return strings.TrimSpace(raw)
	}
}

// detectDocstringFormat identifies the docstring style.
// Note: docstrings may be collapsed into single lines with spaces instead of newlines.
func detectDocstringFormat(doc string) string {
	lower := strings.ToLower(doc)

	// Sphinx style: :param, :type, :returns:, :rtype:, :raises: (check first as it's most distinctive)
	if strings.Contains(doc, ":param ") || strings.Contains(doc, ":type ") ||
		strings.Contains(doc, ":returns:") || strings.Contains(doc, ":rtype:") ||
		strings.Contains(doc, ":raises ") {
		return "sphinx"
	}

	// Google style: Args:, Returns:, Raises:, Yields:, Note:, Example:
	if strings.Contains(lower, "args:") || strings.Contains(lower, "arguments:") ||
		strings.Contains(lower, "parameters:") ||
		(strings.Contains(lower, "returns:") && !strings.Contains(doc, ":returns:")) ||
		(strings.Contains(lower, "raises:") && !strings.Contains(doc, ":raises")) ||
		strings.Contains(lower, "yields:") ||
		strings.Contains(lower, "attributes:") || strings.Contains(lower, "example:") {
		return "google"
	}

	// NumPy style: Parameters section with dashes below (-------)
	if strings.Contains(doc, "Parameters") && strings.Contains(doc, "---") {
		return "numpy"
	}

	return "plain"
}

// parseGoogleDocstring parses Google-style docstrings.
func parseGoogleDocstring(doc string) string {
	var result strings.Builder

	// Extract description (everything before the first section header)
	sections := regexp.MustCompile(`(?i)(Args|Arguments|Parameters|Returns|Yields|Raises|Attributes|Example|Examples|Note|Notes|Warning|Warnings|See Also|Todo):`).Split(doc, 2)
	if len(sections) > 0 {
		desc := strings.TrimSpace(sections[0])
		if desc != "" {
			result.WriteString(desc)
		}
	}

	// Extract Args section for parameter types
	argsMatch := regexp.MustCompile(`(?is)(?:Args|Arguments|Parameters):\s*(.*?)(?:Returns:|Raises:|Yields:|Attributes:|Example|Note|$)`).FindStringSubmatch(doc)
	if len(argsMatch) > 1 {
		params := parseGoogleParams(argsMatch[1])
		if params != "" {
			if result.Len() > 0 {
				result.WriteString(" | ")
			}
			result.WriteString("Params: ")
			result.WriteString(params)
		}
	}

	// Extract Returns section
	returnsMatch := regexp.MustCompile(`(?is)Returns:\s*(.*?)(?:Raises:|Yields:|Example|Note|$)`).FindStringSubmatch(doc)
	if len(returnsMatch) > 1 {
		returns := strings.TrimSpace(returnsMatch[1])
		// Simplify to first line or type
		lines := strings.Split(returns, "\n")
		if len(lines) > 0 && strings.TrimSpace(lines[0]) != "" {
			if result.Len() > 0 {
				result.WriteString(" | ")
			}
			result.WriteString("Returns: ")
			result.WriteString(strings.TrimSpace(lines[0]))
		}
	}

	// Extract Raises section
	raisesMatch := regexp.MustCompile(`(?is)Raises:\s*(.*?)(?:Returns:|Yields:|Example|Note|$)`).FindStringSubmatch(doc)
	if len(raisesMatch) > 1 {
		raises := extractExceptionNames(raisesMatch[1])
		if raises != "" {
			if result.Len() > 0 {
				result.WriteString(" | ")
			}
			result.WriteString("Raises: ")
			result.WriteString(raises)
		}
	}

	if result.Len() == 0 {
		return strings.TrimSpace(doc)
	}

	return result.String()
}

// parseGoogleParams extracts parameter names and types from Google-style Args section.
// Works with both newline-separated and space-collapsed formats.
func parseGoogleParams(args string) string {
	var params []string
	// Pattern: param_name (type): description or param_name: description
	// This regex finds all parameter definitions in the text
	paramPattern := regexp.MustCompile(`(\w+)\s*\(([^)]+)\)\s*:`)
	matches := paramPattern.FindAllStringSubmatch(args, -1)

	for _, m := range matches {
		if len(m) >= 3 {
			name := m[1]
			ptype := m[2]
			params = append(params, name+": "+ptype)
		}
	}

	// If no typed params found, try without type annotation
	if len(params) == 0 {
		simplePattern := regexp.MustCompile(`(\w+)\s*:\s*[^(]`)
		simpleMatches := simplePattern.FindAllStringSubmatch(args, -1)
		for _, m := range simpleMatches {
			if len(m) >= 2 {
				params = append(params, m[1])
			}
		}
	}

	return strings.Join(params, ", ")
}

// parseNumpyDocstring parses NumPy-style docstrings.
func parseNumpyDocstring(doc string) string {
	var result strings.Builder

	// Extract description (first paragraph before any section)
	lines := strings.Split(doc, "\n")
	var desc []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "---") ||
			trimmed == "Parameters" || trimmed == "Returns" || trimmed == "Raises" {
			break
		}
		desc = append(desc, trimmed)
	}
	if len(desc) > 0 {
		result.WriteString(strings.Join(desc, " "))
	}

	// Extract Parameters section
	paramsMatch := regexp.MustCompile(`(?is)Parameters\s*\n-+\n(.*?)(?:\n\n|\n[A-Z]|\z)`).FindStringSubmatch(doc)
	if len(paramsMatch) > 1 {
		params := parseNumpyParams(paramsMatch[1])
		if params != "" {
			if result.Len() > 0 {
				result.WriteString(" | ")
			}
			result.WriteString("Params: ")
			result.WriteString(params)
		}
	}

	// Extract Returns section
	returnsMatch := regexp.MustCompile(`(?is)Returns\s*\n-+\n(.*?)(?:\n\n|\n[A-Z]|\z)`).FindStringSubmatch(doc)
	if len(returnsMatch) > 1 {
		returnType := extractNumpyReturnType(returnsMatch[1])
		if returnType != "" {
			if result.Len() > 0 {
				result.WriteString(" | ")
			}
			result.WriteString("Returns: ")
			result.WriteString(returnType)
		}
	}

	if result.Len() == 0 {
		return strings.TrimSpace(doc)
	}

	return result.String()
}

// parseNumpyParams extracts parameter info from NumPy-style Parameters section.
func parseNumpyParams(params string) string {
	var results []string
	// NumPy format: param_name : type
	paramPattern := regexp.MustCompile(`(?m)^(\w+)\s*:\s*(.+?)(?:\n|$)`)
	matches := paramPattern.FindAllStringSubmatch(params, -1)

	for _, m := range matches {
		if len(m) >= 3 {
			results = append(results, m[1]+": "+strings.TrimSpace(m[2]))
		}
	}

	return strings.Join(results, ", ")
}

// extractNumpyReturnType extracts return type from NumPy Returns section.
func extractNumpyReturnType(returns string) string {
	lines := strings.Split(returns, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && !strings.HasPrefix(trimmed, " ") {
			// First non-indented line is typically the type
			return trimmed
		}
	}
	return ""
}

// parseSphinxDocstring parses Sphinx/reST-style docstrings.
// Works with both newline-separated and space-collapsed formats.
func parseSphinxDocstring(doc string) string {
	var result strings.Builder

	// Extract description (everything before first :directive)
	// Find position of first directive
	firstDirective := regexp.MustCompile(`:\w+\s+\w+:`).FindStringIndex(doc)
	if firstDirective != nil && firstDirective[0] > 0 {
		desc := strings.TrimSpace(doc[:firstDirective[0]])
		if desc != "" {
			result.WriteString(desc)
		}
	} else if !strings.HasPrefix(strings.TrimSpace(doc), ":") {
		// No directive at start, try to extract first sentence
		parts := strings.SplitN(doc, ":param", 2)
		if len(parts) > 0 {
			desc := strings.TrimSpace(parts[0])
			if desc != "" {
				result.WriteString(desc)
			}
		}
	}

	// Extract parameters - look for :param name: and :type name: pairs
	var params []string
	paramPattern := regexp.MustCompile(`:param\s+(\w+):`)
	typePattern := regexp.MustCompile(`:type\s+(\w+):\s*(\w+)`)

	typeMap := make(map[string]string)
	typeMatches := typePattern.FindAllStringSubmatch(doc, -1)
	for _, m := range typeMatches {
		if len(m) >= 3 {
			typeMap[m[1]] = strings.TrimSpace(m[2])
		}
	}

	paramMatches := paramPattern.FindAllStringSubmatch(doc, -1)
	seenParams := make(map[string]bool)
	for _, m := range paramMatches {
		if len(m) >= 2 {
			name := m[1]
			if seenParams[name] {
				continue
			}
			seenParams[name] = true
			if ptype, ok := typeMap[name]; ok {
				params = append(params, name+": "+ptype)
			} else {
				params = append(params, name)
			}
		}
	}

	if len(params) > 0 {
		if result.Len() > 0 {
			result.WriteString(" | ")
		}
		result.WriteString("Params: ")
		result.WriteString(strings.Join(params, ", "))
	}

	// Extract return type
	rtypeMatch := regexp.MustCompile(`:rtype:\s*(\w+)`).FindStringSubmatch(doc)
	if len(rtypeMatch) > 1 {
		if result.Len() > 0 {
			result.WriteString(" | ")
		}
		result.WriteString("Returns: ")
		result.WriteString(strings.TrimSpace(rtypeMatch[1]))
	} else {
		// Try :returns: format - get text until next directive or end
		returnsMatch := regexp.MustCompile(`:returns?:\s*([^:]+?)(?::\w+|$)`).FindStringSubmatch(doc)
		if len(returnsMatch) > 1 {
			returnText := strings.TrimSpace(returnsMatch[1])
			if returnText != "" {
				if result.Len() > 0 {
					result.WriteString(" | ")
				}
				result.WriteString("Returns: ")
				result.WriteString(returnText)
			}
		}
	}

	// Extract raises - look for :raises ExceptionType:
	raisesMatch := regexp.MustCompile(`:raises?\s+(\w+):`).FindAllStringSubmatch(doc, -1)
	if len(raisesMatch) > 0 {
		var exceptions []string
		seenExceptions := make(map[string]bool)
		for _, m := range raisesMatch {
			if len(m) >= 2 {
				exc := m[1]
				if !seenExceptions[exc] {
					exceptions = append(exceptions, exc)
					seenExceptions[exc] = true
				}
			}
		}
		if len(exceptions) > 0 {
			if result.Len() > 0 {
				result.WriteString(" | ")
			}
			result.WriteString("Raises: ")
			result.WriteString(strings.Join(exceptions, ", "))
		}
	}

	if result.Len() == 0 {
		return strings.TrimSpace(doc)
	}

	return result.String()
}

// extractExceptionNames extracts exception class names from a Raises section.
func extractExceptionNames(raises string) string {
	var exceptions []string
	// Pattern matches exception names at the start of lines
	exceptionPattern := regexp.MustCompile(`(?m)^\s*(\w+(?:Error|Exception|Warning)?):?`)
	matches := exceptionPattern.FindAllStringSubmatch(raises, -1)

	for _, m := range matches {
		if len(m) >= 2 {
			name := m[1]
			// Filter to only exception-like names
			if strings.HasSuffix(name, "Error") || strings.HasSuffix(name, "Exception") ||
				strings.HasSuffix(name, "Warning") || name == "ValueError" || name == "TypeError" ||
				name == "KeyError" || name == "IndexError" || name == "RuntimeError" {
				exceptions = append(exceptions, name)
			}
		}
	}

	return strings.Join(exceptions, ", ")
}

func findPyBlockEnd(lines [][]byte, startLineIdx, baseIndent int) int {
	if startLineIdx >= len(lines) {
		return startLineIdx + 1
	}

	for i := startLineIdx + 1; i < len(lines); i++ {
		line := lines[i]
		if len(bytes.TrimSpace(line)) == 0 {
			continue // Skip blank lines
		}

		currentIndent := countLeadingSpaces(line)
		if currentIndent <= baseIndent && len(bytes.TrimSpace(line)) > 0 {
			return i // Return 0-indexed line where block ends
		}
	}

	return len(lines)
}

func countLeadingSpaces(line []byte) int {
	count := 0
	for _, b := range line {
		switch b {
		case ' ':
			count++
		case '\t':
			count += 4 // Treat tab as 4 spaces
		default:
			return count
		}
	}
	return count
}

func pyVisibility(name string, decorators []string) string {
	// Check for private naming conventions
	if strings.HasPrefix(name, "__") && !strings.HasSuffix(name, "__") {
		return "private"
	}
	if strings.HasPrefix(name, "_") {
		return "protected"
	}

	// Check for private decorators
	for _, dec := range decorators {
		if dec == "private" || strings.HasSuffix(dec, ".private") {
			return "private"
		}
	}

	return "public"
}

func (p *PythonParser) findDecorators(content []byte, defPos int, lineStarts []int) []string {
	var decorators []string
	defLine := findLineNumber(defPos, lineStarts)

	// Look backwards from the definition to find decorators
	matches := pyDecoratorPattern.FindAllSubmatchIndex(content[:defPos], -1)
	for _, match := range matches {
		if len(match) < 6 {
			continue
		}
		decLine := findLineNumber(match[0], lineStarts)
		// Decorator should be on the line(s) immediately before the definition
		if decLine >= defLine-5 && decLine < defLine {
			decorators = append(decorators, string(content[match[4]:match[5]]))
		}
	}

	return decorators
}

func (p *PythonParser) isClassMethod(content []byte, pos int, indent string) bool {
	// Look backwards to see if we're inside a class definition
	// This is a simplified check - we look for a class definition with less indentation
	classPattern := regexp.MustCompile(`(?m)^class\s+\w+`)
	matches := classPattern.FindAllIndex(content[:pos], -1)

	// If there's a class before this position and we're indented, it's likely a method
	return len(matches) > 0 && len(indent) > 0
}

func cleanPyParams(params string) string {
	parts := strings.Split(params, ",")
	var cleaned []string

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "self" || part == "cls" || strings.HasPrefix(part, "self:") || strings.HasPrefix(part, "cls:") {
			continue
		}
		cleaned = append(cleaned, part)
	}

	return strings.Join(cleaned, ", ")
}

// Ensure PythonParser implements LanguageParser at compile time.
var _ LanguageParser = (*PythonParser)(nil)
