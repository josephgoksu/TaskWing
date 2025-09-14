# TaskWing Archive System Design

## Overview

The TaskWing Archive System provides a mechanism to preserve completed tasks with enriched metadata, capture lessons learned, and maintain a searchable knowledge base for improving future task management and AI assistance.

### Final Decisions
- Backend: file-based JSON with a lightweight `index.json` (no external DB dependencies). This aligns with TaskWing's local-first design and current build constraints.
- Search: simple case-insensitive substring matching over title, description, and lessons learned; metadata filters for date range and tags; assignees optional.
- Layout: year/month directories under `.taskwing/archive/` with one JSON entry per archived task and a global `index.json` for listings.
- Forward compatibility: structure is compatible with future SQLite+FTS5 migration without changing CLI contracts.

## Architecture

### Storage Structure

```
.taskwing/
├── tasks/              # Active tasks
│   └── tasks.json
├── archive/            # Archived tasks
│   ├── index.json      # Archive index for fast searching
│   ├── 2025/           # Year-based organization
│   │   ├── 01/         # Month-based subdirectories
│   │   │   ├── 2025-01-19_project-name.json
│   │   │   └── 2025-01-19_project-name.md
│   │   └── ...
│   └── patterns.json   # Extracted task patterns
└── knowledge/          # Knowledge base
    ├── KNOWLEDGE.md    # Main knowledge document
    ├── retrospectives/ # Project retrospectives
    └── decisions/      # Architectural decisions
```

## Archive Format Specification

### JSON Archive Format

```json
{
  "version": "1.0",
  "archived_at": "2025-01-19T10:30:00Z",
  "archived_by": "user@example.com",
  "project": {
    "name": "Documentation Unification",
    "description": "Simplify and unify documentation across repo",
    "duration_days": 1,
    "health": "excellent"
  },
  "tasks": [
    {
      "id": "uuid",
      "title": "Task Title",
      "description": "Task Description",
      "status": "completed",
      "priority": "high",
      "created_at": "timestamp",
      "completed_at": "timestamp",
      "actual_duration_hours": 2.5,
      "acceptance_criteria": "...",
      "outcome": {
        "success": true,
        "notes": "What was actually delivered",
        "metrics": {
          "files_changed": 10,
          "lines_added": 500,
          "lines_removed": 300
        }
      },
      "lessons_learned": [
        "What worked well",
        "What could be improved"
      ],
      "dependencies": ["task-id-1", "task-id-2"],
      "subtasks": ["task-id-3", "task-id-4"],
      "tags": ["documentation", "refactoring"]
    }
  ],
  "retrospective": {
    "what_went_well": ["List of successes"],
    "what_went_wrong": ["List of challenges"],
    "action_items": ["Future improvements"],
    "key_decisions": [
      {
        "decision": "Use 5-file structure",
        "rationale": "Reduces cognitive load",
        "outcome": "Successful"
      }
    ]
  },
  "patterns_identified": [
    {
      "pattern": "Documentation Consolidation",
      "trigger": "Multiple overlapping docs",
      "approach": "Audit, Design, Consolidate, Clean",
      "success_factors": ["Clear structure", "Progressive consolidation"]
    }
  ],
  "metrics": {
    "total_tasks": 7,
    "completion_rate": 100,
    "average_task_duration_hours": 1.5,
    "blockers_encountered": 1,
    "rework_required": 0
  }
}
```

### Archive Index Format

```json
{
  "archives": [
    {
      "id": "archive-uuid",
      "date": "2025-01-19",
      "project_name": "Documentation Unification",
      "file_path": "2025/01/2025-01-19_documentation-unification.json",
      "task_count": 7,
      "tags": ["documentation", "refactoring"],
      "summary": "Unified 8 docs into 5 focused guides"
    }
  ],
  "statistics": {
    "total_archives": 1,
    "total_tasks_archived": 7,
    "most_common_patterns": ["consolidation", "refactoring"],
    "average_project_duration_days": 1
  }
}
```

## Archive Command Specification

### Command Interface

```bash
# Archive all completed tasks
taskwing archive

# Archive with specific project name
taskwing archive --project "Documentation Unification"

# Archive with retrospective prompts
taskwing archive --retrospective

# Archive and generate patterns
taskwing archive --extract-patterns

# View archive history
taskwing archive list

# Search archives
taskwing archive search "documentation"

# Restore archived tasks
taskwing archive restore <archive-id>
```

## Migration Plan

### From legacy tasks.json to archive

Goal: Move completed tasks from `.taskwing/tasks/tasks.json` into `.taskwing/archive/` with minimal user friction.

Pseudocode:

```
read tasks.json -> TaskList
for each task where status == done:
  build ArchiveEntry {task fields + archived_at = now}
  write .taskwing/archive/YYYY/MM/<date>_<slug>.json
  append to index.json (id, date, title, tags, path, task_count=1, summary)
```

Notes:
- Keep tasks in the active list by default; restoration is supported via `archive restore`.
- Checksums are not required for archive files; `index.json` is the source of truth for listings.
- Future migration to SQLite+FTS5 can ingest the same JSON; maintain `index.json` during transition.

### Interactive Archive Flow

1. **Pre-archive Summary**
   - Show tasks to be archived
   - Display project metrics
   - Confirm archive action

2. **Retrospective Collection** (optional)
   - Prompt: "What went well?"
   - Prompt: "What challenges did you face?"
   - Prompt: "Key decisions made?"
   - Prompt: "Lessons learned?"

3. **Outcome Documentation**
   - For each task: "Describe the actual outcome"
   - For each task: "Any metrics to capture?"

