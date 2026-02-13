#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

"$ROOT/scripts/sync-docs.sh" --check

stale_patterns=(
  '\\btw\\s+(bootstrap|context|add|plan new|task list|hook)\\b'
  'taskwing context'
  'taskwing add'
  'OpenAI, Ollama support'
)

found=0
for pattern in "${stale_patterns[@]}"; do
  if rg -n --glob '*.md' -e "$pattern" "$ROOT"; then
    echo "check-doc-consistency: stale pattern detected -> $pattern" >&2
    found=1
  fi
done

if [[ "$found" -ne 0 ]]; then
  exit 1
fi

echo "check-doc-consistency: markdown consistency checks passed"
