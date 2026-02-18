// Package app provides the DriftApp for architecture drift detection.
package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/josephgoksu/TaskWing/internal/codeintel"
	"github.com/josephgoksu/TaskWing/internal/knowledge"
	"github.com/josephgoksu/TaskWing/internal/llm"
	"github.com/josephgoksu/TaskWing/internal/memory"

	"github.com/cloudwego/eino/schema"
)

// === Data Models ===

// RuleType classifies the kind of architectural rule.
type RuleType string

const (
	RuleTypeImport     RuleType = "import"     // Import restrictions
	RuleTypeNaming     RuleType = "naming"     // Naming conventions
	RuleTypeDependency RuleType = "dependency" // Layer dependencies
	RuleTypePattern    RuleType = "pattern"    // Code patterns (e.g., repository)
	RuleTypeStructure  RuleType = "structure"  // Directory structure
)

// Severity indicates the importance of a rule violation.
type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
	SeverityInfo    Severity = "info"
)

// RuleSource tracks where a rule was derived from.
type RuleSource struct {
	NodeID    string    `json:"node_id"`
	NodeType  string    `json:"node_type"` // decision, constraint, pattern
	CreatedAt time.Time `json:"created_at"`
}

// CheckCondition defines what kind of check to perform.
type CheckCondition string

const (
	ConditionMustExist    CheckCondition = "must_exist"
	ConditionMustNotExist CheckCondition = "must_not_exist"
	ConditionMustMatch    CheckCondition = "must_match"
	ConditionMustNotMatch CheckCondition = "must_not_match"
	ConditionMustCall     CheckCondition = "must_call"     // X must call Y
	ConditionMustNotCall  CheckCondition = "must_not_call" // X must not call Y
)

// RuleCheck defines a single verification check.
type RuleCheck struct {
	Description string            `json:"description"`
	Query       string            `json:"query"`     // Search query or pattern
	Condition   CheckCondition    `json:"condition"` // must_exist, must_not_exist, etc.
	Parameters  map[string]string `json:"parameters"`
}

// Rule represents an executable architectural constraint.
type Rule struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Type        RuleType    `json:"type"`
	Source      RuleSource  `json:"source"`
	Checks      []RuleCheck `json:"checks"`
	Severity    Severity    `json:"severity"`
	Exemptions  []string    `json:"exemptions"` // Glob patterns to skip
}

// Violation represents a detected architectural drift.
type Violation struct {
	Rule       *Rule           `json:"rule"`
	Symbol     *SymbolResponse `json:"symbol,omitempty"`
	Location   string          `json:"location"` // file:line
	Message    string          `json:"message"`
	Evidence   string          `json:"evidence"`   // Code snippet
	Suggestion string          `json:"suggestion"` // How to fix
	Severity   Severity        `json:"severity"`
}

// DriftSummary provides aggregate statistics.
type DriftSummary struct {
	TotalRules     int `json:"total_rules"`
	Violations     int `json:"violations"`
	Warnings       int `json:"warnings"`
	Passed         int `json:"passed"`
	SymbolsChecked int `json:"symbols_checked"`
}

// DriftReport is the full analysis result.
type DriftReport struct {
	Timestamp    time.Time    `json:"timestamp"`
	RulesChecked int          `json:"rules_checked"`
	Violations   []Violation  `json:"violations"`
	Warnings     []Violation  `json:"warnings"`
	Passed       []string     `json:"passed"` // Rule names that passed
	Summary      DriftSummary `json:"summary"`
}

// DriftRequest configures the drift analysis.
type DriftRequest struct {
	Constraint string   // Specific constraint name to check
	Paths      []string // Limit to specific paths (glob patterns)
	Severity   Severity // Minimum severity to report
}

// === DriftApp ===

// DriftApp detects architectural drift between documented rules and code.
type DriftApp struct {
	ctx          *Context
	knowledgeSvc *knowledge.Service
	queryService *codeintel.QueryService
}

