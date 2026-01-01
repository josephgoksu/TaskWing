# Taskwing Quality Improvement Plan (v2)

## Status: Phase 1 Complete ✅

**Implemented Changes** (2024-01-01):
- Recursive markdown discovery for package READMEs
- Increased file limits (10 → 50) with budget cap (150k chars)
- Language-agnostic priority patterns (Go, TS, Python, Rust, Java, etc.)
- Enhanced CodeAgent prompt with error/security/config sections

## Executive Summary

This document provides a precise root cause analysis and actionable fixes for improving Taskwing's knowledge extraction quality, based on testing against the Markwise codebase.

---

## Part 1: Precise Root Cause Analysis

### Problem 1: DocAgent Only Reads Root + docs/

**File**: `internal/agents/tools/context.go:27-64`

```go
func (g *ContextGatherer) GatherMarkdownDocs() string {
    gatherFromDir(g.BasePath, "", 4000)           // ← Only root
    gatherFromDir(filepath.Join(g.BasePath, "docs"), "docs", 3000)  // ← Only docs/
    return sb.String()
}
```

**What happens**: Only markdown files in the repository root and `docs/` folder are read.

**What's missed**:
- `internal/resilience/README.md` - Documents circuit breaker patterns
- `internal/auth/README.md` - Documents auth middleware
- `internal/vectorsearch/README.md` - Documents embedding cache settings
- `backend-go/AGENTS.md` - Documents the entire Go backend architecture

**Impact**: Package-level documentation with critical implementation details is invisible to the LLM.

---

### Problem 2: CodeAgent Has Severe File Limits

**File**: `internal/agents/tools/context.go:172-315`

```go
func (g *ContextGatherer) GatherSourceCode() string {
    maxFiles := 10        // ← ONLY 10 FILES TOTAL
    maxPerFile := 3000    // ← Truncated to 3000 chars each
```

**What happens**: For a large codebase, only 10 files are read. Priority goes to:
1. Entry points (`main.go`, `cmd/root.go`)
2. First matches of `internal/api/*.go`, `internal/config/*.go`
3. Frontend entry points (`src/index.ts`, `src/App.tsx`)

**What's missed** (never reached due to 10-file limit):
- `internal/resilience/*.go` - Circuit breakers, retries
- `internal/auth/middleware.go` - CORS, JWT validation
- `internal/bookmarks/handler.go` - Error response patterns
- `web/vite.config.ts` - Bundle splitting configuration

**Impact**: 95% of the codebase is never seen by the LLM.

---

### Problem 3: Priority Patterns Miss Middleware/Config Files

**File**: `internal/agents/tools/context.go:210-219`

```go
priorityPatterns := []string{
    "main.go", "cmd/main.go", "cmd/root.go",
    "internal/api/*.go", "internal/server/*.go", "internal/config/*.go",
    "src/index.ts", "src/main.ts", "src/App.tsx",
    // ...
}
```

**What's missing from priority patterns**:
```go
// These patterns don't exist:
"**/middleware*.go",      // Middleware files
"**/handler*.go",         // HTTP handlers
"**/router*.go",          // Router setup
"**/*_config*.go",        // Config structs
"**/model*.go",           // Data models
"**/*schema*.go",         // Database schemas
```

**Impact**: Even if maxFiles was higher, middleware and config files wouldn't be prioritized.

---

### Problem 4: CodeAgent Prompt Asks Wrong Questions

**File**: `internal/config/prompts.go:287-358`

The prompt asks for:
```
1. Architectural patterns (MVC, Clean Architecture, Hexagonal)
2. Design patterns (Repository, Factory, Dependency Injection)
3. Key abstractions (interfaces, base classes, core types)
4. Integration patterns (events, queues, APIs)
```

**What the prompt DOESN'T ask for**:
- Error handling patterns (how errors are wrapped, logged, returned)
- Security configurations (CORS origins, rate limits, auth middleware)
- Performance tuning values (pool sizes, cache TTLs, timeouts)
- Data model schemas (MongoDB collections, field types)
- Middleware chain order and configuration

