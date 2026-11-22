# TaskWing principles

TaskWing turns one goal into many sandboxed agents and brings them back as a few verified changes.

Every feature is built on these rules. Each rule says what breaks without it. A rule that cannot name what it prevents gets deleted.

## Thesis

Running hundreds of agents is easy. What runs out is conflict-free work and human review time. So TaskWing counts success in verified merges, not in agents running.

```
goal → plan + exam (human approves the exam) → N cards → N boxes, each running an agent
     → commits pulled to host → combine → exam in a clean box → review → one verified PR → watch
```

## Where TaskWing sits

```
┌ Control planes ─── GitHub Agent HQ, vendor cloud agents   (one task → one PR)
├ TaskWing ───────── one goal → N boxes → one verified PR   (headless, any vendor)
├ Agent interface ── ACP: Claude Code, Codex, Gemini CLI, Copilot CLI, ...
└ Sandboxes ──────── E2B, Daytona, Modal, Vercel, Cloudflare, Docker, any SSH host
```

TaskWing builds none of the layers above or below it. It plugs into them.

## Principles

| # | Principle | What breaks without it |
|---|---|---|
| 1 | **Only two adapters: Box and Agent.** Box requires only create, exec, copy, list, destroy. SSH, fork, snapshot, pause, webhooks and credential injection are optional capabilities an adapter declares; the core has a fallback for each. Agent = ACP. The core never imports a vendor SDK. | Lock-in to one vendor, and one vendor's quirks spread through the code. A required extra shuts out most providers (`context/sandbox-providers-2026-11.md`). |
| 2 | **Use standards, don't invent them.** ACP for agents, OCI images for environments, git for results, exec + file copy as the lowest common box. | Every new vendor costs a rewrite. Only 8 of 15 providers have native SSH. |
| 3 | **No real secret in a box.** Agents get a placeholder key and a base URL. A proxy outside the box (the provider's own injection when it has one, else TaskWing's) swaps in the real API key or subscription login. No git, GitHub or production credentials either: workers commit locally, the host pulls the commits and pushes. A login the proxy cannot swap (for example AWS request signing) is refused unless the run opts in. Subscription logins (Claude, ChatGPT) go only through the sandbox provider's own handling; TaskWing's proxy carries API keys only and never holds a subscription login. | Anyone with a shell in the box copies your keys and logins. One compromised agent pushes to main. Holding a subscription login ourselves would also break the provider's terms (`context/agent-logins.md`). |
| 4 | **Derive state, don't own it.** Run state is rebuilt from the provider's box list, git refs, GitHub and an append-only event log. | Double dispatch, leaked boxes and stale state after a crash. |
| 5 | **Treat worker output as hostile.** Read worker repos with hooks, fsmonitor and the ext protocol off. Never follow a symlink a worker wrote. | A malicious repo or a prompt injection runs code on the host. |
| 6 | **Record each dispatch durably before sending it. Never auto-retry an unknown outcome.** | Duplicate work and wasted credits. |
| 7 | **Events for agents, reconcile for boxes.** Agent progress comes from ACP session updates and hooks. Box state comes from a `list()` reconcile; provider webhooks only speed it up. | Polling agents at 100 boxes is slow and burns API calls. Waiting on box webhooks breaks on 12 of 15 providers that have none. |
| 8 | **A human approves the exam before dispatch; the engine checks the split.** The user reads the plain-sentence checks and says yes, or adds one. Nothing starts before that. The file split is not shown for approval: the engine refuses a plan where two cards own the same path or a seam has no owner. (Decided 2026-11-14.) | Without the OK, a misread task burns a whole run before anyone sees it. Asking the user to judge a file split asks them to read code, which they will skip. Without the engine check, cards overlap and seams go unowned. |
| 9 | **Success is verified merges, not agents running.** N agents end in one PR. Review time is the budget. | A flood of PRs that nobody can review. |
| 10 | **Only trusted feedback reaches agents:** reviewers GitHub confirms can write, and exact bot logins. | Prompt injection through PR comments. |
| 11 | **One binary, terminal first.** The Go engine does all the work. The CLI is the product: it draws the run as agent squares in the terminal and asks only what only the user can decide. The same runs are available as JSON events and commands (`taskwing engine --json`) for scripts, CI and editors. No desktop app (decided 2026-11-22; the Tauri app stays in git history). | A second front end in another language doubles the build, the release line and the places logic can drift, for the same runs. Without the JSON stream, runs can't be scripted or used in CI. |
| 12 | **The exam is written before the work, by a fresh session, and its tests are hidden.** At plan time a separate agent session, given only the task and the repo, writes the checks as plain sentences plus the tests behind them. The sentences are the goal: the user approves them and the lead may see them. The tests stay on the host, outside every box, and neither the lead nor any worker ever sees them. They run in a clean box on the combined work. After dispatch only the user can change a check. On a failure the exam session explains it in plain words; the lead rules whether the work or the check was wrong, and only the user can change a check. (Exam writer decided 2026-11-14.) | Workers edit tests until they pass, so "verified" means nothing. A lead agent that can loosen the exam does the same one level up. A check nobody can overrule fails good work with no way out (`context/harness-ui-patterns-2026-11.md`). |

## Open questions

| Question | Options | Leaning |
|---|---|---|
| Can herdr over SSH be the agent runtime in any box? | Spike for one day before committing | Untested |

## Non-goals

- A sandbox. Sandboxes are turning into a commodity; TaskWing plugs into them.
- A cockpit for unrelated, independent agents (Sanductor and Conductor do that). TaskWing shows one goal across many boxes.
- A desktop app. The terminal board and the JSON stream cover it.
- A hosted service.
