package policy

import (
	"testing"

	"github.com/spf13/afero"
)

func TestLoader_LoadAll(t *testing.T) {
	fs := afero.NewMemMapFs()

	// Create policies directory structure
	_ = fs.MkdirAll("/project/.taskwing/policies", 0755)

	// Create some .rego files
	protectedZonesRego := `package taskwing.policy

import rego.v1

deny contains msg if {
    some file in input.task.files_modified
    startswith(file, "core/")
    msg := "Cannot modify core files"
}
`
	secretsRego := `package taskwing.policy.secrets

import rego.v1

deny contains msg if {
    some file in input.task.files_modified
    endswith(file, ".env")
    msg := "Cannot modify .env files"
}
`

	_ = afero.WriteFile(fs, "/project/.taskwing/policies/protected_zones.rego", []byte(protectedZonesRego), 0644)
	_ = afero.WriteFile(fs, "/project/.taskwing/policies/secrets.rego", []byte(secretsRego), 0644)
	// Add a non-rego file that should be ignored
	_ = afero.WriteFile(fs, "/project/.taskwing/policies/README.md", []byte("# Policies"), 0644)

	loader := NewLoader(fs, "/project/.taskwing/policies")

	policies, err := loader.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll() error = %v", err)
	}

	if len(policies) != 2 {
		t.Errorf("LoadAll() returned %d policies, want 2", len(policies))
	}

	// Verify policy names
	names := make(map[string]bool)
	for _, p := range policies {
		names[p.Name] = true
		if p.Content == "" {
			t.Errorf("Policy %s has empty content", p.Name)
		}
	}

	if !names["protected_zones"] {
		t.Error("Expected protected_zones policy to be loaded")
	}
	if !names["secrets"] {
		t.Error("Expected secrets policy to be loaded")
	}
}

func TestLoader_LoadAll_Subdirectories(t *testing.T) {
	fs := afero.NewMemMapFs()

	// Create nested directory structure
	_ = fs.MkdirAll("/project/.taskwing/policies/security", 0755)
	_ = fs.MkdirAll("/project/.taskwing/policies/architecture", 0755)

	_ = afero.WriteFile(fs, "/project/.taskwing/policies/defaults.rego", []byte("package defaults"), 0644)
	_ = afero.WriteFile(fs, "/project/.taskwing/policies/security/hardcoded_secrets.rego", []byte("package security"), 0644)
	_ = afero.WriteFile(fs, "/project/.taskwing/policies/architecture/layers.rego", []byte("package architecture"), 0644)

	loader := NewLoader(fs, "/project/.taskwing/policies")

	policies, err := loader.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll() error = %v", err)
	}

	if len(policies) != 3 {
		t.Errorf("LoadAll() returned %d policies, want 3", len(policies))
	}
}

func TestLoader_LoadAll_EmptyDirectory(t *testing.T) {
	fs := afero.NewMemMapFs()

	// Create empty policies directory
	_ = fs.MkdirAll("/project/.taskwing/policies", 0755)

	loader := NewLoader(fs, "/project/.taskwing/policies")

	policies, err := loader.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll() error = %v", err)
	}

	if len(policies) != 0 {
		t.Errorf("LoadAll() returned %d policies, want 0", len(policies))
	}
}

func TestLoader_LoadAll_NonExistentDirectory(t *testing.T) {
	fs := afero.NewMemMapFs()

	// Don't create the directory - it doesn't exist
	loader := NewLoader(fs, "/project/.taskwing/policies")

	policies, err := loader.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll() error = %v (should return empty slice)", err)
	}

	if len(policies) != 0 {
		t.Errorf("LoadAll() returned %d policies for non-existent dir, want 0", len(policies))
	}
}

