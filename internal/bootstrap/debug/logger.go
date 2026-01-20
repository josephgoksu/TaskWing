// Package debug provides debug logging utilities for bootstrap.
package debug

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// LogLevel represents the severity of a log entry.
type LogLevel string

const (
	LevelDebug LogLevel = "debug"
	LevelInfo  LogLevel = "info"
	LevelWarn  LogLevel = "warn"
	LevelError LogLevel = "error"
)

// LogEntry represents a single JSONL log entry.
type LogEntry struct {
	Timestamp  string         `json:"timestamp"`
	Level      LogLevel       `json:"level"`
	Component  string         `json:"component"`
	Event      string         `json:"event"`
	Message    string         `json:"message"`
	DurationMs *int64         `json:"duration_ms,omitempty"`
	Metadata   map[string]any `json:"metadata,omitempty"`
	Agent      string         `json:"agent,omitempty"`
	Phase      string         `json:"phase,omitempty"`
	Error      string         `json:"error,omitempty"`
}

// Options configures the DebugLogger.
type Options struct {
	// OutputDir is the directory for log files.
	// Default: ".taskwing/logs"
	OutputDir string

	// EnableStderr controls whether logs are also written to stderr.
	// Default: true
	EnableStderr bool

	// RetentionCount is the number of log files to keep.
	// Default: 5
	RetentionCount int

	// Component is the default component name for log entries.
	// Default: "bootstrap"
	Component string

	// StderrWriter is the writer for stderr output (for testing).
	// Default: os.Stderr
	StderrWriter io.Writer
}

// DebugLogger provides structured debug logging to file and stderr.
type DebugLogger struct {
	mu           sync.Mutex
	file         *os.File
	writer       *bufio.Writer
	opts         Options
	startTime    time.Time
	logPath      string
	closed       bool
	flushTicker  *time.Ticker
	flushDone    chan struct{}
	stderrWriter io.Writer
}

// NewDebugLogger creates a new debug logger.
// It creates the output directory if it doesn't exist.
func NewDebugLogger(opts Options) (*DebugLogger, error) {
	// Apply defaults
	if opts.OutputDir == "" {
		opts.OutputDir = ".taskwing/logs"
	}
	if opts.RetentionCount == 0 {
		opts.RetentionCount = 5
	}
	if opts.Component == "" {
		opts.Component = "bootstrap"
	}
	stderrWriter := opts.StderrWriter
	if stderrWriter == nil {
		stderrWriter = os.Stderr
	}

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(opts.OutputDir, 0o755); err != nil {
		return nil, fmt.Errorf("create logs directory: %w", err)
	}

	// Generate log filename with timestamp
	now := time.Now().UTC()
	filename := fmt.Sprintf("debug-%s.log", now.Format("20060102T150405Z"))
	logPath := filepath.Join(opts.OutputDir, filename)

	// Create log file with restricted permissions
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, fmt.Errorf("create log file: %w", err)
	}

	// Create symlink to latest log
	latestLink := filepath.Join(opts.OutputDir, "debug-latest.log")
	_ = os.Remove(latestLink) // Remove old symlink if exists
	_ = os.Symlink(filename, latestLink)

	l := &DebugLogger{
		file:         file,
		writer:       bufio.NewWriter(file),
		opts:         opts,
		startTime:    now,
		logPath:      logPath,
		flushTicker:  time.NewTicker(1 * time.Second),
		flushDone:    make(chan struct{}),
		stderrWriter: stderrWriter,
	}

	// Start periodic flush goroutine
	go l.periodicFlush()

	return l, nil
}

// periodicFlush flushes the buffer every second.
func (l *DebugLogger) periodicFlush() {
	for {
		select {
		case <-l.flushTicker.C:
			l.mu.Lock()
			if !l.closed && l.writer != nil {
				_ = l.writer.Flush()
			}
			l.mu.Unlock()
		case <-l.flushDone:
			return
		}
	}
}

// log writes a log entry at the specified level.
func (l *DebugLogger) log(level LogLevel, event, message string, metadata map[string]any) {
	l.logWithFields(level, event, message, metadata, "", "", nil)
}

// logWithFields writes a log entry with all possible fields.
func (l *DebugLogger) logWithFields(level LogLevel, event, message string, metadata map[string]any, agent, phase string, durationMs *int64) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.closed {
		return
	}

	entry := LogEntry{
		Timestamp:  time.Now().UTC().Format(time.RFC3339Nano),
		Level:      level,
		Component:  l.opts.Component,
		Event:      event,
		Message:    message,
		Metadata:   metadata,
		Agent:      agent,
		Phase:      phase,
		DurationMs: durationMs,
	}

	// Marshal to JSON
	data, err := json.Marshal(entry)
	if err != nil {
		// If we can't marshal, write a fallback message
		data = []byte(fmt.Sprintf(`{"timestamp":"%s","level":"error","component":"%s","event":"marshal_error","message":"failed to marshal log entry"}`,
			entry.Timestamp, l.opts.Component))
	}

	// Write to file
	if l.writer != nil {
		_, _ = l.writer.Write(data)
		_, _ = l.writer.Write([]byte("\n"))
	}

	// Write to stderr if enabled
	if l.opts.EnableStderr && l.stderrWriter != nil {
		_, _ = l.stderrWriter.Write(data)
		_, _ = l.stderrWriter.Write([]byte("\n"))
	}
}

