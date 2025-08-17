# TaskWing MCP Subtask Testing Guide

This document contains MCP commands you can copy/paste to test subtask creation functionality.

## Prerequisites

1. Start the TaskWing MCP server:

   ```bash
   ./taskwing mcp
   ```

2. In another terminal, you can test these MCP commands by pasting them into the MCP client.

## Test 1: List Existing Tasks

First, let's see what tasks we have:

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "list-tasks",
    "arguments": {}
  }
}
```

## Test 2: Create a Parent Task

Let's create a main task that we'll add subtasks to:

```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "method": "tools/call",
  "params": {
    "name": "add-task",
    "arguments": {
      "title": "Build Chrome Extension",
      "description": "Create a complete Chrome extension for bookmark management",
      "priority": "high",
      "acceptanceCriteria": "- Extension is installable\n- Core functionality works\n- User interface is complete"
    }
  }
}
```

## Test 3: Get the Parent Task ID

After creating the parent task, copy its UUID from the response. You'll need this for the next test.

## Test 4: Test Placeholder ID Validation (Should Fail)

This should fail with a helpful error message:

```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "method": "tools/call",
  "params": {
    "name": "batch-create-tasks",
    "arguments": {
      "tasks": [
        {
          "title": "Test with Placeholder",
          "description": "This should fail with placeholder ID",
          "priority": "medium",
          "parentId": "task_1"
        }
      ]
    }
  }
}
```

## Test 5: Create Subtasks with Real UUID (Should Work)

Replace `PARENT_TASK_UUID_HERE` with the actual UUID from Test 2:

```json
{
  "jsonrpc": "2.0",
  "id": 4,
  "method": "tools/call",
  "params": {
    "name": "batch-create-tasks",
    "arguments": {
      "tasks": [
        {
          "title": "Design Extension Architecture",
          "description": "Plan the overall structure and components of the Chrome extension",
          "priority": "high",
          "parentId": "PARENT_TASK_UUID_HERE",
          "acceptanceCriteria": "- Architecture diagram created\n- Component responsibilities defined\n- Data flow documented"
        },
        {
          "title": "Implement Content Script",
          "description": "Create the content script for bookmark capture functionality",
          "priority": "high",
          "parentId": "PARENT_TASK_UUID_HERE",
          "acceptanceCriteria": "- Content script captures page data\n- Bookmarks can be saved\n- No conflicts with existing page scripts"
        },
        {
          "title": "Build Popup Interface",
          "description": "Create the extension popup with bookmark management features",
          "priority": "medium",
          "parentId": "PARENT_TASK_UUID_HERE",
          "acceptanceCriteria": "- Popup displays correctly\n- Bookmark list is functional\n- Search and filter work"
        }
      ]
    }
  }
}
```

## Test 6: Verify Parent-Child Relationships

List tasks filtering by parent to see the subtasks:

```json
{
  "jsonrpc": "2.0",
  "id": 5,
  "method": "tools/call",
  "params": {
    "name": "list-tasks",
    "arguments": {
      "parentId": "PARENT_TASK_UUID_HERE"
    }
  }
}
```

## Test 7: Get Parent Task Details

Verify the parent task shows its subtasks:

```json
{
  "jsonrpc": "2.0",
  "id": 6,
  "method": "tools/call",
  "params": {
    "name": "get-task",
    "arguments": {
      "id": "PARENT_TASK_UUID_HERE"
    }
  }
}
```

## Expected Results

- **Test 1**: Should list existing tasks with their UUIDs
- **Test 2**: Should create a parent task and return its UUID
- **Test 4**: Should fail with message about placeholder IDs
- **Test 5**: Should successfully create 3 subtasks linked to parent
- **Test 6**: Should show only the 3 subtasks
- **Test 7**: Should show parent task with `subtaskIds` array containing the 3 subtask UUIDs

## Notes

- Replace `PARENT_TASK_UUID_HERE` with the actual UUID from the parent task creation
- The validation now catches placeholder IDs like `task_1` and provides helpful guidance
- Real UUIDs (like `7b3e4f2a-8c9d-4e5f-b0a1-2c3d4e5f6a7b`) work perfectly
- Parent-child relationships are automatically maintained in both directions
