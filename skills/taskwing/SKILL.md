---
name: taskwing
description: Run one coding task as a team of coding agents with TaskWing, each agent in its own sandbox, ending in one draft pull request checked by a hidden exam. Use when the user mentions TaskWing, wants a feature built by several agents in parallel, wants a swarm or team of agents on one task, or wants bugs found and each proven with a failing test. Also use to install or set up TaskWing in a repository.
---

# TaskWing

TaskWing is an open source harness for coding agents. One task goes in. A lead agent splits it into pieces that share no files, a separate session writes a hidden exam, one agent builds each piece in its own sandbox, and one draft pull request comes out with the exam results.

You, the agent reading this, drive TaskWing from the user's chat. You run the CLI, show the user the plan and the exam, start the run when they say yes, and report the pull request. TaskWing does the rest.

## What works today

| Part | Today | Not yet |
|---|---|---|
| Driving TaskWing (this skill) | Any agent that can run shell commands: Claude Code, Codex, Cursor, Gemini CLI, Antigravity, Copilot and others | |
| Lead and exam writer | Claude Code (`claude` CLI, logged in, on this machine) | Other agents as lead (possible today with `TASKWING_LEAD_CMD`, untested) |
| Workers (one per piece) | Claude Code by default. In boat, `--agent codex` (or `opencode`, `pi`, `prime`, `kimi`, `mistral`) runs that agent in every box, not yet proven in a live run | Gemini CLI; a different agent per piece |
| Sandboxes | boat, or `--provider local` (folders on this machine, no isolation, for trying it) | E2B, Docker, AWS |
| Systems | macOS, Linux, Windows with Git Bash or WSL (every run goes through `bash`) | |

Say this plainly if the user asks: whatever agent they chat with, the lead and exam writer are Claude Code for now, and the builders are Claude Code unless the run uses `--agent` in boat.

## Set up (once per machine and repository)

Do these in order and stop at the first step that needs the user. Tell them what you found in one short list at the end.

**1. The CLI.** Run `taskwing version` and `taskwing help`.

- Missing, or `taskwing help` has no `approve` command: install or upgrade it.

| System | Command |
|---|---|
| macOS or Linux with Homebrew | `brew install josephgoksu/tap/taskwing` (or `brew upgrade taskwing`) |
| macOS or Linux, no Homebrew | `curl -fsSL https://taskwing.app/install.sh \| sh` |
| Windows, or anywhere with Go 1.24+ | `go install github.com/josephgoksu/TaskWing/v2/cmd/taskwing@latest` |

- Still no `approve` after that: the newest release predates it. Build from main: `go install github.com/josephgoksu/TaskWing/v2/cmd/taskwing@main`, then call `$(go env GOBIN)/taskwing` (or `$(go env GOPATH)/bin/taskwing` when `go env GOBIN` is empty) if an older one comes first on PATH.
- A `taskwing` that prints v1 commands is the old v1 tool: `brew upgrade taskwing` replaces it.

**2. What a run needs.** Check each and give the user the exact fix for any that fails:

| Check | Command | Fix |
|---|---|---|
| git with a name and email | `git var GIT_COMMITTER_IDENT` | `git config --global user.name "..."`, `user.email "..."` |
| GitHub CLI logged in | `gh auth status` | the user runs `gh auth login` |
| Claude Code logged in | `claude auth status` | install Claude Code, then the user runs `claude` and types `/login` |
| boat (sandboxes) | `boat --version`, `taskwing config` shows a base box | optional: without it use `--provider local`. Setup: https://github.com/josephgoksu/TaskWing/blob/main/docs/guide.md |

Logins are the user's to do. Never ask for, read or store their tokens.

**3. This skill, for every agent the repository uses.** Two folders cover the common agents:

```bash
url=https://raw.githubusercontent.com/josephgoksu/TaskWing/main/skills/taskwing/SKILL.md
for dir in .agents/skills/taskwing .claude/skills/taskwing; do
  mkdir -p "$dir" && curl -fsSL "$url" -o "$dir/SKILL.md"
done
```

| Folder | Read by |
|---|---|
| `.agents/skills/` | Codex, Cursor, Gemini CLI, Antigravity, GitHub Copilot, OpenCode and most others |
| `.claude/skills/` | Claude Code (it does not read `.agents/skills/`) |

