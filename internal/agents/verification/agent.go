/*
Package verification provides the VerificationAgent for deterministic evidence validation.
*/
package verification

import (
	"bufio"
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/josephgoksu/TaskWing/internal/agents/core"
	"github.com/josephgoksu/TaskWing/internal/utils"
)

// VerifierVersion is the current version of the verification logic.
const VerifierVersion = "1.0.0"

// Agent validates findings by checking their evidence against the actual codebase.
type Agent struct {
	basePath string
	verbose  bool
}

// NewAgent creates a new verification agent.
func NewAgent(basePath string) *Agent {
	return &Agent{basePath: basePath, verbose: false}
}

// SetVerbose enables detailed logging of verification steps.
func (v *Agent) SetVerbose(verbose bool) { v.verbose = verbose }

// Name returns the agent identifier.
func (v *Agent) Name() string { return "verification" }

// Description returns the agent description.
func (v *Agent) Description() string {
	return "Validates findings by checking evidence against actual codebase"
}

// VerifyFindings validates all provided findings and updates their verification status.
func (v *Agent) VerifyFindings(ctx context.Context, findings []core.Finding) []core.Finding {
	result := make([]core.Finding, len(findings))
	copy(result, findings)

	for i := range result {
		select {
		case <-ctx.Done():
			result[i].VerificationStatus = core.VerificationStatusSkipped
			continue
		default:
		}

		verificationResult := v.verifyFinding(&result[i])
		result[i].VerificationResult = &verificationResult
		result[i].VerificationStatus = verificationResult.Status

		if result[i].ConfidenceScore > 0 {
			adjusted := result[i].ConfidenceScore + verificationResult.ConfidenceAdjustment
			if adjusted < 0 {
				adjusted = 0
			} else if adjusted > 1.0 {
				adjusted = 1.0
			}
			result[i].ConfidenceScore = adjusted
			result[i].Confidence = core.ConfidenceLabelFromScore(adjusted)
		}
	}

	return result
}

// VerifySingleFinding validates a single finding's evidence.
func (v *Agent) VerifySingleFinding(finding *core.Finding) core.VerificationResult {
	return v.verifyFinding(finding)
}

func (v *Agent) verifyFinding(finding *core.Finding) core.VerificationResult {
	result := core.VerificationResult{
		VerifiedAt:      time.Now(),
		VerifierVersion: VerifierVersion,
	}

	if len(finding.Evidence) == 0 {
		result.Status = core.VerificationStatusSkipped
		return result
	}

	result.EvidenceResults = make([]core.EvidenceCheckResult, len(finding.Evidence))
	verifiedCount := 0
	partialCount := 0

	for i, evidence := range finding.Evidence {
		checkResult := v.checkEvidence(evidence)
		checkResult.EvidenceIndex = i
		result.EvidenceResults[i] = checkResult

		if checkResult.FileExists && checkResult.SnippetFound {
			if checkResult.LineNumbersMatch || evidence.StartLine == 0 {
				verifiedCount++
			} else {
				partialCount++
			}
		} else if checkResult.FileExists && checkResult.SimilarityScore > 0.5 {
			partialCount++
		}
	}

	totalEvidence := len(finding.Evidence)
	if verifiedCount == totalEvidence {
		result.Status = core.VerificationStatusVerified
		result.ConfidenceAdjustment = 0.1
	} else if verifiedCount+partialCount == totalEvidence && partialCount > 0 {
		result.Status = core.VerificationStatusPartial
		result.ConfidenceAdjustment = 0.0
	} else if verifiedCount+partialCount > 0 {
		result.Status = core.VerificationStatusPartial
		result.ConfidenceAdjustment = -0.1
	} else {
		result.Status = core.VerificationStatusRejected
		result.ConfidenceAdjustment = -0.3
	}

	return result
}

