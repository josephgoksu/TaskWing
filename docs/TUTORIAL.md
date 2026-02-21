# TaskWing Tutorial

TaskWing helps you move from one goal to executed tasks while keeping architecture context persistent across AI sessions.

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

## 1. Bootstrap

```bash
cd your-project
taskwing bootstrap
```

This creates `.taskwing/` and installs AI assistant integration files.

## 2. Create and Activate a Plan

```bash
taskwing goal "Add user authentication"
```

`taskwing goal` runs clarify -> generate -> activate in one step.

## 3. Execute with Slash Commands

In your AI tool:

```text
/tw-next
```

When done:

```text
/tw-done
```

Check current status:

```text
/tw-status
```

## 3.5. First-Run Success Loop (<15 minutes)

Use this minimum loop to validate TaskWing end-to-end:

1. `/tw-plan <goal>` and approve the clarified checkpoint
2. `/tw-next` and approve the implementation checkpoint
3. Make a scoped change
4. `/tw-done` with fresh verification evidence

If you complete this loop once, your setup is healthy and your assistant workflow is aligned with TaskWing contracts.

## 4. Inspect Progress from CLI

```bash
taskwing plan status
taskwing task list
```

## 5. MCP Server

Run MCP server when your AI tool needs stdio MCP integration:

```bash
taskwing mcp
```

## 6. Local Runtime (Optional)

Run TaskWing API/dashboard tooling locally:

```bash
taskwing start
```

Default bind is `127.0.0.1`.

## 7. Troubleshooting

```bash
taskwing doctor
taskwing config show
```

Repair workflow:

```bash
# Apply managed repairs + MCP fixes
taskwing doctor --fix --yes

# Adopt unmanaged TaskWing-like AI files (with backup) and repair
taskwing doctor --fix --adopt-unmanaged --yes --ai claude
```

Bootstrap behavior during drift:

- Managed local drift: `taskwing bootstrap` auto-repairs.
- Unmanaged drift: bootstrap warns and points to `doctor --fix --adopt-unmanaged`.
- Global MCP drift: bootstrap warns and points to `doctor --fix`.

## 8. Optional: Use AWS Bedrock

You can select Bedrock from the interactive config flow:

```bash
taskwing config
```

Or configure directly:

```yaml
llm:
  provider: bedrock
  model: anthropic.claude-sonnet-4-5-20250929-v1:0
  bedrock:
    region: us-east-1
  apiKeys:
    bedrock: ${BEDROCK_API_KEY}
```

Recommended Bedrock model IDs:
- `anthropic.claude-opus-4-6-v1`
- `anthropic.claude-sonnet-4-5-20250929-v1:0`
- `amazon.nova-premier-v1:0`
- `amazon.nova-pro-v1:0`
- `meta.llama4-maverick-17b-instruct-v1:0`

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
