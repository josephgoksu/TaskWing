# TaskWing Eval: Architecture & Internals

This document explains the internal flow of the evaluation system.

## Command Flow

When you run:
```bash
tw eval run -m openai:gpt-4o --label "with-tw" --out .taskwing/eval/runs/test-with
```

The following sequence executes:

```
┌─────────────────────────────────────────────────────────────────────────────┐
│  tw eval run -m openai:gpt-4o --label "with-tw" --out .../test-with         │
└─────────────────────────────────────────────────────────────────────────────┘
                                      │
                                      ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│  1. PARSE FLAGS                                                             │
│     • model = "openai:gpt-4o"                                               │
│     • label = "with-tw"                                                     │
│     • outDir = ".taskwing/eval/runs/test-with"                              │
│     • injectContext = true (default, no --no-context)                       │
└─────────────────────────────────────────────────────────────────────────────┘
                                      │
                                      ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│  2. LOAD CONFIG                                                             │
│     • Read .taskwing/eval/tasks.yaml → tasks[], rules[]                     │
│     • Read .taskwing/eval/prompts/task.txt → prompt template                │
└─────────────────────────────────────────────────────────────────────────────┘
                                      │
                                      ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│  3. BOOTSTRAP (subprocess)                                                  │
│     ┌─────────────────────────────────────────────────────────────────┐     │
│     │  $ tw bootstrap --quiet                                         │     │
│     │    → Analyzes repo, extracts decisions/constraints              │     │
│     │    → Writes to .taskwing/memory/memory.db                       │     │
│     └─────────────────────────────────────────────────────────────────┘     │
└─────────────────────────────────────────────────────────────────────────────┘
                                      │
                                      ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│  4. FOR EACH TASK (T1, T2, ...)                                             │
│  ┌──────────────────────────────────────────────────────────────────────┐   │
│  │                                                                      │   │
│  │  4a. CONTEXT RETRIEVAL (subprocess)                                  │   │
│  │      ┌────────────────────────────────────────────────────────┐      │   │
│  │      │  $ tw context "<task.prompt>"                          │      │   │
│  │      │    → Queries memory.db with task prompt                │      │   │
│  │      │    → Returns relevant decisions, constraints, patterns │      │   │
│  │      └────────────────────────────────────────────────────────┘      │   │
│  │                              │                                       │   │
│  │                              ▼                                       │   │
│  │  4b. BUILD FINAL PROMPT                                              │   │
│  │      ┌────────────────────────────────────────────────────────┐      │   │
│  │      │  template (task.txt)                                   │      │   │
│  │      │    + {{task}} → task.prompt                            │      │   │
│  │      │    + {{repo}} → /path/to/repo                          │      │   │
│  │      │    + "## Project Context\n" + context_output  ◄────────│──────│── INJECTED
│  │      └────────────────────────────────────────────────────────┘      │   │
│  │                              │                                       │   │
│  │                              ▼                                       │   │
│  │  4c. SAVE PROMPT                                                     │   │
│  │      → .taskwing/eval/runs/test-with/prompts/task-T1-gpt-4o.txt      │   │
│  │                              │                                       │   │
│  │                              ▼                                       │   │
│  │  4d. CALL LLM (internal runner)                                      │   │
│  │      ┌────────────────────────────────────────────────────────┐      │   │
│  │      │  OpenAI API                                            │      │   │
│  │      │    model: gpt-4o                                       │      │   │
│  │      │    messages: [{ role: user, content: final_prompt }]   │      │   │
│  │      └────────────────────────────────────────────────────────┘      │   │
│  │                              │                                       │   │
│  │                              ▼                                       │   │
│  │  4e. SAVE OUTPUT                                                     │   │
│  │      → .taskwing/eval/runs/test-with/task-T1-gpt-4o.txt              │   │
│  │                                                                      │   │
│  └──────────────────────────────────────────────────────────────────────┘   │
│                                 │                                           │
│                        (repeat for T2, T3, ...)                             │
└─────────────────────────────────────────────────────────────────────────────┘
                                      │
                                      ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│  5. JUDGE EACH OUTPUT (--judge=true by default)                             │
│  ┌──────────────────────────────────────────────────────────────────────┐   │
│  │  For each task-*.txt output:                                         │   │
│  │                                                                      │   │
│  │  ┌────────────────────────────────────────────────────────────┐      │   │
│  │  │  LLM Judge Prompt:                                         │      │   │
│  │  │    • Task: <original prompt>                               │      │   │
│  │  │    • Expected: <task.expected>                             │      │   │
│  │  │    • Failure Signals: <task.failure_signals>               │      │   │
│  │  │    • AI Response: <model output>                           │      │   │
│  │  │    → Score 0-10 + reason                                   │      │   │
│  │  └────────────────────────────────────────────────────────────┘      │   │
│  │                              │                                       │   │
│  │                              ▼                                       │   │
│  │  { "score": 8, "reason": "Correctly identified..." }                 │   │
│  │                                                                      │   │
│  └──────────────────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────────────────┘
                                      │
                                      ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│  6. WRITE RESULTS                                                           │
│     → .taskwing/eval/runs/test-with/results.json                            │
│                                                                             │
│     {                                                                       │
│       "generatedAt": "2025-12-31T...",                                      │
│       "label": "with-tw",                                                   │
│       "context_mode": "taskwing",                                           │
│       "results": [                                                          │
│         { "task": "T1", "model": "gpt-4o", "score": 8, ... },               │
│         { "task": "T2", "model": "gpt-4o", "score": 7, ... }                │
│       ],                                                                    │
│       "summary": { "gpt-4o": { "total": 2, "avg_score": 7.5 } }             │
│     }                                                                       │
└─────────────────────────────────────────────────────────────────────────────┘
```

