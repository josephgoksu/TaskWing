# TaskWing Eval: Quickstart Guide

This guide explains how to run TaskWing's evaluation system on any repository.

## Prerequisites

1. **TaskWing CLI** installed (`tw` binary in PATH)
2. **LLM API Key** configured (OpenAI, Anthropic, or Gemini)
3. **Repository bootstrapped** with `tw bootstrap`

## Quick Start (5 Minutes)

### Step 1: Bootstrap the Repository

First, analyze your codebase to populate TaskWing's knowledge graph:

```bash
cd /path/to/your/repo
tw bootstrap
```

This takes 2-5 minutes and extracts architectural decisions, patterns, and constraints.

### Step 2: Generate Evaluation Tasks

Auto-generate tasks based on your project's constraints:

```bash
tw eval --model gpt-5-mini-2025-08-07 generate-tasks --count 10
```

This creates `.taskwing/eval/tasks.yaml` with 10 evaluation tasks.

> **Note**: `tw eval init` is optional. The `generate-tasks` command creates `tasks.yaml`, and `eval run` auto-creates the prompt template if missing.

### Step 3: Run Evaluations

#### Case A: Baseline (No Context)
Test how an LLM performs without any project knowledge:

```bash
tw eval --model gpt-5-mini-2025-08-07 run --no-context --label baseline
```

#### Case B: TaskWing Context
Test with TaskWing's knowledge graph injected:

```bash
tw eval --model gpt-5-mini-2025-08-07 run --label taskwing
```

### Step 4: View Results

Compare performance across runs:

```bash
tw eval benchmark
```

## Complete Benchmark Matrix

For thorough comparison, run all 4 cases:

```bash
# Case A: Baseline (no context)
tw eval --model gpt-5-mini-2025-08-07 run --no-context --label baseline

# Case B: TaskWing Only
tw eval --model gpt-5-mini-2025-08-07 run --label taskwing

# Case C: External Tool Native (e.g., Codex)
tw eval --model gpt-5.2-codex run \
  --runner "codex exec --approval-mode full-auto < {prompt_file}" \
  --no-context \
  --label codex-native

# Case D: Combined (TaskWing + External Tool)
tw eval --model gpt-5.2-codex run \
  --runner "codex exec --approval-mode full-auto < {prompt_file}" \
  --label tw+codex
```

## Commands Reference

| Command | Description |
|---------|-------------|
| `tw eval generate-tasks` | Auto-generate evaluation tasks from memory |
| `tw eval run` | Run tasks against models |
| `tw eval judge` | Evaluate outputs against rules |
| `tw eval benchmark` | Compare results across runs |
| `tw eval report --run <dir>` | View detailed results for a specific run |

## Common Flags

| Flag | Commands | Description |
|------|----------|-------------|
| `--model` | All | Model(s) to use (can specify multiple) |
| `--count N` | generate-tasks | Number of tasks to generate |
| `--label` | run | Tag for grouping runs in reports |
| `--no-context` | run | Skip TaskWing context injection |
| `--runner` | run | External command template |
| `--timeout` | run | Max time per task (default: 10m) |

## Troubleshooting

### "Not enough context in memory"
Run `tw bootstrap` first to populate the knowledge graph.

### "No task outputs found"
Ensure tasks completed successfully. Check `.taskwing/eval/runs/<timestamp>/` for output files.

### "Unknown flag: --model"
Update to the latest TaskWing CLI. The `--model` flag is now on the parent `eval` command.
