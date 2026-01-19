// Package presenter provides Markdown formatting for MCP tool responses.
// This package converts app layer response types into token-efficient Markdown
// suitable for LLM consumption, while the internal/ui package handles CLI output.
package mcp

import (
	"fmt"
	"strings"

	agentcore "github.com/josephgoksu/TaskWing/internal/agents/core"
	agentimpl "github.com/josephgoksu/TaskWing/internal/agents/impl"
	"github.com/josephgoksu/TaskWing/internal/app"
	"github.com/josephgoksu/TaskWing/internal/codeintel"
	"github.com/josephgoksu/TaskWing/internal/knowledge"
	"github.com/josephgoksu/TaskWing/internal/policy"
	"github.com/josephgoksu/TaskWing/internal/task"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// FormatRecall converts a RecallResult into token-efficient Markdown.
// Structure: Answer (if present) -> Knowledge -> Symbols
// Includes debt warnings for patterns/decisions marked as technical debt.
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
			// Add debt warning if this is technical debt
			if node.DebtWarning != "" {
				sb.WriteString(fmt.Sprintf("\n   %s", node.DebtWarning))
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

// FormatTaskCompletionBlocked formats a blocked task completion (e.g., policy violations) into Markdown.
// This provides AI agents with clear, actionable information about why completion was blocked.
func FormatTaskCompletionBlocked(result *app.TaskResult) string {
	if result == nil {
		return FormatError("No task result available.")
	}

	var sb strings.Builder

	// Error header
	sb.WriteString("## ❌ Task Completion Blocked\n\n")

	// Task info if available
	if result.Task != nil {
		sb.WriteString(fmt.Sprintf("**Task**: %s\n", result.Task.Title))
		sb.WriteString(fmt.Sprintf("**ID**: `%s`\n", result.Task.ID))
		sb.WriteString(fmt.Sprintf("**Status**: %s (unchanged)\n\n", result.Task.Status))
	}

	// Reason/Message - format as violations if it contains the policy violation pattern
	if result.Message != "" {
		if strings.Contains(result.Message, "Policy violations") {
			sb.WriteString("### Policy Violations\n\n")
			sb.WriteString("The following policy rules blocked task completion:\n\n")
			// The message already contains formatted violations from TaskApp.Complete
			sb.WriteString(result.Message)
		} else {
			sb.WriteString("### Reason\n\n")
			sb.WriteString(result.Message)
		}
		sb.WriteString("\n\n")
	}

	// Next steps hint
	sb.WriteString("### Next Steps\n\n")
	sb.WriteString("1. Review the violations above\n")
	sb.WriteString("2. Remove or modify the blocked files from your changes\n")
	sb.WriteString("3. Retry task completion with `task_complete`\n")

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
			switch t.Status {
			case task.StatusCompleted:
				checkbox = "[x]"
				completed++
			case task.StatusInProgress:
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

// FormatExplainResult converts an ExplainResult into Markdown for MCP.
func FormatExplainResult(result *app.ExplainResult) string {
	if result == nil {
		return "No explanation available."
	}

	var sb strings.Builder

	// Symbol header
	sb.WriteString(fmt.Sprintf("## `%s` (%s)\n", result.Symbol.Name, result.Symbol.Kind))
	sb.WriteString(fmt.Sprintf("%s\n\n", result.Symbol.Location))

	if result.Symbol.Signature != "" {
		sb.WriteString(fmt.Sprintf("```\n%s\n```\n\n", result.Symbol.Signature))
	}

	if result.Symbol.DocComment != "" {
		sb.WriteString(fmt.Sprintf("> %s\n\n", truncate(result.Symbol.DocComment, 200)))
	}

	// Call graph context
	sb.WriteString("### System Context\n\n")

	// Callers
	sb.WriteString(fmt.Sprintf("**Called By** (%d):\n", len(result.Callers)))
	if len(result.Callers) == 0 {
		sb.WriteString("- *(none - may be entry point)*\n")
	} else {
		for i, c := range result.Callers {
			if i >= 5 {
				sb.WriteString(fmt.Sprintf("- *...and %d more*\n", len(result.Callers)-5))
				break
			}
			sb.WriteString(fmt.Sprintf("- `%s` — %s\n", c.Symbol.Name, c.Symbol.Location))
		}
	}

	// Callees
	sb.WriteString(fmt.Sprintf("\n**Calls** (%d):\n", len(result.Callees)))
	if len(result.Callees) == 0 {
		sb.WriteString("- *(none - may be leaf function)*\n")
	} else {
		for i, c := range result.Callees {
			if i >= 5 {
				sb.WriteString(fmt.Sprintf("- *...and %d more*\n", len(result.Callees)-5))
				break
			}
			sb.WriteString(fmt.Sprintf("- `%s` — %s\n", c.Symbol.Name, c.Symbol.Location))
		}
	}

	// Impact summary
	sb.WriteString("\n### Impact Analysis\n")
	sb.WriteString(fmt.Sprintf("- Direct callers: %d\n", result.ImpactStats.DirectCallers))
	sb.WriteString(fmt.Sprintf("- Direct callees: %d\n", result.ImpactStats.DirectCallees))
	if result.ImpactStats.TransitiveDependents > 0 {
		sb.WriteString(fmt.Sprintf("- Transitive dependents: %d (depth %d)\n",
			result.ImpactStats.TransitiveDependents, result.ImpactStats.MaxDepthReached))
	}
	if result.ImpactStats.AffectedFiles > 0 {
		sb.WriteString(fmt.Sprintf("- Files affected: %d\n", result.ImpactStats.AffectedFiles))
	}

	// Related decisions
	if len(result.Decisions) > 0 {
		sb.WriteString("\n### Related Decisions\n")
		for _, d := range result.Decisions {
			sb.WriteString(fmt.Sprintf("- %s\n", d.Summary))
		}
	}

	// Related patterns
	if len(result.Patterns) > 0 {
		sb.WriteString("\n### Related Patterns\n")
		for _, p := range result.Patterns {
			sb.WriteString(fmt.Sprintf("- %s\n", p.Summary))
		}
	}

	// Source code snippets (condensed for tokens)
	if len(result.SourceCode) > 0 {
		sb.WriteString("\n### Source Context\n")
		for _, snippet := range result.SourceCode {
			sb.WriteString(fmt.Sprintf("\n**%s `%s`** (%s):\n", snippet.Kind, snippet.SymbolName, snippet.FilePath))
			// Limit to first 20 lines for tokens
			lines := strings.Split(snippet.Content, "\n")
			if len(lines) > 20 {
				sb.WriteString("```\n")
				sb.WriteString(strings.Join(lines[:20], "\n"))
				sb.WriteString(fmt.Sprintf("\n// ...%d more lines\n", len(lines)-20))
				sb.WriteString("```\n")
			} else {
				sb.WriteString("```\n")
				sb.WriteString(snippet.Content)
				sb.WriteString("\n```\n")
			}
		}
	}

	// AI Explanation
	if result.Explanation != "" {
		sb.WriteString("\n### Explanation\n")
		sb.WriteString(result.Explanation)
		sb.WriteString("\n")
	}

	return strings.TrimSpace(sb.String())
}

// FormatDriftReport converts a DriftReport into Markdown for MCP.
func FormatDriftReport(report *app.DriftReport) string {
	if report == nil {
		return "No drift report available."
	}

	var sb strings.Builder

	// Header
	sb.WriteString("## Architecture Drift Analysis\n\n")
	sb.WriteString(fmt.Sprintf("**Rules checked**: %d\n", report.RulesChecked))
	sb.WriteString(fmt.Sprintf("**Timestamp**: %s\n\n", report.Timestamp.Format("2006-01-02 15:04:05")))

	// No rules
	if report.RulesChecked == 0 {
		sb.WriteString("No architectural rules found in knowledge base.\n")
		sb.WriteString("Run `tw bootstrap` to extract rules, or add rules with `tw add`.\n")
		return sb.String()
	}

	// Violations
	if len(report.Violations) > 0 {
		sb.WriteString(fmt.Sprintf("### ❌ Violations (%d)\n", len(report.Violations)))
		for i, v := range report.Violations {
			if i >= 10 {
				sb.WriteString(fmt.Sprintf("\n*...and %d more violations*\n", len(report.Violations)-10))
				break
			}
			sb.WriteString(fmt.Sprintf("\n**%d. %s**\n", i+1, v.Location))
			if v.Rule != nil {
				sb.WriteString(fmt.Sprintf("- Rule: %s\n", v.Rule.Name))
			}
			sb.WriteString(fmt.Sprintf("- Issue: %s\n", v.Message))
			if v.Evidence != "" {
				sb.WriteString(fmt.Sprintf("- Evidence: `%s`\n", v.Evidence))
			}
			if v.Suggestion != "" {
				sb.WriteString(fmt.Sprintf("- Fix: %s\n", v.Suggestion))
			}
		}
		sb.WriteString("\n")
	}

	// Warnings
	if len(report.Warnings) > 0 {
		sb.WriteString(fmt.Sprintf("### ⚠️ Warnings (%d)\n", len(report.Warnings)))
		for i, w := range report.Warnings {
			if i >= 5 {
				sb.WriteString(fmt.Sprintf("\n*...and %d more warnings*\n", len(report.Warnings)-5))
				break
			}
			sb.WriteString(fmt.Sprintf("- %s: %s\n", w.Location, w.Message))
		}
		sb.WriteString("\n")
	}

	// Passed
	if len(report.Passed) > 0 {
		sb.WriteString(fmt.Sprintf("### ✅ Passed (%d)\n", len(report.Passed)))
		for _, name := range report.Passed {
			sb.WriteString(fmt.Sprintf("- %s\n", name))
		}
		sb.WriteString("\n")
	}

	// Summary
	sb.WriteString("### Summary\n")
	sb.WriteString(fmt.Sprintf("- Violations: %d\n", report.Summary.Violations))
	sb.WriteString(fmt.Sprintf("- Warnings: %d\n", report.Summary.Warnings))
	sb.WriteString(fmt.Sprintf("- Passed: %d\n", report.Summary.Passed))

	return strings.TrimSpace(sb.String())
}

// FormatRemember formats a remember operation result.
func FormatRemember(result *app.AddResult) string {
	if result == nil {
		return FormatError("Failed to add knowledge.")
	}

	if result.ID == "" {
		return FormatError("Failed to add knowledge.")
	}

	var sb strings.Builder
	sb.WriteString("## ✅ Knowledge Saved\n\n")
	sb.WriteString(fmt.Sprintf("**ID**: `%s`\n", result.ID))
	sb.WriteString(fmt.Sprintf("**Type**: %s\n", result.Type))
	sb.WriteString(fmt.Sprintf("**Summary**: %s\n", result.Summary))
	if result.HasEmbedding {
		sb.WriteString("\n*Embedding generated for semantic search.*\n")
	}
	return strings.TrimSpace(sb.String())
}

// === Error Formatters ===

// FormatError returns a standardized Markdown error message.
// Use this for all MCP tool error responses to ensure consistency.
func FormatError(message string) string {
	return fmt.Sprintf("## ❌ Error\n\n**Details**: %s", message)
}

// FormatValidationError returns a Markdown error for validation failures.
func FormatValidationError(field, message string) string {
	return fmt.Sprintf("## ❌ Validation Error\n\n**Field**: `%s`\n**Details**: %s", field, message)
}

// === Summary Formatter ===

// FormatSummary converts a ProjectSummary into token-efficient Markdown.
func FormatSummary(summary *knowledge.ProjectSummary) string {
	if summary == nil {
		return "No project summary available."
	}

	var sb strings.Builder

	// Project overview
	if summary.Overview != nil && summary.Overview.ShortDescription != "" {
		sb.WriteString("## Project Overview\n")
		sb.WriteString(summary.Overview.ShortDescription)
		sb.WriteString("\n\n")
	}

	// Knowledge summary
	sb.WriteString(fmt.Sprintf("## Knowledge Base: %d nodes\n\n", summary.Total))

	if len(summary.Types) > 0 {
		// Sort types for consistent output
		typeOrder := []string{"decision", "pattern", "constraint", "feature", "plan", "note"}
		for _, typeName := range typeOrder {
			if ts, ok := summary.Types[typeName]; ok && ts.Count > 0 {
				icon := typeIcon(typeName)
				sb.WriteString(fmt.Sprintf("### %s %s (%d)\n", icon, cases.Title(language.English).String(typeName)+"s", ts.Count))
				for _, example := range ts.Examples {
					sb.WriteString(fmt.Sprintf("- %s\n", example))
				}
				sb.WriteString("\n")
			}
		}
	}

	return strings.TrimSpace(sb.String())
}

// === Plan Formatters ===

// FormatClarifyResult formats plan clarification output.
func FormatClarifyResult(result *app.ClarifyResult) string {
	if result == nil {
		return FormatError("No clarification result.")
	}

	if !result.Success {
		msg := result.Message
		if msg == "" {
			msg = "Clarification failed with no details"
		}
		return FormatError(msg)
	}

	var sb strings.Builder

	// Ready status
	if result.IsReadyToPlan {
		sb.WriteString("## ✅ Ready to Generate Plan\n\n")
	} else {
		sb.WriteString("## 🔍 Clarification Needed\n\n")
	}

	// Goal summary
	if result.GoalSummary != "" {
		sb.WriteString(fmt.Sprintf("**Goal**: %s\n\n", result.GoalSummary))
	}

	// Questions (if not ready)
	if len(result.Questions) > 0 && !result.IsReadyToPlan {
		sb.WriteString("### Questions\n")
		for i, q := range result.Questions {
			sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, q))
		}
		sb.WriteString("\n")
	}

	// Enriched goal (if ready)
	if result.EnrichedGoal != "" && result.IsReadyToPlan {
		sb.WriteString("### Enriched Specification\n")
		sb.WriteString(result.EnrichedGoal)
		sb.WriteString("\n\n")
		sb.WriteString("> **Next**: Call `plan_generate` with this `enriched_goal` to create tasks.\n")
	}

	// Context used
	if result.ContextUsed != "" {
		sb.WriteString(fmt.Sprintf("\n*%s*\n", result.ContextUsed))
	}

	return strings.TrimSpace(sb.String())
}

// FormatGenerateResult formats plan generation output.
func FormatGenerateResult(result *app.GenerateResult) string {
	if result == nil {
		return FormatError("No generation result.")
	}

	if !result.Success {
		msg := result.Message
		if msg == "" {
			msg = "Plan generation failed with no details"
		}
		return FormatError(msg)
	}

	var sb strings.Builder

	sb.WriteString("## ✅ Plan Generated\n\n")
	sb.WriteString(fmt.Sprintf("**Plan ID**: `%s`\n", result.PlanID))
	sb.WriteString(fmt.Sprintf("**Goal**: %s\n\n", result.Goal))

	// Tasks
	if len(result.Tasks) > 0 {
		sb.WriteString("### Tasks\n")
		for i, t := range result.Tasks {
			sb.WriteString(fmt.Sprintf("%d. **%s** (P%d)\n", i+1, t.Title, t.Priority))
			if t.Description != "" {
				desc := truncate(t.Description, 100)
				sb.WriteString(fmt.Sprintf("   %s\n", desc))
			}
		}
		sb.WriteString("\n")
	}

	// Hint
	if result.Hint != "" {
		sb.WriteString(fmt.Sprintf("> **Hint**: %s\n", result.Hint))
	}

	return strings.TrimSpace(sb.String())
}

// FormatAuditResult formats plan audit output.
func FormatAuditResult(result *app.AuditResult) string {
	if result == nil {
		return FormatError("No audit result.")
	}

	if !result.Success {
		msg := result.Message
		if msg == "" {
			msg = "Audit failed with no details"
		}
		return FormatError(msg)
	}

	var sb strings.Builder

	// Status header
	statusIcon := "🔍"
	switch result.Status {
	case "verified":
		statusIcon = "✅"
	case "needs_revision":
		statusIcon = "⚠️"
	case "failed":
		statusIcon = "❌"
	}

	sb.WriteString(fmt.Sprintf("## %s Audit: %s\n\n", statusIcon, cases.Title(language.English).String(result.Status)))
	sb.WriteString(fmt.Sprintf("**Plan ID**: `%s`\n", result.PlanID))
	sb.WriteString(fmt.Sprintf("**Attempts**: %d\n\n", result.RetryCount))

	// Check results
	sb.WriteString("### Checks\n")
	buildIcon := "❌"
	if result.BuildPassed {
		buildIcon = "✅"
	}
	testIcon := "❌"
	if result.TestsPassed {
		testIcon = "✅"
	}
	sb.WriteString(fmt.Sprintf("- %s Build\n", buildIcon))
	sb.WriteString(fmt.Sprintf("- %s Tests\n", testIcon))
	sb.WriteString("\n")

	// Semantic issues
	if len(result.SemanticIssues) > 0 {
		sb.WriteString("### Semantic Issues\n")
		for _, issue := range result.SemanticIssues {
			sb.WriteString(fmt.Sprintf("- %s\n", issue))
		}
		sb.WriteString("\n")
	}

	// Fixes applied
	if len(result.FixesApplied) > 0 {
		sb.WriteString("### Fixes Applied\n")
		for _, fix := range result.FixesApplied {
			sb.WriteString(fmt.Sprintf("- %s\n", fix))
		}
		sb.WriteString("\n")
	}

	// Message and hint
	if result.Message != "" {
		sb.WriteString(fmt.Sprintf("%s\n\n", result.Message))
	}
	if result.Hint != "" {
		sb.WriteString(fmt.Sprintf("> **Hint**: %s\n", result.Hint))
	}

	return strings.TrimSpace(sb.String())
}

// === Helper Functions ===

// typeIcon returns an emoji for knowledge node type
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
	case "plan":
		return "📝"
	case "note":
		return "📌"
	default:
		return "📄"
	}
}

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
// Includes debt warning if the node is classified as technical debt.
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
	// Add debt warning if this is technical debt
	if node.DebtWarning != "" {
		sb.WriteString(fmt.Sprintf("\n%s", node.DebtWarning))
	}
	return sb.String()
}

