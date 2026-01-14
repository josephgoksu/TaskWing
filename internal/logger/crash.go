// Package logger provides crash logging and recovery for TaskWing.
package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"time"
)

const (
	// CrashLogDir is the directory for crash logs relative to .taskwing
	CrashLogDir = "crash_logs"

	// MaxCrashLogs is the maximum number of crash logs to keep
	MaxCrashLogs = 10
)

// CrashContext stores context for crash logging.
type CrashContext struct {
	mu         sync.RWMutex
	lastInput  string
	lastPrompt string
	command    string
	version    string
	basePath   string
}

// globalContext is the singleton crash context.
var globalContext = &CrashContext{}

// SetBasePath sets the base path for crash logs (typically .taskwing directory).
func SetBasePath(path string) {
	globalContext.mu.Lock()
	defer globalContext.mu.Unlock()
	globalContext.basePath = path
}

// SetVersion sets the application version for crash logs.
func SetVersion(version string) {
	globalContext.mu.Lock()
	defer globalContext.mu.Unlock()
	globalContext.version = version
}

// SetCommand sets the current command being executed.
func SetCommand(cmd string) {
	globalContext.mu.Lock()
	defer globalContext.mu.Unlock()
	globalContext.command = cmd
}

// SetLastInput sets the last user input for crash context.
func SetLastInput(input string) {
	globalContext.mu.Lock()
	defer globalContext.mu.Unlock()
	globalContext.lastInput = truncateForLog(strings.TrimSpace(input), 500)
}

// SetLastPrompt sets the last LLM prompt for crash context.
func SetLastPrompt(prompt string) {
	globalContext.mu.Lock()
	defer globalContext.mu.Unlock()
	globalContext.lastPrompt = truncateForLog(prompt, 2000)
}

func truncateForLog(value string, maxLen int) string {
	if len(value) <= maxLen {
		return value
	}
	return value[:maxLen] + "... [truncated]"
}

// CrashLog represents a crash log entry.
type CrashLog struct {
	Timestamp  time.Time `json:"timestamp"`
	Version    string    `json:"version"`
	Command    string    `json:"command"`
	PanicValue string    `json:"panic_value"`
	StackTrace string    `json:"stack_trace"`
	LastInput  string    `json:"last_input,omitempty"`
	LastPrompt string    `json:"last_prompt,omitempty"`
	GoVersion  string    `json:"go_version"`
	OS         string    `json:"os"`
	Arch       string    `json:"arch"`
}

// HandlePanic is a deferred function that recovers from panics and logs them.
// Usage: defer logger.HandlePanic()
func HandlePanic() {
	if r := recover(); r != nil {
		log := createCrashLog(r)
		if err := writeCrashLog(log); err != nil {
			// If we can't write to crash log, print to stderr
			fmt.Fprintf(os.Stderr, "\n[CRASH] Failed to write crash log: %v\n", err)
			fmt.Fprintf(os.Stderr, "[CRASH] Panic: %v\n%s\n", r, debug.Stack())
		}

		// Print user-friendly message
		fmt.Fprintf(os.Stderr, "\n")
		fmt.Fprintf(os.Stderr, "╭──────────────────────────────────────────────────────╮\n")
		fmt.Fprintf(os.Stderr, "│ 🔴 TaskWing encountered an unexpected error         │\n")
		fmt.Fprintf(os.Stderr, "╰──────────────────────────────────────────────────────╯\n")
		fmt.Fprintf(os.Stderr, "\n")
		fmt.Fprintf(os.Stderr, "A crash log has been saved to:\n")
		fmt.Fprintf(os.Stderr, "  %s\n", getCrashLogPath(log.Timestamp))
		fmt.Fprintf(os.Stderr, "\n")
		fmt.Fprintf(os.Stderr, "Please report this issue at:\n")
		fmt.Fprintf(os.Stderr, "  https://github.com/josephgoksu/TaskWing/issues\n")
		fmt.Fprintf(os.Stderr, "\n")

		// Exit with error code
		os.Exit(1)
	}
}

// createCrashLog creates a CrashLog from a panic value.
func createCrashLog(panicValue any) CrashLog {
	globalContext.mu.RLock()
	defer globalContext.mu.RUnlock()

	return CrashLog{
		Timestamp:  time.Now(),
		Version:    globalContext.version,
		Command:    globalContext.command,
		PanicValue: fmt.Sprintf("%v", panicValue),
		StackTrace: string(debug.Stack()),
		LastInput:  globalContext.lastInput,
		LastPrompt: globalContext.lastPrompt,
		GoVersion:  runtime.Version(),
		OS:         runtime.GOOS,
		Arch:       runtime.GOARCH,
	}
}

