# Implementation Plan: Deep Dive & Architecture Drift Detection

> **Purpose:** Two differentiating features that give TaskWing unique value over Cursor/Copilot

---

## Executive Summary

| Feature | Value Proposition | Complexity | Dependencies |
|---------|-------------------|------------|--------------|
| **Deep Dive (`--deep`)** | "Show me how this function fits into the system" - traverses call graph, not just keyword matching | Medium | Existing call graph, SourceFetcher |
| **Drift Detection (`tw drift`)** | "Your memory says X but code does Y" - finds architectural violations | High | Rule extraction, pattern matching |

Both features leverage existing infrastructure but require new orchestration layers.

---

## Part 1: "Explain This" Deep Dive

### 1.1 User Experience

**CLI Usage:**
```bash
# Explain a symbol with system context
tw context "HandleRequest" --deep

# Explain with explicit symbol lookup
tw explain HandleRequest --depth 3

# MCP tool for AI assistants
{"tool": "explain_symbol", "params": {"symbol": "HandleRequest", "depth": 2}}
```

**Output Example:**
```
🔍 Symbol: HandleRequest (function)
   Location: cmd/api/handler.go:45-78
   Signature: func HandleRequest(w http.ResponseWriter, r *http.Request) error

📊 System Context (depth: 2)
───────────────────────────

⬆️ Called By (2 callers):
   1. main.setupRoutes → cmd/main.go:23
   2. middleware.AuthWrapper → internal/middleware/auth.go:67

⬇️ Calls (4 callees):
   1. validateInput → internal/validation/input.go:12
   2. userService.GetUser → internal/services/user.go:34
   3. responseWriter.JSON → internal/http/response.go:56
   4. logger.Error → internal/logging/logger.go:89

🔗 Impact Analysis:
   Direct dependents: 2 functions
   Transitive dependents: 7 functions (depth 2)

💬 Explanation:
   HandleRequest is the primary HTTP handler for user requests.
   It validates input, retrieves user data via userService, and
   returns JSON responses. It's called by the router setup and
   wrapped by auth middleware. Changes here would affect 7 downstream
   consumers including the main API flow and admin endpoints.

📁 Related Source:
   [Expanding 3 related symbols...]
```

### 1.2 Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     CLI / MCP Layer                         │
│  tw context --deep  │  tw explain  │  explain_symbol tool   │
└─────────────────────────────┬───────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                    ExplainApp (NEW)                         │
│  internal/app/explain.go                                    │
│                                                             │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────┐ │
│  │ Symbol Resolver │  │ Context Builder │  │ Narrator    │ │
│  │ - Find by name  │  │ - Call graph    │  │ - LLM synth │ │
│  │ - Disambiguate  │  │ - Source fetch  │  │ - Streaming │ │
│  └────────┬────────┘  └────────┬────────┘  └──────┬──────┘ │
└───────────┼────────────────────┼───────────────────┼────────┘
            │                    │                   │
            ▼                    ▼                   ▼
┌───────────────────┐  ┌─────────────────┐  ┌───────────────┐
│ codeintel.Query   │  │ app.SourceFetch │  │ llm.ChatModel │
│ - GetCallers      │  │ - FetchContext  │  │ - Stream()    │
│ - GetCallees      │  │ - Token budget  │  │ - Generate()  │
│ - AnalyzeImpact   │  │                 │  │               │
└───────────────────┘  └─────────────────┘  └───────────────┘
```

### 1.3 Data Models

```go
// internal/app/explain.go

// ExplainRequest configures what to explain
type ExplainRequest struct {
    Query       string // Symbol name or search query
    SymbolID    uint32 // Direct symbol ID (optional)
    Depth       int    // Call graph traversal depth (1-5, default: 2)
    IncludeCode bool   // Include source snippets (default: true)
    StreamWriter io.Writer // For streaming output
}

// ExplainResult contains the full explanation
type ExplainResult struct {
    // Target symbol
    Symbol      SymbolResponse `json:"symbol"`

    // Call graph context
    Callers     []CallNode     `json:"callers"`      // Who calls this
    Callees     []CallNode     `json:"callees"`      // What this calls
    ImpactStats ImpactStats    `json:"impact_stats"` // Dependency counts

    // Related knowledge
    Decisions   []NodeResponse `json:"decisions,omitempty"`   // Relevant decisions
    Patterns    []NodeResponse `json:"patterns,omitempty"`    // Matching patterns

    // Source context
    SourceCode  []CodeSnippet  `json:"source_code"`  // Symbol + related code

    // Synthesized explanation
    Explanation string         `json:"explanation"`  // LLM narrative
}

// CallNode represents a node in the call graph
type CallNode struct {
    Symbol    SymbolResponse `json:"symbol"`
    Depth     int            `json:"depth"`
    Relation  string         `json:"relation"` // "calls", "called_by"
    CallSite  string         `json:"call_site"` // "file:line"
}