## Context Injection: With vs Without

The `--no-context` flag controls whether TaskWing knowledge is injected:

```
WITH CONTEXT (default)              │  WITHOUT CONTEXT (--no-context)
────────────────────────────────────│────────────────────────────────────
Step 3: Bootstrap runs              │  Step 3: SKIPPED
Step 4a: tw context retrieves       │  Step 4a: SKIPPED
         project knowledge          │
Step 4b: Context appended to prompt │  Step 4b: No context appended
                                    │
context_mode: "taskwing"            │  context_mode: "none"
```

The **only variable** between the two runs is whether `## Project Context\n...` gets appended to the prompt.

## Internal Runner vs Agent

The default **internal runner** uses a single LLM API call — it is NOT an agent:

```
┌─────────────────────────────────────────────────────────────────────────────┐
│  INTERNAL RUNNER: Single LLM Call                                           │
└─────────────────────────────────────────────────────────────────────────────┘

     ┌──────────────┐         ┌──────────────┐         ┌──────────────┐
     │   Prompt     │────────▶│   LLM API    │────────▶│   Text Out   │
     │   (string)   │         │  (1 request) │         │   (string)   │
     └──────────────┘         └──────────────┘         └──────────────┘
                                    │
                                    │  chatModel.Generate(prompt)
                                    │  → Single request/response
                                    │  → No tools
                                    │  → No iteration
                                    │  → No file access
```

External runners (via `--runner` flag) CAN be agents:

```
┌─────────────────────────────────────────────────────────────────────────────┐
│  EXTERNAL RUNNER: Agent (e.g., Codex, Claude Code)                          │
└─────────────────────────────────────────────────────────────────────────────┘

     ┌──────────┐      ┌─────────────────────────────────────────────┐
     │  Task    │─────▶│              Agent Loop                     │
     └──────────┘      │  ┌─────────────────────────────────────┐    │
                       │  │  Think → Act → Observe → Repeat     │    │
                       │  │                                     │    │
                       │  │  Tools:                             │    │
                       │  │   • Read files                      │    │
                       │  │   • Write files                     │    │
                       │  │   • Run shell commands              │    │
                       │  │   • Search codebase                 │    │
                       │  │   • Make multiple LLM calls         │    │
                       │  └─────────────────────────────────────┘    │
                       │                    │                        │
                       │                    ▼                        │
                       │           Actual code changes               │
                       └─────────────────────────────────────────────┘
```

