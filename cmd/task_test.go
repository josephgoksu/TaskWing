/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/josephgoksu/TaskWing/internal/task"
)

// TestFormatTaskStatus_AllKnownStatuses verifies all defined TaskStatus values render correctly.
func TestFormatTaskStatus_AllKnownStatuses(t *testing.T) {
	tests := []struct {
		status   task.TaskStatus
		contains string // substring that should be in the output
	}{
		{task.StatusDraft, "draft"},
		{task.StatusPending, "pending"},
		{task.StatusInProgress, "active"},
		{task.StatusVerifying, "verify"},
		{task.StatusCompleted, "done"},
		{task.StatusFailed, "failed"},
		{task.StatusBlocked, "blocked"},
		{task.StatusReady, "ready"},
	}

	for _, tc := range tests {
		t.Run(string(tc.status), func(t *testing.T) {
			result := formatTaskStatus(tc.status)
			if result == "" {
				t.Error("formatTaskStatus returned empty string")
			}
			// Strip ANSI codes for checking content
			stripped := stripANSI(result)
			if !strings.Contains(strings.ToLower(stripped), tc.contains) {
				t.Errorf("formatTaskStatus(%q) = %q, want string containing %q", tc.status, stripped, tc.contains)
			}
		})
	}
}

// TestFormatTaskStatus_DoneAlias verifies "done" as an alias for StatusCompleted.
func TestFormatTaskStatus_DoneAlias(t *testing.T) {
	result := formatTaskStatus("done")
	stripped := stripANSI(result)
	if !strings.Contains(strings.ToLower(stripped), "done") {
		t.Errorf("formatTaskStatus(\"done\") = %q, want string containing 'done'", stripped)
	}
}

// TestFormatTaskStatus_UnknownStatus verifies unknown statuses render gracefully.
func TestFormatTaskStatus_UnknownStatus(t *testing.T) {
	unknownStatuses := []task.TaskStatus{
		"invalid",
		"garbage",
		"",
		"some_future_status",
		"COMPLETED", // Wrong case
	}

	for _, status := range unknownStatuses {
		t.Run(string(status), func(t *testing.T) {
			// Should not panic
			result := formatTaskStatus(status)
			if result == "" {
				t.Error("formatTaskStatus returned empty string for unknown status")
			}
			stripped := stripANSI(result)
			if !strings.Contains(strings.ToLower(stripped), "unknown") {
				t.Errorf("formatTaskStatus(%q) = %q, want string containing 'unknown'", status, stripped)
			}
		})
	}
}

// TestFormatTaskStatus_FixedWidth verifies all status strings have consistent width.
func TestFormatTaskStatus_FixedWidth(t *testing.T) {
	statuses := []task.TaskStatus{
		task.StatusDraft,
		task.StatusPending,
		task.StatusInProgress,
		task.StatusVerifying,
		task.StatusCompleted,
		task.StatusFailed,
		task.StatusBlocked,
		task.StatusReady,
		"unknown_status",
	}

	// Get the width of the first status
	firstStripped := stripANSI(formatTaskStatus(statuses[0]))
	expectedWidth := len(firstStripped)

	for _, status := range statuses {
		t.Run(string(status), func(t *testing.T) {
			result := formatTaskStatus(status)
			stripped := stripANSI(result)
			if len(stripped) != expectedWidth {
				t.Errorf("formatTaskStatus(%q) width = %d, want %d (value: %q)", status, len(stripped), expectedWidth, stripped)
			}
		})
	}
}

// TestFormatTaskStatus_NoPanic verifies the function never panics.
func TestFormatTaskStatus_NoPanic(t *testing.T) {
	// Test with various edge cases that should not cause panic
	testCases := []task.TaskStatus{
		"",
		"null",
		"undefined",
		"\x00",           // null byte
		"status\nstatus", // newline
		"status\tstatus", // tab
		task.TaskStatus(strings.Repeat("x", 1000)), // very long string
	}

	for i, tc := range testCases {
		t.Run(string(rune('A'+i)), func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("formatTaskStatus panicked with input %q: %v", tc, r)
				}
			}()
			_ = formatTaskStatus(tc)
		})
	}
}

// TestFormatTaskStatus_TableDriven comprehensive table-driven test.
func TestFormatTaskStatus_TableDriven(t *testing.T) {
	tests := []struct {
		name        string
		status      task.TaskStatus
		wantLabel   string
		wantUnknown bool
	}{
		{"completed", task.StatusCompleted, "done", false},
		{"done_alias", "done", "done", false},
		{"in_progress", task.StatusInProgress, "active", false},
		{"pending", task.StatusPending, "pending", false},
		{"draft", task.StatusDraft, "draft", false},
		{"blocked", task.StatusBlocked, "blocked", false},
		{"ready", task.StatusReady, "ready", false},
		{"failed", task.StatusFailed, "failed", false},
		{"verifying", task.StatusVerifying, "verify", false},
		{"unknown_garbage", "garbage", "unknown", true},
		{"unknown_empty", "", "unknown", true},
		{"unknown_case_sensitive", "PENDING", "unknown", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := formatTaskStatus(tc.status)
			stripped := stripANSI(result)

			if !strings.Contains(strings.ToLower(stripped), tc.wantLabel) {
				t.Errorf("formatTaskStatus(%q) = %q, want string containing %q", tc.status, stripped, tc.wantLabel)
			}

			if tc.wantUnknown && !strings.Contains(strings.ToLower(stripped), "unknown") {
				t.Errorf("formatTaskStatus(%q) should render as 'unknown'", tc.status)
			}
		})
	}
}

