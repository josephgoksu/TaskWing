package task

import (
	"context"
	"fmt"
)

// RecallSearchFunc is the signature for a context/recall search function.
// This breaks the import cycle by avoiding direct dependency on knowledge.Service.
type RecallSearchFunc func(ctx context.Context, query string, limit int) ([]RecallResult, error)

// RecallResult is a minimal struct for context search results.
type RecallResult struct {
	Summary string
	Type    string
	Content string
}

// FormatRichContext builds a rich Markdown context string for a task.
// This is used by both CLI hooks and MCP tools to ensure consistent presentation.
// searchFn may be nil if no recall context is needed.
func FormatRichContext(ctx context.Context, t *Task, p *Plan, searchFn RecallSearchFunc) string {
	var recallContext string
	if len(t.SuggestedRecallQueries) > 0 && searchFn != nil {
		results, err := searchFn(ctx, t.SuggestedRecallQueries[0], 3)
		if err == nil && len(results) > 0 {
			recallContext = "\n## Relevant Architecture Context\n"
			for _, r := range results {
				// Truncate content for display
				content := r.Content
				if len(content) > 100 {
					content = content[:97] + "..."
				}
				recallContext += fmt.Sprintf("- **%s** (%s): %s\n", r.Summary, r.Type, content)
			}
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

	if recallContext != "" {
		contextStr += recallContext
	}

	contextStr += `
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

**Instructions**:
1. First, call task_start MCP tool with task_id and session_id
2. Implement the task following the patterns above
3. When complete, call task_complete MCP tool with summary and files_modified
4. The Stop hook will automatically check for the next task

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
`

	return contextStr
}
