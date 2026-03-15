# TaskWing: Local-First AI Knowledge Layer

TaskWing extracts architectural knowledge from codebases and stores it locally, giving every AI coding tool instant context without your knowledge base leaving your machine.

## Vision Statement

**TaskWing is the local-first knowledge layer for AI-assisted development.**

We extract your architecture -- decisions, patterns, constraints -- into a local SQLite database. Every AI tool gets instant context via MCP, without your codebase flowing through someone else's cloud.

## Private by Architecture

| Property | How |
|----------|-----|
| Knowledge stored locally | SQLite on your filesystem. No cloud database, no sync. |
| No account required | Install via brew and use immediately. Zero data collection surface. |
| AI tools connect locally | MCP queries served over local stdio. No network calls for queries. |
| Air-gappable | Ollama support for fully offline operation. Zero external dependencies. |
| Open source (MIT) | Audit every line. Fork it. Run it on your own terms. |
| No vendor kill switch | MIT license, SQLite storage, standard MCP protocol. |

**What TaskWing controls:** During bootstrap, code context is processed by your chosen LLM provider (cloud or Ollama for full local). After extraction, your knowledge base is stored and queried locally. MCP responses are served over local stdio and never touch the network.

**What your AI tool controls:** Cloud-based AI tools (Claude, Cursor, Copilot) send conversations -- including TaskWing's MCP responses -- to their own servers per their privacy policies. TaskWing cannot control this. To keep everything local, use Ollama for bootstrap and a local AI tool for queries.

## Ecosystem Support

### Supported Models

<!-- TASKWING_PROVIDERS_START -->
[![OpenAI](https://img.shields.io/badge/OpenAI-412991?logo=openai&logoColor=white)](https://platform.openai.com/)
[![Anthropic](https://img.shields.io/badge/Anthropic-191919?logo=anthropic&logoColor=white)](https://www.anthropic.com/)
[![Google Gemini](https://img.shields.io/badge/Google_Gemini-4285F4?logo=google&logoColor=white)](https://ai.google.dev/)
[![AWS Bedrock](https://img.shields.io/badge/AWS_Bedrock-OpenAI--Compatible_Beta-FF9900?logo=amazonaws&logoColor=white)](https://docs.aws.amazon.com/bedrock/latest/userguide/inference-chat-completions.html)
[![Ollama](https://img.shields.io/badge/Ollama-Local-000000?logo=ollama&logoColor=white)](https://ollama.com/)
<!-- TASKWING_PROVIDERS_END -->

### Works With

<!-- TASKWING_TOOLS_START -->
[![Claude Code](https://img.shields.io/badge/Claude_Code-191919?logo=anthropic&logoColor=white)](https://www.anthropic.com/claude-code)
[![OpenAI Codex](https://img.shields.io/badge/OpenAI_Codex-412991?logo=openai&logoColor=white)](https://developers.openai.com/codex)
[![Cursor](https://img.shields.io/badge/Cursor-111111?logo=cursor&logoColor=white)](https://cursor.com/)
[![GitHub Copilot](https://img.shields.io/badge/GitHub_Copilot-181717?logo=githubcopilot&logoColor=white)](https://github.com/features/copilot)
[![Gemini CLI](https://img.shields.io/badge/Gemini_CLI-4285F4?logo=google&logoColor=white)](https://github.com/google-gemini/gemini-cli)
[![OpenCode](https://img.shields.io/badge/OpenCode-000000?logo=opencode&logoColor=white)](https://opencode.ai/)
<!-- TASKWING_TOOLS_END -->

<!-- TASKWING_LEGAL_START -->
Brand names and logos are trademarks of their respective owners; usage here indicates compatibility, not endorsement.
<!-- TASKWING_LEGAL_END -->

## Architecture

```text
┌─────────────────────────────────────────────────────────┐
│                    USER INTERFACE                        │
│  taskwing goal "..."  │  /taskwing:next  │  /taskwing:done   │
└─────────────────────────────────────────────────────────┘
                              │
                              v
┌─────────────────────────────────────────────────────────┐
│                   TASK GENERATION                        │
│  Analyze goal -> Query knowledge graph -> Generate tasks │
└─────────────────────────────────────────────────────────┘
                              │
                              v
┌─────────────────────────────────────────────────────────┐
│            LOCAL KNOWLEDGE GRAPH (The Moat)              │
│  Features │ Patterns │ Decisions │ Constraints │ Files  │
│            Stored in local SQLite -- never synced        │
└─────────────────────────────────────────────────────────┘
                              │
                              v
┌─────────────────────────────────────────────────────────┐
│                 LOCAL MCP SERVER (stdio)                  │
│  Claude │ Cursor │ Copilot │ Codex -- all get context   │
│            No network calls for queries                  │
└─────────────────────────────────────────────────────────┘
```

## Core Commands

<!-- TASKWING_COMMANDS_START -->
- `taskwing bootstrap`
- `taskwing goal "<goal>"`
- `taskwing ask "<query>"`
- `taskwing task`
- `taskwing plan status`
- `taskwing slash`
- `taskwing mcp`
- `taskwing doctor`
- `taskwing config`
- `taskwing start`
<!-- TASKWING_COMMANDS_END -->

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

## Success Metrics

1. Task accuracy: generated tasks reference correct files and patterns.
2. Developer adoption: daily active users running `taskwing goal`.
3. Context utilization: MCP queries per plan execution.
4. Time-to-root-cause: bug investigations with TaskWing context vs. without.

## Monetization (Future)

| Tier        | Price       | Features                          |
| ----------- | ----------- | --------------------------------- |
| Open Source | Free        | Full CLI, local knowledge graph   |
| Team        | $29/seat/mo | Shared knowledge graph, team sync |
| Enterprise  | Custom      | SSO, audit, on-prem               |

_The local knowledge graph is the moat. AI-assisted development is the product._
