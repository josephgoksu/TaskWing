package freshness

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCheckFresh(t *testing.T) {
	ResetCache()
	dir := t.TempDir()

	filePath := filepath.Join(dir, "main.go")
	if err := os.WriteFile(filePath, []byte("package main"), 0644); err != nil {
		t.Fatal(err)
	}

	// Reference time is in the future (file is older)
	evidence := mustJSON(t, []evidenceItem{{FilePath: "main.go"}})
	result := Check(dir, evidence, time.Now().Add(time.Hour))

	if result.Status != StatusFresh {
		t.Fatalf("expected fresh, got %s", result.Status)
	}
	if result.DecayFactor != 1.0 {
		t.Fatalf("expected decay 1.0, got %f", result.DecayFactor)
	}
}

func TestCheckStale(t *testing.T) {
	ResetCache()
	dir := t.TempDir()

	filePath := filepath.Join(dir, "main.go")
	if err := os.WriteFile(filePath, []byte("package main"), 0644); err != nil {
		t.Fatal(err)
	}

	result := Check(dir, mustJSON(t, []evidenceItem{{FilePath: "main.go"}}), time.Now().Add(-time.Hour))

	if result.Status != StatusStale {
		t.Fatalf("expected stale, got %s", result.Status)
	}
	if len(result.StaleFiles) != 1 {
		t.Fatalf("expected 1 stale file, got %d", len(result.StaleFiles))
	}
	if result.DecayFactor != 0.7 {
		t.Fatalf("expected decay 0.7, got %f", result.DecayFactor)
	}
}

func TestCheckAllMissing(t *testing.T) {
	ResetCache()
	dir := t.TempDir()

	result := Check(dir, mustJSON(t, []evidenceItem{{FilePath: "gone.go"}}), time.Now())

	if result.Status != StatusMissing {
		t.Fatalf("expected missing, got %s", result.Status)
	}
	if len(result.MissingFiles) != 1 {
		t.Fatalf("expected 1 missing file, got %d", len(result.MissingFiles))
	}
	// All missing: decay = 1.0 - (1.0 * 0.8) = 0.2
	if result.DecayFactor < 0.19 || result.DecayFactor > 0.21 {
		t.Fatalf("expected decay ~0.2 for all-missing, got %f", result.DecayFactor)
	}
}

func TestCheckNoEvidence(t *testing.T) {
	ResetCache()
	result := Check("/tmp", "", time.Now())

	if result.Status != StatusNoEvidence {
		t.Fatalf("expected no_evidence, got %s", result.Status)
	}
	if result.DecayFactor != 1.0 {
		t.Fatalf("expected decay 1.0, got %f", result.DecayFactor)
	}
}

func TestCheckEmptyEvidenceArray(t *testing.T) {
	ResetCache()
	result := Check("/tmp", "[]", time.Now())

	if result.Status != StatusNoEvidence {
		t.Fatalf("expected no_evidence for empty array, got %s", result.Status)
	}
}

func TestCheckSkipsBuildArtifacts(t *testing.T) {
	ResetCache()
	dir := t.TempDir()

	nmDir := filepath.Join(dir, "node_modules")
	if err := os.MkdirAll(nmDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nmDir, "dep.js"), []byte("module.exports = {}"), 0644); err != nil {
		t.Fatal(err)
	}

	evidence := mustJSON(t, []evidenceItem{{FilePath: "node_modules/dep.js"}})
	result := Check(dir, evidence, time.Now().Add(-time.Hour))

	if result.Status != StatusNoEvidence {
		t.Fatalf("expected no_evidence (build artifact skipped), got %s", result.Status)
	}
}

func TestCheckMixedStaleAndFresh(t *testing.T) {
	ResetCache()
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "fresh.go"), []byte("package a"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "stale.go"), []byte("package b"), 0644); err != nil {
		t.Fatal(err)
	}

	refTime := time.Now().Add(-time.Hour)
	evidence := mustJSON(t, []evidenceItem{
		{FilePath: "fresh.go"},
		{FilePath: "stale.go"},
	})
	result := Check(dir, evidence, refTime)

	if result.Status != StatusStale {
		t.Fatalf("expected stale, got %s", result.Status)
	}
}

