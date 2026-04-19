package config

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/josephgoksu/TaskWing/internal/project"
	"github.com/spf13/viper"
)

// Errors for fail-fast behavior
var (
	ErrProjectContextNotSet = errors.New("project context not initialized: call SetProjectContext during CLI init")
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

// ProjectSlug generates a human-readable, collision-resistant directory name for a project.
// Format: <basename>-<sha256[:6]> (e.g., "taskwing-a1b2c3").
// Symlinks are resolved before hashing to prevent duplicate slugs.
func ProjectSlug(rootPath string) string {
	resolved, err := filepath.EvalSymlinks(rootPath)
	if err != nil {
		resolved = rootPath
	}
	hash := sha256.Sum256([]byte(resolved))
	shortHash := hex.EncodeToString(hash[:3]) // 6 hex chars
	return filepath.Base(resolved) + "-" + shortHash
}

// GetProjectStorePath returns the global storage path for a project.
// Creates the directory and registers the project in the index.
// Path: ~/.taskwing/projects/<slug>/
func GetProjectStorePath(rootPath string) (string, error) {
	globalDir, err := GetGlobalConfigDir()
	if err != nil {
		return "", err
	}

	slug := ProjectSlug(rootPath)
	storePath := filepath.Join(globalDir, "projects", slug)

	if err := os.MkdirAll(storePath, 0700); err != nil {
		return "", fmt.Errorf("create project store: %w", err)
	}

	// Register project in index (non-fatal if it fails)
	_ = registerProject(globalDir, slug, rootPath)

	return storePath, nil
}

// ProjectEntry represents a registered project in the index.
type ProjectEntry struct {
	Slug         string    `json:"slug"`
	RootPath     string    `json:"root_path"`
	LastAccessed time.Time `json:"last_accessed"`
}

// registerProject updates the project index with the given project.
// Uses atomic write (temp file + rename) to prevent corruption.
func registerProject(globalDir, slug, rootPath string) error {
	indexPath := filepath.Join(globalDir, "projects", "index.json")

	var entries []ProjectEntry
	if data, err := os.ReadFile(indexPath); err == nil {
		_ = json.Unmarshal(data, &entries)
	}

	// Update or append
	found := false
	now := time.Now()
	for i := range entries {
		if entries[i].Slug == slug {
			entries[i].RootPath = rootPath
			entries[i].LastAccessed = now
			found = true
			break
		}
	}
	if !found {
		entries = append(entries, ProjectEntry{
			Slug:         slug,
			RootPath:     rootPath,
			LastAccessed: now,
		})
	}

	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}

	// Atomic write: temp file + rename
	tmpPath := indexPath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return err
	}
	return os.Rename(tmpPath, indexPath)
}

// ListRegisteredProjects returns all projects registered in the global index.
func ListRegisteredProjects() ([]ProjectEntry, error) {
	globalDir, err := GetGlobalConfigDir()
	if err != nil {
		return nil, err
	}
	indexPath := filepath.Join(globalDir, "projects", "index.json")
	data, err := os.ReadFile(indexPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var entries []ProjectEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, err
	}
	return entries, nil
}

// GetMemoryBasePath returns the path to the memory directory.
// Resolution order (deterministic, no fallbacks):
// 1. Explicit config via "memory.path" (Viper/env/flag)
// 2. Global project store: ~/.taskwing/projects/<slug>/
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
	// A project must have at least a .git or language manifest marker.
	if ctx.MarkerType == project.MarkerNone {
		return "", fmt.Errorf("no project marker found at %q: run 'taskwing bootstrap' in a project directory", ctx.RootPath)
	}

	return GetProjectStorePath(ctx.RootPath)
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

// GetGlobalKnowledgePath returns the path to the global knowledge database directory.
// Does NOT create the directory -- callers that write should ensure it exists.
func GetGlobalKnowledgePath() (string, error) {
	dir, err := GetGlobalConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "knowledge"), nil
}

// EnsureGlobalKnowledgePath returns the global knowledge path, creating it if needed.
// Use this only when writing to the global knowledge DB.
func EnsureGlobalKnowledgePath() (string, error) {
	knowledgePath, err := GetGlobalKnowledgePath()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(knowledgePath, 0755); err != nil {
		return "", fmt.Errorf("create global knowledge dir: %w", err)
	}
	return knowledgePath, nil
}

// GetProfilePath returns the path to a named profile config file.
// Rejects names containing path separators to prevent directory traversal.
func GetProfilePath(name string) (string, error) {
	if name == "" || strings.Contains(name, "..") || filepath.Base(name) != name {
		return "", fmt.Errorf("invalid profile name: %q", name)
	}
	dir, err := GetGlobalConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "profiles", name+".yaml"), nil
}

// ListProfiles returns the names of all available config profiles.
func ListProfiles() ([]string, error) {
	dir, err := GetGlobalConfigDir()
	if err != nil {
		return nil, err
	}
	profileDir := filepath.Join(dir, "profiles")
	entries, err := os.ReadDir(profileDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".yaml" {
			names = append(names, strings.TrimSuffix(e.Name(), ".yaml"))
		}
	}
	return names, nil
}

// GetProjectConfigPath returns the path to the project config in the global store.
// Returns empty string if no project context is set.
func GetProjectConfigPath() string {
	ctx := GetProjectContext()
	if ctx == nil || ctx.RootPath == "" {
		return ""
	}
	storePath, err := GetProjectStorePath(ctx.RootPath)
	if err != nil {
		return ""
	}
	return filepath.Join(storePath, "config.yaml")
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
