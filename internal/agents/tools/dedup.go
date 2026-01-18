// Package tools provides shared tools for agent analysis.
package tools

import (
	"fmt"
	"strings"

	"github.com/josephgoksu/TaskWing/internal/agents/core"
)

// FindingDeduplicator merges and deduplicates findings from multiple analysis chunks.
type FindingDeduplicator struct {
	similarityThreshold float64 // Jaccard similarity threshold (0.0-1.0)
}

// NewFindingDeduplicator creates a new deduplicator with default settings.
func NewFindingDeduplicator() *FindingDeduplicator {
	return &FindingDeduplicator{
		similarityThreshold: 0.6, // 60% word overlap = likely duplicate
	}
}

// SetSimilarityThreshold sets the Jaccard similarity threshold for deduplication.
// Values closer to 1.0 require more exact matches; closer to 0.0 are more aggressive.
func (d *FindingDeduplicator) SetSimilarityThreshold(threshold float64) {
	if threshold > 0 && threshold <= 1.0 {
		d.similarityThreshold = threshold
	}
}

// DeduplicateFindings merges findings from multiple chunks, removing duplicates.
// Uses title + description similarity to identify duplicates.
// When duplicates are found, keeps the one with higher confidence.
func (d *FindingDeduplicator) DeduplicateFindings(allFindings []core.Finding) []core.Finding {
	if len(allFindings) == 0 {
		return nil
	}

	// Group by type first (decisions vs patterns)
	byType := make(map[string][]core.Finding)
	for _, f := range allFindings {
		byType[string(f.Type)] = append(byType[string(f.Type)], f)
	}

	var result []core.Finding

	for _, findings := range byType {
		deduped := d.deduplicateGroup(findings)
		result = append(result, deduped...)
	}

	return result
}

// deduplicateGroup deduplicates findings within a single type group.
// C4 Fix: When duplicates are found, merges evidence from both findings.
func (d *FindingDeduplicator) deduplicateGroup(findings []core.Finding) []core.Finding {
	if len(findings) == 0 {
		return nil
	}

	// Track which findings are duplicates of earlier ones
	isDuplicate := make([]bool, len(findings))
	// Track which finding each duplicate should merge evidence into
	mergeTarget := make([]int, len(findings))
	for i := range mergeTarget {
		mergeTarget[i] = i // Initially each finding is its own target
	}

	for i := 0; i < len(findings); i++ {
		if isDuplicate[i] {
			continue
		}

		for j := i + 1; j < len(findings); j++ {
			if isDuplicate[j] {
				continue
			}

			if d.areSimilar(findings[i], findings[j]) {
				// Keep the one with higher confidence
				confI := d.getConfidence(findings[i])
				confJ := d.getConfidence(findings[j])

				if confJ > confI {
					// j is better, mark i as duplicate, merge i's evidence into j
					isDuplicate[i] = true
					mergeTarget[i] = j
					break // i is now a duplicate, stop comparing
				} else {
					// i is better or equal, mark j as duplicate, merge j's evidence into i
					isDuplicate[j] = true
					mergeTarget[j] = i
				}
			}
		}
	}

	// C4 Fix: Merge evidence from duplicates into their targets
	for i, f := range findings {
		if isDuplicate[i] && mergeTarget[i] != i {
			target := mergeTarget[i]
			// Merge evidence arrays (dedupe by file path)
			findings[target].Evidence = mergeEvidence(findings[target].Evidence, f.Evidence)
		}
	}

	// Collect non-duplicate findings
	var result []core.Finding
	for i, f := range findings {
		if !isDuplicate[i] {
			result = append(result, f)
		}
	}

	return result
}

// mergeEvidence combines two evidence arrays, removing duplicates by file path + line.
func mergeEvidence(a, b []core.Evidence) []core.Evidence {
	if len(b) == 0 {
		return a
	}
	if len(a) == 0 {
		return b
	}

	seen := make(map[string]bool)
	var result []core.Evidence

	for _, e := range a {
		key := fmt.Sprintf("%s:%d", e.FilePath, e.StartLine)
		if !seen[key] {
			seen[key] = true
			result = append(result, e)
		}
	}

	for _, e := range b {
		key := fmt.Sprintf("%s:%d", e.FilePath, e.StartLine)
		if !seen[key] {
			seen[key] = true
			result = append(result, e)
		}
	}

	return result
}

