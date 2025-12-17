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
- Use the CLI (`taskwing feature ...`, `taskwing decision ...`, `taskwing feature link ...`) as the write path.

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
type MemoryStore interface {
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

    // === Relationships (human-readable) ===
    Link(from, to, relationType string) error
    Unlink(from, to, relationType string) error
    GetDependencies(featureID string) ([]string, error)
    GetDependents(featureID string) ([]string, error)
    GetRelated(featureID string, maxDepth int) ([]string, error)
    FindPath(from, to string) ([]string, error)

    // === Cache Management ===
    RebuildIndex() error
    GetIndex() (*FeatureIndex, error)  // Returns cached if fresh

    // === Integrity ===
    Check() ([]Issue, error)
    Repair() error
}
```

---

## Write-Through Pattern

All mutations go through SQLite first, then propagate.

```go
func (s *Store) CreateFeature(f Feature) error {
    tx, _ := s.db.Begin()
    defer tx.Rollback()

    // 1. Insert to SQLite (source of truth)
    _, err := tx.Exec(`
        INSERT INTO features (id, name, one_liner, status, created_at, updated_at, tags, file_path)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?)
    `, f.ID, f.Name, f.OneLiner, f.Status, f.CreatedAt, f.UpdatedAt, f.Tags, f.FilePath)
    if err != nil {
        return fmt.Errorf("sqlite insert: %w", err)
    }

    // 2. Create markdown file
    if err := s.writeMarkdownFile(f); err != nil {
        return fmt.Errorf("markdown write: %w", err)
    }

    // 3. Commit transaction
    if err := tx.Commit(); err != nil {
        os.Remove(f.FilePath)  // Cleanup on failure
        return fmt.Errorf("commit: %w", err)
    }

    // 4. Invalidate index cache
    s.indexCache = nil

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

## Size Constraints

| Entity | Max Size |
|--------|----------|
| Feature.OneLiner | 100 chars |
| Decision.Summary | 200 chars |
| Feature file | 50KB |
