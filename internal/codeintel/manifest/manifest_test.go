package manifest

import (
	"os"
	"path/filepath"
	"testing"
)

// ============================================================================
// NPM Scanner Tests
// ============================================================================

func TestNpmScanner_CanScan(t *testing.T) {
	s := NewNpmScanner()

	tests := []struct {
		path string
		want bool
	}{
		{"package-lock.json", true},
		{"/foo/bar/package-lock.json", true},
		{"package.json", false},
		{"yarn.lock", false},
		{"Cargo.lock", false},
	}

	for _, tt := range tests {
		if got := s.CanScan(tt.path); got != tt.want {
			t.Errorf("CanScan(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestNpmScanner_ScanV2Format(t *testing.T) {
	tmpDir := t.TempDir()
	lockfile := filepath.Join(tmpDir, "package-lock.json")

	// package-lock.json v2/v3 format with "packages" field
	content := `{
  "name": "my-project",
  "version": "1.0.0",
  "lockfileVersion": 3,
  "packages": {
    "": {
      "name": "my-project",
      "version": "1.0.0"
    },
    "node_modules/lodash": {
      "version": "4.17.21",
      "resolved": "https://registry.npmjs.org/lodash/-/lodash-4.17.21.tgz",
      "integrity": "sha512-v2kDE..."
    },
    "node_modules/@types/node": {
      "version": "18.0.0",
      "resolved": "https://registry.npmjs.org/@types/node/-/node-18.0.0.tgz",
      "integrity": "sha512-abc...",
      "dev": true
    },
    "node_modules/optional-pkg": {
      "version": "1.0.0",
      "optional": true
    }
  }
}`
	if err := os.WriteFile(lockfile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	s := NewNpmScanner()
	result, err := s.Scan(lockfile)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	if result.Ecosystem != "npm" {
		t.Errorf("Ecosystem = %q, want %q", result.Ecosystem, "npm")
	}

	if len(result.Dependencies) != 3 {
		t.Fatalf("got %d dependencies, want 3", len(result.Dependencies))
	}

	// Check lodash
	lodash := findDep(result.Dependencies, "lodash")
	if lodash == nil {
		t.Fatal("lodash not found")
	}
	if lodash.Version != "4.17.21" {
		t.Errorf("lodash.Version = %q, want %q", lodash.Version, "4.17.21")
	}
	if lodash.Dev {
		t.Error("lodash.Dev should be false")
	}

	// Check @types/node (scoped package, dev dependency)
	typesNode := findDep(result.Dependencies, "@types/node")
	if typesNode == nil {
		t.Fatal("@types/node not found")
	}
	if !typesNode.Dev {
		t.Error("@types/node.Dev should be true")
	}

	// Check optional package
	optPkg := findDep(result.Dependencies, "optional-pkg")
	if optPkg == nil {
		t.Fatal("optional-pkg not found")
	}
	if optPkg.Extras["optional"] != "true" {
		t.Error("optional-pkg should have optional extra")
	}
}

func TestNpmScanner_ScanV1Format(t *testing.T) {
	tmpDir := t.TempDir()
	lockfile := filepath.Join(tmpDir, "package-lock.json")

	// package-lock.json v1 format with "dependencies" field
	content := `{
  "name": "my-project",
  "version": "1.0.0",
  "lockfileVersion": 1,
  "dependencies": {
    "express": {
      "version": "4.18.0",
      "resolved": "https://registry.npmjs.org/express/-/express-4.18.0.tgz",
      "integrity": "sha512-xyz...",
      "dependencies": {
        "body-parser": {
          "version": "1.20.0"
        }
      }
    },
    "jest": {
      "version": "29.0.0",
      "dev": true
    }
  }
}`
	if err := os.WriteFile(lockfile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	s := NewNpmScanner()
	result, err := s.Scan(lockfile)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	if len(result.Dependencies) != 3 {
		t.Fatalf("got %d dependencies, want 3 (express, body-parser, jest)", len(result.Dependencies))
	}

	express := findDep(result.Dependencies, "express")
	if express == nil || express.Version != "4.18.0" {
		t.Error("express not found or wrong version")
	}

	bodyParser := findDep(result.Dependencies, "body-parser")
	if bodyParser == nil || bodyParser.Version != "1.20.0" {
		t.Error("nested body-parser not found or wrong version")
	}

	jest := findDep(result.Dependencies, "jest")
	if jest == nil || !jest.Dev {
		t.Error("jest not found or should be dev dependency")
	}
}

// ============================================================================
// Python Scanner Tests
// ============================================================================

func TestPythonScanner_CanScan(t *testing.T) {
	s := NewPythonScanner()

	tests := []struct {
		path string
		want bool
	}{
		{"poetry.lock", true},
		{"requirements.txt", true},
		{"requirements-dev.txt", true},
		{"/foo/bar/requirements.txt", true},
		{"package-lock.json", false},
		{"Cargo.lock", false},
		{"requirements.yaml", false},
	}

	for _, tt := range tests {
		if got := s.CanScan(tt.path); got != tt.want {
			t.Errorf("CanScan(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestPythonScanner_ScanPoetryLock(t *testing.T) {
	tmpDir := t.TempDir()
	lockfile := filepath.Join(tmpDir, "poetry.lock")

	content := `[[package]]
name = "requests"
version = "2.28.0"
description = "HTTP library"

[[package]]
name = "urllib3"
version = "1.26.0"
source = "pypi"
optional = true

[[package]]
name = "flask"
version = "2.2.0"
`
	if err := os.WriteFile(lockfile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	s := NewPythonScanner()
	result, err := s.Scan(lockfile)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	if result.Ecosystem != "pypi" {
		t.Errorf("Ecosystem = %q, want %q", result.Ecosystem, "pypi")
	}

	if len(result.Dependencies) != 3 {
		t.Fatalf("got %d dependencies, want 3", len(result.Dependencies))
	}

	requests := findDep(result.Dependencies, "requests")
	if requests == nil || requests.Version != "2.28.0" {
		t.Error("requests not found or wrong version")
	}

	urllib3 := findDep(result.Dependencies, "urllib3")
	if urllib3 == nil {
		t.Fatal("urllib3 not found")
	}
	if urllib3.Source != "pypi" {
		t.Errorf("urllib3.Source = %q, want %q", urllib3.Source, "pypi")
	}
	if urllib3.Extras["optional"] != "true" {
		t.Error("urllib3 should be optional")
	}
}

func TestPythonScanner_ScanRequirementsTxt(t *testing.T) {
	tmpDir := t.TempDir()
	reqFile := filepath.Join(tmpDir, "requirements.txt")

	content := `# This is a comment
requests==2.28.0
flask>=2.0.0
django~=4.0
numpy
pandas[excel,sql]==1.5.0

# Another comment
-e git+https://github.com/foo/bar.git#egg=bar
-r other-requirements.txt
pytest>=7.0.0
`
	if err := os.WriteFile(reqFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	s := NewPythonScanner()
	result, err := s.Scan(reqFile)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	// Should have: requests, flask, django, numpy, pandas, pytest
	// Should NOT have: comments, -e, -r entries
	if len(result.Dependencies) != 6 {
		t.Fatalf("got %d dependencies, want 6: %v", len(result.Dependencies), result.Dependencies)
	}

	requests := findDep(result.Dependencies, "requests")
	if requests == nil || requests.Version != "2.28.0" {
		t.Error("requests not found or wrong version")
	}

	flask := findDep(result.Dependencies, "flask")
	if flask == nil || flask.Version != "2.0.0" {
		t.Error("flask not found or wrong version")
	}

	numpy := findDep(result.Dependencies, "numpy")
	if numpy == nil {
		t.Error("numpy not found")
	}
	if numpy.Version != "" {
		t.Errorf("numpy should have no version, got %q", numpy.Version)
	}

	pandas := findDep(result.Dependencies, "pandas")
	if pandas == nil {
		t.Fatal("pandas not found")
	}
	if pandas.Extras["extras"] != "excel,sql" {
		t.Errorf("pandas.Extras = %v, want excel,sql", pandas.Extras)
	}
}

// ============================================================================
// Cargo Scanner Tests
// ============================================================================

func TestCargoScanner_CanScan(t *testing.T) {
	s := NewCargoScanner()

	tests := []struct {
		path string
		want bool
	}{
		{"Cargo.lock", true},
		{"/foo/bar/Cargo.lock", true},
		{"Cargo.toml", false},
		{"package-lock.json", false},
		{"poetry.lock", false},
	}

	for _, tt := range tests {
		if got := s.CanScan(tt.path); got != tt.want {
			t.Errorf("CanScan(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestCargoScanner_Scan(t *testing.T) {
	tmpDir := t.TempDir()
	lockfile := filepath.Join(tmpDir, "Cargo.lock")

	content := `# This file is automatically @generated by Cargo.
# It is not intended for manual editing.
version = 3

[[package]]
name = "serde"
version = "1.0.152"
source = "registry+https://github.com/rust-lang/crates.io-index"
checksum = "bb7d1f0d3021d347a83e556fc4683dea2ea09d87bccdf88ff5c12545d89d5efb"

[[package]]
name = "tokio"
version = "1.24.0"
source = "registry+https://github.com/rust-lang/crates.io-index"
checksum = "abc123..."

[[package]]
name = "my-local-crate"
version = "0.1.0"
`
	if err := os.WriteFile(lockfile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	s := NewCargoScanner()
	result, err := s.Scan(lockfile)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	if result.Ecosystem != "crates.io" {
		t.Errorf("Ecosystem = %q, want %q", result.Ecosystem, "crates.io")
	}

	if len(result.Dependencies) != 3 {
		t.Fatalf("got %d dependencies, want 3", len(result.Dependencies))
	}

	serde := findDep(result.Dependencies, "serde")
	if serde == nil {
		t.Fatal("serde not found")
	}
	if serde.Version != "1.0.152" {
		t.Errorf("serde.Version = %q, want %q", serde.Version, "1.0.152")
	}
	if serde.Integrity == "" {
		t.Error("serde should have checksum as integrity")
	}

	tokio := findDep(result.Dependencies, "tokio")
	if tokio == nil || tokio.Version != "1.24.0" {
		t.Error("tokio not found or wrong version")
	}

	localCrate := findDep(result.Dependencies, "my-local-crate")
	if localCrate == nil {
		t.Fatal("my-local-crate not found")
	}
	if localCrate.Source != "" {
		t.Error("local crate should not have source")
	}
}

// ============================================================================
// ScanDirectory Tests
// ============================================================================

func TestScanDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a package-lock.json
	npmLock := filepath.Join(tmpDir, "package-lock.json")
	npmContent := `{
  "lockfileVersion": 3,
  "packages": {
    "node_modules/lodash": {"version": "4.17.21"}
  }
}`
	if err := os.WriteFile(npmLock, []byte(npmContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a requirements.txt
	reqFile := filepath.Join(tmpDir, "requirements.txt")
	if err := os.WriteFile(reqFile, []byte("requests==2.28.0\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a Cargo.lock
	cargoLock := filepath.Join(tmpDir, "Cargo.lock")
	cargoContent := `[[package]]
name = "serde"
version = "1.0.0"
`
	if err := os.WriteFile(cargoLock, []byte(cargoContent), 0644); err != nil {
		t.Fatal(err)
	}

	results, err := ScanDirectory(tmpDir, AllScanners())
	if err != nil {
		t.Fatalf("ScanDirectory failed: %v", err)
	}

	if len(results) != 3 {
		t.Fatalf("got %d results, want 3", len(results))
	}

	// Check we have all ecosystems
	ecosystems := make(map[string]bool)
	for _, r := range results {
		ecosystems[r.Ecosystem] = true
	}

	if !ecosystems["npm"] {
		t.Error("missing npm ecosystem")
	}
	if !ecosystems["pypi"] {
		t.Error("missing pypi ecosystem")
	}
	if !ecosystems["crates.io"] {
		t.Error("missing crates.io ecosystem")
	}
}

// ============================================================================
// Helper Functions
// ============================================================================

func findDep(deps []Dependency, name string) *Dependency {
	for i := range deps {
		if deps[i].Name == name {
			return &deps[i]
		}
	}
	return nil
}
