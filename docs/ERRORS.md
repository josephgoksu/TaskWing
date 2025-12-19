# TaskWing v2 — Error Messages

> **Purpose:** Document error cases and recovery paths for users

---

## Bootstrap Errors

### "No git history found"

**Cause:** Directory is not a git repository.

**Solution:**
```bash
git init
# Or use TaskWing without bootstrap:
taskwing add "Your initial knowledge here"
```

### "No conventional commits found"

**Cause:** Your repo doesn't use `feat:`, `fix:`, etc. commit format.

**What happens:** TaskWing falls back to directory structure scanning.

**Solution:** Either:
1. Adopt [Conventional Commits](https://www.conventionalcommits.org/)
2. Manually add knowledge after bootstrap:
```bash
taskwing add "Authentication module handles OAuth2 and sessions"
```

### "No features detected"

**Cause:** No `feat:` commits AND no recognizable directory structure.

**Solution:**
```bash
taskwing add "Main application core functionality"
```

---

## Knowledge (Add) Errors

### "Content cannot be empty"

**Cause:** Trying to add empty text.

**Solution:**
```bash
taskwing add "Describe what you want to remember"
```

### "No API key found"

**Cause:** AI classification requires an API key.

**Solution:**
```bash
# Set your OpenAI key
export OPENAI_API_KEY=your-key

# Or skip AI classification with --type
taskwing add "Your text" --type decision
```

---

## Context (Search) Errors

### "No knowledge nodes found"

**Cause:** The knowledge graph is empty.

**Solution:**
```bash
# Bootstrap your project first
taskwing bootstrap

# Or add knowledge manually
taskwing add "Your architectural knowledge"
```

### "No matching knowledge found"

**Cause:** Semantic search found no relevant nodes.

**Solution:**
- Try different search terms
- Add more knowledge with `taskwing add`

---

## Integrity Errors

### "Index rebuild failed"

**Cause:** `index.json` cache is corrupted.

**Solution:**
```bash
taskwing memory rebuild-index
```

### "Database locked"

**Cause:** Another process is accessing `memory.db`.

**Solution:** Close other TaskWing processes and retry.

---

## SQLite Errors

### "disk I/O error"

**Cause:** Filesystem issue.

**Solution:**
1. Check disk space
2. Check file permissions on `.taskwing/memory/`

### "database is locked"

**Cause:** Concurrent write access.

**Solution:** Ensure only one TaskWing command runs at a time.

---

## MCP Errors

### "MCP connection failed"

**Cause:** The AI tool couldn't connect to TaskWing.

**Solution:**
```bash
# Restart MCP server
taskwing mcp

# Check if another process is using the connection
```

### "No context available"

**Cause:** Knowledge graph is empty.

**Solution:**
```bash
taskwing bootstrap
# or
taskwing add "Your project context"
```

---

## Need Help?

```bash
# Check memory integrity
taskwing memory check

# Repair if needed
taskwing memory repair

# Rebuild index
taskwing memory rebuild-index
```
