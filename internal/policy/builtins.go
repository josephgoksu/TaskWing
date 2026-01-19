package policy

import (
	"bufio"
	"context"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/rego"
	"github.com/open-policy-agent/opa/v1/types"
	"github.com/spf13/afero"

	"github.com/josephgoksu/TaskWing/internal/codeintel"
)

// BuiltinContext holds dependencies for custom OPA built-ins.
// This enables testability by allowing injection of mock filesystem and codeintel.
type BuiltinContext struct {
	// WorkDir is the working directory for resolving relative paths.
	WorkDir string
	// Fs is the filesystem abstraction (use afero.NewOsFs() for real, afero.NewMemMapFs() for tests).
	Fs afero.Fs
	// CodeIntel is the optional code intelligence repository for symbol queries.
	// Can be nil if code intelligence is not available.
	CodeIntel codeintel.Repository
}

// NewBuiltinContext creates a new context with the OS filesystem.
func NewBuiltinContext(workDir string) *BuiltinContext {
	return &BuiltinContext{
		WorkDir: workDir,
		Fs:      afero.NewOsFs(),
	}
}

// NewBuiltinContextWithCodeIntel creates a context with code intelligence support.
func NewBuiltinContextWithCodeIntel(workDir string, repo codeintel.Repository) *BuiltinContext {
	return &BuiltinContext{
		WorkDir:   workDir,
		Fs:        afero.NewOsFs(),
		CodeIntel: repo,
	}
}

// resolvePath converts a relative path to absolute using the work directory.
func (bc *BuiltinContext) resolvePath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(bc.WorkDir, path)
}

// RegisterBuiltins registers all TaskWing custom built-ins with OPA.
// Call this before creating a rego.New() query.
// Returns the list of registered function names for reference.
func RegisterBuiltins(ctx *BuiltinContext) []string {
	registered := []string{}

	// taskwing.file_line_count(path) -> number
	// Returns the number of lines in a file, or -1 if file doesn't exist.
	fileLineCount := &rego.Function{
		Name: "taskwing.file_line_count",
		Decl: types.NewFunction(
			types.Args(types.S), // string path
			types.N,             // returns number
		),
		Memoize: true,
	}
	rego.RegisterBuiltin1(fileLineCount, func(_ rego.BuiltinContext, a *ast.Term) (*ast.Term, error) {
		path, ok := a.Value.(ast.String)
		if !ok {
			return ast.IntNumberTerm(-1), nil
		}
		count := fileLineCountImpl(ctx, string(path))
		return ast.IntNumberTerm(count), nil
	})
	registered = append(registered, "taskwing.file_line_count")

	// taskwing.has_pattern(path, pattern) -> boolean
	// Returns true if the file contains text matching the regex pattern.
	hasPattern := &rego.Function{
		Name: "taskwing.has_pattern",
		Decl: types.NewFunction(
			types.Args(types.S, types.S), // path, pattern
			types.B,                      // returns boolean
		),
		Memoize: true,
	}
	rego.RegisterBuiltin2(hasPattern, func(_ rego.BuiltinContext, a, b *ast.Term) (*ast.Term, error) {
		path, ok1 := a.Value.(ast.String)
		pattern, ok2 := b.Value.(ast.String)
		if !ok1 || !ok2 {
			return ast.BooleanTerm(false), nil
		}
		result := hasPatternImpl(ctx, string(path), string(pattern))
		return ast.BooleanTerm(result), nil
	})
	registered = append(registered, "taskwing.has_pattern")

	// taskwing.file_imports(path) -> array<string>
	// Returns the list of imports in a file (Go imports, JS requires, etc.)
	// Returns empty array if file doesn't exist or has no imports.
	fileImports := &rego.Function{
		Name: "taskwing.file_imports",
		Decl: types.NewFunction(
			types.Args(types.S),          // path
			types.NewArray(nil, types.S), // returns array of strings
		),
		Memoize: true,
	}
	rego.RegisterBuiltin1(fileImports, func(_ rego.BuiltinContext, a *ast.Term) (*ast.Term, error) {
		path, ok := a.Value.(ast.String)
		if !ok {
			return ast.ArrayTerm(), nil
		}
		imports := fileImportsImpl(ctx, string(path))
		terms := make([]*ast.Term, len(imports))
		for i, imp := range imports {
			terms[i] = ast.StringTerm(imp)
		}
		return ast.ArrayTerm(terms...), nil
	})
	registered = append(registered, "taskwing.file_imports")

	// taskwing.symbol_exists(path, symbol_name) -> boolean
	// Returns true if a symbol with the given name exists in the file.
	// Uses codeintel if available, otherwise falls back to simple text search.
	symbolExists := &rego.Function{
		Name: "taskwing.symbol_exists",
		Decl: types.NewFunction(
			types.Args(types.S, types.S), // path, symbol_name
			types.B,                      // returns boolean
		),
		Memoize: true,
	}
	rego.RegisterBuiltin2(symbolExists, func(_ rego.BuiltinContext, a, b *ast.Term) (*ast.Term, error) {
		path, ok1 := a.Value.(ast.String)
		symbolName, ok2 := b.Value.(ast.String)
		if !ok1 || !ok2 {
			return ast.BooleanTerm(false), nil
		}
		result := symbolExistsImpl(ctx, string(path), string(symbolName))
		return ast.BooleanTerm(result), nil
	})
	registered = append(registered, "taskwing.symbol_exists")

	// taskwing.file_exists(path) -> boolean
	// Returns true if the file exists.
	fileExists := &rego.Function{
		Name: "taskwing.file_exists",
		Decl: types.NewFunction(
			types.Args(types.S), // path
			types.B,             // returns boolean
		),
		Memoize: true,
	}
	rego.RegisterBuiltin1(fileExists, func(_ rego.BuiltinContext, a *ast.Term) (*ast.Term, error) {
		path, ok := a.Value.(ast.String)
		if !ok {
			return ast.BooleanTerm(false), nil
		}
		result := fileExistsImpl(ctx, string(path))
		return ast.BooleanTerm(result), nil
	})
	registered = append(registered, "taskwing.file_exists")

	return registered
}