func (v *Agent) checkEvidence(evidence core.Evidence) core.EvidenceCheckResult {
	result := core.EvidenceCheckResult{}

	if evidence.FilePath == "" {
		result.ErrorMessage = "empty file path"
		return result
	}

	fullPath := evidence.FilePath
	if !filepath.IsAbs(evidence.FilePath) {
		fullPath = filepath.Join(v.basePath, evidence.FilePath)
	}
	fullPath = filepath.Clean(fullPath)

	if !strings.HasPrefix(fullPath, filepath.Clean(v.basePath)) {
		result.ErrorMessage = "path traversal detected"
		return result
	}

	info, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			result.ErrorMessage = "file not found"
		} else {
			result.ErrorMessage = err.Error()
		}
		return result
	}

	if info.IsDir() {
		result.ErrorMessage = "path is a directory, not a file"
		return result
	}

	result.FileExists = true

	if evidence.Snippet == "" {
		result.SnippetFound = true
		result.LineNumbersMatch = true
		return result
	}

	content, err := os.ReadFile(fullPath)
	if err != nil {
		result.ErrorMessage = "failed to read file: " + err.Error()
		return result
	}

	fileContent := string(content)
	normalizedSnippet := normalizeWhitespace(evidence.Snippet)

	if containsNormalized(fileContent, normalizedSnippet) {
		result.SnippetFound = true
	}

	if evidence.StartLine > 0 {
		actualContent := extractLines(fileContent, evidence.StartLine, evidence.EndLine)
		result.ActualContent = utils.Truncate(actualContent, 500)

		normalizedActual := normalizeWhitespace(actualContent)
		if normalizedActual == normalizedSnippet || strings.Contains(normalizedActual, normalizedSnippet) {
			result.LineNumbersMatch = true
			result.SnippetFound = true
		} else {
			result.SimilarityScore = calculateSimilarity(normalizedActual, normalizedSnippet)
		}
	} else if result.SnippetFound {
		result.LineNumbersMatch = true
	}

	if !result.SnippetFound && evidence.GrepPattern != "" {
		if strings.Contains(fileContent, evidence.GrepPattern) {
			result.SnippetFound = true
			result.SimilarityScore = 0.6
		}
	}

	if !result.SnippetFound && result.SimilarityScore == 0 {
		result.SimilarityScore = calculateSimilarity(normalizeWhitespace(fileContent), normalizedSnippet)
	}

	return result
}

func extractLines(content string, start, end int) string {
	if start <= 0 {
		start = 1
	}
	if end <= 0 || end < start {
		end = start
	}

	scanner := bufio.NewScanner(strings.NewReader(content))
	var lines []string
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		if lineNum >= start && lineNum <= end {
			lines = append(lines, scanner.Text())
		}
		if lineNum > end {
			break
		}
	}

	return strings.Join(lines, "\n")
}

func normalizeWhitespace(s string) string {
	lines := strings.Split(s, "\n")
	var normalized []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			words := strings.Fields(trimmed)
			normalized = append(normalized, strings.Join(words, " "))
		}
	}
	return strings.Join(normalized, "\n")
}

func containsNormalized(haystack, needle string) bool {
	return strings.Contains(normalizeWhitespace(haystack), needle)
}

func calculateSimilarity(a, b string) float64 {
	if a == "" || b == "" {
		return 0
	}

	wordsA := strings.Fields(strings.ToLower(a))
	wordsB := strings.Fields(strings.ToLower(b))

	if len(wordsA) == 0 || len(wordsB) == 0 {
		return 0
	}

	setA := make(map[string]bool)
	for _, w := range wordsA {
		setA[w] = true
	}

	intersection := 0
	setB := make(map[string]bool)
	for _, w := range wordsB {
		setB[w] = true
		if setA[w] {
			intersection++
		}
	}

	union := len(setA) + len(setB) - intersection
	if union == 0 {
		return 0
	}

	return float64(intersection) / float64(union)
}

// FilterVerifiedFindings returns only findings that passed verification.
func FilterVerifiedFindings(findings []core.Finding) []core.Finding {
	var result []core.Finding
	for _, f := range findings {
		if f.VerificationStatus != core.VerificationStatusRejected {
			result = append(result, f)
		}
	}
	return result
}

// FilterByMinConfidence returns findings with confidence score >= minScore.
func FilterByMinConfidence(findings []core.Finding, minScore float64) []core.Finding {
	var result []core.Finding
	for _, f := range findings {
		if f.ConfidenceScore >= minScore {
			result = append(result, f)
		}
	}
	return result
}
