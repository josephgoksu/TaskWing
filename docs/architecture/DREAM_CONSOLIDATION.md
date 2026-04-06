# Dream Consolidation: Passive Knowledge Growth

## How It Works

```mermaid
sequenceDiagram
    participant User
    participant Claude as AI Assistant
    participant Hook as session-end hook
    participant LLM as Fast LLM (cheap tier)
    participant KG as Knowledge Graph (SQLite)

    User->>Claude: Work on tasks in a session
    Claude->>Claude: Complete task 1, task 2, task 3...

    Note over User,Claude: Session ends (user closes, context full, or circuit breaker)

    Claude->>Hook: hook session-end fires
    Hook->>Hook: Load session state (tasks_completed > 0?)

    alt No tasks completed
        Hook->>Hook: Skip dream consolidation
    else Tasks were completed
        Hook->>Hook: Collect completion summaries from all completed tasks
        Hook->>LLM: "Extract architectural decisions, patterns, constraints from these completed tasks"
        LLM-->>Hook: JSON findings (type, title, description)

        loop For each finding
            Hook->>KG: IngestFindings(source_agent="dream", confidence=0.6)
            KG->>KG: Dedup against existing nodes (similarity check)
            KG->>KG: Store new nodes or skip duplicates
        end

        Hook-->>User: "Dream: extracted N knowledge items from session"
    end
```

## Knowledge Flow

```mermaid
flowchart TD
    subgraph "Manual (explicit)"
        B[tw bootstrap] --> |"Scan repo, call LLM agents"| KG[(Knowledge Graph)]
        R[/taskwing:remember/] --> |"User-initiated"| KG
    end

    subgraph "Automatic (passive)"
        SE[Session End Hook] --> |"Analyze completed tasks"| D[Dream Consolidation]
        D --> |"source_agent=dream\nconfidence=0.6"| KG
    end

    subgraph "Consumption"
        KG --> |"ask tool"| AI[AI Assistant Context]
        KG --> |"task enrichment"| TC[Task Context]
        KG --> |"FormatCompact()"| PC[Plan Context]
    end

    style D fill:#9b59b6,color:#fff
    style KG fill:#3498db,color:#fff
    style B fill:#2ecc71,color:#fff
```

## Confidence Tiers

| Source | Agent | Confidence | When |
|---|---|---|---|
| Bootstrap (LLM agents) | doc, code, deps, git | 0.8-1.0 | Explicit `tw bootstrap` run |
| User-initiated | remember | 1.0 | User calls `/taskwing:remember` |
| Dream consolidation | dream | 0.6 | Automatic at session end |

Dream findings have lower confidence because they're inferred from task completion summaries, not from direct code/doc evidence. They won't overwrite higher-confidence bootstrap findings during dedup (UpsertNodeBySummary preserves the higher-confidence version).

## What Gets Extracted

The dream prompt asks the LLM to identify:
- **Decisions**: Technology choices made during the session ("chose Redis over Memcached for caching")
- **Patterns**: Recurring approaches established ("all API handlers follow the middleware chain pattern")
- **Constraints**: Rules discovered or enforced ("never deploy without running the security scan")

Only findings that would be valuable for **future sessions** are extracted. Implementation details, debugging steps, and ephemeral work are filtered by the LLM prompt.
