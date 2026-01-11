package presenter

import (
	"strings"
	"testing"

	"github.com/josephgoksu/TaskWing/internal/app"
	"github.com/josephgoksu/TaskWing/internal/codeintel"
	"github.com/josephgoksu/TaskWing/internal/knowledge"
	"github.com/josephgoksu/TaskWing/internal/task"
	"github.com/stretchr/testify/assert"
)

// === FormatRecall Tests ===

func TestFormatRecall_NilResult(t *testing.T) {
	result := FormatRecall(nil)
	assert.Equal(t, "No results found.", result)
}

func TestFormatRecall_EmptyResult(t *testing.T) {
	result := FormatRecall(&app.RecallResult{})
	assert.Equal(t, "No results found.", result)
}

func TestFormatRecall_WithAnswer(t *testing.T) {
	input := &app.RecallResult{
		Answer: "SQLite is used for local storage.",
		Results: []knowledge.NodeResponse{
			{Summary: "SQLite Storage", Type: "decision", Content: "SQLite Storage\nWe use SQLite for persistence."},
		},
	}
	result := FormatRecall(input)

	assert.Contains(t, result, "## Answer")
	assert.Contains(t, result, "SQLite is used for local storage.")
	assert.Contains(t, result, "## Knowledge")
	assert.Contains(t, result, "**SQLite Storage** (decision)")
}

func TestFormatRecall_WithSymbols(t *testing.T) {
	input := &app.RecallResult{
		Symbols: []app.SymbolResponse{
			{Name: "Repository", Kind: "struct", Location: "internal/memory/sqlite.go:45"},
			{Name: "NewStore", Kind: "function", Location: "internal/memory/sqlite.go:100"},
		},
	}
	result := FormatRecall(input)

	assert.Contains(t, result, "## Code Symbols")
	assert.Contains(t, result, "`Repository` (struct)")
	assert.Contains(t, result, "`NewStore` (function)")
}

func TestFormatRecall_KnowledgeContentPreview(t *testing.T) {
	input := &app.RecallResult{
		Results: []knowledge.NodeResponse{
			{
				Summary: "Important Decision",
				Type:    "decision",
				Content: "Important Decision\nThis is the detailed explanation of why we made this decision and what trade-offs were considered.",
			},
		},
	}
	result := FormatRecall(input)

	// Should include content preview without the summary prefix
	assert.Contains(t, result, "This is the detailed explanation")
	// Should not duplicate summary in content
	assert.Equal(t, 1, strings.Count(result, "Important Decision"))
}

// === FormatTask Tests ===

func TestFormatTask_NilResult(t *testing.T) {
	result := FormatTask(nil)
	assert.Equal(t, "No task information.", result)
}

func TestFormatTask_WithTask(t *testing.T) {
	input := &app.TaskResult{
		Success: true,
		Message: "Task started successfully.",
		Task: &task.Task{
			ID:          "task-123",
			Title:       "Implement feature X",
			Description: "Add the new feature to the system.",
			Status:      task.StatusInProgress,
			Priority:    80,
			AcceptanceCriteria: []string{
				"Feature is implemented",
				"Tests pass",
			},
			ValidationSteps: []string{
				"go test ./...",
				"go build .",
			},
		},
		Hint: "Use the recall tool to fetch context.",
	}
	result := FormatTask(input)

	assert.Contains(t, result, "Task started successfully.")
	assert.Contains(t, result, "Implement feature X")
	assert.Contains(t, result, "`task-123`")
	assert.Contains(t, result, "Priority**: 80")
	assert.Contains(t, result, "### Acceptance Criteria")
	assert.Contains(t, result, "- [ ] Feature is implemented")
	assert.Contains(t, result, "### Validation")
	assert.Contains(t, result, "go test ./...")
	assert.Contains(t, result, "> **Hint**: Use the recall tool")
}

func TestFormatTask_CompletedTask(t *testing.T) {
	input := &app.TaskResult{
		Task: &task.Task{
			ID:     "task-456",
			Title:  "Completed task",
			Status: task.StatusCompleted,
			AcceptanceCriteria: []string{
				"Criteria met",
			},
		},
	}
	result := FormatTask(input)

	// Completed tasks should have checked boxes
	assert.Contains(t, result, "- [x] Criteria met")
	assert.Contains(t, result, "completed")
}

// === FormatPlan Tests ===

func TestFormatPlan_NilPlan(t *testing.T) {
	result := FormatPlan(nil)
	assert.Equal(t, "No plan information.", result)
}