func TestCheckPartialMissing(t *testing.T) {
	ResetCache()
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "exists.go"), []byte("package a"), 0644); err != nil {
		t.Fatal(err)
	}

	evidence := mustJSON(t, []evidenceItem{
		{FilePath: "exists.go"},
		{FilePath: "deleted.go"},
	})
	result := Check(dir, evidence, time.Now().Add(time.Hour))

	if result.Status != StatusMissing {
		t.Fatalf("expected missing, got %s", result.Status)
	}
	// 1 of 2 missing: decay = 1.0 - (0.5 * 0.8) = 0.6
	if result.DecayFactor < 0.55 || result.DecayFactor > 0.65 {
		t.Fatalf("expected decay ~0.6 for 50%% missing, got %f", result.DecayFactor)
	}
}

func TestDecaySmoothCurve(t *testing.T) {
	// Verify the decay formula produces a smooth curve with no discontinuities
	// decay = 1.0 - (missingRatio * 0.8)
	ResetCache()
	dir := t.TempDir()

	// Create 4 files, progressively make them missing
	for _, name := range []string{"a.go", "b.go", "c.go", "d.go"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("package x"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	tests := []struct {
		present []string
		missing []string
		wantMin float64
		wantMax float64
	}{
		{[]string{"a.go", "b.go", "c.go"}, []string{"gone1.go"}, 0.75, 0.85},         // 1/4 missing: 0.8
		{[]string{"a.go", "b.go"}, []string{"gone1.go", "gone2.go"}, 0.55, 0.65},     // 2/4 missing: 0.6
		{[]string{"a.go"}, []string{"gone1.go", "gone2.go", "gone3.go"}, 0.35, 0.45}, // 3/4 missing: 0.4
		{nil, []string{"gone1.go", "gone2.go", "gone3.go", "gone4.go"}, 0.15, 0.25},  // 4/4 missing: 0.2
	}

	for _, tt := range tests {
		ResetCache()
		var items []evidenceItem
		for _, p := range tt.present {
			items = append(items, evidenceItem{FilePath: p})
		}
		for _, m := range tt.missing {
			items = append(items, evidenceItem{FilePath: m})
		}
		result := Check(dir, mustJSON(t, items), time.Now().Add(time.Hour))
		if result.DecayFactor < tt.wantMin || result.DecayFactor > tt.wantMax {
			t.Errorf("missing=%d/total=%d: expected decay %.2f-%.2f, got %.4f",
				len(tt.missing), len(tt.present)+len(tt.missing), tt.wantMin, tt.wantMax, result.DecayFactor)
		}
	}
}

func TestFormatStatusFresh(t *testing.T) {
	now := time.Now().Add(-2 * time.Hour)
	s := FormatStatus(Result{Status: StatusFresh}, &now)
	if s != "[verified 2h ago]" {
		t.Fatalf("unexpected format: %q", s)
	}
}

func TestFormatStatusStale(t *testing.T) {
	s := FormatStatus(Result{Status: StatusStale, StaleFiles: []string{"auth.go"}}, nil)
	if s != "[STALE: auth.go]" {
		t.Fatalf("unexpected format: %q", s)
	}
}

func TestFormatStatusNoEvidence(t *testing.T) {
	s := FormatStatus(Result{Status: StatusNoEvidence}, nil)
	if s != "" {
		t.Fatalf("expected empty string for no_evidence, got %q", s)
	}
}

func TestStatCache(t *testing.T) {
	ResetCache()
	dir := t.TempDir()
	filePath := filepath.Join(dir, "cached.go")
	if err := os.WriteFile(filePath, []byte("package a"), 0644); err != nil {
		t.Fatal(err)
	}

	// First call: cache miss
	info1, err1 := defaultCache.stat(filePath)
	if err1 != nil {
		t.Fatal(err1)
	}

	// Delete the file
	os.Remove(filePath)

	// Second call: should return cached result (file still "exists")
	info2, err2 := defaultCache.stat(filePath)
	if err2 != nil {
		t.Fatal("expected cached result, got error")
	}
	if info1.ModTime() != info2.ModTime() {
		t.Fatal("cache returned different result")
	}
}

func TestCheckSkipsBuildArtifactsInSubdirectory(t *testing.T) {
	ResetCache()
	dir := t.TempDir()

	nmDir := filepath.Join(dir, "api", "node_modules")
	if err := os.MkdirAll(nmDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nmDir, "dep.js"), []byte("module.exports = {}"), 0644); err != nil {
		t.Fatal(err)
	}

	evidence := mustJSON(t, []evidenceItem{{FilePath: "api/node_modules/dep.js"}})
	result := Check(dir, evidence, time.Now().Add(-time.Hour))

	if result.Status != StatusNoEvidence {
		t.Fatalf("expected no_evidence (monorepo build artifact skipped), got %s", result.Status)
	}
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
