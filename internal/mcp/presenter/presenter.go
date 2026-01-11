// Package presenter provides Markdown formatting for MCP tool responses.
// This package converts app layer response types into token-efficient Markdown
// suitable for LLM consumption, while the internal/ui package handles CLI output.
package presenter

import (
	"fmt"
	"strings"

	"github.com/josephgoksu/TaskWing/internal/app"
	"github.com/josephgoksu/TaskWing/internal/codeintel"
	"github.com/josephgoksu/TaskWing/internal/knowledge"
	"github.com/josephgoksu/TaskWing/internal/task"
)

// FormatRecall converts a RecallResult into token-efficient Markdown.
// Structure: Answer (if present) -> Knowledge -> Symbols
func FormatRecall(result *app.RecallResult) string {
	if result == nil {
		return "No results found."
	}

	var sb strings.Builder

	// Answer section (if present)
	if result.Answer != "" {
		sb.WriteString("## Answer\n")
		sb.WriteString(result.Answer)
		sb.WriteString("\n\n")
	}

	// Knowledge section
	if len(result.Results) > 0 {
		sb.WriteString("## Knowledge\n")
		for i, node := range result.Results {
			// Format: 1. **Title** (type) - content preview
			sb.WriteString(fmt.Sprintf("%d. **%s** (%s)", i+1, node.Summary, node.Type))
			if node.Content != "" && node.Content != node.Summary {
				// Add content preview (first 150 chars)
				content := cleanContent(node.Content, node.Summary)
				if content != "" {
					preview := truncate(content, 150)
					sb.WriteString(fmt.Sprintf("\n   %s", preview))
				}
			}
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	// Symbols section
	if len(result.Symbols) > 0 {
		sb.WriteString("## Code Symbols\n")
		for _, sym := range result.Symbols {
			// Format: - `Name` (kind) — file:line
			sb.WriteString(fmt.Sprintf("- `%s` (%s) — %s\n", sym.Name, sym.Kind, sym.Location))
		}
	}

	output := strings.TrimSpace(sb.String())
	if output == "" {
		return "No results found."
	}
	return output
}

// FormatTask converts a TaskResult into concise Markdown.
func FormatTask(result *app.TaskResult) string {
	if result == nil {
		return "No task information."
	}

	var sb strings.Builder

	// Status message
	if result.Message != "" {
		sb.WriteString(result.Message)
		sb.WriteString("\n\n")
	}

	// Task details
	if result.Task != nil {
		t := result.Task
		status := statusIcon(t.Status)
		sb.WriteString(fmt.Sprintf("## %s %s\n", status, t.Title))
		sb.WriteString(fmt.Sprintf("**ID**: `%s` | **Priority**: %d | **Status**: %s\n\n", t.ID, t.Priority, t.Status))

		if t.Description != "" {
			sb.WriteString(t.Description)
			sb.WriteString("\n\n")
		}

		// Acceptance criteria as checklist
		if len(t.AcceptanceCriteria) > 0 {
			sb.WriteString("### Acceptance Criteria\n")
			for _, ac := range t.AcceptanceCriteria {
				checkbox := "[ ]"
				if t.Status == task.StatusCompleted {
					checkbox = "[x]"
				}
				sb.WriteString(fmt.Sprintf("- %s %s\n", checkbox, ac))
			}
			sb.WriteString("\n")
		}

		// Validation steps
		if len(t.ValidationSteps) > 0 {
			sb.WriteString("### Validation\n")
			sb.WriteString("```bash\n")
			for _, step := range t.ValidationSteps {
				sb.WriteString(step + "\n")
			}
			sb.WriteString("```\n")
		}
	}

	// Hint for next action
	if result.Hint != "" {
		sb.WriteString(fmt.Sprintf("\n> **Hint**: %s\n", result.Hint))
	}

	// Context (already Markdown)
	if result.Context != "" {
		sb.WriteString("\n---\n")
		sb.WriteString(result.Context)
	}

	return strings.TrimSpace(sb.String())
}

// FormatPlan converts a Plan into concise Markdown.
func FormatPlan(plan *task.Plan) string {
	if plan == nil {
		return "No plan information."
	}

	var sb strings.Builder

	// Header
	sb.WriteString(fmt.Sprintf("## Plan: %s\n", plan.Goal))
	sb.WriteString(fmt.Sprintf("**ID**: `%s` | **Status**: %s\n\n", plan.ID, plan.Status))

	// Task list
	if len(plan.Tasks) > 0 {
		sb.WriteString("### Tasks\n")
		completed := 0
		for _, t := range plan.Tasks {
			checkbox := "[ ]"
			if t.Status == task.StatusCompleted {
				checkbox = "[x]"
				completed++
			} else if t.Status == task.StatusInProgress {
				checkbox = "[~]"
			}
			sb.WriteString(fmt.Sprintf("- %s %s (P%d)\n", checkbox, t.Title, t.Priority))
		}
		sb.WriteString(fmt.Sprintf("\n**Progress**: %d/%d tasks completed\n", completed, len(plan.Tasks)))
	}

	return strings.TrimSpace(sb.String())
}

// FormatSymbolList converts a list of symbols into concise Markdown.
func FormatSymbolList(symbols []codeintel.Symbol) string {
	if len(symbols) == 0 {
		return "No symbols found."
	}

	var sb strings.Builder
	sb.WriteString("## Symbols\n")

	for _, sym := range symbols {
		// Format: - `Name` (kind) — file:line
		location := fmt.Sprintf("%s:%d", sym.FilePath, sym.StartLine)
		visibility := ""
		if sym.Visibility == "private" {
			visibility = " (private)"
		}
		sb.WriteString(fmt.Sprintf("- `%s`%s (%s) — %s\n", sym.Name, visibility, sym.Kind, location))

		// Add signature for functions/methods
		if sym.Signature != "" && (sym.Kind == codeintel.SymbolFunction || sym.Kind == codeintel.SymbolMethod) {
			sig := truncate(sym.Signature, 60)
			sb.WriteString(fmt.Sprintf("  `%s`\n", sig))
		}
	}

	return strings.TrimSpace(sb.String())
}

// FormatSearchResults converts semantic search results into Markdown.
func FormatSearchResults(results []codeintel.SymbolSearchResult) string {
	if len(results) == 0 {
		return "No matching symbols found."
	}

	var sb strings.Builder
	sb.WriteString("## Search Results\n")

	for i, r := range results {
		location := fmt.Sprintf("%s:%d", r.Symbol.FilePath, r.Symbol.StartLine)
		sb.WriteString(fmt.Sprintf("%d. `%s` (%s) — %s\n", i+1, r.Symbol.Name, r.Symbol.Kind, location))

		// Score indicator
		scoreBar := scoreToBar(r.Score)
		sb.WriteString(fmt.Sprintf("   Score: %s %.2f\n", scoreBar, r.Score))

		// Doc comment preview
		if r.Symbol.DocComment != "" {
			doc := truncate(r.Symbol.DocComment, 80)
			sb.WriteString(fmt.Sprintf("   > %s\n", doc))
		}
	}

	return strings.TrimSpace(sb.String())
}

// FormatCallers converts a GetCallersResult into Markdown.
func FormatCallers(result *app.GetCallersResult) string {
	if result == nil || !result.Success {
		msg := "Failed to get callers."
		if result != nil && result.Message != "" {
			msg = result.Message
		}
		return msg
	}

	var sb strings.Builder

	// Target symbol
	if result.Symbol != nil {
		sym := result.Symbol
		sb.WriteString(fmt.Sprintf("## `%s` (%s)\n", sym.Name, sym.Kind))
		sb.WriteString(fmt.Sprintf("%s:%d\n\n", sym.FilePath, sym.StartLine))
	}

	// Callers
	if len(result.Callers) > 0 {
		sb.WriteString("### Called By\n")
		for _, caller := range result.Callers {
			sb.WriteString(fmt.Sprintf("- `%s` — %s:%d\n", caller.Name, caller.FilePath, caller.StartLine))
		}
		sb.WriteString("\n")
	}

	// Callees
	if len(result.Callees) > 0 {
		sb.WriteString("### Calls\n")
		for _, callee := range result.Callees {
			sb.WriteString(fmt.Sprintf("- `%s` — %s:%d\n", callee.Name, callee.FilePath, callee.StartLine))
		}
	}

	output := strings.TrimSpace(sb.String())
	if output == "" {
		return "No call relationships found."
	}
	return output
}

// FormatImpact converts an AnalyzeImpactResult into Markdown.
func FormatImpact(result *app.AnalyzeImpactResult) string {
	if result == nil || !result.Success {
		msg := "Failed to analyze impact."
		if result != nil && result.Message != "" {
			msg = result.Message
		}
		return msg
	}

	var sb strings.Builder

	// Source symbol
	if result.Source != nil {
		sym := result.Source
		sb.WriteString(fmt.Sprintf("## Impact Analysis: `%s`\n", sym.Name))
		sb.WriteString(fmt.Sprintf("%s:%d\n\n", sym.FilePath, sym.StartLine))
	}

	// Summary
	sb.WriteString(fmt.Sprintf("**Affected**: %d symbols across %d files (max depth: %d)\n\n",
		result.AffectedCount, result.AffectedFiles, result.MaxDepth))

	// By depth
	if len(result.ByDepth) > 0 {
		sb.WriteString("### Blast Radius\n")
		for depth := 1; depth <= result.MaxDepth; depth++ {
			if symbols, ok := result.ByDepth[depth]; ok && len(symbols) > 0 {
				sb.WriteString(fmt.Sprintf("**Depth %d** (%d symbols):\n", depth, len(symbols)))
				for _, sym := range symbols {
					sb.WriteString(fmt.Sprintf("- `%s` — %s:%d\n", sym.Name, sym.FilePath, sym.StartLine))
				}
				sb.WriteString("\n")
			}
		}
	}

	return strings.TrimSpace(sb.String())
}

// FormatRemember formats a remember operation result.
func FormatRemember(result *app.AddResult) string {
	if result == nil {
		return "Failed to add knowledge."
	}

	if result.ID == "" {
		return "Failed to add knowledge."
	}

	return fmt.Sprintf("Added knowledge `%s` (%s): %s", result.ID, result.Type, result.Summary)
}

// === Helper Functions ===

// statusIcon returns an emoji for task status
func statusIcon(status task.TaskStatus) string {
	switch status {
	case task.StatusPending:
		return "⏳"
	case task.StatusInProgress:
		return "🔄"
	case task.StatusCompleted:
		return "✅"
	case task.StatusFailed:
		return "❌"
	case task.StatusVerifying:
		return "🔍"
	default:
		return "📋"
	}
}

// truncate shortens a string to maxLen and adds ellipsis
func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}

// cleanContent removes summary prefix from content
func cleanContent(content, summary string) string {
	if len(summary) < 3 {
		return content
	}
	if remainder, found := strings.CutPrefix(content, summary); found {
		return strings.TrimLeft(remainder, "\n\r\t ")
	}
	return content
}

// scoreToBar converts a 0-1 score to a visual bar
func scoreToBar(score float32) string {
	bars := int(score * 5)
	if bars < 1 && score > 0 {
		bars = 1
	}
	if bars > 5 {
		bars = 5
	}
	return strings.Repeat("█", bars) + strings.Repeat("░", 5-bars)
}

// FormatNodeResponse formats a single knowledge node response.
func FormatNodeResponse(node knowledge.NodeResponse) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("**%s** (%s)", node.Summary, node.Type))
	if node.Content != "" && node.Content != node.Summary {
		content := cleanContent(node.Content, node.Summary)
		if content != "" {
			preview := truncate(content, 150)
			sb.WriteString(fmt.Sprintf("\n%s", preview))
		}
	}
	return sb.String()
}
