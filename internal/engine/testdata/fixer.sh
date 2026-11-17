#!/usr/bin/env bash
# Stand-in fixer: fixes whichever bug its prompt names.
prompt=$(cat)
if grep -q "Fix this bug: add subtracts" <<<"$prompt"; then
  echo 'echo $(($1 + $2))' > calc/add.sh; f=calc/add.sh
else
  echo 'echo "$1" | tr a-z A-Z' > text/upper.sh; f=text/upper.sh
fi
grep -q "taskwing_exam" <<<"$prompt" && echo "LEAK: the fixer saw the hidden test" >&2
git add -A && git commit -q -m "fix $f"
echo "PROOF: fixed $f"
