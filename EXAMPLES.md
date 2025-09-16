# TaskWing Usage Examples

💡 **Complete examples and workflows have moved to [docs.taskwing.app/docs/examples](https://docs.taskwing.app/docs/examples)**

## Quick Examples

```bash
# Basic workflow
taskwing add "Implement user auth" --priority high
taskwing start abc123
taskwing done abc123

# Planning flow
taskwing plan --task abc123           # Preview plan (subtasks)
taskwing plan --task abc123 --confirm # Create subtasks
taskwing iterate --task abc123 --step 1 --prompt "split into client/server" --split --confirm
taskwing search "authentication"      # Smart search

# AI task refinement
# Preview enhancements for an existing task
taskwing improve abc123
# Apply enhancements and generate a subtask plan
taskwing improve abc123 --apply --plan
```

## Documentation Links

- [**Workflow Examples**](https://docs.taskwing.app/docs/examples/workflows) - Real-world development workflows
- [**AI Integration Examples**](https://docs.taskwing.app/docs/examples/integrations) - AI assistant conversations
- [**Automation Patterns**](https://docs.taskwing.app/docs/examples/automation) - Streamline your workflow

## Interactive Tutorial

```bash
# Try the interactive tutorial
taskwing quickstart

# Or browse commands interactively
taskwing interactive
```

---

The documentation website provides copy-pasteable examples with explanations and context.
