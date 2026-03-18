<h1 align="center">
  <br>
  <img src="https://taskwing.app/taskwing-icon.svg" alt="TaskWing" width="80">
  <br>
  TaskWing
  <br>
</h1>

<h3 align="center">The local-first knowledge layer for AI development.</h3>

<p align="center">
  <a href="https://taskwing.app">Website</a> ·
  <a href="docs/TUTORIAL.md">Tutorial</a> ·
  <a href="docs/PRODUCT_VISION.md">Vision</a> ·
  <a href="#install">Install</a>
</p>

<p align="center">
  <a href="https://goreportcard.com/report/github.com/josephgoksu/TaskWing"><img src="https://goreportcard.com/badge/github.com/josephgoksu/TaskWing" alt="Go Report Card"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-blue.svg" alt="License"></a>
</p>

<p align="center">
  <img src="demos/ask.gif" alt="TaskWing ask demo" width="800">
</p>

---

Your AI tools start every session from zero -- and every session, your code context flows through someone else's cloud.

**TaskWing takes the opposite approach.** One command extracts your architecture into a local knowledge base on your machine. No cloud. No account. Every AI session after that just *knows* -- without your knowledge base leaving your infrastructure.

```
Without TaskWing              With TaskWing
─────────────────             ─────────────
8-12 file reads               1 MCP query
~25,000 tokens                ~1,500 tokens
2-3 minutes                   42 seconds
No architectural context       170+ knowledge nodes
```

## Install

```bash
brew install josephgoksu/tap/taskwing
```

No signup. No account. Works offline. Everything stays local in SQLite.

<details>
<summary>Alternative: install via curl</summary>

```bash
curl -fsSL https://taskwing.app/install.sh | sh
```
</details>

## Quick Start

```bash
# 1. Extract your architecture (one-time)
cd your-project
taskwing bootstrap
# -> 22 decisions, 12 patterns, 9 constraints extracted

# 2. Connect to your AI tool
taskwing mcp install claude    # or: cursor, gemini, codex, copilot, opencode

# 3. Plan and execute with your AI assistant
/taskwing:plan       # Create a plan via MCP
/taskwing:next       # Get next task with full context
# ...work...
/taskwing:done       # Mark complete, advance to next
```

That's it. Your AI assistant now has local architectural context across every session.

## Private by Architecture

TaskWing keeps your knowledge base on your machine. No cloud database, no account, no sync.

```
  YOUR MACHINE                          EXTERNAL
  ─────────────────────────────────     ─────────────────────────
                                        ┌───────────────────────┐
  ┌──────────────┐   code context       │ LLM Provider          │
  │ Your codebase ├────────────────────>│ (OpenAI, Anthropic,   │
  └──────────────┘   (bootstrap only)   │  Google, Bedrock)     │
         │                              └───────────┬───────────┘
         │                                          │ findings
         v                                          │
  ┌──────────────────────┐  <───────────────────────┘
  │ .taskwing/memory.db  │
  │ Local SQLite         │  Your knowledge base.
  │ Never uploaded.      │  Never leaves your machine.
  └──────────┬───────────┘
             │ local stdio (MCP)
             v
  ┌──────────────────────┐              ┌───────────────────────┐
  │ AI Tool              │  may send    │ Tool's own cloud      │
  │ (Claude, Cursor,     ├─────────────>│ (per their privacy    │
  │  Copilot, Gemini)    │  to their    │  policy)              │
  └──────────────────────┘  servers     └───────────────────────┘


  FULL AIR-GAP (everything stays left of the line):

  ┌──────────────┐        ┌─────────┐        ┌──────────────┐
  │ Your codebase ├──────>│ Ollama  ├──────>│ .taskwing/   │
  └──────────────┘        │ (local) │        │ memory.db    │
                          └─────────┘        └──────┬───────┘
                                                    │ local stdio
                                                    v
                                             ┌──────────────┐
                                             │ Local AI tool │
                                             └──────────────┘
                                             Zero network calls.
```

**What TaskWing controls:** Your knowledge base is stored and queried locally. MCP serves responses over local stdio -- no network calls.

**What your AI tool controls:** Cloud-based tools (Claude, Cursor, Copilot) may send conversations to their own servers. Check their privacy settings (e.g., Cursor's Privacy Mode, Copilot's data retention policies).