// ImpactStats summarizes dependency impact
type ImpactStats struct {
    DirectCallers      int `json:"direct_callers"`
    DirectCallees      int `json:"direct_callees"`
    TransitiveDependents int `json:"transitive_dependents"`
    MaxDepthReached    int `json:"max_depth_reached"`
}
```

### 1.4 Implementation Phases

#### Phase 1: Core ExplainApp (3-4 hours)

**File: `internal/app/explain.go`**

```go
// ExplainApp provides deep symbol explanation
type ExplainApp struct {
    ctx       *Context
    queryService *codeintel.QueryService
}

func NewExplainApp(ctx *Context) *ExplainApp

// Explain generates a comprehensive explanation for a symbol
func (a *ExplainApp) Explain(ctx context.Context, req ExplainRequest) (*ExplainResult, error) {
    // 1. Resolve symbol (by name search or direct ID)
    symbol, err := a.resolveSymbol(ctx, req)

    // 2. Build call graph context
    callers := a.queryService.GetCallers(ctx, symbol.ID)
    callees := a.queryService.GetCallees(ctx, symbol.ID)
    impact := a.queryService.AnalyzeImpact(ctx, symbol.ID, req.Depth)

    // 3. Fetch related source code (symbol + top callers/callees)
    relatedSymbols := collectRelatedSymbols(symbol, callers, callees, limit: 5)
    sourceCode := a.fetchSourceContext(ctx, relatedSymbols)

    // 4. Find relevant knowledge (decisions/patterns mentioning this area)
    decisions := a.findRelevantKnowledge(ctx, symbol, "decision")
    patterns := a.findRelevantKnowledge(ctx, symbol, "pattern")

    // 5. Generate narrative explanation
    explanation := a.generateExplanation(ctx, symbol, callers, callees,
                                         impact, decisions, req.StreamWriter)

    return &ExplainResult{...}, nil
}
```

**Key Methods:**

```go
// resolveSymbol finds the symbol, disambiguating if multiple matches
func (a *ExplainApp) resolveSymbol(ctx context.Context, req ExplainRequest) (*codeintel.Symbol, error) {
    if req.SymbolID > 0 {
        return a.queryService.GetSymbol(ctx, req.SymbolID)
    }

    // Search by name
    results, _ := a.queryService.HybridSearch(ctx, req.Query, 5)

    if len(results) == 0 {
        return nil, ErrSymbolNotFound
    }
    if len(results) == 1 {
        return &results[0].Symbol, nil
    }

    // Multiple matches: prefer exact name match, then exported, then by score
    return disambiguate(results, req.Query)
}

// generateExplanation uses LLM to synthesize a narrative
func (a *ExplainApp) generateExplanation(ctx context.Context, ...) string {
    prompt := buildExplainPrompt(symbol, callers, callees, impact, decisions)

    // Use streaming if writer provided
    if streamWriter != nil {
        return a.streamExplanation(ctx, prompt, streamWriter)
    }
    return a.generateBlocking(ctx, prompt)
}
```

#### Phase 2: CLI Integration (1-2 hours)

**Option A: Extend `context` command with `--deep`**

```go
// cmd/context.go

var (
    contextDeep  bool
    contextDepth int
)

func init() {
    contextCmd.Flags().BoolVar(&contextDeep, "deep", false,
        "Deep dive: follow call graph to show system context")
    contextCmd.Flags().IntVar(&contextDepth, "depth", 2,
        "Call graph traversal depth (1-5)")
}

func runContext(cmd *cobra.Command, args []string) error {
    // ... existing setup ...

    if contextDeep {
        return runDeepExplain(ctx, recallApp, query, contextDepth, streamWriter)
    }

    // ... existing recall logic ...
}

func runDeepExplain(ctx context.Context, ...) error {
    explainApp := app.NewExplainApp(appCtx)
    result, err := explainApp.Explain(ctx, app.ExplainRequest{
        Query:       query,
        Depth:       depth,
        IncludeCode: true,
        StreamWriter: streamWriter,
    })

    // Render result (structured + narrative)
    ui.RenderExplainResult(result)
    return nil
}
```

**Option B: New `explain` command (cleaner UX)**

```go
// cmd/explain.go

var explainCmd = &cobra.Command{
    Use:   "explain <symbol>",
    Short: "Deep dive into a symbol's role in the system",
    Long: `Explain how a function/type fits into the codebase.

Shows:
  - Who calls this symbol and what it calls
  - Impact analysis (downstream dependents)
  - Related architectural decisions
  - Source code context

Examples:
  tw explain HandleRequest
  tw explain UserService.CreateUser --depth 3
  tw explain --id 1234  # By symbol ID`,
    Args: cobra.MinimumNArgs(1),
    RunE: runExplain,
}
```

#### Phase 3: MCP Tool (1 hour)

```go
// cmd/mcp_server.go