**Impact**: Even if the right files were read, the LLM wouldn't extract the information we need.

---

### Problem 5: DepsAgent Only Extracts "What", Not "How"

**File**: `internal/config/prompts.go:244-285`

The prompt asks:
```
Identify KEY TECHNOLOGY DECISIONS from the dependencies:
1. Framework choices (React, Vue, Express, Chi, etc.)
2. Database drivers (what databases are used)
...
```

**What we get**: "The project uses chi for HTTP routing"

**What we need**: "Chi is configured with CORS middleware allowing origins: markwise.app, my.markwise.app, localhost:5181. Rate limiting is 100 req/min."

**Impact**: We know WHAT libraries are used but not HOW they're configured.

---

## Part 2: Specific Fixes

### Fix 1: Expand Markdown Discovery (Low Effort, High Impact)

**File**: `internal/agents/tools/context.go`

**Current** (lines 27-64):
```go
func (g *ContextGatherer) GatherMarkdownDocs() string {
    gatherFromDir(g.BasePath, "", 4000)
    gatherFromDir(filepath.Join(g.BasePath, "docs"), "docs", 3000)
    return sb.String()
}
```

**Fix**: Add recursive markdown discovery for internal packages and key locations:

```go
func (g *ContextGatherer) GatherMarkdownDocs() string {
    var sb strings.Builder
    seen := make(map[string]bool)

    // Existing: root and docs/
    gatherFromDir(g.BasePath, "", 4000)
    gatherFromDir(filepath.Join(g.BasePath, "docs"), "docs", 3000)

    // NEW: Package-level READMEs (critical for implementation details)
    packageDirs := []string{"internal", "pkg", "lib", "src", "backend-go/internal"}
    for _, dir := range packageDirs {
        filepath.WalkDir(filepath.Join(g.BasePath, dir), func(path string, d os.DirEntry, err error) error {
            if err != nil || d.IsDir() {
                return nil
            }
            name := strings.ToLower(d.Name())
            if name == "readme.md" || name == "agents.md" || name == "claude.md" {
                relPath, _ := filepath.Rel(g.BasePath, path)
                content, _ := os.ReadFile(path)
                if len(content) > 2000 {
                    content = content[:2000]
                }
                sb.WriteString(fmt.Sprintf("## PACKAGE DOC: %s\n```\n%s\n```\n\n", relPath, content))
            }
            return nil
        })
    }

    return sb.String()
}
```

**Expected improvement**: Circuit breakers, auth patterns, and vectorsearch configs documented in package READMEs will now be visible.

---

### Fix 2: Increase File Limits and Add Targeted Patterns (Medium Effort)

**File**: `internal/agents/tools/context.go`

**Current** (line 175-176):
```go
maxFiles := 10
maxPerFile := 3000
```

**Fix**: Increase limits and add middleware-specific patterns:

```go
maxFiles := 25        // Increase from 10
maxPerFile := 4000    // Increase from 3000

// Add these to priorityPatterns:
priorityPatterns := []string{
    // Existing entry points...
    "main.go", "cmd/main.go", "cmd/root.go",

    // NEW: Middleware and security (HIGH PRIORITY)
    "**/middleware*.go",
    "**/middleware/*.go",
    "**/auth/middleware.go",
    "**/cors*.go",
    "**/ratelimit*.go",

    // NEW: Handlers and routers
    "**/handler*.go",
    "**/router*.go",
    "**/routes*.go",

    // NEW: Config and models
    "**/config/*.go",
    "**/config.go",
    "**/model*.go",
    "**/schema*.go",
    "**/types*.go",

    // NEW: Error handling
    "**/error*.go",
    "**/resilience/*.go",

    // Existing frontend...
    "src/index.ts", "src/App.tsx",

    // NEW: Frontend config
    "vite.config.*",
    "tailwind.config.*",
    "*.config.js",
    "*.config.ts",
}
```

**Expected improvement**: Middleware, config, and error handling files will be prioritized and read.

---

