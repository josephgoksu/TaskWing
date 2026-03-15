# Multi-Agent Bootstrap Pipeline - Implementation Plan

**Date:** 2026-03-15
**Status:** Planned
**Target:** v1.22.0 (Phase 1), v1.23.0 (Phase 2), v1.24.0 (Phase 3)

## Overview

Add 4 internal post-processing agents to the bootstrap pipeline that cross-validate,
refine, and synthesize findings. Users run `taskwing bootstrap` and get better output.
No user-facing configuration. Invisible quality improvement.

## Current Pipeline

```
DocAgent ──┐
CodeAgent ─┤──→ aggregate ──→ verify evidence ──→ deduplicate ──→ store
GitAgent  ─┤
DepsAgent ─┘
```

## New Pipeline

```
DocAgent ──┐
CodeAgent ─┤──→ aggregate ──→ verify evidence ──→ deduplicate
GitAgent  ─┤
DepsAgent ─┘
              ──→ CrossChecker (find contradictions)
              ──→ CoverageAuditor (find gaps, re-extract)
              ──→ Refiner (sharpen vague findings)
              ──→ Synthesizer (produce architecture narrative)
              ──→ store
```

Each phase is independently skippable on failure (graceful degradation).
No backward compatibility flag. Post-processing is always on. No --skip-postprocess.

---

## Agent Roster

### 1. Synthesizer
- **Role:** Produce coherent architecture narrative from all findings
- **Input:** All verified findings + project metadata
- **Output:** ArchitectureSummary node (overview, style, decisions, data flow, risks)
- **Model:** User's configured model. Ollama 14B+ recommended.
- **Ship:** v1.22.0

### 2. CrossChecker
- **Role:** Find contradictions between agent findings
- **Input:** All findings grouped by source agent
- **Output:** Contradiction list + unsupported claims. Confidence adjustments.
- **Model:** Works on 7B local models. Structured comparison task.
- **Ship:** v1.22.0

### 3. Refiner
- **Role:** Sharpen vague/low-confidence findings using actual source code
- **Input:** Findings with confidence < 0.8 + contradicted findings + source code
- **Output:** Revised findings with updated descriptions and confidence
- **Model:** Needs strong model. 14B+ local, Sonnet/GPT-4o-mini cloud.
- **Ship:** v1.23.0

### 4. CoverageAuditor
- **Role:** Find important unanalyzed code areas, trigger re-extraction
- **Input:** File tree + covered files set
- **Output:** Top 5 coverage gaps. Re-runs CodeAgent on top 3.
- **Model:** Simplest task. 7B works fine.
- **Ship:** v1.24.0

---

## Package Structure

```
internal/agents/postprocess/     -- NEW PACKAGE
  postprocess.go                 -- Pipeline orchestrator
  context.go                     -- Shared state between phases
  cross_checker.go               -- CrossChecker agent
  coverage_auditor.go            -- CoverageAuditor agent
  refiner.go                     -- Refiner agent
  synthesizer.go                 -- Synthesizer agent
  prompts.go                     -- Prompt templates
```

Modified files:
- `internal/agents/core/types.go` -- add Contradiction, CoverageGap, UnsupportedClaim, ArchitectureSummary
- `internal/bootstrap/service.go` -- insert pipeline call between verification and ingestion

---

## Communication Protocol

Agents share state via `postprocess.Context` (passed by pointer):
- Each agent reads what it needs, writes its output
- No agent-to-agent messaging
- FinalFindings() method applies all refinements and removals

Each agent gets minimal context:
- CrossChecker: all findings grouped by agent
- CoverageAuditor: file tree + covered files set
- Refiner: only low-confidence + contradicted findings + source code
- Synthesizer: all final findings

---

## Prompt Architecture

Key techniques:
- **CrossChecker**: "You MUST identify at least 3 items" (prevents "looks good")
- **CoverageAuditor**: Contrast file tree vs covered files (simple pattern matching)
- **Refiner**: "You may NOT leave any finding unchanged" (forces actual review)
- **Synthesizer**: "Every sentence must reference a specific file path" (prevents generic output)

---

## Model Strategy

| Agent | Min Local | Recommended Local | Cloud |
|-------|----------|-------------------|-------|
| CrossChecker | 7B | Qwen 2.5 14B | Haiku |
| CoverageAuditor | 7B | Qwen 2.5 7B | Haiku |
| Refiner | 14B | Qwen 2.5 32B | Sonnet |
| Synthesizer | 14B | Qwen 2.5 32B | Sonnet |

Principle: never make cloud calls the user didn't configure. If Ollama, everything local.

---

## Failure Modes

Each phase has:
- 90-second timeout (local) / 30-second timeout (cloud)
- JSON parse retry (2 attempts with repair)
- Graceful skip on failure (next phase runs with unmodified findings)
- Phase results logged for debugging

Degradation order: Full -> no narrative -> no gaps -> no refinement -> no cross-check

Safeguards:
- Refiner: cap removals at 20% of total findings
- CrossChecker: cap confidence reduction at 0.2 per finding
- CoverageAuditor: re-extract top 3 gaps only, 60s timeout
- Synthesizer: require >5 file paths in output or skip

---

## Ship Schedule

| Week | Deliverable | Version |
|------|-------------|---------|
| Week 1 | Pipeline + Synthesizer + CrossChecker | dev |
| Week 2 | Testing, prompt tuning, Ollama validation | dev |
| Week 2 end | **v1.22.0** release | v1.22.0 |
| Week 3 | Refiner + testing | dev |
| Week 3 end | **v1.23.0** release | v1.23.0 |
| Week 4 | CoverageAuditor + testing | dev |
| Week 4 end | **v1.24.0** release | v1.24.0 |

---

## Success Metrics

| Metric | Current | Target |
|--------|---------|--------|
| Architecture summary exists | No | Yes, with >5 file refs |
| Contradictions detected | 0 | >1 per repo with doc drift |
| Avg finding confidence | ~0.72 | >0.80 after refinement |
| File coverage | ~35-50% | >65% after gap-fill |
| Bootstrap Quality Score | ~0.50 | >0.75 |

BQS = (0.3 * coverage) + (0.3 * avgConfidence) + (0.2 * (1 - contradictionRate)) + (0.2 * hasSummary)
