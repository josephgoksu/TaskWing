package logger

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCrashHandler_SetContext(t *testing.T) {
	// Reset global context
	globalContext = &CrashContext{}

	SetBasePath("/tmp/test-taskwing")
	SetVersion("1.0.0-test")
	SetCommand("test command")
	SetLastInput("test input")
	SetLastPrompt("test prompt")

	globalContext.mu.RLock()
	defer globalContext.mu.RUnlock()

	if globalContext.basePath != "/tmp/test-taskwing" {
		t.Errorf("Expected basePath '/tmp/test-taskwing', got '%s'", globalContext.basePath)
	}
	if globalContext.version != "1.0.0-test" {
		t.Errorf("Expected version '1.0.0-test', got '%s'", globalContext.version)
	}
	if globalContext.command != "test command" {
		t.Errorf("Expected command 'test command', got '%s'", globalContext.command)
	}
	if globalContext.lastInput != "test input" {
		t.Errorf("Expected lastInput 'test input', got '%s'", globalContext.lastInput)
	}
	if globalContext.lastPrompt != "test prompt" {
		t.Errorf("Expected lastPrompt 'test prompt', got '%s'", globalContext.lastPrompt)
	}
}

func TestCrashHandler_SetLastPrompt_Truncation(t *testing.T) {
	// Reset global context
	globalContext = &CrashContext{}

	// Create a long prompt
	longPrompt := strings.Repeat("a", 3000)
	SetLastPrompt(longPrompt)

	globalContext.mu.RLock()
	defer globalContext.mu.RUnlock()

	if len(globalContext.lastPrompt) > 2100 {
		t.Errorf("Expected prompt to be truncated, got length %d", len(globalContext.lastPrompt))
	}
	if !strings.Contains(globalContext.lastPrompt, "[truncated]") {
		t.Error("Expected truncated prompt to contain '[truncated]'")
	}
}

func TestCrashHandler_CreateCrashLog(t *testing.T) {
	// Reset global context
	globalContext = &CrashContext{
		version:   "1.0.0",
		command:   "test",
		lastInput: "user input",
	}

	log := createCrashLog("test panic")

	if log.PanicValue != "test panic" {
		t.Errorf("Expected PanicValue 'test panic', got '%s'", log.PanicValue)
	}
	if log.Version != "1.0.0" {
		t.Errorf("Expected Version '1.0.0', got '%s'", log.Version)
	}
	if log.Command != "test" {
		t.Errorf("Expected Command 'test', got '%s'", log.Command)
	}
	if log.LastInput != "user input" {
		t.Errorf("Expected LastInput 'user input', got '%s'", log.LastInput)
	}
	if log.StackTrace == "" {
		t.Error("Expected non-empty StackTrace")
	}
	if log.GoVersion == "" {
		t.Error("Expected non-empty GoVersion")
	}
}

func TestCrashHandler_FormatCrashLog(t *testing.T) {
	log := CrashLog{
		Timestamp:  time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC),
		Version:    "1.0.0",
		Command:    "test",
		PanicValue: "test panic",
		StackTrace: "goroutine 1 [running]:\nmain.main()",
		LastInput:  "user input",
		GoVersion:  "go1.24.3",
		OS:         "darwin",
		Arch:       "arm64",
	}

	formatted := formatCrashLog(log)

	expectedStrings := []string{
		"TASKWING CRASH LOG",
		"Timestamp: 2025-01-01T12:00:00Z",
		"Version:   1.0.0",
		"Command:   test",
		"Go:        go1.24.3",
		"OS/Arch:   darwin/arm64",
		"PANIC VALUE",
		"test panic",
		"STACK TRACE",
		"goroutine 1 [running]",
		"LAST USER INPUT",
		"user input",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(formatted, expected) {
			t.Errorf("Expected formatted log to contain '%s'", expected)
		}
	}
}

func TestCrashHandler_WriteCrashLog(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	basePath := filepath.Join(tmpDir, ".taskwing")

	// Set up context
	globalContext = &CrashContext{
		basePath: basePath,
		version:  "1.0.0",
		command:  "test",
	}

	log := CrashLog{
		Timestamp:  time.Now(),
		Version:    "1.0.0",
		Command:    "test",
		PanicValue: "test panic",
		StackTrace: "test stack",
		GoVersion:  "go1.24",
		OS:         "test",
		Arch:       "test",
	}

	err := writeCrashLog(log)
	if err != nil {
		t.Fatalf("writeCrashLog failed: %v", err)
	}

	// Verify directory was created
	crashDir := filepath.Join(basePath, CrashLogDir)
	if _, err := os.Stat(crashDir); os.IsNotExist(err) {
		t.Error("Expected crash log directory to be created")
	}

	// Verify file was created
	logs, err := ListCrashLogs()
	if err != nil {
		t.Fatalf("ListCrashLogs failed: %v", err)
	}
	if len(logs) != 1 {
		t.Errorf("Expected 1 crash log, got %d", len(logs))
	}

	// Verify file content
	if len(logs) > 0 {
		content, err := ReadCrashLog(logs[0])
		if err != nil {
			t.Fatalf("ReadCrashLog failed: %v", err)
		}
		if !strings.Contains(content, "test panic") {
			t.Error("Expected crash log to contain panic value")
		}
	}
}

func TestCrashHandler_CleanOldLogs(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	basePath := filepath.Join(tmpDir, ".taskwing")
	crashDir := filepath.Join(basePath, CrashLogDir)

	if err := os.MkdirAll(crashDir, 0755); err != nil {
		t.Fatalf("Failed to create crash dir: %v", err)
	}

	// Set up context
	globalContext = &CrashContext{basePath: basePath}

	// Create more than MaxCrashLogs files
	for i := range MaxCrashLogs + 5 {
		filename := filepath.Join(crashDir, "crash_20250101_1200"+string(rune('0'+i%10))+string(rune('0'+i/10))+".log")
		if err := os.WriteFile(filename, []byte("test"), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
	}

	// Clean old logs
	if err := cleanOldCrashLogs(crashDir); err != nil {
		t.Fatalf("cleanOldCrashLogs failed: %v", err)
	}

	// Verify count
	logs, err := ListCrashLogs()
	if err != nil {
		t.Fatalf("ListCrashLogs failed: %v", err)
	}
	if len(logs) != MaxCrashLogs {
		t.Errorf("Expected %d crash logs after cleanup, got %d", MaxCrashLogs, len(logs))
	}
}

func TestCrashHandler_GetCrashLogPath(t *testing.T) {
	globalContext = &CrashContext{basePath: "/tmp/test"}

	testTime := time.Date(2025, 1, 15, 14, 30, 45, 0, time.UTC)
	path := getCrashLogPath(testTime)

	expectedPath := "/tmp/test/crash_logs/crash_20250115_143045.log"
	if path != expectedPath {
		t.Errorf("Expected path '%s', got '%s'", expectedPath, path)
	}
}

func TestCrashHandler_DefaultBasePath(t *testing.T) {
	// Reset global context with empty basePath
	globalContext = &CrashContext{}

	dir := getCrashLogDir()
	expected := ".taskwing/crash_logs"
	if dir != expected {
		t.Errorf("Expected default dir '%s', got '%s'", expected, dir)
	}
}
