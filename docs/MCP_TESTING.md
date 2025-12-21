# MCP Feature Testing Plan

> Extends TESTING.md with MCP-specific test cases

---

## Test Matrix

| Test Case | Repo | What to Verify |
|-----------|------|----------------|
| MCP-01 | markwise.app | Server starts, returns valid JSON-RPC |
| MCP-02 | markwise.app | `project-context` tool returns knowledge |
| MCP-03 | markwise.app | Query filtering works semantically |
| MCP-04 | markwise.app | Local install creates correct config |
| MCP-05 | markwise.app | Global install creates correct config |

---

## MCP-01: Server Initialization

```bash
cd ~/taskwing-tests/markwise.app

# Send initialize request
(echo '{"jsonrpc":"2.0","method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}},"id":1}'; sleep 2) | tw mcp 2>&1 | head -15
```

**Expected:**
- Banner: `🎯 TaskWing MCP Server Starting...`
- JSON response with `serverInfo.name: "taskwing"`, `version: "2.0.0"`
- No errors

---

## MCP-02: Project Context Tool (Full)

```bash
cd ~/taskwing-tests/markwise.app

(echo '{"jsonrpc":"2.0","method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}},"id":1}';
 echo '{"jsonrpc":"2.0","method":"initialized","params":{},"id":2}';
 echo '{"jsonrpc":"2.0","method":"tools/call","params":{"name":"project-context","arguments":{}},"id":3}';
 sleep 2) | tw mcp 2>&1 | tail -20
```

**Expected:**
- Response contains `nodes` with features and decisions
- `total` count matches `tw list` count

---

## MCP-03: Semantic Query

```bash
cd ~/taskwing-tests/markwise.app

(echo '{"jsonrpc":"2.0","method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}},"id":1}';
 echo '{"jsonrpc":"2.0","method":"initialized","params":{},"id":2}';
 echo '{"jsonrpc":"2.0","method":"tools/call","params":{"name":"project-context","arguments":{"query":"database"}},"id":3}';
 sleep 2) | tw mcp 2>&1 | grep -i "mongo\|lance"
```

**Expected:**
- Response contains MongoDB/LanceDB related nodes
- Results are filtered (not all nodes returned)

---

## MCP-04: Local Install

```bash
cd ~/taskwing-tests/markwise.app
rm -rf .claude .cursor .windsurf .gemini

tw mcp install claude
tw mcp install cursor

cat .claude/mcp.json
cat .cursor/mcp.json
```

**Expected:**
- Files created in **project directory** (not ~/.claude)
- Each contains `"taskwing"` server entry
- `command` points to taskwing binary
- No `cwd` field (runs from current dir)

---

## MCP-05: Global Install

```bash
cd ~/taskwing-tests/markwise.app

tw mcp install claude --global

cat ~/.claude/mcp.json | grep -A 5 "taskwing-markwise"
```

**Expected:**
- Server added to `~/.claude/mcp.json`
- Server name is project-specific: `taskwing-markwise.app`
- Includes `cwd` pointing to project path

---

## Cleanup After Testing

**IMPORTANT:** After testing, clean up test artifacts:

```bash
# Clean local test configs
cd ~/taskwing-tests/markwise.app
rm -rf .claude .cursor .windsurf .gemini

# Clean global config (manual edit required)
# Remove "taskwing-markwise.app" entry from ~/.claude/mcp.json
```

---

## Latest Test Results

| Test | Status | Date | Notes |
|------|--------|------|-------|
| MCP-01 | ✅ PASS | 2025-12-19 | serverInfo correct |
| MCP-02 | ✅ PASS | 2025-12-19 | Full context returned |
| MCP-03 | ✅ PASS | 2025-12-19 | 5 filtered results for "database" |
| MCP-04 | ✅ PASS | 2025-12-19 | .claude/mcp.json created locally |
| MCP-05 | ✅ PASS | 2025-12-19 | taskwing-markwise.app added to global |

**All MCP features verified.**
