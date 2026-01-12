package config

import (
	"os"
	"path/filepath"
	"sync"

	"github.com/josephgoksu/TaskWing/internal/project"
	"github.com/spf13/viper"
)

// projectContext holds the detected project context.
// This is set during CLI initialization and used by GetMemoryBasePath.
var (
	projectContext     *project.Context
	projectContextOnce sync.Once
	projectContextMu   sync.RWMutex
)

// GetGlobalConfigDir returns the path to the global configuration directory (~/.taskwing).
// This is the source of truth for where global config lives.
// It's a variable to allow overriding in tests.
var GetGlobalConfigDir = func() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".taskwing"), nil
}

// SetProjectContext sets the detected project context for use by GetMemoryBasePath.
// This should be called during CLI initialization (PersistentPreRun).
func SetProjectContext(ctx *project.Context) {
	projectContextMu.Lock()
	defer projectContextMu.Unlock()
	projectContext = ctx
}

// GetProjectContext returns the detected project context.
// Returns nil if no context has been set.
func GetProjectContext() *project.Context {
	projectContextMu.RLock()
	defer projectContextMu.RUnlock()
	return projectContext
}

// DetectProjectContext detects the project root from the current working directory.
// This is called lazily on first access to ensure consistent behavior.
// Returns the detected context, or nil if detection fails.
func DetectProjectContext() *project.Context {
	projectContextOnce.Do(func() {
		cwd, err := os.Getwd()
		if err != nil {
			return
		}
		ctx, err := project.Detect(cwd)
		if err != nil {
			return
		}
		SetProjectContext(ctx)
	})
	return GetProjectContext()
}

// GetMemoryBasePath returns the path to the memory directory.
// Resolution order (first match wins):
// 1. Explicit config via "memory.path" (Viper/env/flag)
// 2. Detected project root: {project_root}/.taskwing/memory
// 3. Local project directory: .taskwing/memory (if exists) - legacy fallback
// 4. XDG_DATA_HOME/taskwing/memory (if XDG_DATA_HOME is set)
// 5. Global fallback: ~/.taskwing/memory
func GetMemoryBasePath() string {
	// 1. Check Viper config (flags/config file/env)
	if path := viper.GetString("memory.path"); path != "" {
		return path
	}

	// 2. Check detected project context
	if ctx := GetProjectContext(); ctx != nil && ctx.RootPath != "" {
		projectMemory := filepath.Join(ctx.RootPath, ".taskwing", "memory")
		// Only use if .taskwing exists at project root
		if info, err := os.Stat(filepath.Join(ctx.RootPath, ".taskwing")); err == nil && info.IsDir() {
			return projectMemory
		}
	}

	// 3. Check for local project .taskwing/memory directory (legacy fallback)
	// This allows per-project isolation when running from within a project
	localMemory := ".taskwing/memory"
	if info, err := os.Stat(localMemory); err == nil && info.IsDir() {
		return localMemory
	}

	// 4. Check XDG_DATA_HOME
	if xdgData := os.Getenv("XDG_DATA_HOME"); xdgData != "" {
		return filepath.Join(xdgData, "taskwing", "memory")
	}

	// 5. Fallback to ~/.taskwing/memory (global)
	dir, err := GetGlobalConfigDir()
	if err != nil {
		return "./memory"
	}
	return filepath.Join(dir, "memory")
}

// GetProjectRoot returns the detected project root path.
// Falls back to current working directory if detection hasn't been run.
func GetProjectRoot() string {
	if ctx := GetProjectContext(); ctx != nil {
		return ctx.RootPath
	}
	// Fallback to cwd
	cwd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return cwd
}
