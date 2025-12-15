package cmd

import (
	"testing"
)

func TestAddCommand_Structure(t *testing.T) {
	// Test that command exists and has expected properties
	if addCmd == nil {
		t.Fatal("addCmd should not be nil")
	}

	if addCmd.Use != "add [task description]" {
		t.Errorf("Use mismatch: got %q, want %q", addCmd.Use, "add [task description]")
	}

	expectedAliases := []string{"mk", "create", "new", "t"}
	if len(addCmd.Aliases) != len(expectedAliases) {
		t.Errorf("Aliases count mismatch: got %d, want %d", len(addCmd.Aliases), len(expectedAliases))
	}

	for i, alias := range expectedAliases {
		if i >= len(addCmd.Aliases) || addCmd.Aliases[i] != alias {
			t.Errorf("Alias %d mismatch: got %q, want %q", i, addCmd.Aliases[i], alias)
		}
	}
}

func TestAddCommand_Flags(t *testing.T) {
	// Test that expected flags exist
	expectedFlags := []string{
		"no-ai",
		"breakdown",
		"detect-deps",
		"title",
		"dependencies",
		"parentID",
		"non-interactive",
		"start",
		"priority",
		"description",
		"acceptance",
	}

	flags := addCmd.Flags()

	for _, flagName := range expectedFlags {
		flag := flags.Lookup(flagName)
		if flag == nil {
			t.Errorf("expected flag %q to exist", flagName)
		}
	}

	// Test short flags exist
	expectedShortFlags := map[string]string{
		"s": "start",
		"P": "priority",
		"d": "description",
		"a": "acceptance",
	}

	for shortFlag, longFlag := range expectedShortFlags {
		flag := flags.ShorthandLookup(shortFlag)
		if flag == nil {
			t.Errorf("expected short flag -%s for --%s to exist", shortFlag, longFlag)
		}
	}
}
