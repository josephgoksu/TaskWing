package cmd

import (
	"bytes"
	"os"
	"testing"
)

// TestEvalNoDriftImports ensures eval.go doesn't import internal packages
// that would cause it to reimplement CLI logic instead of invoking CLI.
// If this test fails, eval has drifted from the "CLI invoker" pattern.
func TestEvalNoDriftImports(t *testing.T) {
	content, err := os.ReadFile("eval.go")
	if err != nil {
		t.Fatalf("failed to read eval.go: %v", err)
	}

	// These imports indicate internal reimplementation (FORBIDDEN in eval)
	// Eval should invoke CLI as subprocess, not use these packages directly.
	forbidden := []string{
		`"github.com/josephgoksu/TaskWing/internal/bootstrap"`,
		`"github.com/josephgoksu/TaskWing/internal/knowledge"`,
		`"github.com/josephgoksu/TaskWing/internal/memory"`,
		`"github.com/josephgoksu/TaskWing/internal/agents/core"`,
		`"github.com/josephgoksu/TaskWing/internal/agents/planning"`,
	}

	for _, f := range forbidden {
		if bytes.Contains(content, []byte(f)) {
			t.Errorf("eval.go imports %s — this causes drift. Eval must invoke CLI as subprocess, not reimplement internals.", f)
		}
	}

	// Allowed imports (for judging and UI):
	// - internal/eval (types, config)
	// - internal/llm (for LLM judge only)
	// - internal/ui (for rendering reports)
}
