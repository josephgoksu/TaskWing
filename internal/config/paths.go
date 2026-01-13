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
func SetProjectContext(ctx *project.Context) {
	if ctx == nil {
		panic("SetProjectContext called with nil context")
	}
	projectContextMu.Lock()
	defer projectContextMu.Unlock()
	projectContext = ctx
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

// MustGetProjectContext returns the project context or panics if not set.
// Use this when project context is required and absence is a programming error.
func MustGetProjectContext() *project.Context {
	ctx := GetProjectContext()
	if ctx == nil {
		panic(ErrProjectContextNotSet)
	}
	return ctx
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

	SetProjectContext(ctx)
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
func GetMemoryBasePathOrGlobal() string {
	path, err := GetMemoryBasePath()
	if err == nil {
		return path
	}

	// Only fall back to global for non-project commands
	dir, err := GetGlobalConfigDir()
	if err != nil {
		// This is a critical failure - can't determine any valid path
		panic(fmt.Sprintf("cannot determine memory path: %v", err))
	}
	return filepath.Join(dir, "memory")
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

// MustGetProjectRoot returns the project root or panics.
// Use when project root is required and absence is a programming error.
func MustGetProjectRoot() string {
	root, err := GetProjectRoot()
	if err != nil {
		panic(err)
	}
	return root
}
