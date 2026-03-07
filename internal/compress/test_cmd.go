package compress

import (
	"bytes"
	"fmt"
	"strings"
)

// testPipeline returns a compression pipeline tuned for test runner output.
func testPipeline(_ string) *Pipeline {
	return NewPipeline(
		StripANSI,
		StripProgress,
		CollapseWhitespace,
		ExtractFailuresOnly,
		SmartSummary,
		TruncatePaths,
		LimitLineCount(150),
	)
}

// ExtractFailuresOnly keeps failure details and collapses passing tests.
func ExtractFailuresOnly(data []byte) []byte {
	lines := bytes.Split(data, []byte("\n"))
	if len(lines) < 5 {
		return data
	}

	var result [][]byte
	passCount := 0
	inFailure := false

	for _, line := range lines {
		s := string(line)

		// Detect failure markers across test frameworks
		isFailure := strings.Contains(s, "FAIL") ||
			strings.Contains(s, "FAILED") ||
			strings.Contains(s, "Error:") ||
			strings.Contains(s, "panic:") ||
			strings.Contains(s, "ERRORS") ||
			strings.HasPrefix(s, "E ") // pytest style

		isPass := strings.HasPrefix(s, "ok ") ||
			strings.Contains(s, "PASS") ||
			strings.Contains(s, "passed") ||
			strings.HasPrefix(s, "  ✓") ||
			strings.HasPrefix(s, "  √")

		isSummary := strings.HasPrefix(s, "---") ||
			strings.HasPrefix(s, "===") ||
			strings.Contains(s, "test result:") ||
			strings.Contains(s, "Tests:") ||
			strings.Contains(s, "Test Suites:")

		if isFailure {
			// Flush pass count before failure block
			if passCount > 0 {
				result = append(result, []byte(fmt.Sprintf("  ... (%d passing tests)", passCount)))
				passCount = 0
			}
			inFailure = true
			result = append(result, line)
		} else if inFailure {
			// Keep lines following a failure until a blank line or pass
			if len(bytes.TrimSpace(line)) == 0 {
				inFailure = false
				result = append(result, line)
			} else if isPass {
				inFailure = false
				passCount++
			} else {
				result = append(result, line)
			}
		} else if isPass {
			passCount++
		} else if isSummary {
			if passCount > 0 {
				result = append(result, []byte(fmt.Sprintf("  ... (%d passing tests)", passCount)))
				passCount = 0
			}
			result = append(result, line)
		} else {
			// Non-test output (compilation errors, warnings, etc.) — keep
			result = append(result, line)
		}
	}

	if passCount > 0 {
		result = append(result, []byte(fmt.Sprintf("  ... (%d passing tests)", passCount)))
	}

	return bytes.Join(result, []byte("\n"))
}
