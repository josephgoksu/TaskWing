package cmd

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/josephgoksu/TaskWing/internal/agents"
	"github.com/josephgoksu/TaskWing/internal/llm"
	_ "modernc.org/sqlite"
)

// TestBootstrapPersistence verifies that agent findings are correctly written to the SQLite DB
func TestBootstrapPersistence(t *testing.T) {
	// 1. Setup temporary memory directory
	tmpDir, err := os.MkdirTemp("", "taskwing-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// 2. Prepare mock findings
	findings := []agents.Finding{
		{
			Type:        agents.FindingTypeFeature,
			Title:       "Core Feature",
			Description: "The core functionality",
			SourceAgent: "doc_agent",
		},
		{
			Type:        agents.FindingTypePattern,
			Title:       "Hexagonal Architecture",
			Description: "Ports and Adapters",
			SourceAgent: "code_agent",
			Metadata: map[string]any{
				"context":      "Applied in internal/core",
				"solution":     "Decouples core logic",
				"consequences": "More boilerplate",
			},
		},
		{
			Type:        agents.FindingTypeDecision,
			Title:       "Use SQLite",
			Description: "Embedded database choice",
			Why:         "Simplicity and zero deps",
			Tradeoffs:   "No concurrent writes",
			SourceAgent: "code_agent",
			Metadata: map[string]any{
				"component": "Core Feature", // Links to existing feature
			},
		},
		{
			Type:        agents.FindingTypeDecision,
			Title:       "Use Cobra",
			Description: "CLI Framework",
			Why:         "Standard for Go CLIs",
			Tradeoffs:   "Heavy dependency",
			SourceAgent: "code_agent",
			Metadata: map[string]any{
				"component": "CLI Interface", // Should auto-create this feature
			},
		},
		{
			Type:        agents.FindingTypeDecision,
			Title:       "Orphan Decision",
			Description: "Should be skipped",
			Why:         "No component context",
			SourceAgent: "code_agent",
			Metadata: map[string]any{
				"component": "", // Missing component -> Should strictly skip
			},
		},
	}

	// 3. Run persistence
	ctx := context.Background()
	llmCfg := llm.Config{} // Empty config, no embeddings generation expected since API key is empty

	// Call the unexported saveToMemory function (since we are in package cmd)
	err = saveToMemory(ctx, findings, tmpDir, llmCfg)
	if err != nil {
		t.Fatalf("saveToMemory failed: %v", err)
	}

	// 4. Verify Database State
	dbPath := filepath.Join(tmpDir, "memory.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	defer db.Close()

	// Verify Features
	var featureCount int
	err = db.QueryRow("SELECT COUNT(*) FROM features").Scan(&featureCount)
	if err != nil {
		t.Fatal(err)
	}
	// Core Feature (1) + CLI Interface (1) + Core Architecture (1 from smart fallback) = 3
	if featureCount != 3 {
		t.Errorf("Expected 3 features, got %d", featureCount)
	}

	// Verify "CLI Interface" was auto-created
	var cliFeatureID string
	err = db.QueryRow("SELECT id FROM features WHERE name = 'CLI Interface'").Scan(&cliFeatureID)
	if err == sql.ErrNoRows {
		t.Error("Feature 'CLI Interface' was not auto-created")
	} else if err != nil {
		t.Fatal(err)
	}

	// Verify Patterns
	var patternCount int
	err = db.QueryRow("SELECT COUNT(*) FROM patterns").Scan(&patternCount)
	if err != nil {
		t.Fatal(err)
	}
	if patternCount != 1 {
		t.Errorf("Expected 1 pattern, got %d", patternCount)
	}

	var patternContext string
	err = db.QueryRow("SELECT context FROM patterns WHERE name = 'Hexagonal Architecture'").Scan(&patternContext)
	if err != nil {
		t.Fatal(err)
	}
	if patternContext != "Applied in internal/core" {
		t.Errorf("Pattern context mismatch, got %s", patternContext)
	}

	// Verify Decisions
	var decisionCount int
	err = db.QueryRow("SELECT COUNT(*) FROM decisions").Scan(&decisionCount)
	if err != nil {
		t.Fatal(err)
	}
	// "Use SQLite" (1) + "Use Cobra" (1) + "Orphan Decision" (1 via smart fallback) = 3
	if decisionCount != 3 {
		t.Errorf("Expected 3 decisions, got %d", decisionCount)
	}

	// Verify Decision Linking
	var linkedFeatureID string
	err = db.QueryRow("SELECT feature_id FROM decisions WHERE title = 'Use Cobra'").Scan(&linkedFeatureID)
	if err != nil {
		t.Fatal(err)
	}
	if linkedFeatureID != cliFeatureID {
		t.Errorf("Decision 'Use Cobra' linked to %s, expected %s (CLI Interface)", linkedFeatureID, cliFeatureID)
	}
}
