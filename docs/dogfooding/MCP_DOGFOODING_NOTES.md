# MCP Dogfooding Notes

> Notes from using TaskWing MCP tools during development.

## Session: 2026-01-18 - Building New Agents

### Note #1: `recall` with `answer=true`
**Tool**: `mcp__taskwing-mcp__recall`
**Query**: "agent implementation pattern deterministic chain system prompt"

**Result**: Excellent context. Returned:
- DeterministicChain pattern explanation with file locations
- System prompt locations (`internal/config/prompts.go:463`, `:513`)
- Code symbols with line numbers
- Related constraint: "Deterministic Agent Patterns"

**Verdict**: High value. Single call gave enough context to understand the architecture.

---

### Note #2: `code search` relevance scores
**Tool**: `mcp__taskwing-mcp__code` with `action=search`
**Query**: "BaseAgent DeterministicChain agent implementation"

**Result**: Lower relevance scores (max 0.30), but still pointed to correct files:
- `internal/agents/impl/audit.go`
- `internal/agents/core/eino.go`
- `internal/agents/core/base.go`

**Verdict**: Useful for discovery, but scores suggest semantic search could be improved. Consider:
- Better embedding model?
- More context in index?

---

### Note #3: `code explain` is the killer feature
**Tool**: `mcp__taskwing-mcp__code` with `action=explain`
**Query**: "DeterministicChain"

**Result**: Comprehensive output including:
- Full source code with line numbers
- Related decisions (CloudWeGo Eino, Bubble Tea, CGO-free SQLite)
- Related patterns (Adding a New Agent, Deterministic Agent Patterns)
- AI-generated explanation of architectural significance
- Impact analysis (callers/callees)

**Verdict**: This is what developers actually need. Deep context in one call.

---

## Improvement Ideas

1. **Search relevance**: 0.30 scores feel low. Investigate embedding quality.
2. **Cross-reference**: `recall` + `code explain` together would be even better.
3. **Streaming**: Long `explain` calls could benefit from streaming output.

---

### Note #4: Context integration for agents
**Use case**: SimplifyAgent needs architectural context to avoid removing patterns that are intentional.

**Approach**: Handler fetches context via `RecallApp.Query()` before invoking agent:
```go
result, err := recallApp.Query(ctx, "patterns and constraints for "+filePath, ...)
kgContext = formatRecallContext(result)
```

**Verdict**: Clean pattern. Agents stay focused on their task, MCP layer handles context fetching. Could be abstracted into a helper.

---

### Note #5: Adding a new action to unified tools
**Task**: Add `simplify` action to the `code` tool.

**Steps**:
1. Add action const to `internal/mcp/types.go`
2. Update `IsValid()` and `ValidCodeActions()`
3. Add handler to `internal/mcp/handlers.go`
4. Add formatter to `internal/mcp/presenter.go`
5. Update MCP tool description in `cmd/mcp_server.go`

**Verdict**: Pattern is clear and easy to follow. Could use a generator or scaffolding tool for new actions.

---

## Summary

| Tool | Use Case | Verdict |
|------|----------|---------|
| `recall` + `answer=true` | Quick architecture questions | Excellent |
| `code search` | Discovery, finding files | Good (needs tuning) |
| `code explain` | Deep dive into symbols | Excellent |
| `code simplify` | Reduce code complexity | New - testing needed |
| `code find` | Not tested yet | TBD |
| `code impact` | Not tested yet | TBD |

### Note #6: Adding a new standalone tool
**Task**: Add `debug` as a new MCP tool.

**Different from unified tools**: Debug is not code intelligence, so separate tool makes sense.

**Pattern**:
1. Add params type to `internal/mcp/types.go`
2. Add handler to `internal/mcp/handlers.go`
3. Add formatter to `internal/mcp/presenter.go`
4. Register tool in `cmd/mcp_server.go`

**Observation**: Handler pattern is consistent - fetch context, invoke agent, format output.

---

