package agents

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEinoReadFileTool(t *testing.T) {
	// Create temp directory with test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.go")
	content := `package main

func main() {
	fmt.Println("Hello, World!")
}
`
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	tool := NewEinoReadFileTool(tmpDir)
	ctx := context.Background()

	// Test Info
	info, err := tool.Info(ctx)
	if err != nil {
		t.Fatalf("Info() error: %v", err)
	}
	if info.Name != "read_file" {
		t.Errorf("Name = %q, want 'read_file'", info.Name)
	}

	// Test InvokableRun
	result, err := tool.InvokableRun(ctx, `{"path": "test.go"}`)
	if err != nil {
		t.Fatalf("InvokableRun() error: %v", err)
	}
	if !strings.Contains(result, "func main()") {
		t.Errorf("Result should contain 'func main()', got: %s", result)
	}

	// Test path traversal prevention
	_, err = tool.InvokableRun(ctx, `{"path": "../etc/passwd"}`)
	if err == nil {
		t.Error("Should reject path traversal")
	}
}

func TestEinoGrepTool(t *testing.T) {
	// Create temp directory with test files
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main\nfunc main() {}"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "util.go"), []byte("package main\nfunc helper() {}"), 0644); err != nil {
		t.Fatal(err)
	}

	tool := NewEinoGrepTool(tmpDir)
	ctx := context.Background()

	// Test Info
	info, err := tool.Info(ctx)
	if err != nil {
		t.Fatalf("Info() error: %v", err)
	}
	if info.Name != "grep_search" {
		t.Errorf("Name = %q, want 'grep_search'", info.Name)
	}

	// Test search
	result, err := tool.InvokableRun(ctx, `{"pattern": "func main"}`)
	if err != nil {
		t.Fatalf("InvokableRun() error: %v", err)
	}
	if !strings.Contains(result, "main.go") {
		t.Errorf("Result should contain 'main.go', got: %s", result)
	}

	// Test no matches
	result, err = tool.InvokableRun(ctx, `{"pattern": "nonexistent_pattern_xyz"}`)
	if err != nil {
		t.Fatalf("InvokableRun() error: %v", err)
	}
	if !strings.Contains(result, "No matches") {
		t.Errorf("Should report no matches, got: %s", result)
	}
}

func TestEinoListDirTool(t *testing.T) {
	// Create temp directory with structure
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "internal")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "util.go"), []byte("package internal"), 0644); err != nil {
		t.Fatal(err)
	}

	tool := NewEinoListDirTool(tmpDir)
	ctx := context.Background()

	// Test Info
	info, err := tool.Info(ctx)
	if err != nil {
		t.Fatalf("Info() error: %v", err)
	}
	if info.Name != "list_dir" {
		t.Errorf("Name = %q, want 'list_dir'", info.Name)
	}

	// Test listing root
	result, err := tool.InvokableRun(ctx, `{}`)
	if err != nil {
		t.Fatalf("InvokableRun() error: %v", err)
	}
	if !strings.Contains(result, "internal/") {
		t.Errorf("Result should contain 'internal/', got: %s", result)
	}
	if !strings.Contains(result, "main.go") {
		t.Errorf("Result should contain 'main.go', got: %s", result)
	}

	// Test listing subdirectory
	result, err = tool.InvokableRun(ctx, `{"path": "internal"}`)
	if err != nil {
		t.Fatalf("InvokableRun() error: %v", err)
	}
	if !strings.Contains(result, "util.go") {
		t.Errorf("Result should contain 'util.go', got: %s", result)
	}
}

func TestEinoExecTool(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewEinoExecTool(tmpDir)
	ctx := context.Background()

	// Test Info
	info, err := tool.Info(ctx)
	if err != nil {
		t.Fatalf("Info() error: %v", err)
	}
	if info.Name != "exec_command" {
		t.Errorf("Name = %q, want 'exec_command'", info.Name)
	}

	// Test allowed command
	_, err = tool.InvokableRun(ctx, `{"command": "wc", "args": ["--version"]}`)
	// wc --version might not be supported on all systems, so just check no permission error
	if err != nil && strings.Contains(err.Error(), "not allowed") {
		t.Errorf("wc should be allowed, got: %v", err)
	}

	// Test disallowed command
	_, err = tool.InvokableRun(ctx, `{"command": "rm", "args": ["-rf", "/"]}`)
	if err == nil || !strings.Contains(err.Error(), "not allowed") {
		t.Error("Should reject disallowed command 'rm'")
	}
}

func TestCreateEinoTools(t *testing.T) {
	tools := CreateEinoTools("/tmp/test")
	if len(tools) != 4 {
		t.Errorf("Expected 4 tools, got %d", len(tools))
	}

	// Verify all tools implement the interface
	ctx := context.Background()
	for _, tool := range tools {
		info, err := tool.Info(ctx)
		if err != nil {
			t.Errorf("Tool.Info() error: %v", err)
		}
		if info.Name == "" {
			t.Error("Tool name should not be empty")
		}
	}
}
