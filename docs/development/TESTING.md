# TaskWing Testing Framework

> **Goal:** Continuous quality improvement through structured feedback loops

---

## The Feedback Loop Machine

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│   TEST      │────▶│   GATHER    │────▶│  IMPLEMENT  │────▶│  COMPARE    │
│   on repo   │     │   feedback  │     │   changes   │     │   results   │
└─────────────┘     └─────────────┘     └─────────────┘     └─────────────┘
       ▲                                                            │
       │                                                            │
       │            ┌─────────────────────────────────┐            │
       └────────────│  DECISION: Keep / Remove / Dig  │◀───────────┘
                    └─────────────────────────────────┘
```

---

## Phase 1: Test on Repositories

### Test Suite

| Repo | Language | Size | Purpose |
|------|----------|------|---------|
| markwise.app | Go/TS | Large mono | Your project (ground truth) |
| [cal.com/cal.com](https://github.com/calcom/cal.com) | TS | Large | Scheduling product |
| [umami-software/umami](https://github.com/umami-software/umami) | TS/Node | Medium | Analytics product |
| TaskWing CLI | Go | Medium | Dogfooding (use on itself) |

### Metrics to Capture

| Metric | How to Measure |
|--------|----------------|
| **Accuracy** | % of inferred decisions that are correct |
| **Completeness** | Did it find all major features? |
| **Relevance** | Are RAG answers sourced correctly? |
| **Speed** | Bootstrap time, query response time |
| **Token efficiency** | Tokens used per bootstrap |

---

## Phase 2: Gather Feedback

### Feedback Template

```markdown
## Repo: [name]
## Date: [YYYY-MM-DD]
## Version: [tw version]

### Accuracy (1-10): ___
- Wrong inferences: [list]
- Correct inferences: [list]

### Completeness (1-10): ___
- Missing features: [list]
- Found features: [list]

### RAG Quality (1-10): ___
- Query tested: [query]
- Answer quality: [good/bad]
- Sources relevant: [yes/no]

### Issues Found
- [ ] Issue 1
- [ ] Issue 2
```

---

## Phase 3: Implement Changes

### Change Classification

| Type | Action |
|------|--------|
| **P0 Bug** | Fix immediately |
| **P1 Quality** | Fix this sprint |
| **P2 Enhancement** | Backlog |

### Implementation Rule

Every change MUST be:
1. Isolated (one change per test)
2. Measurable (before/after metrics)
3. Reversible (can rollback)

---

## Phase 4: Compare Results

### Decision Matrix

| Result | Action |
|--------|--------|
| **Score ↑** | Keep change, document why |
| **Score ↓** | Revert change, document why |
| **Score =** | Analyze deeper — hidden benefit or wasted effort? |

### Comparison Table

| Metric | Before | After | Δ | Verdict |
|--------|--------|-------|---|---------|
| Accuracy | 7/10 | 8/10 | +1 | ✅ Keep |
| Completeness | 6/10 | 6/10 | 0 | 🔍 Analyze |
| RAG Quality | 5/10 | 4/10 | -1 | ❌ Revert |

---

## Test Execution Checklist

### Per Repository

```bash
# 1. Baseline
cd [repo]
rm -rf .taskwing
tw bootstrap
tw list > baseline.txt

# 2. Test queries
tw context "main architecture" --answer > q1.txt
tw context "database choice" --answer > q2.txt
tw context "deployment" --answer > q3.txt

# 3. Record metrics
# Fill feedback template
```

### Per Change

1. [ ] Record baseline metrics
2. [ ] Implement change
3. [ ] Re-run on same repos
4. [ ] Compare metrics
5. [ ] Decision: Keep / Revert / Dig

---

## Quality Baseline (Current)

| Repo | Accuracy | Completeness | RAG | Date |
|------|----------|--------------|-----|------|
| markwise.app | ?/10 | ?/10 | ?/10 | TBD |
| NestJS | ?/10 | ?/10 | ?/10 | TBD |

**Action:** Fill this table with your first test run.

---

## References

- [ROADMAP.md](../architecture/ROADMAP.md) — Version planning
- [SYSTEM_DESIGN.md](../architecture/SYSTEM_DESIGN.md) — System design

---

## Functional Test Cases (MCP)

> Merged from MCP_TESTING.md

### Test Matrix

| Test Case | What to Verify |
|-----------|----------------|
| MCP-01 | Server starts, returns valid JSON-RPC |
| MCP-02 | `project-context` tool returns knowledge |
| MCP-03 | Query filtering works semantically |
| MCP-04 | Local install creates correct config |
| MCP-05 | Global install creates correct config |

### MCP-01: Server Initialization

```bash
cd ~/taskwing-tests/markwise.app

# Send initialize request
(echo '{"jsonrpc":"2.0","method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}},"id":1}'; sleep 2) | tw mcp 2>&1 | head -15
```

**Expected:**
- Banner: `🎯 TaskWing MCP Server Starting...`
- JSON response with `serverInfo.name: "taskwing"`, `version: "2.0.0"`
- No errors

### MCP-02: Project Context Tool (Full)

```bash
cd ~/taskwing-tests/markwise.app

(echo '{"jsonrpc":"2.0","method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}},"id":1}';
 echo '{"jsonrpc":"2.0","method":"initialized","params":{},"id":2}';
 echo '{"jsonrpc":"2.0","method":"tools/call","params":{"name":"project-context","arguments":{}},"id":3}';
 sleep 2) | tw mcp 2>&1 | tail -20
```

**Expected:**
- Response contains `nodes` with features and decisions
- `total` count matches `tw list` count

### MCP-03: Semantic Query

```bash
cd ~/taskwing-tests/markwise.app

(echo '{"jsonrpc":"2.0","method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}},"id":1}';
 echo '{"jsonrpc":"2.0","method":"initialized","params":{},"id":2}';
 echo '{"jsonrpc":"2.0","method":"tools/call","params":{"name":"project-context","arguments":{"query":"database"}},"id":3}';
 sleep 2) | tw mcp 2>&1 | grep -i "mongo\|lance"
```

**Expected:**
- Response contains MongoDB/LanceDB related nodes
- Results are filtered (not all nodes returned)

### MCP-04: Local Install

```bash
cd ~/taskwing-tests/markwise.app
rm -rf .claude .cursor .windsurf .gemini

tw mcp install claude
tw mcp install cursor

cat .claude/mcp.json
cat .cursor/mcp.json
```

**Expected:**
- Files created in **project directory** (not ~/.claude)
- Each contains `"taskwing"` server entry
- `command` points to taskwing binary
- No `cwd` field (runs from current dir)

### MCP-05: Global Install

```bash
cd ~/taskwing-tests/markwise.app

tw mcp install claude --global

cat ~/.claude/mcp.json | grep -A 5 "taskwing-markwise"
```

**Expected:**
- Server added to `~/.claude/mcp.json`
- Server name is project-specific: `taskwing-markwise.app`
- Includes `cwd` pointing to project path

### MCP-06: Codex Manual Config

```bash
cd ~/taskwing-tests/markwise.app
mkdir -p .codex
cat > .codex/mcp.json <<'JSON'
{
  "mcp": {
    "servers": {
      "taskwing": {
        "command": "/usr/local/bin/taskwing",
        "args": ["mcp"]
      }
    }
  }
}
JSON

cat .codex/mcp.json
```

**Expected:**
- `.codex/mcp.json` exists in project root
- Contains a `taskwing` server entry
