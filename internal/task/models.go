package task

import (
	"encoding/json"
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
	StatusBlocked    TaskStatus = "blocked"     // Waiting on dependencies
	StatusReady      TaskStatus = "ready"       // Dependencies met, ready for execution
)

// PhaseStatus represents the lifecycle state of a phase
type PhaseStatus string

const (
	PhaseStatusPending  PhaseStatus = "pending"  // Not yet expanded into tasks
	PhaseStatusExpanded PhaseStatus = "expanded" // Tasks have been generated
	PhaseStatusSkipped  PhaseStatus = "skipped"  // User decided to skip this phase
)

// GenerationMode indicates how a plan was generated
type GenerationMode string

const (
	GenerationModeBatch       GenerationMode = "batch"       // All tasks generated at once (legacy)
	GenerationModeInteractive GenerationMode = "interactive" // Staged workflow with phases
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

// Phase represents a high-level work chunk in an interactive plan.
// Phases are created during the "decompose" stage and expanded into tasks during "expand".
type Phase struct {
	ID            string      `json:"id"`
	PlanID        string      `json:"plan_id"`
	Title         string      `json:"title"`
	Description   string      `json:"description"`
	Rationale     string      `json:"rationale"`      // Why this phase exists and what it achieves
	OrderIndex    int         `json:"order_index"`    // Sequence in the plan (0-based)
	Status        PhaseStatus `json:"status"`         // pending, expanded, skipped
	ExpectedTasks int         `json:"expected_tasks"` // Estimated number of tasks (from decomposition)
	CreatedAt     time.Time   `json:"created_at"`
	UpdatedAt     time.Time   `json:"updated_at"`

	// Computed/joined fields (not stored directly)
	Tasks []Task `json:"tasks,omitempty"` // Tasks belonging to this phase (when loaded)
}

// Validate checks if the phase has all required fields and valid data.
func (p *Phase) Validate() error {
	if strings.TrimSpace(p.Title) == "" {
		return fmt.Errorf("phase title required")
	}
	if len(p.Title) > 200 {
		return fmt.Errorf("phase title too long (max 200 chars)")
	}
	if p.OrderIndex < 0 {
		return fmt.Errorf("phase order_index must be >= 0")
	}
	if p.ExpectedTasks < 0 {
		return fmt.Errorf("phase expected_tasks must be >= 0")
	}
	return nil
}

// PlanDraftState stores the intermediate state during interactive plan generation.
// This enables resume capability if the user stops midway through the workflow.
type PlanDraftState struct {
	CurrentStage    string   `json:"current_stage"`             // "clarify", "decompose", "expand", "finalize"
	CurrentPhaseIdx int      `json:"current_phase_idx"`         // Which phase is being expanded (0-based)
	EnrichedGoal    string   `json:"enriched_goal,omitempty"`   // From clarify stage
	ClarifyHistory  string   `json:"clarify_history,omitempty"` // Q&A history
	PhasesFeedback  []string `json:"phases_feedback,omitempty"` // User feedback on phases
	LastUpdated     string   `json:"last_updated"`              // ISO8601 timestamp
}

// ClarifySessionState represents the lifecycle state of a clarify session.
type ClarifySessionState string

const (
	ClarifySessionStateNew              ClarifySessionState = "new_session"
	ClarifySessionStateAwaitingAnswers  ClarifySessionState = "awaiting_answers"
	ClarifySessionStateReadyToPlan      ClarifySessionState = "ready_to_plan"
	ClarifySessionStateMaxRoundsReached ClarifySessionState = "max_rounds_exceeded"
)

// ClarifySession stores persisted state for a multi-round clarification loop.
type ClarifySession struct {
	ID                   string              `json:"id"`
	Goal                 string              `json:"goal"`
	EnrichedGoal         string              `json:"enriched_goal,omitempty"`
	GoalSummary          string              `json:"goal_summary,omitempty"`
	State                ClarifySessionState `json:"state"`
	RoundIndex           int                 `json:"round_index"`
	MaxRounds            int                 `json:"max_rounds"`
	MaxQuestionsPerRound int                 `json:"max_questions_per_round"`
	CurrentQuestions     []string            `json:"current_questions,omitempty"`
	IsReadyToPlan        bool                `json:"is_ready_to_plan"`
	LastContextUsed      string              `json:"last_context_used,omitempty"`
	CreatedAt            time.Time           `json:"created_at"`
	UpdatedAt            time.Time           `json:"updated_at"`
}

// ClarifyTurn stores one round of clarify interaction.
type ClarifyTurn struct {
	ID               string    `json:"id"`
	SessionID        string    `json:"session_id"`
	RoundIndex       int       `json:"round_index"`
	Questions        []string  `json:"questions,omitempty"`
	Answers          []string  `json:"answers,omitempty"`
	GoalSummary      string    `json:"goal_summary,omitempty"`
	EnrichedGoal     string    `json:"enriched_goal,omitempty"`
	IsReadyToPlan    bool      `json:"is_ready_to_plan"`
	AutoAnswered     bool      `json:"auto_answered"`
	MaxRoundsReached bool      `json:"max_rounds_reached"`
	ContextSummary   string    `json:"context_summary,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
}

// Task represents a discrete unit of work to be executed by an agent
type Task struct {
	ID                 string     `json:"id"`
	PlanID             string     `json:"plan_id"`
	PhaseID            string     `json:"phase_id,omitempty"` // Optional: links task to a phase (interactive mode)
	Title              string     `json:"title"`
	Description        string     `json:"description"`
	Status             TaskStatus `json:"status"`
	Priority           int        `json:"priority"`   // 0-100 (High to Low)
	Complexity         string     `json:"complexity"` // "low", "medium", "high"
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
	FilesModified     []string `json:"filesModified,omitempty"`     // Files touched during task (actual)

	// Sentinel tracking - for deviation detection
	ExpectedFiles []string `json:"expectedFiles,omitempty"` // Files plan says should be modified (predicted)
	GitBaseline   []string `json:"gitBaseline,omitempty"`   // Files already modified when task started (for accurate diff)

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

type taskAlias Task

// UnmarshalJSON enforces strict snake_case payloads.
func (t *Task) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if _, hasLegacyPlanID := raw["planId"]; hasLegacyPlanID {
		return fmt.Errorf("planId is no longer supported; use plan_id")
	}

	var aux taskAlias
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	*t = Task(aux)
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
	TaskCount    int        `json:"taskCount,omitempty"` // Precomputed count for list views (avoids loading all tasks)
	CreatedAt    time.Time  `json:"createdAt"`
	UpdatedAt    time.Time  `json:"updatedAt"`

	// Audit fields
	LastAuditReport string `json:"lastAuditReport,omitempty"` // JSON-serialized AuditReport

	// Interactive generation fields (Phase-based workflow)
	Phases         []Phase         `json:"phases,omitempty"`          // High-level phases (interactive mode only)
	DraftState     *PlanDraftState `json:"draft_state,omitempty"`     // Intermediate state for resume capability
	GenerationMode GenerationMode  `json:"generation_mode,omitempty"` // "batch" or "interactive"
}

// GetTaskCount returns the number of tasks in this plan.
// It uses TaskCount if set (from ListPlans), otherwise falls back to len(Tasks).
// This handles both cases: ListPlans (which sets TaskCount but not Tasks)
// and GetPlanWithTasks (which populates Tasks but not TaskCount).
func (p *Plan) GetTaskCount() int {
	if p.TaskCount > 0 {
		return p.TaskCount
	}
	return len(p.Tasks)
}

// Scope customization is available via .taskwing.yaml or ~/.taskwing/config.yaml:
//
//	task:
//	  scopes:
//	    custom_scope:
//	      - keyword1
//	      - keyword2
//	  maxKeywords: 15  # default: 10
//	  minWordLength: 4 # default: 3
//
// See scope_config.go for the implementation.

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
//
// This is part of the early binding context strategy - see docs/architecture/ADR_CONTEXT_BINDING.md
//
// Algorithm Overview:
// 1. KEYWORD EXTRACTION:
//   - Combine title and description into lowercase text
//   - Remove punctuation, split into words
//   - Filter out stop words (common English words like "the", "and", etc.)
//   - Keep words >= minWordLength (default: 3 chars) for keywords
//   - Limit to maxKeywords (default: 10) to keep context focused
//
// 2. SCOPE INFERENCE:
//   - Collect ALL words >= 2 chars (to match abbreviations like "db", "ui")
//   - For each configured scope, count keyword matches
//   - Highest-scoring scope wins; defaults to "general" if no matches
//   - Scopes are configurable via task.scopes in .taskwing.yaml
//
// 3. RECALL QUERY GENERATION:
//   - Query 1: "<scope> patterns constraints decisions" - domain-specific architecture
//   - Query 2: Top 5 keywords joined - content-specific search
//   - Query 3: Simplified title words - intent-focused search
//
// Configuration (in .taskwing.yaml or ~/.taskwing/config.yaml):
//
//	task:
//	  scopes:
//	    custom_domain:
//	      - keyword1
//	      - keyword2
//	  maxKeywords: 15   # default: 10
//	  minWordLength: 4  # default: 3
func (t *Task) EnrichAIFields() {
	cfg := GetScopeConfig()

	// Extract keywords from title and description
	text := strings.ToLower(t.Title + " " + t.Description)

	// Remove punctuation and split into words
	re := regexp.MustCompile(`[^a-zA-Z0-9\s]`)
	text = re.ReplaceAllString(text, " ")
	words := strings.Fields(text)

	// First pass: collect ALL words >= minWordLenScope (2) for scope matching
	// This catches abbreviations like "db", "ui", "ai"
	allWords := make(map[string]bool)
	minLenScope := cfg.MinWordLengthForScope()
	for _, word := range words {
		if len(word) >= minLenScope && !stopWords[word] {
			allWords[word] = true
		}
	}

	// Second pass: filter to >= minWordLength (default 3) for keyword extraction
	minLen := cfg.MinWordLength()
	seen := make(map[string]bool)
	var keywords []string
	for _, word := range words {
		if len(word) < minLen {
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

	// Limit to configured max keywords (first ones tend to be most relevant)
	maxKw := cfg.MaxKeywords()
	if len(keywords) > maxKw {
		keywords = keywords[:maxKw]
	}
	effectiveKeywords := t.Keywords
	if len(effectiveKeywords) == 0 {
		effectiveKeywords = keywords
		t.Keywords = keywords
	}

	// Infer scope using configurable scope keywords
	effectiveScope := t.Scope
	if effectiveScope == "" {
		effectiveScope = cfg.InferScope(allWords)
		t.Scope = effectiveScope
	}

	// Generate suggested recall queries
	var queries []string

	// Query 1: Scope-based patterns and constraints
	queries = append(queries, effectiveScope+" patterns constraints decisions")

	// Query 2: Top keywords (up to 5)
	if len(effectiveKeywords) > 0 {
		topKw := effectiveKeywords
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
		if len(w) >= minLen && !stopWords[w] {
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