// NewDriftApp creates a new drift detection application.
func NewDriftApp(ctx *Context) *DriftApp {
	ks := knowledge.NewService(ctx.Repo, ctx.LLMCfg)

	// Get database handle for codeintel
	var queryService *codeintel.QueryService
	if ctx.Repo != nil {
		store := ctx.Repo.GetDB()
		if store != nil && store.DB() != nil {
			codeRepo := codeintel.NewRepository(store.DB())
			queryService = codeintel.NewQueryService(codeRepo, ctx.LLMCfg)
		}
	}

	return &DriftApp{
		ctx:          ctx,
		knowledgeSvc: ks,
		queryService: queryService,
	}
}

// Analyze runs drift detection and returns a report.
func (a *DriftApp) Analyze(ctx context.Context, req DriftRequest) (*DriftReport, error) {
	// 1. Extract rules from knowledge base
	rules, err := a.extractRules(ctx)
	if err != nil {
		return nil, fmt.Errorf("extract rules: %w", err)
	}

	// Filter by constraint name if specified
	if req.Constraint != "" {
		rules = filterRulesByName(rules, req.Constraint)
	}

	if len(rules) == 0 {
		return &DriftReport{
			Timestamp: time.Now(),
			Summary:   DriftSummary{},
			Passed:    []string{},
		}, nil
	}

	// 2. Detect violations
	allViolations, err := a.detectViolations(ctx, rules, req.Paths)
	if err != nil {
		return nil, fmt.Errorf("detect violations: %w", err)
	}

	// 3. Separate by severity
	var violations, warnings []Violation
	var passed []string

	ruleViolations := make(map[string]bool)
	for _, v := range allViolations {
		ruleViolations[v.Rule.ID] = true
		if v.Severity == SeverityWarning {
			warnings = append(warnings, v)
		} else {
			violations = append(violations, v)
		}
	}

	for _, rule := range rules {
		if !ruleViolations[rule.ID] {
			passed = append(passed, rule.Name)
		}
	}

	// 4. Build report
	return &DriftReport{
		Timestamp:    time.Now(),
		RulesChecked: len(rules),
		Violations:   violations,
		Warnings:     warnings,
		Passed:       passed,
		Summary: DriftSummary{
			TotalRules: len(rules),
			Violations: len(violations),
			Warnings:   len(warnings),
			Passed:     len(passed),
		},
	}, nil
}

// extractRules pulls architectural rules from the knowledge base.
func (a *DriftApp) extractRules(ctx context.Context) ([]Rule, error) {
	// Get constraints, patterns, and relevant decisions
	constraints, _ := a.knowledgeSvc.SearchByType(ctx, "", "constraint", 100)
	patterns, _ := a.knowledgeSvc.SearchByType(ctx, "", "pattern", 100)
	decisions, _ := a.knowledgeSvc.SearchByType(ctx, "", "decision", 100)

	// Combine candidates
	var candidates []memory.Node
	for _, sn := range constraints {
		if sn.Node != nil {
			candidates = append(candidates, *sn.Node)
		}
	}
	for _, sn := range patterns {
		if sn.Node != nil {
			candidates = append(candidates, *sn.Node)
		}
	}
	for _, sn := range decisions {
		if sn.Node != nil {
			candidates = append(candidates, *sn.Node)
		}
	}

	if len(candidates) == 0 {
		return nil, nil
	}

	// Use LLM to classify each as verifiable rule
	var rules []Rule
	for _, node := range candidates {
		rule, err := a.classifyRule(ctx, node)
		if err != nil {
			continue // Skip non-verifiable
		}
		if rule != nil {
			rules = append(rules, *rule)
		}
	}

	return rules, nil
}

