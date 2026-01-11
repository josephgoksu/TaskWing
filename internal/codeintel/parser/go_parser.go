// Package parser provides Go source code parsing for symbol extraction.
// It uses go/ast, go/parser, and go/types to extract symbols and relationships.
package parser

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"golang.org/x/tools/go/packages"
)

// SymbolKind represents the type of code symbol.
type SymbolKind string

const (
	SymbolFunction  SymbolKind = "function"
	SymbolMethod    SymbolKind = "method"
	SymbolStruct    SymbolKind = "struct"
	SymbolInterface SymbolKind = "interface"
	SymbolType      SymbolKind = "type"
	SymbolVariable  SymbolKind = "variable"
	SymbolConstant  SymbolKind = "constant"
	SymbolField     SymbolKind = "field"
	SymbolPackage   SymbolKind = "package"
)

// RelationType defines the kind of relationship between symbols.
type RelationType string

const (
	RelationCalls      RelationType = "calls"
	RelationImplements RelationType = "implements"
	RelationExtends    RelationType = "extends"
	RelationUses       RelationType = "uses"
	RelationDefines    RelationType = "defines"
	RelationReferences RelationType = "references"
)

// Symbol represents a code symbol extracted from parsing.
type Symbol struct {
	ID           uint32     `json:"id"`
	Name         string     `json:"name"`
	Kind         SymbolKind `json:"kind"`
	FilePath     string     `json:"filePath"`
	StartLine    int        `json:"startLine"`
	EndLine      int        `json:"endLine"`
	Signature    string     `json:"signature,omitempty"`
	DocComment   string     `json:"docComment,omitempty"`
	ModulePath   string     `json:"modulePath,omitempty"`
	Visibility   string     `json:"visibility"`
	Language     string     `json:"language"`
	FileHash     string     `json:"fileHash,omitempty"`
	Embedding    []float32  `json:"embedding,omitempty"`
	LastModified time.Time  `json:"lastModified"`
}

