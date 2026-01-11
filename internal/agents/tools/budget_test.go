package tools

import (
	"sync"
	"testing"
)

func TestContextBudget_Reserve(t *testing.T) {
	b := NewContextBudget(100)

	// Reserve within budget
	if err := b.Reserve(50); err != nil {
		t.Errorf("Reserve(50) failed: %v", err)
	}
	if b.Used() != 50 {
		t.Errorf("Used() = %d, want 50", b.Used())
	}
	if b.Remaining() != 50 {
		t.Errorf("Remaining() = %d, want 50", b.Remaining())
	}

	// Reserve more, still within budget
	if err := b.Reserve(50); err != nil {
		t.Errorf("Reserve(50) failed: %v", err)
	}
	if b.Remaining() != 0 {
		t.Errorf("Remaining() = %d, want 0", b.Remaining())
	}

	// Reserve exceeding budget
	if err := b.Reserve(1); err != ErrBudgetExceeded {
		t.Errorf("Reserve(1) = %v, want ErrBudgetExceeded", err)
	}
}

func TestContextBudget_TryReserve(t *testing.T) {
	b := NewContextBudget(10)

	if !b.TryReserve(5) {
		t.Error("TryReserve(5) should succeed")
	}
	if !b.TryReserve(5) {
		t.Error("TryReserve(5) should succeed")
	}
	if b.TryReserve(1) {
		t.Error("TryReserve(1) should fail when budget exhausted")
	}
}

func TestContextBudget_IsExhausted(t *testing.T) {
	b := NewContextBudget(10)

	if b.IsExhausted() {
		t.Error("NewContextBudget should not be exhausted")
	}

	_ = b.Reserve(10)
	if !b.IsExhausted() {
		t.Error("Budget should be exhausted after full reservation")
	}
}

func TestContextBudget_ThreadSafety(t *testing.T) {
	b := NewContextBudget(1000)
	var wg sync.WaitGroup

	// Spawn 100 goroutines each reserving 10 tokens
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = b.Reserve(10)
		}()
	}

	wg.Wait()

	if b.Used() != 1000 {
		t.Errorf("Used() = %d, want 1000 after concurrent reservations", b.Used())
	}
}
