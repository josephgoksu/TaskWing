# Phase 5: SEO Alignment

**Date:** 2026-03-14
**Status:** Complete
**Depends on:** Phase 3 (copy recommendations)

## Target Keyword Clusters

### Primary (high intent, emerging volume)

| Keyword | Intent | Competition | Notes |
|---------|--------|-------------|-------|
| "local-first AI coding tool" | Discovery | Low | Emerging category. TaskWing can own this. |
| "private AI code assistant" | Discovery | Medium | Cline and Continue.dev compete here. |
| "AI coding tool that keeps code local" | Discovery | Low | Long-tail, high intent. |
| "MCP server for AI coding" | Technical | Low | TaskWing is one of few standalone MCP servers. |
| "air-gapped AI development" | Enterprise | Low | Niche but high-value. Gov/defense audience. |

### Secondary (established volume, higher competition)

| Keyword | Intent | Competition | Notes |
|---------|--------|-------------|-------|
| "AI code context" / "AI coding context" | Discovery | Medium | Broad but relevant. |
| "cursor alternative privacy" | Comparison | Medium | Competitor-adjacent. |
| "copilot alternative open source" | Comparison | High | High volume, competitive. |
| "self-hosted AI code assistant" | Discovery | Medium | Continue.dev and Cline rank here. |
| "AI architecture extraction" | Technical | Low | Unique to TaskWing. Own this entirely. |

### Long-tail (low volume, near-zero competition)

| Keyword | Intent | Notes |
|---------|--------|-------|
| "extract architecture decisions from codebase" | Problem-aware | Unique value prop. Blog post target. |
| "persistent context for AI coding tools" | Problem-aware | Directly addresses the pain. |
| "SQLite AI knowledge base" | Technical | Differentiator. |
| "MCP protocol AI development" | Technical | Growing with MCP adoption. |
| "ollama air-gapped coding" | Enterprise | Niche but zero competition. |

## Metadata Recommendations

### README / GitHub
- **Repository description**: "Local-first AI knowledge layer. Extract architecture, query from any AI tool via MCP. Private by architecture."
- **Topics/tags**: `ai`, `mcp`, `local-first`, `developer-tools`, `architecture`, `sqlite`, `cli`, `privacy`, `ollama`, `code-intelligence`

### Landing Page (taskwing.app)

**Title tag**: "TaskWing -- Local-First AI Knowledge Layer for Development"
**Meta description**: "Extract your codebase architecture into a local SQLite database. Give Claude, Cursor, Copilot, and Gemini instant context via MCP -- without your code leaving your machine."

### Structured Data (JSON-LD)

```json
{
  "@context": "https://schema.org",
  "@type": "SoftwareApplication",
  "name": "TaskWing",
  "applicationCategory": "DeveloperApplication",
  "operatingSystem": "macOS, Linux",
  "offers": {
    "@type": "Offer",
    "price": "0",
    "priceCurrency": "USD"
  },
  "description": "Local-first AI knowledge layer. Extract architecture from codebases, store in local SQLite, query from any AI tool via MCP.",
  "keywords": "local-first, AI coding, MCP, architecture extraction, private, SQLite"
}
```

## Content Strategy (blog post targets)

| Title | Target Keyword | Type |
|-------|---------------|------|
| "Why your AI coding tool shouldn't need your codebase in the cloud" | private AI code assistant | Thought leadership |
| "How TaskWing extracts 170+ architecture decisions from any codebase" | AI architecture extraction | Product deep-dive |
| "Local-first AI development: what it means and why it matters" | local-first AI coding tool | Category definition |
| "Air-gapped AI coding with Ollama and TaskWing" | air-gapped AI development | Tutorial |
| "Cursor vs TaskWing: cloud context vs local knowledge" | cursor alternative privacy | Comparison |
| "The MCP protocol: how AI tools share context locally" | MCP server AI coding | Technical explainer |

## Priority Actions

1. **Immediate**: Update GitHub repo description and topics
2. **This week**: Update landing page title/meta with sovereignty keywords
3. **This month**: Publish "Why your AI coding tool shouldn't need your codebase in the cloud" blog post
4. **Ongoing**: Each blog post targets one keyword cluster from above
