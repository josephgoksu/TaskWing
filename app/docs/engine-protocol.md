# Engine protocol

The engine is the `taskwing` binary. The desktop app starts `taskwing engine --json` and talks to it over stdin and stdout. Each message is one JSON object on one line, with a string `type`. The app ignores lines that aren't JSON objects with a `type`, and lines over 1 MiB.

When the app quits it closes the engine's stdin. The engine should then stop its agents and exit; after 2 seconds the app kills it.

The engine's first line should be `engine.hello`. An engine that exits before sending any event is treated as missing (an older `taskwing` has no `engine` command), and the app switches to its demo engine.

## Events (engine to app)

| type | fields | meaning |
|---|---|---|
| `engine.hello` | `version` | The engine is up and speaks this protocol. |
| `run.created` | `run: {id, repo, task, created_at}` | A run exists. The app opens it if the user just started it. |
| `plan.ready` | `run_id, cards[{id, title, owns[], proof:{kind}}], names[], checks[{id, text, added_by}]` | The lead's split and the exam sentences. `kind` is `page`, `request`, `test`, `file` or `timeline`. |
| `run.stage` | `run_id, stage, paused?` | `planning`, `approve`, `build`, `exam`, `pr` or `done`. |
| `card.update` | `run_id, card{id, state, detail?, proof?{kind, text, status?}}` | `state` is `waiting`, `working`, `passed`, `done`, `needs` or `stuck`. |
| `question` | `run_id, question{id, card_id, text, options[]}` | An agent needs a decision (up to 4 options). |
| `question.closed` | `run_id, question_id, choice` | The decision was taken. |
| `note` | `run_id, note{from, to, text, at}` | One agent told another something. |
| `exam.result` | `run_id, checks[{id, result}]` | `result` is `pass` or `fail`. |
| `pr.opened` | `run_id, number, url, draft` | The draft pull request. `url` must be `https://`. |
| `run.finished` | `run_id, ok, secrets_found, ended_at` | `ok: false` means the exam failed and nothing was pushed. |
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
