# Using TaskWing

```
taskwing run "<task>"
```

```
1/4 Plan          lead splits the task  ·  a separate session writes the hidden exam
                  you read both, then answer
2/4 Build         one sandbox and one agent per piece, one line each
3/4 Exam          pieces joined on one branch, exam runs in a clean sandbox
4/4 Pull request  pushed from your machine, draft pull request, sandboxes deleted
```

## 1. Install

```bash
go install github.com/josephgoksu/TaskWing/v2/cmd/taskwing@latest    # needs Go 1.24+
~/go/bin/taskwing version     # an old v1 from Homebrew may come first on PATH
```

`brew install josephgoksu/tap/taskwing` installs the latest v2 release (an alpha for now). On a machine with v1 from Homebrew, `brew upgrade` replaces it with v2, which has none of the v1 commands; v1 stays at tag `v1.23.2`.

`taskwing version` prints:

| Build | Prints |
|---|---|
| A release (Homebrew, packages, archives) | the tag, for example `taskwing v2.0.0` |
| `go install ...@vX.Y.Z` or `@latest` | the module version |
| Built from source | `taskwing dev` |

Needs on your machine:

| Tool | Why | What breaks without it |
|---|---|---|
| `bash` | every agent and check command runs through it, Windows included | nothing starts |
| `git` | the host pulls, joins and pushes the agents' commits | no join, no branch |
| `claude` (logged in) | the lead and the exam writer run on your machine | no plan, no exam |
| `gh` (logged in) | the host opens the pull request; boxes never get it | no pull request |
| `boat` CLI (logged in) | makes the sandboxes (not needed with `--provider local`) | the run stops before planning |

## 2. Try it with no sandboxes first (2 minutes)

`--provider local` runs the agents in folders on your machine with your own Claude login. It removes GitHub and SSH tokens from the agents' environment, but it is not a sandbox: an agent can read and change anything you can. It only shows the flow.

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

| Rule | What breaks without it |
|---|---|
| Never give the base box a name that starts with "taskwing" | `taskwing doctor --clean` treats it as a leftover |
| Don't use a `--no-env` box | It has no secrets, but also no Claude login, so `boat prompt` fails with "Claude login required" |
| Save the base within 14 days of stopping it: `boat snapshot <id> <name>`, or make a new one | A stopped box keeps its data for 14 days; after that `boat fork` fails |

The end-of-run secret scan lists any leak, including SSH keys. The agents' own Claude login is expected and named as such.

## 4. Run it

```bash
taskwing config boat-base bx_abc123   # once; or export TASKWING_BOAT_BASE=bx_abc123
cd path/to/repo                       # clean working tree, on the branch the PR should target
taskwing run "add an alert webhook, a list-evaluations endpoint and a disposition API"
```

Before it plans, taskwing checks that the boat CLI is on your PATH and a base box is set, so a first run doesn't spend tokens on a plan and then fail.

### Answer the plan

After step 1 you see the plan and the exam. Type one of:

| You type | What happens |
|---|---|
| `y` | the agents start |
| `n` | the run stops; the plan and exam stay in the run folder |
| `r` | the lead plans again and a new exam is written |
| any sentence | it becomes your own check; it gets its own hidden tests, written while the agents build, and the question comes back |

During step 2, type `e` then Enter to run the exam now with what the agents have so far. `--yes` skips the question.

### Flags

