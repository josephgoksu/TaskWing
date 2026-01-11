package tokens

import "testing"

func TestEstimateTokens(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{"empty string", "", 0},
		{"single char", "a", 1},           // 1/4 rounded up = 1
		{"four chars", "abcd", 1},         // 4/4 = 1
		{"five chars", "abcde", 2},        // 5/4 rounded up = 2
		{"eight chars", "abcdefgh", 2},    // 8/4 = 2
		{"1000 chars", string(make([]byte, 1000)), 250}, // 1000/4 = 250
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EstimateTokens(tt.input)
			if got != tt.expected {
				t.Errorf("EstimateTokens(%q) = %d, want %d", tt.input, got, tt.expected)
			}
		})
	}
}

func TestEstimateBudgetChars(t *testing.T) {
	tests := []struct {
		tokens   int
		expected int
	}{
		{0, 0},
		{1, 4},
		{100, 400},
		{250000, 1000000}, // 250k tokens = 1M chars
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := EstimateBudgetChars(tt.tokens)
			if got != tt.expected {
				t.Errorf("EstimateBudgetChars(%d) = %d, want %d", tt.tokens, got, tt.expected)
			}
		})
	}
}
