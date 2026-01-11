package parser

import (
	"os"
	"path/filepath"
	"testing"
)

// TestGoParser_ParseFile tests basic file parsing.
func TestGoParser_ParseFile(t *testing.T) {
	// Create a temporary Go file
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.go")

	content := `package main

import "fmt"

// User represents a user in the system.
type User struct {
	ID   int
	Name string
}

// GetName returns the user's name.
func (u *User) GetName() string {
	return u.Name
}

// CreateUser creates a new user.
func CreateUser(name string) *User {
	return &User{Name: name}
}

func main() {
	u := CreateUser("Alice")
	fmt.Println(u.GetName())
}
`
	err := os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	parser := NewGoParser(tmpDir)
	result, err := parser.ParseFile(filePath)
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	// Should have: package, User struct, ID field, Name field, GetName method, CreateUser func, main func
	if len(result.Symbols) < 5 {
		t.Errorf("Expected at least 5 symbols, got %d", len(result.Symbols))
	}

	// Verify we have the expected symbol types
	symbolsByKind := make(map[SymbolKind]int)
	for _, s := range result.Symbols {
		symbolsByKind[s.Kind]++
	}

	if symbolsByKind[SymbolPackage] != 1 {
		t.Errorf("Expected 1 package symbol, got %d", symbolsByKind[SymbolPackage])
	}
	if symbolsByKind[SymbolStruct] != 1 {
		t.Errorf("Expected 1 struct symbol, got %d", symbolsByKind[SymbolStruct])
	}
	if symbolsByKind[SymbolMethod] != 1 {
		t.Errorf("Expected 1 method symbol, got %d", symbolsByKind[SymbolMethod])
	}
	if symbolsByKind[SymbolFunction] < 2 {
		t.Errorf("Expected at least 2 function symbols, got %d", symbolsByKind[SymbolFunction])
	}
}

// TestGoParser_ExtractDocComments tests documentation extraction.
func TestGoParser_ExtractDocComments(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "doc.go")

	content := `package example

// Config holds configuration options.
// It supports multiple backends.
type Config struct {
	// Backend is the storage backend name.
	Backend string
}

// NewConfig creates a default config.
func NewConfig() *Config {
	return &Config{Backend: "sqlite"}
}
`
	err := os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	parser := NewGoParser(tmpDir)
	result, err := parser.ParseFile(filePath)
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	// Find the Config struct
	var configSym *Symbol
	for i := range result.Symbols {
		if result.Symbols[i].Name == "Config" && result.Symbols[i].Kind == SymbolStruct {
			configSym = &result.Symbols[i]
			break
		}
	}

	if configSym == nil {
		t.Fatal("Config struct not found")
	}

	if configSym.DocComment == "" {
		t.Error("Expected Config to have a doc comment")
	}

	if configSym.DocComment != "Config holds configuration options.\nIt supports multiple backends." {
		t.Errorf("Unexpected doc comment: %q", configSym.DocComment)
	}
}

// TestGoParser_ExtractSignatures tests function signature extraction.
func TestGoParser_ExtractSignatures(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "sig.go")

	content := `package example

func Simple() {}

func WithParams(a int, b string) {}

func WithReturn() int { return 0 }

func WithMultiReturn() (int, error) { return 0, nil }

func WithNamedReturn() (result int, err error) { return 0, nil }

func (r *Receiver) Method() {}

type Receiver struct{}
`
	err := os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	parser := NewGoParser(tmpDir)
	result, err := parser.ParseFile(filePath)
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	signatures := make(map[string]string)
	for _, s := range result.Symbols {
		if s.Kind == SymbolFunction || s.Kind == SymbolMethod {
			signatures[s.Name] = s.Signature
		}
	}

	testCases := []struct {
		name     string
		expected string
	}{
		{"Simple", "func Simple()"},
		{"WithParams", "func WithParams(a int, b string)"},
		{"WithReturn", "func WithReturn() int"},
		{"WithMultiReturn", "func WithMultiReturn() (int, error)"},
		{"WithNamedReturn", "func WithNamedReturn() (result int, err error)"},
		{"Method", "func (r *Receiver) Method()"},
	}

	for _, tc := range testCases {
		sig, ok := signatures[tc.name]
		if !ok {
			t.Errorf("Function %s not found", tc.name)
			continue
		}
		if sig != tc.expected {
			t.Errorf("Signature for %s:\n  got:      %q\n  expected: %q", tc.name, sig, tc.expected)
		}
	}
}