| Flag | Default | What it does |
|---|---|---|
| `--provider` | `boat` (`$TASKWING_PROVIDER`) | `local` runs on your machine (trying it only) |
| `--boat-base` | `$TASKWING_BOAT_BASE`, else `taskwing config` | box to fork |
| `--boat-env` | `$TASKWING_BOAT_ENV` | boat environment to pin forks to |
| `--agent` | `claude` (`$TASKWING_AGENT`) | the coding agent in each boat box: `claude`, `codex`, `opencode`, `pi`, `prime`, `kimi` or `mistral` ([Other agents](#other-agents)) |
| `--model` | `opus-5-5` for claude, else the boat dashboard's (`$TASKWING_MODEL`) | agent model in boat |
| `--effort` | `medium` for claude, else the boat dashboard's (`$TASKWING_EFFORT`) | agent effort in boat |
| `--max-cards` | 4 | most pieces the lead may make, or bugs a hunt keeps |
| `--no-pr` | off | stop after the exam, keep the branch locally |
| `--yes` | off | do not ask before starting or before a PR with failed checks |
| `--keep` | off | keep the sandboxes to look inside |

### Environment

| Variable | Default | What it does |
|---|---|---|
| `TASKWING_PROVIDER` | `boat` | same as `--provider` |
| `TASKWING_BOAT_BASE` | `taskwing config` value | same as `--boat-base` |
| `TASKWING_BOAT_ENV` | none | same as `--boat-env` |
| `TASKWING_AGENT` | `claude` | same as `--agent` |
| `TASKWING_MODEL` | `opus-5-5` for claude | same as `--model` |
| `TASKWING_EFFORT` | `medium` for claude | same as `--effort` |
| `TASKWING_LEAD_CMD` | `claude -p` with read-only tools | the lead; the prompt arrives on stdin |
| `TASKWING_EXAM_CMD` | `claude -p` with write tools | the exam writer; the prompt arrives on stdin |
| `TASKWING_HUNT_CMD` | the exam writer's command | the bug hunter |
| `TASKWING_LOCAL_AGENT` | `claude -p --dangerously-skip-permissions` | the agents in `--provider local` |
| `TASKWING_BOAT_BIN` | `boat` | path to the boat CLI |
| `TASKWING_HOME` | `~/.taskwing` | where config, runs, repo cache and local boxes live |

## Other agents

boat runs seven coding agents in every box, and `--agent` picks the one that builds each piece:

```bash
taskwing run "add discount codes at checkout, with tests" --agent codex
taskwing run "..." --agent codex --model gpt-6.1-sol --effort high     # a model of your own
```

| Rule | What breaks without it |
|---|---|
| Connect the agent's login in boat (Agents & Proxying) and let the base box's environment pass it | The first prompt fails with `provider_not_configured` and every piece ends stuck |
| Without `--model` and `--effort`, an agent other than claude uses the model boat's dashboard picked for it | Each agent has its own model names and effort levels; Claude's defaults would be refused |
| The secret scan expects each agent's own login (`CLAUDE_CODE_OAUTH_TOKEN` for claude, `OPENAI_API_KEY` for codex: `box.AgentLogins`) and reports any other credential | An agent cannot run without its login; anything else in a box is a leak |
| One agent for every box in a run | Routing pieces to different agents is planned (FEATURES A5) |
| The lead and the exam writer still run on this machine with `TASKWING_LEAD_CMD` and `TASKWING_EXAM_CMD` (Claude Code by default) | `--agent` changes the builders only; a Codex lead needs its own command |

With `--provider local`, the agents are a command on this machine: set `TASKWING_LOCAL_AGENT` (the prompt arrives on stdin, the working directory is the repository).

## Bug hunt

```bash
taskwing hunt "the payment scoring code" --no-pr     # or no area: the whole repository
```

```
1/4 Hunt          a session writes a failing test for each bug it suspects; a test that passes today is dropped
                  you see the proven bugs, then answer
2/4 Build         one agent per bug; it gets the bug but not the test
3/4 Exam          every bug's test passes, and all existing tests still pass
4/4 Pull request  one draft pull request with a Bugs table
```

`--max-cards` caps how many bugs it keeps (default 4). Two bugs that need the same files: the second is left for the next hunt.

## 5. After a run

| Where | What |
|---|---|
| `~/.taskwing/runs/<id>/` | `plan.json`, `exam/` (`checks.json` and `exam.patch`, the hidden tests), `exam-results.json`, `pr-body.md`, `events.jsonl` |
| `~/.taskwing/runs/<id>/logs/` | every agent's full log |
| branch `taskwing/<id>` | the joined work |
| `taskwing doctor` | lists leftover sandboxes and agent refs; `--clean` deletes them, never the base box |

A run id looks like `tw-20261122-143005-a1b2` (date, time, 4 hex).

| Exit code | Meaning |
|---|---|
| 0 | done |
| 1 | error, or stopped with Ctrl-C |
| 2 | usage error (unknown command or flag) |
| 3 | the exam ran and some checks failed |

## What's next

Everything planned, with its milestone: [FEATURES.md](FEATURES.md).