// Add to registerTools()
tools = append(tools, mcp.Tool{
    Name: "explain_symbol",
    Description: `Deep dive into a symbol's role in the system.
Shows call graph, impact analysis, and narrative explanation.
Use this when you need to understand how a function/type fits
into the larger codebase architecture.`,
    InputSchema: map[string]any{
        "type": "object",
        "properties": map[string]any{
            "symbol": map[string]any{
                "type":        "string",
                "description": "Symbol name to explain (function, type, etc.)",
            },
            "symbol_id": map[string]any{
                "type":        "integer",
                "description": "Direct symbol ID (optional, overrides name search)",
            },
            "depth": map[string]any{
                "type":        "integer",
                "description": "Call graph depth (1-5, default: 2)",
                "default":     2,
            },
        },
        "required": []string{"symbol"},
    },
})

// Handler
func handleExplainSymbol(ctx context.Context, params struct {
    Symbol   string `json:"symbol"`
    SymbolID int    `json:"symbol_id"`
    Depth    int    `json:"depth"`
}) (any, error) {
    explainApp := app.NewExplainApp(appCtx)
    return explainApp.Explain(ctx, app.ExplainRequest{
        Query:    params.Symbol,
        SymbolID: uint32(params.SymbolID),
        Depth:    max(1, min(5, params.Depth)),
    })
}
```

#### Phase 4: UI Rendering (1 hour)

```go
// internal/ui/explain.go

func RenderExplainResult(result *app.ExplainResult) {
    // Symbol header
    fmt.Printf("🔍 Symbol: %s (%s)\n", result.Symbol.Name, result.Symbol.Kind)
    fmt.Printf("   Location: %s\n", result.Symbol.Location)
    if result.Symbol.Signature != "" {
        fmt.Printf("   Signature: %s\n", result.Symbol.Signature)
    }

    // Call graph
    fmt.Println("\n📊 System Context")
    fmt.Println("─────────────────")

    fmt.Printf("\n⬆️  Called By (%d):\n", len(result.Callers))
    for i, c := range result.Callers[:min(5, len(result.Callers))] {
        fmt.Printf("   %d. %s → %s\n", i+1, c.Symbol.Name, c.CallSite)
    }

    fmt.Printf("\n⬇️  Calls (%d):\n", len(result.Callees))
    for i, c := range result.Callees[:min(5, len(result.Callees))] {
        fmt.Printf("   %d. %s → %s\n", i+1, c.Symbol.Name, c.CallSite)
    }

    // Impact stats
    fmt.Println("\n🔗 Impact Analysis:")
    fmt.Printf("   Direct dependents: %d\n", result.ImpactStats.DirectCallers)
    fmt.Printf("   Transitive dependents: %d (depth %d)\n",
               result.ImpactStats.TransitiveDependents,
               result.ImpactStats.MaxDepthReached)

    // Narrative explanation
    if result.Explanation != "" {
        fmt.Println("\n💬 Explanation:")
        fmt.Println(wrapText(result.Explanation, 80, "   "))
    }
}
```

### 1.5 LLM Prompt for Explanation

```go
const explainPromptTemplate = `You are explaining how a code symbol fits into a larger system.

## Target Symbol
Name: {{.Symbol.Name}}
Kind: {{.Symbol.Kind}}
File: {{.Symbol.FilePath}}:{{.Symbol.StartLine}}
{{if .Symbol.DocComment}}Documentation: {{.Symbol.DocComment}}{{end}}

## Call Graph Context
### Who calls this ({{len .Callers}} callers):
{{range .Callers}}- {{.Symbol.Name}} ({{.Symbol.FilePath}})
{{end}}

### What this calls ({{len .Callees}} callees):
{{range .Callees}}- {{.Symbol.Name}} ({{.Symbol.FilePath}})
{{end}}

### Impact Analysis
- Direct callers: {{.ImpactStats.DirectCallers}}
- Transitive dependents: {{.ImpactStats.TransitiveDependents}} (up to depth {{.ImpactStats.MaxDepthReached}})

{{if .Decisions}}
## Related Architectural Decisions
{{range .Decisions}}- [{{.Type}}] {{.Summary}}
{{end}}
{{end}}

{{if .SourceCode}}
## Source Code Context
{{range .SourceCode}}### {{.Kind}} {{.SymbolName}} ({{.FilePath}})
` + "```" + `
{{.Content}}
` + "```" + `
{{end}}
{{end}}

## Task
Write a concise explanation (2-3 paragraphs) that:
1. Describes what this symbol does and its purpose
2. Explains how it fits into the system (who uses it, what it depends on)
3. Notes any architectural significance or decisions related to it
4. Mentions the impact of changes (who would be affected)

Be specific and reference actual code locations when relevant.`
```

---

## Part 2: Architecture Drift Detection

### 2.1 User Experience

**CLI Usage:**
```bash
# Full drift analysis
tw drift

