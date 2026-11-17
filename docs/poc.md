# Try the PoC

```
taskwing run "<task>"
```

```
1/4 Plan    lead splits the task  ·  a separate session writes the hidden exam
            you read both, press y
2/4 Build   one sandbox and one agent per piece, live squares
3/4 Exam    pieces joined on one branch, exam runs in a clean sandbox
4/4 PR      pushed from your Mac, draft pull request, sandboxes deleted
```

## 1. Install

```bash
go install github.com/josephgoksu/TaskWing/cmd/taskwing@main
~/go/bin/taskwing version     # taskwing v2-poc (an old v1 from Homebrew may come first on PATH)
```

Needs on your Mac: `git`, `claude` (logged in), `gh` (logged in), and the `boat` CLI (logged in).

## 2. Try it with no sandboxes first (2 minutes)

`--provider local` runs the agents in folders on your Mac with your own Claude login. No isolation; it only shows the flow.

```bash
cd ~/some/repo                       # clean working tree
taskwing run "add a /health endpoint with a test" --provider local --no-pr
```

## 3. Prepare a boat base box (once per repository)

Every agent box is a fork of this one, so it needs what the repository needs to build and test.

Agents need your Claude login and nothing else. boat's default environment also passes your GitHub token, gh login and secret files into every box, and taskwing refuses to start agents in a box that holds GitHub credentials. So make a separate environment first:

1. In boat, create an environment (for example `tw-agents`) that passes only the agent login: GitHub, secret files, boat CLI credentials and repositories all off. Keep `base` as it is.
2. Make the base box from it, install the toolchain, stop it:

```bash
boat new --environment tw-agents     # note the box id, e.g. bx_abc123 (check boat new --help)
boat ssh bx_abc123
#   install the repository's toolchain (Go, Node, Docker services...)
#   do NOT log in to gh: boxes never get GitHub credentials
#   exit
boat stop bx_abc123                  # stopping takes the snapshot that forks start from
```

Never give the base box a name that starts with "taskwing". A `--no-env` box has no secrets, but also no Claude login, so `boat prompt` fails in it with "Claude login required". The end-of-run secret scan lists any leak; the agents' own Claude login is expected and named as such.

A stopped box keeps its data for 14 days. After that, `boat fork` fails; save it with `boat snapshot <id> <name>` or make a new base.

## 4. Run it

```bash
taskwing config boat-base bx_abc123   # once; or export TASKWING_BOAT_BASE=bx_abc123
cd path/to/osprey                    # clean working tree, on the branch the PR should target
taskwing run "add an alert webhook, a list-evaluations endpoint and a disposition API"
```

| Flag | Default | What it does |
|---|---|---|
| `--provider` | `boat` | `local` runs on your Mac (testing only) |
| `--boat-base` | `$TASKWING_BOAT_BASE` | box to fork |
| `--boat-env` | none | boat environment to pin forks to |
| `--model` / `--effort` | `opus-5-5` / `medium` | agent model in boat |
| `--max-cards` | 4 | most pieces the lead may make |
| `--no-pr` | off | stop after the exam, keep the branch locally |
| `--yes` | off | do not ask before starting or before a PR with failed checks |
| `--keep` | off | keep the sandboxes to look inside |

The lead and the exam writer run on your Mac through `claude -p`. Change them with `TASKWING_LEAD_CMD` and `TASKWING_EXAM_CMD` (the prompt arrives on stdin).

## Bug hunt

```bash
taskwing hunt "the payment scoring code" --no-pr     # or no area: the whole repository
```

```
1 Hunt   a session writes a failing test for each bug it suspects; a test that passes today = dropped
2 Pick   you see the proven bugs, press y
3 Fix    one agent per bug, it gets the bug but not the test
4 Exam   every bug's test passes + all existing tests pass
5 PR     one draft pull request with a Bugs table
```

`--max-cards` caps how many bugs it keeps (default 4). Two bugs that need the same files: the second is left for the next hunt.

## 5. After a run

| Where | What |
|---|---|
| `~/.taskwing/runs/<id>/` | `plan.json`, `exam/` (the hidden tests), `exam-results.json`, `pr-body.md`, `events.jsonl` |
| `~/.taskwing/runs/<id>/logs/` | every agent's full log |
| branch `taskwing/<id>` | the joined work |
| `taskwing doctor` | lists leftover sandboxes and refs; `--clean` deletes them |

Exit code: 0 all checks passed, 3 some checks failed, 1 error.

## Not in the PoC

The cockpit, the price tag before you say yes, starting from a Claude Code skill, live proof previews, run replays, NATS, other vendors, resuming after a crash.