// === Built-in Implementations ===

// fileLineCountImpl returns the number of lines in a file, or -1 if it doesn't exist.
func fileLineCountImpl(ctx *BuiltinContext, path string) int {
	fullPath := ctx.resolvePath(path)

	file, err := ctx.Fs.Open(fullPath)
	if err != nil {
		return -1 // File doesn't exist or can't be read
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	count := 0
	for scanner.Scan() {
		count++
	}

	if err := scanner.Err(); err != nil {
		return -1
	}

	return count
}

// hasPatternImpl returns true if the file contains text matching the regex pattern.
func hasPatternImpl(ctx *BuiltinContext, path, pattern string) bool {
	fullPath := ctx.resolvePath(path)

	// Compile the regex
	re, err := regexp.Compile(pattern)
	if err != nil {
		return false // Invalid regex
	}

	content, err := afero.ReadFile(ctx.Fs, fullPath)
	if err != nil {
		return false // File doesn't exist or can't be read
	}

	return re.Match(content)
}

// fileImportsImpl extracts imports from a file.
// Currently supports Go import statements.
func fileImportsImpl(ctx *BuiltinContext, path string) []string {
	fullPath := ctx.resolvePath(path)

	content, err := afero.ReadFile(ctx.Fs, fullPath)
	if err != nil {
		return []string{} // File doesn't exist or can't be read
	}

	// First try codeintel if available
	if ctx.CodeIntel != nil {
		symbols, err := ctx.CodeIntel.FindSymbolsByFile(context.Background(), path)
		if err == nil && len(symbols) > 0 {
			// Look for package symbols that might have import information
			// This is a simplification - full implementation would parse AST
			var imports []string
			for _, sym := range symbols {
				if sym.Kind == codeintel.SymbolPackage {
					imports = append(imports, sym.Name)
				}
			}
			if len(imports) > 0 {
				return imports
			}
		}
	}

	// Fallback: Parse imports directly from file content
	return parseGoImports(string(content))
}

// parseGoImports extracts import paths from Go source code.
func parseGoImports(content string) []string {
	var imports []string

	// Simple regex-based extraction for Go imports
	// Handles both single imports and import blocks
	singleImport := regexp.MustCompile(`(?m)^import\s+"([^"]+)"`)
	blockImport := regexp.MustCompile(`(?s)import\s*\(\s*([^)]+)\s*\)`)
	importLine := regexp.MustCompile(`"([^"]+)"`)

	// Single imports
	for _, match := range singleImport.FindAllStringSubmatch(content, -1) {
		if len(match) > 1 {
			imports = append(imports, match[1])
		}
	}

	// Import blocks
	for _, block := range blockImport.FindAllStringSubmatch(content, -1) {
		if len(block) > 1 {
			for _, line := range importLine.FindAllStringSubmatch(block[1], -1) {
				if len(line) > 1 {
					imports = append(imports, line[1])
				}
			}
		}
	}

	return imports
}

// symbolExistsImpl checks if a symbol with the given name exists in the file.
func symbolExistsImpl(ctx *BuiltinContext, path, symbolName string) bool {
	// First try codeintel if available
	if ctx.CodeIntel != nil {
		symbols, err := ctx.CodeIntel.FindSymbolsByFile(context.Background(), path)
		if err == nil {
			for _, sym := range symbols {
				if sym.Name == symbolName {
					return true
				}
			}
		}
		// If codeintel returned results but didn't find the symbol, trust it
		if err == nil {
			return false
		}
	}

	// Fallback: Simple text-based search
	fullPath := ctx.resolvePath(path)
	content, err := afero.ReadFile(ctx.Fs, fullPath)
	if err != nil {
		return false
	}

	// Look for common symbol definition patterns
	patterns := []string{
		`\bfunc\s+` + regexp.QuoteMeta(symbolName) + `\s*\(`,             // Go function
		`\bfunc\s*\([^)]*\)\s*` + regexp.QuoteMeta(symbolName) + `\s*\(`, // Go method
		`\btype\s+` + regexp.QuoteMeta(symbolName) + `\s+`,               // Go type
		`\bvar\s+` + regexp.QuoteMeta(symbolName) + `\b`,                 // Go var
		`\bconst\s+` + regexp.QuoteMeta(symbolName) + `\b`,               // Go const
		`\bclass\s+` + regexp.QuoteMeta(symbolName) + `\b`,               // JS/TS class
		`\bfunction\s+` + regexp.QuoteMeta(symbolName) + `\s*\(`,         // JS function
		`\bdef\s+` + regexp.QuoteMeta(symbolName) + `\s*\(`,              // Python function
	}

	for _, pattern := range patterns {
		re, err := regexp.Compile(pattern)
		if err != nil {
			continue
		}
		if re.Match(content) {
			return true
		}
	}

	return false
}

// fileExistsImpl checks if a file exists.
func fileExistsImpl(ctx *BuiltinContext, path string) bool {
	fullPath := ctx.resolvePath(path)
	exists, _ := afero.Exists(ctx.Fs, fullPath)
	return exists
}

// GetBuiltinNames returns the list of available TaskWing built-in function names.
func GetBuiltinNames() []string {
	return []string{
		"taskwing.file_line_count",
		"taskwing.has_pattern",
		"taskwing.file_imports",
		"taskwing.symbol_exists",
		"taskwing.file_exists",
	}
}

// IsBuiltin returns true if the given name is a TaskWing built-in function.
func IsBuiltin(name string) bool {
	for _, bn := range GetBuiltinNames() {
		if bn == name || strings.HasSuffix(bn, "."+name) {
			return true
		}
	}
	return false
}
