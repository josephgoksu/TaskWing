#!/usr/bin/env bash
set -euo pipefail

TW_BIN="${TW_BIN:-tw}"
REPO_DIR="${REPO_DIR:-$(pwd)}"
DELAY_HOURS="${DELAY_HOURS:-0}"
TIMEOUT="${TIMEOUT:-2m}"
JUDGE="${JUDGE:-true}"
DRY_RUN="${DRY_RUN:-}"

INTERNAL_MODELS=(
  "gpt-5-mini-2025-08-07"
)

CODEX_MODELS=(
  "gpt-5.2-codex"
)

CLAUDE_MODELS=(
  "claude-sonnet"
)

GEMINI_MODELS=(
  "gemini-2.0-flash"
)

RUNNER_CODEX="${RUNNER_CODEX:-codex exec --approval-mode full-auto < {prompt_file}}"
RUNNER_CLAUDE="${RUNNER_CLAUDE:-claude -p < {prompt_file}}"
RUNNER_GEMINI="${RUNNER_GEMINI:-gemini -p < {prompt_file}}"

cd "$REPO_DIR"

if [[ ! "$DELAY_HOURS" =~ ^[0-9]+$ ]]; then
  echo "DELAY_HOURS must be a non-negative integer." >&2
  exit 1
fi

if [[ "$DELAY_HOURS" -gt 0 ]]; then
  echo "Sleeping for ${DELAY_HOURS} hour(s) before starting..."
  sleep "$((DELAY_HOURS * 3600))"
fi

run_cmd() {
  local -a args=("$@")
  local printable
  printf -v printable '%q ' "${args[@]}"
  echo "+ ${printable}"
  if [[ -n "$DRY_RUN" ]]; then
    return 0
  fi
  "${args[@]}"
}

echo "Starting TaskWing eval runs in $(pwd)"

for model in "${INTERNAL_MODELS[@]}"; do
  run_cmd "$TW_BIN" eval --model "$model" run --no-context --label "baseline" --timeout "$TIMEOUT" --judge="$JUDGE"
  run_cmd "$TW_BIN" eval --model "$model" run --label "taskwing" --timeout "$TIMEOUT" --judge="$JUDGE"
done

if command -v codex >/dev/null 2>&1; then
  for model in "${CODEX_MODELS[@]}"; do
    run_cmd "$TW_BIN" eval --model "$model" run --runner "$RUNNER_CODEX" --no-context --label "codex-native" --timeout "$TIMEOUT" --judge="$JUDGE"
    run_cmd "$TW_BIN" eval --model "$model" run --runner "$RUNNER_CODEX" --label "tw+codex" --timeout "$TIMEOUT" --judge="$JUDGE"
  done
else
  echo "Skipping Codex runs (codex CLI not found)."
fi

if command -v claude >/dev/null 2>&1; then
  for model in "${CLAUDE_MODELS[@]}"; do
    run_cmd "$TW_BIN" eval --model "$model" run --runner "$RUNNER_CLAUDE" --no-context --label "claude-native" --timeout "$TIMEOUT" --judge="$JUDGE"
    run_cmd "$TW_BIN" eval --model "$model" run --runner "$RUNNER_CLAUDE" --label "tw+claude" --timeout "$TIMEOUT" --judge="$JUDGE"
  done
else
  echo "Skipping Claude runs (claude CLI not found)."
fi

if command -v gemini >/dev/null 2>&1; then
  for model in "${GEMINI_MODELS[@]}"; do
    run_cmd "$TW_BIN" eval --model "$model" run --runner "$RUNNER_GEMINI" --no-context --label "gemini-native" --timeout "$TIMEOUT" --judge="$JUDGE"
    run_cmd "$TW_BIN" eval --model "$model" run --runner "$RUNNER_GEMINI" --label "tw+gemini" --timeout "$TIMEOUT" --judge="$JUDGE"
  done
else
  echo "Skipping Gemini runs (gemini CLI not found)."
fi

echo "All scheduled eval runs completed."