// FormatDebugResult formats the output from the DebugAgent.
func FormatDebugResult(findings []agentcore.Finding) string {
	if len(findings) == 0 {
		return "No debug analysis available."
	}

	var sb strings.Builder
	for _, f := range findings {
		sb.WriteString("## Debug Analysis\n\n")
		sb.WriteString(fmt.Sprintf("**Most Likely Cause**: %s\n\n", f.Description))

		// Hypotheses - handle both direct type and []interface{} from JSON
		if hypothesesRaw, ok := f.Metadata["hypotheses"]; ok {
			hypotheses := extractDebugHypotheses(hypothesesRaw)
			if len(hypotheses) > 0 {
				sb.WriteString("### Hypotheses\n")
				for i, h := range hypotheses {
					var icon string
					switch h.Likelihood {
					case "high":
						icon = "🔴"
					case "medium":
						icon = "🟡"
					default:
						icon = "🔵"
					}
					sb.WriteString(fmt.Sprintf("\n%d. %s **%s** (%s)\n", i+1, icon, h.Cause, h.Likelihood))
					sb.WriteString(fmt.Sprintf("   %s\n", h.Reasoning))
					if len(h.CodeLocations) > 0 {
						sb.WriteString("   📍 Check: ")
						sb.WriteString(strings.Join(h.CodeLocations, ", "))
						sb.WriteString("\n")
					}
				}
				sb.WriteString("\n")
			}
		}

		// Investigation steps - handle both direct type and []interface{} from JSON
		if stepsRaw, ok := f.Metadata["investigation_steps"]; ok {
			steps := extractDebugSteps(stepsRaw)
			if len(steps) > 0 {
				sb.WriteString("### Investigation Steps\n")
				for _, s := range steps {
					sb.WriteString(fmt.Sprintf("%d. **%s**\n", s.Step, s.Action))
					if s.Command != "" {
						sb.WriteString(fmt.Sprintf("   ```\n   %s\n   ```\n", s.Command))
					}
					if s.ExpectedFinding != "" {
						sb.WriteString(fmt.Sprintf("   → Look for: %s\n", s.ExpectedFinding))
					}
				}
				sb.WriteString("\n")
			}
		}

		// Quick fixes - handle both direct type and []interface{} from JSON
		if fixesRaw, ok := f.Metadata["quick_fixes"]; ok {
			fixes := extractDebugFixes(fixesRaw)
			if len(fixes) > 0 {
				sb.WriteString("### Quick Fixes\n")
				for _, fix := range fixes {
					sb.WriteString(fmt.Sprintf("- **%s**", fix.Fix))
					if fix.When != "" {
						sb.WriteString(fmt.Sprintf(" (%s)", fix.When))
					}
					sb.WriteString("\n")
				}
			}
		}
	}

	return strings.TrimSpace(sb.String())
}