### Implications for Evaluation

| Aspect | Internal Runner (LLM Call) | External Runner (Agent) |
|--------|---------------------------|------------------------|
| Output | Text describing what it *would* do | Actual code changes |
| Verification | LLM judge reads text | Can compile, run tests, diff |
| Reality check | None - could be hallucinated | Grounded in actual files |
| What you measure | "Can LLM describe the right approach?" | "Can LLM actually implement it?" |

## Code References

- **Command definition**: `cmd/eval.go:34-654`
- **Task execution**: `cmd/eval.go:691-811` (`runEvalTasks`)
- **Internal LLM call**: `cmd/eval.go:814-849` (`runEvalTaskInternal`)
- **LLM Judge**: `cmd/eval.go:851-933` (`runLLMJudge`)
- **Types**: `internal/eval/types.go`
- **Config loading**: `internal/eval/config.go`
- **Templates**: `internal/eval/templates.go`

## Output Directory Structure

After a run, the output directory contains:

```
.taskwing/eval/runs/<timestamp>/
├── memory/
│   └── <model>/           # Model-specific memory snapshot
├── prompts/
│   ├── task-T1-<model>.txt    # Final prompt sent to LLM
│   └── task-T2-<model>.txt
├── task-T1-<model>.txt        # LLM response
├── task-T2-<model>.txt
└── results.json               # Scores and summary
```

## Known Limitations

1. **LLM-as-Judge bias**: Same model family generates and judges responses
2. **No objective verification**: Responses are scored semantically, not tested
3. **Single-turn only**: Internal runner doesn't support multi-turn or tool use
4. **Prompt template contamination**: Baseline prompts may still reference TaskWing

See [EVALUATION.md](EVALUATION.md) for usage quickstart.

---

## External Runners

Use external AI coding agents (Claude Code, Codex, Gemini CLI) as runners for TaskWing benchmarks.

### Prerequisites

Ensure the external tool is installed and authenticated:

- **Claude Code**: `claude` CLI
- **OpenAI Codex**: `codex` CLI
- **Gemini CLI**: `gemini` CLI

### Runner Command Template

The `--runner` flag accepts a command template. Use `{prompt_file}` as a placeholder for the prompt path.

```bash
tw eval --model <model> run \
  --runner "<command> < {prompt_file}" \
  --label <label>
```

### Examples

#### Claude Code

```bash
# Native (Claude's own context)
tw eval --model claude-sonnet run \
  --runner "claude -p < {prompt_file}" \
  --no-context \
  --label claude-native

# Combined (TaskWing + Claude)
tw eval --model claude-sonnet run \
  --runner "claude -p < {prompt_file}" \
  --label tw+claude
```

#### OpenAI Codex

```bash
# Native (Codex's own context, no TaskWing)
tw eval run \
  --runner "codex exec --full-auto < {prompt_file}" \
  --no-context \
  --label codex-native

# Combined (TaskWing context + Codex)
tw eval run \
  --runner "codex exec --full-auto < {prompt_file}" \
  --label codex-taskwing

# With specific model label
tw eval --model o4-mini run \
  --runner "codex exec -m o4-mini --full-auto < {prompt_file}" \
  --label codex-o4
```

> **Note**: `--model` is optional for external runners. If omitted, defaults to `<runner>-default`.

> **For agentic runners**: Use `--sandbox read-only` to prevent file modifications, or `--reset-repo` to reset git state between tasks.

#### Gemini CLI

```bash
tw eval --model gemini-2.0-flash run \
  --runner "gemini -p < {prompt_file}" \
  --no-context \
  --label gemini-native
```

### Runner Flags Reference

| Flag | Description | Required |
|------|-------------|----------|
| `--runner` | Command template with `{prompt_file}` placeholder | Yes |
| `--label` | Run identifier for grouping in reports | **Yes** |
| `--no-context` | Skip TaskWing context injection (baseline mode) | No |
| `--timeout` | Max execution time per task (default: `10m`) | No |
