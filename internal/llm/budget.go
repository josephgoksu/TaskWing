// Context budget computation derived from model capacity.
// Replaces hardcoded char/node limits with proportional allocations.
package llm

// ContextBudgets holds all context limits derived from a model's capacity.
// All char limits assume ~4 chars per token.
type ContextBudgets struct {
	ModelID        string
	MaxInputTokens int

	// Planning and task context
	GoalSummaryChars    int // Max chars for user's planning goal
	TaskMaxNodes        int // Max knowledge nodes per task enrichment
	ConstraintChars     int // Max chars per constraint in compact context
	RelevantNodeChars   int // Max chars per relevant node in compact context
	DefaultMaxNodes     int // Max nodes for planning context retrieval
	NodesPerQuery       int // Max results per search query
	ArchitectureMDChars int // Max chars for ARCHITECTURE.md in first task

	// Agent context
	WaveDescChars    int // Max chars per description in wave context
	WaveSummaryChars int // Total wave context budget
	DocFileChars     int // Max chars per documentation file for agents
}

// ComputeBudgets derives context limits from a model's context window.
// Uses the 40% rule: allocate at most 40% of context to gathered content,
// leaving 60% for the prompt template, system message, and response.
func ComputeBudgets(modelID string) ContextBudgets {
	maxTokens := GetMaxInputTokens(modelID)
	contentBudget := maxTokens * 40 / 100 // 40% of context for content
	charsPerToken := 4

	b := ContextBudgets{
		ModelID:        modelID,
		MaxInputTokens: maxTokens,
	}

	// Scale based on content budget in chars
	budgetChars := contentBudget * charsPerToken

	switch {
	case maxTokens >= 200_000: // Claude Opus/Sonnet, GPT-4o, Gemini Pro
		b.GoalSummaryChars = 1000
		b.TaskMaxNodes = 30
		b.ConstraintChars = 800
		b.RelevantNodeChars = 1200
		b.DefaultMaxNodes = 40
		b.NodesPerQuery = 8
		b.ArchitectureMDChars = 16000
		b.WaveDescChars = 600
		b.WaveSummaryChars = min(budgetChars/4, 24000)
		b.DocFileChars = 8000

	case maxTokens >= 100_000: // Mid-range models
		b.GoalSummaryChars = 800
		b.TaskMaxNodes = 25
		b.ConstraintChars = 600
		b.RelevantNodeChars = 1000
		b.DefaultMaxNodes = 30
		b.NodesPerQuery = 6
		b.ArchitectureMDChars = 12000
		b.WaveDescChars = 500
		b.WaveSummaryChars = min(budgetChars/4, 18000)
		b.DocFileChars = 6000

	case maxTokens >= 32_000: // GPT-4o-mini, smaller models
		b.GoalSummaryChars = 500
		b.TaskMaxNodes = 20
		b.ConstraintChars = 500
		b.RelevantNodeChars = 800
		b.DefaultMaxNodes = 25
		b.NodesPerQuery = 5
		b.ArchitectureMDChars = 8000
		b.WaveDescChars = 400
		b.WaveSummaryChars = min(budgetChars/4, 12000)
		b.DocFileChars = 5000

	default: // Small/local models (8K-32K)
		b.GoalSummaryChars = 300
		b.TaskMaxNodes = 10
		b.ConstraintChars = 300
		b.RelevantNodeChars = 500
		b.DefaultMaxNodes = 15
		b.NodesPerQuery = 3
		b.ArchitectureMDChars = 4000
		b.WaveDescChars = 200
		b.WaveSummaryChars = min(budgetChars/4, 6000)
		b.DocFileChars = 3000
	}

	return b
}