// classifyRule uses LLM to determine if a node is a verifiable rule.
func (a *DriftApp) classifyRule(ctx context.Context, node memory.Node) (*Rule, error) {
	prompt := buildRuleClassificationPrompt(node)

	chatModel, err := llm.NewCloseableChatModel(ctx, a.ctx.LLMCfg)
	if err != nil {
		return nil, err
	}
	defer func() { _ = chatModel.Close() }()

	messages := []*schema.Message{schema.UserMessage(prompt)}
	resp, err := chatModel.Generate(ctx, messages)
	if err != nil {
		return nil, err
	}

	// Parse JSON response
	var result struct {
		IsVerifiable bool     `json:"is_verifiable"`
		RuleType     RuleType `json:"rule_type"`
		Severity     Severity `json:"severity"`
		Checks       []struct {
			Description string            `json:"description"`
			Condition   CheckCondition    `json:"condition"`
			Query       string            `json:"query"`
			Parameters  map[string]string `json:"parameters"`
		} `json:"checks"`
		Exemptions []string `json:"exemptions"`
	}

	// Extract JSON from response
	content := extractJSON(resp.Content)
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return nil, err
	}

	if !result.IsVerifiable {
		return nil, nil
	}

	// Convert checks
	checks := make([]RuleCheck, len(result.Checks))
	for i, c := range result.Checks {
		checks[i] = RuleCheck{
			Description: c.Description,
			Condition:   c.Condition,
			Query:       c.Query,
			Parameters:  c.Parameters,
		}
	}

	severity := result.Severity
	if severity == "" {
		severity = SeverityError
	}

	return &Rule{
		ID:          generateRuleID(node),
		Name:        node.Summary,
		Description: node.Text(),
		Type:        result.RuleType,
		Source: RuleSource{
			NodeID:    node.ID,
			NodeType:  node.Type,
			CreatedAt: time.Now(),
		},
		Checks:     checks,
		Severity:   severity,
		Exemptions: result.Exemptions,
	}, nil
}

// detectViolations checks all rules against the codebase.
func (a *DriftApp) detectViolations(ctx context.Context, rules []Rule, paths []string) ([]Violation, error) {
	if a.queryService == nil {
		return nil, fmt.Errorf("code intelligence not available (run 'taskwing bootstrap' first)")
	}

	var violations []Violation

	for _, rule := range rules {
		ruleViolations, err := a.checkRule(ctx, rule, paths)
		if err != nil {
			continue // Log but continue with other rules
		}
		violations = append(violations, ruleViolations...)
	}

	return violations, nil
}

// checkRule runs all checks for a single rule.
func (a *DriftApp) checkRule(ctx context.Context, rule Rule, paths []string) ([]Violation, error) {
	switch rule.Type {
	case RuleTypeDependency:
		return a.checkDependencyRule(ctx, rule, paths)
	case RuleTypePattern:
		return a.checkPatternRule(ctx, rule, paths)
	case RuleTypeNaming:
		return a.checkNamingRule(ctx, rule, paths)
	case RuleTypeImport:
		return a.checkImportRule(ctx, rule, paths)
	default:
		return nil, nil
	}
}

// checkDependencyRule verifies layer dependencies (e.g., "services must not call handlers").
func (a *DriftApp) checkDependencyRule(ctx context.Context, rule Rule, paths []string) ([]Violation, error) {
	var violations []Violation

	for _, check := range rule.Checks {
		if check.Condition != ConditionMustNotCall && check.Condition != ConditionMustCall {
			continue
		}

		// Get caller pattern from parameters
		callerPattern := check.Parameters["caller_pattern"]
		calleePattern := check.Parameters["callee_pattern"]

		if callerPattern == "" || calleePattern == "" {
			continue
		}

		// Find all symbols matching caller pattern
		callers, err := a.queryService.HybridSearch(ctx, callerPattern, 100)
		if err != nil {
			continue
		}

		callerRe, _ := regexp.Compile(callerPattern)
		calleeRe, _ := regexp.Compile(calleePattern)

		for _, result := range callers {
			sym := result.Symbol

			// Check path filter
			if !matchesPaths(sym.FilePath, paths) {
				continue
			}
			if matchesExemptions(sym.FilePath, rule.Exemptions) {
				continue
			}

			// Check if caller matches pattern
			if callerRe != nil && !callerRe.MatchString(sym.Name) && !callerRe.MatchString(sym.FilePath) {
				continue
			}

			// Get what this symbol calls
			callees, err := a.queryService.GetCallees(ctx, sym.ID)
			if err != nil {
				continue
			}

			for _, callee := range callees {
				matches := false
				if calleeRe != nil {
					matches = calleeRe.MatchString(callee.Name) || calleeRe.MatchString(callee.FilePath)
				}

				if check.Condition == ConditionMustNotCall && matches {
					violations = append(violations, Violation{
						Rule:     &rule,
						Symbol:   &SymbolResponse{Name: sym.Name, Kind: string(sym.Kind), FilePath: sym.FilePath, StartLine: sym.StartLine},
						Location: fmt.Sprintf("%s:%d", sym.FilePath, sym.StartLine),
						Message:  fmt.Sprintf("%s calls %s, which violates: %s", sym.Name, callee.Name, check.Description),
						Evidence: fmt.Sprintf("%s → %s", sym.Name, callee.Name),
						Severity: rule.Severity,
					})
				}
			}
		}
	}

	return violations, nil
}

