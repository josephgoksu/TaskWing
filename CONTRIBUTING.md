# Contributing to TaskWing

Thanks for helping. TaskWing is early, so issues, ideas and small pull requests all move it forward.

## Before you start

| Step | Why |
|---|---|
| Read [PRINCIPLES.md](PRINCIPLES.md) | Every change must fit it. A change that breaks a principle updates the principle first, with its reason |
| Check [docs/FEATURES.md](docs/FEATURES.md) | Shows what is built and what is planned. Pick a planned row or open an issue for a new one |
| For anything larger than a fix, open an issue first | Agreeing on the shape saves you a rewrite |

## Set up

```bash
git clone https://github.com/josephgoksu/TaskWing
cd TaskWing
go build ./cmd/taskwing          # the engine and CLI (Go 1.24+)
cd app && npm install && npm run dev   # the desktop app (Rust and Node 20+, see app/README.md)
```

## Checks

CI runs these on every pull request. Run them before you push.

| Part | Command |
|---|---|
| Engine | `gofmt -l cmd internal`, `go vet ./...`, `go test -race ./...` |
| App | `cd app && npm ci && npm run check` |
| App shell | `cd app/src-tauri && cargo fmt --check && cargo clippy --all-targets -- -D warnings && cargo test` |

## House rules

- One focused change per pull request, with a [conventional commit](https://www.conventionalcommits.org) title: `feat(engine): ...`, `fix(app): ...`, `docs: ...`.
- A pull request that builds a feature, or adds or renames a concept, updates [docs/FEATURES.md](docs/FEATURES.md) and [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) in the same pull request.
- The engine never calls a model and never imports a vendor SDK. New sandboxes and agents go behind the Box and Agent adapters.
- No real secret ever goes into a sandbox, and no sandbox ever gets a GitHub login.
- Every rule in a doc says what breaks without it. Tables and lists over prose.

## Licence

TaskWing is AGPL-3.0. By sending a pull request you agree that your contribution is licensed under the same terms.