// areSimilar checks if two findings are likely duplicates.
func (d *FindingDeduplicator) areSimilar(a, b core.Finding) bool {
	// Different types are never similar
	if a.Type != b.Type {
		return false
	}

	// Check title similarity
	titleSim := jaccardSimilarity(a.Title, b.Title)
	if titleSim >= d.similarityThreshold {
		return true
	}

	// Check description/what similarity
	descSim := jaccardSimilarity(a.Description, b.Description)
	if descSim >= d.similarityThreshold {
		return true
	}

	// Check combined title+description
	combinedA := a.Title + " " + a.Description
	combinedB := b.Title + " " + b.Description
	combinedSim := jaccardSimilarity(combinedA, combinedB)

	return combinedSim >= d.similarityThreshold
}

// getConfidence extracts confidence as a float64 for comparison.
func (d *FindingDeduplicator) getConfidence(f core.Finding) float64 {
	// Prefer ConfidenceScore (float64) if set
	if f.ConfidenceScore > 0 {
		return f.ConfidenceScore
	}

	// Fall back to legacy string Confidence field
	switch strings.ToLower(f.Confidence) {
	case "high":
		return 0.9
	case "medium":
		return 0.6
	case "low":
		return 0.3
	}

	return 0.5 // Default medium confidence
}

// jaccardSimilarity calculates word-level Jaccard similarity between two strings.
// Returns a value between 0.0 (no overlap) and 1.0 (identical).
func jaccardSimilarity(a, b string) float64 {
	wordsA := tokenize(a)
	wordsB := tokenize(b)

	if len(wordsA) == 0 && len(wordsB) == 0 {
		return 1.0 // Both empty = identical
	}
	if len(wordsA) == 0 || len(wordsB) == 0 {
		return 0.0 // One empty = no similarity
	}

	// Build sets
	setA := make(map[string]bool)
	for _, w := range wordsA {
		setA[w] = true
	}

	setB := make(map[string]bool)
	for _, w := range wordsB {
		setB[w] = true
	}

	// Calculate intersection and union
	intersection := 0
	for w := range setA {
		if setB[w] {
			intersection++
		}
	}

	union := len(setA) + len(setB) - intersection

	if union == 0 {
		return 0.0
	}

	return float64(intersection) / float64(union)
}

// tokenize splits a string into lowercase word tokens.
func tokenize(s string) []string {
	s = strings.ToLower(s)

	// Replace common separators with spaces
	replacer := strings.NewReplacer(
		"-", " ",
		"_", " ",
		"/", " ",
		".", " ",
		",", " ",
		":", " ",
		";", " ",
		"(", " ",
		")", " ",
		"[", " ",
		"]", " ",
		"{", " ",
		"}", " ",
	)
	s = replacer.Replace(s)

	// Split and filter
	parts := strings.Fields(s)
	var tokens []string

	for _, p := range parts {
		// Skip very short words and common stop words
		if len(p) < 2 {
			continue
		}
		if isStopWord(p) {
			continue
		}
		tokens = append(tokens, p)
	}

	return tokens
}

// C5 Fix: Package-level stop words map to avoid allocation on every call
var stopWords = map[string]bool{
	"the": true, "a": true, "an": true, "and": true, "or": true,
	"but": true, "in": true, "on": true, "at": true, "to": true,
	"for": true, "of": true, "with": true, "by": true, "from": true,
	"is": true, "are": true, "was": true, "were": true, "be": true,
	"been": true, "being": true, "have": true, "has": true, "had": true,
	"do": true, "does": true, "did": true, "will": true, "would": true,
	"could": true, "should": true, "may": true, "might": true, "must": true,
	"this": true, "that": true, "these": true, "those": true,
	"it": true, "its": true, "as": true, "if": true, "then": true,
}

// isStopWord returns true for common English stop words.
func isStopWord(word string) bool {
	return stopWords[word]
}

// DeduplicateRelationships removes duplicate relationships.
// C7 Fix: Normalizes case for comparison to catch duplicates like "AuthService" vs "authservice".
func (d *FindingDeduplicator) DeduplicateRelationships(rels []core.Relationship) []core.Relationship {
	if len(rels) == 0 {
		return nil
	}

	seen := make(map[string]bool)
	var result []core.Relationship

	for _, r := range rels {
		// Create a canonical key for this relationship (case-insensitive)
		key := strings.ToLower(r.From) + "|" + strings.ToLower(r.To) + "|" + strings.ToLower(r.Relation)
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, r)
	}

	return result
}
