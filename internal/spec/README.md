# Package: spec

Spec and task storage for feature specifications.

## Purpose

Persists feature specifications and development tasks to disk using JSON+Markdown format. Supports CRUD operations for specs and tasks.

## Storage Layout

```
.taskwing/specs/
├── add-stripe-integration/
│   ├── spec.json     # Full spec data
│   ├── spec.md       # Human-readable markdown
│   └── tasks.json    # Task list (if present)
└── fix-login-bug/
    ├── spec.json
    └── spec.md
```

## Key Components

| Type | Purpose |
|------|---------|
| `Store` | File-based storage operations |
| `Spec` | Full feature specification with agent analyses |
| `Task` | Individual development task |
| `SpecSummary` / `TaskSummary` | Lightweight views for listings |
| `TaskContext` | Full context for AI tools |

## Usage

```go
import "github.com/josephgoksu/TaskWing/internal/spec"

// Create store
store, err := spec.NewStore("/path/to/project")

// Create spec
s, err := store.CreateSpec("Add Stripe", "Payment processing")

// List specs
specs, err := store.ListSpecs()

// Get tasks
tasks, err := store.ListTasks("") // all tasks
tasks, err := store.ListTasks("add-stripe") // by spec

// Update task status
err = store.UpdateTaskStatus("task-abc", spec.StatusInProgress)

// Get full context for AI
ctx, err := store.GetTaskContext("task-abc")
```

## Status Values

| Status | Meaning |
|--------|---------|
| `draft` | Initial state |
| `approved` | Approved for implementation |
| `in-progress` | Currently being worked on |
| `done` | Completed |
