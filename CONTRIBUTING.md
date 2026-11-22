# Contributing to TaskWing

Thanks for helping. TaskWing is early, so issues, ideas and small pull requests all move it forward.

## Before you start

| Step | Why |
|---|---|
| Read [PRINCIPLES.md](PRINCIPLES.md) | Every change must fit it. A change that breaks a principle updates the principle first, with its reason |
| Check [docs/FEATURES.md](docs/FEATURES.md) | Shows what is built and what is planned. Pick a planned row or open an issue for a new one |
| For anything larger than a fix, open an issue first | Agreeing on the shape saves you a rewrite |

## Set up

Needs Go 1.24+, `bash` and `git`. A real run also needs `claude`, `gh` and the `boat` CLI ([docs/guide.md](docs/guide.md)).

```bash
git clone https://github.com/josephgoksu/TaskWing
cd TaskWing
go build ./cmd/taskwing          # the engine and CLI
```

## Checks

CI runs these on every pull request. Run them before you push.

| Part | Command |
|---|---|
| Engine | `gofmt -l cmd internal`, `go vet ./...`, `go test -race ./...` |
| CLI build | `go build ./cmd/taskwing` |
| Release config | `goreleaser release --snapshot --clean --skip=publish`; every `token:` in `.goreleaser.yaml` must be a bare `'{{ .Env.NAME }}'` |

Without the last two, a broken binary or release config shows up only when a tag is pushed.

## House rules

- One focused change per pull request, with a [conventional commit](https://www.conventionalcommits.org) title: `feat(engine): ...`, `fix(cli): ...`, `docs: ...`.
- A pull request that builds a feature, or adds or renames a concept, updates [docs/FEATURES.md](docs/FEATURES.md) and [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) in the same pull request.
- The engine never calls a model and never imports a vendor SDK. New sandboxes and agents go behind the Box and Agent adapters.
- No real secret ever goes into a sandbox, and no sandbox ever gets a GitHub login.
- Every rule in a doc says what breaks without it. Tables and lists over prose.

## Licence

TaskWing is AGPL-3.0. By sending a pull request you agree that your contribution is licensed under the same terms.
