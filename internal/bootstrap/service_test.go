package bootstrap

import (
	"testing"

	"github.com/josephgoksu/TaskWing/internal/llm"
)

func TestNewService(t *testing.T) {
	basePath := "/test/path"
	cfg := llm.Config{
		Provider: "openai",
		Model:    "gpt-4",
	}

	svc := NewService(basePath, cfg)

	if svc == nil {
		t.Fatal("NewService returned nil")
	}
	if svc.basePath != basePath {
		t.Errorf("basePath = %q, want %q", svc.basePath, basePath)
	}
	if svc.initializer == nil {
		t.Error("initializer is nil")
	}
}

func TestBootstrapResult(t *testing.T) {
	result := &BootstrapResult{
		FindingsCount: 5,
		Warnings:      []string{"warning1", "warning2"},
		Errors:        nil,
	}

	if result.FindingsCount != 5 {
		t.Errorf("FindingsCount = %d, want 5", result.FindingsCount)
	}
	if len(result.Warnings) != 2 {
		t.Errorf("Warnings count = %d, want 2", len(result.Warnings))
	}
	if result.Errors != nil {
		t.Error("Errors should be nil")
	}
}

func TestJoinMax(t *testing.T) {
	tests := []struct {
		name   string
		parts  []string
		n      int
		expect string
	}{
		{
			name:   "empty",
			parts:  []string{},
			n:      3,
			expect: "",
		},
		{
			name:   "fewer than n",
			parts:  []string{"a", "b"},
			n:      3,
			expect: "a, b",
		},
		{
			name:   "exactly n",
			parts:  []string{"a", "b", "c"},
			n:      3,
			expect: "a, b, c",
		},
		{
			name:   "more than n",
			parts:  []string{"a", "b", "c", "d", "e"},
			n:      3,
			expect: "a, b, c, ...",
		},
		{
			name:   "single item",
			parts:  []string{"only"},
			n:      3,
			expect: "only",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := joinMax(tt.parts, tt.n)
			if got != tt.expect {
				t.Errorf("joinMax(%v, %d) = %q, want %q", tt.parts, tt.n, got, tt.expect)
			}
		})
	}
}

func TestService_InitializeProject(t *testing.T) {
	tmpDir := t.TempDir()
	svc := NewService(tmpDir, llm.Config{})

	// Test with empty AIs
	err := svc.InitializeProject(false, []string{})
	if err != nil {
		t.Errorf("InitializeProject with empty AIs failed: %v", err)
	}

	// Test with valid AI
	err = svc.InitializeProject(false, []string{"claude"})
	if err != nil {
		t.Errorf("InitializeProject with claude failed: %v", err)
	}
}