# Check specific constraint
tw drift --constraint "repository-pattern"

# Focus on specific paths
tw drift --path "internal/services/*"

# Output formats
tw drift --json
tw drift --format markdown > drift-report.md
```

**Output Example:**
```
🔍 Architecture Drift Analysis
══════════════════════════════

📋 Checking 5 architectural rules...

❌ VIOLATION: Repository Pattern
   Rule: "All database access must go through repository layer"
   Source: Decision from bootstrap (2024-01-15)

   Found 3 violations:

   1. internal/services/user.go:45
      ├─ Function: UserService.GetStats
      ├─ Issue: Direct call to db.Query()
      └─ Expected: Should call userRepo.GetStats()

   2. internal/handlers/admin.go:89
      ├─ Function: AdminHandler.DeleteUser
      ├─ Issue: Direct call to db.Exec()
      └─ Expected: Should call userRepo.Delete()

   3. cmd/migrate/main.go:23
      ├─ Function: runMigration
      ├─ Issue: Direct call to db.Exec()
      └─ Note: Migration scripts may be exempt

✅ PASSED: Error Handling Pattern
   Rule: "All public functions must return errors, not panic"
   Checked: 234 public functions

✅ PASSED: Naming Convention
   Rule: "Handlers must end with 'Handler' suffix"
   Checked: 12 handler files

⚠️ WARNING: Dependency Direction
   Rule: "internal/domain must not import internal/infra"

   Found 1 potential issue:

   1. internal/domain/user.go:3
      ├─ Import: "internal/infra/database"
      └─ Severity: Warning (may be intentional)

────────────────────────────────
📊 Summary: 3 violations, 1 warning, 2 passed
💡 Run 'tw drift --fix' for suggested fixes
```

### 2.2 Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     CLI Layer                                │
│  tw drift  │  tw drift --constraint X  │  drift MCP tool    │
└─────────────────────────────┬───────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                   DriftApp (NEW)                            │
│  internal/app/drift.go                                       │
│                                                              │
│  ┌─────────────────┐  ┌─────────────────┐  ┌──────────────┐ │
│  │ Rule Extractor  │  │ Violation       │  │ Report       │ │
│  │ - Parse nodes   │  │ Detector        │  │ Generator    │ │
│  │ - Classify type │  │ - Run checks    │  │ - Format     │ │
│  │ - Build rules   │  │ - Collect evid. │  │ - Suggest    │ │
│  └────────┬────────┘  └────────┬────────┘  └──────┬───────┘ │
└───────────┼────────────────────┼───────────────────┼─────────┘
            │                    │                   │
            ▼                    ▼                   ▼
┌───────────────────┐  ┌─────────────────┐  ┌───────────────┐
│ knowledge.Service │  │ codeintel.Query │  │ llm.ChatModel │
│ - GetConstraints  │  │ - SearchSymbols │  │ - Classify    │
│ - GetPatterns     │  │ - GetCallers    │  │ - Suggest fix │
└───────────────────┘  └─────────────────┘  └───────────────┘
```

### 2.3 Data Models

```go
// internal/app/drift.go

// Rule represents an executable architectural constraint
type Rule struct {
    ID          string       `json:"id"`
    Name        string       `json:"name"`
    Description string       `json:"description"`
    Type        RuleType     `json:"type"`
    Source      RuleSource   `json:"source"`      // Where rule came from
    Checks      []RuleCheck  `json:"checks"`      // What to verify
    Severity    Severity     `json:"severity"`    // error, warning, info
    Exemptions  []string     `json:"exemptions"`  // Paths to skip
}

type RuleType string
const (
    RuleTypeImport     RuleType = "import"      // Import restrictions
    RuleTypeNaming     RuleType = "naming"      // Naming conventions
    RuleTypeDependency RuleType = "dependency"  // Layer dependencies
    RuleTypePattern    RuleType = "pattern"     // Code patterns (e.g., repository)
    RuleTypeStructure  RuleType = "structure"   // Directory structure
)

type RuleSource struct {
    NodeID    string    `json:"node_id"`    // Source knowledge node
    NodeType  string    `json:"node_type"`  // decision, constraint, pattern
    CreatedAt time.Time `json:"created_at"`
}

// RuleCheck defines a single verification check
type RuleCheck struct {
    Description string            `json:"description"`
    Query       string            `json:"query"`       // Search query or pattern
    Condition   CheckCondition    `json:"condition"`   // must_exist, must_not_exist, etc.
    Parameters  map[string]string `json:"parameters"`  // Check-specific params
}

type CheckCondition string
const (
    ConditionMustExist    CheckCondition = "must_exist"
    ConditionMustNotExist CheckCondition = "must_not_exist"
    ConditionMustMatch    CheckCondition = "must_match"
    ConditionMustNotMatch CheckCondition = "must_not_match"
    ConditionMustCall     CheckCondition = "must_call"      // X must call Y
    ConditionMustNotCall  CheckCondition = "must_not_call"  // X must not call Y
)

// Violation represents a detected drift
type Violation struct {
    Rule       *Rule          `json:"rule"`
    Symbol     *SymbolResponse `json:"symbol,omitempty"`
    Location   string         `json:"location"`    // file:line
    Message    string         `json:"message"`
    Evidence   string         `json:"evidence"`    // Code snippet
    Suggestion string         `json:"suggestion"`  // How to fix
    Severity   Severity       `json:"severity"`
}

// DriftReport is the full analysis result
type DriftReport struct {
    Timestamp   time.Time    `json:"timestamp"`
    RulesChecked int         `json:"rules_checked"`
    Violations  []Violation  `json:"violations"`
    Warnings    []Violation  `json:"warnings"`
    Passed      []string     `json:"passed"`       // Rule names that passed
    Summary     DriftSummary `json:"summary"`
}

type DriftSummary struct {
    TotalRules     int `json:"total_rules"`
    Violations     int `json:"violations"`
    Warnings       int `json:"warnings"`
    Passed         int `json:"passed"`
    SymbolsChecked int `json:"symbols_checked"`
}
```

