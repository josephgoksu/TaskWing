# Design and build a prototype

You are the designer and the builder. Build a clickable desktop prototype of an app, with sample data, as ONE self-contained file named `index.html` in the current directory. No build step. It must work when opened from disk in a browser. Desktop only, no phone layout.

There is no specification. Use your own imagination and taste. The only inputs are what the product does and what the user said he wants, below. Everything else is yours to decide: the concept, the screens, the layout, the metaphors, the interactions, the look.

## What the product does

TaskWing is an open-source tool. The user gives it one task for a code repository. A lead agent (the user's own coding agent, running on his Mac in a terminal session, on his flat subscription) understands the task, improves the prompt, splits it into pieces, and starts many coding agents (Claude Code, Codex, Gemini and others). Each agent works in its own cloud sandbox on its own files. The agents can pass notes to each other. Before work starts, the lead agent writes acceptance checks that the worker agents never see; the finished, combined work must pass them in a clean sandbox. The result is one verified pull request for the user to merge.

## What the user said he wants (his own words)

- "I want to run my sandbox agent from one place... see visually what's happening. So I don't need to ask the agent every time where are we at."
- "I want to give a task. I want to use all agentic capabilities. It must be fully agentic application. After that, it should be easily doing the rest of things. I don't want to manage a lot of things. Yes, I should have access to settings, the details, but what I want is very beautiful UI and UX."
- He saw a native Mac app and said: "Look how beautiful it looks. It looks like a native app. This is something that makes me happy."
- "I don't wanna see code changes."
- "I would like to see mini boxes that represent AI sandboxes."
- "It would be nice to see agent memories and their interaction with each other."
- "Remember it's a task based swarm of agents. So the task must be summarized and visible shortly. Imagine I have 23 different swarm histories in one project."
- "Improve agent and sandbox representation. Be creative. Use best practices."
- He wants to be able to create, change and delete a task, and to talk to a single agent.
- Planning is done by the agent, not by filling a form.
- He may also open the agents' terminal sessions himself, and may even start a task from there.

## His reactions to earlier attempts

- Dense "manager" dashboards with many panels: he did not like them.
- A minimal one-input-box version: "it's too simple".
- His latest ask: "Be more creative. Come with crazy good ideas."

## Rules

- One file, `index.html`, in the current directory. Inline CSS and JS. No frameworks to install, no network needed to run.
- It must be interactive: a person should be able to click through a task from start to merged pull request.
- Plain words in the UI. No lorem ipsum. Realistic sample data.
- This session cannot run shell commands or open a browser: any command is refused and ends the run. Use only your file tools. Write `index.html` directly with your file-writing tool.
- When you finish, reply with a short note: the idea in one sentence, and what can be clicked.
