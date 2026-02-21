# TaskWing Evaluation Methodology

## Overview

We evaluated whether injecting project-specific context via TaskWing
improves the quality of LLM-generated architectural responses compared
to a baseline (no context) scenario.

**Result: +122% improvement** (3.6 → 8.0 average score).

## Setup

| Parameter        | Value                                      |
|------------------|--------------------------------------------|
| **Codebase**     | Production Go/React monorepo               |
| **LLM judge**    | gpt-5-mini                                 |
| **Tasks**        | 5 architectural questions                  |
| **Scoring**      | 1–10 per task, averaged                    |
| **Conditions**   | Baseline (no context) vs TaskWing-injected |

## Tasks

Each task required the LLM to answer an architectural question about
the codebase. Correct answers required knowing:

1. The primary language (Go, not TypeScript)
2. Correct file paths and project structure
3. Correct build/generate commands
4. Architectural patterns and constraints
5. Technology decisions and their rationale

## Results

### Per-Task Scores

| Task | Without Context | With TaskWing | Delta |
|------|---------------:|-------------:|------:|
| T1   | 6              | 8            | +2    |
| T2   | 3              | 8            | +5    |
| T3   | 3              | 8            | +5    |
| T4   | 3              | 8            | +5    |
| T5   | 3              | 8            | +5    |
| **Avg** | **3.6**     | **8.0**      | **+4.4** |

**Improvement: +122%** (8.0 / 3.6 - 1)

### Without Context (Baseline)

The LLM without context consistently:
- Assumed TypeScript instead of Go
- Referenced nonexistent files like `src/types/openapi.ts`
- Suggested `npm run generate` instead of `make generate-api`
- Missed architectural constraints entirely

Only T1 scored above 3, likely due to generic reasoning.

### With TaskWing (Context Injected)

TaskWing's MCP integration provided the LLM with:
- **Decisions**: Technology choices and their rationale
- **Patterns**: File structure conventions and API patterns
- **Constraints**: Build requirements and deployment rules

The LLM consistently identified Go, referenced correct file paths
(`internal/api/types.gen.go`), and used correct commands.

## Scoring Criteria

- **8–10**: Correct language, correct paths, correct commands,
  respects constraints
- **5–7**: Partially correct; right language but wrong paths,
  or right paths but wrong commands
- **1–4**: Wrong language or fundamentally incorrect assumptions
- **Rule**: Wrong tech stack identification = automatic score ≤ 3

## What TaskWing Provides

During the evaluation, TaskWing injected the following context
via the MCP protocol:

```
Decisions:  22 (e.g., "PostgreSQL over MongoDB", "OpenAPI codegen")
Patterns:   12 (e.g., "internal/api/handlers/ convention")
Constraints: 9 (e.g., "No .env in production — use SSM")
```

This context was extracted automatically by `taskwing bootstrap`
in under 3 seconds.

## Reproducing

1. Clone any Go or multi-language repository
2. Run `taskwing bootstrap` to extract context
3. Ask the same architectural questions with and without
   TaskWing's MCP server connected
4. Score responses on a 1–10 scale using the criteria above

## Limitations

- Single codebase evaluated (Go/React monorepo)
- Single LLM judge model (gpt-5-mini)
- 5 tasks may not capture all architectural reasoning scenarios
- Scores are relative — absolute quality depends on the model used

We plan to expand this evaluation to more codebases and models
in future iterations.
