# TaskWing Architecture

> **Version:** 2.1
> **Updated:** 2025-12-26
> **Audience:** Developers contributing to or integrating with TaskWing

---

## Table of Contents

1. [Overview](#overview)
2. [High-Level Architecture](#high-level-architecture)
3. [Core Concepts](#core-concepts)
4. [The Bootstrap Pipeline](#the-bootstrap-pipeline)
5. [Evidence-Based Verification](#evidence-based-verification)
6. [Knowledge Graph](#knowledge-graph)
7. [Package Structure](#package-structure)
8. [Data Flow Examples](#data-flow-examples)
9. [Extension Points](#extension-points)
10. [Tech Stack](#tech-stack)

---

## Overview

### What is TaskWing?

TaskWing is a **knowledge extraction and context layer** for codebases. It analyzes your repository to build a queryable knowledge graph of:

- **Features** — What the product does
- **Decisions** — Why things are built a certain way
- **Patterns** — Recurring architectural solutions
- **Constraints** — Rules that must be followed

### Why Does This Matter?

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           THE PROBLEM                                       │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│   AI Coding Assistants (Claude, Cursor, Copilot) are powerful but:          │
│                                                                             │
│   ❌ They don't know WHY your code is structured a certain way              │
│   ❌ They can't see architectural decisions made 6 months ago               │
│   ❌ They suggest patterns that violate your team's conventions             │
│   ❌ They lack context about constraints ("always use read replica")        │
│                                                                             │
├─────────────────────────────────────────────────────────────────────────────┤
│                           THE SOLUTION                                      │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│   TaskWing extracts and serves this context:                                │
│                                                                             │
│   ✅ Analyzes docs, code, git history, and dependencies                     │
│   ✅ Builds a knowledge graph with semantic search                          │
│   ✅ Serves context via MCP (Model Context Protocol)                        │
│   ✅ Verifies all findings against actual code (no hallucinations)          │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### One-Line Summary

> **TaskWing turns your codebase into a queryable knowledge base that AI tools can use for context.**

---

## High-Level Architecture

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              USER INTERFACES                                │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│   ┌─────────────┐    ┌─────────────┐    ┌─────────────┐                     │
│   │   CLI (tw)  │    │ MCP Server  │    │  (Future)   │                     │
│   │             │    │             │    │ Web Dashboard│                    │
│   │ • bootstrap │    │ • project-  │    │             │                     │
│   │ • context   │    │   context   │    │             │                     │
│   │ • add/list  │    │   tool      │    │             │                     │
│   └──────┬──────┘    └──────┬──────┘    └─────────────┘                     │
│          │                  │                                               │
│          └────────┬─────────┘                                               │
│                   ▼                                                         │
├─────────────────────────────────────────────────────────────────────────────┤
│                           INTELLIGENCE LAYER                                │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│   ┌─────────────────────────────────────────────────────────────────────┐   │
│   │                      ANALYSIS AGENTS                                │   │
│   │                                                                     │   │
│   │   ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐           │   │
│   │   │ DocAgent │  │CodeAgent │  │ GitAgent │  │DepsAgent │           │   │
│   │   │ (docs)   │  │ (code)   │  │ (commits)│  │(deps)    │           │   │
│   │   └──────────┘  └──────────┘  └──────────┘  └──────────┘           │   │
│   │                                                                     │   │
│   └─────────────────────────────┬───────────────────────────────────────┘   │
│                                 │                                           │
│                                 ▼                                           │
│   ┌─────────────────────────────────────────────────────────────────────┐   │
│   │                   VERIFICATION AGENT                                │   │
│   │                                                                     │   │
│   │   • Checks file existence        • Validates line numbers          │   │
│   │   • Verifies code snippets       • Rejects hallucinations          │   │
│   │                                                                     │   │
│   └─────────────────────────────┬───────────────────────────────────────┘   │
│                                 │                                           │
│                                 ▼                                           │
├─────────────────────────────────────────────────────────────────────────────┤
│                            KNOWLEDGE LAYER                                  │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│   ┌─────────────────────────────────────────────────────────────────────┐   │
│   │                    KNOWLEDGE SERVICE                                │   │
│   │                                                                     │   │
│   │   • Semantic search (embeddings + FTS5)                             │   │
│   │   • RAG answers ("tw context --answer")                             │   │
│   │   • Graph linking (co-occurrence, semantic similarity)              │   │
│   │                                                                     │   │
│   └─────────────────────────────┬───────────────────────────────────────┘   │
│                                 │                                           │
│                                 ▼                                           │
├─────────────────────────────────────────────────────────────────────────────┤
│                            STORAGE LAYER                                    │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│   ┌─────────────────────────────────────────────────────────────────────┐   │
│   │                         SQLite                                      │   │
│   │                   (Single Source of Truth)                          │   │
│   │                                                                     │   │
│   │   Tables: nodes, node_edges, features, decisions, patterns          │   │
│   │   Location: .taskwing/memory/memory.db                              │   │
│   │                                                                     │   │
│   └─────────────────────────────────────────────────────────────────────┘   │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## Core Concepts

### 1. Findings

A **Finding** is a piece of knowledge extracted by an agent:

```go
type Finding struct {
    Type        FindingType  // decision, feature, pattern, constraint
    Title       string       // "Use JWT for authentication"
    Description string       // What was decided/implemented
    Why         string       // Reasoning behind the decision
    Tradeoffs   string       // Known tradeoffs

    // Evidence (required for verification)
    Evidence           []Evidence          // File:line references
    ConfidenceScore    float64             // 0.0-1.0
    VerificationStatus VerificationStatus  // pending, verified, rejected

    SourceAgent string         // Which agent produced this
    Metadata    map[string]any // Agent-specific data
}
```

### 2. Evidence

Every finding must include **evidence** — proof from the codebase:

```go
type Evidence struct {
    FilePath    string  // "internal/auth/jwt.go"
    StartLine   int     // 45
    EndLine     int     // 52
    Snippet     string  // Actual code from the file
    GrepPattern string  // Pattern to verify snippet exists
}
```

**Why evidence matters:**
- LLMs can hallucinate (invent files/code that don't exist)
- Evidence enables automated verification
- Users can click through to see the actual code

### 3. Nodes

Findings are stored as **Nodes** in the knowledge graph:

```go
type Node struct {
    ID                 string    // Unique identifier
    Type               string    // decision, feature, pattern, constraint
    Summary            string    // Short title
    Content            string    // Full description
    SourceAgent        string    // Agent that created this

    // Verification
    VerificationStatus string    // pending_verification, verified, rejected
    Evidence           string    // JSON: [{file_path, snippet, ...}]
    ConfidenceScore    float64   // 0.0-1.0 (adjusted by verification)

    // Embeddings for semantic search
    Embedding          []float32 // Vector from OpenAI
}
```

### 4. Verification Status

| Status | Meaning | Action |
|--------|---------|--------|
| `pending_verification` | Not yet checked | Will be verified on ingest |
| `verified` | All evidence confirmed | Stored with +0.1 confidence boost |
| `partial` | Some evidence confirmed | Stored with warning |
| `rejected` | Evidence not found | **Discarded** (not stored) |
| `skipped` | No evidence provided | Stored with lower confidence |

---

## The Bootstrap Pipeline

When you run `tw bootstrap`, TaskWing executes a **Map-Reduce pipeline**:

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                         BOOTSTRAP PIPELINE                                  │
│                                                                             │
│  tw bootstrap                                                               │
│       │                                                                     │
│       ▼                                                                     │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │                     PHASE 1: MAP (Parallel Analysis)                │   │
│  │                                                                     │   │
│  │   Each agent runs in its own goroutine:                             │   │
│  │                                                                     │   │
│  │   ┌──────────────────────────────────────────────────────────────┐  │   │
│  │   │  DocAgent          │  Reads: *.md files                      │  │   │
│  │   │                    │  Extracts: Features, Constraints        │  │   │
│  │   │                    │  Prompt: config.PromptTemplateDocAgent  │  │   │
│  │   ├────────────────────┼─────────────────────────────────────────┤  │   │
│  │   │  CodeAgent         │  Reads: Entry points, handlers, configs │  │   │
│  │   │                    │  Extracts: Patterns, Decisions          │  │   │
│  │   │                    │  Prompt: config.PromptTemplateCodeAgent │  │   │
│  │   ├────────────────────┼─────────────────────────────────────────┤  │   │
│  │   │  GitAgent          │  Reads: git log, git shortlog           │  │   │
│  │   │                    │  Extracts: Milestones, Evolution        │  │   │
│  │   │                    │  Prompt: config.PromptTemplateGitAgent  │  │   │
│  │   ├────────────────────┼─────────────────────────────────────────┤  │   │
│  │   │  DepsAgent         │  Reads: go.mod, package.json            │  │   │
│  │   │                    │  Extracts: Tech decisions, Stack        │  │   │
│  │   │                    │  Prompt: config.PromptTemplateDepsAgent │  │   │
│  │   └──────────────────────────────────────────────────────────────┘  │   │
│  │                                                                     │   │
│  │   Output: []agents.Output (one per agent)                           │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                    │                                        │
│                                    ▼                                        │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │                     PHASE 2: AGGREGATE                              │   │
│  │                                                                     │   │
│  │   agents.AggregateFindings(outputs) → []Finding                     │   │
│  │                                                                     │   │
│  │   • Combines all agent outputs into single slice                    │   │
│  │   • Each finding tagged with SourceAgent                            │   │
│  │                                                                     │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                    │                                        │
│                                    ▼                                        │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │                     PHASE 3: VERIFY                                 │   │
│  │                                                                     │   │
│  │   VerificationAgent.VerifyFindings(findings)                        │   │
│  │                                                                     │   │
│  │   For each finding:                                                 │   │
│  │   1. Check if file exists at cited path                             │   │
│  │   2. Read file content                                              │   │
│  │   3. Check if snippet exists (exact or fuzzy match)                 │   │
│  │   4. Validate line numbers match                                    │   │
│  │   5. Calculate similarity score                                     │   │
│  │   6. Set status: verified | partial | rejected                      │   │
│  │   7. Adjust confidence score                                        │   │
│  │                                                                     │   │
│  │   FilterVerifiedFindings() → Reject findings that failed            │   │
│  │                                                                     │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                    │                                        │
│                                    ▼                                        │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │                     PHASE 4: INGEST                                 │   │
│  │                                                                     │   │
│  │   KnowledgeService.IngestFindings()                                 │   │
│  │                                                                     │   │
│  │   Step 1: purgeStaleData()                                          │   │
│  │           Delete old nodes from agents being re-run                 │   │
│  │                                                                     │   │
│  │   Step 2: ingestNodes()                                             │   │
│  │           • Deduplicate by content hash                             │   │
│  │           • Generate embeddings (OpenAI API)                        │   │
│  │           • Store verification status + evidence                    │   │
│  │           • Insert into SQLite                                      │   │
│  │                                                                     │   │
│  │   Step 3: ingestStructuredData()                                    │   │
│  │           • Create Feature records                                  │   │
│  │           • Create Decision records (linked to features)            │   │
│  │           • Create Pattern records                                  │   │
│  │                                                                     │   │
│  │   Step 4: linkKnowledgeGraph()                                      │   │
│  │           • Co-occurrence edges (same agent → relates_to)           │   │
│  │           • Structural edges (decision → affects → feature)         │   │
│  │           • Semantic edges (cosine similarity > 0.7)                │   │
│  │                                                                     │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                    │                                        │
│                                    ▼                                        │
│                           SQLite Knowledge Graph                            │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Code Path

```
cmd/bootstrap.go
    └── runAgentBootstrap()
        ├── bootstrap.NewDefaultAgents()          # Create agent instances
        ├── ui.NewBootstrapModel()                # TUI for progress
        ├── agents.AggregateFindings()            # Combine outputs
        └── knowledge.Service.IngestFindings()    # Store to DB
            ├── verifyFindings()                  # Run VerificationAgent
            ├── purgeStaleData()                  # Delete old nodes
            ├── ingestNodes()                     # Create nodes + embeddings
            ├── ingestStructuredData()            # Features, Decisions
            └── linkKnowledgeGraph()              # Create edges
```

---

## Evidence-Based Verification

### The Problem: LLM Hallucinations

LLMs can confidently claim things that aren't true:

```json
{
  "title": "Uses Redis for session storage",
  "evidence": [{
    "file_path": "internal/session/redis.go",  // ← This file doesn't exist!
    "snippet": "func NewRedisStore()..."
  }]
}
```

### The Solution: VerificationAgent

A **deterministic** (no LLM) agent that validates evidence:

```go
// internal/agents/verification_agent.go

func (v *VerificationAgent) checkEvidence(evidence Evidence) EvidenceCheckResult {
    result := EvidenceCheckResult{}

    // 1. Check file exists
    fullPath := filepath.Join(v.basePath, evidence.FilePath)
    if _, err := os.Stat(fullPath); os.IsNotExist(err) {
        result.ErrorMessage = "file not found"
        return result
    }
    result.FileExists = true

    // 2. Read file content
    content, _ := os.ReadFile(fullPath)

    // 3. Check if snippet exists anywhere in file
    if containsNormalized(string(content), evidence.Snippet) {
        result.SnippetFound = true
    }

    // 4. Check line numbers match
    if evidence.StartLine > 0 {
        actualContent := extractLines(string(content), evidence.StartLine, evidence.EndLine)
        if strings.Contains(actualContent, evidence.Snippet) {
            result.LineNumbersMatch = true
        } else {
            result.SimilarityScore = calculateSimilarity(actualContent, evidence.Snippet)
        }
    }

    return result
}
```

### Verification Flow

```
Finding with Evidence
         │
         ▼
┌─────────────────────────────────────┐
│  For each Evidence item:            │
│                                     │
│  1. Does file exist?                │
│     NO  → ErrorMessage = "not found"│
│     YES → Continue                  │
│                                     │
│  2. Is snippet in file?             │
│     YES → SnippetFound = true       │
│     NO  → Try grep pattern          │
│                                     │
│  3. Do line numbers match?          │
│     YES → LineNumbersMatch = true   │
│     NO  → Calculate similarity      │
│                                     │
└─────────────────────────────────────┘
         │
         ▼
┌─────────────────────────────────────┐
│  Determine Overall Status:          │
│                                     │
│  All evidence verified → VERIFIED   │
│  Some verified         → PARTIAL    │
│  None verified         → REJECTED   │
│  No evidence provided  → SKIPPED    │
│                                     │
└─────────────────────────────────────┘
         │
         ▼
┌─────────────────────────────────────┐
│  Adjust Confidence:                 │
│                                     │
│  VERIFIED: +0.1                     │
│  PARTIAL:  0 to -0.1                │
│  REJECTED: -0.3 (then discarded)    │
│                                     │
└─────────────────────────────────────┘
```

### Security: Path Traversal Protection

```go
// Prevent access outside project directory
fullPath = filepath.Clean(fullPath)
if !strings.HasPrefix(fullPath, filepath.Clean(v.basePath)) {
    result.ErrorMessage = "path traversal detected"
    return result
}
```

---

## Knowledge Graph

### Node Types

| Type | Description | Extracted By |
|------|-------------|--------------|
| `feature` | Product capability (what it does) | DocAgent |
| `decision` | Architectural choice (why) | GitAgent, CodeAgent |
| `pattern` | Recurring solution | CodeAgent |
| `constraint` | Rule that must be followed | DocAgent |

### Edge Types

| Relation | Meaning | Example |
|----------|---------|---------|
| `depends_on` | A requires B | Auth depends_on Database |
| `affects` | A influences B | Decision affects Feature |
| `extends` | A adds to B | Pattern extends Feature |
| `relates_to` | Loose association | Co-occurrence |
| `semantically_similar` | Vector similarity > 0.7 | Auto-detected |

### Graph Queries

**Semantic search** (used by `tw context`):

```go
// Hybrid search: FTS5 + Vector similarity
func (s *Service) Search(ctx context.Context, query string, limit int) ([]ScoredNode, error) {
    // 1. FTS5 keyword search (fast, free)
    ftsResults, _ := s.repo.SearchFTS(query, limit*2)

    // 2. Vector similarity search (requires embedding)
    queryEmbedding, _ := GenerateEmbedding(ctx, query, s.llmCfg)
    nodes, _ := s.repo.ListNodesWithEmbeddings()
    for _, n := range nodes {
        similarity := CosineSimilarity(queryEmbedding, n.Embedding)
        if similarity > threshold {
            // Add to results
        }
    }

    // 3. Merge and rank by combined score
    return mergedResults, nil
}
```

---

## Package Structure

```
taskWing-cli/
├── cmd/                          # CLI commands (Cobra)
│   ├── root.go                   # Base command, global flags
│   ├── bootstrap.go              # tw bootstrap
│   ├── context.go                # tw context "query"
│   ├── add.go                    # tw add "knowledge"
│   ├── list.go                   # tw list [--type X]
│   ├── plan.go                   # tw plan new/list/export
│   ├── start.go                  # tw start (watch mode)
│   └── mcp_server.go             # tw mcp (MCP server)
│
├── internal/
│   ├── agents/                   # LLM-powered analysis agents
│   │   ├── agent.go              # Finding, Output types
│   │   ├── base_agent.go         # BaseAgent with Generate()
│   │   ├── evidence.go           # Evidence, VerificationStatus types
│   │   ├── verification_agent.go # Deterministic evidence checker
│   │   ├── doc_agent.go          # Analyzes *.md files
│   │   ├── analysis/code.go      # ReactAgent (interactive exploration)
│   │   ├── git_deps_agent.go     # Git history + dependencies
│   │   ├── watch_agent.go        # File change detection
│   │   ├── context_gatherer.go   # File reading utilities
│   │   └── tools/eino.go         # Tools for ReactAgent
│   │
│   ├── knowledge/                # Semantic search + RAG
│   │   ├── service.go            # KnowledgeService (search, ask)
│   │   ├── ingest.go             # IngestFindings pipeline
│   │   ├── embed.go              # GenerateEmbedding()
│   │   ├── classify.go           # AI classification
│   │   └── config.go             # Thresholds and weights
│   │
│   ├── memory/                   # Storage layer
│   │   ├── models.go             # Node, Feature, Decision types
│   │   ├── sqlite.go             # SQLite implementation
│   │   └── repository.go         # Repository interface
│   │
│   ├── llm/                      # Multi-provider LLM client
│   │   └── client.go             # NewChatModel (OpenAI, Ollama)
│   │
│   ├── config/                   # Configuration
│   │   └── prompts.go            # All agent prompts (single source)
│   │
│   ├── bootstrap/                # Bootstrap orchestration
│   │   └── agents.go             # NewDefaultAgents()
│   │
│   └── ui/                       # Terminal UI (Bubble Tea)
│       ├── bootstrap_model.go    # Progress display
│       └── dashboard.go          # Results rendering
│
├── docs/                         # Documentation
│   ├── ARCHITECTURE.md           # This file
│   ├── DATA_MODEL.md             # Schema details
│   ├── ROADMAP.md                # Version planning
│   └── BOOTSTRAP_INTERNALS.md    # Bootstrap internals
│
└── .taskwing/                    # Project data (created by tw)
    └── memory/
        ├── memory.db             # SQLite database
        ├── index.json            # Cache (regenerated)
        └── features/*.md         # Generated markdown
```

---

## Data Flow Examples

### Example 1: Bootstrap

```
User runs: tw bootstrap

1. cmd/bootstrap.go:runAgentBootstrap()
2. Creates agents: DocAgent, CodeAgent, GitAgent, DepsAgent
3. TUI shows progress as agents run in parallel
4. Each agent calls LLM with prompts from config/prompts.go
5. LLM returns JSON with findings + evidence
6. Agents parse JSON into []Finding
7. AggregateFindings() combines all agent outputs
8. VerificationAgent checks each finding's evidence
9. Rejected findings are filtered out
10. IngestFindings() stores verified findings
11. Embeddings generated for semantic search
12. Graph edges created based on similarity
13. SQLite now contains queryable knowledge
```

### Example 2: Context Query

```
User runs: tw context "authentication"

1. cmd/context.go handles command
2. knowledge.Service.Search() called
3. FTS5 search: "authentication" → keyword matches
4. Embedding generated for query
5. Vector similarity calculated against all nodes
6. Results merged, ranked by combined score
7. Top N results returned
8. If --answer flag: RAG prompt sent to LLM
9. LLM generates answer using retrieved context
```

### Example 3: MCP Query

```
AI tool queries: recall tool

1. internal/agents/core/mcp.go handles request
2. Query embedded + searched
3. Top results formatted as context
4. Returned to AI tool (Claude, Cursor, etc.)
5. AI uses context for better responses
```

---

## Extension Points

### Adding a New Agent

1. Create `internal/agents/my_agent.go`:

```go
type MyAgent struct {
    BaseAgent
}

func NewMyAgent(cfg llm.Config) *MyAgent {
    return &MyAgent{
        BaseAgent: NewBaseAgent("my_agent", "Description", cfg),
    }
}

func (a *MyAgent) Run(ctx context.Context, input Input) (Output, error) {
    // 1. Gather context
    content := gatherMyContent(input.BasePath)

    // 2. Build prompt
    prompt := fmt.Sprintf(config.PromptTemplateMyAgent, content)

    // 3. Call LLM
    rawOutput, err := a.Generate(ctx, []*schema.Message{
        schema.UserMessage(prompt),
    })

    // 4. Parse response
    findings, err := a.parseResponse(rawOutput)

    // 5. Return output
    return BuildOutput(a.Name(), findings, rawOutput, time.Since(start)), nil
}
```

2. Add prompt to `internal/config/prompts.go`
3. Register in `internal/bootstrap/agents.go`

### Adding a New Finding Type

1. Add to `internal/agents/agent.go`:

```go
const (
    FindingTypeDecision   FindingType = "decision"
    FindingTypeFeature    FindingType = "feature"
    FindingTypePattern    FindingType = "pattern"
    FindingTypeConstraint FindingType = "constraint"
    FindingTypeMyType     FindingType = "my_type"  // New
)
```

2. Handle in `internal/knowledge/ingest.go`:

```go
case agents.FindingTypeMyType:
    // Custom ingestion logic
```

---

## Tech Stack

| Component | Technology | Why |
|-----------|------------|-----|
| Language | Go 1.24 | Fast, single binary, great concurrency |
| CLI Framework | Cobra | Industry standard, great UX |
| Storage | SQLite (modernc.org/sqlite) | Zero dependencies, embedded, fast |
| LLM Client | CloudWeGo Eino | Multi-provider, tool support |
| Embeddings | OpenAI text-embedding-3-small | Best quality/cost ratio |
| MCP | mcp-go-sdk | Standard for AI tool integration |
| TUI | Bubble Tea | Beautiful terminal UIs |

---

## Related Documentation

| Document | Purpose |
|----------|---------|
| [DATA_MODEL.md](./DATA_MODEL.md) | Database schema, node types, verification |
| [ROADMAP.md](./ROADMAP.md) | Version planning, upcoming features |
| [BOOTSTRAP_INTERNALS.md](./BOOTSTRAP_INTERNALS.md) | Bootstrap scanner details |
| [GETTING_STARTED.md](./GETTING_STARTED.md) | User guide |
| [MCP_INTEGRATION.md](./MCP_INTEGRATION.md) | MCP server setup |
