// Package tools provides shared tools for agent analysis.
package tools

import (
	"errors"
	"sync"
)

// ErrBudgetExceeded is returned when a token reservation would exceed the budget.
var ErrBudgetExceeded = errors.New("context budget exceeded")

// MaxSafeContextBudget is the maximum tokens we'll use for context, regardless of model limits.
// This prevents hitting practical API limits that are lower than documented limits.
// Set to 80k tokens to safely fit within all providers' practical limits with room for:
// - System prompt overhead (~5-10k tokens)
// - Response buffer
// - Safety margin for token estimation variance
const MaxSafeContextBudget = 80_000

// ContextBudget tracks token usage and enforces limits.
// Use this to prevent agents from exceeding model context windows.
// Thread-safe for concurrent access.
type ContextBudget struct {
	mu          sync.Mutex
	totalBudget int
	usedTokens  int
}

// NewContextBudget creates a budget tracker with the given total token limit.
// Use 90% of model's MaxInputTokens to leave room for system prompt and response.
func NewContextBudget(totalTokens int) *ContextBudget {
	return &ContextBudget{
		totalBudget: totalTokens,
		usedTokens:  0,
	}
}

// NewSafeContextBudget creates a budget tracker that respects MaxSafeContextBudget.
// Use this for agents that need to stay within practical API limits regardless of
// the model's theoretical context window.
func NewSafeContextBudget(requestedTokens int) *ContextBudget {
	return &ContextBudget{
		totalBudget: min(requestedTokens, MaxSafeContextBudget),
		usedTokens:  0,
	}
}

// Reserve attempts to allocate tokens from the budget.
// Returns ErrBudgetExceeded if the reservation would exceed the limit.
func (b *ContextBudget) Reserve(tokens int) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.usedTokens+tokens > b.totalBudget {
		return ErrBudgetExceeded
	}
	b.usedTokens += tokens
	return nil
}

// TryReserve attempts to reserve tokens, returning success status without error.
// Useful for optional content that can be skipped if budget is full.
func (b *ContextBudget) TryReserve(tokens int) bool {
	return b.Reserve(tokens) == nil
}

// Remaining returns the number of tokens still available in the budget.
func (b *ContextBudget) Remaining() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.totalBudget - b.usedTokens
}

// Used returns the number of tokens currently consumed.
func (b *ContextBudget) Used() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.usedTokens
}

// Total returns the total budget capacity.
func (b *ContextBudget) Total() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.totalBudget
}

// IsExhausted returns true if no tokens remain in the budget.
func (b *ContextBudget) IsExhausted() bool {
	return b.Remaining() <= 0
}