### 2.4 Implementation Phases

#### Phase 1: Rule Extraction (4-5 hours)

**Challenge:** Convert natural language constraints into executable rules.

```go
// internal/app/drift.go

type RuleExtractor struct {
    knowledgeService *knowledge.Service
    llmConfig        llm.Config
}

// ExtractRules pulls architectural rules from knowledge base
func (e *RuleExtractor) ExtractRules(ctx context.Context) ([]Rule, error) {
    // 1. Get all constraint and decision nodes
    constraints := e.knowledgeService.ListNodesByType(ctx, "constraint")
    decisions := e.knowledgeService.ListNodesByType(ctx, "decision")
    patterns := e.knowledgeService.ListNodesByType(ctx, "pattern")

    // 2. Filter for architectural rules (not all decisions are rules)
    candidates := filterArchitecturalRules(decisions, patterns)
    candidates = append(candidates, constraints...)

    // 3. Use LLM to classify and structure each rule
    var rules []Rule
    for _, node := range candidates {
        rule, err := e.classifyAndStructureRule(ctx, node)
        if err != nil || rule == nil {
            continue // Not a verifiable rule
        }
        rules = append(rules, *rule)
    }

    return rules, nil
}

// classifyAndStructureRule uses LLM to parse a node into executable Rule
func (e *RuleExtractor) classifyAndStructureRule(ctx context.Context, node memory.Node) (*Rule, error) {
    prompt := buildRuleClassificationPrompt(node)

    // LLM returns structured JSON
    response, err := e.callLLM(ctx, prompt)
    if err != nil {
        return nil, err
    }

    // Parse JSON response into Rule
    var result struct {
        IsVerifiable bool     `json:"is_verifiable"`
        RuleType     RuleType `json:"rule_type"`
        Checks       []struct {
            Description string         `json:"description"`
            Condition   CheckCondition `json:"condition"`
            Query       string         `json:"query"`
            Parameters  map[string]string `json:"parameters"`
        } `json:"checks"`
        Exemptions []string `json:"exemptions"`
    }

    if err := json.Unmarshal([]byte(response), &result); err != nil {
        return nil, err
    }

    if !result.IsVerifiable {
        return nil, nil // Not a rule we can verify
    }

    return &Rule{
        ID:          generateRuleID(node),
        Name:        node.Summary,
        Description: node.Content,
        Type:        result.RuleType,
        Source:      RuleSource{NodeID: node.ID, NodeType: node.Type},
        Checks:      convertChecks(result.Checks),
        Exemptions:  result.Exemptions,
    }, nil
}
```

**LLM Prompt for Rule Classification:**

```go
const ruleClassificationPrompt = `Analyze this architectural statement and determine if it's a verifiable code rule.

## Statement
Type: {{.Node.Type}}
Summary: {{.Node.Summary}}
Content: {{.Node.Content}}

## Task
1. Determine if this is a verifiable architectural rule (not just documentation)
2. If verifiable, classify the rule type and define checks

