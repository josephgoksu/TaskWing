# TaskWing MCP ↔ AI Tool Interaction Flow

How TaskWing exposes project knowledge and task lifecycle to AI coding assistants (Claude Code, Cursor, Copilot, etc.) via the Model Context Protocol.

## High-Level Architecture

```mermaid
flowchart LR
    subgraph "AI Tool (Claude, Cursor, Copilot)"
        AI[AI Assistant]
        SC[Slash Commands<br/>/taskwing:plan<br/>/taskwing:next<br/>/taskwing:ask]
    end

    subgraph "TaskWing MCP Server (stdio)"
        MCP[MCP Handler<br/>JSON-RPC]
        Tools[Tool Dispatch<br/>ask, task, plan,<br/>code, debug, remember]
    end

    subgraph "TaskWing Core"
        App[App Layer<br/>PlanApp, TaskApp, AskApp]
        KG[Knowledge Service]
        LLM[LLM Client<br/>OpenAI/Anthropic/Gemini]
    end

    subgraph "Local Storage"
        DB[(SQLite<br/>memory.db)]
        MD[Markdown<br/>ARCHITECTURE.md]
    end

    AI -->|invokes| SC
    SC -->|MCP call| MCP
    MCP -->|dispatch| Tools
    Tools -->|business logic| App
    App -->|search / store| KG
    App -->|enrich| LLM
    KG -->|read / write| DB
    KG -->|export| MD

    style AI fill:#3498db,color:#fff
    style MCP fill:#9b59b6,color:#fff
    style DB fill:#e67e22,color:#fff
```

## The Six MCP Tools

```mermaid
flowchart TD
    Root[MCP Server] --> Ask[ask<br/>Search knowledge]
    Root --> Task[task<br/>Lifecycle: next/current/start/complete]
    Root --> Plan[plan<br/>clarify/decompose/expand/generate/finalize]
    Root --> Code[code<br/>find/search/explain/callers/impact]
    Root --> Debug[debug<br/>Root-cause analysis]
    Root --> Remember[remember<br/>Store new knowledge]

    Ask -.reads.-> KG[(Knowledge Graph)]
    Task -.reads/writes.-> KG
    Plan -.reads/writes.-> KG
    Code -.reads.-> CI[(Code Intel Index)]
    Debug -.reads.-> KG
    Remember -.writes.-> KG

    style Ask fill:#2ecc71,color:#fff
    style Task fill:#2ecc71,color:#fff
    style Plan fill:#2ecc71,color:#fff
    style Code fill:#f39c12,color:#fff
    style Debug fill:#f39c12,color:#fff
    style Remember fill:#e74c3c,color:#fff
```

- **Green**: Knowledge graph operations (read/write nodes)
- **Orange**: Code intelligence (symbols, callers, impact)
- **Red**: Mutation-only (write knowledge)

## Plan Creation Flow (Standard)

```mermaid
sequenceDiagram
    actor User
    participant AI as AI Assistant
    participant MCP as TaskWing MCP
    participant KG as Knowledge Graph
    participant LLM

    User->>AI: /taskwing:plan "Add Stripe billing"
    AI->>MCP: plan(action=clarify, goal=...)
    MCP->>KG: search relevant constraints, decisions
    KG-->>MCP: project context
    MCP->>LLM: clarify goal with context
    LLM-->>MCP: clarifying questions + enriched_goal
    MCP-->>AI: questions + session_id

    AI->>User: Present decisions
    User->>AI: Answer decisions

    AI->>MCP: plan(action=clarify, answers)
    MCP->>LLM: refine enriched_goal
    LLM-->>MCP: is_ready_to_plan=true
    MCP-->>AI: Ready to generate

    User->>AI: Approve
    AI->>MCP: plan(action=generate, enriched_goal, session_id)
    MCP->>LLM: decompose into tasks with context
    LLM-->>MCP: task list
    MCP->>KG: store plan + tasks
    MCP-->>AI: plan_id + tasks

    AI->>User: Show plan summary
```

## Plan Creation Flow (Passthrough)

```mermaid
sequenceDiagram
    actor User
    participant AI as AI Assistant
    participant MCP as TaskWing MCP
    participant KG as Knowledge Graph

    Note over User,AI: User already has a detailed plan (e.g., from markdown doc)

    AI->>MCP: plan(action=generate,<br/>tasks=[{title, description,<br/>acceptance_criteria, ...}])

    Note over MCP: Passthrough detected (tasks array provided)<br/>Skips clarify, LLM rewrite, verifier

    MCP->>KG: store plan + tasks verbatim
    KG-->>MCP: plan_id
    MCP-->>AI: plan created

    Note over AI: Tasks stored exactly as user wrote them<br/>No acceptance criteria rewriting<br/>No path correction
```

## Task Execution Loop

