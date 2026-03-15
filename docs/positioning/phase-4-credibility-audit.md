# Phase 4: Security & Credibility Audit

**Date:** 2026-03-14
**Status:** Complete
**Depends on:** Phase 3 (copy recommendations)

## Purpose

Audit every sovereignty/privacy claim in the proposed copy for technical accuracy.
Flag anything that overpromises. Suggest qualifiers.

## Claim-by-Claim Audit

### Claim 1: "Your knowledge base is 100% local"
- **Verdict: TRUE**
- SQLite database lives at `.taskwing/memory/memory.db` on the local filesystem
- No cloud sync, no remote backup, no phone-home
- MCP queries are served over local stdio -- no network calls
- **No qualifier needed**

### Claim 2: "No cloud. No account."
- **Verdict: TRUE with qualifier**
- No TaskWing account exists. No registration endpoint. No user database.
- However: during bootstrap, code context IS sent to an LLM API (OpenAI, Anthropic, Google, Bedrock)
- **Required qualifier**: "TaskWing has no cloud service and no account system. During initial architecture extraction, code context is processed by your chosen LLM provider according to their data policies. After extraction, all knowledge stays local."
- **Recommendation**: Add this to a "How it works" section. Don't bury it -- transparency builds more trust than an absolute claim that gets debunked.

### Claim 3: "Every AI session after that just knows -- without your codebase leaving your infrastructure"
- **Verdict: PARTIALLY TRUE -- needs scoping**
- After bootstrap, MCP queries ARE fully local (stdio, no network)
- BUT: `taskwing plan` and `taskwing goal` make LLM API calls, sending plan context to the provider
- `taskwing debug` and `taskwing code explain` also use LLM APIs
- **Required qualifier**: Change "without your codebase leaving your infrastructure" to "without your knowledge base leaving your machine." The knowledge BASE is local; some COMMANDS still call LLM APIs.
- **Alternative**: "Your architectural knowledge stays on your machine. LLM-powered commands use your configured provider."

### Claim 4: "Fully air-gappable with Ollama"
- **Verdict: TRUE**
- With Ollama configured, zero external network calls are made
- Bootstrap, planning, and queries all go through local Ollama
- **No qualifier needed** (but note: Ollama model quality is lower than cloud providers)

### Claim 5: "Open source (MIT) -- audit every line"
- **Verdict: TRUE**
- Full source at github.com/josephgoksu/TaskWing, MIT license
- No "open core" -- all features in the open repo
- **No qualifier needed**

### Claim 6: "No telemetry by default"
- **Verdict: TRUE**
- Telemetry is opt-in with explicit consent prompt
- Auto-disabled in CI, non-interactive terminals
- PostHog analytics only collect: command names, success/failure, duration, OS, CLI version
- No code, file paths, or personal data collected
- **No qualifier needed**

### Claim 7: "Private by architecture"
- **Verdict: CREDIBLE but not absolute**
- The architecture IS private-by-default for storage and MCP queries
- But the architecture also includes LLM API calls for bootstrap/plan/debug
- **Recommendation**: Use "private by architecture" but define what it means: "Your knowledge base is stored locally and queried locally. LLM-powered analysis uses your chosen provider -- including fully local options via Ollama."

## Claims to AVOID

| Claim | Why | Risk |
|-------|-----|------|
| "Your code never leaves your machine" | False during bootstrap with cloud LLMs | Immediate credibility loss if called out |
| "Zero data collection" | Telemetry exists (opt-in). PostHog is a third party. | Technically defensible but easily misread |
| "GDPR compliant" | No formal DPA, no data protection officer, no DPIA conducted | Legal liability if claimed without basis |
| "SOC 2" or "enterprise-grade security" | No cert, no formal security audit | Enterprise buyers will verify and find nothing |
| "End-to-end encrypted" | Data is not encrypted at rest in SQLite by default | Factually wrong |

## Recommended "How It Works" Transparency Section

```
## How Your Data Flows

1. **Bootstrap** (one-time)
   Your code context is sent to your configured LLM provider for analysis.
   - Cloud providers: OpenAI, Anthropic, Google, AWS Bedrock
   - Fully local: Ollama (zero network calls)
   Provider data policies apply during this step.

2. **Knowledge Storage** (permanent, local)
   Extracted architecture (decisions, patterns, constraints) is stored
   in SQLite on your filesystem. Never synced. Never uploaded.

3. **AI Tool Queries** (ongoing, local)
   MCP queries from Claude, Cursor, Copilot etc. are served over
   local stdio. Your knowledge base never leaves your machine.

4. **Planning & Analysis** (on-demand)
   Commands like `goal`, `plan`, `debug` use your configured LLM.
   Same provider choice as bootstrap -- including Ollama for air-gap.
```

## Summary

| Proposed Copy | Accurate? | Action |
|--------------|-----------|--------|
| "knowledge base is local" | Yes | Keep |
| "no cloud, no account" | Yes (for TaskWing itself) | Keep, add LLM provider qualifier |
| "codebase never leaves your infrastructure" | Overstates | Change to "knowledge base stays local" |
| "air-gappable with Ollama" | Yes | Keep |
| "open source, audit every line" | Yes | Keep |
| "no telemetry by default" | Yes | Keep |
| "private by architecture" | Credible with definition | Keep, define clearly |
