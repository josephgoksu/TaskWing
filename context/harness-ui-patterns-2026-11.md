# UI patterns from other agent harness tools (2026-11-14)

What other tools do, and what the TaskWing cockpit takes from them. Most comparison pages are written by vendors, so treat rankings as opinion. Only the sources below were read.

## Tools

| Tool | UI model | Worth noting |
|---|---|---|
| Sanductor, Conductor | Sidebar of workspaces, agent transcript, diff pane | Native feel. One agent per branch, no shared goal. Conductor has a Checks tab before merge. |
| Vibe Kanban | Kanban: To do, In progress, In review, Done | Several attempts per task. Reported shut down April 2026, community maintained (unconfirmed). |
| GitHub mission control | One list of agent tasks, live session log, PR link | Steer mid-run: pause, refine, restart. Review order: log, then files, then checks. |
| Codex `agents` dashboard | Terminal list grouped by project: active, blocked, completed | Terminal only. |
| attnbox | One inbox of "who is waiting on you and what for" | Each item names the action (approve, answer, review, unblock) and whether the status is certain or a guess. |
| Windsurf command center | Kanban of local and cloud sessions by status | Not read in detail (page 404). |

## Patterns taken

| Pattern | Source | Where it is in the prototype |
|---|---|---|
| Queue of what needs you, each with an action verb | attnbox, Crosley | "Your turn": Answer, Unblock, Review |
| Say when a status is a guess | attnbox | "looping?" card and "This is a guess from its activity" |
| Warn early when an agent repeats a failing approach | GitHub | Box 05 "may be looping", before it stalls |
| A phase per run | Crosley | Plan, Build, Combine, Review, PR in the task summary |
| Claim next to its proof | Crosley | Review pane: "It works: 22 of 22 tests pass" |
| Close with explicit gaps | Crosley | Whole PR tab: "Known gaps going into the PR" |
| Low-risk things pass quietly | Crosley | "14 small things were handled without you" |
| Recovery control beside the failure | Crosley | Restart sits on the stalled box's question |

## Patterns skipped

| Pattern | Why |
|---|---|
| Kanban columns | The floor already lays boxes out in work order. Columns by status would lose who feeds whom. |
| Several attempts per task | Doubles cost and review. Revisit if one agent often fails a card. |
| Chat as the main surface | Named as an anti-pattern: "a board, not a scroll". |
| Live terminal transcript as the main view | Joseph does not want to read code or commands in the cockpit. It stays one tab away (see tmux below). |

## Lessons from boat.dev and Sanductor (2026-11-14)

Read from Joseph's boat.dev dashboard screenshots (Agents & Proxying) and Sanductor's README text (features, getting started, configuration).

| Lesson | Source | Where it is in the prototype |
|---|---|---|
| One row per agent: dot, name, one-line login summary | boat | Settings: Agents list |
| Star marks the default | boat | Star picks the lead agent |
| Cost and auto-stop shown per sandbox | boat | Agent tab: "$1.20 so far, stops itself after 45 quiet minutes" |
| The login lives with the provider, the app only shows its state | boat | "Logins live in boat. TaskWing never holds them." |
| First-run checks with a Recheck button, each naming the command to run | Sanductor | Setup board |
| Start from a prompt, an issue or a PR | Sanductor | "from" chip on the new-task box |
| Schedules: once, daily, weekly, monthly | Sanductor | "repeat" chip on the new-task box |
| After the PR opens, fix reviews and CI, never merge or push without the user | Sanductor | PR tab: "After the pull request opens" |
| Agents keep running after the app quits | Sanductor (herdr) | tmux session, see below |
| Sandbox as a locked-down local user, no containers | Sanductor (sandvault) | "Sandboxes run on: This Mac" (option shown, not designed further) |

## tmux as the terminal door (2026-11-14)

| Fact | Consequence |
|---|---|
| The lead agent is the real Claude Code running in a tmux session named `taskwing` | The flat subscription keeps working, and the session outlives the app |
| `tmux attach -t taskwing` opens the same session in the user's own terminal | The cockpit and the terminal are two views of one agent, not two agents |
| Typing `/swarm` in that session starts a task | A task started in the terminal shows up in the cockpit |
| `taskwing attach NN` opens one worker's session | Deep inspection without putting a transcript in the main view |

What breaks without it: the app would need its own agent loop to plan, which means an API key and per-token cost.

Not verified: that tmux windows can be mirrored into a Tauri view without lag, and that boat exposes a worker's session for attach.

## Proof windows (decided 2026-11-14)

Each sandbox card shows live proof of what its agent is building, instead of only a status word.

| Rule | What breaks without it |
|---|---|
| The lead agent names one watch target per piece in the plan: a page, a request, a test command or a file | The engine would have to guess what to show, which needs model code in the engine |
| The window shows proof of whatever kind fits: a page for UI, one line for an API, a test count for checks | A screenshot on every card is blank for most backend pieces |
| A broken or building app keeps its last good frame | Nine error screens mid-run |
| Before and Now can be flipped per agent, and the PR opens with before/after pairs | Review falls back to reading code |
| Build order: the before/after gallery at the end first, live windows second | The gallery is most of the review value and needs no live capture |

Tripwire: if fewer than half the pieces in real tasks have something to show, the window moves to the selected agent only.

Not verified: whether boat exposes a preview port per box, and the memory cost of several local dev servers.

## Hidden exam (decided 2026-11-14)

Before any agent starts, the acceptance checks for the finished work are written and the user approves them. The agents never see the tests. Changed 2026-11-14: a fresh exam session writes them, not the lead, and the user approves the exam instead of the plan (`PRINCIPLES.md` #8 and #12).

| Rule | What breaks without it |
|---|---|
| The exam is written at plan time, before the first agent starts | Checks written after the work describe what was built, not what was asked |
| Agents cannot read or change the exam; it lives outside every agent's sandbox | An agent edits a test until it passes |
| The exam runs in a clean sandbox on the joined work | A pass that depends on one agent's leftover state |
| Each check is one plain sentence the user can read | The user has to read test code to know what "verified" means |
| A product answer from the user becomes a check | The answer is given once and never enforced |
| The user can add a check in plain words; the lead agent turns it into a test | The exam only covers what the lead agent thought of |
| Once work starts, changing a check needs the user's OK | The lead agent loosens the exam to pass it |
| On a failed check the lead agent says whether the work or the exam was wrong | A bad check fails good work with no way out |

In the prototype: a dark exam card on the floor, the list of checks in the "Exam · PR" tab, and the first line under "Why you can trust it" in the finished view.

Not verified: how the engine keeps the exam files out of an agent's reach on each sandbox provider.

## What breaks without this

Without a single queue, the user polls every box. Without proof next to each claim, approval is trust in prose.

## Sources

- https://blakecrosley.com/blog/agents-need-supervision-surfaces
- https://github.blog/ai-and-ml/github-copilot/how-to-orchestrate-agents-using-mission-control/
- https://github.com/wookat/attnbox
- https://sitebloom.pages.dev/writing/the-current-shape-of-agent-orchestration
- https://www.vibekanban.com/docs/core-features/reviewing-code-changes
- https://www.conductor.build/docs
- https://github.com/MikeMcQuaid/Sanductor (README text pasted by Joseph)
- boat.dev dashboard, Agents & Proxying page (screenshots from Joseph)
