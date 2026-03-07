package compress

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
)

var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

// StripANSI removes ANSI escape codes from output.
func StripANSI(data []byte) []byte {
	return ansiRegex.ReplaceAll(data, nil)
}

// CollapseWhitespace normalizes excessive blank lines to at most one.
func CollapseWhitespace(data []byte) []byte {
	lines := bytes.Split(data, []byte("\n"))
	var result [][]byte
	blankCount := 0
	for _, line := range lines {
		trimmed := bytes.TrimSpace(line)
		if len(trimmed) == 0 {
			blankCount++
			if blankCount <= 1 {
				result = append(result, nil)
			}
		} else {
			blankCount = 0
			result = append(result, line)
		}
	}
	return bytes.Join(result, []byte("\n"))
}

// TruncatePaths converts absolute paths to relative paths based on common prefixes.
func TruncatePaths(data []byte) []byte {
	lines := bytes.Split(data, []byte("\n"))
	// Find common path prefix
	var prefix string
	for _, line := range lines {
		s := string(line)
		if idx := strings.Index(s, "/Users/"); idx >= 0 {
			end := strings.IndexByte(s[idx:], ' ')
			if end < 0 {
				end = len(s) - idx
			}
			path := s[idx : idx+end]
			parts := strings.Split(path, "/")
			if len(parts) > 4 {
				candidate := strings.Join(parts[:4], "/")
				if prefix == "" {
					prefix = candidate
				}
			}
		}
	}
	if prefix == "" {
		return data
	}
	return bytes.ReplaceAll(data, []byte(prefix+"/"), []byte("./"))
}

// DeduplicateLines collapses consecutive identical lines with a count.
func DeduplicateLines(data []byte) []byte {
	lines := bytes.Split(data, []byte("\n"))
	if len(lines) <= 1 {
		return data
	}

	var result [][]byte
	current := lines[0]
	count := 1

	for i := 1; i < len(lines); i++ {
		if bytes.Equal(lines[i], current) {
			count++
		} else {
			if count > 2 {
				result = append(result, current)
				result = append(result, []byte(fmt.Sprintf("  ... (%d identical lines)", count-1)))
			} else {
				for j := 0; j < count; j++ {
					result = append(result, current)
				}
			}
			current = lines[i]
			count = 1
		}
	}
	// Flush last group
	if count > 2 {
		result = append(result, current)
		result = append(result, []byte(fmt.Sprintf("  ... (%d identical lines)", count-1)))
	} else {
		for j := 0; j < count; j++ {
			result = append(result, current)
		}
	}

	return bytes.Join(result, []byte("\n"))
}

var progressRegex = regexp.MustCompile(`(?m)^.*(\r|[\[=>#\-]{3,}|\.{3,}|\d+%|ETA|eta).*$`)

// StripProgress removes progress bars, spinners, and percentage indicators.
func StripProgress(data []byte) []byte {
	return progressRegex.ReplaceAll(data, nil)
}

// StripComments removes comment lines from linter output (lines starting with // or #).
func StripComments(data []byte) []byte {
	lines := bytes.Split(data, []byte("\n"))
	var result [][]byte
	for _, line := range lines {
		trimmed := bytes.TrimSpace(line)
		if bytes.HasPrefix(trimmed, []byte("//")) || bytes.HasPrefix(trimmed, []byte("#")) {
			continue
		}
		result = append(result, line)
	}
	return bytes.Join(result, []byte("\n"))
}

// LimitLineCount caps output at N lines with a summary of truncated lines.
func LimitLineCount(maxLines int) Filter {
	return func(data []byte) []byte {
		lines := bytes.Split(data, []byte("\n"))
		if len(lines) <= maxLines {
			return data
		}
		truncated := lines[:maxLines]
		truncated = append(truncated, []byte(fmt.Sprintf("\n... (%d more lines truncated)", len(lines)-maxLines)))
		return bytes.Join(truncated, []byte("\n"))
	}
}

// GroupByDirectory groups file listings by directory.
func GroupByDirectory(data []byte) []byte {
	lines := bytes.Split(data, []byte("\n"))
	groups := make(map[string][]string)
	var order []string

	for _, line := range lines {
		s := strings.TrimSpace(string(line))
		if s == "" {
			continue
		}
		dir := "."
		if idx := strings.LastIndex(s, "/"); idx >= 0 {
			dir = s[:idx]
		}
		if _, exists := groups[dir]; !exists {
			order = append(order, dir)
		}
		groups[dir] = append(groups[dir], s)
	}

	var buf bytes.Buffer
	for _, dir := range order {
		files := groups[dir]
		buf.WriteString(fmt.Sprintf("%s/ (%d files)\n", dir, len(files)))
		for _, f := range files {
			buf.WriteString("  " + f + "\n")
		}
	}
	return buf.Bytes()
}

// TruncateJSON truncates large JSON values to prevent bloated output.
func TruncateJSON(data []byte) []byte {
	// Simple heuristic: if a line is very long (>500 chars) and looks like JSON, truncate it
	lines := bytes.Split(data, []byte("\n"))
	var result [][]byte
	for _, line := range lines {
		if len(line) > 500 {
			// Keep first 200 chars + indicator
			truncated := make([]byte, 200)
			copy(truncated, line[:200])
			truncated = append(truncated, []byte(fmt.Sprintf("... (%d chars truncated)", len(line)-200))...)
			result = append(result, truncated)
		} else {
			result = append(result, line)
		}
	}
	return bytes.Join(result, []byte("\n"))
}

// SmartSummary replaces large repeated sections with one-liners.
func SmartSummary(data []byte) []byte {
	// Collapse "ok" test results that repeat
	lines := bytes.Split(data, []byte("\n"))
	if len(lines) < 10 {
		return data
	}

	var result [][]byte
	okCount := 0
	for _, line := range lines {
		s := string(line)
		if strings.HasPrefix(s, "ok  \t") || strings.HasPrefix(s, "ok \t") {
			okCount++
			if okCount <= 3 {
				result = append(result, line)
			}
		} else {
			if okCount > 3 {
				result = append(result, []byte(fmt.Sprintf("  ... (%d more passing packages)", okCount-3)))
				okCount = 0
			}
			result = append(result, line)
		}
	}
	if okCount > 3 {
		result = append(result, []byte(fmt.Sprintf("  ... (%d more passing packages)", okCount-3)))
	}
	return bytes.Join(result, []byte("\n"))
}
