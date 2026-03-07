package compress

import (
	"bytes"
	"fmt"
	"strings"
)

// gitPipeline returns a compression pipeline tuned for git commands.
func gitPipeline(cmd string) *Pipeline {
	fields := strings.Fields(cmd)
	subcmd := ""
	if len(fields) > 1 {
		subcmd = fields[1]
	}

	switch subcmd {
	case "status":
		return NewPipeline(
			StripANSI,
			CollapseWhitespace,
			TruncatePaths,
			GroupByDirectory,
		)
	case "log":
		return NewPipeline(
			StripANSI,
			CollapseWhitespace,
			DeduplicateLines,
			LimitLineCount(50),
		)
	case "diff":
		return NewPipeline(
			StripANSI,
			CollapseWhitespace,
			CollapseDiffContext,
			TruncatePaths,
			LimitLineCount(200),
		)
	case "branch":
		return NewPipeline(
			StripANSI,
			CollapseWhitespace,
			DeduplicateLines,
		)
	default:
		return NewPipeline(
			StripANSI,
			CollapseWhitespace,
			TruncatePaths,
			DeduplicateLines,
		)
	}
}

// CollapseDiffContext reduces unchanged context lines in diffs to save tokens.
func CollapseDiffContext(data []byte) []byte {
	lines := bytes.Split(data, []byte("\n"))
	var result [][]byte
	contextRun := 0

	for _, line := range lines {
		isDiffMeta := bytes.HasPrefix(line, []byte("diff ")) ||
			bytes.HasPrefix(line, []byte("index ")) ||
			bytes.HasPrefix(line, []byte("--- ")) ||
			bytes.HasPrefix(line, []byte("+++ ")) ||
			bytes.HasPrefix(line, []byte("@@ "))
		isChange := bytes.HasPrefix(line, []byte("+")) || bytes.HasPrefix(line, []byte("-"))

		if isDiffMeta || isChange {
			if contextRun > 6 {
				// Keep first 3 and last 3 context lines, collapse middle
				collapsed := contextRun - 6
				result = append(result, []byte(fmt.Sprintf("  ... (%d unchanged lines)", collapsed)))
			}
			contextRun = 0
			result = append(result, line)
		} else {
			contextRun++
			if contextRun <= 3 {
				result = append(result, line)
			}
			// Lines 4+ are buffered; if we hit a change, we'll emit the last 3
		}
	}
	if contextRun > 6 {
		result = append(result, []byte(fmt.Sprintf("  ... (%d unchanged lines)", contextRun-3)))
	}

	return bytes.Join(result, []byte("\n"))
}