func TestFormatPlan_WithTasks(t *testing.T) {
	input := &task.Plan{
		ID:     "plan-abc",
		Goal:   "Implement MCP optimization",
		Status: task.PlanStatusActive,
		Tasks: []task.Task{
			{Title: "Task 1", Status: task.StatusCompleted, Priority: 90},
			{Title: "Task 2", Status: task.StatusInProgress, Priority: 80},
			{Title: "Task 3", Status: task.StatusPending, Priority: 70},
		},
	}
	result := FormatPlan(input)

	assert.Contains(t, result, "## Plan: Implement MCP optimization")
	assert.Contains(t, result, "`plan-abc`")
	assert.Contains(t, result, "### Tasks")
	assert.Contains(t, result, "- [x] Task 1") // Completed
	assert.Contains(t, result, "- [~] Task 2") // In progress
	assert.Contains(t, result, "- [ ] Task 3") // Pending
	assert.Contains(t, result, "**Progress**: 1/3 tasks completed")
}

// === FormatSymbolList Tests ===

func TestFormatSymbolList_Empty(t *testing.T) {
	result := FormatSymbolList(nil)
	assert.Equal(t, "No symbols found.", result)

	result = FormatSymbolList([]codeintel.Symbol{})
	assert.Equal(t, "No symbols found.", result)
}

func TestFormatSymbolList_WithSymbols(t *testing.T) {
	input := []codeintel.Symbol{
		{
			Name:      "HandleRequest",
			Kind:      codeintel.SymbolFunction,
			FilePath:  "cmd/server.go",
			StartLine: 42,
			Signature: "func(ctx context.Context, req Request) Response",
		},
		{
			Name:       "privateHelper",
			Kind:       codeintel.SymbolFunction,
			FilePath:   "internal/utils.go",
			StartLine:  15,
			Visibility: "private",
		},
	}
	result := FormatSymbolList(input)

	assert.Contains(t, result, "## Symbols")
	assert.Contains(t, result, "`HandleRequest`")
	assert.Contains(t, result, "cmd/server.go:42")
	assert.Contains(t, result, "`func(ctx context.Context")
	assert.Contains(t, result, "(private)")
}

// === FormatSearchResults Tests ===

func TestFormatSearchResults_Empty(t *testing.T) {
	result := FormatSearchResults(nil)
	assert.Equal(t, "No matching symbols found.", result)
}

func TestFormatSearchResults_WithResults(t *testing.T) {
	input := []codeintel.SymbolSearchResult{
		{
			Symbol: codeintel.Symbol{
				Name:       "SearchCode",
				Kind:       codeintel.SymbolFunction,
				FilePath:   "internal/search.go",
				StartLine:  100,
				DocComment: "SearchCode performs semantic code search.",
			},
			Score: 0.85,
		},
	}
	result := FormatSearchResults(input)

	assert.Contains(t, result, "## Search Results")
	assert.Contains(t, result, "`SearchCode`")
	assert.Contains(t, result, "0.85")
	assert.Contains(t, result, "SearchCode performs semantic")
}

// === FormatCallers Tests ===

func TestFormatCallers_Nil(t *testing.T) {
	result := FormatCallers(nil)
	assert.Equal(t, "Failed to get callers.", result)
}

func TestFormatCallers_WithRelationships(t *testing.T) {
	input := &app.GetCallersResult{
		Success: true,
		Symbol: &codeintel.Symbol{
			Name:      "ProcessData",
			Kind:      codeintel.SymbolFunction,
			FilePath:  "internal/processor.go",
			StartLine: 50,
		},
		Callers: []codeintel.Symbol{
			{Name: "HandleRequest", FilePath: "cmd/handler.go", StartLine: 30},
		},
		Callees: []codeintel.Symbol{
			{Name: "ValidateInput", FilePath: "internal/validator.go", StartLine: 10},
		},
	}
	result := FormatCallers(input)

	assert.Contains(t, result, "`ProcessData`")
	assert.Contains(t, result, "### Called By")
	assert.Contains(t, result, "`HandleRequest`")
	assert.Contains(t, result, "### Calls")
	assert.Contains(t, result, "`ValidateInput`")
}

// === FormatImpact Tests ===

func TestFormatImpact_Nil(t *testing.T) {
	result := FormatImpact(nil)
	assert.Equal(t, "Failed to analyze impact.", result)
}

