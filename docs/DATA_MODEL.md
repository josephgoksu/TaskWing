# TaskWing v2 — Data Model

> **Created:** 2025-12-15
> **Updated:** 2025-12-17
> **Status:** Active Development

---

## Design Principles

1. **Decisions are the moat** — Features change; decisions explain why
2. **Single source of truth** — SQLite holds all metadata
3. **Markdown for prose** — Human-readable, not canonical
4. **Index is a cache** — Regenerated from SQLite
5. **Graph-native** — Recursive CTEs for relationship traversal
6. **Keep it simple** — No magic inference in v2.0

---

## Storage Architecture

```
.taskwing/
├── memory/
│   ├── memory.db            # SQLite: THE source of truth
│   ├── index.json           # Cache: regenerated from SQLite
│   └── features/
│       ├── auth.md          # Generated markdown snapshot
│       └── users.md
└── .taskwing.yaml            # Optional project config (or use ~/.taskwing.yaml)
```

### Authority Hierarchy

| Storage | Role | Can Be Edited? |
|---------|------|----------------|
| `memory.db` | **Source of truth** for features, decisions, and relationships | Via CLI only |
| `features/*.md` | Human-readable snapshot (generated from SQLite) | **No** (manual edits are overwritten) |
| `index.json` | Performance cache | Never (regenerated) |

### Markdown Behavior (Current)

TaskWing currently **does not parse markdown back into SQLite**.

That means:
- `features/*.md` is generated for readability.
- Any manual edits may be overwritten the next time TaskWing updates that feature.
- Use the CLI (`taskwing add`, `taskwing delete`) as the write path.

### Sync (Planned)

Planned future work (not implemented in v2.0):
- `taskwing sync` to import markdown into SQLite
- Stable feature/decision IDs in markdown
- Relationship serialization in markdown to allow full rebuilds

---

## SQLite Schema

```sql
-- memory.db

CREATE TABLE features (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    one_liner TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'active',  -- active, deprecated, planned
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    tags TEXT,  -- JSON array: ["core", "auth"]
    file_path TEXT NOT NULL,
    decision_count INTEGER DEFAULT 0
);

CREATE TABLE decisions (
    id TEXT PRIMARY KEY,
    feature_id TEXT NOT NULL,
    title TEXT NOT NULL,
    summary TEXT NOT NULL,
    reasoning TEXT,
    tradeoffs TEXT,
    created_at TEXT NOT NULL,
    FOREIGN KEY (feature_id) REFERENCES features(id) ON DELETE CASCADE
);

CREATE TABLE edges (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    from_feature TEXT NOT NULL,
    to_feature TEXT NOT NULL,
    edge_type TEXT NOT NULL,  -- depends_on, extends, replaces, related
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (from_feature) REFERENCES features(id) ON DELETE CASCADE,
    FOREIGN KEY (to_feature) REFERENCES features(id) ON DELETE CASCADE,
    UNIQUE(from_feature, to_feature, edge_type)
);

-- Indexes for graph traversal
CREATE INDEX idx_edges_from ON edges(from_feature);
CREATE INDEX idx_edges_to ON edges(to_feature);
CREATE INDEX idx_edges_type ON edges(edge_type);
CREATE INDEX idx_decisions_feature ON decisions(feature_id);

-- New knowledge graph tables (v2 pivot)
CREATE TABLE nodes (
    id TEXT PRIMARY KEY,
    content TEXT NOT NULL,              -- Original text input
    type TEXT,                          -- AI-inferred: decision, feature, plan, note
    summary TEXT,                       -- AI-extracted title/summary
    source_agent TEXT DEFAULT '',       -- Agent that created this node (doc, code, git, deps)
    embedding BLOB,                     -- Vector for similarity search
    created_at TEXT NOT NULL,
    -- Evidence-Based Verification fields (v2.1+)
    verification_status TEXT DEFAULT 'pending_verification',  -- pending_verification, verified, partial, rejected, skipped
    evidence TEXT,                      -- JSON: [{file_path, start_line, end_line, snippet, grep_pattern}]
    verification_result TEXT,           -- JSON: {status, evidence_results, confidence_adjustment, verified_at, verifier_version}
    confidence_score REAL DEFAULT 0.5   -- Numeric confidence 0.0-1.0
);

CREATE TABLE node_edges (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    from_node TEXT NOT NULL,
    to_node TEXT NOT NULL,
    relation TEXT NOT NULL,             -- relates_to, depends_on, affects, etc.
    properties TEXT,                    -- JSON for arbitrary metadata
    confidence REAL DEFAULT 1.0,        -- AI confidence score
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (from_node) REFERENCES nodes(id) ON DELETE CASCADE,
    FOREIGN KEY (to_node) REFERENCES nodes(id) ON DELETE CASCADE,
    UNIQUE(from_node, to_node, relation)
);

-- Indexes for new tables
CREATE INDEX idx_nodes_type ON nodes(type);
CREATE INDEX idx_nodes_source_agent ON nodes(source_agent);
CREATE INDEX idx_nodes_summary_agent ON nodes(summary, source_agent);
```