// writeCrashLog writes a crash log to disk.
func writeCrashLog(log CrashLog) error {
	dir := getCrashLogDir()

	// Create directory if it doesn't exist
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create crash log dir: %w", err)
	}

	// Clean up old crash logs
	if err := cleanOldCrashLogs(dir); err != nil {
		// Non-fatal, continue with writing
		fmt.Fprintf(os.Stderr, "[WARN] Failed to clean old crash logs: %v\n", err)
	}

	// Write crash log file
	path := getCrashLogPath(log.Timestamp)
	content := formatCrashLog(log)

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("write crash log: %w", err)
	}

	return nil
}

// getCrashLogDir returns the directory for crash logs.
func getCrashLogDir() string {
	globalContext.mu.RLock()
	basePath := globalContext.basePath
	globalContext.mu.RUnlock()

	if basePath == "" {
		// Default to current directory
		basePath = ".taskwing"
	}

	return filepath.Join(basePath, CrashLogDir)
}

// getCrashLogPath returns the path for a crash log file.
func getCrashLogPath(t time.Time) string {
	filename := fmt.Sprintf("crash_%s.log", t.Format("20060102_150405"))
	return filepath.Join(getCrashLogDir(), filename)
}

// formatCrashLog formats a CrashLog as human-readable text.
func formatCrashLog(log CrashLog) string {
	var sb strings.Builder

	sb.WriteString("=" + strings.Repeat("=", 79) + "\n")
	sb.WriteString("TASKWING CRASH LOG\n")
	sb.WriteString("=" + strings.Repeat("=", 79) + "\n\n")

	sb.WriteString(fmt.Sprintf("Timestamp: %s\n", log.Timestamp.Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("Version:   %s\n", log.Version))
	sb.WriteString(fmt.Sprintf("Command:   %s\n", log.Command))
	sb.WriteString(fmt.Sprintf("Go:        %s\n", log.GoVersion))
	sb.WriteString(fmt.Sprintf("OS/Arch:   %s/%s\n", log.OS, log.Arch))

	sb.WriteString("\n" + strings.Repeat("-", 80) + "\n")
	sb.WriteString("PANIC VALUE\n")
	sb.WriteString(strings.Repeat("-", 80) + "\n")
	sb.WriteString(log.PanicValue + "\n")

	sb.WriteString("\n" + strings.Repeat("-", 80) + "\n")
	sb.WriteString("STACK TRACE\n")
	sb.WriteString(strings.Repeat("-", 80) + "\n")
	sb.WriteString(log.StackTrace)

	if log.LastInput != "" {
		sb.WriteString("\n" + strings.Repeat("-", 80) + "\n")
		sb.WriteString("LAST USER INPUT\n")
		sb.WriteString(strings.Repeat("-", 80) + "\n")
		sb.WriteString(log.LastInput + "\n")
	}

	if log.LastPrompt != "" {
		sb.WriteString("\n" + strings.Repeat("-", 80) + "\n")
		sb.WriteString("LAST LLM PROMPT\n")
		sb.WriteString(strings.Repeat("-", 80) + "\n")
		sb.WriteString(log.LastPrompt + "\n")
	}

	sb.WriteString("\n" + strings.Repeat("=", 80) + "\n")
	sb.WriteString("END OF CRASH LOG\n")
	sb.WriteString(strings.Repeat("=", 80) + "\n")

	return sb.String()
}

// cleanOldCrashLogs removes old crash logs, keeping only MaxCrashLogs most recent.
func cleanOldCrashLogs(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	// Filter to only crash log files
	var crashLogs []os.DirEntry
	for _, e := range entries {
		if !e.IsDir() && strings.HasPrefix(e.Name(), "crash_") && strings.HasSuffix(e.Name(), ".log") {
			crashLogs = append(crashLogs, e)
		}
	}

	// If under limit, nothing to do
	if len(crashLogs) <= MaxCrashLogs {
		return nil
	}

	// Sort by name (which includes timestamp, so oldest first)
	// Note: os.ReadDir already returns sorted entries

	// Remove oldest files
	toRemove := len(crashLogs) - MaxCrashLogs
	for i := range toRemove {
		path := filepath.Join(dir, crashLogs[i].Name())
		if err := os.Remove(path); err != nil {
			return fmt.Errorf("remove old crash log %s: %w", crashLogs[i].Name(), err)
		}
	}

	return nil
}

// ListCrashLogs returns a list of all crash logs in the crash log directory.
func ListCrashLogs() ([]string, error) {
	dir := getCrashLogDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var logs []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasPrefix(e.Name(), "crash_") && strings.HasSuffix(e.Name(), ".log") {
			logs = append(logs, filepath.Join(dir, e.Name()))
		}
	}

	return logs, nil
}

// ReadCrashLog reads a crash log file.
func ReadCrashLog(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(content), nil
}