func TestFormatImpact_WithResults(t *testing.T) {
	input := &app.AnalyzeImpactResult{
		Success: true,
		Source: &codeintel.Symbol{
			Name:      "CoreFunction",
			FilePath:  "internal/core.go",
			StartLine: 25,
		},
		AffectedCount: 5,
		AffectedFiles: 3,
		MaxDepth:      2,
		ByDepth: map[int][]codeintel.Symbol{
			1: {{Name: "DirectCaller", FilePath: "a.go", StartLine: 10}},
			2: {{Name: "IndirectCaller", FilePath: "b.go", StartLine: 20}},
		},
	}
	result := FormatImpact(input)

	assert.Contains(t, result, "## Impact Analysis: `CoreFunction`")
	assert.Contains(t, result, "**Affected**: 5 symbols across 3 files")
	assert.Contains(t, result, "### Blast Radius")
	assert.Contains(t, result, "**Depth 1**")
	assert.Contains(t, result, "`DirectCaller`")
	assert.Contains(t, result, "**Depth 2**")
	assert.Contains(t, result, "`IndirectCaller`")
}

// === FormatRemember Tests ===

func TestFormatRemember_Nil(t *testing.T) {
	result := FormatRemember(nil)
	assert.Contains(t, result, "## ❌ Error")
	assert.Contains(t, result, "Failed to add knowledge")
}

func TestFormatRemember_Success(t *testing.T) {
	input := &app.AddResult{
		ID:           "n-abc123",
		Type:         "decision",
		Summary:      "Use SQLite for storage",
		HasEmbedding: true,
	}
	result := FormatRemember(input)

	assert.Contains(t, result, "## ✅ Knowledge Saved")
	assert.Contains(t, result, "**ID**: `n-abc123`")
	assert.Contains(t, result, "**Type**: decision")
	assert.Contains(t, result, "**Summary**: Use SQLite for storage")
	assert.Contains(t, result, "Embedding generated")
}

func TestFormatRemember_EmptyID(t *testing.T) {
	input := &app.AddResult{
		ID: "",
	}
	result := FormatRemember(input)
	assert.Contains(t, result, "## ❌ Error")
}

// === FormatError Tests ===

func TestFormatError(t *testing.T) {
	result := FormatError("Something went wrong")
	assert.Contains(t, result, "## ❌ Error")
	assert.Contains(t, result, "**Details**: Something went wrong")
}

func TestFormatValidationError(t *testing.T) {
	result := FormatValidationError("query", "query is required")
	assert.Contains(t, result, "## ❌ Validation Error")
	assert.Contains(t, result, "**Field**: `query`")
	assert.Contains(t, result, "**Details**: query is required")
}

// === FormatSummary Tests ===

func TestFormatSummary_Nil(t *testing.T) {
	result := FormatSummary(nil)
	assert.Equal(t, "No project summary available.", result)
}

func TestFormatSummary_WithTypes(t *testing.T) {
	input := &knowledge.ProjectSummary{
		Total: 10,
		Types: map[string]knowledge.TypeSummary{
			"decision": {Count: 3, Examples: []string{"Use SQLite", "API Design"}},
			"pattern":  {Count: 2, Examples: []string{"Repository Pattern"}},
		},
	}
	result := FormatSummary(input)

	assert.Contains(t, result, "## Knowledge Base: 10 nodes")
	assert.Contains(t, result, "### 📋 Decisions (3)")
	assert.Contains(t, result, "### 🧩 Patterns (2)")
	assert.Contains(t, result, "- Use SQLite")
}

// === FormatClarifyResult Tests ===

func TestFormatClarifyResult_Nil(t *testing.T) {
	result := FormatClarifyResult(nil)
	assert.Contains(t, result, "## ❌ Error")
}

func TestFormatClarifyResult_NotReady(t *testing.T) {
	input := &app.ClarifyResult{
		Success:       true,
		GoalSummary:   "Implement authentication",
		Questions:     []string{"OAuth or JWT?", "Session storage?"},
		IsReadyToPlan: false,
	}
	result := FormatClarifyResult(input)

	assert.Contains(t, result, "## 🔍 Clarification Needed")
	assert.Contains(t, result, "**Goal**: Implement authentication")
	assert.Contains(t, result, "### Questions")
	assert.Contains(t, result, "1. OAuth or JWT?")
	assert.Contains(t, result, "2. Session storage?")
}