## Rule Types
- import: Controls which packages can import others
- naming: Enforces naming conventions (suffixes, prefixes, patterns)
- dependency: Controls layer dependencies (services can't call handlers)
- pattern: Enforces design patterns (all DB access via repositories)
- structure: Directory organization rules

## Response Format (JSON)
{
  "is_verifiable": true/false,
  "reasoning": "why this is/isn't a verifiable rule",
  "rule_type": "pattern|import|naming|dependency|structure",
  "checks": [
    {
      "description": "what this check verifies",
      "condition": "must_call|must_not_call|must_match|must_not_match",
      "query": "search query or pattern to find relevant code",
      "parameters": {
        "caller_pattern": "regex for functions that must follow rule",
        "callee_pattern": "regex for what they must/must not call",
        "file_pattern": "glob for files to check"
      }
    }
  ],
  "exemptions": ["paths or patterns that are exempt"]
}

Examples:
- "All HTTP handlers must validate input" → verifiable (pattern: handlers must call validation)
- "We chose Go for performance" → not verifiable (design decision, not a rule)
- "Services must not import handlers" → verifiable (dependency rule)
- "Use consistent error handling" → partially verifiable (pattern: public funcs return error)
`
```

#### Phase 2: Violation Detection (4-5 hours)

```go
// internal/app/drift.go

type ViolationDetector struct {
    queryService *codeintel.QueryService
    repo         codeintel.Repository
}

// DetectViolations checks all rules against the codebase
func (d *ViolationDetector) DetectViolations(ctx context.Context, rules []Rule) ([]Violation, error) {
    var violations []Violation

    for _, rule := range rules {
        ruleViolations, err := d.checkRule(ctx, rule)
        if err != nil {
            // Log but continue with other rules
            continue
        }
        violations = append(violations, ruleViolations...)
    }

    return violations, nil
}

// checkRule runs all checks for a single rule
func (d *ViolationDetector) checkRule(ctx context.Context, rule Rule) ([]Violation, error) {
    switch rule.Type {
    case RuleTypeDependency:
        return d.checkDependencyRule(ctx, rule)
    case RuleTypePattern:
        return d.checkPatternRule(ctx, rule)
    case RuleTypeNaming:
        return d.checkNamingRule(ctx, rule)
    case RuleTypeImport:
        return d.checkImportRule(ctx, rule)
    default:
        return nil, nil
    }
}

// checkDependencyRule verifies layer dependencies
// Example: "Services must not call handlers"
func (d *ViolationDetector) checkDependencyRule(ctx context.Context, rule Rule) ([]Violation, error) {
    var violations []Violation

    for _, check := range rule.Checks {
        // Find symbols matching caller pattern
        callerPattern := check.Parameters["caller_pattern"]
        callers, _ := d.queryService.SearchSymbolsByPattern(ctx, callerPattern)

        // For each caller, check what it calls
        for _, caller := range callers {
            if isExempt(caller.FilePath, rule.Exemptions) {
                continue
            }

            callees, _ := d.queryService.GetCallees(ctx, caller.ID)

            for _, callee := range callees {
                // Check if callee matches forbidden pattern
                if matchesForbiddenPattern(callee, check) {
                    violations = append(violations, Violation{
                        Rule:     &rule,
                        Symbol:   toSymbolResponse(caller),
                        Location: fmt.Sprintf("%s:%d", caller.FilePath, caller.StartLine),
                        Message:  fmt.Sprintf("%s calls %s (forbidden by %s)",
                                              caller.Name, callee.Name, rule.Name),
                        Evidence: fetchCodeSnippet(caller),
                        Severity: rule.Severity,
                    })
                }
            }
        }
    }

    return violations, nil
}

// checkPatternRule verifies design pattern adherence
// Example: "All DB access must go through repository layer"
func (d *ViolationDetector) checkPatternRule(ctx context.Context, rule Rule) ([]Violation, error) {
    var violations []Violation

    for _, check := range rule.Checks {
        switch check.Condition {
        case ConditionMustCall:
            // Find callers that SHOULD call something
            mustCallPattern := check.Parameters["must_call_pattern"]
            callerPattern := check.Parameters["caller_pattern"]

            callers, _ := d.queryService.SearchSymbolsByPattern(ctx, callerPattern)

            for _, caller := range callers {
                callees, _ := d.queryService.GetCallees(ctx, caller.ID)

                // Check if any callee matches required pattern
                hasRequiredCall := false
                for _, callee := range callees {
                    if matchesPattern(callee, mustCallPattern) {
                        hasRequiredCall = true
                        break
                    }
                }

                if !hasRequiredCall {
                    violations = append(violations, Violation{
                        Rule:     &rule,
                        Symbol:   toSymbolResponse(caller),
                        Location: fmt.Sprintf("%s:%d", caller.FilePath, caller.StartLine),
                        Message:  fmt.Sprintf("%s should call %s but doesn't",
                                              caller.Name, mustCallPattern),
                        Suggestion: fmt.Sprintf("Add call to %s", mustCallPattern),
                        Severity:   rule.Severity,
                    })
                }
            }

        case ConditionMustNotCall:
            // Find callers that should NOT call something
            forbiddenPattern := check.Parameters["forbidden_pattern"]
            callerPattern := check.Parameters["caller_pattern"]

            callers, _ := d.queryService.SearchSymbolsByPattern(ctx, callerPattern)

            for _, caller := range callers {
                callees, _ := d.queryService.GetCallees(ctx, caller.ID)

                for _, callee := range callees {
                    if matchesPattern(callee, forbiddenPattern) {
                        violations = append(violations, Violation{
                            Rule:     &rule,
                            Symbol:   toSymbolResponse(caller),
                            Location: fmt.Sprintf("%s:%d", caller.FilePath, caller.StartLine),
                            Message:  fmt.Sprintf("%s calls %s (forbidden)",
                                                  caller.Name, callee.Name),
                            Evidence: fetchCodeSnippet(caller),
                            Severity: rule.Severity,
                        })
                    }
                }
            }
        }
    }

    return violations, nil
}
```

#### Phase 3: CLI Command (2 hours)

```go
// cmd/drift.go

var driftCmd = &cobra.Command{
    Use:   "drift",
    Short: "Detect architectural drift from documented decisions",
    Long: `Analyze your codebase for violations of documented architectural rules.

Checks constraints, patterns, and decisions stored in TaskWing memory against
actual code patterns in your indexed codebase.

Examples:
  tw drift                           # Full analysis
  tw drift --constraint "repo-pattern" # Check specific rule
  tw drift --path "internal/services/*" # Focus on specific paths
  tw drift --json                    # Machine-readable output`,
    RunE: runDrift,
}

