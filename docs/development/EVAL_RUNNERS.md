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
# Native
tw eval --model gpt-5.2-codex run \
  --runner "codex exec --approval-mode full-auto < {prompt_file}" \
  --no-context \
  --label codex-native

# Combined
tw eval --model gpt-5.2-codex run \
  --runner "codex exec --approval-mode full-auto < {prompt_file}" \
  --label tw+codex
```

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
  Model                              Run 1        Run 2
  baseline (gpt-4o)                    40%           ─
  claude-native (claude-sonnet)         ─          75%
  tw+claude (claude-sonnet)             ─          85%
```
