package tools

import "testing"

func TestNewContextBudget(t *testing.T) {
	budget := NewContextBudget(10000)
	if budget.Total() != 10000 {
		t.Errorf("Expected total budget 10000, got %d", budget.Total())
	}
	if budget.Used() != 0 {
		t.Errorf("Expected used tokens 0, got %d", budget.Used())
	}
}

func TestNewSafeContextBudget_UnderLimit(t *testing.T) {
	// Request less than MaxSafeContextBudget - should get requested amount
	budget := NewSafeContextBudget(50000)
	if budget.Total() != 50000 {
		t.Errorf("Expected total budget 50000, got %d", budget.Total())
	}
}

func TestNewSafeContextBudget_OverLimit(t *testing.T) {
	// Request more than MaxSafeContextBudget - should be capped
	budget := NewSafeContextBudget(500000) // 500k tokens
	if budget.Total() != MaxSafeContextBudget {
		t.Errorf("Expected total budget %d (MaxSafeContextBudget), got %d", MaxSafeContextBudget, budget.Total())
	}
}

func TestNewSafeContextBudget_GeminiScenario(t *testing.T) {
	// Simulate Gemini's 1M token limit with 50% budget
	geminiLimit := 1_000_000
	requestedBudget := int(float64(geminiLimit) * 0.5) // 500k tokens

	budget := NewSafeContextBudget(requestedBudget)

	// Should be capped at MaxSafeContextBudget
	if budget.Total() != MaxSafeContextBudget {
		t.Errorf("Gemini scenario: expected budget capped at %d, got %d", MaxSafeContextBudget, budget.Total())
	}
}

func TestContextBudget_Reserve(t *testing.T) {
	budget := NewContextBudget(100)

	// Should succeed
	if err := budget.Reserve(50); err != nil {
		t.Errorf("Reserve 50 should succeed: %v", err)
	}
	if budget.Used() != 50 {
		t.Errorf("Expected used 50, got %d", budget.Used())
	}

	// Should succeed
	if err := budget.Reserve(50); err != nil {
		t.Errorf("Reserve another 50 should succeed: %v", err)
	}

	// Should fail - budget exhausted
	if err := budget.Reserve(1); err != ErrBudgetExceeded {
		t.Errorf("Reserve when exhausted should return ErrBudgetExceeded, got: %v", err)
	}
}

func TestContextBudget_TryReserve(t *testing.T) {
	budget := NewContextBudget(100)

	if !budget.TryReserve(100) {
		t.Error("TryReserve 100 should succeed")
	}

	if budget.TryReserve(1) {
		t.Error("TryReserve when exhausted should return false")
	}
}

func TestContextBudget_IsExhausted(t *testing.T) {
	budget := NewContextBudget(100)

	if budget.IsExhausted() {
		t.Error("Fresh budget should not be exhausted")
	}

	_ = budget.Reserve(100)

	if !budget.IsExhausted() {
		t.Error("Full budget should be exhausted")
	}
}

func TestContextBudget_Remaining(t *testing.T) {
	budget := NewContextBudget(100)

	if budget.Remaining() != 100 {
		t.Errorf("Expected remaining 100, got %d", budget.Remaining())
	}

	_ = budget.Reserve(30)

	if budget.Remaining() != 70 {
		t.Errorf("Expected remaining 70, got %d", budget.Remaining())
	}
}