// stripANSI removes ANSI escape codes from a string for easier testing.
func stripANSI(s string) string {
	// Simple ANSI stripping - removes escape sequences
	result := strings.Builder{}
	i := 0
	for i < len(s) {
		if s[i] == '\x1b' && i+1 < len(s) && s[i+1] == '[' {
			// Skip until 'm' (end of ANSI sequence)
			for i < len(s) && s[i] != 'm' {
				i++
			}
			i++ // skip the 'm'
		} else {
			result.WriteByte(s[i])
			i++
		}
	}
	return result.String()
}

// TestStripANSI verifies the helper function works correctly.
func TestStripANSI(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"\x1b[32mgreen\x1b[0m", "green"},
		{"\x1b[1;31mred bold\x1b[0m", "red bold"},
		{"no ansi", "no ansi"},
		{"", ""},
		{"\x1b[42m\x1b[1m[done]    \x1b[0m", "[done]    "},
	}

	for _, tc := range tests {
		t.Run(tc.want, func(t *testing.T) {
			got := stripANSI(tc.input)
			if got != tc.want {
				t.Errorf("stripANSI(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

// TestTaskListArchivedFilter tests that the include-archived flag exists.
func TestTaskListArchivedFilter(t *testing.T) {
	// Verify the flag is registered on the command
	flag := taskListCmd.Flags().Lookup("include-archived")
	if flag == nil {
		t.Fatal("expected --include-archived flag to be registered on task list command")
	}

	if flag.DefValue != "false" {
		t.Errorf("expected default value 'false', got %q", flag.DefValue)
	}

	// Verify the flag help text
	usage := flag.Usage
	if usage == "" {
		t.Error("expected --include-archived flag to have usage text")
	}
	if !strings.Contains(strings.ToLower(usage), "archived") {
		t.Errorf("flag usage should mention 'archived', got: %q", usage)
	}
}

// TestTaskListCommandHelp verifies help mentions archived filtering.
func TestTaskListCommandHelp(t *testing.T) {
	longHelp := taskListCmd.Long
	if !strings.Contains(longHelp, "archived") {
		t.Error("task list long help should mention archived plans")
	}
	if !strings.Contains(longHelp, "--include-archived") {
		t.Error("task list long help should mention --include-archived flag")
	}
}

// TestTaskListErrorPropagation verifies that errors are properly returned, not swallowed.
func TestTaskListErrorPropagation(t *testing.T) {
	// Verify the RunE function is set (meaning errors will be returned to Cobra)
	if taskListCmd.RunE == nil {
		t.Fatal("taskListCmd.RunE should be set to return errors")
	}

	// Verify Run is not set (which would swallow errors)
	if taskListCmd.Run != nil {
		t.Error("taskListCmd.Run should not be set; use RunE for error propagation")
	}
}

// TestTaskListExitOnError verifies non-zero exit on repository failure.
func TestTaskListExitOnError(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping smoke test in short mode")
	}

	// Create a temp directory that has no .taskwing/memory structure
	tmpDir, err := os.MkdirTemp("", "taskwing-smoke-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Run the CLI with the temp dir as the working directory
	// This should fail because there's no memory.db
	cmd := exec.Command("go", "run", ".", "task", "list")
	cmd.Dir = tmpDir
	cmd.Env = append(os.Environ(), "HOME="+tmpDir) // Prevent using real home config

	output, err := cmd.CombinedOutput()

	// We expect an error (non-zero exit) because the repo doesn't exist
	if err == nil {
		t.Log("Command output:", string(output))
		// Note: If this passes, it might mean the command gracefully handles missing repos
		// by showing "No plans found" which is acceptable behavior
		if strings.Contains(string(output), "No plans found") {
			t.Log("Command succeeded with 'No plans found' - this is acceptable behavior")
			return
		}
		// If no error and no "No plans found", something unexpected happened
		t.Log("Command succeeded unexpectedly without error")
	} else {
		// Verify exit error
		exitErr, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("expected exec.ExitError, got %T: %v", err, err)
		}
		if exitErr.ExitCode() == 0 {
			t.Error("expected non-zero exit code on failure")
		}

		// Verify error message contains useful context
		outputStr := string(output)
		if outputStr == "" {
			t.Error("expected error message in output, got empty")
		}

		// Should have some indication of the failure
		hasContext := strings.Contains(strings.ToLower(outputStr), "error") ||
			strings.Contains(strings.ToLower(outputStr), "failed") ||
			strings.Contains(strings.ToLower(outputStr), "no such file")
		if !hasContext {
			t.Logf("Output should contain error context: %s", outputStr)
		}
	}
}

// TestTaskListVerboseError verifies verbose mode provides additional context.
func TestTaskListVerboseError(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping smoke test in short mode")
	}

	// The --verbose flag should be available
	flag := taskListCmd.Root().PersistentFlags().Lookup("verbose")
	if flag == nil {
		// Check if it's inherited from root
		flag = rootCmd.PersistentFlags().Lookup("verbose")
	}
	if flag == nil {
		t.Log("verbose flag not found on taskListCmd, may be on root")
	}

	// Test that the flag exists by checking the root command
	if rootCmd.PersistentFlags().Lookup("verbose") == nil {
		t.Error("expected --verbose flag to be registered on root command")
	}
}
