package compress

import "strings"

// ForCommand returns the appropriate compression pipeline for a given command.
func ForCommand(command string) *Pipeline {
	cmd := strings.TrimSpace(command)
	base := baseCommand(cmd)

	switch {
	// Git commands
	case base == "git":
		return gitPipeline(cmd)
	// Test runners
	case isTestCommand(base, cmd):
		return testPipeline(cmd)
	// Generic commands
	default:
		return genericPipeline(cmd)
	}
}

// baseCommand extracts the first word of a command string.
func baseCommand(cmd string) string {
	fields := strings.Fields(cmd)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

// isTestCommand checks if a command is a test runner.
func isTestCommand(base, cmd string) bool {
	testBases := map[string]bool{
		"go":     strings.Contains(cmd, " test"),
		"cargo":  strings.Contains(cmd, " test"),
		"npm":    strings.Contains(cmd, " test"),
		"npx":    strings.Contains(cmd, "jest") || strings.Contains(cmd, "vitest"),
		"pytest": true,
		"python": strings.Contains(cmd, "-m pytest") || strings.Contains(cmd, "-m unittest"),
		"jest":   true,
		"vitest": true,
		"make":   strings.Contains(cmd, "test"),
	}
	if match, ok := testBases[base]; ok {
		return match
	}
	return false
}

// DefaultPipeline returns a minimal compression pipeline.
func DefaultPipeline() *Pipeline {
	return NewPipeline(
		StripANSI,
		CollapseWhitespace,
		TruncatePaths,
		DeduplicateLines,
	)
}