// TestGoParser_ExtractInterfaces tests interface extraction.
func TestGoParser_ExtractInterfaces(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "iface.go")

	content := `package example

// Reader is an interface for reading.
type Reader interface {
	Read(p []byte) (n int, err error)
}

// Writer is an interface for writing.
type Writer interface {
	Write(p []byte) (n int, err error)
}

// ReadWriter combines Reader and Writer.
type ReadWriter interface {
	Reader
	Writer
}
`
	err := os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	parser := NewGoParser(tmpDir)
	result, err := parser.ParseFile(filePath)
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	interfaces := make(map[string]Symbol)
	for _, s := range result.Symbols {
		if s.Kind == SymbolInterface {
			interfaces[s.Name] = s
		}
	}

	if len(interfaces) != 3 {
		t.Errorf("Expected 3 interfaces, got %d", len(interfaces))
	}

	for _, name := range []string{"Reader", "Writer", "ReadWriter"} {
		if _, ok := interfaces[name]; !ok {
			t.Errorf("Interface %s not found", name)
		}
	}

	// Check visibility
	for name, iface := range interfaces {
		if iface.Visibility != "public" {
			t.Errorf("Interface %s should be public, got %s", name, iface.Visibility)
		}
	}
}

// TestGoParser_ExtractCallRelations tests call site extraction.
func TestGoParser_ExtractCallRelations(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "calls.go")

	content := `package example

func helper() int {
	return 42
}

func caller() {
	x := helper()
	_ = x
}
`
	err := os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	parser := NewGoParser(tmpDir)
	result, err := parser.ParseFile(filePath)
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	// Should have at least one call relation
	callRelations := 0
	for _, rel := range result.Relations {
		if rel.RelationType == RelationCalls {
			callRelations++
			// Verify metadata contains callee name
			if callee, ok := rel.Metadata["calleeName"].(string); !ok || callee == "" {
				t.Error("Call relation missing calleeName in metadata")
			}
		}
	}

	if callRelations == 0 {
		t.Error("Expected at least one call relation")
	}
}

// TestGoParser_ExtractConstants tests constant extraction.
func TestGoParser_ExtractConstants(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "const.go")

	content := `package example

// MaxSize is the maximum size.
const MaxSize = 1024

const (
	// StatusOK indicates success.
	StatusOK = 200
	// StatusError indicates failure.
	StatusError = 500
)

var defaultName = "example"
`
	err := os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	parser := NewGoParser(tmpDir)
	result, err := parser.ParseFile(filePath)
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	constants := make(map[string]Symbol)
	variables := make(map[string]Symbol)

	for _, s := range result.Symbols {
		if s.Kind == SymbolConstant {
			constants[s.Name] = s
		}
		if s.Kind == SymbolVariable {
			variables[s.Name] = s
		}
	}

	if len(constants) != 3 {
		t.Errorf("Expected 3 constants, got %d", len(constants))
	}

	if len(variables) != 1 {
		t.Errorf("Expected 1 variable, got %d", len(variables))
	}

	// Check visibility
	if constants["MaxSize"].Visibility != "public" {
		t.Error("MaxSize should be public")
	}
	if variables["defaultName"].Visibility != "private" {
		t.Error("defaultName should be private")
	}
}

// TestGoParser_ExtractFields tests struct field extraction.
func TestGoParser_ExtractFields(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "fields.go")

	content := `package example

type Config struct {
	// Host is the server host.
	Host string
	// Port is the server port.
	Port int
	timeout int // private field
}
`
	err := os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	parser := NewGoParser(tmpDir)
	result, err := parser.ParseFile(filePath)
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	fields := make(map[string]Symbol)
	for _, s := range result.Symbols {
		if s.Kind == SymbolField {
			fields[s.Name] = s
		}
	}

	if len(fields) != 3 {
		t.Errorf("Expected 3 fields, got %d", len(fields))
	}

	if fields["Host"].Visibility != "public" {
		t.Error("Host should be public")
	}
	if fields["timeout"].Visibility != "private" {
		t.Error("timeout should be private")
	}
}

// TestGoParser_ParseDirectory tests directory parsing.
func TestGoParser_ParseDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	// Create multiple Go files
	files := map[string]string{
		"main.go": `package main

func main() {}
`,
		"util.go": `package main

func helper() {}
`,
		"sub/sub.go": `package sub

func SubFunc() {}
`,
	}

	for path, content := range files {
		fullPath := filepath.Join(tmpDir, path)
		os.MkdirAll(filepath.Dir(fullPath), 0755)
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write %s: %v", path, err)
		}
	}

	parser := NewGoParser(tmpDir)
	result, err := parser.ParseDirectory(tmpDir)
	if err != nil {
		t.Fatalf("ParseDirectory failed: %v", err)
	}

	// Should have symbols from all files
	funcNames := make(map[string]bool)
	for _, s := range result.Symbols {
		if s.Kind == SymbolFunction {
			funcNames[s.Name] = true
		}
	}

	for _, name := range []string{"main", "helper", "SubFunc"} {
		if !funcNames[name] {
			t.Errorf("Function %s not found", name)
		}
	}
}

