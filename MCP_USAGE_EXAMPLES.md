# TaskWing Enhanced MCP Tools - Usage Examples

This guide provides practical examples of using the enhanced MCP tools to improve AI assistant interactions with TaskWing.

## 🔍 Enhanced Filtering Examples

### Natural Language Queries
The `filter-tasks-enhanced` tool supports human-readable queries:

```json
// Find high priority unfinished work
{
  "tool": "filter-tasks-enhanced",
  "arguments": {
    "query": "high priority unfinished tasks"
  }
}

// Find tasks currently being worked on
{
  "tool": "filter-tasks-enhanced", 
  "arguments": {
    "query": "tasks in progress"
  }
}

// Find urgent items that need attention
{
  "tool": "filter-tasks-enhanced",
  "arguments": {
    "query": "urgent tasks needing review"
  }
}
```

### Complex Logical Expressions
```json
// Multiple conditions with AND/OR logic
{
  "tool": "filter-tasks-enhanced",
  "arguments": {
    "expression": "status=todo AND priority=high",
    "limit": 10
  }
}

// Fuzzy text search across fields
{
  "tool": "filter-tasks-enhanced",
  "arguments": {
    "filter": "title~=authentication",
    "fuzzy_match": true,
    "include_score": true
  }
}
```

### Error Recovery Example
```json
// Invalid query with helpful response
{
  "tool": "filter-tasks-enhanced",
  "arguments": {
    "filter": "stat=todo"  // Typo in field name
  }
}

// Response includes suggestions:
{
  "error": {
    "code": "FILTER_ERROR",
    "message": "Filter execution failed: unknown field 'stat'",
    "details": {
      "suggestions": [
        "Valid fields: status, priority, title, description, id",
        "Did you mean 'status'?"
      ]
    }
  }
}
```

## 🎯 Smart Task Resolution Examples

### Partial ID Resolution
```json
// Find task using partial UUID
{
  "tool": "resolve-task-enhanced",
  "arguments": {
    "reference": "7b3e4f2a"  // Just the first 8 characters
  }
}

// Response with high confidence match:
{
  "resolved": true,
  "match": {
    "task": {
      "id": "7b3e4f2a-8c9d-4e5f-b0a1-2c3d4e5f6a7b",
      "title": "Implement user authentication"
    },
    "score": 0.95,
    "type": "partial_id"
  },
  "message": "✓ High confidence match: Implement user authentication (95% confidence, matched by partial_id)"
}
```

### Fuzzy Title Matching
```json
// Find task with typos in title
{
  "tool": "resolve-task-enhanced",
  "arguments": {
    "reference": "implment authen",  // Typos and partial words
    "prefer_current": true
  }
}

// Response with suggestions:
{
  "resolved": false,
  "matches": [
    {
      "task": {
        "title": "Implement user authentication",
        "id": "7b3e4f2a-8c9d-4e5f-b0a1-2c3d4e5f6a7b"
      },
      "score": 0.75,
      "type": "title"
    }
  ],
  "message": "Found 1 potential matches for 'implment authen'. Top match: Implement user authentication (75% confidence)"
}
```

### Context-Aware Resolution
```json
// Prioritize current task and related items
{
  "tool": "resolve-task-enhanced",
  "arguments": {
    "reference": "auth",
    "prefer_current": true,
    "max_suggestions": 3
  }
}
```

## ⚡ Smart Autocomplete Examples

### Context-Aware Suggestions
```json
// Get suggestions for active work context
{
  "tool": "task-autocomplete-smart",
  "arguments": {
    "input": "impl",
    "context": "current",
    "limit": 5
  }
}

// Response prioritizes related tasks:
{
  "suggestions": [
    {
      "task": {
        "title": "Implement user authentication",
        "status": "todo"
      },
      "score": 0.92,
      "type": "title"
    },
    {
      "task": {
        "title": "Implement password reset",
        "status": "doing"
      },
      "score": 0.88,
      "type": "title"
    }
  ],
  "count": 2,
  "message": "Found 2 autocomplete suggestions for 'impl'. Top suggestion: Implement user authentication (92% match) (context: current)"
}
```

### Priority-Based Suggestions
```json
// Get suggestions focused on urgent work
{
  "tool": "task-autocomplete-smart",
  "arguments": {
    "input": "fix",
    "context": "priority"
  }
}
```

## 🛠️ Error Handling Examples

