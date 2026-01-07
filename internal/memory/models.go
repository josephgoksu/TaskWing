package memory

import "time"

// Feature represents a major component or capability in the codebase.
// Features are the primary organizing unit for project memory.
type Feature struct {
	ID            string    `json:"id"`             // Unique identifier (f-xxx)
	Name          string    `json:"name"`           // Human-readable name (e.g., "Auth")
	OneLiner      string    `json:"oneLiner"`       // Brief description (max 100 chars)
	Status        string    `json:"status"`         // active, deprecated, planned
	Tags          []string  `json:"tags,omitempty"` // Categorization tags
	FilePath      string    `json:"filePath"`       // Path to markdown file
	DecisionCount int       `json:"decisionCount"`  // Cached count of decisions
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

// FeatureSummary is a lightweight view of a feature for listings.
type FeatureSummary struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	OneLiner      string `json:"oneLiner"`
	Status        string `json:"status"`
	DecisionCount int    `json:"decisionCount"`
}

// Decision captures an architectural or technical decision made for a feature.
// Decisions are the core value proposition - they explain WHY things exist.
type Decision struct {
	ID        string    `json:"id"`                  // Unique identifier (d-xxx)
	FeatureID string    `json:"featureId"`           // Parent feature
	Title     string    `json:"title"`               // Decision title (max 100 chars)
	Summary   string    `json:"summary"`             // Brief summary (max 200 chars)
	Reasoning string    `json:"reasoning,omitempty"` // Why this decision was made
	Tradeoffs string    `json:"tradeoffs,omitempty"` // Known tradeoffs
	CreatedAt time.Time `json:"createdAt"`
}

// Edge represents a relationship between two features.
// Edges form a directed graph enabling dependency analysis.
type Edge struct {
	ID          int64     `json:"id"`          // Auto-increment ID
	FromFeature string    `json:"fromFeature"` // Source feature ID
	ToFeature   string    `json:"toFeature"`   // Target feature ID
	EdgeType    string    `json:"edgeType"`    // depends_on, extends, replaces, related
	CreatedAt   time.Time `json:"createdAt"`
}

// EdgeType constants for relationship types.
const (
	EdgeTypeDependsOn = "depends_on" // A requires B to function
	EdgeTypeExtends   = "extends"    // A adds capabilities to B
	EdgeTypeReplaces  = "replaces"   // A supersedes B (migration)
	EdgeTypeRelated   = "related"    // Loose association
)

// FeatureStatus constants.
const (
	FeatureStatusActive     = "active"
	FeatureStatusDeprecated = "deprecated"
	FeatureStatusPlanned    = "planned"
)

// FeatureIndex is the cached summary of all features for quick MCP context loading.
type FeatureIndex struct {
	Features    []FeatureSummary `json:"features"`
	LastUpdated time.Time        `json:"lastUpdated"`
}

// Issue represents a problem found during integrity checks.
type Issue struct {
	Type      string `json:"type"`                // missing_file, orphan_edge, index_mismatch
	FeatureID string `json:"featureId,omitempty"` // Related feature if applicable
	Message   string `json:"message"`             // Human-readable description
}

// === New Knowledge Graph Types (v2 pivot) ===

// Node represents a piece of knowledge in the graph.
// Nodes are the universal storage unit - AI classifies them at write-time.
type Node struct {
	ID          string    `json:"id"`                    // Unique identifier (n-xxx)
	Content     string    `json:"content"`               // Original text input
	Type        string    `json:"type,omitempty"`        // AI-inferred: decision, feature, plan, note
	Summary     string    `json:"summary,omitempty"`     // AI-extracted title/summary
	SourceAgent string    `json:"sourceAgent,omitempty"` // Agent that created this node (doc, code, git, deps)
	Embedding   []float32 `json:"embedding,omitempty"`   // Vector for similarity search
	CreatedAt   time.Time `json:"createdAt"`

	// Evidence-Based Verification fields (v2.1+)
	// These support the verification pipeline that validates agent findings

	// VerificationStatus tracks validation state: pending_verification, verified, partial, rejected, skipped
	VerificationStatus string `json:"verificationStatus,omitempty"`

	// Evidence is JSON-serialized []Evidence containing file:line references and snippets
	Evidence string `json:"evidence,omitempty"`

	// VerificationResult is JSON-serialized VerificationResult with detailed check outcomes
	VerificationResult string `json:"verificationResult,omitempty"`

	// ConfidenceScore is numeric confidence (0.0-1.0) adjusted by verification
	ConfidenceScore float64 `json:"confidenceScore,omitempty"`
}

// NodeEdge represents a relationship between two nodes.
type NodeEdge struct {
	ID         int64          `json:"id"`                   // Auto-increment ID
	FromNode   string         `json:"fromNode"`             // Source node ID
	ToNode     string         `json:"toNode"`               // Target node ID
	Relation   string         `json:"relation"`             // relates_to, depends_on, affects, etc.
	Properties map[string]any `json:"properties,omitempty"` // Arbitrary JSON metadata
	Confidence float64        `json:"confidence"`           // AI confidence score (0.0-1.0)
	CreatedAt  time.Time      `json:"createdAt"`
}

// NodeType constants for classification.
const (
	NodeTypeDecision   = "decision"
	NodeTypeFeature    = "feature"
	NodeTypePlan       = "plan"
	NodeTypeNote       = "note"
	NodeTypeUnknown    = "unknown"
	NodeTypeConstraint = "constraint" // For mandatory architectural rules
	NodeTypePattern    = "pattern"    // For recurring patterns and workflows
)

// NodeRelation constants for edge types.
const (
	NodeRelationDependsOn           = "depends_on"
	NodeRelationRelatesTo           = "relates_to"
	NodeRelationAffects             = "affects"
	NodeRelationExtends             = "extends"
	NodeRelationSemanticallySimilar = "semantically_similar"
	NodeRelationSharesEvidence      = "shares_evidence" // Nodes referencing same files
)

// ProjectOverview represents the high-level description of a project.
// It provides context for AI assistants about what the project does.
type ProjectOverview struct {
	ShortDescription string    `json:"short_description"` // One-sentence summary (max ~100 chars)
	LongDescription  string    `json:"long_description"`  // Detailed description (2-3 paragraphs)
	GeneratedAt      time.Time `json:"generated_at"`      // When the overview was auto-generated
	LastEditedAt     time.Time `json:"last_edited_at"`    // When manually edited (zero if never)
}
