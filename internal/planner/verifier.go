// Package planner provides plan verification using code intelligence.
package planner

import (
	"context"

	"github.com/josephgoksu/TaskWing/internal/codeintel"
)

// Verifier defines the interface for plan verification.
type Verifier interface {
	// Verify validates and optionally corrects tasks using code intelligence.
	// Returns the verified (and potentially corrected) tasks.
	Verify(ctx context.Context, tasks []LLMTaskSchema) ([]LLMTaskSchema, error)
}

// PlanVerifier validates plan tasks against the actual codebase using code intelligence.
// It uses the codeintel.QueryService to verify file paths, symbol references,
// and dependency relationships mentioned in tasks.
type PlanVerifier struct {
	query *codeintel.QueryService
}

// NewPlanVerifier creates a new PlanVerifier with the given query service.
func NewPlanVerifier(query *codeintel.QueryService) *PlanVerifier {
	return &PlanVerifier{
		query: query,
	}
}

// Verify validates tasks against the codebase and attempts to correct any issues.
// It checks:
// - File paths referenced in task descriptions
// - Symbol names mentioned in tasks
// - Dependency relationships
//
// Returns the verified (and potentially corrected) tasks.
func (v *PlanVerifier) Verify(ctx context.Context, tasks []LLMTaskSchema) ([]LLMTaskSchema, error) {
	// TODO: Implement verification logic in subsequent tasks
	// For now, pass through unchanged
	return tasks, nil
}

// Ensure PlanVerifier implements Verifier
var _ Verifier = (*PlanVerifier)(nil)
