package manifest

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// CargoScanner parses Rust Cargo.lock files.
type CargoScanner struct{}

// NewCargoScanner creates a new Cargo.lock scanner.
func NewCargoScanner() *CargoScanner {
	return &CargoScanner{}
}

// Name returns the scanner name.
func (s *CargoScanner) Name() string {
	return "cargo"
}

// SupportedFiles returns the glob patterns for Cargo lockfiles.
func (s *CargoScanner) SupportedFiles() []string {
	return []string{"Cargo.lock"}
}

// CanScan checks if the file is a Cargo.lock.
func (s *CargoScanner) CanScan(path string) bool {
	return filepath.Base(path) == "Cargo.lock"
}

// Scan parses a Cargo.lock and extracts dependencies.
func (s *CargoScanner) Scan(path string) (*ScanResult, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	result := &ScanResult{
		Lockfile:  path,
		Ecosystem: "crates.io",
	}

	// Cargo.lock is TOML format with [[package]] sections
	packages := parseCargoPackages(string(content))
	for _, pkg := range packages {
		dep := Dependency{
			Name:        pkg.name,
			Version:     pkg.version,
			Source:      pkg.source,
			Integrity:   pkg.checksum,
			Ecosystem:   "crates.io",
			LockfileRef: path,
		}

		result.Dependencies = append(result.Dependencies, dep)
	}

	return result, nil
}

// cargoPackage holds parsed Cargo package data.
type cargoPackage struct {
	name     string
	version  string
	source   string
	checksum string
}

// parseCargoPackages extracts packages from Cargo.lock content.
func parseCargoPackages(content string) []cargoPackage {
	var packages []cargoPackage

	namePattern := regexp.MustCompile(`(?m)^name\s*=\s*"([^"]+)"`)
	versionPattern := regexp.MustCompile(`(?m)^version\s*=\s*"([^"]+)"`)
	sourcePattern := regexp.MustCompile(`(?m)^source\s*=\s*"([^"]+)"`)
	checksumPattern := regexp.MustCompile(`(?m)^checksum\s*=\s*"([^"]+)"`)

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

		pkg := cargoPackage{}

		if m := namePattern.FindStringSubmatch(section); m != nil {
			pkg.name = m[1]
		}
		if m := versionPattern.FindStringSubmatch(section); m != nil {
			pkg.version = m[1]
		}
		if m := sourcePattern.FindStringSubmatch(section); m != nil {
			pkg.source = m[1]
		}
		if m := checksumPattern.FindStringSubmatch(section); m != nil {
			pkg.checksum = m[1]
		}

		if pkg.name != "" && pkg.version != "" {
			packages = append(packages, pkg)
		}
	}

	return packages
}