```mermaid
flowchart TD
    Start([Session Start]) --> GetNext["/taskwing:next<br/>task action=next"]
    GetNext --> FetchContext[Fetch scope-aware context<br/>via ask queries]
    FetchContext --> ShowBrief[Show task brief:<br/>description + acceptance criteria]
    ShowBrief --> ImplGate{User<br/>approves?}

    ImplGate -->|approve| Implement[AI implements task]
    ImplGate -->|skip| GetNext

    Implement --> Done["/taskwing:done<br/>task action=complete"]
    Done --> UpdateDB[Update task status<br/>store completion summary]
    UpdateDB --> More{More pending<br/>tasks?}

    More -->|yes| GetNext
    More -->|no| SessionEnd[Session end hook fires]

    SessionEnd --> DreamCheck{Any tasks<br/>completed?}
    DreamCheck -->|no| End([Session complete])
    DreamCheck -->|yes| Dream[Dream consolidation:<br/>LLM analyzes completion summaries]
    Dream --> StoreNodes[Store new knowledge<br/>source_agent=dream<br/>confidence=0.6]
    StoreNodes --> End

    style Start fill:#3498db,color:#fff
    style End fill:#3498db,color:#fff
    style Dream fill:#9b59b6,color:#fff
    style StoreNodes fill:#9b59b6,color:#fff
```

## Ask Tool Flow (Knowledge Search)

```mermaid
flowchart TD
    Start[AI calls ask] --> Check{Query type?}

    Check -->|"all=true"| Dump[Direct SQLite dump<br/>No LLM call]
    Check -->|query string| Hybrid[Hybrid search]

    Hybrid --> Embed[Generate query embedding]
    Embed --> Parallel{Parallel search}

    Parallel --> Semantic[Semantic search<br/>cosine similarity]
    Parallel --> FTS[FTS5 full-text]
    Parallel --> Graph[Graph expansion<br/>edges]

    Semantic --> Merge[Rerank + dedupe]
    FTS --> Merge
    Graph --> Merge

    Merge --> Filter[Filter by workspace]
    Filter --> Format[Format for AI context]
    Format --> Return[Return to AI tool]

    Dump --> Return

    style Dump fill:#2ecc71,color:#fff
    style Return fill:#3498db,color:#fff
```

## Knowledge Growth Paths

```mermaid
flowchart LR
    subgraph "Explicit (User-initiated)"
        B[tw bootstrap] --> Scan[Scan repo files]
        Scan --> Agents[LLM agents:<br/>doc, code, deps, git]
        Agents --> Findings1[Findings<br/>confidence=0.8-1.0]

        R["/taskwing:remember"] --> UserFind[User-curated node<br/>confidence=1.0]
    end

    subgraph "Passive (Automatic)"
        SE[Session end hook] --> Check{tasks completed > 0?}
        Check -->|yes| Dream[LLM analyzes<br/>completion summaries]
        Dream --> Findings2[Dream findings<br/>confidence=0.6]
        Check -->|no| Skip[Skip]
    end

    Findings1 --> Ingest[Ingest pipeline]
    UserFind --> Ingest
    Findings2 --> Ingest

    Ingest --> Verify[Verify evidence]
    Verify --> Dedup{Dedup check}

    Dedup --> Exact[Exact summary match]
    Dedup --> Jaccard[Jaccard text<br/>threshold 0.45]
    Dedup --> Cosine[Embedding cosine<br/>threshold 0.85]

    Exact --> Merge[Merge or insert]
    Jaccard --> Merge
    Cosine --> Merge

    Merge --> DB[(SQLite)]

    style Dream fill:#9b59b6,color:#fff
    style DB fill:#e67e22,color:#fff
```

## Context Binding (Early + Late)

```mermaid
sequenceDiagram
    participant Plan as Plan Generation
    participant Enricher as TaskEnricher
    participant KG as Knowledge Graph
    participant Task as Task Storage
    participant Display as Task Display

    Note over Plan,Task: Early Binding (at creation)

    Plan->>Enricher: enrich task with scope + queries
    Enricher->>KG: ask(scope="auth", queries=[...])
    KG-->>Enricher: scope-relevant nodes + constraints
    Enricher-->>Plan: ContextSummary
    Plan->>Task: store task with ContextSummary

    Note over Display,KG: Late Binding (at display)

    Display->>Task: get task
    Task-->>Display: task + ContextSummary
    alt ContextSummary exists
        Display->>Display: FormatRichContext() uses embedded
    else ContextSummary empty
        Display->>KG: fresh fetch fallback
        KG-->>Display: current context
    end

    Display-->>Display: render with budget<br/>(scales by model capacity)
```

## Key Design Principles

1. **Local-first**: All knowledge lives in local SQLite. MCP transport is stdio (no network).
2. **Trust user input**: Passthrough mode preserves user-provided tasks verbatim. No LLM rewriting unless requested.
3. **Hybrid dedup**: Three-layer match (exact → Jaccard → embedding cosine) prevents duplicates across LLM re-runs.
4. **Budget-aware**: Context limits scale with the model's context window (8K to 200K+ tiers).
5. **Passive knowledge growth**: Dream consolidation extracts knowledge from completed work without manual bootstrap.
6. **Workspace scoping**: Partial bootstraps only affect their own workspace, never destroy other repos' nodes.

## Related Documents

- [Dream Consolidation](./DREAM_CONSOLIDATION.md) - Passive knowledge extraction from session end
- [Command Risk Classification](./ADR_COMMAND_RISK_CLASSIFICATION.md) - Future MCP tool safety tiers
- [Context Binding](./ADR_CONTEXT_BINDING.md) - Early + late binding rationale
