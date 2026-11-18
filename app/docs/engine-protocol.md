# Engine protocol

The engine is the `taskwing` binary. The desktop app starts `taskwing engine --json` and talks to it over stdin and stdout. Each message is one JSON object on one line, with a string `type`. The app ignores lines that aren't JSON objects with a `type`, and lines over 1 MiB.

When the app quits it closes the engine's stdin. The engine should then stop its agents and exit; after 2 seconds the app kills it.

The engine's first line should be `engine.hello`. An engine that exits before sending any event is treated as missing (an older `taskwing` has no `engine` command), and the app switches to its demo engine.

## Events (engine to app)

| type | fields | meaning |
|---|---|---|
| `engine.hello` | `version, protocol, can[]` | The engine is up and speaks this protocol. `can` lists the optional commands it handles; the app hides buttons for the rest. No `can` means all of them. |
| `run.created` | `run: {id, repo, task, created_at}` | A run exists. The app opens it if the user just started it. |
| `plan.ready` | `run_id, cards[{id, title, owns[], proof:{kind}}], names[], checks[{id, text, added_by}]` | The lead's split and the exam sentences. `kind` is `page`, `request`, `test`, `file` or `timeline`. |
| `run.stage` | `run_id, stage, paused?` | `planning`, `approve`, `build`, `exam`, `pr` or `done`. |
| `card.update` | `run_id, card{id, state, detail?, proof?{kind, text, status?}}` | `state` is `waiting`, `working`, `passed`, `done`, `needs` or `stuck`. |
| `question` | `run_id, question{id, card_id, text, options[]}` | An agent needs a decision (up to 4 options). |
| `question.closed` | `run_id, question_id, choice` | The decision was taken. |
| `note` | `run_id, note{from, to, text, at}` | One agent told another something. |
| `exam.result` | `run_id, checks[{id, result}]` | `result` is `pass` or `fail`. |
| `pr.opened` | `run_id, number, url, draft` | The draft pull request. `url` must be `https://`. |
| `run.error` | `run_id, message` | The run stopped on an error before the exam could decide it (no such repository, a dirty working tree, a sandbox that would not start). `run.finished` with `ok: false` follows. |
| `run.finished` | `run_id, ok, secrets_found, ended_at` | `ok: false` means the exam failed or the run stopped, and nothing was pushed. |
| `engine.exit` | `spoke` | Sent by the app's shell when the engine stops. `spoke` is false if the engine never sent an event. |

## Commands (app to engine)

| type | fields |
|---|---|
| `run.start` | `repo, task, sandbox, models` |
| `plan.approve` | `run_id, checks[{id, text, added_by}]` (the user may add checks) |
| `plan.redo` | `run_id` |
| `answer` | `run_id, question_id, choice` |
| `exam.now` | `run_id` |
| `run.pause` / `run.resume` | `run_id` |

## The taskwing engine

`taskwing engine --json` (`internal/engine/stream.go`) runs the same plan, exam, build, join, exam and pull request steps as `taskwing run`. What it does with each field:

| Field | Meaning |
|---|---|
| `repo` | An absolute folder on this machine (or `~/...`), or a GitHub `owner/name`. A GitHub repository is cloned once with the GitHub CLI into `~/.taskwing/repos/owner/name` and reset to its default branch before each run, so only one run at a time may use it (a second gets `run.error`). A folder must be a git repository with no uncommitted changes. |
| `sandbox` | `boat` (the default) or `Local folders`. Local folders are for testing: agents run on this machine with your own login and no isolation, and the app labels them that way. Boat settings come from the same environment and `taskwing config` as `taskwing run`. |
| `models` | Only `Claude` works today. `Mixed` and `Open models` stop the run with `run.error` until model routing is built. |
| `plan.approve` `checks` | Checks with `added_by: "user"` get their own exam session, which writes tests for them while the agents build. The builders never see those tests. A user check no test could be written for fails in the exam, and so does every one past the tenth. |
| `plan.redo` | The lead plans again and a new exam is written. A second `plan.ready` follows. |
| `exam.now` | Only during the build. Agents still working are stopped, whatever they wrote so far is kept (their card ends as `done`), and the exam runs on the joined work. |

Cards end as `passed` when the agent finished its piece, `done` when it was stopped for the exam, and `stuck` when it failed or its work conflicted with the others in the join and was left out. Every card's `proof.kind` is `test` today.

A failed check means no pull request: the joined work stays on the local branch `taskwing/<run id>`. Each run also keeps a log in `~/.taskwing/runs/<run id>/`.

Not built yet: `run.pause` and `run.resume` (not in `can`), `question` and `note` events. When stdin closes the engine stops every agent and deletes its sandboxes, which can take longer than the app's 2 seconds; `taskwing doctor --clean` removes any box left behind.
