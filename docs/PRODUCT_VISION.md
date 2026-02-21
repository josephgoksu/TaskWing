# TaskWing: AI-Native Task Management

TaskWing helps me turn a goal into executed tasks with persistent context across AI sessions.

## Vision Statement

**TaskWing is AI-native task management that actually understands your codebase.**

We don't just store tasks. We generate context-aware development plans by analyzing your architecture, patterns, and decisions.

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
│  taskwing goal "..."  │  /tw-next  │  /tw-done   │
└─────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────┐
│                   TASK GENERATION                        │
│  Analyze goal → Query knowledge graph → Generate tasks   │
└─────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────┐
│               KNOWLEDGE GRAPH (The Moat)                 │
│  Features │ Patterns │ Decisions │ Constraints │ Files  │
└─────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────┐
│                    MCP SERVER                            │
│  Claude │ Cursor │ Copilot │ Codex — all get context    │
└─────────────────────────────────────────────────────────┘
```

## Core Commands

<!-- TASKWING_COMMANDS_START -->
- `taskwing bootstrap`
- `taskwing goal "<goal>"`
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

| Tier | Price | Features |
|------|-------|----------|
| Open Source | Free | Full CLI, local knowledge graph |
| Team | $29/seat/mo | Shared knowledge graph, team sync |
| Enterprise | Custom | SSO, audit, on-prem |

*The knowledge graph is the moat. Task management is the product.*
