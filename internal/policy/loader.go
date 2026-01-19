package policy

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/afero"
)

// DefaultPoliciesDir is the default directory for policy files relative to .taskwing.
const DefaultPoliciesDir = "policies"

// PolicyFile represents a loaded Rego policy file.
type PolicyFile struct {
	// Path is the absolute path to the policy file.
	Path string `json:"path"`
	// Name is the base name of the file without extension.
	Name string `json:"name"`
	// Content is the raw Rego source code.
	Content string `json:"content"`
}

// Loader scans and loads .rego policy files from the configured directory.
// It uses an afero.Fs interface for filesystem operations, enabling
// easy testing with in-memory filesystems.
type Loader struct {
	fs      afero.Fs
	baseDir string // Base directory (e.g., .taskwing/policies)
}

// NewLoader creates a new policy loader using the provided filesystem.
// The baseDir should be the path to the policies directory.
// Use afero.NewOsFs() for real filesystem operations,
// or afero.NewMemMapFs() for testing.
func NewLoader(fs afero.Fs, baseDir string) *Loader {
	return &Loader{
		fs:      fs,
		baseDir: baseDir,
	}
}

// NewOsLoader creates a Loader using the real operating system filesystem.
// The baseDir should be the absolute path to the policies directory.
func NewOsLoader(baseDir string) *Loader {
	return NewLoader(afero.NewOsFs(), baseDir)
}

// LoadAll loads all .rego policy files from the configured directory.
// It returns a slice of PolicyFile containing the path and content of each file.
// Subdirectories are scanned recursively.
func (l *Loader) LoadAll() ([]*PolicyFile, error) {
	// Check if directory exists
	exists, err := afero.DirExists(l.fs, l.baseDir)
	if err != nil {
		return nil, fmt.Errorf("check policies directory: %w", err)
	}
	if !exists {
		// Return empty slice if directory doesn't exist (no policies configured)
		return []*PolicyFile{}, nil
	}

	var policies []*PolicyFile

	err = afero.Walk(l.fs, l.baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Only load .rego files
		if !strings.HasSuffix(info.Name(), ".rego") {
			return nil
		}

		policy, err := l.loadFile(path)
		if err != nil {
			return fmt.Errorf("load policy %s: %w", path, err)
		}

		policies = append(policies, policy)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("walk policies directory: %w", err)
	}

	return policies, nil
}

// LoadFile loads a single .rego policy file by path.
func (l *Loader) LoadFile(path string) (*PolicyFile, error) {
	return l.loadFile(path)
}

// loadFile reads a policy file and returns its content.
func (l *Loader) loadFile(path string) (*PolicyFile, error) {
	file, err := l.fs.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}
	defer func() { _ = file.Close() }()

	content, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	// Extract name from filename (without .rego extension)
	name := strings.TrimSuffix(filepath.Base(path), ".rego")

	return &PolicyFile{
		Path:    path,
		Name:    name,
		Content: string(content),
	}, nil
}

// Exists checks if the policies directory exists.
func (l *Loader) Exists() (bool, error) {
	return afero.DirExists(l.fs, l.baseDir)
}

// ListFiles returns the paths of all .rego files in the policies directory.
// This is a lightweight alternative to LoadAll when only paths are needed.
func (l *Loader) ListFiles() ([]string, error) {
	exists, err := afero.DirExists(l.fs, l.baseDir)
	if err != nil {
		return nil, fmt.Errorf("check policies directory: %w", err)
	}
	if !exists {
		return []string{}, nil
	}

	var paths []string

	err = afero.Walk(l.fs, l.baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		if strings.HasSuffix(info.Name(), ".rego") {
			paths = append(paths, path)
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("walk policies directory: %w", err)
	}

	return paths, nil
}

// GetPoliciesPath constructs the full path to the policies directory
// given a project root path.
func GetPoliciesPath(projectRoot string) string {
	return filepath.Join(projectRoot, ".taskwing", DefaultPoliciesDir)
}
