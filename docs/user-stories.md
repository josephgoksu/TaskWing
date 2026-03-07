# TaskWing User Stories

## 1. Backend Engineer Debugging a Production Bug

> "Our payment webhook is silently dropping events. I need to find the root cause and fix it."

```bash
taskwing bootstrap          # Index codebase, auto-analyze architecture
taskwing plan "Fix payment webhook dropping events silently"
taskwing execute                      # AI CLI investigates, traces the bug, applies the fix
```

TaskWing feeds the AI CLI your architecture context — retry policies, error handling patterns, webhook middleware chain — so it doesn't waste time rediscovering your codebase. The fix lands with tests that cover the exact failure mode.

---

## 2. Full-Stack Engineer Building a Feature Across Layers

> "We need user profile avatars — upload on the frontend, storage on the backend, CDN serving."

```bash
taskwing plan "Add user profile avatar upload with CDN-backed serving"
taskwing plan status                  # Review: API endpoint, S3 upload, React component, CDN config
taskwing execute --all                # Executes tasks in dependency order across the stack
```

TaskWing decomposes the goal into ordered tasks — backend storage first, then API endpoint, then frontend component — each with acceptance criteria. The AI CLI implements each layer knowing how your existing upload patterns, auth middleware, and component library work.

---

## 3. DevOps Engineer Setting Up CI/CD for a New Service

> "We spun up a new Go microservice. It needs the same CI/CD pipeline as our other services."

```bash
taskwing bootstrap
taskwing plan "Add CI/CD pipeline matching existing service patterns"
taskwing execute --all
```

Bootstrap captures your existing GitHub Actions workflows, Docker build conventions, and deployment constraints. The AI CLI replicates the pattern for the new service — same linting, same test matrix, same deploy targets — without you copying and pasting YAML.

---

## 4. Tech Lead Paying Down Technical Debt

> "Our auth module has three different token validation paths. Consolidate them before they cause a security incident."

```bash
taskwing bootstrap          # Auto-surfaces the debt: three validation paths, confidence 0.9
taskwing plan "Consolidate auth token validation into a single middleware"
taskwing plan status                  # Review: extract shared validator, migrate callers, remove dead paths
taskwing execute
```

Bootstrap's debt classification flags the problem with evidence — file paths, line numbers, grep patterns. The plan ensures callers are migrated one by one with tests at each step, not a risky big-bang rewrite.

---

## 5. Mobile Engineer Adding Offline Support

> "The app crashes when users lose connectivity mid-sync. We need proper offline queueing."

```bash
taskwing plan "Add offline request queueing with retry on reconnection"
taskwing execute --all
```

TaskWing knows your networking layer, state management patterns, and existing retry logic from bootstrap. Tasks are ordered: queue data structure first, then interceptor integration, then UI indicators, then edge-case tests.

---

## 6. Junior Developer Onboarding to a New Codebase

> "I just joined the team. I need to add a simple health check endpoint but I don't know where anything is."

```bash
taskwing bootstrap          # Builds a knowledge map of the codebase (auto-analyzes)
taskwing plan "Add /healthz endpoint with database connectivity check"
taskwing execute
```

The junior doesn't need to spend days reading code. TaskWing's bootstrap extracts the routing patterns, middleware chain, and database connection setup. The AI CLI follows the existing conventions exactly — same error format, same middleware stack, same test style.

---

## 7. Data Engineer Building a New Pipeline Stage

> "We need to add a deduplication step between ingestion and transformation in our ETL pipeline."

```bash
taskwing bootstrap
taskwing plan "Add deduplication stage between ingestion and transformation"
taskwing execute --all
```

Bootstrap maps the pipeline architecture — message formats, checkpoint patterns, idempotency keys. The AI CLI adds the new stage following the same patterns: same logging, same metrics, same error recovery, same config structure.

---

## 8. Security Engineer Hardening an API

> "Pen test flagged rate limiting gaps and missing input validation on three endpoints."

```bash
taskwing plan "Add rate limiting and input validation to user, payment, and admin endpoints"
taskwing plan status                  # Review: rate limiter middleware, per-endpoint validation, integration tests
taskwing execute --all
```

TaskWing decomposes the hardening into isolated tasks — middleware first, then endpoint-specific validation, then abuse-scenario tests. Each task carries the security constraints from bootstrap so the AI CLI knows your existing auth patterns and validation library.

---

## 9. Platform Engineer Migrating a Database

> "We're moving from Postgres to CockroachDB. Need to audit and fix all incompatible queries."

```bash
taskwing bootstrap          # Auto-captures all SQL patterns, ORM usage, query builders
taskwing plan "Migrate from Postgres to CockroachDB — fix incompatible queries"
taskwing execute
```

Bootstrap inventories every raw SQL query, ORM call pattern, and migration file. The plan targets each incompatibility — serial vs UUID, transaction semantics, upsert syntax — as a separate task with validation queries to prove correctness.

---

## 10. Founder Shipping an MVP Feature Solo

> "I need to add Stripe subscription billing by Friday. Backend, frontend, webhooks, the works."

```bash
taskwing plan "Add Stripe subscription billing with plan selection UI and webhook handling"
taskwing plan status                  # 6 tasks: config, models, API, webhooks, UI, e2e tests
taskwing execute --all
```

One person, one command. TaskWing breaks the feature into the right build order, gives each task the full architecture context, and the AI CLI implements it end-to-end. You review the code, not write it from scratch.
