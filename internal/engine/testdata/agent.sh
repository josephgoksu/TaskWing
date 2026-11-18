#!/usr/bin/env bash
# Stand-in coding agent: reads its prompt, writes its file, commits.
prompt=$(cat)
if grep -q "Your piece: Greeting" <<<"$prompt"; then f=greet/hello.txt; w=hello
else f=farewell/bye.txt; w=bye; fi
echo "writing $f"
if [ -n "${FAKE_AGENT_WRITE_FIRST:-}" ]; then mkdir -p "$(dirname "$f")" && echo "$w" > "$f"; fi
sleep "${FAKE_AGENT_SLEEP:-1}"
mkdir -p "$(dirname "$f")"
echo "$w" > "$f"
if [ -n "$FAKE_AGENT_STRAY" ] && [ "$w" = bye ]; then echo stray > stray.txt; fi
git add -A && git commit -q -m "add $f"
echo "PROOF: $f says $w"
