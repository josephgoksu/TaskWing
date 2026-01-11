package manifest

import (
	"path/filepath"
	"strings"
)

// NpmScanner parses npm package-lock.json files.
type NpmScanner struct{}

// NewNpmScanner creates a new npm lockfile scanner.
func NewNpmScanner() *NpmScanner {
	return &NpmScanner{}
}

// Name returns the scanner name.
func (s *NpmScanner) Name() string {
	return "npm"
}

// SupportedFiles returns the glob patterns for npm lockfiles.
func (s *NpmScanner) SupportedFiles() []string {
	return []string{"package-lock.json"}
}

// CanScan checks if the file is a package-lock.json.
func (s *NpmScanner) CanScan(path string) bool {
	return filepath.Base(path) == "package-lock.json"
}

// Scan parses a package-lock.json and extracts dependencies.
func (s *NpmScanner) Scan(path string) (*ScanResult, error) {
	var lockfile packageLockJSON
	if err := readJSON(path, &lockfile); err != nil {
		return nil, err
	}

	result := &ScanResult{
		Lockfile:  path,
		Ecosystem: "npm",
	}

	// package-lock.json v2/v3 format uses "packages" field
	if lockfile.Packages != nil {
		for pkgPath, pkg := range lockfile.Packages {
			// Skip the root package (empty path)
			if pkgPath == "" {
				continue
			}

			// Extract package name from path (e.g., "node_modules/lodash" -> "lodash")
			name := extractPackageName(pkgPath)
			if name == "" {
				continue
			}

			dep := Dependency{
				Name:        name,
				Version:     pkg.Version,
				Resolved:    pkg.Resolved,
				Integrity:   pkg.Integrity,
				Dev:         pkg.Dev,
				Ecosystem:   "npm",
				LockfileRef: path,
			}

			if pkg.Optional {
				dep.Extras = map[string]string{"optional": "true"}
			}

			result.Dependencies = append(result.Dependencies, dep)
		}
	}

	// package-lock.json v1 format uses "dependencies" field
	if lockfile.Dependencies != nil && len(result.Dependencies) == 0 {
		s.extractV1Dependencies(lockfile.Dependencies, path, &result.Dependencies, false)
	}

	return result, nil
}

// extractV1Dependencies recursively extracts dependencies from v1 format.
func (s *NpmScanner) extractV1Dependencies(deps map[string]packageLockV1Dep, lockfilePath string, result *[]Dependency, dev bool) {
	for name, pkg := range deps {
		dep := Dependency{
			Name:        name,
			Version:     pkg.Version,
			Resolved:    pkg.Resolved,
			Integrity:   pkg.Integrity,
			Dev:         pkg.Dev || dev,
			Ecosystem:   "npm",
			LockfileRef: lockfilePath,
		}

		if pkg.Optional {
			dep.Extras = map[string]string{"optional": "true"}
		}

		*result = append(*result, dep)

		// Recursively extract nested dependencies
		if pkg.Dependencies != nil {
			s.extractV1Dependencies(pkg.Dependencies, lockfilePath, result, pkg.Dev || dev)
		}
	}
}

// extractPackageName extracts the package name from a node_modules path.
// e.g., "node_modules/@types/node" -> "@types/node"
// e.g., "node_modules/lodash" -> "lodash"
func extractPackageName(path string) string {
	const prefix = "node_modules/"
	if !strings.HasPrefix(path, prefix) {
		return ""
	}

	// Remove the prefix
	name := strings.TrimPrefix(path, prefix)

	// Handle nested node_modules (take the last package)
	if idx := strings.LastIndex(name, prefix); idx != -1 {
		name = name[idx+len(prefix):]
	}

	return name
}

// packageLockJSON represents the structure of package-lock.json.
type packageLockJSON struct {
	Name            string                      `json:"name"`
	Version         string                      `json:"version"`
	LockfileVersion int                         `json:"lockfileVersion"`
	Packages        map[string]packageLockV2Pkg `json:"packages"`     // v2/v3 format
	Dependencies    map[string]packageLockV1Dep `json:"dependencies"` // v1 format
}

// packageLockV2Pkg represents a package in v2/v3 format.
type packageLockV2Pkg struct {
	Version   string `json:"version"`
	Resolved  string `json:"resolved"`
	Integrity string `json:"integrity"`
	Dev       bool   `json:"dev"`
	Optional  bool   `json:"optional"`
	Peer      bool   `json:"peer"`
}

// packageLockV1Dep represents a dependency in v1 format.
type packageLockV1Dep struct {
	Version      string                      `json:"version"`
	Resolved     string                      `json:"resolved"`
	Integrity    string                      `json:"integrity"`
	Dev          bool                        `json:"dev"`
	Optional     bool                        `json:"optional"`
	Dependencies map[string]packageLockV1Dep `json:"dependencies"` // Nested deps
}
