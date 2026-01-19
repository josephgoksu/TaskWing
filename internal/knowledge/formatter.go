// Package knowledge provides compact Markdown formatting for recall results.
// This formatter produces token-efficient output by grouping nodes by type
// and removing all JSON metadata and embedding data.
package knowledge

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// CompactFormatter produces condensed Markdown output for recall results.
// It groups nodes by type and strips all unnecessary metadata to minimize tokens.
type CompactFormatter struct {
	// MaxContentLen limits content preview length per node (default: 120)
	MaxContentLen int
	// MaxNodesPerType limits nodes shown per type (default: 5)
	MaxNodesPerType int
	// ShowEvidence includes file:line references (default: false for token savings)
	ShowEvidence bool
}

// DefaultCompactFormatter returns a formatter with sensible defaults for LLM consumption.
func DefaultCompactFormatter() *CompactFormatter {
	return &CompactFormatter{
		MaxContentLen:   120,
		MaxNodesPerType: 5,
		ShowEvidence:    false,
	}
}

// FormatNodes converts NodeResponse slice into compact, grouped Markdown.
// Output structure: Decisions -> Patterns -> Constraints -> Features -> Other
// This reduces token count by ~50% compared to JSON or verbose formats.
func (f *CompactFormatter) FormatNodes(nodes []NodeResponse) string {
	if len(nodes) == 0 {
		return "No results found."
	}

	// Group nodes by type
	grouped := f.groupByType(nodes)

	var sb strings.Builder

	// Render in priority order
	typeOrder := []string{"decision", "pattern", "constraint", "feature", "documentation", "note"}
	for _, typeName := range typeOrder {
		if group, ok := grouped[typeName]; ok && len(group) > 0 {
			f.renderTypeGroup(&sb, typeName, group)
		}
	}

	// Handle any remaining types not in our priority list
	for typeName, group := range grouped {
		if !slices.Contains(typeOrder, typeName) && len(group) > 0 {
			f.renderTypeGroup(&sb, typeName, group)
		}
	}

	result := strings.TrimSpace(sb.String())
	if result == "" {
		return "No results found."
	}
	return result
}

// FormatWithAnswer formats nodes with an optional AI-generated answer.
func (f *CompactFormatter) FormatWithAnswer(answer string, nodes []NodeResponse, symbols []SymbolMatch) string {
	var sb strings.Builder

	// Answer first (most important for LLM)
	if answer != "" {
		sb.WriteString("## Answer\n")
		sb.WriteString(answer)
		sb.WriteString("\n\n")
	}

	// Knowledge nodes grouped by type
	if len(nodes) > 0 {
		sb.WriteString(f.FormatNodes(nodes))
		sb.WriteString("\n")
	}

	// Code symbols (compact format)
	if len(symbols) > 0 {
		sb.WriteString("\n## Symbols\n")
		for _, sym := range symbols {
			fmt.Fprintf(&sb, "- `%s` (%s) — %s\n", sym.Name, sym.Kind, sym.Location)
		}
	}

	return strings.TrimSpace(sb.String())
}

// SymbolMatch is a compact representation of a code symbol for formatting.
type SymbolMatch struct {
	Name     string
	Kind     string
	Location string
}

// groupByType organizes nodes into type buckets.
func (f *CompactFormatter) groupByType(nodes []NodeResponse) map[string][]NodeResponse {
	grouped := make(map[string][]NodeResponse)
	for _, node := range nodes {
		typeName := normalizeType(node.Type)
		grouped[typeName] = append(grouped[typeName], node)
	}

	// Sort each group by score (highest first)
	for typeName := range grouped {
		sort.Slice(grouped[typeName], func(i, j int) bool {
			return grouped[typeName][i].MatchScore > grouped[typeName][j].MatchScore
		})
		// Apply max limit
		if f.MaxNodesPerType > 0 && len(grouped[typeName]) > f.MaxNodesPerType {
			grouped[typeName] = grouped[typeName][:f.MaxNodesPerType]
		}
	}

	return grouped
}