var (
    driftConstraint string
    driftPath       string
    driftVerbose    bool
)

func init() {
    rootCmd.AddCommand(driftCmd)
    driftCmd.Flags().StringVar(&driftConstraint, "constraint", "",
        "Check specific constraint by ID or name")
    driftCmd.Flags().StringVar(&driftPath, "path", "",
        "Focus on specific file paths (glob pattern)")
    driftCmd.Flags().BoolVar(&driftVerbose, "verbose", false,
        "Show detailed check information")
}

func runDrift(cmd *cobra.Command, args []string) error {
    // Initialize
    repo, _ := openRepo()
    defer repo.Close()

    llmCfg, _ := getLLMConfigForRole(cmd, llm.RoleQuery)
    appCtx := app.NewContextWithConfig(repo, llmCfg)
    driftApp := app.NewDriftApp(appCtx)

    // Progress feedback
    if !isQuiet() {
        fmt.Println("🔍 Architecture Drift Analysis")
        fmt.Println("══════════════════════════════")
        fmt.Println()
    }

    // Run analysis
    ctx := context.Background()
    report, err := driftApp.Analyze(ctx, app.DriftOptions{
        ConstraintFilter: driftConstraint,
        PathFilter:       driftPath,
        Verbose:          driftVerbose,
    })
    if err != nil {
        return err
    }

    // Output
    if isJSON() {
        return printJSON(report)
    }

    ui.RenderDriftReport(report)
    return nil
}
```

#### Phase 4: UI Rendering (1-2 hours)

```go
// internal/ui/drift.go

func RenderDriftReport(report *app.DriftReport) {
    // Summary header
    fmt.Printf("📋 Checking %d architectural rules...\n\n", report.RulesChecked)

    // Violations (errors)
    for _, v := range report.Violations {
        fmt.Printf("❌ VIOLATION: %s\n", v.Rule.Name)
        fmt.Printf("   Rule: \"%s\"\n", v.Rule.Description)
        fmt.Printf("   Source: %s\n\n", v.Rule.Source.NodeType)

        fmt.Printf("   Location: %s\n", v.Location)
        if v.Symbol != nil {
            fmt.Printf("   ├─ Function: %s\n", v.Symbol.Name)
        }
        fmt.Printf("   ├─ Issue: %s\n", v.Message)
        if v.Suggestion != "" {
            fmt.Printf("   └─ Expected: %s\n", v.Suggestion)
        }
        fmt.Println()
    }

    // Warnings
    for _, w := range report.Warnings {
        fmt.Printf("⚠️  WARNING: %s\n", w.Rule.Name)
        fmt.Printf("   %s at %s\n\n", w.Message, w.Location)
    }

    // Passed rules
    for _, passed := range report.Passed {
        fmt.Printf("✅ PASSED: %s\n", passed)
    }

    // Summary
    fmt.Println()
    fmt.Println("────────────────────────────────")
    fmt.Printf("📊 Summary: %d violations, %d warnings, %d passed\n",
               report.Summary.Violations, report.Summary.Warnings, report.Summary.Passed)
}
```

#### Phase 5: MCP Tool (1 hour)

```go
// cmd/mcp_server.go

tools = append(tools, mcp.Tool{
    Name: "detect_drift",
    Description: `Detect architectural drift between documented decisions and actual code.
Checks constraints, patterns, and decisions against the indexed codebase.
Returns violations where code doesn't match documented architecture.`,
    InputSchema: map[string]any{
        "type": "object",
        "properties": map[string]any{
            "constraint": map[string]any{
                "type":        "string",
                "description": "Specific constraint to check (optional)",
            },
            "path": map[string]any{
                "type":        "string",
                "description": "Focus on specific file paths (glob pattern)",
            },
        },
    },
})
```

---

## Part 3: Testing Strategy

### 3.1 Deep Dive Tests

```go
// internal/app/explain_test.go