For every repository instead of this one, use the same files under `~/.agents/skills/taskwing/` and `~/.claude/skills/taskwing/` (Antigravity's global folder is `~/.gemini/config/skills/taskwing/`). `gh skill install josephgoksu/TaskWing taskwing --agent <agent>` works too, with a recent GitHub CLI.

**4. The repository's agent docs.** Add this block to each of `AGENTS.md`, `CLAUDE.md` and `GEMINI.md` that exists at the repository root. If none exists, create `AGENTS.md`. If the block is already there, replace it, never add a second one.

```markdown
<!-- taskwing:start -->
## TaskWing

Large tasks that split into independent pieces can run as a team of agents with TaskWing (skill: `taskwing`): `taskwing run "<task>" --plan-only`, show the user the plan and the exam, then `taskwing approve <run id>` after their yes. Never approve a plan for the user. Runs end as one draft pull request that the user reviews and merges.
<!-- taskwing:end -->
```

**5. Commit.** Show the user what changed (`git status`) and ask before you commit. A run needs a clean working tree, so these files must be committed or stashed before the first run.

## Run a task

**1. Get ready.**
- The working tree must be clean (`git status --porcelain` prints nothing) and on the branch the pull request should target.
- Write the task as one or two sentences: what to build, what "done" means, "with tests". The lead reads the code itself; do not paste code into the task.
- Pick where agents run. boat when it is set up. Otherwise `--provider local --no-pr`, and tell the user first: local agents run on this machine with the user's own login, with no isolation.

**2. Plan.** This takes a few minutes, sometimes more than 10. Run it directly only if your tool waits at least 15 minutes; a tool timeout stops the plan.

```bash
taskwing run "<task>" --plan-only            # add --provider local --no-pr when there is no boat
```

Otherwise (Claude Code's Bash, for example, stops a command after 10 minutes) start it in the background like step 4 and check the log until it has an `Approve it:` line, or `taskwing:` and an error:

```bash
plan_log=$(mktemp)
nohup taskwing run "<task>" --plan-only > "$plan_log" 2>&1 &
```

It prints the plan (one line per piece), the exam (plain-sentence checks), a run id such as `tw-20261124-143005-a1b2` and the `taskwing approve` command to run, with the flags of this plan. Nothing is built yet.

**3. Ask the user.** Show them the plan and every exam check, word for word. Then ask one question with four answers:

| The user says | You do |
|---|---|
| Yes | step 4 |
| Add a check ("it must also ...") | step 4 with `--check "<their sentence>"`, once per check |
| Plan again | delete this run (`rm -rf "${TASKWING_HOME:-$HOME/.taskwing}/runs/<run id>"`), then run step 2 again (a new run id) |
| Stop | delete this run the same way: its exam must not stay on disk where the next run's lead can read it |

Never answer this yourself. The exam is what "done" means, and only the user approves it.

**4. Build.** A build takes 5 to 60 minutes, longer than most tool timeouts, so start it in the background with a log. Use the `taskwing approve` command step 2 printed: it carries the same `--provider`, `--boat-base` and `--no-pr` flags as the plan.

```bash
run=tw-20261124-143005-a1b2                   # the run id from step 2
log="${TASKWING_HOME:-$HOME/.taskwing}/runs/$run/approve.log"
nohup taskwing approve "$run" > "$log" 2>&1 &   # plus the flags step 2 printed after the run id
```

If your harness has its own way to run a command in the background (Claude Code's background Bash, for example), use that instead of `nohup`. Then check the log every minute or two with `tail -n 30 "$log"`. Each agent prints a line when its state changes: `working`, `done` or `stuck`. Tell the user only when something changes: an agent stuck, the exam result, the end.

**5. Report.** The run is over when the log has a `Done` line, or `taskwing:` and an error. Report:
- the score (`3 of 3 checks passed`) and the pull request link, or the branch `taskwing/<run id>` when there is no pull request;
- any stuck agent, and the log of that agent in `~/.taskwing/runs/<run id>/logs/`.

The pull request is a draft. The user reviews and merges it. Never merge it, never push to `taskwing/` branches, never mark it ready.

### Bug hunt

`taskwing hunt "<area>" --plan-only` instead of `taskwing run`. It shows proven bugs (each with a failing test) instead of a plan. Same question, same `taskwing approve`.

## Rules

| Rule | What breaks without it |
|---|---|
| Never run `taskwing approve` or `--yes` without the user's yes in this chat | Agents build a plan nobody agreed to, on the user's money |
| Never open `~/.taskwing/runs/<id>/exam/`, and never write or change tests named `taskwing_exam` | The exam is hidden so nobody can teach to it; a peek makes "passed" mean nothing |
| One run per repository at a time | Two runs fight over the same branch and clone |
| Clean working tree before every run | Agents start from the last commit, so uncommitted work is left out and the run refuses to start |
| Leave the sandboxes to TaskWing; clean up with `taskwing doctor --clean` | A base box deleted by hand breaks every later run |

## When something fails

| Message | Fix |
|---|---|
| `you have uncommitted changes` | commit or stash, then run again |
| `the claude CLI on this machine is not logged in` | the user runs `claude`, types `/login` |
| `the GitHub CLI is not logged in` | the user runs `gh auth login`, or add `--no-pr` |
| `no origin remote to push to` | add `--no-pr`, or the user adds a remote |
| `no boat base box set` or `the boat CLI is not installed` | `--provider local` to try it, or boat setup in docs/guide.md |
| `the repository moved since the plan` | a commit landed after step 2: plan again |
| `was already approved` | each plan builds once: plan again |
| `Done` with fewer checks passed than the total (exit code 3 in the foreground) | the run finished and some exam checks failed; report which, from the log |
| leftover sandboxes | `taskwing doctor`, then `taskwing doctor --clean` |

Everything else (flags, environment variables, what a run leaves behind): https://github.com/josephgoksu/TaskWing/blob/main/docs/guide.md
