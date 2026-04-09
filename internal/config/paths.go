package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/josephgoksu/TaskWing/internal/project"
	"github.com/spf13/viper"
)

// Errors for fail-fast behavior
var (
	ErrProjectContextNotSet = errors.New("project context not initialized: call SetProjectContext during CLI init")
	ErrNoTaskWingDir        = errors.New("no .taskwing directory found at project root")
	ErrDetectionFailed      = errors.New("project detection failed")
)

// projectContext holds the detected project context.
// This is set during CLI initialization and used by GetMemoryBasePath.
var (
	projectContext   *project.Context
	projectContextMu sync.RWMutex
)

// GetGlobalConfigDir returns the path to the global configuration directory (~/.taskwing).
// This is the source of truth for where global config lives.
// It's a variable to allow overriding in tests.
var GetGlobalConfigDir = func() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home directory: %w", err)
	}
	return filepath.Join(home, ".taskwing"), nil
}

// SetProjectContext sets the detected project context for use by GetMemoryBasePath.
// This MUST be called during CLI initialization before any command that needs project context.
// Returns error if ctx is nil.
func SetProjectContext(ctx *project.Context) error {
	if ctx == nil {
		return errors.New("SetProjectContext called with nil context")
	}
	projectContextMu.Lock()
	defer projectContextMu.Unlock()
	projectContext = ctx
	return nil
}

// ClearProjectContext resets the project context. Only use in tests.
func ClearProjectContext() {
	projectContextMu.Lock()
	defer projectContextMu.Unlock()
	projectContext = nil
}

// GetProjectContext returns the detected project context.
// Returns nil if no context has been set - callers must check.
func GetProjectContext() *project.Context {
	projectContextMu.RLock()
	defer projectContextMu.RUnlock()
	return projectContext
}

// DetectAndSetProjectContext detects the project root and sets it.
// Returns error if detection fails - no silent fallbacks.
func DetectAndSetProjectContext() (*project.Context, error) {
	// Return existing context if already set
	if ctx := GetProjectContext(); ctx != nil {
		return ctx, nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("get working directory: %w", err)
	}

	ctx, err := project.Detect(cwd)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDetectionFailed, err)
	}

	if err := SetProjectContext(ctx); err != nil {
		return nil, fmt.Errorf("set project context: %w", err)
	}
	return ctx, nil
}

// GetMemoryBasePath returns the path to the memory directory.
// Resolution order (deterministic, no fallbacks):
// 1. Explicit config via "memory.path" (Viper/env/flag)
// 2. Detected project root: {project_root}/.taskwing/memory
//
// Returns error if no valid path can be determined.
func GetMemoryBasePath() (string, error) {
	// 1. Check Viper config (flags/config file/env) - explicit override always wins
	if path := viper.GetString("memory.path"); path != "" {
		return path, nil
	}

	// 2. Use detected project context - REQUIRED
	ctx := GetProjectContext()
	if ctx == nil {
		return "", ErrProjectContextNotSet
	}

	if ctx.RootPath == "" {
		return "", fmt.Errorf("project context has empty RootPath")
	}

	// Reject CWD-fallback contexts (MarkerNone) to prevent accidental writes to HOME.
	// A project must have at least a .git, language manifest, or .taskwing marker.
	if ctx.MarkerType == project.MarkerNone {
		return "", fmt.Errorf("no project marker found at %q: run 'taskwing bootstrap' in a project directory", ctx.RootPath)
	}

	taskwingDir := filepath.Join(ctx.RootPath, ".taskwing")
	if info, err := os.Stat(taskwingDir); err != nil || !info.IsDir() {
		return "", fmt.Errorf("%w: %s", ErrNoTaskWingDir, taskwingDir)
	}

	return filepath.Join(taskwingDir, "memory"), nil
}

// GetMemoryBasePathOrGlobal returns memory path, falling back to global ~/.taskwing/memory.
//
// USAGE POLICY - Only use this function for:
//   - MCP server (may run in sandboxed environments without project context)
//   - Hook commands (may run before project context is established)
//   - Non-project commands (help, version, etc.)
//
// ALL OTHER COMMANDS should use GetMemoryBasePath() which enforces fail-fast behavior.
// Using this function inappropriately masks project detection failures.
func GetMemoryBasePathOrGlobal() (string, error) {
	path, err := GetMemoryBasePath()
	if err == nil {
		return path, nil
	}

	// Only fall back to global for non-project commands
	dir, err := GetGlobalConfigDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine memory path: %w", err)
	}
	return filepath.Join(dir, "memory"), nil
}

// AutonomousModeMarkerName is the marker file written when the user explicitly
// invokes autonomous task execution (e.g. via /taskwing:next or task action=next).
// The continue-check Stop hook only auto-continues to the next task when this
// marker exists. Without it, ANY assistant turn that ends would trigger the
// hook to start executing tasks - even harmless commands like /taskwing:context.
const AutonomousModeMarkerName = ".autonomous_mode"

// MarkAutonomousMode writes the autonomous mode marker file to the project's
// memory directory. Called by the MCP task next handler when the user
// explicitly starts task execution. Failure to write is non-fatal.
func MarkAutonomousMode() {
	memoryPath, err := GetMemoryBasePath()
	if err != nil {
		return
	}
	markerPath := filepath.Join(memoryPath, AutonomousModeMarkerName)
	_ = os.WriteFile(markerPath, []byte("1"), 0644)
}

// IsAutonomousMode returns true if the autonomous mode marker exists in the
// project's memory directory. Used by the continue-check hook to decide
// whether to block the assistant turn and continue to the next task.
func IsAutonomousMode(memoryPath string) bool {
	if memoryPath == "" {
		return false
	}
	_, err := os.Stat(filepath.Join(memoryPath, AutonomousModeMarkerName))
	return err == nil
}

// ClearAutonomousMode removes the autonomous mode marker. Called by the
// session-end hook to ensure a fresh session does not auto-continue.
func ClearAutonomousMode(memoryPath string) {
	if memoryPath == "" {
		return
	}
	_ = os.Remove(filepath.Join(memoryPath, AutonomousModeMarkerName))
}

// GetProjectRoot returns the detected project root path.
// Returns error if project context is not set.
func GetProjectRoot() (string, error) {
	ctx := GetProjectContext()
	if ctx == nil {
		return "", ErrProjectContextNotSet
	}
	if ctx.RootPath == "" {
		return "", fmt.Errorf("project context has empty RootPath")
	}
	return ctx.RootPath, nil
}