// extractDebugHypotheses safely extracts hypotheses from interface{}.
// Handles both direct []DebugHypothesis and []interface{} from JSON.
func extractDebugHypotheses(raw interface{}) []agentimpl.DebugHypothesis {
	// Direct type match (from agent output before serialization)
	if typed, ok := raw.([]agentimpl.DebugHypothesis); ok {
		return typed
	}
	// Handle []interface{} from JSON unmarshaling
	if arr, ok := raw.([]interface{}); ok {
		result := make([]agentimpl.DebugHypothesis, 0, len(arr))
		for _, item := range arr {
			if m, ok := item.(map[string]interface{}); ok {
				h := agentimpl.DebugHypothesis{
					Cause:      getStringField(m, "cause"),
					Likelihood: getStringField(m, "likelihood"),
					Reasoning:  getStringField(m, "reasoning"),
				}
				if locs, ok := m["code_locations"].([]interface{}); ok {
					for _, loc := range locs {
						if s, ok := loc.(string); ok {
							h.CodeLocations = append(h.CodeLocations, s)
						}
					}
				}
				result = append(result, h)
			}
		}
		return result
	}
	return nil
}

// extractDebugSteps safely extracts investigation steps from interface{}.
func extractDebugSteps(raw interface{}) []agentimpl.DebugInvestigationStep {
	if typed, ok := raw.([]agentimpl.DebugInvestigationStep); ok {
		return typed
	}
	if arr, ok := raw.([]interface{}); ok {
		result := make([]agentimpl.DebugInvestigationStep, 0, len(arr))
		for _, item := range arr {
			if m, ok := item.(map[string]interface{}); ok {
				s := agentimpl.DebugInvestigationStep{
					Step:            getIntField(m, "step"),
					Action:          getStringField(m, "action"),
					Command:         getStringField(m, "command"),
					ExpectedFinding: getStringField(m, "expected_finding"),
				}
				result = append(result, s)
			}
		}
		return result
	}
	return nil
}

