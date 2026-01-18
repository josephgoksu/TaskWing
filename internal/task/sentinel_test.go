package task

import (
	"testing"
)

func TestSentinelAnalyze_PerfectMatch(t *testing.T) {
	s := NewSentinel()
	task := &Task{
		ID:            "task-123",
		Title:         "Add authentication",
		ExpectedFiles: []string{"internal/auth/handler.go", "internal/auth/middleware.go"},
		FilesModified: []string{"internal/auth/handler.go", "internal/auth/middleware.go"},
	}

	report := s.Analyze(task)

	if len(report.Deviations) != 0 {
		t.Errorf("Expected 0 deviations, got %d", len(report.Deviations))
	}
	if report.DeviationRate != 0.0 {
		t.Errorf("Expected deviation rate 0.0, got %f", report.DeviationRate)
	}
	if report.HasDeviations() {
		t.Error("Expected HasDeviations() to return false")
	}
}

func TestSentinelAnalyze_DriftDetection(t *testing.T) {
	s := NewSentinel()
	task := &Task{
		ID:            "task-123",
		Title:         "Add authentication",
		ExpectedFiles: []string{"internal/auth/handler.go"},
		FilesModified: []string{"internal/auth/handler.go", "internal/auth/extra.go", "internal/db/schema.go"},
	}

	report := s.Analyze(task)

	driftDeviations := report.GetDeviationsByType(DeviationDrift)
	if len(driftDeviations) != 2 {
		t.Errorf("Expected 2 drift deviations, got %d", len(driftDeviations))
	}

	if !report.HasDeviations() {
		t.Error("Expected HasDeviations() to return true")
	}
}

func TestSentinelAnalyze_MissingDetection(t *testing.T) {
	s := NewSentinel()
	task := &Task{
		ID:            "task-123",
		Title:         "Add authentication",
		ExpectedFiles: []string{"internal/auth/handler.go", "internal/auth/middleware.go"},
		FilesModified: []string{"internal/auth/handler.go"},
	}

	report := s.Analyze(task)

	missingDeviations := report.GetDeviationsByType(DeviationMissing)
	if len(missingDeviations) != 1 {
		t.Errorf("Expected 1 missing deviation, got %d", len(missingDeviations))
	}
	if missingDeviations[0].File != "internal/auth/middleware.go" {
		t.Errorf("Expected missing file to be middleware.go, got %s", missingDeviations[0].File)
	}
}

func TestSentinelAnalyze_HighRiskFile(t *testing.T) {
	s := NewSentinel()
	task := &Task{
		ID:            "task-123",
		Title:         "Update config",
		ExpectedFiles: []string{},
		FilesModified: []string{"config/secrets.yaml"},
	}

	report := s.Analyze(task)

	if !report.HasCriticalDeviations() {
		t.Error("Expected HasCriticalDeviations() to return true for secrets file")
	}

	driftDeviations := report.GetDeviationsByType(DeviationDrift)
	if len(driftDeviations) != 1 {
		t.Fatalf("Expected 1 drift deviation, got %d", len(driftDeviations))
	}
	if driftDeviations[0].Severity != SeverityError {
		t.Errorf("Expected severity Error for secrets file, got %s", driftDeviations[0].Severity)
	}
}

func TestSentinelAnalyze_EmptyExpected(t *testing.T) {
	s := NewSentinel()
	task := &Task{
		ID:            "task-123",
		Title:         "Research task",
		ExpectedFiles: []string{},
		FilesModified: []string{"notes.md"},
	}

	report := s.Analyze(task)

	// Files modified with no expected = deviation rate of 1.0
	if report.DeviationRate != 1.0 {
		t.Errorf("Expected deviation rate 1.0, got %f", report.DeviationRate)
	}
}

func TestSentinelAnalyze_NoFiles(t *testing.T) {
	s := NewSentinel()
	task := &Task{
		ID:            "task-123",
		Title:         "Review task",
		ExpectedFiles: []string{},
		FilesModified: []string{},
	}

	report := s.Analyze(task)

	if report.DeviationRate != 0.0 {
		t.Errorf("Expected deviation rate 0.0, got %f", report.DeviationRate)
	}
	if report.HasDeviations() {
		t.Error("Expected no deviations for empty task")
	}
}

func TestNormalizePath(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"./internal/auth/handler.go", "internal/auth/handler.go"},
		{"internal/auth/handler.go", "internal/auth/handler.go"},
		{"internal//auth//handler.go", "internal/auth/handler.go"},
		{"./internal/../internal/auth/handler.go", "internal/auth/handler.go"},
	}

	for _, tt := range tests {
		result := normalizePath(tt.input)
		if result != tt.expected {
			t.Errorf("normalizePath(%q) = %q, expected %q", tt.input, result, tt.expected)
		}
	}
}
