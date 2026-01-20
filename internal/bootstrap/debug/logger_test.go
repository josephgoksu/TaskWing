package debug

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestNewDebugLogger(t *testing.T) {
	tmpDir := t.TempDir()

	logger, err := NewDebugLogger(Options{
		OutputDir:    tmpDir,
		EnableStderr: false,
	})
	if err != nil {
		t.Fatalf("NewDebugLogger failed: %v", err)
	}
	defer logger.Close()

	// Check log file was created
	if logger.LogPath() == "" {
		t.Error("LogPath() should not be empty")
	}

	if _, err := os.Stat(logger.LogPath()); os.IsNotExist(err) {
		t.Errorf("Log file should exist at %s", logger.LogPath())
	}

	// Check symlink was created
	latestLink := filepath.Join(tmpDir, "debug-latest.log")
	if _, err := os.Lstat(latestLink); os.IsNotExist(err) {
		t.Error("debug-latest.log symlink should exist")
	}
}

func TestDebugLoggerCreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	logsDir := filepath.Join(tmpDir, "nested", "logs")

	logger, err := NewDebugLogger(Options{
		OutputDir:    logsDir,
		EnableStderr: false,
	})
	if err != nil {
		t.Fatalf("NewDebugLogger failed: %v", err)
	}
	defer logger.Close()

	// Check directory was created
	if _, err := os.Stat(logsDir); os.IsNotExist(err) {
		t.Error("Logs directory should be created")
	}
}

func TestLogMethods(t *testing.T) {
	tmpDir := t.TempDir()
	var stderrBuf bytes.Buffer

	logger, err := NewDebugLogger(Options{
		OutputDir:    tmpDir,
		EnableStderr: true,
		StderrWriter: &stderrBuf,
	})
	if err != nil {
		t.Fatalf("NewDebugLogger failed: %v", err)
	}

	// Log messages at each level
	logger.Debug("test_debug", "Debug message", map[string]any{"key": "value"})
	logger.Info("test_info", "Info message", nil)
	logger.Warn("test_warn", "Warn message", nil)
	logger.Error("test_error", "Error message", nil)

	// Close to flush
	logger.Close()

	// Read log file
	content, err := os.ReadFile(logger.LogPath())
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	// Verify each line is valid JSON with required fields
	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	if len(lines) != 4 {
		t.Errorf("Expected 4 log lines, got %d", len(lines))
	}

	expectedLevels := []string{"debug", "info", "warn", "error"}
	expectedEvents := []string{"test_debug", "test_info", "test_warn", "test_error"}

	for i, line := range lines {
		var entry LogEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Errorf("Line %d is not valid JSON: %v", i, err)
			continue
		}

		if entry.Timestamp == "" {
			t.Errorf("Line %d missing timestamp", i)
		}
		if string(entry.Level) != expectedLevels[i] {
			t.Errorf("Line %d: expected level %s, got %s", i, expectedLevels[i], entry.Level)
		}
		if entry.Event != expectedEvents[i] {
			t.Errorf("Line %d: expected event %s, got %s", i, expectedEvents[i], entry.Event)
		}
		if entry.Component != "bootstrap" {
			t.Errorf("Line %d: expected component bootstrap, got %s", i, entry.Component)
		}
	}

	// Verify stderr output matches file output
	stderrContent := stderrBuf.String()
	if !strings.Contains(stderrContent, "test_debug") {
		t.Error("Stderr should contain debug log")
	}
	if !strings.Contains(stderrContent, "test_info") {
		t.Error("Stderr should contain info log")
	}
}

func TestStartPhase(t *testing.T) {
	tmpDir := t.TempDir()

	logger, err := NewDebugLogger(Options{
		OutputDir:    tmpDir,
		EnableStderr: false,
	})
	if err != nil {
		t.Fatalf("NewDebugLogger failed: %v", err)
	}

	// Start a phase
	stop := logger.StartPhase("test_phase", map[string]any{"foo": "bar"})

	// Simulate some work
	time.Sleep(10 * time.Millisecond)

	// Stop the phase
	stop(nil)

	logger.Close()

	// Read and verify
	content, err := os.ReadFile(logger.LogPath())
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	if len(lines) != 2 {
		t.Errorf("Expected 2 log lines (phase_start, phase_end), got %d", len(lines))
	}

	// Verify phase_start
	var startEntry LogEntry
	if err := json.Unmarshal([]byte(lines[0]), &startEntry); err != nil {
		t.Errorf("phase_start is not valid JSON: %v", err)
	}
	if startEntry.Event != "phase_start" {
		t.Errorf("Expected phase_start event, got %s", startEntry.Event)
	}
	if startEntry.Phase != "test_phase" {
		t.Errorf("Expected phase test_phase, got %s", startEntry.Phase)
	}

	// Verify phase_end
	var endEntry LogEntry
	if err := json.Unmarshal([]byte(lines[1]), &endEntry); err != nil {
		t.Errorf("phase_end is not valid JSON: %v", err)
	}
	if endEntry.Event != "phase_end" {
		t.Errorf("Expected phase_end event, got %s", endEntry.Event)
	}
	if endEntry.DurationMs == nil {
		t.Error("phase_end should have duration_ms")
	} else if *endEntry.DurationMs < 10 {
		t.Errorf("Duration should be at least 10ms, got %d", *endEntry.DurationMs)
	}
}

