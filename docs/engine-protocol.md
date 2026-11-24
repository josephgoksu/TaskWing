# Engine protocol

The engine is the `taskwing` binary. `taskwing engine --json` runs tasks for a script, an editor or another front end, over stdin and stdout. Each message is one JSON object on one line, with a string `type`. A client should ignore lines that aren't JSON objects with a `type`, and lines over 1 MiB.

To stop, a client closes the engine's stdin. The engine then stops its agents, deletes their sandboxes (unless it was started with `--keep`) and exits. A run in progress ends with a `run.error` that says it was stopped.

The engine's first line is `engine.hello`. An engine that exits before sending any event is missing or too old (an older `taskwing` has no `engine` command).

## Events (engine to client)

| type | fields | meaning |
|---|---|---|
| `engine.hello` | `version, protocol, can[]` | The engine is up and speaks this protocol. `version` is always set (`dev` for a source build). `can` lists the optional commands it handles; a client should not send the rest. No `can` means all of them. |
| `run.created` | `run: {id, repo, task, created_at}` | A run exists. |
| `plan.ready` | `run_id, cards[{id, title, owns[], proof:{kind}}], names[], checks[{id, text, added_by}]` | The lead's split and the exam sentences. `kind` is `page`, `request`, `test`, `file` or `timeline`. |
| `run.stage` | `run_id, stage, paused?` | `planning`, `approve`, `build`, `exam`, `pr` or `done`. `paused` is reserved, not built. |
| `card.update` | `run_id, card{id, state, detail?, proof?{kind, text, status?}}` | `state` is `waiting`, `working`, `passed`, `done` or `stuck`; `needs` is reserved, not built. `proof` is left out when there is none, never `null`. |
| `question` | `run_id, question{id, card_id, text, options[]}` | An agent needs a decision (up to 4 options). Reserved, not built. |
| `question.closed` | `run_id, question_id, choice` | The decision was taken. Reserved, not built. |
| `note` | `run_id, note{from, to, text, at}` | One agent told another something. Reserved, not built. |
| `exam.result` | `run_id, checks[{id, result}]` | `result` is `pass` or `fail`. |
| `pr.opened` | `run_id, number, url, draft` | The draft pull request. Sent only with a real `https://` URL and number. |
| `run.error` | `run_id, message` | The run stopped on an error before the exam could decide it (no such repository, a dirty working tree, a sandbox that would not start). `run.finished` with `ok: false` follows. |
| `run.finished` | `run_id, ok, secrets_found, ended_at` | `ok: false` means the exam failed or the run stopped. Nothing was pushed, unless `run.error` says `gh pr create failed`: the branch is pushed first, so `taskwing/<run id>` is then on the remote. |

## Commands (client to engine)

| type | fields |
|---|---|
| `run.start` | `repo, task, sandbox, models, no_pr?` |
| `plan.approve` | `run_id, checks[{id, text, added_by}]` (the user may add checks; each needs its own `id`, not `k1`, `k2`... which are the exam's, and one without an `id` is ignored) |
| `plan.redo` | `run_id` |
| `answer` | `run_id, question_id, choice` (reserved, not built) |
| `exam.now` | `run_id` |
| `run.pause` / `run.resume` | `run_id` (reserved, not built) |

## The taskwing engine

`taskwing engine --json` (`internal/engine/stream.go`) runs the same plan, exam, build, join, exam and pull request steps as `taskwing run`. What it does with each field:

| Field | Meaning |
|---|---|
| `repo` | An absolute folder on this machine (or `~/...`), or a GitHub `owner/name`. A GitHub repository is cloned once with the GitHub CLI into `~/.taskwing/repos/owner/name` and reset to its default branch before each run, so only one run at a time may use it (a second gets `run.error`). A folder must be a git repository with no uncommitted changes. |
| `sandbox` | `boat` (the default) or `local`; the old label `Local folders` is still accepted. Local is for trying it: agents run on this machine with your own login and no isolation (GitHub and SSH tokens are removed from their environment), and a client should say so. Boat settings come from the same environment and `taskwing config` as `taskwing run`. |
| `models` | The coding agent in every box, by its boat name: `claude` (the default, or `TASKWING_AGENT` when set; any case, so the old `Claude` is accepted), `codex`, `opencode`, `pi`, `prime`, `kimi` or `mistral`. One agent for all boxes; `Mixed` and `Open models` stop the run with `run.error` until model routing is built. With `sandbox: local`, anything but `claude` needs `TASKWING_LOCAL_AGENT`. |
| `no_pr` | Optional. `true` stops after the exam: nothing is pushed and no pull request is opened; the joined work stays on the local branch. |
| `plan.approve` `checks` | Checks with `added_by: "user"` get their own exam session, which writes tests for them while the agents build. The builders never see those tests. A user check no test could be written for fails in the exam, and so does every one past the tenth. |
| `plan.redo` | The lead plans again and a new exam is written. A second `plan.ready` follows. |
| `exam.now` | Only during the build. Agents still working are stopped, whatever they wrote so far is kept (their card ends as `done`), and the exam runs on the joined work. |

Cards end as `passed` when the agent finished its piece, `done` when it was stopped for the exam, and `stuck` when it failed or its work conflicted with the others in the join and was left out. Every card's `proof.kind` is `test` today.

A failed check means no pull request: the joined work stays on the local branch `taskwing/<run id>`. Each run also keeps a log in `~/.taskwing/runs/<run id>/`.

Not built yet: `run.pause` and `run.resume` (not in `can`), `question` and `note` events. When stdin closes the engine stops every agent and deletes its sandboxes, which can take a while, so give it time before killing it; `taskwing doctor --clean` removes any box left behind.