func TestFormatClarifyResult_Ready(t *testing.T) {
	input := &app.ClarifyResult{
		Success:       true,
		GoalSummary:   "Implement JWT auth",
		EnrichedGoal:  "Implement JWT authentication with refresh tokens...",
		IsReadyToPlan: true,
	}
	result := FormatClarifyResult(input)

	assert.Contains(t, result, "## ✅ Ready to Generate Plan")
	assert.Contains(t, result, "### Enriched Specification")
	assert.Contains(t, result, "JWT authentication with refresh tokens")
	assert.Contains(t, result, "Call `plan_generate`")
}

// === FormatGenerateResult Tests ===

func TestFormatGenerateResult_Nil(t *testing.T) {
	result := FormatGenerateResult(nil)
	assert.Contains(t, result, "## ❌ Error")
}

func TestFormatGenerateResult_Success(t *testing.T) {
	input := &app.GenerateResult{
		Success: true,
		PlanID:  "plan-123",
		Goal:    "Add user authentication",
		Tasks: []task.Task{
			{Title: "Create user model", Priority: 90},
			{Title: "Add login endpoint", Priority: 80},
		},
		Hint: "Use task_next to begin.",
	}
	result := FormatGenerateResult(input)

	assert.Contains(t, result, "## ✅ Plan Generated")
	assert.Contains(t, result, "**Plan ID**: `plan-123`")
	assert.Contains(t, result, "1. **Create user model** (P90)")
	assert.Contains(t, result, "2. **Add login endpoint** (P80)")
	assert.Contains(t, result, "> **Hint**: Use task_next to begin.")
}

// === FormatAuditResult Tests ===

func TestFormatAuditResult_Nil(t *testing.T) {
	result := FormatAuditResult(nil)
	assert.Contains(t, result, "## ❌ Error")
}

func TestFormatAuditResult_Verified(t *testing.T) {
	input := &app.AuditResult{
		Success:     true,
		PlanID:      "plan-456",
		Status:      "verified",
		BuildPassed: true,
		TestsPassed: true,
		RetryCount:  1,
		Message:     "All checks passed.",
		Hint:        "Create a PR.",
	}
	result := FormatAuditResult(input)

	assert.Contains(t, result, "## ✅ Audit: Verified")
	assert.Contains(t, result, "**Plan ID**: `plan-456`")
	assert.Contains(t, result, "- ✅ Build")
	assert.Contains(t, result, "- ✅ Tests")
	assert.Contains(t, result, "> **Hint**: Create a PR.")
}

func TestFormatAuditResult_NeedsRevision(t *testing.T) {
	input := &app.AuditResult{
		Success:        true,
		PlanID:         "plan-789",
		Status:         "needs_revision",
		BuildPassed:    true,
		TestsPassed:    false,
		SemanticIssues: []string{"Missing error handling", "Unused variable"},
		RetryCount:     3,
	}
	result := FormatAuditResult(input)

	assert.Contains(t, result, "## ⚠️ Audit: Needs_revision")
	assert.Contains(t, result, "- ✅ Build")
	assert.Contains(t, result, "- ❌ Tests")
	assert.Contains(t, result, "### Semantic Issues")
	assert.Contains(t, result, "- Missing error handling")
}

// === Helper Function Tests ===

func TestTruncate(t *testing.T) {
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"short", 10, "short"},
		{"exactly10!", 10, "exactly10!"},
		{"this is a longer string", 10, "this is a ..."},
		{"", 10, ""},
		{"unicode: ", 5, "unico..."},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := truncate(tt.input, tt.maxLen)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCleanContent(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		summary  string
		expected string
	}{
		{
			name:     "removes summary prefix",
			content:  "My Title\nThe actual content here.",
			summary:  "My Title",
			expected: "The actual content here.",
		},
		{
			name:     "short summary unchanged",
			content:  "AB content",
			summary:  "AB",
			expected: "AB content",
		},
		{
			name:     "no match returns content",
			content:  "Different content",
			summary:  "My Title",
			expected: "Different content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cleanContent(tt.content, tt.summary)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestScoreToBar(t *testing.T) {
	tests := []struct {
		score    float32
		expected string
	}{
		{0.0, "░░░░░"},
		{0.1, "█░░░░"},
		{0.5, "██░░░"},
		{0.8, "████░"},
		{1.0, "█████"},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			result := scoreToBar(tt.score)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestStatusIcon(t *testing.T) {
	assert.Equal(t, "⏳", statusIcon(task.StatusPending))
	assert.Equal(t, "🔄", statusIcon(task.StatusInProgress))
	assert.Equal(t, "✅", statusIcon(task.StatusCompleted))
	assert.Equal(t, "❌", statusIcon(task.StatusFailed))
}