### Fix 3: Enhance CodeAgent Prompt (Medium Effort, High Impact)

**File**: `internal/config/prompts.go`

**Add these sections to `PromptTemplateCodeAgent`** (after line 296):

```go
const PromptTemplateCodeAgent = `You are a software architect analyzing source code...

// ... existing content ...

## ADDITIONAL ANALYSIS REQUIRED

### 5. ERROR HANDLING PATTERNS
How does this codebase handle errors?
- Error wrapping (pkg/errors, fmt.Errorf with %w, custom)
- Logging library and patterns (slog, zap, logrus)
- HTTP error response format (status codes, body structure)
- Where errors are caught vs propagated

### 6. SECURITY & MIDDLEWARE CONFIGURATION
What security measures are configured?
- CORS settings (allowed origins, methods, credentials)
- Rate limiting (limits per endpoint, window size)
- Authentication middleware (JWT validation, session handling)
- Request validation and sanitization

### 7. PERFORMANCE TUNING VALUES
What are the configured performance parameters?
- Connection pool sizes (database, HTTP clients)
- Cache TTLs and size limits
- Timeout values (request, database, external calls)
- Retry policies and circuit breaker thresholds

### 8. DATA MODEL SCHEMAS
What data structures are used?
- Database collection/table schemas
- API request/response types
- Configuration structs with defaults
- Key interfaces and their implementations

For each finding, include:
- Exact file path and line numbers
- Code snippet as evidence
- Actual configuration values (not just "uses caching" but "cache TTL is 24 hours")

// ... existing JSON format ...
`
```

**Expected improvement**: LLM will extract security configs, error patterns, and performance tuning values.

---

### Fix 4: Add Config Value Extraction (Medium Effort)

**File**: `internal/agents/tools/context.go`

**Add new function**:

```go
// GatherConfigFiles specifically targets configuration files with actual values
func (g *ContextGatherer) GatherConfigFiles() string {
    var sb strings.Builder

    configPatterns := []string{
        ".env.example",
        "config/*.go",
        "config/*.yaml",
        "config/*.json",
        "**/config.go",
        "vite.config.*",
        "tailwind.config.*",
        "package.json",  // For size-limit, scripts
    }

    for _, pattern := range configPatterns {
        matches, _ := filepath.Glob(filepath.Join(g.BasePath, pattern))
        for _, match := range matches {
            content, err := os.ReadFile(match)
            if err != nil {
                continue
            }
            relPath, _ := filepath.Rel(g.BasePath, match)

            // For .env files, only show structure not values
            if strings.Contains(relPath, ".env") {
                content = redactEnvValues(content)
            }

            if len(content) > 2000 {
                content = content[:2000]
            }
            sb.WriteString(fmt.Sprintf("## CONFIG: %s\n```\n%s\n```\n\n", relPath, content))
        }
    }

    return sb.String()
}
```

**Update CodeAgent** to include config files in context:

```go
// In code_deterministic.go Run():
configFiles := gatherer.GatherConfigFiles()

chainInput := map[string]any{
    "ProjectName": input.ProjectName,
    "DirTree":     dirTree,
    "SourceCode":  sourceCode,
    "ConfigFiles": configFiles,  // NEW
}
```

---

### Fix 5: Add Security-Focused Grep Patterns (Low Effort)

**File**: `internal/agents/tools/context.go`

**Add new function**:

