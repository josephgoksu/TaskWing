# TaskWing

Headless, vendor-agnostic orchestrator: one goal, many sandboxed agents, one verified PR. Go, AGPL-3.0.

The v1 knowledge layer was removed in November 2026. Nothing from it carries over.

## Read first

| File | Why |
|---|---|
| `PRINCIPLES.md` | The foundation. Every change must fit it. A change that breaks a principle updates the principle first, with its reason. |
| `SPEC.md` | What v2 is built from: stages, engine, cockpit, CLI, first milestone. |
| `docs/ARCHITECTURE.md` | The source of truth: every concept, how they connect, who owns each. A change that adds, renames or removes a concept updates it in the same pull request. |
| `docs/FEATURES.md` | Every feature with its status. A change that builds a feature updates its row in the same pull request. |
| `context/` | Research and decisions behind the principles. |

## Decisions

| Decision | Choice | Why |
|---|---|---|
| Where agents run | Inside the box | Works with every agent CLI unchanged and scales to hundreds of boxes |
| How agents log in | Placeholder key + base URL; a proxy outside the box swaps in the real credential (`PRINCIPLES.md` #3). Subscription logins go through the sandbox provider (boat); TaskWing's proxy carries API keys only (decided 2026-11-14) | Agent CLIs offer many login methods (`context/agent-logins.md`). Holding a subscription login ourselves breaks the provider's terms; boat already handles it |
| Who plans | The user's own agent (the real Claude Code, started on their machine) leads with a TaskWing skill. The engine has no model code (decided 2026-11-14) | Keeps the flat subscription, works with any vendor's agent, and the engine only checks and runs the plan |
| Cockpit app | Tauri, with the Go engine bundled as a sidecar (decided 2026-11-13) | Reuses the web UI from the prototype, runs on Mac, Linux and Windows. SwiftUI was rejected: Mac only, full rewrite, head-on with Sanductor |
| Phone pings | A push service called from the engine, not a phone app | Works on day one with no app to build |
| Sandboxes | boat first, local (Docker) later, never our own (decided 2026-11-14) | boat already runs agent CLIs, handles subscription logins and proxies keys. Local Docker gives a free backend for tests. Building a sandbox competes with a commodity |
| What the user approves | The exam (plain-sentence checks), before any agent starts. The engine checks the file split itself (`PRINCIPLES.md` #8, decided 2026-11-14) | The exam is what "done" means and can be judged in seconds; a file split can't be judged without reading code |
| Who writes the exam | A fresh agent session that gets only the task and the repo. The lead sees the sentences, never the tests (`PRINCIPLES.md` #12, decided 2026-11-14) | A lead that knows the tests can teach to them through its hints to workers |
| Pictures over text | The cockpit shows state as pictures: squares for agents, locks for checks, arrows for notes, icons for decisions. Words only for the one question and the decisions log. Memories & Notes is no longer a tab (Joseph, 2026-11-14: "visual rather than text, simplicity over complexity") | Paragraphs on every card turn the cockpit into a report nobody reads |
| Sandbox cards | One proof per agent, drawn as a picture (a page, a request, a timeline), with the sentence on hover; no file names. Above 8 agents the cards collapse into one square per agent: grey done, blue working, outline waiting, amber needs you, red stuck (decided 2026-11-14) | File names are code. Cards stop being readable past 8; squares stay readable at 100 |
| Licence | AGPL-3.0 with a NOTICE file (decided 2026-11-15) | Open source, so open source events accept it. Copies must credit TaskWing and stay open, even when run as a hosted service |
| Cockpit look | Flight Deck is the v2 base (locked 2026-11-14) | Stops redesign churn; changes go into Flight Deck from here on |

## Prototypes

Sample data only. Nothing here is the product; it is what the v2 cockpit should look and feel like.

| Prototype | Files | Hosted copy | State |
|---|---|---|---|
| Flight Deck | `prototype/flight-deck/index.html` (open from disk) | https://claude.ai/artifact/Fdwt2HC66usmjLzyEvNYkM | Locked as the v2 base. Carbon's look plus the single question, the decisions log, proof windows, the hidden exam, exam approval before start, proof pictures and agent squares |
| Flight Deck, first pass | `prototype/flight-deck/carbon-original.html`, prompt in `brief.md` | none | Gemini Carbon's untouched output, kept for comparison |
| Cockpit boards | `prototype/cockpit/*.dc.html` (render only inside the claude.ai Design artifact) | https://claude.ai/artifact/LqXpXhWPuHmK4LQLfRdZmT | Earlier. Source of the proof window and exam ideas |

The hosted copies are private to the owner's claude.ai account.

## House style

- Every rule in a doc says what breaks without it.
- Tables and lists over prose.