func TestStartPhaseWithError(t *testing.T) {
	tmpDir := t.TempDir()

	logger, err := NewDebugLogger(Options{
		OutputDir:    tmpDir,
		EnableStderr: false,
	})
	if err != nil {
		t.Fatalf("NewDebugLogger failed: %v", err)
	}

	stop := logger.StartPhase("failing_phase", nil)
	stop(os.ErrNotExist)

	logger.Close()

	content, err := os.ReadFile(logger.LogPath())
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	if len(lines) != 2 {
		t.Errorf("Expected 2 log lines, got %d", len(lines))
	}

	// Verify phase_error
	var errorEntry LogEntry
	if err := json.Unmarshal([]byte(lines[1]), &errorEntry); err != nil {
		t.Errorf("phase_error is not valid JSON: %v", err)
	}
	if errorEntry.Event != "phase_error" {
		t.Errorf("Expected phase_error event, got %s", errorEntry.Event)
	}
	if errorEntry.Level != LevelError {
		t.Errorf("Expected error level, got %s", errorEntry.Level)
	}
}

func TestConcurrentWrites(t *testing.T) {
	tmpDir := t.TempDir()

	logger, err := NewDebugLogger(Options{
		OutputDir:    tmpDir,
		EnableStderr: false,
	})
	if err != nil {
		t.Fatalf("NewDebugLogger failed: %v", err)
	}

	// Write from multiple goroutines
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				logger.Info("concurrent_write", "Message", map[string]any{"goroutine": n, "iteration": j})
			}
		}(i)
	}
	wg.Wait()

	logger.Close()

	// Verify all lines are valid JSON
	content, err := os.ReadFile(logger.LogPath())
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	if len(lines) != 100 {
		t.Errorf("Expected 100 log lines, got %d", len(lines))
	}

	for i, line := range lines {
		var entry LogEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Errorf("Line %d is not valid JSON: %v\nContent: %s", i, err, line)
		}
	}
}

func TestWithComponent(t *testing.T) {
	tmpDir := t.TempDir()

	logger, err := NewDebugLogger(Options{
		OutputDir:    tmpDir,
		EnableStderr: false,
		Component:    "main",
	})
	if err != nil {
		t.Fatalf("NewDebugLogger failed: %v", err)
	}

	// Create sub-logger with different component
	subLogger := logger.WithComponent("sub")
	subLogger.Info("sub_event", "Sub message", nil)

	logger.Close()

	content, err := os.ReadFile(logger.LogPath())
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	var entry LogEntry
	if err := json.Unmarshal(content, &entry); err != nil {
		t.Fatalf("Invalid JSON: %v", err)
	}
	if entry.Component != "sub" {
		t.Errorf("Expected component 'sub', got '%s'", entry.Component)
	}
}

func TestWithAgent(t *testing.T) {
	tmpDir := t.TempDir()

	logger, err := NewDebugLogger(Options{
		OutputDir:    tmpDir,
		EnableStderr: false,
	})
	if err != nil {
		t.Fatalf("NewDebugLogger failed: %v", err)
	}

	agentLogger := logger.WithAgent("git")
	agentLogger.Info("agent_event", "Agent message", nil)

	logger.Close()

	content, err := os.ReadFile(logger.LogPath())
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	var entry LogEntry
	if err := json.Unmarshal(content, &entry); err != nil {
		t.Fatalf("Invalid JSON: %v", err)
	}
	if entry.Agent != "git" {
		t.Errorf("Expected agent 'git', got '%s'", entry.Agent)
	}
}

func TestFilePermissions(t *testing.T) {
	tmpDir := t.TempDir()

	logger, err := NewDebugLogger(Options{
		OutputDir:    tmpDir,
		EnableStderr: false,
	})
	if err != nil {
		t.Fatalf("NewDebugLogger failed: %v", err)
	}
	defer logger.Close()

	info, err := os.Stat(logger.LogPath())
	if err != nil {
		t.Fatalf("Failed to stat log file: %v", err)
	}

	// Check file permissions (should be 0600)
	perm := info.Mode().Perm()
	if perm != 0o600 {
		t.Errorf("Expected permissions 0600, got %o", perm)
	}
}