// checkPatternRule verifies design pattern adherence.
func (a *DriftApp) checkPatternRule(ctx context.Context, rule Rule, paths []string) ([]Violation, error) {
	var violations []Violation

	for _, check := range rule.Checks {
		// Search for relevant symbols
		results, err := a.queryService.HybridSearch(ctx, check.Query, 100)
		if err != nil {
			continue
		}

		targetPattern := check.Parameters["target_pattern"]
		requiredPattern := check.Parameters["required_pattern"]

		targetRe, _ := regexp.Compile(targetPattern)
		requiredRe, _ := regexp.Compile(requiredPattern)

		for _, result := range results {
			sym := result.Symbol

			if !matchesPaths(sym.FilePath, paths) {
				continue
			}
			if matchesExemptions(sym.FilePath, rule.Exemptions) {
				continue
			}

			// Check if symbol should follow this pattern
			if targetRe != nil && !targetRe.MatchString(sym.Name) && !targetRe.MatchString(sym.FilePath) {
				continue
			}

			// Get what this symbol calls
			callees, err := a.queryService.GetCallees(ctx, sym.ID)
			if err != nil {
				continue
			}

			// Check if any callee matches required pattern
			hasRequired := false
			for _, callee := range callees {
				if requiredRe != nil && (requiredRe.MatchString(callee.Name) || requiredRe.MatchString(callee.FilePath)) {
					hasRequired = true
					break
				}
			}

			if check.Condition == ConditionMustCall && !hasRequired && requiredRe != nil {
				violations = append(violations, Violation{
					Rule:     &rule,
					Symbol:   &SymbolResponse{Name: sym.Name, Kind: string(sym.Kind), FilePath: sym.FilePath, StartLine: sym.StartLine},
					Location: fmt.Sprintf("%s:%d", sym.FilePath, sym.StartLine),
					Message:  fmt.Sprintf("%s should follow pattern: %s", sym.Name, check.Description),
					Severity: rule.Severity,
				})
			}
		}
	}

	return violations, nil
}

// checkNamingRule verifies naming conventions.
func (a *DriftApp) checkNamingRule(ctx context.Context, rule Rule, paths []string) ([]Violation, error) {
	var violations []Violation

	for _, check := range rule.Checks {
		results, err := a.queryService.HybridSearch(ctx, check.Query, 100)
		if err != nil {
			continue
		}

		namePattern := check.Parameters["name_pattern"]
		filePattern := check.Parameters["file_pattern"]

		nameRe, _ := regexp.Compile(namePattern)
		fileRe, _ := regexp.Compile(filePattern)

		for _, result := range results {
			sym := result.Symbol

			if !matchesPaths(sym.FilePath, paths) {
				continue
			}
			if matchesExemptions(sym.FilePath, rule.Exemptions) {
				continue
			}

			// Check if file matches pattern (e.g., handler files)
			if fileRe != nil && !fileRe.MatchString(sym.FilePath) {
				continue
			}

			// Check if name follows convention
			if nameRe != nil {
				if check.Condition == ConditionMustMatch && !nameRe.MatchString(sym.Name) {
					violations = append(violations, Violation{
						Rule:       &rule,
						Symbol:     &SymbolResponse{Name: sym.Name, Kind: string(sym.Kind), FilePath: sym.FilePath, StartLine: sym.StartLine},
						Location:   fmt.Sprintf("%s:%d", sym.FilePath, sym.StartLine),
						Message:    fmt.Sprintf("'%s' doesn't match naming convention: %s", sym.Name, check.Description),
						Suggestion: fmt.Sprintf("Rename to match pattern: %s", namePattern),
						Severity:   rule.Severity,
					})
				}
			}
		}
	}

	return violations, nil
}

