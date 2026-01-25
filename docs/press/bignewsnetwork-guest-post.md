# AI coding assistants forget your architecture. Here’s how to stop paying that tax every day.

AI coding assistants are now part of everyday development workflows. Most developers already use them or plan to.
At the same time, trust in these tools is declining, largely due to inconsistent behavior on real-world, complex codebases.

The real cost isn’t incorrect code. It’s the _repeat work_ caused by a simple limitation:

**AI assistants don’t remember what your team already decided.**

---

## The hidden productivity problem

### What teams experience

- Each new session starts with re-explaining architecture, conventions, and constraints.
- Suggestions drift over time, even within the same codebase.
- Code reviews become noisy as AI-generated changes subtly violate local rules.
- Code that works in isolation breaks system-level assumptions.

### Why this happens

Most AI assistants operate with short-lived context. Even when they can read files, they don’t reliably retain:

- architectural decisions and rationale
- hard constraints that must be respected
- team-specific conventions and patterns

As a result, assistants improvise. Over time, that improvisation erodes consistency.

---

## A practical approach: persistent architectural memory

**TaskWing** is an open-source command-line tool designed to give AI assistants access to persistent architectural context.

It captures three categories of knowledge:

- **Decisions**: why specific approaches or technologies were chosen
- **Patterns**: conventions and structures intentionally repeated
- **Constraints**: rules that should never be violated

TaskWing is **local-first**. All data is stored in a local SQLite database rather than a cloud service. For many teams, this is critical: architectural knowledge and conventions are intellectual property and often cannot be sent outside the organization.

AI assistants access this information through **Model Context Protocol (MCP)**, a standard interface for connecting models to external tools and data sources.

---

## Security and trust considerations

Any system that allows AI assistants to access tools or data introduces risk. Tool access should always require deliberate user consent and cautious handling.

There have already been real-world security issues in MCP-based tooling that were resolved after disclosure, underscoring the importance of minimal surface area and careful integration.

TaskWing’s design choice is to keep memory local and require assistants to explicitly request context rather than exporting it automatically.

---

## What changes when assistants can ask “what are the rules here?”

When an assistant can query architectural context before generating code, consistency improves. Internal testing on real codebases showed fewer follow-up corrections and better adherence to established conventions compared to workflows without persistent context.

This aligns with broader industry signals: AI adoption is growing, but reliability and governance are now the deciding factors for long-term use. Tools that address consistency are what turn AI assistants from demos into dependable teammates.

---

## Getting started

```bash
brew install josephgoksu/tap/taskwing
```

or

```bash
curl -fsSL https://taskwing.app/install.sh | sh
```

Typical workflow:

1. Extract decisions, patterns, and constraints from a repository
2. Connect an AI assistant via MCP
3. Allow the assistant to query context before generating code

---

## Who it’s for

**Well suited for**

- teams with established architectural conventions
- larger or long-lived codebases
- privacy-sensitive or regulated environments

**Not a replacement for**

- clear documentation
- human review and judgment

---

## Open source

TaskWing is MIT-licensed, free to use, and works offline after installation.

- Website: [https://taskwing.app](https://taskwing.app)
- GitHub: [https://github.com/josephgoksu/TaskWing](https://github.com/josephgoksu/TaskWing)
- Connect: [https://josephgoksu.com/connect](https://josephgoksu.com/connect)
