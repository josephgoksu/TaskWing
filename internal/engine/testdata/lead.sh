#!/usr/bin/env bash
# Stand-in lead: the first plan overlaps on purpose, the second is fine.
prompt=$(cat)
if ! grep -q "previous plan was refused" <<<"$prompt"; then
  echo '{"task":"greet and farewell","cards":[{"id":"c1","title":"Greeting","owns":["greet/**"],"prompt":"make greet"},{"id":"c2","title":"Farewell","owns":["greet/bye.txt"],"prompt":"make farewell"}]}'
  exit 0
fi
cat <<'JSON'
Here is the plan:
```json
{"task": "Add a greeting and a farewell",
 "cards": [
  {"id": "c1", "title": "Greeting", "owns": ["greet/**"], "prompt": "Write greet/hello.txt containing hello."},
  {"id": "c2", "title": "Farewell", "owns": ["farewell/"], "prompt": "Write farewell/bye.txt containing bye."}
 ],
 "seams": [{"between": ["c1", "c2"], "owner": "c1", "contract": "both files end with a newline"}]}
```
JSON