// checkImportRule verifies import restrictions.
func (a *DriftApp) checkImportRule(ctx context.Context, rule Rule, paths []string) ([]Violation, error) {
	// Import checking requires AST analysis which we'd need to implement separately
	// For now, return nil - this is a placeholder for future implementation
	return nil, nil
}

// === Helper Functions ===

func buildRuleClassificationPrompt(node memory.Node) string {
	return fmt.Sprintf(`Analyze this architectural statement and determine if it's a verifiable code rule.

## Statement
Type: %s
Summary: %s
Content: %s

## Task
1. Determine if this is a verifiable architectural rule (not just documentation)
2. If verifiable, classify the rule type and define checks

## Rule Types
- import: Controls which packages can import others
- naming: Enforces naming conventions (suffixes, prefixes, patterns)
- dependency: Controls layer dependencies (services can't call handlers)
- pattern: Enforces design patterns (all DB access via repositories)
- structure: Directory organization rules

## Response Format (JSON only, no markdown)
{
  "is_verifiable": true,
  "rule_type": "pattern",
  "severity": "error",
  "checks": [
    {
      "description": "what this check verifies",
      "condition": "must_call",
      "query": "search query to find relevant symbols",
      "parameters": {
        "caller_pattern": "regex for functions that must follow rule",
        "callee_pattern": "regex for what they must/must not call",
        "target_pattern": "regex for symbols to check",
        "required_pattern": "regex for required calls",
        "name_pattern": "regex for naming convention",
        "file_pattern": "regex for file paths"
      }
    }
  ],
  "exemptions": ["test files", "migrations"]
}

Rules for is_verifiable=true:
- "All HTTP handlers must validate input" → verifiable (pattern)
- "Services must not import handlers" → verifiable (dependency)
- "Handlers must end with 'Handler' suffix" → verifiable (naming)

Rules for is_verifiable=false:
- "We chose Go for performance" → not verifiable (design rationale)
- "The team prefers simplicity" → not verifiable (principle)

Return ONLY valid JSON, no explanations.`, node.Type, node.Summary, node.Text())
}

func generateRuleID(node memory.Node) string {
	hash := sha256.Sum256([]byte(node.ID + node.Summary))
	return hex.EncodeToString(hash[:8])
}

func filterRulesByName(rules []Rule, name string) []Rule {
	var filtered []Rule
	nameLower := strings.ToLower(name)
	for _, rule := range rules {
		if strings.Contains(strings.ToLower(rule.Name), nameLower) {
			filtered = append(filtered, rule)
		}
	}
	return filtered
}

func matchesPaths(filePath string, patterns []string) bool {
	if len(patterns) == 0 {
		return true
	}
	for _, pattern := range patterns {
		if matched, _ := regexp.MatchString(pattern, filePath); matched {
			return true
		}
	}
	return false
}

func matchesExemptions(filePath string, exemptions []string) bool {
	for _, exemption := range exemptions {
		if strings.Contains(filePath, exemption) {
			return true
		}
		if matched, _ := regexp.MatchString(exemption, filePath); matched {
			return true
		}
	}
	return false
}

func extractJSON(content string) string {
	// Find JSON object in response
	start := strings.Index(content, "{")
	end := strings.LastIndex(content, "}")
	if start >= 0 && end > start {
		return content[start : end+1]
	}
	return content
}
