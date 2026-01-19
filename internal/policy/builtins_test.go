package policy

import (
	"testing"

	"github.com/spf13/afero"
)

func TestFileLineCountImpl(t *testing.T) {
	fs := afero.NewMemMapFs()
	ctx := &BuiltinContext{
		WorkDir: "/project",
		Fs:      fs,
	}

	// Create a test file with 5 lines
	_ = fs.MkdirAll("/project", 0755)
	content := "line1\nline2\nline3\nline4\nline5"
	_ = afero.WriteFile(fs, "/project/test.go", []byte(content), 0644)

	tests := []struct {
		name string
		path string
		want int
	}{
		{
			name: "existing file",
			path: "test.go",
			want: 5,
		},
		{
			name: "absolute path",
			path: "/project/test.go",
			want: 5,
		},
		{
			name: "non-existent file",
			path: "nonexistent.go",
			want: -1,
		},
		{
			name: "empty path resolves to workdir",
			path: "",
			want: 0, // Empty path resolves to directory which has 0 scannable lines
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fileLineCountImpl(ctx, tt.path)
			if got != tt.want {
				t.Errorf("fileLineCountImpl() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestFileLineCountImpl_EmptyFile(t *testing.T) {
	fs := afero.NewMemMapFs()
	ctx := &BuiltinContext{
		WorkDir: "/project",
		Fs:      fs,
	}

	_ = fs.MkdirAll("/project", 0755)
	_ = afero.WriteFile(fs, "/project/empty.go", []byte(""), 0644)

	got := fileLineCountImpl(ctx, "empty.go")
	if got != 0 {
		t.Errorf("fileLineCountImpl() for empty file = %d, want 0", got)
	}
}

func TestHasPatternImpl(t *testing.T) {
	fs := afero.NewMemMapFs()
	ctx := &BuiltinContext{
		WorkDir: "/project",
		Fs:      fs,
	}

	_ = fs.MkdirAll("/project", 0755)
	content := `package main

import "fmt"

func main() {
	password := "secret123"
	apiKey := "sk-abc123"
	db.Query("SELECT * FROM users")
}
`
	_ = afero.WriteFile(fs, "/project/main.go", []byte(content), 0644)

	tests := []struct {
		name    string
		path    string
		pattern string
		want    bool
	}{
		{
			name:    "find hardcoded password",
			path:    "main.go",
			pattern: `password\s*:?=\s*"[^"]+"`,
			want:    true,
		},
		{
			name:    "find api key pattern",
			path:    "main.go",
			pattern: `apiKey\s*:?=\s*"sk-`,
			want:    true,
		},
		{
			name:    "find raw SQL",
			path:    "main.go",
			pattern: `db\.Query\(`,
			want:    true,
		},
		{
			name:    "pattern not found",
			path:    "main.go",
			pattern: `DOESNOTEXIST`,
			want:    false,
		},
		{
			name:    "file not found",
			path:    "nonexistent.go",
			pattern: `.*`,
			want:    false,
		},
		{
			name:    "invalid regex",
			path:    "main.go",
			pattern: `[invalid`,
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasPatternImpl(ctx, tt.path, tt.pattern)
			if got != tt.want {
				t.Errorf("hasPatternImpl() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFileImportsImpl(t *testing.T) {
	fs := afero.NewMemMapFs()
	ctx := &BuiltinContext{
		WorkDir: "/project",
		Fs:      fs,
	}

	_ = fs.MkdirAll("/project", 0755)

	// Single import
	singleImport := `package main

import "fmt"

func main() {}
`
	_ = afero.WriteFile(fs, "/project/single.go", []byte(singleImport), 0644)

	// Block import
	blockImport := `package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/example/pkg"
)

func main() {}
`
	_ = afero.WriteFile(fs, "/project/block.go", []byte(blockImport), 0644)

	// No imports
	noImport := `package main

func main() {}
`
	_ = afero.WriteFile(fs, "/project/none.go", []byte(noImport), 0644)

	tests := []struct {
		name      string
		path      string
		wantCount int
	}{
		{
			name:      "single import",
			path:      "single.go",
			wantCount: 1,
		},
		{
			name:      "block imports",
			path:      "block.go",
			wantCount: 4, // context, fmt, strings, github.com/example/pkg
		},
		{
			name:      "no imports",
			path:      "none.go",
			wantCount: 0,
		},
		{
			name:      "file not found",
			path:      "nonexistent.go",
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fileImportsImpl(ctx, tt.path)
			if len(got) != tt.wantCount {
				t.Errorf("fileImportsImpl() returned %d imports, want %d: %v", len(got), tt.wantCount, got)
			}
		})
	}
}

func TestSymbolExistsImpl(t *testing.T) {
	fs := afero.NewMemMapFs()
	ctx := &BuiltinContext{
		WorkDir: "/project",
		Fs:      fs,
	}

	_ = fs.MkdirAll("/project", 0755)

	goCode := `package main

func MyFunction() {}

func (s *Server) HandleRequest() {}

type Config struct {
	Port int
}

var globalVar = "test"

const MaxRetries = 3
`
	_ = afero.WriteFile(fs, "/project/code.go", []byte(goCode), 0644)

	jsCode := `class UserService {
	constructor() {}
}

function processData(data) {
	return data;
}
`
	_ = afero.WriteFile(fs, "/project/code.js", []byte(jsCode), 0644)

	tests := []struct {
		name       string
		path       string
		symbolName string
		want       bool
	}{
		{
			name:       "Go function",
			path:       "code.go",
			symbolName: "MyFunction",
			want:       true,
		},
		{
			name:       "Go method",
			path:       "code.go",
			symbolName: "HandleRequest",
			want:       true,
		},
		{
			name:       "Go type",
			path:       "code.go",
			symbolName: "Config",
			want:       true,
		},
		{
			name:       "Go var",
			path:       "code.go",
			symbolName: "globalVar",
			want:       true,
		},
		{
			name:       "Go const",
			path:       "code.go",
			symbolName: "MaxRetries",
			want:       true,
		},
		{
			name:       "JS class",
			path:       "code.js",
			symbolName: "UserService",
			want:       true,
		},
		{
			name:       "JS function",
			path:       "code.js",
			symbolName: "processData",
			want:       true,
		},
		{
			name:       "symbol not found",
			path:       "code.go",
			symbolName: "NonExistentSymbol",
			want:       false,
		},
		{
			name:       "file not found",
			path:       "nonexistent.go",
			symbolName: "Anything",
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := symbolExistsImpl(ctx, tt.path, tt.symbolName)
			if got != tt.want {
				t.Errorf("symbolExistsImpl(%q, %q) = %v, want %v", tt.path, tt.symbolName, got, tt.want)
			}
		})
	}
}

func TestFileExistsImpl(t *testing.T) {
	fs := afero.NewMemMapFs()
	ctx := &BuiltinContext{
		WorkDir: "/project",
		Fs:      fs,
	}

	_ = fs.MkdirAll("/project", 0755)
	_ = afero.WriteFile(fs, "/project/exists.txt", []byte("content"), 0644)

	tests := []struct {
		name string
		path string
		want bool
	}{
		{
			name: "existing file",
			path: "exists.txt",
			want: true,
		},
		{
			name: "absolute existing file",
			path: "/project/exists.txt",
			want: true,
		},
		{
			name: "non-existent file",
			path: "nonexistent.txt",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fileExistsImpl(ctx, tt.path)
			if got != tt.want {
				t.Errorf("fileExistsImpl() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseGoImports(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    []string
	}{
		{
			name: "single import",
			content: `package main
import "fmt"
`,
			want: []string{"fmt"},
		},
		{
			name: "import block",
			content: `package main
import (
	"context"
	"fmt"
)
`,
			want: []string{"context", "fmt"},
		},
		{
			name: "named imports",
			content: `package main
import (
	ctx "context"
	. "fmt"
	_ "net/http/pprof"
)
`,
			want: []string{"context", "fmt", "net/http/pprof"},
		},
		{
			name:    "no imports",
			content: `package main`,
			want:    []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseGoImports(tt.content)
			if len(got) != len(tt.want) {
				t.Errorf("parseGoImports() returned %d imports, want %d: %v", len(got), len(tt.want), got)
			}
		})
	}
}

func TestGetBuiltinNames(t *testing.T) {
	names := GetBuiltinNames()
	if len(names) < 5 {
		t.Errorf("GetBuiltinNames() returned %d names, want at least 5", len(names))
	}

	expected := []string{
		"taskwing.file_line_count",
		"taskwing.has_pattern",
		"taskwing.file_imports",
		"taskwing.symbol_exists",
		"taskwing.file_exists",
	}

	for _, e := range expected {
		found := false
		for _, n := range names {
			if n == e {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("GetBuiltinNames() missing %q", e)
		}
	}
}

func TestIsBuiltin(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"taskwing.file_line_count", true},
		{"taskwing.has_pattern", true},
		{"file_line_count", true}, // Short form should match
		{"unknown_builtin", false},
		{"rego.parse_json", false}, // Not a TaskWing builtin
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsBuiltin(tt.name); got != tt.want {
				t.Errorf("IsBuiltin(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

func TestNewBuiltinContext(t *testing.T) {
	ctx := NewBuiltinContext("/test/dir")
	if ctx == nil {
		t.Fatal("NewBuiltinContext() returned nil")
	}
	if ctx.WorkDir != "/test/dir" {
		t.Errorf("WorkDir = %q, want %q", ctx.WorkDir, "/test/dir")
	}
	if ctx.Fs == nil {
		t.Error("Fs is nil")
	}
	if ctx.CodeIntel != nil {
		t.Error("CodeIntel should be nil for basic context")
	}
}
