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

- [ROADMAP.md](./ROADMAP.md) — Version planning
- [ARCHITECTURE.md](./ARCHITECTURE.md) — System design