4. **Pattern Extraction**
   - Identify common task structures
   - Extract successful approaches
   - Note reusable workflows

5. **Archive Creation**
   - Generate JSON archive entry per task
   - Update archive index
   - Optionally clear completed tasks (future flag)
   - Generate summary report

## Knowledge Base Integration

### KNOWLEDGE.md Structure

```markdown
# TaskWing Knowledge Base

## Projects Completed

### Documentation Unification (2025-01-19)
- **Archive**: [2025-01-19_documentation-unification.json](archive/2025/01/...)
- **Summary**: Reduced 8 docs to 5 focused guides
- **Key Learning**: Progressive consolidation works best
- **Pattern**: Audit → Design → Implement → Clean

## Common Patterns

### Pattern: Documentation Consolidation
- **When to use**: Multiple overlapping documents
- **Approach**: 
  1. Audit existing docs
  2. Design new structure
  3. Consolidate progressively
  4. Clean up redundant files
- **Success Rate**: 100% (1/1 projects)

## Decision Log

### 2025-01-19: Use separate MCP.md
- **Context**: MCP documentation was fragmented
- **Decision**: Keep MCP.md as dedicated file
- **Rationale**: Complex topic deserves focused documentation
- **Outcome**: ✅ Successful - improved clarity

## Metrics Dashboard

- **Total Projects**: 1
- **Total Tasks Completed**: 7
- **Average Completion Rate**: 100%
- **Most Successful Pattern**: Progressive Consolidation
```

## MCP Resource Specification

### New MCP Resource: taskwing://archive

```go
// Returns historical task data
type ArchiveResource struct {
    Archives []Archive `json:"archives"`
    Patterns []Pattern `json:"patterns"`
    Metrics  Metrics   `json:"metrics"`
}

// Query parameters
// ?project=name - Filter by project name
// ?date_from=2025-01-01 - Filter by date range
// ?pattern=consolidation - Filter by pattern type
// ?tag=documentation - Filter by tags
```

### AI Context Enhancement

When AI tools query the archive resource, they receive:

1. **Similar Past Tasks**: Tasks that match current work
2. **Successful Patterns**: Proven approaches for similar work
3. **Historical Metrics**: Time estimates based on past performance
4. **Lessons Learned**: Relevant insights from previous projects
5. **Decision History**: Past decisions and their outcomes

## Pattern Library Specification

### Pattern Format

```json
{
  "pattern_id": "uuid",
  "name": "Documentation Consolidation",
  "category": "refactoring",
  "description": "Consolidating multiple overlapping documents",
  "when_to_use": [
    "Multiple files with redundant content",
    "User confusion about where to find information",
    "Maintenance burden from keeping multiple docs in sync"
  ],
  "task_breakdown": [
    {
      "phase": "Audit",
      "tasks": ["Inventory files", "Identify overlaps", "Assess quality"],
      "typical_duration_hours": 1
    },
    {
      "phase": "Design",
      "tasks": ["Define new structure", "Plan content mapping"],
      "typical_duration_hours": 0.5
    },
    {
      "phase": "Implementation",
      "tasks": ["Create new files", "Migrate content", "Update references"],
      "typical_duration_hours": 3
    },
    {
      "phase": "Cleanup",
      "tasks": ["Remove old files", "Verify links", "Update indexes"],
      "typical_duration_hours": 0.5
    }
  ],
  "success_factors": [
    "Start with comprehensive audit",
    "Design before implementing",
    "Progressive consolidation",
    "Immediate testing of changes"
  ],
  "common_pitfalls": [
    "Losing valuable content during consolidation",
    "Breaking existing references",
    "Inconsistent formatting"
  ],
  "examples": [
    {
      "project": "Documentation Unification",
      "date": "2025-01-19",
      "outcome": "successful",
      "archive_ref": "2025/01/2025-01-19_documentation-unification.json"
    }
  ],
  "metrics": {
    "usage_count": 1,
    "success_rate": 100,
    "average_duration_hours": 5,
    "rework_rate": 0
  }
}
```

## Implementation Plan

### Phase 1: Core Archive System
1. Create archive directory structure
2. Implement `taskwing archive` command
3. Build JSON serialization/deserialization
4. Create archive index management

### Phase 2: Knowledge Base
1. Generate KNOWLEDGE.md template
2. Implement retrospective collection
3. Build pattern extraction logic
4. Create metrics aggregation

### Phase 3: MCP Integration
1. Add taskwing://archive resource
2. Implement archive querying
3. Build pattern matching for AI
4. Add historical context to responses

### Phase 4: Advanced Features
1. Archive search and filtering
2. Pattern library management
3. Metrics dashboard
4. Archive restoration/viewing

## Benefits

### For Users
- Preserve project history and outcomes
- Learn from past experiences
- Track productivity metrics
- Build organizational knowledge

### For AI Assistance
- Access to historical context
- Pattern-based suggestions
- Accurate time estimates
- Improved task breakdowns
- Learn from past decisions

### For Teams
- Shared knowledge base
- Consistent patterns
- Best practices documentation
- Decision history tracking

## Security & Privacy Considerations

1. **Sensitive Data**: Archive command should prompt before archiving sensitive tasks
2. **Personal Information**: Option to anonymize user data in archives
3. **Access Control**: Archive files respect same permissions as task files
4. **Export Control**: Option to exclude certain fields during archive

## Future Enhancements

1. **Cloud Sync**: Optional cloud backup for archives
2. **Team Analytics**: Aggregate metrics across team members
3. **AI Training**: Use archives to fine-tune AI suggestions
4. **Visualization**: Generate charts and graphs from archive data
5. **Integration**: Export to external knowledge management systems