// TestGoParser_SkipsIgnoredDirs tests that parser skips vendor, etc.
func TestGoParser_SkipsIgnoredDirs(t *testing.T) {
	tmpDir := t.TempDir()

	// Create files in ignored directories
	files := map[string]string{
		"main.go": `package main

func main() {}
`,
		"vendor/lib/lib.go": `package lib

func VendorFunc() {}
`,
		".hidden/hidden.go": `package hidden

func HiddenFunc() {}
`,
	}

	for path, content := range files {
		fullPath := filepath.Join(tmpDir, path)
		os.MkdirAll(filepath.Dir(fullPath), 0755)
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write %s: %v", path, err)
		}
	}

	parser := NewGoParser(tmpDir)
	result, err := parser.ParseDirectory(tmpDir)
	if err != nil {
		t.Fatalf("ParseDirectory failed: %v", err)
	}

	// Should NOT have symbols from vendor or hidden dirs
	for _, s := range result.Symbols {
		if s.Name == "VendorFunc" {
			t.Error("VendorFunc should have been skipped (in vendor/)")
		}
		if s.Name == "HiddenFunc" {
			t.Error("HiddenFunc should have been skipped (in .hidden/)")
		}
	}
}

// TestGoParser_FileHash tests that file hashes are computed.
func TestGoParser_FileHash(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "hash.go")

	content := `package example

func Test() {}
`
	err := os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	parser := NewGoParser(tmpDir)
	result, err := parser.ParseFile(filePath)
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	// All symbols should have the same file hash
	var fileHash string
	for _, s := range result.Symbols {
		if fileHash == "" {
			fileHash = s.FileHash
		} else if s.FileHash != fileHash {
			t.Error("All symbols from same file should have same hash")
		}
	}

	if len(fileHash) != 64 { // SHA256 hex = 64 chars
		t.Errorf("Expected 64-char SHA256 hash, got %d chars", len(fileHash))
	}
}

// TestGoParser_RelativePaths tests that file paths are relative.
func TestGoParser_RelativePaths(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "internal", "pkg")
	os.MkdirAll(subDir, 0755)
	filePath := filepath.Join(subDir, "test.go")

	content := `package pkg

func Test() {}
`
	err := os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	parser := NewGoParser(tmpDir)
	result, err := parser.ParseFile(filePath)
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	for _, s := range result.Symbols {
		if filepath.IsAbs(s.FilePath) {
			t.Errorf("FilePath should be relative, got: %s", s.FilePath)
		}
		expected := filepath.Join("internal", "pkg", "test.go")
		if s.FilePath != expected {
			t.Errorf("Expected FilePath %q, got %q", expected, s.FilePath)
		}
	}
}

// TestGoParser_ModulePath tests module path extraction.
func TestGoParser_ModulePath(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "internal", "memory")
	os.MkdirAll(subDir, 0755)
	filePath := filepath.Join(subDir, "store.go")

	content := `package memory

func NewStore() {}
`
	err := os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	parser := NewGoParser(tmpDir)
	result, err := parser.ParseFile(filePath)
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	for _, s := range result.Symbols {
		if s.Name == "NewStore" {
			expected := filepath.Join("internal", "memory")
			if s.ModulePath != expected {
				t.Errorf("Expected ModulePath %q, got %q", expected, s.ModulePath)
			}
		}
	}
}

// TestGoParser_LineNumbers tests that line numbers are accurate.
func TestGoParser_LineNumbers(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "lines.go")

	content := `package example

// Line 3
type Config struct { // Line 4
	Name string // Line 5
} // Line 6

// Line 8
func NewConfig() *Config { // Line 9
	return nil // Line 10
} // Line 11
`
	err := os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	parser := NewGoParser(tmpDir)
	result, err := parser.ParseFile(filePath)
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	lineChecks := map[string]struct{ start, end int }{
		"Config":    {4, 6},
		"NewConfig": {9, 11},
	}

	for _, s := range result.Symbols {
		if expected, ok := lineChecks[s.Name]; ok {
			if s.StartLine != expected.start || s.EndLine != expected.end {
				t.Errorf("%s: expected lines %d-%d, got %d-%d",
					s.Name, expected.start, expected.end, s.StartLine, s.EndLine)
			}
		}
	}
}
