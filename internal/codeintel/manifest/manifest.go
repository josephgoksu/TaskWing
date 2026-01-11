// Package manifest provides scanners for parsing dependency lockfiles
// from various package managers (npm, Python, Rust).
package manifest

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Dependency represents a parsed dependency from a lockfile.
type Dependency struct {
	Name        string            `json:"name"`
	Version     string            `json:"version"`
	Resolved    string            `json:"resolved,omitempty"`    // URL or path where package was resolved from
	Integrity   string            `json:"integrity,omitempty"`   // Hash for verification (npm uses this)
	Dev         bool              `json:"dev,omitempty"`         // Whether this is a dev dependency
	Source      string            `json:"source,omitempty"`      // Source type (registry, git, path, etc.)
	Ecosystem   string            `json:"ecosystem"`             // npm, pypi, crates.io
	LockfileRef string            `json:"lockfile_ref"`          // Path to the lockfile this came from
	Extras      map[string]string `json:"extras,omitempty"`      // Additional metadata
}

// ScanResult contains all dependencies extracted from a lockfile.
type ScanResult struct {
	Lockfile     string       `json:"lockfile"`
	Ecosystem    string       `json:"ecosystem"`
	Dependencies []Dependency `json:"dependencies"`
}

// ManifestScanner defines the interface for lockfile parsers.
type ManifestScanner interface {
	// Name returns the scanner name (e.g., "npm", "poetry", "cargo")
	Name() string

	// SupportedFiles returns glob patterns for files this scanner handles
	SupportedFiles() []string

	// CanScan checks if the scanner can handle the given file
	CanScan(path string) bool

	// Scan parses a lockfile and returns extracted dependencies
	Scan(path string) (*ScanResult, error)
}

// ScanDirectory scans a directory for all supported lockfiles using the provided scanners.
func ScanDirectory(dir string, scanners []ManifestScanner) ([]ScanResult, error) {
	var results []ScanResult

	for _, scanner := range scanners {
		for _, pattern := range scanner.SupportedFiles() {
			matches, err := filepath.Glob(filepath.Join(dir, pattern))
			if err != nil {
				continue
			}

			for _, match := range matches {
				if !scanner.CanScan(match) {
					continue
				}

				result, err := scanner.Scan(match)
				if err != nil {
					// Log but continue scanning other files
					continue
				}

				if result != nil && len(result.Dependencies) > 0 {
					results = append(results, *result)
				}
			}
		}
	}

	return results, nil
}

// AllScanners returns all available manifest scanners.
func AllScanners() []ManifestScanner {
	return []ManifestScanner{
		NewNpmScanner(),
		NewPythonScanner(),
		NewCargoScanner(),
	}
}

// readJSON is a helper to read and parse JSON files.
func readJSON(path string, v any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}
