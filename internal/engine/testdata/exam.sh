#!/usr/bin/env bash
# Stand-in exam writer: adds one test file and prints the checks.
cat >/dev/null
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
