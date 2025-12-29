# TaskWing: Engineering Intelligence

[![Go Report Card](https://goreportcard.com/badge/github.com/josephgoksu/TaskWing)](https://goreportcard.com/report/github.com/josephgoksu/TaskWing)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

> **Stop Answering the Same Questions.**
> Give your AI tools the context they need to understand *why* your code works, not just *how*.

## 🚀 The Value Proposition

As your codebase grows, context is lost. Decisions are forgotten. Onboarding takes weeks.
TaskWing scans your repository, extracts architectural intent, and serves it to your AI agents (Claude, Cursor, Copilot) via the Model Context Protocol (MCP).

**The Result**:
*   **Developers**: Get answers rooted in *your* architecture, not generic patterns.
*   **Leads**: Stop repeating "we use JWT here" for the 100th time.
*   **AI**: Stops hallucinating libraries you don't use.

## ⚡ Quick Start

```bash
# 1. Install (Mac/Linux)
curl -fsSL https://taskwing.app/install.sh | bash

# 2. Bootstrap your repo (Auto-extract knowledge)
cd /path/to/your/repo
tw bootstrap

# 3. Start the MCP Server (Connect to Claude/Cursor)
tw mcp
```

👉 **[Full Getting Started Guide](docs/development/GETTING_STARTED.md)**

## 📚 Knowledge Architecture

We organize documentation to be trusted, actionable, and scalable.

| Scope | Directory | Purpose |
|-------|-----------|---------|
| **The Constitution** | [`docs/architecture/`](docs/architecture/) | **Immutable Principles.** System design, data privacy, and roadmap. |
| **The Playbook** | [`docs/development/`](docs/development/) | **Developer Guide.** Internals, agent architecture, and testing. |
| **The Reference** | [`docs/reference/`](docs/reference/) | **Facts.** Telemetry policy, error codes, integrations. |

## 🏢 Enterprise & Teams

Using TaskWing in a team? We provide:
*   **Shared Knowledge Graph**: Sync context across your entire engineering org.
*   **Governance**: Enforce architectural constraints automatically.

[Contact Sales](mailto:enterprise@taskwing.app) for early access to TaskWing Cloud.

## License

MIT. Built for engineers, by engineers.