```go
// GatherSecurityPatterns searches for security-related code patterns
func (g *ContextGatherer) GatherSecurityPatterns() string {
    patterns := map[string]string{
        "CORS":           `cors\.(New|Handler|Options)|AllowedOrigins|Access-Control`,
        "RateLimit":      `rate.*limit|RateLimit|throttle|NewLimiter`,
        "Auth":           `jwt\.(Parse|Sign|Valid)|Bearer|Authorization`,
        "CircuitBreaker": `circuit.*breaker|CircuitBreaker|gobreaker`,
        "Retry":          `retry\.|Retry\(|backoff|MaxRetries`,
        "Timeout":        `Timeout:|WithTimeout|context\.WithDeadline`,
    }

    var sb strings.Builder
    for name, pattern := range patterns {
        matches := grepPattern(g.BasePath, pattern, 5) // max 5 matches per pattern
        if len(matches) > 0 {
            sb.WriteString(fmt.Sprintf("## %s Patterns\n", name))
            for _, m := range matches {
                sb.WriteString(fmt.Sprintf("- %s:%d: %s\n", m.File, m.Line, m.Content))
            }
            sb.WriteString("\n")
        }
    }

    return sb.String()
}
```

---

## Part 3: Implementation Priority

### Phase 1: Quick Wins (1-2 days)

| Task | File | Lines to Change | Impact |
|------|------|-----------------|--------|
| Add `internal/**/README.md` to markdown discovery | `context.go:27-64` | ~15 lines | High - captures package docs |
| Increase maxFiles from 10 to 25 | `context.go:175` | 1 line | Medium - reads more code |
| Add middleware patterns to priorityPatterns | `context.go:210-219` | ~10 lines | High - finds security code |
| Add error/security/perf sections to prompt | `prompts.go:287-358` | ~40 lines | High - extracts right info |

### Phase 2: Medium Effort (3-5 days)

| Task | File | Effort | Impact |
|------|------|--------|--------|
| Add `GatherConfigFiles()` function | `context.go` | New function | Captures .env, vite.config |
| Add `GatherSecurityPatterns()` grep | `context.go` | New function | Finds CORS, rate limits |
| Update CodeAgent to include new context | `code_deterministic.go` | ~10 lines | Uses new data |
| Add config extraction to prompt | `prompts.go` | ~20 lines | Extracts values |

### Phase 3: Architecture Changes (1+ week)

| Task | Effort | Impact |
|------|--------|--------|
| Add SchemaAgent for data model extraction | New agent | Captures MongoDB schemas |
| Add SecurityAgent for dedicated security analysis | New agent | Deep security coverage |
| Implement AST parsing for Go struct extraction | New package | Precise schema extraction |

---

## Part 4: Validation

### Test Queries (Before/After)

Run these queries against Markwise and compare scores:

| Query | Current Score | Target Score |
|-------|---------------|--------------|
| `circuit breaker retry` | 0.00 | 0.20+ |
| `middleware CORS rate limiting` | 0.11 | 0.20+ |
| `MongoDB connection pooling` | 0.16 | 0.25+ |
| `error handling 500 logging` | 0.13 | 0.20+ |
| `bundle size optimization code splitting` | 0.17 | 0.25+ |

### Validation Commands

```bash
# Re-bootstrap after changes
cd /path/to/markwise
rm -rf .taskwing/memory
taskwing bootstrap

# Test queries
taskwing context "circuit breaker retry"
taskwing context "CORS middleware configuration"
taskwing context "error handling patterns"
```

---

## Part 5: Summary of Changes

### Files to Modify

| File | Change Type | Description |
|------|-------------|-------------|
| `internal/agents/tools/context.go` | Modify | Expand markdown discovery, increase file limits, add patterns |
| `internal/config/prompts.go` | Modify | Add error/security/config sections to CodeAgent prompt |
| `internal/agents/analysis/code_deterministic.go` | Modify | Include config files in chain input |

### New Functions to Add

1. `GatherConfigFiles()` - Target config files specifically
2. `GatherSecurityPatterns()` - Grep for security-related code
3. Recursive markdown walker for package READMEs

### Prompt Additions

1. Error handling patterns section
2. Security & middleware configuration section
3. Performance tuning values section
4. Data model schemas section

---

## Conclusion

The core issues are:
1. **Too few files read** (10 max) - most codebase never seen
2. **Wrong files prioritized** - entry points over middleware/config
3. **Wrong questions asked** - architecture patterns but not security/error/config details
4. **Package READMEs ignored** - critical implementation docs missed

The fixes are straightforward code changes to `context.go` and `prompts.go`. Phase 1 changes can be implemented in 1-2 days and will significantly improve coverage for real developer scenarios.