func TestExplainApp_Explain(t *testing.T) {
    tests := []struct {
        name     string
        query    string
        depth    int
        wantErr  bool
        validate func(*testing.T, *ExplainResult)
    }{
        {
            name:  "explain simple function",
            query: "HandleRequest",
            depth: 2,
            validate: func(t *testing.T, r *ExplainResult) {
                assert.Equal(t, "HandleRequest", r.Symbol.Name)
                assert.NotEmpty(t, r.Callers)
                assert.NotEmpty(t, r.Explanation)
            },
        },
        {
            name:    "symbol not found",
            query:   "NonExistentFunction",
            wantErr: true,
        },
        {
            name:  "deep traversal",
            query: "main",
            depth: 5,
            validate: func(t *testing.T, r *ExplainResult) {
                assert.Greater(t, r.ImpactStats.TransitiveDependents, 0)
            },
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // ... test implementation
        })
    }
}
```

### 3.2 Drift Detection Tests

```go
// internal/app/drift_test.go

func TestRuleExtractor_ExtractRules(t *testing.T) {
    tests := []struct {
        name     string
        nodes    []memory.Node
        wantLen  int
        wantType RuleType
    }{
        {
            name: "extract pattern rule",
            nodes: []memory.Node{
                {
                    Type:    "constraint",
                    Summary: "Repository Pattern",
                    Content: "All database access must go through repository layer",
                },
            },
            wantLen:  1,
            wantType: RuleTypePattern,
        },
        {
            name: "skip non-verifiable",
            nodes: []memory.Node{
                {
                    Type:    "decision",
                    Summary: "We chose Go",
                    Content: "We chose Go for performance reasons",
                },
            },
            wantLen: 0, // Not a rule
        },
    }

    // ... test implementation
}

func TestViolationDetector_CheckPatternRule(t *testing.T) {
    // Test with mock symbol data
    // Verify correct violations are detected
}
```

---

## Part 4: Rollout Plan

### Phase 1: Deep Dive MVP (Week 1)
- [ ] Implement ExplainApp core
- [ ] Add `--deep` flag to context command
- [ ] Basic UI rendering
- [ ] Unit tests

### Phase 2: Deep Dive Polish (Week 2)
- [ ] Add `tw explain` command
- [ ] MCP tool integration
- [ ] Streaming support
- [ ] Documentation

### Phase 3: Drift Detection Foundation (Week 3)
- [ ] Rule extraction with LLM classification
- [ ] Basic pattern rule checking
- [ ] CLI command structure

### Phase 4: Drift Detection Full (Week 4)
- [ ] All rule types (import, naming, dependency, structure)
- [ ] UI rendering
- [ ] MCP tool
- [ ] Documentation and examples

### Phase 5: Integration & Polish (Week 5)
- [ ] End-to-end testing
- [ ] Performance optimization
- [ ] User documentation
- [ ] Release notes

---

## Appendix: File Locations

### New Files to Create

```
internal/app/explain.go       # ExplainApp implementation
internal/app/explain_test.go  # Tests
internal/app/drift.go         # DriftApp implementation
internal/app/drift_test.go    # Tests
internal/ui/explain.go        # Explain result rendering
internal/ui/drift.go          # Drift report rendering
cmd/explain.go                # tw explain command
cmd/drift.go                  # tw drift command
docs/development/DEEP_DIVE.md # Feature documentation
docs/development/DRIFT.md     # Feature documentation
```

### Existing Files to Modify

```
cmd/context.go                # Add --deep flag
cmd/mcp_server.go             # Add explain_symbol and detect_drift tools
internal/codeintel/query.go   # May need SearchSymbolsByPattern
internal/knowledge/service.go # May need GetRuleNodes helper
```

---

## Questions to Resolve Before Implementation

1. **Deep Dive Disambiguation**: When multiple symbols match, how to handle?
   - Option A: Return all matches with disambiguation UI
   - Option B: Use scoring heuristics (prefer exported, exact match, etc.)
   - Option C: Require exact match or ID

2. **Drift Rule Persistence**: Should extracted rules be cached?
   - Option A: Extract fresh each time (simpler, always current)
   - Option B: Cache with invalidation on memory changes
   - Option C: Store as first-class "rule" nodes

3. **Drift Exemptions**: How to handle intentional violations?
   - Option A: Comment annotations (`// drift:exempt`)
   - Option B: Exemption file (`.taskwing/drift-exemptions.yaml`)
   - Option C: Interactive "mark as intentional" command

4. **LLM Dependency**: Drift detection requires LLM for rule extraction.
   - Should we support a "basic mode" without LLM?
   - Pre-defined rule templates (import rules, naming rules)?
