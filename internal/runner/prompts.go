package runner

import (
	"fmt"
	"strings"
)

// BootstrapAnalysisPrompt builds a prompt for AI CLI to analyze a codebase
// and extract architectural findings with full fidelity.
func BootstrapAnalysisPrompt(projectPath string, existingContext string, focusArea string) string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "Analyze the codebase at %s", projectPath)

	if focusArea != "" {
		fmt.Fprintf(&sb, " with a focus on %s", focusArea)
	}
	sb.WriteString(".\n\n")

	sb.WriteString(`IMPORTANT: You MUST use your tools to read actual source files before making findings.
- Use Read/Glob to explore the directory structure and key files
- Use Bash to run ` + "`git log --oneline -100`" + ` for commit history
- Read go.mod/package.json/Cargo.toml for dependency analysis
- Read CI/CD configs (.github/workflows/, Makefile, Dockerfile, etc.)
Do NOT guess from the path alone — verify every finding with evidence from actual files.

ANALYSIS CATEGORIES (be thorough across ALL of these):

1. Architecture & Design Patterns
   - Code organization (MVC, Clean, Hexagonal, etc.)
   - Repository/Factory/DI patterns
   - Key abstractions, interfaces, core types
   - Integration patterns (events, queues, APIs)

2. Error Handling & Logging
   - Error creation, wrapping, propagation patterns
   - Logging library and conventions
   - HTTP/API error response formats
   - Custom error types or codes

3. Security & Middleware
   - CORS settings, allowed origins/methods
   - Rate limiting configuration
   - Authentication patterns (JWT, sessions, cookies)
   - Request validation and sanitization

4. Performance & Resilience
   - Connection pool sizes, cache TTLs
   - Timeout values (request, DB, external calls)
   - Circuit breakers, retry policies, backoff

5. Data Models & Configuration
   - Database table/collection structures
   - API request/response types
   - Configuration structs with default values
   - Key interfaces and implementations

6. CI/CD & Build System
   - Build commands and toolchain
   - CI pipeline steps and permissions
   - Deployment targets and constraints
   - Release process

7. Dependencies & Technology Choices
   - Framework selections and rationale
   - Database drivers and ORMs
   - Testing frameworks
   - Notable library choices (observability, etc.)

8. Git History & Project Evolution
   - Major milestones and feature additions
   - Architecture changes over time
   - Active development areas

Extract architectural findings from the codebase. For each finding, identify:

FINDING TYPES (use exactly one):
- "decision"      — Architectural choice (framework, library, design pattern selection)
- "pattern"       — Recurring code pattern, convention, or workflow
- "constraint"    — Limitation, requirement, build rule, or security policy
- "feature"       — Product capability or user-facing functionality
- "metadata"      — Project metadata (git stats, build config, CI/CD setup)
- "documentation" — Documentation conventions (README structure, doc standards)

REQUIRED FIELDS:
- type: One of the types above
- title: Concise finding title
- description: Detailed explanation of what this finding represents
- confidence_score: 0.0 to 1.0 indicating how confident you are

RECOMMENDED FIELDS:
- why: Rationale for why this approach was chosen (especially for decisions/patterns)
- tradeoffs: Benefits and drawbacks of the approach
- evidence: Array of file references proving this finding exists
- metadata: Key-value pairs with additional context

EVIDENCE FIELDS:
- file_path: Relative path to the source file
- start_line / end_line: Line range where evidence exists
- snippet: Short relevant code excerpt
- grep_pattern: A regex pattern to find this evidence (for re-verification)
- evidence_type: "file" (default, source code) or "git" (git history/logs)

METADATA KEYS (include when relevant):
- "component": Category/scope (e.g., "auth", "api", "build", "testing")
- "severity": For constraints — "critical", "high", or "medium"
- "type": Sub-classification (e.g., "workflow" for pattern findings)
- "trigger": What triggers a workflow pattern
- "steps": Steps in a workflow pattern

DEBT CLASSIFICATION (assess for EVERY finding):
- debt_score: 0.0 (clean/intentional) to 1.0 (pure technical debt to be eliminated)
- debt_reason: Why this is considered technical debt
- refactor_hint: How to eliminate or improve this debt
Indicators of debt: TODO/FIXME/HACK comments, compatibility shims, workarounds, missing error handling

RELATIONSHIPS between findings (at the top level):
- from: Title of the source finding
- to: Title of the target finding
- relation: "depends_on", "affects", "extends", or "relates_to"
- reason: Why they are related
`)

	if existingContext != "" {
		sb.WriteString("\nExisting project context (avoid duplicating these findings):\n")
		sb.WriteString(existingContext)
		sb.WriteString("\n\n")
	}

	sb.WriteString(`Respond with ONLY a JSON object in this exact format:
{
  "findings": [
    {
      "type": "decision",
      "title": "Concise finding title",
      "description": "What this finding represents",
      "why": "Why this approach was chosen",
      "tradeoffs": "Benefits and drawbacks",
      "confidence_score": 0.85,
      "evidence": [
        {
          "file_path": "relative/path/to/file.go",
          "start_line": 10,
          "end_line": 25,
          "snippet": "relevant code snippet",
          "grep_pattern": "func NewService",
          "evidence_type": "file"
        }
      ],
      "metadata": {
        "component": "api"
      },
      "debt_score": 0.0,
      "debt_reason": "",
      "refactor_hint": ""
    }
  ],
  "relationships": [
    {
      "from": "Finding title A",
      "to": "Finding title B",
      "relation": "depends_on",
      "reason": "A requires B to function"
    }
  ]
}`)

	return sb.String()
}

