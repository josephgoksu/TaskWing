# TaskWing

One task in, one verified pull request out. Your own coding agent plans the task and splits it into pieces, many agents build the pieces in cloud sandboxes, a hidden exam checks the joined work in a clean sandbox, and you merge one pull request.

```mermaid
flowchart LR
  T[Your task] --> L[Lead agent plans<br/>and splits it]
  L --> E{{You approve<br/>the exam}}
  E --> A1[Agent] & A2[Agent] & A3[Agent]
  A1 & A2 & A3 --> J[Work joined]
  J --> X[Hidden exam in<br/>a clean sandbox]
  X --> P[One pull request]
  P --> M((You merge))
```

![The cockpit](docs/cockpit.png)

Version 2 is being designed. A first proof of concept runs in the terminal: [docs/poc.md](docs/poc.md).

```bash
go install github.com/josephgoksu/TaskWing/cmd/taskwing@v2-poc
taskwing run "add an alert webhook with tests"
```

The v1 knowledge layer was removed; it stays available at tag `v1.23.2`.

| Read | Why |
|---|---|
| [PRINCIPLES.md](PRINCIPLES.md) | The rules every feature is built on |
| [SPEC.md](SPEC.md) | What v2 is made of and the first milestone |
| [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) | Every concept and how they connect |
| [docs/FEATURES.md](docs/FEATURES.md) | What is built and what is planned |
| [AGENTS.md](AGENTS.md) | Decisions so far |
| [prototype/flight-deck/index.html](prototype/flight-deck/index.html) | What the desktop cockpit looks like (open from disk) |

Licence: AGPL-3.0 (see [LICENSE](LICENSE) and [NOTICE](NOTICE)). Earlier copies (up to v1.23.2 and the first v2-poc builds) keep their old licence.