func TestLoader_LoadFile(t *testing.T) {
	fs := afero.NewMemMapFs()

	content := `package test.policy

deny contains msg if {
    msg := "test violation"
}
`
	_ = fs.MkdirAll("/project/.taskwing/policies", 0755)
	_ = afero.WriteFile(fs, "/project/.taskwing/policies/test.rego", []byte(content), 0644)

	loader := NewLoader(fs, "/project/.taskwing/policies")

	policy, err := loader.LoadFile("/project/.taskwing/policies/test.rego")
	if err != nil {
		t.Fatalf("LoadFile() error = %v", err)
	}

	if policy.Name != "test" {
		t.Errorf("Name = %v, want test", policy.Name)
	}

	if policy.Content != content {
		t.Errorf("Content mismatch")
	}

	if policy.Path != "/project/.taskwing/policies/test.rego" {
		t.Errorf("Path = %v, want /project/.taskwing/policies/test.rego", policy.Path)
	}
}

func TestLoader_LoadFile_NotFound(t *testing.T) {
	fs := afero.NewMemMapFs()

	loader := NewLoader(fs, "/project/.taskwing/policies")

	_, err := loader.LoadFile("/project/.taskwing/policies/nonexistent.rego")
	if err == nil {
		t.Error("LoadFile() expected error for non-existent file")
	}
}

func TestLoader_Exists(t *testing.T) {
	fs := afero.NewMemMapFs()

	loader := NewLoader(fs, "/project/.taskwing/policies")

	// Directory doesn't exist
	exists, err := loader.Exists()
	if err != nil {
		t.Fatalf("Exists() error = %v", err)
	}
	if exists {
		t.Error("Exists() = true for non-existent directory")
	}

	// Create directory
	_ = fs.MkdirAll("/project/.taskwing/policies", 0755)

	exists, err = loader.Exists()
	if err != nil {
		t.Fatalf("Exists() error = %v", err)
	}
	if !exists {
		t.Error("Exists() = false for existing directory")
	}
}

func TestLoader_ListFiles(t *testing.T) {
	fs := afero.NewMemMapFs()

	_ = fs.MkdirAll("/project/.taskwing/policies", 0755)
	_ = afero.WriteFile(fs, "/project/.taskwing/policies/a.rego", []byte("package a"), 0644)
	_ = afero.WriteFile(fs, "/project/.taskwing/policies/b.rego", []byte("package b"), 0644)
	_ = afero.WriteFile(fs, "/project/.taskwing/policies/readme.md", []byte("# README"), 0644)

	loader := NewLoader(fs, "/project/.taskwing/policies")

	paths, err := loader.ListFiles()
	if err != nil {
		t.Fatalf("ListFiles() error = %v", err)
	}

	if len(paths) != 2 {
		t.Errorf("ListFiles() returned %d paths, want 2", len(paths))
	}
}

func TestLoader_ListFiles_NonExistentDirectory(t *testing.T) {
	fs := afero.NewMemMapFs()

	loader := NewLoader(fs, "/project/.taskwing/policies")

	paths, err := loader.ListFiles()
	if err != nil {
		t.Fatalf("ListFiles() error = %v (should return empty slice)", err)
	}

	if len(paths) != 0 {
		t.Errorf("ListFiles() returned %d paths for non-existent dir, want 0", len(paths))
	}
}

func TestGetPoliciesPath(t *testing.T) {
	tests := []struct {
		projectRoot string
		want        string
	}{
		{
			projectRoot: "/home/user/project",
			want:        "/home/user/project/.taskwing/policies",
		},
		{
			projectRoot: "/project",
			want:        "/project/.taskwing/policies",
		},
	}

	for _, tt := range tests {
		t.Run(tt.projectRoot, func(t *testing.T) {
			got := GetPoliciesPath(tt.projectRoot)
			if got != tt.want {
				t.Errorf("GetPoliciesPath() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewOsLoader(t *testing.T) {
	// Just verify it can be created without panicking
	loader := NewOsLoader("/tmp/test-policies")
	if loader == nil {
		t.Error("NewOsLoader() returned nil")
	}
	if loader.baseDir != "/tmp/test-policies" {
		t.Errorf("baseDir = %v, want /tmp/test-policies", loader.baseDir)
	}
}
