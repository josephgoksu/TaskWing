# TaskWing Eval: External Runners

Use external AI coding agents (Claude Code, Codex, Gemini CLI) as runners for TaskWing benchmarks.

## Prerequisites

Ensure the external tool is installed and authenticated:

- **Claude Code**: `claude` CLI
- **OpenAI Codex**: `codex` CLI
- **Gemini CLI**: `gemini` CLI

## Runner Command Template

The `--runner` flag accepts a command template. Use `{prompt_file}` as a placeholder for the prompt path.

```bash
tw eval --model <model> run \
  --runner "<command> < {prompt_file}" \
  --label <label>
```

## Examples

### Claude Code

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

### OpenAI Codex

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

> **Note**: `--model` is optional for external runners. If omitted, defaults to `<runner>-default` (e.g., `codex-default`).

> **For agentic runners**: Use `--sandbox read-only` to prevent file modifications, or `--reset-repo` to reset git state between tasks:
> ```bash
> # Read-only mode (Codex explains but doesn't modify)
> tw eval run --runner "codex exec -s read-only --full-auto < {prompt_file}" --label codex-readonly
>
> # Reset mode (allows modifications but cleans up between tasks)
> tw eval run --runner "codex exec --full-auto < {prompt_file}" --reset-repo --label codex-reset
> ```

### Gemini CLI

```bash
# Native
tw eval --model gemini-2.0-flash run \
  --runner "gemini -p < {prompt_file}" \
  --no-context \
  --label gemini-native
```

## Flags Reference

| Flag | Description | Required |
|------|-------------|----------|
| `--runner` | Command template with `{prompt_file}` placeholder | Yes |
| `--label` | Run identifier for grouping in reports | **Yes** |
| `--no-context` | Skip TaskWing context injection (baseline mode) | No |
| `--timeout` | Max execution time per task (default: `10m`) | No |

## Viewing Results

```bash
tw eval benchmark
```

Output:
```
╭─────────────────────────────────────────╮
│  BENCHMARK RESULTS · 3 runs · 3 models  │
╰─────────────────────────────────────────╯

Score History

  Model                               | Avg      | T1   T2   T3   T4   T5
----------------------------------------------------------------------------------------------------
  baseline (gpt-5-nano)               | 6.6      |  6    7    8    5    7
  with-taskwing (gpt-5-nano)          | 7.8      |  8    8    8    9    6
  codex-readonly-with-taskwing        | 8.0      |  8    8    8    8    8

Overall Best (Avg)
  1. codex-readonly-with-taskwing   8.0
  2. with-taskwing                  7.8
  3. baseline                       6.6
```

> **Key Insight**: TaskWing context improved scores by +18-21%. The baseline model incorrectly assumed a Node.js backend; with TaskWing context, it correctly identified the Go backend and referenced the actual `make generate-api` workflow.
