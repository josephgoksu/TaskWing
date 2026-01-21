package app

import "testing"

// TestDefaultRecallOptions verifies that default recall options have expected values.
func TestDefaultRecallOptions(t *testing.T) {
	opts := DefaultRecallOptions()

	// Basic defaults
	if opts.Limit != 5 {
		t.Errorf("Limit = %d, want 5", opts.Limit)
	}
	if opts.SymbolLimit != 5 {
		t.Errorf("SymbolLimit = %d, want 5", opts.SymbolLimit)
	}
	if opts.GenerateAnswer != false {
		t.Error("GenerateAnswer should be false by default")
	}
	if opts.IncludeSymbols != true {
		t.Error("IncludeSymbols should be true by default")
	}

	// Workspace defaults
	if opts.Workspace != "" {
		t.Errorf("Workspace = %q, want empty string (all workspaces)", opts.Workspace)
	}
	if opts.IncludeRoot != true {
		t.Error("IncludeRoot should be true by default")
	}
}

// TestValidateWorkspace tests workspace validation logic.
func TestValidateWorkspace(t *testing.T) {
	tests := []struct {
		name      string
		workspace string
		wantErr   bool
	}{
		{"empty is valid", "", false},
		{"root is valid", "root", false},
		{"simple name", "osprey", false},
		{"with hyphen", "my-service", false},
		{"with underscore", "my_service", false},
		{"with numbers", "service123", false},
		{"uppercase", "MyService", false},
		{"mixed", "My-Service_123", false},
		{"invalid space", "my service", true},
		{"invalid slash", "my/service", true},
		{"invalid dot", "my.service", true},
		{"invalid colon", "my:service", true},
		{"invalid at", "my@service", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateWorkspace(tt.workspace)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateWorkspace(%q) error = %v, wantErr %v", tt.workspace, err, tt.wantErr)
			}
		})
	}
}

// TestRecallOptionsWorkspaceDefaults tests that workspace filtering defaults work correctly.
func TestRecallOptionsWorkspaceDefaults(t *testing.T) {
	// Default options: no workspace filter, include root
	opts := DefaultRecallOptions()

	// When no workspace is specified, all workspaces should be searched
	if opts.Workspace != "" {
		t.Errorf("default workspace filter should be empty, got %q", opts.Workspace)
	}

	// IncludeRoot should be true so that root/global knowledge is always visible
	if !opts.IncludeRoot {
		t.Error("IncludeRoot should default to true for workspace-aware searches")
	}

	// When workspace is set, IncludeRoot=true means we get workspace+root results
	opts.Workspace = "osprey"
	opts.IncludeRoot = true
	// This test documents the expected behavior - actual filtering is in repository layer

	// When IncludeRoot=false, we should only get workspace-specific results
	opts.IncludeRoot = false
	// This should exclude root nodes (implementation in repository layer)
}