// extractDebugFixes safely extracts quick fixes from interface{}.
func extractDebugFixes(raw interface{}) []agentimpl.DebugQuickFix {
	if typed, ok := raw.([]agentimpl.DebugQuickFix); ok {
		return typed
	}
	if arr, ok := raw.([]interface{}); ok {
		result := make([]agentimpl.DebugQuickFix, 0, len(arr))
		for _, item := range arr {
			if m, ok := item.(map[string]interface{}); ok {
				f := agentimpl.DebugQuickFix{
					Fix:  getStringField(m, "fix"),
					When: getStringField(m, "when"),
				}
				result = append(result, f)
			}
		}
		return result
	}
	return nil
}

// getStringField safely extracts a string from a map.
func getStringField(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

// getIntField safely extracts an int from a map (handles float64 from JSON).
func getIntField(m map[string]interface{}, key string) int {
	if v, ok := m[key].(float64); ok {
		return int(v)
	}
	if v, ok := m[key].(int); ok {
		return v
	}
	return 0
}

// FormatSimplifyResult formats the output from the SimplifyAgent.
func FormatSimplifyResult(findings []agentcore.Finding) string {
	if len(findings) == 0 {
		return "No simplification results."
	}

	var sb strings.Builder
	for _, f := range findings {
		sb.WriteString("## Code Simplification\n\n")
		sb.WriteString(f.Description)
		sb.WriteString("\n\n")

		if meta, ok := f.Metadata["simplified_code"].(string); ok && meta != "" {
			sb.WriteString("### Simplified Code\n")
			sb.WriteString("```\n")
			sb.WriteString(meta)
			sb.WriteString("\n```\n\n")
		}

		// Extract line counts - handle both int and float64 from JSON
		orig := getIntFromMetadata(f.Metadata, "original_lines")
		simp := getIntFromMetadata(f.Metadata, "simplified_lines")
		reduction := getIntFromMetadata(f.Metadata, "reduction_percentage")
		if orig > 0 && simp > 0 {
			sb.WriteString(fmt.Sprintf("**Lines**: %d → %d (-%d%%)\n", orig, simp, reduction))
		}

		if risk, ok := f.Metadata["risk_assessment"].(string); ok && risk != "" {
			sb.WriteString(fmt.Sprintf("**Risk**: %s\n\n", risk))
		}

		// Extract changes - handle both direct type and []interface{} from JSON
		if changesRaw, ok := f.Metadata["changes"]; ok {
			changes := extractSimplifyChanges(changesRaw)
			if len(changes) > 0 {
				sb.WriteString("### Changes Made\n")
				for _, c := range changes {
					sb.WriteString(fmt.Sprintf("- **%s**: %s", c.What, c.Why))
					if c.Risk != "none" && c.Risk != "" {
						sb.WriteString(fmt.Sprintf(" (risk: %s)", c.Risk))
					}
					sb.WriteString("\n")
				}
			}
		}
	}

	return strings.TrimSpace(sb.String())
}

// extractSimplifyChanges safely extracts changes from interface{}.
// Handles both direct []SimplifyChange and []interface{} from JSON.
func extractSimplifyChanges(raw interface{}) []agentimpl.SimplifyChange {
	// Direct type match (from agent output before serialization)
	if typed, ok := raw.([]agentimpl.SimplifyChange); ok {
		return typed
	}
	// Handle []interface{} from JSON unmarshaling
	if arr, ok := raw.([]interface{}); ok {
		result := make([]agentimpl.SimplifyChange, 0, len(arr))
		for _, item := range arr {
			if m, ok := item.(map[string]interface{}); ok {
				c := agentimpl.SimplifyChange{
					What: getStringField(m, "what"),
					Why:  getStringField(m, "why"),
					Risk: getStringField(m, "risk"),
				}
				result = append(result, c)
			}
		}
		return result
	}
	return nil
}

// getIntFromMetadata safely extracts an int from metadata map.
// Handles both int and float64 (from JSON unmarshaling).
func getIntFromMetadata(m map[string]any, key string) int {
	if m == nil {
		return 0
	}
	if v, ok := m[key].(float64); ok {
		return int(v)
	}
	if v, ok := m[key].(int); ok {
		return v
	}
	return 0
}

// === Policy Formatters ===

// FormatPolicyCheckResult formats policy check results into Markdown.
func FormatPolicyCheckResult(decision *policy.PolicyDecision, files []string) string {
	if decision == nil {
		return FormatError("No policy decision available.")
	}

	var sb strings.Builder

	// Status header
	if decision.IsAllowed() {
		sb.WriteString("## ✅ Policy Check Passed\n\n")
	} else {
		sb.WriteString("## ❌ Policy Violations Detected\n\n")
	}

	// Files checked
	sb.WriteString(fmt.Sprintf("**Files checked**: %d\n", len(files)))
	sb.WriteString(fmt.Sprintf("**Decision ID**: `%s`\n\n", decision.DecisionID))

	// List files
	if len(files) > 0 {
		sb.WriteString("### Files\n")
		for _, f := range files {
			sb.WriteString(fmt.Sprintf("- %s\n", f))
		}
		sb.WriteString("\n")
	}

	// Violations
	if len(decision.Violations) > 0 {
		sb.WriteString("### Violations\n")
		for _, v := range decision.Violations {
			sb.WriteString(fmt.Sprintf("- %s\n", v))
		}
	}

	return strings.TrimSpace(sb.String())
}

// FormatPolicyList formats a list of policies into Markdown.
func FormatPolicyList(policies []*policy.PolicyFile, policiesDir string) string {
	var sb strings.Builder

	sb.WriteString("## OPA Policies\n\n")
	sb.WriteString(fmt.Sprintf("**Directory**: `%s`\n", policiesDir))
	sb.WriteString(fmt.Sprintf("**Count**: %d policy file(s)\n\n", len(policies)))

	if len(policies) == 0 {
		sb.WriteString("No policies loaded.\n")
		sb.WriteString("Run `tw policy init` to create the default policy.\n")
		return sb.String()
	}

	sb.WriteString("### Loaded Policies\n")
	for _, p := range policies {
		sb.WriteString(fmt.Sprintf("- **%s** (`%s`)\n", p.Name, p.Path))
	}

	return strings.TrimSpace(sb.String())
}

// FormatPolicyExplain formats a single policy explanation into Markdown.
func FormatPolicyExplain(p *policy.PolicyFile) string {
	if p == nil {
		return FormatError("No policy available.")
	}

	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("## Policy: %s\n\n", p.Name))
	sb.WriteString(fmt.Sprintf("**Path**: `%s`\n\n", p.Path))
	sb.WriteString("### Source\n")
	sb.WriteString("```rego\n")
	sb.WriteString(p.Content)
	sb.WriteString("\n```\n")

	return strings.TrimSpace(sb.String())
}

// FormatPoliciesExplain formats multiple policy explanations into Markdown.
func FormatPoliciesExplain(policies []*policy.PolicyFile) string {
	if len(policies) == 0 {
		return "No policies loaded.\nRun `tw policy init` to create the default policy."
	}

	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("## OPA Policies (%d)\n\n", len(policies)))

	for i, p := range policies {
		if i > 0 {
			sb.WriteString("\n---\n\n")
		}
		sb.WriteString(fmt.Sprintf("### %s\n\n", p.Name))
		sb.WriteString(fmt.Sprintf("**Path**: `%s`\n\n", p.Path))
		sb.WriteString("```rego\n")
		// Limit to first 50 lines for token efficiency
		lines := strings.Split(p.Content, "\n")
		if len(lines) > 50 {
			sb.WriteString(strings.Join(lines[:50], "\n"))
			sb.WriteString(fmt.Sprintf("\n// ...%d more lines\n", len(lines)-50))
		} else {
			sb.WriteString(p.Content)
		}
		sb.WriteString("\n```\n")
	}

	return strings.TrimSpace(sb.String())
}
