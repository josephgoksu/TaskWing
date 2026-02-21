package task

import (
	"context"
	"fmt"
	"strings"
)

// AskSearchFunc is the signature for a context/ask search function.
// This breaks the import cycle by avoiding direct dependency on knowledge.Service.
type AskSearchFunc func(ctx context.Context, query string, limit int) ([]AskResult, error)

// AskResult is a minimal struct for context search results.
type AskResult struct {
	Summary string
	Type    string
	Content string
}

// FormatRichContext builds a rich Markdown context string for a task.
// This is used by both CLI hooks and MCP tools to ensure consistent presentation.
//
// Context Binding Strategy (see docs/architecture/ADR_CONTEXT_BINDING.md):
// - Early binding: Uses Task.ContextSummary if available (populated at creation)
// - Late binding: Falls back to searchFn if ContextSummary is empty (backward compatibility)
func FormatRichContext(ctx context.Context, t *Task, p *Plan, searchFn AskSearchFunc) string {
	var askContext string

	// Early binding: Use pre-computed ContextSummary if available
	if t.ContextSummary != "" {
		askContext = "\n" + t.ContextSummary
	} else if len(t.SuggestedAskQueries) > 0 && searchFn != nil {
		// Late binding fallback: Fetch context dynamically using ALL queries
		var allResults []AskResult
		for _, query := range t.SuggestedAskQueries {
			results, err := searchFn(ctx, query, 3)
			if err == nil {
				allResults = append(allResults, results...)
			}
		}

		if len(allResults) > 0 {
			var sb strings.Builder
			sb.WriteString("\n## Relevant Architecture Context\n")
			seen := make(map[string]bool) // Dedupe by summary
			for _, r := range allResults {
				if seen[r.Summary] {
					continue
				}
				seen[r.Summary] = true

				// Truncate content for display (consistent with early binding at 300 chars)
				content := r.Content
				if len(content) > 300 {
					content = content[:297] + "..."
				}
				sb.WriteString(fmt.Sprintf("- **%s** (%s): %s\n", r.Summary, r.Type, content))
			}
			askContext = sb.String()
		}
	}

	// Calculate progress
	progress := 0
	completed := 0
	for _, pt := range p.Tasks {
		if pt.Status == StatusCompleted {
			completed++
		}
	}
	if len(p.Tasks) > 0 {
		progress = completed * 100 / len(p.Tasks)
	}

	contextStr := fmt.Sprintf(`
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
🔄 CONTINUING TO NEXT TASK
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Plan Progress: %d%% (%d/%d tasks completed)

## Next Task: %s
**ID**: %s
**Priority**: %d
**Scope**: %s

### Description
%s

### Acceptance Criteria
`, progress, completed, len(p.Tasks), t.Title, t.ID, t.Priority, t.Scope, t.Description)

	for _, ac := range t.AcceptanceCriteria {
		contextStr += fmt.Sprintf("- [ ] %s\n", ac)
	}

	// Render validation steps if present
	if len(t.ValidationSteps) > 0 {
		contextStr += "\n### Validation Steps\n"
		for _, vs := range t.ValidationSteps {
			contextStr += fmt.Sprintf("- [ ] %s\n", vs)
		}
	}

	if askContext != "" {
		contextStr += askContext
	}

	contextStr += `
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

**Instructions**:
1. First, call MCP tool ` + "`task`" + ` with action=` + "`start`" + `, task_id, and session_id
2. Implement the task following the patterns above
3. When complete, call MCP tool ` + "`task`" + ` with action=` + "`complete`" + `, summary, and files_modified
4. The Stop hook will automatically check for the next task

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
`

	return contextStr
}
