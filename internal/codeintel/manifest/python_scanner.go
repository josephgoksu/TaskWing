package manifest

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// PythonScanner parses Python dependency files (poetry.lock, requirements.txt).
type PythonScanner struct{}

// NewPythonScanner creates a new Python dependency scanner.
func NewPythonScanner() *PythonScanner {
	return &PythonScanner{}
}

// Name returns the scanner name.
func (s *PythonScanner) Name() string {
	return "python"
}

// SupportedFiles returns the glob patterns for Python dependency files.
func (s *PythonScanner) SupportedFiles() []string {
	return []string{"poetry.lock", "requirements.txt", "requirements-*.txt", "requirements/*.txt"}
}

// CanScan checks if the file is a supported Python dependency file.
func (s *PythonScanner) CanScan(path string) bool {
	base := filepath.Base(path)
	return base == "poetry.lock" ||
		base == "requirements.txt" ||
		strings.HasPrefix(base, "requirements-") && strings.HasSuffix(base, ".txt")
}

// Scan parses a Python dependency file and extracts dependencies.
func (s *PythonScanner) Scan(path string) (*ScanResult, error) {
	base := filepath.Base(path)

	if base == "poetry.lock" {
		return s.scanPoetryLock(path)
	}

	return s.scanRequirementsTxt(path)
}

// scanPoetryLock parses a poetry.lock file (TOML format).
func (s *PythonScanner) scanPoetryLock(path string) (*ScanResult, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	result := &ScanResult{
		Lockfile:  path,
		Ecosystem: "pypi",
	}

	// poetry.lock is TOML format with [[package]] sections
	// We parse it manually to avoid TOML dependency
	packages := parsePoetryPackages(string(content))
	for _, pkg := range packages {
		dep := Dependency{
			Name:        pkg.name,
			Version:     pkg.version,
			Source:      pkg.source,
			Ecosystem:   "pypi",
			LockfileRef: path,
		}

		if pkg.optional {
			dep.Extras = map[string]string{"optional": "true"}
		}

		result.Dependencies = append(result.Dependencies, dep)
	}

	return result, nil
}

// poetryPackage holds parsed poetry package data.
type poetryPackage struct {
	name     string
	version  string
	source   string
	optional bool
}

// parsePoetryPackages extracts packages from poetry.lock content.
func parsePoetryPackages(content string) []poetryPackage {
	var packages []poetryPackage

	namePattern := regexp.MustCompile(`(?m)^name\s*=\s*"([^"]+)"`)
	versionPattern := regexp.MustCompile(`(?m)^version\s*=\s*"([^"]+)"`)
	sourcePattern := regexp.MustCompile(`(?m)^source\s*=\s*"([^"]+)"`)
	optionalPattern := regexp.MustCompile(`(?m)^optional\s*=\s*true`)

	// Split content by [[package]] markers
	parts := strings.Split(content, "[[package]]")

	for _, section := range parts {
		// Skip parts before first [[package]] or empty sections
		section = strings.TrimSpace(section)
		if section == "" {
			continue
		}

		// Stop at other section types like [[metadata]]
		if idx := strings.Index(section, "[["); idx != -1 {
			section = section[:idx]
		}

		pkg := poetryPackage{}

		if m := namePattern.FindStringSubmatch(section); m != nil {
			pkg.name = m[1]
		}
		if m := versionPattern.FindStringSubmatch(section); m != nil {
			pkg.version = m[1]
		}
		if m := sourcePattern.FindStringSubmatch(section); m != nil {
			pkg.source = m[1]
		}
		if optionalPattern.MatchString(section) {
			pkg.optional = true
		}

		if pkg.name != "" && pkg.version != "" {
			packages = append(packages, pkg)
		}
	}

	return packages
}

// scanRequirementsTxt parses a requirements.txt file.
func (s *PythonScanner) scanRequirementsTxt(path string) (*ScanResult, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	result := &ScanResult{
		Lockfile:  path,
		Ecosystem: "pypi",
	}

	// Patterns for requirements.txt entries
	// Format: package==version, package>=version, package~=version, etc.
	pkgPattern := regexp.MustCompile(`^([a-zA-Z0-9][-a-zA-Z0-9._]*)\s*(==|>=|<=|~=|!=|>|<)?\s*([^\s;#\[]*)`)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Skip -r includes, -e editable installs, and other flags
		if strings.HasPrefix(line, "-") {
			continue
		}

		// Parse the line
		if match := pkgPattern.FindStringSubmatch(line); match != nil {
			name := match[1]
			version := match[3]

			// Normalize package name (PEP 503)
			name = strings.ToLower(strings.ReplaceAll(name, "_", "-"))

			dep := Dependency{
				Name:        name,
				Version:     version,
				Ecosystem:   "pypi",
				LockfileRef: path,
			}

			// Check for extras like [extra1,extra2]
			if idx := strings.Index(line, "["); idx != -1 {
				if endIdx := strings.Index(line[idx:], "]"); endIdx != -1 {
					extras := line[idx+1 : idx+endIdx]
					dep.Extras = map[string]string{"extras": extras}
				}
			}

			result.Dependencies = append(result.Dependencies, dep)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return result, nil
}
