# TaskWing Roadmap

> **Vision:** Institutional Knowledge Layer for Engineering Teams

---

## Version 2.0 — Solo Developer ✅

**Status:** Feature Complete
**Theme:** Core CLI for individual developers

| Feature | Status |
|---------|--------|
| `tw add "text"` — AI-classified knowledge | ✅ |
| `tw list` — View nodes by type | ✅ |
| `tw context "query"` — Semantic search | ✅ |
| `tw context --answer` — RAG answers | ✅ |
| `tw bootstrap` — Auto-generate from repo | ✅ |
| `tw mcp` — AI tool integration | ✅ |
| `tw delete` — Remove nodes | ✅ |
| Dynamic file discovery (40+ patterns) | ✅ |
| Evidence-based findings (file:line refs) | ✅ |
| Deterministic verification (VerificationAgent) | ✅ |

---

## Version 2.1 — Team Collaboration

**Status:** Not Started
**Theme:** Shared knowledge across team members

| Feature | Priority |
|---------|----------|
| Cloud sync (remote backend) | P0 |
| Team workspaces | P0 |
| Permissions (owner, editor, viewer) | P1 |
| Conflict resolution | P1 |
| Activity feed | P2 |

---

## Version 2.2 — Integrations + Semantic Verification

**Status:** Not Started
**Theme:** Meet developers where they work + smarter verification

| Feature | Priority |
|---------|----------|
| GitHub PR comments (auto-context) | P0 |
| **Semantic LLM Verification** | P0 |
| Slack notifications | P1 |
| Linear integration | P2 |
| VSCode extension | P2 |

### Semantic LLM Verification (v2.2)

After deterministic checks pass, use an LLM to semantically validate findings:

| Check | Purpose |
|-------|---------|
| **Evidence Relevance** | Does the snippet actually support the claimed decision? |
| **Reasoning Validation** | Does the "why" make sense given the code context? |
| **Staleness Detection** | Has the code changed since the finding was created? |
| **Cross-Reference** | Do multiple findings about the same topic contradict? |

**Implementation Plan:**
1. Create `SemanticVerificationAgent` (uses LLM, not deterministic)
2. Run after `VerificationAgent` for findings with `status=verified` or `status=partial`
3. Add `semantic_verification_result` column to nodes table
4. Configurable: opt-in via `--semantic-verify` flag (costs API tokens)

**Why Separate from Deterministic?**
- Deterministic verification is fast, free, and catches obvious errors
- Semantic verification is slow, costs tokens, but catches subtle issues
- Users can choose their quality/cost tradeoff

---

## Version 3.0 — Continuous Intelligence

 **Status:** Future
 **Theme:** Proactive knowledge management (Always-on Watch Agent)

 | Feature | Description |
 |---------|-------------|
 | `WatchAgent` | Monitor filesystem via fsnotify |
 | Debounced Analysis | Update memory in real-time (<2s latency) |
 | Relationship Synthesis | Cross-reference findings across agents |
 | Auto-update nodes | Detect when decisions become stale |
 | Knowledge gaps detection | "You have no docs for auth" |

---

## Version 4.0 — Organization Knowledge

**Status:** Future
**Theme:** Cross-project knowledge graph

| Feature | Description |
|---------|-------------|
| Multi-repo knowledge | Link decisions across projects |
| Org-wide search | "How do we handle auth?" |
| Knowledge templates | Reusable decision patterns |
| Compliance tracking | "All projects must use X" |

---

## Version 5.0 — AI Workflows

**Status:** Future
**Theme:** AI-driven knowledge automation

| Feature | Description |
|---------|-------------|
| PR reviewer | Auto-suggest decisions from PRs |
| Onboarding assistant | New dev Q&A bot |
| Decision recommender | "Based on similar projects..." |
| Knowledge analytics | Usage, gaps, trends |

---

## Release Cadence

| Version | Target |
|---------|--------|
| 2.0 | ✅ Released |
| 2.1 | Q1 2025 |
| 2.2 | Q2 2025 |
| 3.0 | H2 2025 |

---

## References

- [SYSTEM_DESIGN.md](./SYSTEM_DESIGN.md) — System design
- [BOOTSTRAP_INTERNALS.md](../development/BOOTSTRAP_INTERNALS.md) — Bootstrap feature details
- [GETTING_STARTED.md](../development/GETTING_STARTED.md) — User guide
