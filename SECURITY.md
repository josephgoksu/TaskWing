# Security

TaskWing runs coding agents in sandboxes and pushes their work from your machine, so isolation bugs matter.

## Report a vulnerability

Please do not open a public issue. Use **Report a vulnerability** on the repository's [Security tab](https://github.com/josephgoksu/TaskWing/security/advisories/new).

Especially welcome:

- A way for a worker in a sandbox to reach a real secret, a GitHub login, or the hidden exam tests
- A way for worker output (commits, files, symlinks) to run code on the host
- A way for a pull request comment to steer an agent

## What is in place

| Protection | What breaks without it |
|---|---|
| No GitHub login in any sandbox; the host pushes | One compromised agent pushes to `main` |
| Host git handles agent commits with hooks, fsmonitor and the `ext` protocol off | A worker's repo runs code on your machine at join |
| The box secret scan flags GitHub tokens and SSH keys | A leaked credential goes unnoticed |
| The hidden tests stay on the host and in the exam box; `plan.json` is saved only after the exam is written, and a new plan deletes the old exam first | Agents, or the lead, teach to the tests |

`--provider local` removes GitHub and SSH tokens from the agents' environment, but it is not a sandbox: agents can read and change anything you can. A bug there that needs local mode is still welcome, but isolation is not promised.

## Supported versions

Only the latest release and `main` get fixes. v1 (up to `v1.23.2`) is no longer maintained.
