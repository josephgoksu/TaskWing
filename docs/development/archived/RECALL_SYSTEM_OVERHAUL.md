# Recall System Overhaul: Implementation Plan

> **Status**: Proposed
> **Created**: 2025-01-09
> **Goal**: Improve recall tool quality and reduce context waste

---

## Executive Summary

The current recall system has several issues:
1. Returns verbose JSON that wastes AI context tokens
2. Uses general-purpose embeddings not optimized for code
3. Over-relies on keyword search (65% FTS vs 35% vector)
4. No reranking step to improve result quality
5. No text chunking before embedding

This plan upgrades the system to use state-of-the-art open-source models (Qwen3) and implements best practices from 2025 RAG research.

---

## Current System Analysis

### Embedding Models in Use

| Provider | Model | Dimensions | Quality (MTEB) |
|----------|-------|------------|----------------|
| OpenAI | `text-embedding-3-small` | 1536 | ~58 |
| Gemini | `text-embedding-004` | 768 | ~68 |
| Ollama | `nomic-embed-text` | 768 | ~62 |

**Problem**: These are general-purpose text embeddings. Research shows they struggle with code-specific elements like syntax, variable dependencies, and API structures.

### Search Configuration

```go
// internal/knowledge/config.go (current)
FTSWeight = 0.65              // Keyword search dominates
VectorWeight = 0.35           // Semantic search underweighted
VectorScoreThreshold = 0.25   // Too permissive
```

### Output Format

Current response includes:
- Full JSON structure with metadata
- Embedding vectors (completely unnecessary)
- Internal IDs and timestamps
- Verbose nested objects

**Result**: Wastes 70%+ of tokens on data AI assistants don't need.

---

## Target Architecture

### Models