### Note #7: Slash commands
**Task**: Add `/tw-simplify` and `/tw-debug` slash commands.

**Pattern**:
1. Add command to `cmd/slash.go` (`slashSimplifyCmd`, etc.)
2. Add content to `cmd/slash_content.go`
3. Content tells AI which MCP tools to call

**Key insight**: Slash commands are just prompts that tell AI how to use MCP tools. They bridge user intent to tool invocation.

---

## Patterns Discovered

1. **Context → Agent → Format**: MCP handlers should fetch context, run agent, format output
2. **Unified tools**: Single tool with `action` param is cleaner than many small tools
3. **Presenter separation**: Format logic separate from business logic is maintainable
4. **Slash commands**: Prompts that teach AI how to combine MCP tools for specific workflows

---

### Note #8: Type Assertion Bug in Formatters
**Issue**: QA audit found critical runtime bug in `FormatDebugResult` and `FormatSimplifyResult`.

**Problem**: Direct type assertions like `f.Metadata["changes"].([]SimplifyChange)` fail at runtime when data has been through JSON serialization/deserialization. JSON unmarshals:
- Slices → `[]interface{}`
- Maps → `map[string]interface{}`
- Numbers → `float64` (not `int`)

**Solution**: Added extraction helper functions that handle both direct types and `[]interface{}`:
```go
func extractSimplifyChanges(raw interface{}) []SimplifyChange {
    // Direct type match (from agent before serialization)
    if typed, ok := raw.([]SimplifyChange); ok {
        return typed
    }
    // Handle []interface{} from JSON
    if arr, ok := raw.([]interface{}); ok {
        result := make([]SimplifyChange, 0, len(arr))
        for _, item := range arr {
            if m, ok := item.(map[string]interface{}); ok {
                c := SimplifyChange{
                    What: getStringField(m, "what"),
                    Why:  getStringField(m, "why"),
                    Risk: getStringField(m, "risk"),
                }
                result = append(result, c)
            }
        }
        return result
    }
    return nil
}
```

**Key insight**: Always assume metadata coming through MCP could be JSON-serialized. Don't trust direct type assertions on complex types.

**Files fixed**:
- `internal/mcp/presenter.go` - Added `extractDebugHypotheses`, `extractDebugSteps`, `extractDebugFixes`, `extractSimplifyChanges`, `getIntFromMetadata`

**Tests added**:
- `TestFormatDebugResult_WithJSONStyleData` - Tests with `[]interface{}` data
- `TestFormatSimplifyResult_WithJSONStyleData` - Tests with `float64` numbers
- `TestExtractSimplifyChanges_DirectType` - Tests direct type handling
- `TestExtractDebugHypotheses_DirectType` - Tests direct type handling
- `TestGetIntFromMetadata` - Tests int extraction from `float64`

---

### Note #9: Path Traversal Vulnerability
**Issue**: QA audit found security vulnerability in `handleCodeSimplify`.

**Problem**: The original code:
```go
basePath, _ := config.GetProjectRoot()
fullPath := filePath
if basePath != "" && !strings.HasPrefix(filePath, "/") {
    fullPath = basePath + "/" + filePath
}
content, err := readFileContent(fullPath)
```

This allowed `../../../etc/passwd` to escape the project root via path traversal.

**Solution**: Added `validateAndResolvePath()` function that:
1. Uses `filepath.Clean()` to normalize the path
2. Rejects any path containing `..` after cleaning
3. Verifies the resolved absolute path starts with the project root
4. Checks that the path is a file, not a directory

**Tests added**:
- `TestValidateAndResolvePath_PathTraversal` - Tests various traversal attacks
- `TestValidateAndResolvePath_ValidPaths` - Tests legitimate paths
- `TestValidateAndResolvePath_DirectoryRejection` - Rejects directories
- `TestHandleCodeTool_SimplifyPathTraversal` - Integration test

**Key insight**: Any file reading handler exposed via MCP needs strict path validation. MCP tools are exposed to potentially untrusted input.

---
