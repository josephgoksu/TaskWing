<h1 align="center">
  <br>
  <img src="https://taskwing.app/taskwing-icon.svg" alt="TaskWing" width="80">
  <br>
  TaskWing
  <br>
</h1>

<h3 align="center">Your AI assistant forgets everything. <em>Every single session.</em></h3>

<p align="center">
  Context compression tools save tokens. TaskWing saves <strong>knowledge</strong> — decisions, patterns, and architecture that compound across every session.
</p>

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

## Works With

<!-- TASKWING_TOOLS_START -->
[![Claude Code](https://img.shields.io/badge/Claude_Code-191919?logo=anthropic&logoColor=white)](https://www.anthropic.com/claude-code)
[![OpenAI Codex](https://img.shields.io/badge/OpenAI_Codex-412991?logo=openai&logoColor=white)](https://developers.openai.com/codex)
[![Cursor](https://img.shields.io/badge/Cursor-111111?logo=cursor&logoColor=white)](https://cursor.com/)
[![GitHub Copilot](https://img.shields.io/badge/GitHub_Copilot-181717?logo=githubcopilot&logoColor=white)](https://github.com/features/copilot)
[![Gemini CLI](https://img.shields.io/badge/Gemini_CLI-4285F4?logo=google&logoColor=white)](https://github.com/google-gemini/gemini-cli)
[![OpenCode](https://img.shields.io/badge/OpenCode-000000?logo=opencode&logoColor=white)](https://opencode.ai/)
<!-- TASKWING_TOOLS_END -->

<p align="center">
  <img src="demos/ask.gif" alt="TaskWing ask demo" width="800">
</p>

---

## The Problem

You explain "we use PostgreSQL, not MongoDB" on Monday. Again on Tuesday. Again on Wednesday. Your AI assistant has no memory. **Every session starts from zero.**

A typical project accumulates 50+ architectural decisions, dozens of patterns, and critical constraints — none of which survive a session restart. You spend more time re-explaining context than building features.

Context compression tools reduce token waste within a session. But when the session ends, everything is gone. **The real problem isn't token cost — it's knowledge loss.**

## How TaskWing Fixes This

```
Without TaskWing:
  Session 1: "We use PostgreSQL, here's why..." (re-explain)
  Session 2: "We use PostgreSQL, here's why..." (re-explain)
  Session 3: "We use PostgreSQL, here's why..." (re-explain)

With TaskWing:
  taskwing bootstrap → 63 decisions, 28 patterns, 12 constraints extracted
  Session 1: AI already knows your architecture
  Session 2: AI still knows. Plus what you decided yesterday.
  Session 3: AI knows everything. Context compounds.
```

One command extracts your architecture into a local SQLite database. Every AI session after that just *knows* — permanently.

| What | How |
|:-----|:----|
| **Persistent memory** | Decisions, patterns, and constraints survive every session restart |
| **Goal-to-execution** | Turn "Add Stripe billing" into 5 decomposed, context-rich tasks |
| **Code intelligence** | Symbol search, call graphs, impact analysis across your codebase |
| **Works everywhere** | Claude Code, Cursor, Copilot, Gemini, OpenCode — via MCP |

## Try It (60 seconds)

```bash
# Install
brew install josephgoksu/tap/taskwing

# Extract your architecture
cd your-project && taskwing bootstrap
# → "63 decisions, 28 patterns, 12 constraints extracted"

# Ask it anything about your project
taskwing ask "what database do we use and why?"
# → Returns the decision, reasoning, and tradeoffs — instantly
```

No signup. No account. Works offline. Everything stays local in SQLite.

## Full Workflow

```bash
# 1. Bootstrap (once per project)
taskwing bootstrap

# 2. Set a goal and generate a plan
taskwing plan "Add Stripe billing"
# → Plan decomposed into 5 executable tasks with architectural context

# 3. Execute with your AI assistant
/tw-next       # Get next task — AI already knows your stack
# ...work...
/tw-done       # Mark complete, advance to next

# 4. Your AI remembers decisions made today — tomorrow, next week, forever
/tw-ask "why did we choose Stripe over Paddle?"
# → Returns the decision from step 2, with full reasoning
```

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

## Why Not Just a CLAUDE.md File?

| | CLAUDE.md | Context compression tools | TaskWing |
|:--|:---------|:-------------------------|:---------|
| **Survives session restart** | Yes | No | Yes |
| **Auto-extracted from code** | No (hand-written) | No | Yes |
| **Searchable** | No | Session only | Always (FTS + vector + graph) |
| **Grows over time** | Only if you maintain it | No | Automatically |
| **Understands code symbols** | No | No | Call graphs, impact analysis |
| **Plans and tracks tasks** | No | No | Goal → plan → execute → verify |

CLAUDE.md is a good start. Context compression is useful within a session. TaskWing is what happens when your project intelligence becomes **permanent, searchable, and compounding**.

## MCP Tools

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

## MCP Setup

Add to your AI tool's MCP config:

```json
{
  "mcpServers": {
    "taskwing-mcp": {
      "command": "taskwing",
      "args": ["mcp"]
    }
  }
}
```

## Slash Commands

Once connected, use these slash commands from your AI assistant:

| Command | When to use |
|:--------|:------------|
| `/tw-ask` | Search project knowledge (decisions, patterns, constraints) |
| `/tw-remember` | Persist a decision, pattern, or insight to project memory |
| `/tw-next` | Start the next approved task with full context |
| `/tw-done` | Complete the current task after verification |
| `/tw-status` | Check current task progress and acceptance criteria |
| `/tw-plan` | Clarify a goal and build an approved execution plan |
| `/tw-debug` | Root-cause-first debugging before proposing fixes |
| `/tw-explain` | Deep explanation of a code symbol and its call graph |
| `/tw-simplify` | Simplify code while preserving behavior |

## Core Commands

<!-- TASKWING_COMMANDS_START -->
- `taskwing bootstrap`
- `taskwing plan "<description>"`
- `taskwing ask "<query>"`
- `taskwing task`
- `taskwing plan status`
- `taskwing slash`
- `taskwing mcp`
- `taskwing doctor`
- `taskwing config`
- `taskwing start`
<!-- TASKWING_COMMANDS_END -->

## Autonomous Task Execution (Hooks)

TaskWing integrates with Claude Code's hook system for autonomous plan execution:

```bash
taskwing hook session-init      # Initialize session tracking
taskwing hook continue-check    # Check if should continue to next task
taskwing hook session-end       # Cleanup session
taskwing hook status            # View current session state
```

**Circuit breakers** prevent runaway execution:
- `--max-tasks=5` — Stop after N tasks for human review
- `--max-minutes=30` — Stop after N minutes

## AWS Bedrock Setup

TaskWing supports Bedrock as a first-class provider:

```yaml
llm:
  provider: bedrock
  model: anthropic.claude-sonnet-4-5-20250929-v1:0
  bedrock:
    region: us-east-1
  apiKeys:
    bedrock: ${BEDROCK_API_KEY}
```

<details>
<summary>Recommended Bedrock models</summary>

| Model | Use case |
|:------|:---------|
| `anthropic.claude-opus-4-6-v1` | Highest quality reasoning |
| `anthropic.claude-sonnet-4-5-20250929-v1:0` | Best default balance |
| `amazon.nova-premier-v1:0` | AWS flagship Nova |
| `amazon.nova-pro-v1:0` | Strong balance |
| `meta.llama4-maverick-17b-instruct-v1:0` | Open-weight general model |

</details>

Or configure interactively: `taskwing config`

## Documentation

- [Getting Started](docs/TUTORIAL.md)
- [Product Vision](docs/PRODUCT_VISION.md)
- [Architecture](docs/architecture/)
- [Workflow Contract v1](docs/WORKFLOW_CONTRACT_V1.md)
- [Workflow Pack](docs/WORKFLOW_PACK.md)

## License

[MIT](LICENSE)