**Full air-gap:** Use [Ollama](https://ollama.com/) for bootstrap + a local AI tool. Nothing leaves your machine.

## Works With

<!-- TASKWING_TOOLS_START -->
[![Claude Code](https://img.shields.io/badge/Claude_Code-191919?logo=anthropic&logoColor=white)](https://www.anthropic.com/claude-code)
[![OpenAI Codex](https://img.shields.io/badge/OpenAI_Codex-412991?logo=openai&logoColor=white)](https://developers.openai.com/codex)
[![Cursor](https://img.shields.io/badge/Cursor-111111?logo=cursor&logoColor=white)](https://cursor.com/)
[![GitHub Copilot](https://img.shields.io/badge/GitHub_Copilot-181717?logo=githubcopilot&logoColor=white)](https://github.com/features/copilot)
[![Gemini CLI](https://img.shields.io/badge/Gemini_CLI-4285F4?logo=google&logoColor=white)](https://github.com/google-gemini/gemini-cli)
[![OpenCode](https://img.shields.io/badge/OpenCode-000000?logo=opencode&logoColor=white)](https://opencode.ai/)
<!-- TASKWING_TOOLS_END -->

## Supported Models

<!-- TASKWING_PROVIDERS_START -->
[![OpenAI](https://img.shields.io/badge/OpenAI-412991?logo=openai&logoColor=white)](https://platform.openai.com/)
[![Anthropic](https://img.shields.io/badge/Anthropic-191919?logo=anthropic&logoColor=white)](https://www.anthropic.com/)
[![Google Gemini](https://img.shields.io/badge/Google_Gemini-4285F4?logo=google&logoColor=white)](https://ai.google.dev/)
[![AWS Bedrock](https://img.shields.io/badge/AWS_Bedrock-OpenAI--Compatible_Beta-FF9900?logo=amazonaws&logoColor=white)](https://docs.aws.amazon.com/bedrock/latest/userguide/inference-chat-completions.html)
[![Ollama](https://img.shields.io/badge/Ollama-Local-000000?logo=ollama&logoColor=white)](https://ollama.com/)
<!-- TASKWING_PROVIDERS_END -->

<!-- TASKWING_LEGAL_START -->
Brand names and logos are trademarks of their respective owners; usage here indicates compatibility, not endorsement.
<!-- TASKWING_LEGAL_END -->

## What It Does

| Capability | Description |
|:-----------|:------------|
| **Local knowledge** | Extracts decisions, patterns, and constraints into local SQLite |
| **Goal to tasks** | Turns a goal into an executable plan with decomposed tasks |
| **AI-driven lifecycle** | Task execution -- next, start, complete, verify |
| **Code analysis** | Symbol search, call graphs, impact analysis, simplification |
| **Root cause first** | AI-powered diagnosis before proposing fixes |
| **Works everywhere** | Exposes everything to 6+ AI tools via local MCP |

## Slash Commands

Use these from your AI assistant once connected:

| Command | When to use |
|:--------|:------------|
| `/taskwing:ask` | Search project knowledge (decisions, patterns, constraints) |
| `/taskwing:remember` | Persist a decision, pattern, or insight to project memory |
| `/taskwing:next` | Start the next approved task with full context |
| `/taskwing:done` | Complete the current task after verification |
| `/taskwing:status` | Check current task progress and acceptance criteria |
| `/taskwing:plan` | Clarify a goal and build an approved execution plan |
| `/taskwing:debug` | Root-cause-first debugging before proposing fixes |
| `/taskwing:explain` | Deep explanation of a code symbol and its call graph |
| `/taskwing:simplify` | Simplify code while preserving behavior |

<details>
<summary>MCP setup (manual)</summary>

`taskwing mcp install` handles this automatically. If you need to configure manually, add to your AI tool's MCP config:

```json
{
  "mcpServers": {
    "taskwing": {
      "command": "taskwing",
      "args": ["mcp"]
    }
  }
}
```

<!-- TASKWING_MCP_TOOLS_START -->
| Tool | Description |
|------|-------------|
| `ask` | Search project knowledge (decisions, patterns, constraints) |
| `task` | Unified task lifecycle (`next`, `current`, `start`, `complete`) |
| `plan` | Plan management (`clarify`, `decompose`, `expand`, `generate`, `finalize`, `audit`) |
| `code` | Code intelligence (`find`, `search`, `explain`, `callers`, `impact`, `simplify`) |
| `debug` | Diagnose issues systematically with AI-powered analysis |
| `remember` | Store knowledge in project memory |
<!-- TASKWING_MCP_TOOLS_END -->

</details>

<details>
<summary>Autonomous task execution (hooks)</summary>

TaskWing integrates with Claude Code's hook system for autonomous plan execution:

```bash
taskwing hook session-init      # Initialize session tracking
taskwing hook continue-check    # Check if should continue to next task
taskwing hook session-end       # Cleanup session
taskwing hook status            # View current session state
```

**Circuit breakers** prevent runaway execution:
- `--max-tasks=5` -- Stop after N tasks for human review
- `--max-minutes=30` -- Stop after N minutes

</details>

<details>
<summary>AWS Bedrock setup</summary>

```yaml
llm:
  provider: bedrock
  model: anthropic.claude-sonnet-4-5-20250929-v1:0
  bedrock:
    region: us-east-1
  apiKeys:
    bedrock: ${BEDROCK_API_KEY}
```

| Model | Use case |
|:------|:---------|
| `anthropic.claude-opus-4-6-v1` | Highest quality reasoning |
| `anthropic.claude-sonnet-4-5-20250929-v1:0` | Best default balance |
| `amazon.nova-premier-v1:0` | AWS flagship Nova |
| `amazon.nova-pro-v1:0` | Strong balance |
| `meta.llama4-maverick-17b-instruct-v1:0` | Open-weight general model |

Or configure interactively: `taskwing config`

</details>

<!-- TASKWING_COMMANDS_START -->
- `taskwing bootstrap`
- `taskwing ask "<query>"`
- `taskwing task`
- `taskwing mcp`
- `taskwing doctor`
- `taskwing config`
- `taskwing start`
<!-- TASKWING_COMMANDS_END -->

## Documentation

- [Getting Started](docs/TUTORIAL.md)
- [Product Vision](docs/PRODUCT_VISION.md)
- [Architecture](docs/architecture/)
- [Workflow Pack](docs/WORKFLOW_PACK.md)

## License

[MIT](LICENSE)
