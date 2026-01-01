# TaskWing: AI-Native Task Management

[![Go Report Card](https://goreportcard.com/badge/github.com/josephgoksu/TaskWing)](https://goreportcard.com/report/github.com/josephgoksu/TaskWing)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

> **Generate context-aware development tasks that actually match your architecture.**
> No more generic AI suggestions that ignore your patterns, constraints, and decisions.

## 🚀 The Problem

You ask an AI to "add Stripe billing" and it suggests:
- Libraries you've banned ❌
- Patterns you don't use ❌
- Files that don't exist ❌

Traditional task management (Jira, Asana) treats code as external. AI tools hallucinate because they don't understand *your* architecture.

## ✅ The Solution

TaskWing extracts knowledge from your codebase and uses it to generate accurate tasks:

```bash
$ tw plan new "Add Stripe subscription billing"

✓ Analyzed codebase (46 nodes, 22 decisions, 12 patterns)
✓ Generated 7 tasks based on your architecture

Plan: stripe-billing
  [ ] T1: Add Stripe SDK to backend-go (see: go.mod, internal/payments/)
  [ ] T2: Create subscription webhook handler (pattern: internal/api/handlers/)
  [ ] T3: Add billing_status to User model (constraint: use types.gen.go)
  [ ] T4: Update OpenAPI spec with /billing endpoints (workflow: make generate-api)
  [ ] T5: Implement frontend billing page (see: web/src/pages/)
  [ ] T6: Add Stripe keys to SSM (policy: no .env in prod)
  [ ] T7: Update CDK for billing service IAM (see: cdk/lib/)
```

Every task references **your actual files, patterns, and constraints**.

## ⚡ Quick Start

```bash
# 1. Install
curl -fsSL https://taskwing.app/install.sh | bash

# 2. Bootstrap your repo (extract knowledge)
cd /path/to/your/repo
tw bootstrap

# 3. Generate a plan
tw plan new "Implement user authentication"

# 4. Start working (provides context to AI tools via MCP)
tw plan start auth
tw mcp
```

## 📈 Validated Impact

We tested AI responses with and without TaskWing context on a real production codebase (Go backend, React frontend, OpenAPI codegen):

| Configuration | Avg Score (0-10) | Pass Rate | Notes |
|---------------|------------------|-----------|-------|
| **Baseline** (no context) | 3.6 | 0% | Wrong tech stack, wrong file paths |
| **With TaskWing** | **8.0** | 100% | **+122%** - Correct architecture throughout |

```
Score History (gpt-5-mini-2025-08-07)
─────────────────────────────────────────────────────
  no-context     | Avg: 3.6 | T1: 6  T2: 3  T3: 3  T4: 3  T5: 3
  with-taskwing  | Avg: 8.0 | T1: 8  T2: 8  T3: 8  T4: 8  T5: 8
```

**Why baseline fails:** Without context, models assume TypeScript/Next.js patterns (`src/types/openapi.ts`, `npm run generate`). With TaskWing, they correctly reference Go paths (`internal/api/types.gen.go`, `make generate-api`).

> TaskWing context injection helps AI tools understand your actual architecture. [Full methodology →](docs/development/EVALUATION.md)

👉 **[Full Getting Started Guide](docs/development/GETTING_STARTED.md)** · **[Product Vision](docs/PRODUCT_VISION.md)**

## 📚 Knowledge Architecture

We organize documentation to be trusted, actionable, and scalable.

| Scope | Directory | Purpose |
|-------|-----------|---------|
| **The Constitution** | [`docs/architecture/`](docs/architecture/) | **Immutable Principles.** System design, data privacy, and roadmap. |
| **The Playbook** | [`docs/development/`](docs/development/) | **Developer Guide.** Internals, agent architecture, and testing. |
| **The Reference** | [`docs/reference/`](docs/reference/) | **Facts.** Telemetry policy, error codes, integrations. |

<!--
## 🏢 Enterprise & Teams

Using TaskWing in a team? We provide:
*   **Shared Knowledge Graph**: Sync context across your entire engineering org.
*   **Governance**: Enforce architectural constraints automatically.

[Contact Sales](mailto:enterprise@taskwing.app) for early access to TaskWing Cloud.
-->

## License

MIT. Built for engineers, by engineers.
