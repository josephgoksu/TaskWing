// Package util provides shared utility functions.
package util

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// Standard ID lengths for TaskWing entities.
const (
	// TaskIDLength is the full length of a task ID (e.g., "task-abcdef12").
	TaskIDLength = 13 // "task-" (5) + 8 hex chars
	// PlanIDLength is the full length of a plan ID (e.g., "plan-abcdef12").
	PlanIDLength = 13 // "plan-" (5) + 8 hex chars
	// DefaultShortIDLength is the default number of characters for short IDs.
	DefaultShortIDLength = 8
	// MaxAmbiguousCandidates is the max number of candidates to show in ambiguous error.
	MaxAmbiguousCandidates = 5
)

// Errors returned by ID resolution functions.
var (
	ErrAmbiguousID = errors.New("ambiguous ID prefix")
	ErrNotFound    = errors.New("not found")
)

// ShortID returns a shortened version of an ID.
// If n is 0 or negative, DefaultShortIDLength (8) is used.
// The function preserves the prefix (e.g., "task-" or "plan-") and truncates the suffix.
//
// Examples:
//
//	ShortID("task-abcdef12", 0) → "task-abc" (8 chars total including prefix)
//	ShortID("task-abcdef12", 10) → "task-abcde" (10 chars total)
//	ShortID("plan-xyz", 20) → "plan-xyz" (no truncation if shorter)
func ShortID(id string, n int) string {
	if n <= 0 {
		n = DefaultShortIDLength
	}
	if len(id) <= n {
		return id
	}
	return id[:n]
}

// IDPrefixResolver provides methods to find IDs by prefix.
// This is implemented by Repository.
type IDPrefixResolver interface {
	FindTaskIDsByPrefix(ctx context.Context, prefix string) ([]string, error)
	FindPlanIDsByPrefix(ctx context.Context, prefix string) ([]string, error)
}

// ResolveTaskID resolves a task ID or prefix to a full task ID.
//
// Resolution rules:
//  1. If idOrPrefix is a full-length ID and exists, return it.
//  2. If idOrPrefix matches exactly one task ID prefix, return that ID.
//  3. If multiple matches, return ErrAmbiguousID with candidates.
//  4. If no matches, return ErrNotFound.
func ResolveTaskID(ctx context.Context, resolver IDPrefixResolver, idOrPrefix string) (string, error) {
	if idOrPrefix == "" {
		return "", fmt.Errorf("task ID: %w", ErrNotFound)
	}

	// Normalize: if no prefix, assume task prefix
	normalized := idOrPrefix
	if !strings.HasPrefix(normalized, "task-") {
		normalized = "task-" + normalized
	}

	candidates, err := resolver.FindTaskIDsByPrefix(ctx, normalized)
	if err != nil {
		return "", fmt.Errorf("find task IDs: %w", err)
	}

	return resolveFromCandidates(normalized, candidates, "task")
}

// ResolvePlanID resolves a plan ID or prefix to a full plan ID.
//
// Resolution rules:
//  1. If idOrPrefix is a full-length ID and exists, return it.
//  2. If idOrPrefix matches exactly one plan ID prefix, return that ID.
//  3. If multiple matches, return ErrAmbiguousID with candidates.
//  4. If no matches, return ErrNotFound.
func ResolvePlanID(ctx context.Context, resolver IDPrefixResolver, idOrPrefix string) (string, error) {
	if idOrPrefix == "" {
		return "", fmt.Errorf("plan ID: %w", ErrNotFound)
	}

	// Normalize: if no prefix, assume plan prefix
	normalized := idOrPrefix
	if !strings.HasPrefix(normalized, "plan-") {
		normalized = "plan-" + normalized
	}

	candidates, err := resolver.FindPlanIDsByPrefix(ctx, normalized)
	if err != nil {
		return "", fmt.Errorf("find plan IDs: %w", err)
	}

	return resolveFromCandidates(normalized, candidates, "plan")
}

// resolveFromCandidates handles the common resolution logic.
func resolveFromCandidates(prefix string, candidates []string, entityType string) (string, error) {
	switch len(candidates) {
	case 0:
		return "", fmt.Errorf("%s with prefix %q: %w", entityType, prefix, ErrNotFound)
	case 1:
		return candidates[0], nil
	default:
		// Ambiguous: multiple matches
		shown := candidates
		if len(shown) > MaxAmbiguousCandidates {
			shown = shown[:MaxAmbiguousCandidates]
		}
		return "", fmt.Errorf("%w: prefix %q matches %d %ss: %v",
			ErrAmbiguousID, prefix, len(candidates), entityType, shown)
	}
}
