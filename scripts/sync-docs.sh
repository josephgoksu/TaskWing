#!/usr/bin/env bash
set -euo pipefail

MODE="write"
if [[ "${1:-}" == "--check" ]]; then
  MODE="check"
fi

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PARTIALS_DIR="$ROOT/docs/_partials"

TARGETS=(
  "$ROOT/README.md"
  "$ROOT/docs/TUTORIAL.md"
  "$ROOT/docs/PRODUCT_VISION.md"
  "$ROOT/CLAUDE.md"
  "$ROOT/GEMINI.md"
)

REQUIRED_MARKERS=(
  "TASKWING_PROVIDERS:$PARTIALS_DIR/providers.md"
  "TASKWING_TOOLS:$PARTIALS_DIR/tools.md"
  "TASKWING_COMMANDS:$PARTIALS_DIR/core_commands.md"
)

OPTIONAL_MARKERS=(
  "TASKWING_MCP_TOOLS:$PARTIALS_DIR/mcp_tools.md"
  "TASKWING_LEGAL:$PARTIALS_DIR/legal.md"
)

apply_marker() {
  local src="$1"
  local dst="$2"
  local marker="$3"
  local partial="$4"
  local required="$5"

  local start="<!-- ${marker}_START -->"
  local end="<!-- ${marker}_END -->"
  local start_count end_count

  start_count=$(grep -Fc "$start" "$src" || true)
  end_count=$(grep -Fc "$end" "$src" || true)

  if [[ "$required" == "true" ]]; then
    if [[ "$start_count" -ne 1 || "$end_count" -ne 1 ]]; then
      echo "sync-docs: $src must contain exactly one marker pair for $marker" >&2
      return 1
    fi
  else
    if [[ "$start_count" -eq 0 && "$end_count" -eq 0 ]]; then
      cp "$src" "$dst"
      return 0
    fi
    if [[ "$start_count" -ne 1 || "$end_count" -ne 1 ]]; then
      echo "sync-docs: $src has malformed optional marker pair for $marker" >&2
      return 1
    fi
  fi

  awk -v start="$start" -v end="$end" '
    BEGIN { in_block=0; start_seen=0; end_seen=0; block="" }
    FNR==NR {
      block = block $0 ORS
      next
    }
    {
      if ($0 == start) {
        print $0
        printf "%s", block
        in_block=1
        start_seen++
        next
      }
      if ($0 == end) {
        if (in_block != 1) {
          exit 42
        }
        in_block=0
        end_seen++
        print $0
        next
      }
      if (!in_block) {
        print $0
      }
    }
    END {
      if (in_block == 1) {
        exit 43
      }
      if (start_seen != 1 || end_seen != 1) {
        exit 44
      }
    }
  ' "$partial" "$src" > "$dst"
}

out_of_sync=0

for target in "${TARGETS[@]}"; do
  if [[ ! -f "$target" ]]; then
    echo "sync-docs: missing target file $target" >&2
    exit 1
  fi

  current="$target"
  tmp_files=()

  for entry in "${REQUIRED_MARKERS[@]}"; do
    marker="${entry%%:*}"
    partial="${entry#*:}"
    tmp_next="$(mktemp)"
    apply_marker "$current" "$tmp_next" "$marker" "$partial" "true"
    tmp_files+=("$tmp_next")
    current="$tmp_next"
  done

  for entry in "${OPTIONAL_MARKERS[@]}"; do
    marker="${entry%%:*}"
    partial="${entry#*:}"
    tmp_next="$(mktemp)"
    apply_marker "$current" "$tmp_next" "$marker" "$partial" "false"
    tmp_files+=("$tmp_next")
    current="$tmp_next"
  done

  if [[ "$MODE" == "check" ]]; then
    if ! cmp -s "$target" "$current"; then
      echo "sync-docs: out-of-sync file: $target" >&2
      diff -u "$target" "$current" || true
      out_of_sync=1
    fi
  else
    cp "$current" "$target"
  fi

  for tmp in "${tmp_files[@]}"; do
    rm -f "$tmp"
  done

done

if [[ "$MODE" == "check" && "$out_of_sync" -ne 0 ]]; then
  exit 1
fi

if [[ "$MODE" == "check" ]]; then
  echo "sync-docs: all managed markdown blocks are in sync"
else
  echo "sync-docs: updated managed markdown blocks"
fi
