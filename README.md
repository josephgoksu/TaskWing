# TaskWing

> TaskWing helps turn a goal into executed tasks with persistent context across AI sessions.

[![Go Report Card](https://goreportcard.com/badge/github.com/josephgoksu/TaskWing)](https://goreportcard.com/report/github.com/josephgoksu/TaskWing)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

## Supported Models

<!-- TASKWING_PROVIDERS_START -->

[![OpenAI](https://img.shields.io/badge/OpenAI-412991?logo=openai&logoColor=white)](https://platform.openai.com/)
[![Anthropic](https://img.shields.io/badge/Anthropic-191919?logo=anthropic&logoColor=white)](https://www.anthropic.com/)
[![Google Gemini](https://img.shields.io/badge/Google_Gemini-4285F4?logo=google&logoColor=white)](https://ai.google.dev/)
[![AWS Bedrock](https://img.shields.io/badge/AWS_Bedrock-OpenAI--Compatible_Beta-FF9900?logo=amazonaws&logoColor=white)](https://docs.aws.amazon.com/bedrock/latest/userguide/inference-chat-completions.html)
[![Ollama](https://img.shields.io/badge/Ollama-Local-000000?logo=ollama&logoColor=white)](https://ollama.com/)

<!-- TASKWING_PROVIDERS_END -->

## Works With

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

## Focused Workflow

```bash
# 1) Bootstrap project memory
cd your-project
taskwing bootstrap

# 2) Create and activate a plan from one goal
taskwing goal "Add Stripe billing"

# 3) Execute from your AI assistant
/tw-next
# ...work...
/tw-done
```

## What TaskWing Does

- Stores architecture decisions, constraints, and patterns in local project memory.
- Generates executable tasks from a goal using that memory.
- Exposes context and task lifecycle tools to AI assistants via MCP.

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

| Tool       | Description                                                                         |
| ---------- | ----------------------------------------------------------------------------------- |
| `ask`      | Search project knowledge (decisions, patterns, constraints)                         |
| `task`     | Unified task lifecycle (`next`, `current`, `start`, `complete`)                     |
| `plan`     | Plan management (`clarify`, `decompose`, `expand`, `generate`, `finalize`, `audit`) |
| `code`     | Code intelligence (`find`, `search`, `explain`, `callers`, `impact`, `simplify`)    |
| `debug`    | Diagnose issues systematically with AI-powered analysis                             |
| `remember` | Store knowledge in project memory                                                   |

<!-- TASKWING_MCP_TOOLS_END -->

## AWS Bedrock (OpenAI-Compatible) Setup

TaskWing supports Bedrock as a first-class provider for chat/planning/query flows.

```yaml
llm:
  provider: bedrock
  model: anthropic.claude-sonnet-4-5-20250929-v1:0
  bedrock:
    region: us-east-1
  apiKeys:
    bedrock: ${BEDROCK_API_KEY}
```

You can also configure it interactively:

```bash
taskwing config
```

Recommended Bedrock model IDs:

- `anthropic.claude-opus-4-6-v1` (highest quality reasoning)
- `anthropic.claude-sonnet-4-5-20250929-v1:0` (best default balance)
- `amazon.nova-premier-v1:0` (AWS flagship Nova)
- `amazon.nova-pro-v1:0` (strong balance)
- `meta.llama4-maverick-17b-instruct-v1:0` (open-weight strong general model)

## MCP Setup (Claude/Codex)

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

## Docs

- [Getting Started](docs/TUTORIAL.md)
- [Product Vision](docs/PRODUCT_VISION.md)
- [Architecture](docs/architecture/)
- [Workflow Contract v1](docs/WORKFLOW_CONTRACT_V1.md)
- [Workflow Pack](docs/WORKFLOW_PACK.md)

## License

MIT
