package compress

import (
	"bytes"
	"fmt"
	"strings"
)

// genericPipeline returns a compression pipeline for common shell commands.
func genericPipeline(cmd string) *Pipeline {
	base := baseCommand(cmd)

	switch base {
	case "ls", "find", "fd":
		return NewPipeline(
			StripANSI,
			CollapseWhitespace,
			GroupByDirectory,
			LimitLineCount(100),
		)
	case "grep", "rg", "ag":
		return NewPipeline(
			StripANSI,
			CollapseWhitespace,
			TruncatePaths,
			DeduplicateLines,
			LimitLineCount(100),
		)
	case "cat", "head", "tail", "less":
		return NewPipeline(
			StripANSI,
			TruncateJSON,
			LimitLineCount(200),
		)
	case "docker":
		return NewPipeline(
			StripANSI,
			StripProgress,
			CollapseWhitespace,
			DeduplicateLines,
			LimitLineCount(80),
		)
	case "curl", "wget":
		return NewPipeline(
			StripANSI,
			StripProgress,
			TruncateJSON,
			LimitLineCount(100),
		)
	default:
		return NewPipeline(
			StripANSI,
			CollapseWhitespace,
			TruncatePaths,
			DeduplicateLines,
			LimitLineCount(200),
		)
	}
}

// UltraCompact is an opt-in extreme compression mode.
// It strips comments, collapses aggressively, and caps output at 50 lines.
func UltraCompact(data []byte) []byte {
	lines := bytes.Split(data, []byte("\n"))
	var result [][]byte

	for _, line := range lines {
		trimmed := bytes.TrimSpace(line)
		// Skip empty lines
		if len(trimmed) == 0 {
			continue
		}
		// Skip comment lines
		if bytes.HasPrefix(trimmed, []byte("//")) || bytes.HasPrefix(trimmed, []byte("#")) {
			continue
		}
		// Truncate long lines
		if len(line) > 120 {
			truncated := make([]byte, 120)
			copy(truncated, line[:120])
			truncated = append(truncated, []byte("...")...)
			result = append(result, truncated)
		} else {
			result = append(result, line)
		}
	}

	if len(result) > 50 {
		kept := result[:50]
		kept = append(kept, []byte(fmt.Sprintf("\n... (%d more lines, ultra-compact mode)", len(result)-50)))
		return bytes.Join(kept, []byte("\n"))
	}
	return bytes.Join(result, []byte("\n"))
}

// ForCommandUltra returns an ultra-compact pipeline for maximum compression.
func ForCommandUltra(command string) *Pipeline {
	return NewPipeline(
		StripANSI,
		StripProgress,
		StripComments,
		CollapseWhitespace,
		TruncatePaths,
		DeduplicateLines,
		UltraCompact,
	)
}

// CompressWithLevel runs compression at a specified level.
func CompressWithLevel(command string, raw []byte, ultra bool) ([]byte, Stats) {
	var pipeline *Pipeline
	if ultra {
		pipeline = ForCommandUltra(command)
	} else {
		pipeline = ForCommand(command)
	}
	output := pipeline.Run(raw)
	return output, Stats{
		InputBytes:  len(raw),
		OutputBytes: len(output),
		Command:     command,
	}
}

// EstimateTokens gives a rough token count estimate (~4 chars per token).
func EstimateTokens(data []byte) int {
	// Conservative estimate: ~4 bytes per token for English text
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return 0
	}
	words := len(strings.Fields(string(trimmed)))
	// Tokens ≈ 0.75 * words for code/CLI output
	tokens := int(float64(words) * 0.75)
	if tokens < 1 && len(trimmed) > 0 {
		return 1
	}
	return tokens
}
