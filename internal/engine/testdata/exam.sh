#!/usr/bin/env bash
# Stand-in exam writer: adds one test file and prints the checks.
prompt=$(cat)
if grep -q "Checks the user added" <<<"$prompt"; then
  # Checks the user added with plan.approve: one per sentence, in order.
  mkdir -p taskwing_exam_user
  cat > taskwing_exam_user/check.sh <<'SH'
#!/usr/bin/env bash
[ -f "$1" ] && [ -z "$(tail -c1 "$1")" ]
SH
  echo '{"checks": [{"text": "The greeting ends with a newline", "command": "bash taskwing_exam_user/check.sh greet/hello.txt"}]}'
  exit 0
fi
mkdir -p taskwing_exam
cat > taskwing_exam/check.sh <<'SH'
#!/usr/bin/env bash
grep -qx "$2" "$1"
SH
cat <<'JSON'
{"checks": [
  {"text": "The greeting says hello", "command": "bash taskwing_exam/check.sh greet/hello.txt hello"},
  {"text": "The farewell says bye", "command": "bash taskwing_exam/check.sh farewell/bye.txt bye"},
  {"text": "The readme is still there", "command": "test -f README.md"}
]}
JSON