// FocusAreas defines the parallel bootstrap analysis focus areas.
// Covers all FindingTypes: decisions, patterns, constraints, features, metadata, documentation.
var FocusAreas = []string{
	"architectural decisions, features, and technology choices - framework selections, design patterns, product capabilities, dependency choices (read go.mod/package.json), and rationale behind each decision",
	"code patterns, conventions, and implementation details - recurring patterns, error handling, logging, security middleware, performance configuration (timeouts, pools, caches), data model schemas, and development conventions",
	"constraints, CI/CD, and documentation - build requirements, deployment constraints, security policies, system limitations, CI/CD pipeline configuration (.github/workflows/), release process, and documentation standards",
	"git history and project evolution - run 'git log --oneline -150' to analyze major milestones, architecture changes, feature additions, active development areas, and foundational decisions from commit history",
}

// PlanGenerationPrompt builds a prompt for AI CLI to generate a task plan from a goal.
func PlanGenerationPrompt(goal string, projectContext string) string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "Generate a development plan to achieve this goal: %q\n\n", goal)

	if projectContext != "" {
		sb.WriteString("Project context (architectural decisions, patterns, constraints):\n")
		sb.WriteString(projectContext)
		sb.WriteString("\n\n")
	}

	sb.WriteString(`Create a plan with concrete, actionable tasks. Each task should be:
- Small enough for a single work session
- Have clear acceptance criteria
- Include validation steps (CLI commands to verify completion)
- Specify which type of agent should handle it (coder, qa, architect, researcher)
- Include scope and keywords for context retrieval

Order tasks by dependency - tasks that depend on others should come later.
Priority is 0-100 where lower numbers = higher priority.

Respond with ONLY a JSON object in this exact format:
{
  "goal_summary": "Concise summary (max 100 chars)",
  "rationale": "Why this plan is structured this way (min 20 chars)",
  "tasks": [
    {
      "title": "Action-oriented task title (max 200 chars)",
      "description": "Detailed task description (min 10 chars)",
      "priority": 10,
      "complexity": "low|medium|high",
      "assigned_agent": "coder|qa|architect|researcher",
      "acceptance_criteria": ["Criterion 1", "Criterion 2"],
      "validation_steps": ["go test ./...", "go build ./..."],
      "depends_on": [],
      "expected_files": ["path/to/file.go"],
      "scope": "api",
      "keywords": ["auth", "middleware", "jwt"]
    }
  ],
  "estimated_complexity": "low|medium|high",
  "prerequisites": ["Optional prerequisite 1"],
  "risk_factors": ["Optional risk 1"]
}`)

	return sb.String()
}

// TaskExecutionPrompt builds a prompt for AI CLI to implement a specific task.
func TaskExecutionPrompt(taskTitle, taskDescription string, acceptanceCriteria []string, contextSummary string, validationSteps []string) string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "Implement this task: %s\n\n", taskTitle)
	fmt.Fprintf(&sb, "Description: %s\n\n", taskDescription)

	if len(acceptanceCriteria) > 0 {
		sb.WriteString("Acceptance Criteria:\n")
		for _, ac := range acceptanceCriteria {
			fmt.Fprintf(&sb, "- %s\n", ac)
		}
		sb.WriteString("\n")
	}

	if contextSummary != "" {
		sb.WriteString("Project Context:\n")
		sb.WriteString(contextSummary)
		sb.WriteString("\n\n")
	}

	if len(validationSteps) > 0 {
		sb.WriteString("Validation Steps (run these to verify your work):\n")
		for _, vs := range validationSteps {
			fmt.Fprintf(&sb, "  %s\n", vs)
		}
		sb.WriteString("\n")
	}

	sb.WriteString(`Implement the changes needed to satisfy the acceptance criteria.
After making changes, run the validation steps to verify your work.

When you are done, respond with ONLY a JSON object summarizing what you did:
{
  "status": "completed|failed|partial",
  "summary": "Brief description of what was implemented",
  "files_modified": ["list", "of", "modified", "files"],
  "error": "Error message if status is failed"
}`)

	return sb.String()
}
