// Token estimation utilities for LLM context management.
package llm

// EstimateTokens provides a heuristic-based token count estimate for text.
// Uses the industry standard approximation of ~4 characters per token.
// This is intentionally simple and dependency-free for fast, predictable behavior.
func EstimateTokens(text string) int {
	if len(text) == 0 {
		return 0
	}
	// Standard heuristic: 1 token ≈ 4 characters
	return (len(text) + 3) / 4 // Round up to be conservative
}

// EstimateBudgetChars converts a token budget to approximate character limit.
// Use this when you want to enforce a character-based limit from a token budget.
func EstimateBudgetChars(tokens int) int {
	return tokens * 4
}
