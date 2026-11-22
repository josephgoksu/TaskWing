# TaskWing

Headless, vendor-agnostic orchestrator: one goal, many sandboxed agents, one verified PR. Go, AGPL-3.0.

The v1 knowledge layer was removed in November 2026. Nothing from it carries over.

## Read first

| File | Why |
|---|---|
| `PRINCIPLES.md` | The foundation. Every change must fit it. A change that breaks a principle updates the principle first, with its reason. |
| `SPEC.md` | What v2 is built from: stages, engine, CLI, first milestone. |
| `docs/ARCHITECTURE.md` | The source of truth: every concept, how they connect, who owns each. A change that adds, renames or removes a concept updates it in the same pull request. |
| `docs/FEATURES.md` | Every feature with its status. A change that builds a feature updates its row in the same pull request. |
| `context/` | Research and decisions behind the principles. |

## Decisions

| Decision | Choice | Why |
|---|---|---|
| Where agents run | Inside the box | Works with every agent CLI unchanged and scales to hundreds of boxes |
| How agents log in | Placeholder key + base URL; a proxy outside the box swaps in the real credential (`PRINCIPLES.md` #3). Subscription logins go through the sandbox provider (boat); TaskWing's proxy carries API keys only (decided 2026-11-14) | Agent CLIs offer many login methods (`context/agent-logins.md`). Holding a subscription login ourselves breaks the provider's terms; boat already handles it |
| Who plans | The user's own agent (the real Claude Code, started on their machine) leads with a TaskWing skill. The engine has no model code (decided 2026-11-14) | Keeps the flat subscription, works with any vendor's agent, and the engine only checks and runs the plan |
| Front end | The terminal only: one Go binary, the run drawn as agent squares, plus `taskwing engine --json` for scripts (decided 2026-11-22). The Tauri desktop app (2026-11-17) was removed and stays in git history | Developers install CLIs from Homebrew; one language, one release line, no app signing. A desktop shelf already full of agent apps |
| Phone pings | A push service called from the engine, not a phone app | Works on day one with no app to build |
| Sandboxes | boat first, local (Docker) later, never our own (decided 2026-11-14) | boat already runs agent CLIs, handles subscription logins and proxies keys. Local Docker gives a free backend for tests. Building a sandbox competes with a commodity |
| What the user approves | The exam (plain-sentence checks), before any agent starts. The engine checks the file split itself (`PRINCIPLES.md` #8, decided 2026-11-14) | The exam is what "done" means and can be judged in seconds; a file split can't be judged without reading code |
| Who writes the exam | A fresh agent session that gets only the task and the repo. The lead sees the sentences, never the tests (`PRINCIPLES.md` #12, decided 2026-11-14) | A lead that knows the tests can teach to them through its hints to workers |
| Pictures over text | The terminal board shows state as pictures: one square per agent, one short line each. Words only for the one question and the result (Joseph, 2026-11-14: "visual rather than text, simplicity over complexity") | Paragraphs per agent turn the board into a log nobody reads |
| Agent squares | One square per agent: grey done, blue working, outline waiting, amber needs you, red stuck (decided 2026-11-14). One proof line per agent, no file names | File names are code. Squares stay readable at 100 agents |
| Licence | AGPL-3.0 with a NOTICE file (decided 2026-11-15) | Open source, so open source events accept it. Copies must credit TaskWing and stay open, even when run as a hosted service |

## Checks

CI runs these on every pull request (`.github/workflows/ci.yml`). Run them before you push.

| Part | Command |
|---|---|
| Engine | `gofmt -l cmd internal`, `go vet ./...`, `go test -race ./...` |

## Releases

| Release | How |
|---|---|
| CLI | Push tag `vX.Y.Z` (or `vX.Y.Z-alpha.N`). `.github/workflows/release.yml` runs GoReleaser (`.goreleaser.yaml`): archives for macOS, Linux and Windows, `.deb`/`.rpm`/`.apk`, and the Homebrew formula in `josephgoksu/homebrew-tap` (needs the `HOMEBREW_TAP_TOKEN` secret; without it the release skips the formula). CI builds the same config as a snapshot on every pull request. The module path is `github.com/josephgoksu/TaskWing/v2`, so v2 tags work with `go install` |

## House style

- Every rule in a doc says what breaks without it.
- Tables and lists over prose.