// SymbolRelation represents a relationship between two symbols.
type SymbolRelation struct {
	FromSymbolID uint32         `json:"fromSymbolId"`
	ToSymbolID   uint32         `json:"toSymbolId"`
	RelationType RelationType   `json:"relationType"`
	CallSiteLine int            `json:"callSiteLine,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
}

// GoParser extracts symbols and relationships from Go source files.
type GoParser struct {
	fset     *token.FileSet
	basePath string // Root path for relative file paths
}

// NewGoParser creates a new Go parser instance.
func NewGoParser(basePath string) *GoParser {
	return &GoParser{
		fset:     token.NewFileSet(),
		basePath: basePath,
	}
}

// SupportedExtensions returns the file extensions this parser can handle.
// GoParser handles .go files only.
func (p *GoParser) SupportedExtensions() []string {
	return []string{".go"}
}

// Language returns the language identifier for this parser.
func (p *GoParser) Language() string {
	return "go"
}

// CanParse returns true if this parser can handle the given file path.
func (p *GoParser) CanParse(filePath string) bool {
	ext := strings.ToLower(filepath.Ext(filePath))
	return ext == ".go"
}

// ParseResult contains the symbols and relations extracted from parsing.
type ParseResult struct {
	Symbols   []Symbol
	Relations []SymbolRelation
	Errors    []error
}

// ParseFile parses a single Go source file and extracts symbols.
func (p *GoParser) ParseFile(filePath string) (*ParseResult, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading file: %w", err)
	}

	file, err := parser.ParseFile(p.fset, filePath, content, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parsing file: %w", err)
	}

	result := &ParseResult{
		Symbols:   make([]Symbol, 0),
		Relations: make([]SymbolRelation, 0),
	}

	fileHash := ComputeHash(content)
	relPath := p.relativePath(filePath)
	modulePath := extractModulePath(relPath)

	// Extract package-level symbols
	p.extractSymbols(file, relPath, fileHash, modulePath, result)

	return result, nil
}

// ParseDirectory parses all Go files in a directory recursively.
func (p *GoParser) ParseDirectory(dirPath string) (*ParseResult, error) {
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
			if strings.HasPrefix(name, ".") || name == "vendor" || name == "testdata" || name == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}

		// Only process .go files, skip test files for now
		if !strings.HasSuffix(path, ".go") {
			return nil
		}

		result, err := p.ParseFile(path)
		if err != nil {
			combined.Errors = append(combined.Errors, fmt.Errorf("%s: %w", path, err))
			return nil // Continue with other files
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

// ParsePackageWithTypes parses a package with full type information.
// This enables interface implementation detection and accurate call graph analysis.
func (p *GoParser) ParsePackageWithTypes(pkgPath string) (*ParseResult, error) {
	cfg := &packages.Config{
		Mode: packages.NeedName |
			packages.NeedFiles |
			packages.NeedSyntax |
			packages.NeedTypes |
			packages.NeedTypesInfo |
			packages.NeedImports,
		Fset: p.fset,
		Dir:  p.basePath,
	}

	pkgs, err := packages.Load(cfg, pkgPath)
	if err != nil {
		return nil, fmt.Errorf("loading package: %w", err)
	}

	result := &ParseResult{
		Symbols:   make([]Symbol, 0),
		Relations: make([]SymbolRelation, 0),
	}

	// Track symbols by their unique key for relation building
	symbolMap := make(map[string]int) // key -> index in result.Symbols

	for _, pkg := range pkgs {
		if pkg.Errors != nil {
			for _, e := range pkg.Errors {
				result.Errors = append(result.Errors, e)
			}
		}

		for i, file := range pkg.Syntax {
			filePath := pkg.GoFiles[i]
			content, _ := os.ReadFile(filePath)
			fileHash := ComputeHash(content)
			relPath := p.relativePath(filePath)
			modulePath := extractModulePath(relPath)

			p.extractTypedSymbols(file, pkg, relPath, fileHash, modulePath, result, symbolMap)
		}
	}

	return result, nil
}

// extractSymbols extracts symbols from an AST file without type info.
func (p *GoParser) extractSymbols(file *ast.File, filePath, fileHash, modulePath string, result *ParseResult) {
	now := time.Now()

	// Package declaration
	result.Symbols = append(result.Symbols, Symbol{
		Name:         file.Name.Name,
		Kind:         SymbolPackage,
		FilePath:     filePath,
		StartLine:    p.fset.Position(file.Package).Line,
		EndLine:      p.fset.Position(file.Package).Line,
		ModulePath:   modulePath,
		Visibility:   "public",
		Language:     "go",
		FileHash:     fileHash,
		LastModified: now,
	})

	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			p.extractFuncDecl(d, filePath, fileHash, modulePath, now, result)
		case *ast.GenDecl:
			p.extractGenDecl(d, filePath, fileHash, modulePath, now, result)
		}
	}
}

// extractFuncDecl extracts a function or method declaration.
func (p *GoParser) extractFuncDecl(d *ast.FuncDecl, filePath, fileHash, modulePath string, now time.Time, result *ParseResult) {
	kind := SymbolFunction
	if d.Recv != nil {
		kind = SymbolMethod
	}

	sig := buildFuncSignature(d)
	docComment := extractDocComment(d.Doc)

	sym := Symbol{
		Name:         d.Name.Name,
		Kind:         kind,
		FilePath:     filePath,
		StartLine:    p.fset.Position(d.Pos()).Line,
		EndLine:      p.fset.Position(d.End()).Line,
		Signature:    sig,
		DocComment:   docComment,
		ModulePath:   modulePath,
		Visibility:   visibility(d.Name.Name),
		Language:     "go",
		FileHash:     fileHash,
		LastModified: now,
	}

	result.Symbols = append(result.Symbols, sym)

	// Extract call relations within the function body
	if d.Body != nil {
		p.extractCallsFromBlock(d.Body, len(result.Symbols)-1, result)
	}
}

// extractGenDecl extracts type, const, and var declarations.
func (p *GoParser) extractGenDecl(d *ast.GenDecl, filePath, fileHash, modulePath string, now time.Time, result *ParseResult) {
	docComment := extractDocComment(d.Doc)

	for _, spec := range d.Specs {
		switch s := spec.(type) {
		case *ast.TypeSpec:
			p.extractTypeSpec(s, d, filePath, fileHash, modulePath, docComment, now, result)
		case *ast.ValueSpec:
			p.extractValueSpec(s, d, filePath, fileHash, modulePath, docComment, now, result)
		}
	}
}

// extractTypeSpec extracts struct, interface, and type alias declarations.
func (p *GoParser) extractTypeSpec(s *ast.TypeSpec, d *ast.GenDecl, filePath, fileHash, modulePath, parentDoc string, now time.Time, result *ParseResult) {
	var kind SymbolKind
	var sig string

	switch t := s.Type.(type) {
	case *ast.StructType:
		kind = SymbolStruct
		sig = buildStructSignature(s.Name.Name, t)
		// Extract struct fields
		p.extractStructFields(t, s.Name.Name, filePath, fileHash, modulePath, now, result)
	case *ast.InterfaceType:
		kind = SymbolInterface
		sig = buildInterfaceSignature(s.Name.Name, t)
	default:
		kind = SymbolType
		sig = fmt.Sprintf("type %s", s.Name.Name)
	}

	doc := extractDocComment(s.Doc)
	if doc == "" {
		doc = parentDoc
	}

	result.Symbols = append(result.Symbols, Symbol{
		Name:         s.Name.Name,
		Kind:         kind,
		FilePath:     filePath,
		StartLine:    p.fset.Position(s.Pos()).Line,
		EndLine:      p.fset.Position(s.End()).Line,
		Signature:    sig,
		DocComment:   doc,
		ModulePath:   modulePath,
		Visibility:   visibility(s.Name.Name),
		Language:     "go",
		FileHash:     fileHash,
		LastModified: now,
	})
}

// extractStructFields extracts field declarations from a struct.
func (p *GoParser) extractStructFields(t *ast.StructType, structName, filePath, fileHash, modulePath string, now time.Time, result *ParseResult) {
	if t.Fields == nil {
		return
	}

	for _, field := range t.Fields.List {
		for _, name := range field.Names {
			result.Symbols = append(result.Symbols, Symbol{
				Name:         name.Name,
				Kind:         SymbolField,
				FilePath:     filePath,
				StartLine:    p.fset.Position(field.Pos()).Line,
				EndLine:      p.fset.Position(field.End()).Line,
				Signature:    fmt.Sprintf("%s.%s %s", structName, name.Name, exprToString(field.Type)),
				DocComment:   extractDocComment(field.Doc),
				ModulePath:   modulePath,
				Visibility:   visibility(name.Name),
				Language:     "go",
				FileHash:     fileHash,
				LastModified: now,
			})
		}
	}
}

// extractValueSpec extracts const and var declarations.
func (p *GoParser) extractValueSpec(s *ast.ValueSpec, d *ast.GenDecl, filePath, fileHash, modulePath, parentDoc string, now time.Time, result *ParseResult) {
	kind := SymbolVariable
	if d.Tok == token.CONST {
		kind = SymbolConstant
	}

	doc := extractDocComment(s.Doc)
	if doc == "" {
		doc = parentDoc
	}

	for _, name := range s.Names {
		var sig string
		if s.Type != nil {
			sig = exprToString(s.Type)
		}

		result.Symbols = append(result.Symbols, Symbol{
			Name:         name.Name,
			Kind:         kind,
			FilePath:     filePath,
			StartLine:    p.fset.Position(name.Pos()).Line,
			EndLine:      p.fset.Position(s.End()).Line,
			Signature:    sig,
			DocComment:   doc,
			ModulePath:   modulePath,
			Visibility:   visibility(name.Name),
			Language:     "go",
			FileHash:     fileHash,
			LastModified: now,
		})
	}
}

// extractCallsFromBlock extracts function calls from a block statement.
func (p *GoParser) extractCallsFromBlock(block *ast.BlockStmt, callerIdx int, result *ParseResult) {
	ast.Inspect(block, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		// Get the function name being called
		var calleeName string
		switch fn := call.Fun.(type) {
		case *ast.Ident:
			calleeName = fn.Name
		case *ast.SelectorExpr:
			calleeName = fn.Sel.Name
		default:
			return true
		}

		// Create a placeholder relation (target will be resolved later)
		rel := SymbolRelation{
			FromSymbolID: uint32(callerIdx), // Temporary index
			ToSymbolID:   0,                 // Will be resolved during indexing
			RelationType: RelationCalls,
			CallSiteLine: p.fset.Position(call.Pos()).Line,
			Metadata: map[string]any{
				"calleeName": calleeName,
			},
		}
		result.Relations = append(result.Relations, rel)

		return true
	})
}

// extractTypedSymbols extracts symbols with full type information.
func (p *GoParser) extractTypedSymbols(file *ast.File, pkg *packages.Package, filePath, fileHash, modulePath string, result *ParseResult, symbolMap map[string]int) {
	now := time.Now()

	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			p.extractTypedFuncDecl(d, pkg, filePath, fileHash, modulePath, now, result, symbolMap)
		case *ast.GenDecl:
			p.extractGenDecl(d, filePath, fileHash, modulePath, now, result)
		}
	}

	// Detect interface implementations using type info
	p.detectImplementations(pkg, result, symbolMap)
}

// extractTypedFuncDecl extracts a function with type-resolved call information.
func (p *GoParser) extractTypedFuncDecl(d *ast.FuncDecl, pkg *packages.Package, filePath, fileHash, modulePath string, now time.Time, result *ParseResult, symbolMap map[string]int) {
	kind := SymbolFunction
	receiverType := ""
	if d.Recv != nil {
		kind = SymbolMethod
		if len(d.Recv.List) > 0 {
			receiverType = exprToString(d.Recv.List[0].Type)
		}
	}

	sig := buildFuncSignature(d)
	docComment := extractDocComment(d.Doc)

	sym := Symbol{
		Name:         d.Name.Name,
		Kind:         kind,
		FilePath:     filePath,
		StartLine:    p.fset.Position(d.Pos()).Line,
		EndLine:      p.fset.Position(d.End()).Line,
		Signature:    sig,
		DocComment:   docComment,
		ModulePath:   modulePath,
		Visibility:   visibility(d.Name.Name),
		Language:     "go",
		FileHash:     fileHash,
		LastModified: now,
	}

	symIdx := len(result.Symbols)
	result.Symbols = append(result.Symbols, sym)

	// Build symbol key for lookup
	key := buildSymbolKey(modulePath, receiverType, d.Name.Name)
	symbolMap[key] = symIdx

	// Extract typed call relations
	if d.Body != nil && pkg.TypesInfo != nil {
		p.extractTypedCalls(d.Body, pkg, symIdx, result)
	}
}

// extractTypedCalls extracts call relations using type information.
func (p *GoParser) extractTypedCalls(block *ast.BlockStmt, pkg *packages.Package, callerIdx int, result *ParseResult) {
	ast.Inspect(block, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		// Resolve the function being called using type info
		var calleeName, calleePkg, receiverType string

		switch fn := call.Fun.(type) {
		case *ast.Ident:
			calleeName = fn.Name
			if obj := pkg.TypesInfo.Uses[fn]; obj != nil {
				if obj.Pkg() != nil {
					calleePkg = obj.Pkg().Path()
				}
			}
		case *ast.SelectorExpr:
			calleeName = fn.Sel.Name
			// Check if it's a method call on a type
			if sel, ok := pkg.TypesInfo.Selections[fn]; ok {
				if sel.Recv() != nil {
					receiverType = sel.Recv().String()
				}
			} else if obj := pkg.TypesInfo.Uses[fn.Sel]; obj != nil {
				if obj.Pkg() != nil {
					calleePkg = obj.Pkg().Path()
				}
			}
		default:
			return true
		}

		rel := SymbolRelation{
			FromSymbolID: uint32(callerIdx),
			ToSymbolID:   0, // Will be resolved during indexing
			RelationType: RelationCalls,
			CallSiteLine: p.fset.Position(call.Pos()).Line,
			Metadata: map[string]any{
				"calleeName":   calleeName,
				"calleePkg":    calleePkg,
				"receiverType": receiverType,
			},
		}
		result.Relations = append(result.Relations, rel)

		return true
	})
}

// detectImplementations finds which types implement which interfaces.
func (p *GoParser) detectImplementations(pkg *packages.Package, result *ParseResult, symbolMap map[string]int) {
	if pkg.Types == nil {
		return
	}

	scope := pkg.Types.Scope()

	// Collect all interfaces and concrete types
	var interfaces []*types.Named
	var concreteTypes []*types.Named

	for _, name := range scope.Names() {
		obj := scope.Lookup(name)
		if typeName, ok := obj.(*types.TypeName); ok {
			if named, ok := typeName.Type().(*types.Named); ok {
				underlying := named.Underlying()
				if _, ok := underlying.(*types.Interface); ok {
					interfaces = append(interfaces, named)
				} else if _, ok := underlying.(*types.Struct); ok {
					concreteTypes = append(concreteTypes, named)
				}
			}
		}
	}

	// Check each concrete type against each interface
	for _, concrete := range concreteTypes {
		for _, iface := range interfaces {
			ifaceType := iface.Underlying().(*types.Interface)

			// Check if concrete type implements interface (both value and pointer)
			concretePtr := types.NewPointer(concrete)
			if types.Implements(concrete, ifaceType) || types.Implements(concretePtr, ifaceType) {
				rel := SymbolRelation{
					FromSymbolID: 0, // Will be resolved during indexing
					ToSymbolID:   0, // Will be resolved during indexing
					RelationType: RelationImplements,
					Metadata: map[string]any{
						"implementor": concrete.Obj().Name(),
						"interface":   iface.Obj().Name(),
					},
				}
				result.Relations = append(result.Relations, rel)
			}
		}
	}
}

// Helper functions

// ComputeHash computes a SHA256 hash of the content.
func ComputeHash(content []byte) string {
	hash := sha256.Sum256(content)
	return hex.EncodeToString(hash[:])
}

func (p *GoParser) relativePath(absPath string) string {
	if p.basePath == "" {
		return absPath
	}
	rel, err := filepath.Rel(p.basePath, absPath)
	if err != nil {
		return absPath
	}
	return rel
}

func extractModulePath(filePath string) string {
	dir := filepath.Dir(filePath)
	if dir == "." || dir == "" {
		return ""
	}
	return dir
}

func visibility(name string) string {
	if len(name) > 0 && unicode.IsUpper(rune(name[0])) {
		return "public"
	}
	return "private"
}

func extractDocComment(doc *ast.CommentGroup) string {
	if doc == nil {
		return ""
	}
	return strings.TrimSpace(doc.Text())
}

func buildFuncSignature(d *ast.FuncDecl) string {
	var sb strings.Builder
	sb.WriteString("func")

	// Receiver
	if d.Recv != nil && len(d.Recv.List) > 0 {
		sb.WriteString(" (")
		for i, field := range d.Recv.List {
			if i > 0 {
				sb.WriteString(", ")
			}
			if len(field.Names) > 0 {
				sb.WriteString(field.Names[0].Name)
				sb.WriteString(" ")
			}
			sb.WriteString(exprToString(field.Type))
		}
		sb.WriteString(")")
	}

	sb.WriteString(" ")
	sb.WriteString(d.Name.Name)

	// Parameters
	sb.WriteString("(")
	if d.Type.Params != nil {
		for i, field := range d.Type.Params.List {
			if i > 0 {
				sb.WriteString(", ")
			}
			for j, name := range field.Names {
				if j > 0 {
					sb.WriteString(", ")
				}
				sb.WriteString(name.Name)
			}
			if len(field.Names) > 0 {
				sb.WriteString(" ")
			}
			sb.WriteString(exprToString(field.Type))
		}
	}
	sb.WriteString(")")

	// Results
	if d.Type.Results != nil && len(d.Type.Results.List) > 0 {
		if len(d.Type.Results.List) == 1 && len(d.Type.Results.List[0].Names) == 0 {
			sb.WriteString(" ")
			sb.WriteString(exprToString(d.Type.Results.List[0].Type))
		} else {
			sb.WriteString(" (")
			for i, field := range d.Type.Results.List {
				if i > 0 {
					sb.WriteString(", ")
				}
				for j, name := range field.Names {
					if j > 0 {
						sb.WriteString(", ")
					}
					sb.WriteString(name.Name)
				}
				if len(field.Names) > 0 {
					sb.WriteString(" ")
				}
				sb.WriteString(exprToString(field.Type))
			}
			sb.WriteString(")")
		}
	}

	return sb.String()
}

func buildStructSignature(name string, t *ast.StructType) string {
	var sb strings.Builder
	sb.WriteString("type ")
	sb.WriteString(name)
	sb.WriteString(" struct")

	if t.Fields != nil && len(t.Fields.List) > 0 {
		sb.WriteString(" { ")
		for i, field := range t.Fields.List {
			if i > 0 {
				sb.WriteString("; ")
			}
			for j, n := range field.Names {
				if j > 0 {
					sb.WriteString(", ")
				}
				sb.WriteString(n.Name)
			}
			if len(field.Names) > 0 {
				sb.WriteString(" ")
			}
			sb.WriteString(exprToString(field.Type))
		}
		sb.WriteString(" }")
	}

	return sb.String()
}

func buildInterfaceSignature(name string, t *ast.InterfaceType) string {
	var sb strings.Builder
	sb.WriteString("type ")
	sb.WriteString(name)
	sb.WriteString(" interface")

	if t.Methods != nil && len(t.Methods.List) > 0 {
		sb.WriteString(" { ")
		for i, method := range t.Methods.List {
			if i > 0 {
				sb.WriteString("; ")
			}
			if len(method.Names) > 0 {
				sb.WriteString(method.Names[0].Name)
				if ft, ok := method.Type.(*ast.FuncType); ok {
					sb.WriteString(funcTypeToString(ft))
				}
			} else {
				// Embedded interface
				sb.WriteString(exprToString(method.Type))
			}
		}
		sb.WriteString(" }")
	}

	return sb.String()
}

func funcTypeToString(ft *ast.FuncType) string {
	var sb strings.Builder
	sb.WriteString("(")
	if ft.Params != nil {
		for i, field := range ft.Params.List {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(exprToString(field.Type))
		}
	}
	sb.WriteString(")")

	if ft.Results != nil && len(ft.Results.List) > 0 {
		if len(ft.Results.List) == 1 {
			sb.WriteString(" ")
			sb.WriteString(exprToString(ft.Results.List[0].Type))
		} else {
			sb.WriteString(" (")
			for i, field := range ft.Results.List {
				if i > 0 {
					sb.WriteString(", ")
				}
				sb.WriteString(exprToString(field.Type))
			}
			sb.WriteString(")")
		}
	}

	return sb.String()
}

func exprToString(expr ast.Expr) string {
	if expr == nil {
		return ""
	}

	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.StarExpr:
		return "*" + exprToString(e.X)
	case *ast.SelectorExpr:
		return exprToString(e.X) + "." + e.Sel.Name
	case *ast.ArrayType:
		if e.Len != nil {
			return "[" + exprToString(e.Len) + "]" + exprToString(e.Elt)
		}
		return "[]" + exprToString(e.Elt)
	case *ast.MapType:
		return "map[" + exprToString(e.Key) + "]" + exprToString(e.Value)
	case *ast.ChanType:
		switch e.Dir {
		case ast.SEND:
			return "chan<- " + exprToString(e.Value)
		case ast.RECV:
			return "<-chan " + exprToString(e.Value)
		default:
			return "chan " + exprToString(e.Value)
		}
	case *ast.FuncType:
		return "func" + funcTypeToString(e)
	case *ast.InterfaceType:
		if e.Methods == nil || len(e.Methods.List) == 0 {
			return "interface{}"
		}
		return "interface{...}"
	case *ast.StructType:
		if e.Fields == nil || len(e.Fields.List) == 0 {
			return "struct{}"
		}
		return "struct{...}"
	case *ast.Ellipsis:
		return "..." + exprToString(e.Elt)
	case *ast.BasicLit:
		return e.Value
	default:
		return "?"
	}
}

func buildSymbolKey(modulePath, receiverType, name string) string {
	if receiverType != "" {
		return modulePath + ":" + receiverType + "." + name
	}
	return modulePath + ":" + name
}
