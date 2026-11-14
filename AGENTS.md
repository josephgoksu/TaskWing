# TaskWing

Headless, vendor-agnostic orchestrator: one goal, many sandboxed agents, one verified PR. Go, MIT.

The v1 knowledge layer was removed in November 2026. Nothing from it carries over.

## Read first

| File | Why |
|---|---|
| `PRINCIPLES.md` | The foundation. Every change must fit it. A change that breaks a principle updates the principle first, with its reason. |
| `context/` | Research and decisions behind the principles. |

## Decisions

| Decision | Choice | Why |
|---|---|---|
| Where agents run | Inside the box | Works with every agent CLI unchanged and scales to hundreds of boxes |
| How agents log in | Placeholder key + base URL; a proxy outside the box swaps in the real credential (`PRINCIPLES.md` #3). Subscription logins go through the sandbox provider (boat); TaskWing's proxy carries API keys only (decided 2026-11-14) | Agent CLIs offer many login methods (`context/agent-logins.md`). Holding a subscription login ourselves breaks the provider's terms; boat already handles it |
| Who plans | The user's own agent (the real Claude Code, started on their machine) leads with a TaskWing skill. The engine has no model code (decided 2026-11-14) | Keeps the flat subscription, works with any vendor's agent, and the engine only checks and runs the plan |
| Cockpit app | Tauri, with the Go engine bundled as a sidecar (decided 2026-11-13) | Reuses the web UI from the prototype, runs on Mac, Linux and Windows. SwiftUI was rejected: Mac only, full rewrite, head-on with Sanductor |
| Phone pings | A push service called from the engine, not a phone app | Works on day one with no app to build |

## Prototypes

Sample data only. Nothing here is the product; it is what the v2 cockpit should look and feel like.

| Prototype | Files | Hosted copy | State |
|---|---|---|---|
| Flight Deck | `prototype/flight-deck/index.html` (open from disk) | https://claude.ai/artifact/Fdwt2HC66usmjLzyEvNYkM | Current. Carbon's look plus the single question, the decisions log, proof windows and the hidden exam |
| Flight Deck, first pass | `prototype/flight-deck/carbon-original.html`, prompt in `brief.md` | none | Gemini Carbon's untouched output, kept for comparison |
| Cockpit boards | `prototype/cockpit/*.dc.html` (render only inside the claude.ai Design artifact) | https://claude.ai/artifact/LqXpXhWPuHmK4LQLfRdZmT | Earlier. Source of the proof window and exam ideas |

The hosted copies are private to the owner's claude.ai account.

## House style

- Every rule in a doc says what breaks without it.
- Tables and lists over prose.