| Component | Model | Dimensions | Quality |
|-----------|-------|------------|---------|
| Embedding | Qwen3-Embedding-8B | 1024 | 70.58 MTEB (#1) |
| Reranking | Qwen3-Reranker-8B | - | 81.22 MTEB-Code |

### Search Flow

```
User Query
    │
    ▼
┌─────────────────────────────────────┐
│  Parallel Search                     │
│  ┌─────────────┐  ┌───────────────┐ │
│  │ FTS (BM25)  │  │ Vector Search │ │
│  │ Top 30      │  │ Top 30        │ │
│  └──────┬──────┘  └───────┬───────┘ │
│         │                 │         │
│         └────────┬────────┘         │
│                  ▼                  │
│         Merge & Deduplicate         │
│              Top 30                 │
└─────────────────┬───────────────────┘
                  │
                  ▼
┌─────────────────────────────────────┐
│  Qwen3-Reranker-8B                  │
│  Score each (query, doc) pair       │
│  Return Top 5                       │
└─────────────────┬───────────────────┘
                  │
                  ▼
┌─────────────────────────────────────┐
│  Format as Compact Markdown         │
│  - Group by type                    │
│  - No JSON, no embeddings           │
│  - Human-readable summaries         │
└─────────────────────────────────────┘
```

### Output Format

**Before** (verbose JSON):
```json
{
  "nodes": [{
    "id": "abc123-def456-...",
    "content": "We decided to use SQLite...",
    "type": "decision",
    "summary": "Database choice",
    "embedding": [0.123, 0.456, ...],
    "created_at": "2025-01-09T10:00:00Z",
    "score": 0.85
  }],
  "query": "database",
  "total": 1
}
```

**After** (compact markdown):
```markdown
## Architecture Context

### Decisions
- **Database: SQLite over PostgreSQL**
  Single-file distribution for CLI tools. No server dependency.
  Trade-off: Limited concurrent writes, but acceptable for single-user CLI.

### Patterns
- **Repository pattern** in `internal/memory/`
  All data access through MemoryStore interface.

### Related Features
- memory-store → depends on sqlite
- knowledge-service → uses memory-store
```

---

## Implementation Phases

### Phase 1: Compact Output Format

**Duration**: 1-2 days
**Risk**: Low
**Impact**: Immediate 70% token reduction

#### Changes

1. **New response formatter** (`internal/knowledge/formatter.go`):
   ```go
   type OutputFormat string

   const (
       FormatMarkdown OutputFormat = "markdown"  // Default
       FormatJSON     OutputFormat = "json"      // For programmatic access
       FormatDebug    OutputFormat = "debug"     // Include scores, IDs
   )

   func FormatResults(results []ScoredNode, format OutputFormat) string
   ```

2. **Update MCP handler** (`cmd/mcp_server.go`):
   - Remove embeddings from response
   - Group results by type (decision, pattern, feature)
   - Return markdown by default
   - Add `format` parameter for JSON when needed

3. **Markdown template**:
   ```markdown
   ## Context for: "{query}"

   {{#if decisions}}
   ### Decisions
   {{#each decisions}}
   - **{{summary}}**
     {{content}}
   {{/each}}
   {{/if}}

   {{#if patterns}}
   ### Patterns
   {{#each patterns}}
   - **{{summary}}** in `{{source}}`
     {{content}}
   {{/each}}
   {{/if}}

   {{#if features}}
   ### Related Features
   {{#each features}}
   - {{name}}{{#if dependencies}} → {{dependencies}}{{/if}}
   {{/each}}
   {{/if}}
   ```

#### Files to Modify

- `cmd/mcp_server.go` - Update `handleNodeContext()`
- `internal/knowledge/service.go` - Add `FormatResults()` method
- New: `internal/knowledge/formatter.go`

---

### Phase 2: Rebalance Search Weights

**Duration**: 0.5 days
**Risk**: Low
**Impact**: Better semantic matching

#### Changes

```go
// internal/knowledge/config.go

const (
    // Rebalanced weights - trust semantic search more
    FTSWeight    = 0.40  // was 0.65
    VectorWeight = 0.60  // was 0.35

    // Stricter threshold to filter noise
    VectorScoreThreshold    = 0.35  // was 0.25
    MinResultScoreThreshold = 0.12  // was 0.08
)
```

#### Rationale

- Current 65% FTS weight makes semantic search almost decorative
- Vector search should be primary for "meaning" queries
- FTS remains important for exact keyword matches (function names, etc.)
- Higher thresholds reduce noise in results

---

### Phase 3: Qwen3 Embedding Support

**Duration**: 2-3 days
**Risk**: Medium (new dependency)
**Impact**: Significantly better code understanding

#### Option A: Ollama Integration (Recommended)

Wait for or contribute Qwen3 support in Ollama:
```bash
ollama pull qwen3-embed:8b
```

Configuration:
```yaml
# .taskwing.yaml
llm:
  provider: ollama
  embedding_model: qwen3-embed:8b
```

#### Option B: Text Embeddings Inference (TEI)

Run Qwen3 via HuggingFace TEI as sidecar:
```bash
docker run -p 8080:80 \
  ghcr.io/huggingface/text-embeddings-inference:1.7.2 \
  --model-id Qwen/Qwen3-Embedding-8B \
  --dtype float16
```

Add TEI provider to `internal/llm/client.go`:
```go
case ProviderTEI:
    return newTEIEmbedder(ctx, cfg)
```

#### Option C: Direct Transformers (Python sidecar)

For maximum control, run Python service:
```python
from sentence_transformers import SentenceTransformer

model = SentenceTransformer("Qwen/Qwen3-Embedding-8B")
embeddings = model.encode(texts, prompt_name="query")
```

#### Implementation Details

1. **Add provider constant**:
   ```go
   // internal/llm/constants.go
   const (
       ProviderTEI = "tei"  // Text Embeddings Inference
   )
   ```

2. **TEI embedder implementation**:
   ```go
   // internal/llm/tei_embedder.go
   type TEIEmbedder struct {
       baseURL string
       client  *http.Client
   }

   func (e *TEIEmbedder) EmbedStrings(ctx context.Context, texts []string) ([][]float64, error) {
       // POST to /embed endpoint
   }
   ```

3. **Configuration extension**:
   ```go
   type Config struct {
       // ... existing fields
       EmbeddingProvider string  // Can differ from chat provider
       EmbeddingBaseURL  string  // For TEI/custom endpoints
       EmbeddingDims     int     // Target dimensions (Qwen3 supports 32-4096)
   }
   ```

4. **Dimension standardization**:
   - Store target dimension in config (recommend 1024)
   - Qwen3 supports Matryoshka Representation Learning (MRL)
   - Can truncate embeddings to smaller dims without re-embedding

#### Database Migration

```sql
-- Add columns for embedding metadata
ALTER TABLE nodes ADD COLUMN embedding_model TEXT DEFAULT '';
ALTER TABLE nodes ADD COLUMN embedding_dims INTEGER DEFAULT 0;

-- Index for finding nodes needing re-embedding
CREATE INDEX idx_nodes_embedding_model ON nodes(embedding_model);
```

Re-embedding strategy:
```go
func (s *Service) MigrateEmbeddings(ctx context.Context, targetModel string) error {
    nodes, _ := s.repo.ListNodesWithoutModel(targetModel)
    for _, n := range nodes {
        embedding, _ := GenerateEmbedding(ctx, n.Content, s.llmCfg)
        s.repo.UpdateNodeEmbedding(n.ID, embedding, targetModel)
    }
}
```

---

### Phase 4: Add Reranking

**Duration**: 2-3 days
**Risk**: Medium
**Impact**: 15-30% accuracy improvement

#### Architecture

```go
// internal/knowledge/reranker.go

type Reranker interface {
    Rerank(ctx context.Context, query string, docs []string) ([]float32, error)
}

type Qwen3Reranker struct {
    baseURL string
    model   string
}

func (r *Qwen3Reranker) Rerank(ctx context.Context, query string, docs []string) ([]float32, error) {
    // Format as instruction + query + document pairs
    // Return relevance scores
}
```

#### Integration Point

```go
// internal/knowledge/service.go

func (s *Service) searchInternal(ctx context.Context, query string, ...) ([]ScoredNode, error) {
    // 1. FTS search
    ftsResults := s.repo.SearchFTS(query, 30)

    // 2. Vector search
    vectorResults := s.searchVector(ctx, query, 30)

    // 3. Merge and deduplicate
    merged := mergeResults(ftsResults, vectorResults)

    // 4. NEW: Rerank if enabled
    if s.reranker != nil && len(merged) > 5 {
        merged = s.rerank(ctx, query, merged, 20)  // Rerank top 20
    }

    // 5. Return top K
    return merged[:min(len(merged), limit)], nil
}
```

#### Configuration

```yaml
# .taskwing.yaml
reranking:
  enabled: true
  provider: tei  # or ollama when available
  model: Qwen/Qwen3-Reranker-8B
  base_url: http://localhost:8081
  top_k: 20  # Rerank this many candidates
```

#### Reranker Prompt Format

```
<Instruct>: Given a code architecture query, determine if this document is relevant.
<Query>: {user_query}
<Document>: {document_content}
```

Response: Extract logits for "yes"/"no" tokens, compute relevance probability.

---

### Phase 5: Text Chunking (Optional)

**Duration**: 3-4 days
**Risk**: High (significant refactor)
**Impact**: Better embeddings for long documents

#### Current Problem

Full documents are embedded as-is. Long documents get diluted embeddings that match poorly.

#### Solution: Hierarchical Chunking

```
Document
    │
    ├── Chunk 1 (embedding)
    │       │
    │       └── Parent reference
    │
    ├── Chunk 2 (embedding)
    │       │
    │       └── Parent reference
    │
    └── Chunk 3 (embedding)
            │
            └── Parent reference
```

#### Implementation

1. **Chunking strategies**:
   ```go
   // internal/knowledge/chunker.go

   type Chunker interface {
       Chunk(content string, metadata ChunkMetadata) []Chunk
   }

   type RecursiveChunker struct {
       MaxTokens int  // 400-500 recommended
       Overlap   int  // 10-20% of MaxTokens
   }

   type ASTChunker struct {
       // For code files - respects function/class boundaries
       Language string
   }
   ```

2. **Schema extension**:
   ```sql
   CREATE TABLE chunks (
       id TEXT PRIMARY KEY,
       node_id TEXT REFERENCES nodes(id),
       content TEXT NOT NULL,
       chunk_index INTEGER,
       embedding BLOB,
       created_at TEXT
   );
   ```

3. **Search modification**:
   - Search chunks instead of nodes
   - Aggregate chunk scores to parent node
   - Return parent node content with highlighted chunks

#### AST-Based Chunking for Code

Use tree-sitter for language-aware boundaries:
```go
// Respects function/class boundaries
// Includes necessary context (imports, type definitions)
// Preserves semantic units
```

**Note**: This phase is optional. Phases 1-4 provide significant improvement without chunking complexity.

---

## Configuration Schema

### Full Configuration

```yaml
# .taskwing.yaml

llm:
  # Chat model (for classification, RAG answers)
  provider: gemini
  model: gemini-2.0-flash
  api_key: ${GEMINI_API_KEY}

embedding:
  # Can use different provider than chat
  provider: tei
  model: Qwen/Qwen3-Embedding-8B
  base_url: http://localhost:8080
  dimensions: 1024  # Qwen3 supports 32-4096

reranking:
  enabled: true
  provider: tei
  model: Qwen/Qwen3-Reranker-8B
  base_url: http://localhost:8081
  top_k: 20

search:
  fts_weight: 0.40
  vector_weight: 0.60
  vector_threshold: 0.35
  min_score_threshold: 0.12
  result_limit: 5

output:
  format: markdown  # markdown, json, debug
  include_scores: false
  include_sources: true
```

### Environment Variables

```bash
# Provider API keys
GEMINI_API_KEY=...
OPENAI_API_KEY=...

# TEI endpoints (for Qwen3)
TASKWING_EMBEDDING_URL=http://localhost:8080
TASKWING_RERANKER_URL=http://localhost:8081

# Feature flags
TASKWING_RERANKING_ENABLED=true
```

---

## Deployment Options for Qwen3

### Option 1: Docker Compose (Recommended for Development)

```yaml
# docker-compose.yml
version: '3.8'

services:
  embedding:
    image: ghcr.io/huggingface/text-embeddings-inference:1.7.2
    command: --model-id Qwen/Qwen3-Embedding-8B --dtype float16
    ports:
      - "8080:80"
    deploy:
      resources:
        reservations:
          devices:
            - driver: nvidia
              count: 1
              capabilities: [gpu]

  reranker:
    image: ghcr.io/huggingface/text-embeddings-inference:1.7.2
    command: --model-id Qwen/Qwen3-Reranker-8B --dtype float16
    ports:
      - "8081:80"
    deploy:
      resources:
        reservations:
          devices:
            - driver: nvidia
              count: 1
              capabilities: [gpu]
```

### Option 2: Ollama (When Available)

```bash
# Simpler, but waiting on Ollama to add Qwen3 embedding support
ollama pull qwen3-embed:8b
ollama pull qwen3-reranker:8b
```

### Option 3: Cloud API (Future)

If Alibaba or third parties offer hosted Qwen3 embedding API:
```yaml
embedding:
  provider: qwen-cloud
  api_key: ${QWEN_API_KEY}
```

---

## Testing Strategy

### Unit Tests

```go
// internal/knowledge/formatter_test.go
func TestFormatResults_Markdown(t *testing.T) {
    results := []ScoredNode{
        {Node: memory.Node{Type: "decision", Summary: "Use SQLite"}, Score: 0.9},
    }
    output := FormatResults(results, FormatMarkdown)
    assert.Contains(t, output, "### Decisions")
    assert.Contains(t, output, "Use SQLite")
    assert.NotContains(t, output, "embedding")
}
```

### Integration Tests

```go
// tests/integration/recall_test.go
func TestRecall_CompactOutput(t *testing.T) {
    // Call MCP recall tool
    // Verify response is markdown
    // Verify no JSON structure
    // Verify token count < threshold
}
```

### Benchmark Tests

```go
// internal/knowledge/search_bench_test.go
func BenchmarkSearch_WithReranking(b *testing.B) {
    // Compare latency with/without reranking
    // Target: <500ms total response time
}
```

---

## Success Metrics

| Metric | Current | Target |
|--------|---------|--------|
| Response token count | ~2000 | <500 |
| Retrieval accuracy (manual eval) | ~60% | >85% |
| Search latency (p95) | 200ms | <500ms (with reranking) |
| Code-specific recall | Poor | Good |

---

## Rollout Plan

### Week 1
- [ ] Phase 1: Compact output format
- [ ] Phase 2: Rebalance search weights
- [ ] Release v1.7.0

### Week 2-3
- [ ] Phase 3: Qwen3 embedding support (TEI integration)
- [ ] Database migration for embedding metadata
- [ ] Release v1.8.0

### Week 4
- [ ] Phase 4: Reranking integration
- [ ] Performance benchmarking
- [ ] Release v1.9.0

### Future
- [ ] Phase 5: Text chunking (if needed based on metrics)
- [ ] Ollama native support when available

---

## Open Questions

1. **GPU requirements**: Qwen3-8B needs ~16GB VRAM. Should we also support smaller models (Qwen3-Embedding-0.6B)?

2. **Fallback behavior**: What happens when TEI service is unavailable? Graceful degradation to Gemini?

3. **Embedding versioning**: How to handle mixed embeddings during migration? Re-embed all at once or lazy migration?

4. **Cost tracking**: Should we track embedding API costs separately from chat model costs?

---

## References

### Research
- [MTEB Leaderboard](https://huggingface.co/spaces/mteb/leaderboard) - Embedding model benchmarks
- [Qwen3 Embedding Paper](https://arxiv.org/abs/2506.05176) - Model architecture and training
- [RAG Best Practices 2025](https://docs.anthropic.com/en/docs/build-with-claude/retrieval-augmented-generation) - Chunking and retrieval strategies

### Models
- [Qwen3-Embedding-8B](https://huggingface.co/Qwen/Qwen3-Embedding-8B) - #1 on MTEB multilingual
- [Qwen3-Reranker-8B](https://huggingface.co/Qwen/Qwen3-Reranker-8B) - 81.22 on MTEB-Code

### Tools
- [Text Embeddings Inference](https://github.com/huggingface/text-embeddings-inference) - Production embedding server
- [tree-sitter](https://tree-sitter.github.io/tree-sitter/) - AST parsing for code chunking