> **Note:** In v2.0, `decision_count` is maintained by application code (no SQLite triggers).

---

## Index.json (Cache)

Generated from SQLite, never edited directly.

```json
{
  "features": [
    {
      "id": "f-001",
      "name": "Auth",
      "oneLiner": "User authentication and authorization",
      "status": "active",
      "decisionCount": 3
    }
  ],
  "lastUpdated": "2025-12-16T10:00:00Z"
}
```

**Regeneration:**
```go
func (s *Store) RebuildIndex() error {
    features, _ := s.db.Query("SELECT id, name, one_liner, status, decision_count FROM features")
    index := FeatureIndex{Features: features, LastUpdated: time.Now()}
    return writeJSON(".taskwing/memory/index.json", index)
}
```

---

## Feature Markdown (Prose Only)

No frontmatter metadata. Just human-readable content.

```markdown
# Auth

User authentication and authorization system. Handles login, registration, OAuth2, and session management.

## Decisions

### Use JWT over sessions
- **Summary:** Stateless JWT tokens instead of server sessions
- **Why:** Easier horizontal scaling, no session store needed
- **Trade-offs:** Need token refresh logic, larger payload
- **Date:** 2024-01-10

### OAuth2 for social login
- **Summary:** Support Google and GitHub OAuth2
- **Why:** Reduce friction for user registration
- **Date:** 2024-02-15

## Notes

Consider adding passkey support in future iteration.
```

**On read:** Load feature metadata from SQLite, load prose content from markdown file.

---

## Unified Store Interface

```go
type Repository interface {
    // === Feature CRUD (atomic: SQLite + markdown + index) ===
    CreateFeature(f Feature) error
    UpdateFeature(f Feature) error
    DeleteFeature(id string) error
    GetFeature(id string) (*Feature, error)
    ListFeatures() ([]FeatureSummary, error)

    // === Decision CRUD ===
    AddDecision(featureID string, d Decision) error
    UpdateDecision(d Decision) error
    DeleteDecision(id string) error
    GetDecisions(featureID string) ([]Decision, error)

    // === Relationships ===
    Link(from, to, relationType string) error
    Unlink(from, to, relationType string) error
    GetDependencies(featureID string) ([]string, error)
    GetDependents(featureID string) ([]string, error)
    GetRelated(featureID string, maxDepth int) ([]string, error)

    // === Node Access (Knowledge Graph) ===
    CreateNode(n Node) error
    GetNode(id string) (*Node, error)
    ListNodes(filter string) ([]Node, error)
    UpdateNodeEmbedding(id string, embedding []float32) error
    DeleteNode(id string) error
    DeleteNodesByAgent(agent string) error

    // === Cache Management ===
    GetIndex() (*FeatureIndex, error)

    // === Integrity ===
    Check() ([]Issue, error)
    Repair() error
}
```

---

## Write-Through Pattern

All mutations go through SQLite first, then propagate.

```go
func (r *Repository) CreateFeature(f Feature) error {
    // 1. Save to DB (Primary)
    if err := r.db.CreateFeature(f); err != nil {
        return fmt.Errorf("db create: %w", err)
    }

    // 2. Fetch decisions (usually empty on create)
    decisions, _ := r.db.GetDecisions(f.ID)

    // 3. Save to File (Secondary)
    if err := r.files.WriteFeature(f, decisions); err != nil {
        // Compensating transaction: undo DB change
        _ = r.db.DeleteFeature(f.ID)
        return fmt.Errorf("file create: %w", err)
    }

    return nil
}
```

---

## Graph Queries

**Get all dependencies (recursive):**
```sql
WITH RECURSIVE deps AS (
    SELECT to_feature, 1 as depth
    FROM edges
    WHERE from_feature = ? AND edge_type = 'depends_on'
    UNION ALL
    SELECT e.to_feature, d.depth + 1
    FROM edges e
    JOIN deps d ON e.from_feature = d.to_feature
    WHERE e.edge_type = 'depends_on' AND d.depth < 10
)
SELECT DISTINCT to_feature FROM deps;
```

