#!/usr/bin/env bash
# Stand-in bug hunter: two real bugs, one false alarm, one overlap.
cat >/dev/null
echo '[ "$(bash calc/add.sh 2 3)" = 5 ]' > taskwing_exam_b1.sh
echo '[ "$(bash text/upper.sh hi)" = HI ]' > taskwing_exam_b2.sh
echo 'true' > taskwing_exam_b3.sh
echo 'false' > taskwing_exam_b4.sh
cat <<'JSON'
{"bugs": [
  {"id": "b1", "title": "add subtracts instead of adding", "where": "calc/add.sh:1", "why": "add 2 3 prints -1, not 5", "owns": ["calc/**"], "test": "taskwing_exam_b1.sh", "command": "bash taskwing_exam_b1.sh"},
  {"id": "b2", "title": "upper does not uppercase", "where": "text/upper.sh:1", "why": "upper hi prints hi", "owns": ["text/"], "test": "taskwing_exam_b2.sh", "command": "bash taskwing_exam_b2.sh"},
  {"id": "b3", "title": "a false alarm", "where": "README.md", "why": "nothing", "owns": ["docs/**"], "test": "taskwing_exam_b3.sh", "command": "bash taskwing_exam_b3.sh"},
  {"id": "b4", "title": "add also bad", "where": "calc/add.sh", "why": "same file", "owns": ["calc/add.sh"], "test": "taskwing_exam_b4.sh", "command": "bash taskwing_exam_b4.sh"}
 ],
 "regression": "test -f README.md",
 "dropped": [{"title": "readme typo", "why": "not a bug"}]}
JSON