// Debug logs a debug-level message.
func (l *DebugLogger) Debug(event, message string, metadata map[string]any) {
	l.log(LevelDebug, event, message, metadata)
}

// Info logs an info-level message.
func (l *DebugLogger) Info(event, message string, metadata map[string]any) {
	l.log(LevelInfo, event, message, metadata)
}

// Warn logs a warning-level message.
func (l *DebugLogger) Warn(event, message string, metadata map[string]any) {
	l.log(LevelWarn, event, message, metadata)
}

// Error logs an error-level message.
func (l *DebugLogger) Error(event, message string, metadata map[string]any) {
	l.log(LevelError, event, message, metadata)
}

// ErrorWithErr logs an error-level message with an error object.
func (l *DebugLogger) ErrorWithErr(event, message string, err error, metadata map[string]any) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.closed {
		return
	}

	errStr := ""
	if err != nil {
		errStr = err.Error()
	}

	entry := LogEntry{
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		Level:     LevelError,
		Component: l.opts.Component,
		Event:     event,
		Message:   message,
		Metadata:  metadata,
		Error:     errStr,
	}

	data, _ := json.Marshal(entry)

	if l.writer != nil {
		_, _ = l.writer.Write(data)
		_, _ = l.writer.Write([]byte("\n"))
	}

	if l.opts.EnableStderr && l.stderrWriter != nil {
		_, _ = l.stderrWriter.Write(data)
		_, _ = l.stderrWriter.Write([]byte("\n"))
	}
}

// WithComponent returns a new logger with a different component name.
// The new logger shares the same file and options.
func (l *DebugLogger) WithComponent(component string) *DebugLogger {
	return &DebugLogger{
		file:         l.file,
		writer:       l.writer,
		opts:         Options{OutputDir: l.opts.OutputDir, EnableStderr: l.opts.EnableStderr, RetentionCount: l.opts.RetentionCount, Component: component},
		startTime:    l.startTime,
		logPath:      l.logPath,
		flushTicker:  l.flushTicker,
		flushDone:    l.flushDone,
		stderrWriter: l.stderrWriter,
	}
}

// WithAgent logs with an agent field set.
func (l *DebugLogger) WithAgent(agent string) *AgentLogger {
	return &AgentLogger{
		logger: l,
		agent:  agent,
	}
}

// AgentLogger is a logger scoped to a specific agent.
type AgentLogger struct {
	logger *DebugLogger
	agent  string
}

// Info logs an info message for this agent.
func (a *AgentLogger) Info(event, message string, metadata map[string]any) {
	a.logger.logWithFields(LevelInfo, event, message, metadata, a.agent, "", nil)
}

// Debug logs a debug message for this agent.
func (a *AgentLogger) Debug(event, message string, metadata map[string]any) {
	a.logger.logWithFields(LevelDebug, event, message, metadata, a.agent, "", nil)
}

// Error logs an error message for this agent.
func (a *AgentLogger) Error(event, message string, metadata map[string]any) {
	a.logger.logWithFields(LevelError, event, message, metadata, a.agent, "", nil)
}

// StartPhase begins timing a phase and returns a stopper function.
// The stopper logs the phase completion with duration.
func (l *DebugLogger) StartPhase(phase string, metadata map[string]any) func(err error) {
	start := time.Now()

	// Log phase start
	l.logWithFields(LevelDebug, "phase_start", fmt.Sprintf("Starting %s", phase), metadata, "", phase, nil)

	// Return stopper function
	return func(err error) {
		duration := time.Since(start).Milliseconds()

		if err != nil {
			errMeta := metadata
			if errMeta == nil {
				errMeta = make(map[string]any)
			}
			errMeta["error"] = err.Error()
			l.logWithFields(LevelError, "phase_error", fmt.Sprintf("Phase %s failed", phase), errMeta, "", phase, &duration)
		} else {
			l.logWithFields(LevelDebug, "phase_end", fmt.Sprintf("Completed %s", phase), metadata, "", phase, &duration)
		}
	}
}

// LogPath returns the path to the current log file.
func (l *DebugLogger) LogPath() string {
	return l.logPath
}

// Close flushes buffers, closes the file, and runs retention pruning.
func (l *DebugLogger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.closed {
		return nil
	}

	l.closed = true

	// Stop periodic flush
	l.flushTicker.Stop()
	close(l.flushDone)

	// Flush and close file
	var errs []error
	if l.writer != nil {
		if err := l.writer.Flush(); err != nil {
			errs = append(errs, fmt.Errorf("flush: %w", err))
		}
	}
	if l.file != nil {
		if err := l.file.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close: %w", err))
		}
	}

	// Run retention pruning (outside lock)
	l.mu.Unlock()
	if err := l.pruneOldLogs(); err != nil {
		errs = append(errs, fmt.Errorf("prune: %w", err))
	}
	l.mu.Lock()

	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}

// pruneOldLogs removes old log files beyond the retention count.
func (l *DebugLogger) pruneOldLogs() error {
	return PruneLogFiles(l.opts.OutputDir, l.opts.RetentionCount)
}