**Get all dependents:**
```sql
WITH RECURSIVE dependents AS (
    SELECT from_feature, 1 as depth
    FROM edges
    WHERE to_feature = ? AND edge_type = 'depends_on'
    UNION ALL
    SELECT e.from_feature, d.depth + 1
    FROM edges e
    JOIN dependents d ON e.to_feature = d.from_feature
    WHERE e.edge_type = 'depends_on' AND d.depth < 10
)
SELECT DISTINCT from_feature FROM dependents;
```

---

## Edge Types

| Type | Meaning |
|------|---------|
| `depends_on` | A requires B to function |
| `extends` | A adds capabilities to B |
| `replaces` | A supersedes B (migration) |
| `related` | Loose association |

---

## Integrity Checks

```go
func (s *Store) Check() ([]Issue, error) {
    var issues []Issue

    // Check: every feature has a markdown file
    features, _ := s.db.Query("SELECT id, file_path FROM features")
    for _, f := range features {
        if _, err := os.Stat(f.FilePath); os.IsNotExist(err) {
            issues = append(issues, Issue{Type: "missing_file", FeatureID: f.ID})
        }
    }

    // Check: every edge references existing features
    edges, _ := s.db.Query("SELECT from_feature, to_feature FROM edges")
    for _, e := range edges {
        if !s.featureExists(e.From) {
            issues = append(issues, Issue{Type: "orphan_edge", From: e.From})
        }
    }

    // Check: index.json matches SQLite
    // ...

    return issues, nil
}
```

---

## Evidence-Based Verification (v2.1+)

All findings from agents must include **evidence** — file paths, line numbers, and code snippets that support each claim. This prevents LLM hallucinations and enables automated verification.

### Evidence Structure

```json
{
  "file_path": "internal/api/handler.go",
  "start_line": 45,
  "end_line": 52,
  "snippet": "func NewHandler(db *sql.DB) *Handler {\n    return &Handler{db: db}\n}",
  "grep_pattern": "func NewHandler"
}
```

### Verification Status

| Status | Meaning |
|--------|---------|
| `pending_verification` | Finding not yet verified |
| `verified` | All evidence confirmed |
| `partial` | Some evidence confirmed |
| `rejected` | Evidence could not be confirmed (finding discarded) |
| `skipped` | No evidence to verify |

### Confidence Scores

Confidence is a numeric value between 0.0 and 1.0:

| Range | Label | Meaning |
|-------|-------|---------|
| 0.9-1.0 | High | Direct evidence (exact code match) |
| 0.7-0.89 | Medium-High | Strong inference (pattern clearly visible) |
| 0.5-0.69 | Medium | Reasonable inference (based on conventions) |
| Below 0.5 | Low | Weak inference (speculation — avoid) |

### Verification Pipeline

```
┌──────────────────────────────────────────────────────────────────────┐
│                     VERIFICATION PIPELINE                            │
├──────────────────────────────────────────────────────────────────────┤
│                                                                      │
│   Agent Output   →   VerificationAgent   →   Filtered Findings       │
│   (with evidence)    (deterministic)         (verified only)         │
│                                                                      │
│   Checks performed per evidence item:                                │
│   1. FileExists: Does the file exist at the cited path?              │
│   2. SnippetFound: Is the snippet present anywhere in the file?      │
│   3. LineNumbersMatch: Does content at specified lines match?        │
│   4. SimilarityScore: Fuzzy match fallback (Jaccard similarity)      │
│                                                                      │
│   Confidence Adjustment:                                             │
│   • Verified: +0.1                                                   │
│   • Partial: 0 to -0.1                                               │
│   • Rejected: -0.3                                                   │
└──────────────────────────────────────────────────────────────────────┘
```

### Future: Semantic LLM Verification

After deterministic verification, a future phase will add **semantic verification** using an LLM to:
- Validate that the snippet actually supports the claimed decision
- Check if the reasoning makes sense given the code context
- Detect outdated evidence (code changed but finding not updated)

This is planned for v2.2+ and documented in [ROADMAP.md](./ROADMAP.md).

---

## Size Constraints

| Entity | Max Size |
|--------|----------|
| Feature.OneLiner | 100 chars |
| Decision.Summary | 200 chars |
| Feature file | 50KB |
