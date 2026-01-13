package task

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

// TaskStatus represents the lifecycle state of a task
type TaskStatus string

const (
	StatusDraft      TaskStatus = "draft"       // Initial creation, not ready for execution
	StatusPending    TaskStatus = "pending"     // Ready to be picked up by an agent
	StatusInProgress TaskStatus = "in_progress" // Agent is actively working
	StatusVerifying  TaskStatus = "verifying"   // Work done, running validation
	StatusCompleted  TaskStatus = "completed"   // Successfully verified
	StatusFailed     TaskStatus = "failed"      // Execution or verification failed
)

// PlanStatus represents the lifecycle state of a plan
type PlanStatus string

const (
	PlanStatusDraft         PlanStatus = "draft"          // Initial creation
	PlanStatusActive        PlanStatus = "active"         // Currently being executed
	PlanStatusCompleted     PlanStatus = "completed"      // All tasks done
	PlanStatusVerified      PlanStatus = "verified"       // Audit agent validated successfully
	PlanStatusNeedsRevision PlanStatus = "needs_revision" // Audit agent found issues after max retries
	PlanStatusArchived      PlanStatus = "archived"       // No longer active
)

// Task represents a discrete unit of work to be executed by an agent
type Task struct {
	ID                 string     `json:"id"`
	PlanID             string     `json:"planId"`
	Title              string     `json:"title"`
	Description        string     `json:"description"`
	Status             TaskStatus `json:"status"`
	Priority           int        `json:"priority"` // 0-100 (High to Low)
	AssignedAgent      string     `json:"assignedAgent"`
	ParentTaskID       string     `json:"parentTaskId,omitempty"`
	ContextSummary     string     `json:"contextSummary"` // AI-generated summary of linked nodes
	AcceptanceCriteria []string   `json:"acceptanceCriteria"`
	ValidationSteps    []string   `json:"validationSteps"` // CLI commands

	// AI integration fields - for MCP tool context fetching
	Scope                  string   `json:"scope,omitempty"`                  // e.g., "auth", "api", "vectorsearch"
	Keywords               []string `json:"keywords,omitempty"`               // Extracted from title/description
	SuggestedRecallQueries []string `json:"suggestedRecallQueries,omitempty"` // Pre-computed queries for recall tool

	// Session tracking - for AI tool state management
	ClaimedBy   string    `json:"claimedBy,omitempty"`   // Session ID that claimed this task
	ClaimedAt   time.Time `json:"claimedAt,omitempty"`   // When the task was claimed
	CompletedAt time.Time `json:"completedAt,omitempty"` // When the task was completed

	// Completion tracking
	CompletionSummary string   `json:"completionSummary,omitempty"` // AI-generated summary on completion
	FilesModified     []string `json:"filesModified,omitempty"`     // Files touched during task

	// Computed/Joined fields (not in tasks table directly)
	Dependencies []string `json:"dependencies"` // IDs of tasks
	ContextNodes []string `json:"contextNodes"` // IDs of knowledge nodes

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Validate checks if the task has all required fields and valid data.
func (t *Task) Validate() error {
	if strings.TrimSpace(t.Title) == "" {
		return fmt.Errorf("title required")
	}
	if len(t.Title) > 200 {
		return fmt.Errorf("title too long (max 200 chars)")
	}
	if strings.TrimSpace(t.Description) == "" {
		return fmt.Errorf("description required")
	}
	if t.Priority < 0 || t.Priority > 100 {
		return fmt.Errorf("priority must be between 0 and 100")
	}
	return nil
}

// AuditReport contains the results of an audit run
type AuditReport struct {
	Status         string    `json:"status"`         // "passed", "failed", "fixed"
	BuildOutput    string    `json:"buildOutput"`    // stdout/stderr from build command
	TestOutput     string    `json:"testOutput"`     // stdout/stderr from test command
	SemanticIssues []string  `json:"semanticIssues"` // Issues found by LLM analysis
	FixesApplied   []string  `json:"fixesApplied"`   // List of fixes that were auto-applied
	RetryCount     int       `json:"retryCount"`     // Number of fix attempts made
	CompletedAt    time.Time `json:"completedAt"`    // When the audit finished
	ErrorMessage   string    `json:"errorMessage"`   // Error if audit failed to run
}

// Plan represents a collection of tasks to achieve a high-level goal
type Plan struct {
	ID           string     `json:"id"`
	Goal         string     `json:"goal"`         // Concise summary for UI display (max 100 chars, AI-generated or truncated from user input)
	EnrichedGoal string     `json:"enrichedGoal"` // Full technical specification refined by Clarifying Agent
	Status       PlanStatus `json:"status"`       // draft, active, completed, verified, needs_revision, archived
	Tasks        []Task     `json:"tasks"`
	CreatedAt    time.Time  `json:"createdAt"`
	UpdatedAt    time.Time  `json:"updatedAt"`

	// Audit fields
	LastAuditReport string `json:"lastAuditReport,omitempty"` // JSON-serialized AuditReport
}

// scopeKeywords maps scope names to keywords that indicate them
var scopeKeywords = map[string][]string{
	"auth":         {"auth", "authentication", "login", "logout", "session", "cookie", "jwt", "token", "password", "credential", "oauth", "sso"},
	"api":          {"api", "endpoint", "handler", "route", "rest", "graphql", "grpc", "request", "response", "middleware"},
	"database":     {"database", "db", "sql", "sqlite", "postgres", "mysql", "migration", "schema", "query", "table", "index"},
	"vectorsearch": {"vector", "embedding", "lancedb", "similarity", "semantic", "search", "rag", "retrieval"},
	"llm":          {"llm", "openai", "claude", "gemini", "ollama", "prompt", "completion", "chat", "model", "inference"},
	"cli":          {"cli", "command", "flag", "cobra", "terminal", "argument", "subcommand"},
	"mcp":          {"mcp", "tool", "protocol", "context", "stdio", "jsonrpc"},
	"bootstrap":    {"bootstrap", "scan", "analyze", "extract", "discover", "pattern"},
	"ui":           {"ui", "tui", "interface", "display", "render", "bubbletea", "lipgloss"},
	"test":         {"test", "testing", "mock", "fixture", "assert", "benchmark", "coverage"},
}

// stopWords are common words to exclude from keyword extraction
var stopWords = map[string]bool{
	"the": true, "a": true, "an": true, "and": true, "or": true, "but": true,
	"in": true, "on": true, "at": true, "to": true, "for": true, "of": true,
	"with": true, "by": true, "from": true, "as": true, "is": true, "was": true,
	"are": true, "be": true, "been": true, "being": true, "have": true, "has": true,
	"had": true, "do": true, "does": true, "did": true, "will": true, "would": true,
	"could": true, "should": true, "may": true, "might": true, "must": true,
	"shall": true, "can": true, "need": true, "this": true, "that": true,
	"these": true, "those": true, "it": true, "its": true, "i": true, "we": true,
	"you": true, "he": true, "she": true, "they": true, "them": true, "their": true,
	"what": true, "which": true, "who": true, "whom": true, "when": true, "where": true,
	"why": true, "how": true, "all": true, "each": true, "every": true, "both": true,
	"few": true, "more": true, "most": true, "other": true, "some": true, "such": true,
	"no": true, "nor": true, "not": true, "only": true, "own": true, "same": true,
	"so": true, "than": true, "too": true, "very": true, "just": true, "also": true,
}

// EnrichAIFields populates Scope, Keywords, and SuggestedRecallQueries from title/description.
// Call this before CreateTask to ensure AI integration fields are set.
func (t *Task) EnrichAIFields() {
	// Extract keywords from title and description
	text := strings.ToLower(t.Title + " " + t.Description)

	// Remove punctuation and split into words
	re := regexp.MustCompile(`[^a-zA-Z0-9\s]`)
	text = re.ReplaceAllString(text, " ")
	words := strings.Fields(text)

	// First pass: collect ALL words >= 2 chars for scope matching (includes "db", "ui", etc.)
	allWords := make(map[string]bool)
	for _, word := range words {
		if len(word) >= 2 && !stopWords[word] {
			allWords[word] = true
		}
	}

	// Second pass: filter to >= 3 chars for keyword extraction (more meaningful terms)
	seen := make(map[string]bool)
	var keywords []string
	for _, word := range words {
		if len(word) < 3 {
			continue
		}
		if stopWords[word] {
			continue
		}
		if seen[word] {
			continue
		}
		seen[word] = true
		keywords = append(keywords, word)
	}

	// Limit to top 10 keywords (first ones tend to be most relevant)
	if len(keywords) > 10 {
		keywords = keywords[:10]
	}
	t.Keywords = keywords

	// Infer scope from ALL words (including 2-char like "db", "ui")
	scopeScores := make(map[string]int)
	for scope, scopeKws := range scopeKeywords {
		for _, kw := range scopeKws {
			if allWords[kw] {
				scopeScores[scope]++
			}
		}
	}

	// Find highest scoring scope
	bestScope := "general"
	bestScore := 0
	for scope, score := range scopeScores {
		if score > bestScore {
			bestScore = score
			bestScope = scope
		}
	}
	t.Scope = bestScope

	// Generate suggested recall queries
	var queries []string

	// Query 1: Scope-based patterns and constraints
	queries = append(queries, t.Scope+" patterns constraints decisions")

	// Query 2: Top keywords (up to 5)
	if len(keywords) > 0 {
		topKw := keywords
		if len(topKw) > 5 {
			topKw = topKw[:5]
		}
		queries = append(queries, strings.Join(topKw, " "))
	}

	// Query 3: Title-based (simplified)
	titleWords := strings.Fields(strings.ToLower(t.Title))
	var titleKw []string
	for _, w := range titleWords {
		w = re.ReplaceAllString(w, "")
		if len(w) >= 3 && !stopWords[w] {
			titleKw = append(titleKw, w)
		}
	}
	if len(titleKw) > 0 {
		if len(titleKw) > 4 {
			titleKw = titleKw[:4]
		}
		queries = append(queries, strings.Join(titleKw, " "))
	}

	t.SuggestedRecallQueries = queries
}