### Task Not Found with Suggestions
```json
// Attempting to get non-existent task
{
  "tool": "get-task",
  "arguments": {
    "id": "invalid-uuid"
  }
}

// Enhanced error response:
{
  "error": {
    "code": "TASK_NOT_FOUND", 
    "message": "Task invalid-uuid not found",
    "details": {
      "suggestions": [
        "Use 'list-tasks' to see all available tasks",
        "Try 'resolve-task-enhanced' with partial ID or title for fuzzy matching",
        "Check if the task was deleted or moved"
      ],
      "help_commands": [
        "list-tasks",
        "resolve-task-enhanced",
        "task-autocomplete-smart"
      ],
      "tip": "💡 Use fuzzy search tools like 'resolve-task-enhanced' for better task discovery"
    }
  }
}
```

### Validation Errors with Corrections
```json
// Creating task with invalid priority
{
  "tool": "add-task",
  "arguments": {
    "title": "New task",
    "priority": "hi"  // Invalid priority
  }
}

// Helpful validation error:
{
  "error": {
    "code": "INVALID_PRIORITY",
    "message": "Invalid priority value",
    "details": {
      "field": "priority",
      "value": "hi",
      "valid_values": ["low", "medium", "high", "urgent"],
      "suggestions": ["Did you mean 'high'?"],
      "tip": "💡 Check field formats: status (todo/doing/review/done), priority (low/medium/high/urgent)"
    }
  }
}
```

## 🔄 Migration from Basic Tools

### Filtering Migration
```json
// Old way (basic tool):
{
  "tool": "filter-tasks",
  "arguments": {
    "filter": "$.status == \"todo\""
  }
}

// New way (enhanced tool):
{
  "tool": "filter-tasks-enhanced", 
  "arguments": {
    "query": "unfinished tasks"
  }
}
```

### Resolution Migration
```json
// Old way (basic tool):
{
  "tool": "resolve-task-reference",
  "arguments": {
    "reference": "7b3e4f2a",
    "exact": false
  }
}

// New way (enhanced tool):
{
  "tool": "resolve-task-enhanced",
  "arguments": {
    "reference": "7b3e4f2a",
    "prefer_current": true,
    "fields": ["id", "title", "description"]
  }
}
```

## 📊 Response Insights

### Understanding Match Quality
```json
// Enhanced responses include metadata:
{
  "tasks": [...],
  "count": 5,
  "matched_fields": {
    "title": ["authentication", "login"],
    "priority": ["high"]
  },
  "query_type": "natural",
  "execution_time_ms": 15
}
```

### Using Suggestions
```json
// When no results found:
{
  "tasks": [],
  "count": 0,
  "suggestions": [
    "Try: status=todo",
    "Try: priority=high", 
    "high priority tasks"
  ],
  "query_type": "simple"
}
```

## 🎛️ Advanced Configuration

### Field-Specific Searches
```json
{
  "tool": "resolve-task-enhanced",
  "arguments": {
    "reference": "database migration",
    "fields": ["description"],  // Only search descriptions
    "max_suggestions": 10
  }
}
```

### Performance Optimization
```json
{
  "tool": "filter-tasks-enhanced",
  "arguments": {
    "query": "high priority tasks",
    "limit": 20,  // Limit results for performance
    "fields": "id,title,status"  // Only return needed fields
  }
}
```

## 🔍 Debugging and Monitoring

### Execution Time Tracking
All enhanced tools include execution time in responses:
```json
{
  "execution_time_ms": 25,
  "query_type": "natural",
  "filter_used": "high priority unfinished tasks"
}
```

### Score Analysis
Understanding match confidence:
- `1.0`: Exact match
- `0.9-0.99`: Very high confidence
- `0.7-0.89`: High confidence  
- `0.5-0.69`: Medium confidence
- `0.3-0.49`: Low confidence
- `<0.3`: Very low confidence (filtered out)

## 💡 Best Practices

### 1. Use Natural Language When Possible
```json
// Preferred - more intuitive for AI
{"query": "urgent tasks needing review"}

// vs basic syntax
{"expression": "priority=urgent AND status=review"}
```

### 2. Leverage Context Awareness
```json
// Use current task context for related work
{"reference": "auth", "prefer_current": true}
```

### 3. Handle Errors Gracefully
```javascript
// Example error handling in AI tools
try {
  const result = await callMCPTool("filter-tasks-enhanced", params);
  return result;
} catch (error) {
  if (error.details?.suggestions) {
    // Use suggestions to retry with better parameters
    return suggestAlternatives(error.details.suggestions);
  }
  throw error;
}
```

### 4. Optimize for Performance
```json
// Limit results for better performance
{
  "limit": 50,
  "fields": "id,title,status"  // Only essential fields
}
```

This enhanced MCP implementation makes TaskWing significantly more AI-friendly, reducing errors and improving the user experience for both human users and AI assistants.