// renderTypeGroup renders a single type group with icon and formatted entries.
func (f *CompactFormatter) renderTypeGroup(sb *strings.Builder, typeName string, nodes []NodeResponse) {
	icon := typeIcon(typeName)
	title := formatTypeName(typeName)

	fmt.Fprintf(sb, "### %s %s\n", icon, title)

	for _, node := range nodes {
		// Compact single-line format: - **Summary**: content_preview
		fmt.Fprintf(sb, "- **%s**", node.Summary)

		// Add truncated content if different from summary
		content := cleanContentPreview(node.Content, node.Summary, f.MaxContentLen)
		if content != "" {
			fmt.Fprintf(sb, ": %s", content)
		}

		// Add evidence references if enabled
		if f.ShowEvidence && len(node.Evidence) > 0 {
			refs := formatEvidenceRefs(node.Evidence, 2) // Max 2 refs
			if refs != "" {
				fmt.Fprintf(sb, " [%s]", refs)
			}
		}

		// Add debt warning inline if present
		if node.DebtWarning != "" {
			fmt.Fprintf(sb, " ⚠️ %s", node.DebtWarning)
		}

		sb.WriteString("\n")
	}
	sb.WriteString("\n")
}

// === Helper Functions ===

// normalizeType standardizes type names for grouping.
func normalizeType(t string) string {
	t = strings.ToLower(strings.TrimSpace(t))
	// Map common variants
	switch t {
	case "decisions", "architectural_decision":
		return "decision"
	case "patterns", "architectural_pattern":
		return "pattern"
	case "constraints", "architectural_constraint":
		return "constraint"
	case "features":
		return "feature"
	case "docs", "doc":
		return "documentation"
	case "notes":
		return "note"
	}
	return t
}

// typeIcon returns a compact icon for each type.
func typeIcon(typeName string) string {
	switch typeName {
	case "decision":
		return "📋"
	case "pattern":
		return "🧩"
	case "constraint":
		return "⚠️"
	case "feature":
		return "✨"
	case "documentation":
		return "📄"
	case "note":
		return "📌"
	default:
		return "•"
	}
}

// formatTypeName converts type to display title.
func formatTypeName(typeName string) string {
	switch typeName {
	case "decision":
		return "Decisions"
	case "pattern":
		return "Patterns"
	case "constraint":
		return "Constraints"
	case "feature":
		return "Features"
	case "documentation":
		return "Documentation"
	case "note":
		return "Notes"
	default:
		return cases.Title(language.English).String(typeName)
	}
}

// cleanContentPreview extracts meaningful content preview, removing redundant summary.
func cleanContentPreview(content, summary string, maxLen int) string {
	if content == "" || content == summary {
		return ""
	}

	// Remove summary prefix if present
	cleaned := content
	if after, found := strings.CutPrefix(content, summary); found {
		cleaned = strings.TrimLeft(after, "\n\r\t :.-")
	}

	// Remove newlines for compact single-line output
	cleaned = strings.ReplaceAll(cleaned, "\n", " ")
	cleaned = strings.ReplaceAll(cleaned, "\r", "")

	// Collapse multiple spaces
	for strings.Contains(cleaned, "  ") {
		cleaned = strings.ReplaceAll(cleaned, "  ", " ")
	}

	cleaned = strings.TrimSpace(cleaned)

	// Truncate
	if maxLen > 0 && len(cleaned) > maxLen {
		cleaned = cleaned[:maxLen] + "..."
	}

	return cleaned
}

// formatEvidenceRefs formats evidence references compactly.
func formatEvidenceRefs(evidence []EvidenceRef, maxRefs int) string {
	if len(evidence) == 0 {
		return ""
	}

	var refs []string
	for i, e := range evidence {
		if i >= maxRefs {
			break
		}
		if e.Lines != "" {
			refs = append(refs, fmt.Sprintf("%s:%s", e.File, e.Lines))
		} else {
			refs = append(refs, e.File)
		}
	}

	return strings.Join(refs, ", ")
}

// TokenEstimate provides rough token count for a formatted string.
// Uses ~4 chars per token as a simple heuristic.
func TokenEstimate(text string) int {
	return len(text) / 4
}
