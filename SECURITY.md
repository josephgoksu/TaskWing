# Security

TaskWing runs coding agents in sandboxes and pushes their work from your machine, so isolation bugs matter.

## Report a vulnerability

Please do not open a public issue. Use **Report a vulnerability** on the repository's [Security tab](https://github.com/josephgoksu/TaskWing/security/advisories/new).

Especially welcome:

- A way for a worker in a sandbox to reach a real secret, a GitHub login, or the hidden exam tests
- A way for worker output (commits, files, symlinks) to run code on the host
- A way for a pull request comment to steer an agent

## Supported versions

Only the latest release and `main` get fixes. v1 (up to `v1.23.2`) is no longer maintained.
