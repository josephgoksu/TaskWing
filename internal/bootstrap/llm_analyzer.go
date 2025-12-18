package bootstrap

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/josephgoksu/TaskWing/internal/llm"
)

// LLMAnalyzer uses an LLM to infer architectural decisions from code
type LLMAnalyzer struct {
	BasePath  string
	ChatModel model.BaseChatModel
	Model     string // For display purposes
}

// AnalyzedFeature is an LLM-inferred feature with reasoning
type AnalyzedFeature struct {
	Name        string             `json:"name"`
	Description string             `json:"description"`
	Purpose     string             `json:"purpose"` // WHY it exists
	KeyFiles    []string           `json:"key_files"`
	Decisions   []AnalyzedDecision `json:"decisions"`
	// Relationships inferred by LLM
	DependsOn []string `json:"depends_on,omitempty"` // Feature names this depends on
	Extends   []string `json:"extends,omitempty"`    // Feature names this extends
	RelatedTo []string `json:"related_to,omitempty"` // Loosely related features
}

// AnalyzedDecision is an LLM-inferred decision
type AnalyzedDecision struct {
	Title      string `json:"title"`
	Why        string `json:"why"`        // The reasoning
	Tradeoffs  string `json:"tradeoffs"`  // What was sacrificed
	Confidence string `json:"confidence"` // high/medium/low
}

// LLMResponse is the structured response from the LLM
type LLMResponse struct {
	Features []AnalyzedFeature `json:"features"`
}

// NewLLMAnalyzer creates a new LLM-powered analyzer using Eino ChatModel.
func NewLLMAnalyzer(ctx context.Context, basePath string, cfg llm.Config) (*LLMAnalyzer, error) {
	chatModel, err := llm.NewChatModel(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create chat model: %w", err)
	}
	return &LLMAnalyzer{
		BasePath:  basePath,
		ChatModel: chatModel,
		Model:     cfg.Model,
	}, nil
}

// Analyze sends context to LLM and gets inferred architecture
func (a *LLMAnalyzer) Analyze(ctx context.Context) (*LLMResponse, error) {
	// 1. Gather context
	projectContext, err := a.gatherContext()
	if err != nil {
		return nil, fmt.Errorf("gather context: %w", err)
	}

	// 2. Build prompt
	prompt := a.buildPrompt(projectContext)

	// 3. Call LLM with streaming
	response, err := a.callLLMStreaming(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("call LLM: %w", err)
	}

	return response, nil
}

// ProjectContext holds all the gathered project information
type ProjectContext struct {
	DirectoryTree       string
	PackageFiles        map[string]string // package.json, go.mod, etc.
	ReadmeContent       string
	DocFiles            map[string]string // AGENTS.md, docs/*.md, etc.
	DeploymentInfo      map[string]string // Dockerfile, docker-compose.yaml
	EntryPoints         map[string]string // main.go, cmd/*.go - initialization patterns
	ConfigFiles         map[string]string // .env.example, vite.config.ts, Makefile
	ImportGraph         string            // Go import relationships (internal packages)
	DiscoveredFiles     map[string]string // Dynamically discovered high-value files
	GitSummary          string
	CurrentDependencies []string // Parsed from package.json/go.mod - the ACTUAL current stack
}

// FileCategory represents the type of discovered file
type FileCategory string

const (
	CategoryDocs   FileCategory = "docs"
	CategoryDeps   FileCategory = "deps"
	CategoryDeploy FileCategory = "deploy"
	CategoryEntry  FileCategory = "entry"
	CategoryConfig FileCategory = "config"
	CategoryBuild  FileCategory = "build"
	CategoryEnv    FileCategory = "env"
	CategoryCI     FileCategory = "ci"
)

// DiscoveredFile represents a file found via pattern matching
type DiscoveredFile struct {
	Path     string
	Category FileCategory
	Weight   int // 1-10, higher = more important
}

// filePattern defines a pattern for discovering important files
type filePattern struct {
	pattern  string // glob or regex pattern
	category FileCategory
	weight   int
	maxDepth int // 0 = root only, 1 = one level, -1 = any depth
}

// discoverImportantFiles scans the project and returns high-value files
func (a *LLMAnalyzer) discoverImportantFiles() []DiscoveredFile {
	patterns := []filePattern{
		// Documentation (high priority)
		{"README*", CategoryDocs, 10, 0},
		{"ARCHITECTURE*", CategoryDocs, 10, 0},
		{"DESIGN*", CategoryDocs, 9, 0},
		{"AGENTS.md", CategoryDocs, 9, 0},
		{"CLAUDE.md", CategoryDocs, 9, 0},
		{"GEMINI.md", CategoryDocs, 9, 0},
		{"CONTRIBUTING*", CategoryDocs, 7, 0},
		{"*.md", CategoryDocs, 5, 1}, // docs/*.md

		// Deployment (high priority)
		{"Dockerfile*", CategoryDeploy, 10, 1},
		{"docker-compose*", CategoryDeploy, 10, 0},
		{"compose.yaml", CategoryDeploy, 10, 0},
		{"compose.yml", CategoryDeploy, 10, 0},
		{".dockerignore", CategoryDeploy, 5, 0},

		// Entry points (any language)
		{"main.*", CategoryEntry, 9, 2},
		{"index.*", CategoryEntry, 8, 1},
		{"app.*", CategoryEntry, 7, 1},
		{"server.*", CategoryEntry, 7, 1},
		{"cmd/*/main.*", CategoryEntry, 9, 2},

		// Dependencies (any language)
		{"package.json", CategoryDeps, 10, 2},
		{"go.mod", CategoryDeps, 10, 2},
		{"requirements.txt", CategoryDeps, 10, 1},
		{"Pipfile", CategoryDeps, 9, 0},
		{"pyproject.toml", CategoryDeps, 10, 1},
		{"Gemfile", CategoryDeps, 10, 0},
		{"Cargo.toml", CategoryDeps, 10, 1},
		{"build.gradle*", CategoryDeps, 9, 1},
		{"pom.xml", CategoryDeps, 9, 1},
		{"composer.json", CategoryDeps, 9, 0},

		// Config files
		{"*config.ts", CategoryConfig, 8, 1},
		{"*config.js", CategoryConfig, 8, 1},
		{"*config.mjs", CategoryConfig, 8, 1},
		{"*.config.yaml", CategoryConfig, 7, 0},
		{"*.config.yml", CategoryConfig, 7, 0},
		{"*.config.json", CategoryConfig, 7, 0},
		{"tsconfig.json", CategoryConfig, 8, 1},

		// Build tools
		{"Makefile", CategoryBuild, 8, 1},
		{"justfile", CategoryBuild, 8, 0},
		{"Taskfile*", CategoryBuild, 7, 0},
		{"turbo.json", CategoryBuild, 7, 0},
		{"nx.json", CategoryBuild, 7, 0},

		// Environment
		{".env.example", CategoryEnv, 8, 1},
		{".env.sample", CategoryEnv, 8, 1},
		{"env.example", CategoryEnv, 8, 0},

		// CI/CD
		{".github/workflows/*.yaml", CategoryCI, 8, 2},
		{".github/workflows/*.yml", CategoryCI, 8, 2},
		{".gitlab-ci.yml", CategoryCI, 8, 0},
		{"Jenkinsfile", CategoryCI, 7, 0},
	}

	var discovered []DiscoveredFile

	filepath.WalkDir(a.BasePath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		relPath, _ := filepath.Rel(a.BasePath, path)
		depth := strings.Count(relPath, string(os.PathSeparator))

		// Skip common ignore dirs
		name := d.Name()
		if d.IsDir() {
			if strings.HasPrefix(name, ".") && name != ".github" {
				return filepath.SkipDir
			}
			if name == "node_modules" || name == "vendor" || name == "__pycache__" ||
				name == "dist" || name == "build" || name == ".git" {
				return filepath.SkipDir
			}
			return nil
		}

		// Match against patterns
		for _, p := range patterns {
			if p.maxDepth >= 0 && depth > p.maxDepth {
				continue
			}
			matched, _ := filepath.Match(p.pattern, name)
			if !matched {
				// Try matching full relative path for patterns like "cmd/*/main.*"
				matched, _ = filepath.Match(p.pattern, relPath)
			}
			if matched {
				discovered = append(discovered, DiscoveredFile{
					Path:     relPath,
					Category: p.category,
					Weight:   p.weight,
				})
				break // Only match first pattern
			}
		}
		return nil
	})

	// Sort by weight (descending)
	sort.Slice(discovered, func(i, j int) bool {
		return discovered[i].Weight > discovered[j].Weight
	})

	return discovered
}

func (a *LLMAnalyzer) gatherContext() (*ProjectContext, error) {
	ctx := &ProjectContext{
		PackageFiles:    make(map[string]string),
		DocFiles:        make(map[string]string),
		DeploymentInfo:  make(map[string]string),
		EntryPoints:     make(map[string]string),
		ConfigFiles:     make(map[string]string),
		DiscoveredFiles: make(map[string]string),
	}

	out := os.Stderr

	// 0. Dynamic file discovery (language-agnostic)
	fmt.Fprint(out, "   🔎 Discovering important files...")
	discovered := a.discoverImportantFiles()
	// Read top files by category
	categoryLimits := map[FileCategory]int{
		CategoryDocs:   5,
		CategoryDeps:   10,
		CategoryDeploy: 5,
		CategoryEntry:  6,
		CategoryConfig: 8,
		CategoryBuild:  3,
		CategoryEnv:    3,
		CategoryCI:     3,
	}
	categoryCounts := make(map[FileCategory]int)
	for _, f := range discovered {
		if categoryCounts[f.Category] >= categoryLimits[f.Category] {
			continue
		}
		content, err := os.ReadFile(filepath.Join(a.BasePath, f.Path))
		if err != nil {
			continue
		}
		// Truncate based on category
		maxLen := 1500
		if f.Category == CategoryEntry {
			// Only first 80 lines for entry points
			lines := strings.Split(string(content), "\n")
			if len(lines) > 80 {
				lines = lines[:80]
			}
			content = []byte(strings.Join(lines, "\n"))
		} else if len(content) > maxLen {
			content = content[:maxLen]
		}
		ctx.DiscoveredFiles[f.Path] = string(content)
		categoryCounts[f.Category]++
	}
	fmt.Fprintf(out, " %d files\n", len(ctx.DiscoveredFiles))

	// 1. Get directory tree (top 2 levels, limited output)
	fmt.Fprint(out, "   📁 Scanning directory structure...")
	var tree strings.Builder
	lineCount := 0
	maxLines := 50
	filepath.WalkDir(a.BasePath, func(path string, d os.DirEntry, err error) error {
		if err != nil || lineCount >= maxLines {
			return nil
		}
		rel, _ := filepath.Rel(a.BasePath, path)
		depth := strings.Count(rel, string(os.PathSeparator))
		if depth > 2 {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		// Skip hidden and common ignore dirs
		name := d.Name()
		if strings.HasPrefix(name, ".") || name == "node_modules" || name == "vendor" {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		indent := strings.Repeat("  ", depth)
		if d.IsDir() {
			tree.WriteString(fmt.Sprintf("%s%s/\n", indent, name))
		} else {
			tree.WriteString(fmt.Sprintf("%s%s\n", indent, name))
		}
		lineCount++
		return nil
	})
	ctx.DirectoryTree = tree.String()
	fmt.Fprintf(out, " (%d entries)\n", lineCount)

	// 2. Read key config files recursively (depth 2, limit size)
	fmt.Fprint(out, "   📦 Reading package files...")
	configFileNames := map[string]bool{
		"package.json":   true,
		"go.mod":         true,
		"Cargo.toml":     true,
		"pyproject.toml": true,
	}
	foundConfigs := []string{}
	filepath.WalkDir(a.BasePath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(a.BasePath, path)
		depth := strings.Count(rel, string(os.PathSeparator))
		if depth > 2 {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		// Skip hidden and common ignore dirs
		name := d.Name()
		if strings.HasPrefix(name, ".") || name == "node_modules" || name == "vendor" {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		// Check if this is a config file we want
		if !d.IsDir() && configFileNames[name] {
			content, err := os.ReadFile(path)
			if err != nil {
				return nil
			}
			// Truncate to 1500 chars
			if len(content) > 1500 {
				content = content[:1500]
			}
			// Use relative path as key to distinguish multiple package.json files
			ctx.PackageFiles[rel] = string(content)
			foundConfigs = append(foundConfigs, rel)
		}
		return nil
	})
	if len(foundConfigs) > 0 {
		fmt.Fprintf(out, " %s\n", strings.Join(foundConfigs, ", "))
	} else {
		fmt.Fprintln(out, " none found")
	}

	// 2b. Parse actual dependencies from ALL found package files
	for relPath, content := range ctx.PackageFiles {
		if strings.HasSuffix(relPath, "package.json") {
			deps := parsePackageJSONDeps(content)
			ctx.CurrentDependencies = append(ctx.CurrentDependencies, deps...)
		}
		if strings.HasSuffix(relPath, "go.mod") {
			deps := parseGoModDeps(content)
			ctx.CurrentDependencies = append(ctx.CurrentDependencies, deps...)
		}
	}

	// 3. Read README (truncated)
	fmt.Fprint(out, "   📄 Reading README...")
	readmeFiles := []string{"README.md", "readme.md", "README", "README.txt"}
	readmeFound := false
	for _, f := range readmeFiles {
		path := filepath.Join(a.BasePath, f)
		if content, err := os.ReadFile(path); err == nil {
			if len(content) > 1500 {
				content = content[:1500]
			}
			ctx.ReadmeContent = string(content)
			fmt.Fprintf(out, " %s (%d chars)\n", f, len(ctx.ReadmeContent))
			readmeFound = true
			break
		}
	}
	if !readmeFound {
		fmt.Fprintln(out, " not found")
	}

	// 3b. Read documentation and agent instruction files
	fmt.Fprint(out, "   📚 Reading docs...")
	agentFiles := []string{
		"AGENTS.md", "CLAUDE.md", "GEMINI.md", "CURSORRULES", ".cursorrules",
		"ARCHITECTURE.md", "DESIGN.md", "CONTRIBUTING.md",
	}
	docsFound := []string{}
	for _, f := range agentFiles {
		path := filepath.Join(a.BasePath, f)
		if content, err := os.ReadFile(path); err == nil {
			if len(content) > 2000 {
				content = content[:2000]
			}
			ctx.DocFiles[f] = string(content)
			docsFound = append(docsFound, f)
		}
	}
	// Also scan docs/ directory
	docsDir := filepath.Join(a.BasePath, "docs")
	if entries, err := os.ReadDir(docsDir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			if !strings.HasSuffix(name, ".md") {
				continue
			}
			path := filepath.Join(docsDir, name)
			if content, err := os.ReadFile(path); err == nil {
				if len(content) > 2000 {
					content = content[:2000]
				}
				ctx.DocFiles["docs/"+name] = string(content)
				docsFound = append(docsFound, "docs/"+name)
			}
			// Limit to 5 docs files to avoid bloating prompt
			if len(docsFound) >= 10 {
				break
			}
		}
	}
	if len(docsFound) > 0 {
		fmt.Fprintf(out, " %d files\n", len(docsFound))
	} else {
		fmt.Fprintln(out, " none found")
	}

	// 3c. Read deployment files (Dockerfile, docker-compose) for topology
	fmt.Fprint(out, "   🐳 Reading deployment files...")
	deployFiles := []string{
		"Dockerfile", "docker-compose.yaml", "docker-compose.yml",
		"compose.yaml", "compose.yml",
	}
	deployFound := []string{}
	for _, f := range deployFiles {
		path := filepath.Join(a.BasePath, f)
		if content, err := os.ReadFile(path); err == nil {
			if len(content) > 2000 {
				content = content[:2000]
			}
			ctx.DeploymentInfo[f] = string(content)
			deployFound = append(deployFound, f)
		}
	}
	// Also check common subdirs
	for _, subdir := range []string{"backend-go", "backend", "api", "server"} {
		for _, f := range deployFiles[:1] { // Just Dockerfile
			path := filepath.Join(a.BasePath, subdir, f)
			if content, err := os.ReadFile(path); err == nil {
				if len(content) > 1500 {
					content = content[:1500]
				}
				ctx.DeploymentInfo[subdir+"/"+f] = string(content)
				deployFound = append(deployFound, subdir+"/"+f)
			}
		}
	}
	if len(deployFound) > 0 {
		fmt.Fprintf(out, " %s\n", strings.Join(deployFound, ", "))
	} else {
		fmt.Fprintln(out, " none found")
	}

	// 3d. Read entry points (main.go, cmd/*.go) for initialization patterns
	fmt.Fprint(out, "   🚀 Reading entry points...")
	entryFound := []string{}
	// Check for main.go in root and common subdirs
	mainDirs := []string{"", "backend-go", "backend", "api", "server", "cmd"}
	for _, dir := range mainDirs {
		mainPath := filepath.Join(a.BasePath, dir, "main.go")
		if content, err := os.ReadFile(mainPath); err == nil {
			// Read first 100 lines (init patterns)
			lines := strings.Split(string(content), "\n")
			if len(lines) > 100 {
				lines = lines[:100]
			}
			key := "main.go"
			if dir != "" {
				key = dir + "/main.go"
			}
			ctx.EntryPoints[key] = strings.Join(lines, "\n")
			entryFound = append(entryFound, key)
		}
	}
	// Check cmd/ subdirectories
	cmdDir := filepath.Join(a.BasePath, "cmd")
	if entries, err := os.ReadDir(cmdDir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				mainPath := filepath.Join(cmdDir, entry.Name(), "main.go")
				if content, err := os.ReadFile(mainPath); err == nil {
					lines := strings.Split(string(content), "\n")
					if len(lines) > 80 {
						lines = lines[:80]
					}
					key := "cmd/" + entry.Name() + "/main.go"
					ctx.EntryPoints[key] = strings.Join(lines, "\n")
					entryFound = append(entryFound, key)
				}
			}
		}
	}
	// Check for cmd/*.go files directly
	if entries, err := os.ReadDir(cmdDir); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".go") {
				filePath := filepath.Join(cmdDir, entry.Name())
				if content, err := os.ReadFile(filePath); err == nil {
					lines := strings.Split(string(content), "\n")
					if len(lines) > 60 {
						lines = lines[:60]
					}
					key := "cmd/" + entry.Name()
					ctx.EntryPoints[key] = strings.Join(lines, "\n")
					entryFound = append(entryFound, key)
					if len(entryFound) >= 8 {
						break // Limit to avoid bloating
					}
				}
			}
		}
	}
	if len(entryFound) > 0 {
		fmt.Fprintf(out, " %d files\n", len(entryFound))
	} else {
		fmt.Fprintln(out, " none found")
	}

	// 3e. Read config files for framework/external dependency info
	fmt.Fprint(out, "   ⚙️  Reading config files...")
	cfgFiles := []string{
		".env.example", ".env.sample", "env.example",
		"vite.config.ts", "vite.config.js",
		"tsconfig.json", "next.config.js", "next.config.mjs",
		"Makefile", "justfile",
		"turbo.json", "nx.json",
	}
	configFound := []string{}
	// Check root
	for _, cf := range cfgFiles {
		path := filepath.Join(a.BasePath, cf)
		if content, err := os.ReadFile(path); err == nil {
			if len(content) > 1500 {
				content = content[:1500]
			}
			ctx.ConfigFiles[cf] = string(content)
			configFound = append(configFound, cf)
		}
	}
	// Check common subdirs for .env.example and vite.config
	for _, subdir := range []string{"web", "frontend", "app", "admin-dashboard", "landing-page"} {
		for _, f := range []string{".env.example", "vite.config.ts", "vite.config.js"} {
			path := filepath.Join(a.BasePath, subdir, f)
			if content, err := os.ReadFile(path); err == nil {
				if len(content) > 1000 {
					content = content[:1000]
				}
				ctx.ConfigFiles[subdir+"/"+f] = string(content)
				configFound = append(configFound, subdir+"/"+f)
			}
		}
	}
	if len(configFound) > 0 {
		fmt.Fprintf(out, " %d files\n", len(configFound))
	} else {
		fmt.Fprintln(out, " none found")
	}

	// 3f. Parse Go imports to understand internal package relationships
	fmt.Fprint(out, "   🔗 Analyzing imports...")
	ctx.ImportGraph = a.parseGoImports()
	if ctx.ImportGraph != "" {
		fmt.Fprintln(out, " done")
	} else {
		fmt.Fprintln(out, " no Go packages found")
	}

	// 4. Summarize git history (oldest → newest) to keep prompts bounded while
	// still capturing the full timeline.
	fmt.Fprint(out, "   🔍 Analyzing git history...")
	gitSummary, err := a.summarizeGitHistory()
	if err != nil {
		// Don't fail bootstrap because git isn't available; directory + config context is still useful.
		gitSummary = fmt.Sprintf("Git history summary unavailable: %v", err)
		fmt.Fprintln(out, " unavailable")
	} else if gitSummary == "" {
		fmt.Fprintln(out, " no git repo")
	} else {
		// Count commits from the summary
		lines := strings.Split(gitSummary, "\n")
		for _, line := range lines {
			if strings.Contains(line, "Total commits:") {
				parts := strings.Split(line, ":")
				if len(parts) > 1 {
					fmt.Fprintf(out, " %s commits\n", strings.TrimSpace(parts[1]))
				}
				break
			}
		}
	}
	ctx.GitSummary = gitSummary

	fmt.Fprintln(out, "")

	return ctx, nil
}

type scopeStats struct {
	Scope         string
	Total         int
	TypeCounts    map[string]int
	FirstSeen     time.Time
	LastSeen      time.Time
	FirstExamples []string
	LastExamples  []string
}

func (s *scopeStats) addExample(subject string) {
	const maxFirst = 2
	const maxLast = 2

	if len(s.FirstExamples) < maxFirst {
		s.FirstExamples = append(s.FirstExamples, subject)
	}

	if len(s.LastExamples) < maxLast {
		s.LastExamples = append(s.LastExamples, subject)
		return
	}
	s.LastExamples = append(s.LastExamples[1:], subject)
}

func addRecent(recent []string, subject string, max int) []string {
	if max <= 0 {
		return recent
	}
	if len(recent) < max {
		return append(recent, subject)
	}
	return append(recent[1:], subject)
}

func (a *LLMAnalyzer) summarizeGitHistory() (string, error) {
	gitDir := filepath.Join(a.BasePath, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		return "", nil
	}

	cmd := exec.Command("git", "log", "--format=%H|%ai|%s", "--reverse")
	cmd.Dir = a.BasePath

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("git log stdout: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("git log start: %w", err)
	}

	// Parse conventional commits
	conventionalWithScope := regexp.MustCompile(`^(feat|fix|refactor|perf|docs|style|test|chore|build|ci)\(([^)]+)\):\s*(.+)$`)
	conventionalNoScope := regexp.MustCompile(`^(feat|fix|refactor|perf):\s*(.+)$`)

	scanner := bufio.NewScanner(stdout)
	buf := make([]byte, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	totalCommits := 0
	totalConventional := 0
	var firstSHA, firstDate, firstSubject string
	var lastSHA, lastDate, lastSubject string

	byScope := make(map[string]*scopeStats)
	typeCounts := map[string]int{"feat": 0, "fix": 0, "refactor": 0, "perf": 0}
	yearCounts := make(map[int]int)
	recentConventional := make([]string, 0, 20)

	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, "|", 3)
		if len(parts) != 3 {
			continue
		}

		sha := parts[0]
		dateStr := parts[1]
		subject := parts[2]

		totalCommits++
		lastSHA = sha
		lastDate = dateStr
		lastSubject = subject
		if firstSHA == "" {
			firstSHA = sha
			firstDate = dateStr
			firstSubject = subject
		}

		commitTime, _ := time.Parse("2006-01-02 15:04:05 -0700", dateStr)
		if !commitTime.IsZero() {
			yearCounts[commitTime.Year()]++
		}

		var commitType, scope, description string
		matches := conventionalWithScope.FindStringSubmatch(subject)
		if matches != nil {
			commitType = matches[1]
			scope = matches[2]
			description = matches[3]
		} else if matches := conventionalNoScope.FindStringSubmatch(subject); matches != nil {
			commitType = matches[1]
			scope = "General"
			description = matches[2]
		}

		if commitType == "" {
			continue
		}
		if commitType != "feat" && commitType != "fix" && commitType != "refactor" && commitType != "perf" {
			continue
		}

		totalConventional++
		typeCounts[commitType]++

		ss := byScope[scope]
		if ss == nil {
			ss = &scopeStats{
				Scope:      scope,
				TypeCounts: map[string]int{"feat": 0, "fix": 0, "refactor": 0, "perf": 0},
				FirstSeen:  commitTime,
				LastSeen:   commitTime,
			}
			byScope[scope] = ss
		}

		ss.Total++
		ss.TypeCounts[commitType]++
		if !commitTime.IsZero() {
			if ss.FirstSeen.IsZero() {
				ss.FirstSeen = commitTime
			}
			ss.LastSeen = commitTime
		}

		ss.addExample(subject)
		recentDate := dateStr
		if !commitTime.IsZero() {
			recentDate = commitTime.Format("2006-01-02")
		}
		recentConventional = addRecent(recentConventional, fmt.Sprintf("%s [%s] %s", recentDate, scope, description), 20)
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("scan git log: %w", err)
	}
	if err := cmd.Wait(); err != nil {
		return "", fmt.Errorf("git log: %w", err)
	}

	if totalCommits == 0 {
		return "", nil
	}

	// Sort years ascending for readability
	years := make([]int, 0, len(yearCounts))
	for y := range yearCounts {
		years = append(years, y)
	}
	sort.Ints(years)

	// Sort scopes by conventional volume
	scopes := make([]*scopeStats, 0, len(byScope))
	for _, ss := range byScope {
		scopes = append(scopes, ss)
	}
	sort.Slice(scopes, func(i, j int) bool {
		if scopes[i].Total == scopes[j].Total {
			return scopes[i].Scope < scopes[j].Scope
		}
		return scopes[i].Total > scopes[j].Total
	})

	const maxScopes = 15
	if len(scopes) > maxScopes {
		scopes = scopes[:maxScopes]
	}

	var sb strings.Builder
	sb.WriteString("Git history summary (oldest → newest):\n")
	sb.WriteString(fmt.Sprintf("- Total commits: %d\n", totalCommits))
	if firstSHA != "" && lastSHA != "" {
		sb.WriteString(fmt.Sprintf("- Range: %s (%s) → %s (%s)\n", firstSHA[:8], firstDate, lastSHA[:8], lastDate))
	}
	if firstSubject != "" {
		sb.WriteString(fmt.Sprintf("- First commit: %s\n", firstSubject))
	}
	if lastSubject != "" {
		sb.WriteString(fmt.Sprintf("- Latest commit: %s\n", lastSubject))
	}
	sb.WriteString(fmt.Sprintf("- Conventional commits (feat/fix/refactor/perf): %d\n", totalConventional))
	sb.WriteString(fmt.Sprintf("  - feat=%d fix=%d refactor=%d perf=%d\n", typeCounts["feat"], typeCounts["fix"], typeCounts["refactor"], typeCounts["perf"]))

	if len(years) > 0 {
		sb.WriteString("- Commits by year: ")
		for i, y := range years {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(fmt.Sprintf("%d=%d", y, yearCounts[y]))
		}
		sb.WriteString("\n")
	}

	if len(scopes) > 0 {
		sb.WriteString("- Top scopes (by conventional commit volume):\n")
		for _, ss := range scopes {
			firstSeen := ""
			lastSeen := ""
			if !ss.FirstSeen.IsZero() {
				firstSeen = ss.FirstSeen.Format("2006-01-02")
			}
			if !ss.LastSeen.IsZero() {
				lastSeen = ss.LastSeen.Format("2006-01-02")
			}
			sb.WriteString(fmt.Sprintf("  - %s: total=%d (feat=%d fix=%d refactor=%d perf=%d) first=%s last=%s\n",
				ss.Scope, ss.Total, ss.TypeCounts["feat"], ss.TypeCounts["fix"], ss.TypeCounts["refactor"], ss.TypeCounts["perf"], firstSeen, lastSeen))

			if len(ss.FirstExamples) > 0 || len(ss.LastExamples) > 0 {
				sb.WriteString("    examples:\n")
				for _, ex := range ss.FirstExamples {
					sb.WriteString(fmt.Sprintf("      - %s\n", ex))
				}
				for _, ex := range ss.LastExamples {
					alreadyListed := false
					for _, first := range ss.FirstExamples {
						if ex == first {
							alreadyListed = true
							break
						}
					}
					if alreadyListed {
						continue
					}
					sb.WriteString(fmt.Sprintf("      - %s\n", ex))
				}
			}
		}
	}

	if len(recentConventional) > 0 {
		sb.WriteString("- RECENT ACTIVITY (most relevant for current architecture - PRIORITIZE THESE):\n")
		for _, line := range recentConventional {
			sb.WriteString(fmt.Sprintf("  - %s\n", line))
		}
	}

	return sb.String(), nil
}

func (a *LLMAnalyzer) buildPrompt(ctx *ProjectContext) string {
	var sb strings.Builder

	sb.WriteString(`You are an expert software architect analyzing a codebase.
Your task is to identify the major FEATURES and their KEY DECISIONS.

For each feature, explain:
- WHY it exists (the problem it solves)
- KEY DECISIONS made (technical choices with reasoning)
- TRADEOFFS accepted (what was sacrificed for the chosen approach)

Focus on architectural decisions, not implementation details.
Limit to 5-7 main features maximum.

IMPORTANT INSTRUCTIONS:
1. PRIORITIZE the "CURRENT TECH STACK" section over git commit history.
   If git mentions a technology NOT in current dependencies, it was likely removed.

2. Base your analysis on CURRENT state, not historical commits.

3. If "Project Documentation" is provided (AGENTS.md, ARCHITECTURE.md, etc.),
   use it as the authoritative source for architectural decisions.


RESPOND IN JSON FORMAT ONLY:
{
  "features": [
    {
      "name": "Feature Name",
      "description": "One-line description",
      "purpose": "WHY this feature exists - the problem it solves",
      "key_files": ["path/to/main/file.ts"],
      "depends_on": ["OtherFeatureName"],
      "extends": [],
      "related_to": ["AnotherFeatureName"],
      "decisions": [
        {
          "title": "Decision title",
          "why": "Full explanation of WHY this choice was made",
          "tradeoffs": "What was sacrificed or risks accepted",
          "confidence": "high|medium|low"
        }
      ]
    }
  ]
}

RELATIONSHIP TYPES:
- depends_on: This feature REQUIRES another feature to function (e.g., API depends on Auth)
- extends: This feature ADDS CAPABILITIES to another feature (e.g., PremiumUsers extends Users)
- related_to: Loose association, often interact but neither depends on the other

---
PROJECT CONTEXT:

`)

	sb.WriteString("## Directory Structure:\n```\n")
	sb.WriteString(ctx.DirectoryTree)
	sb.WriteString("```\n\n")

	if ctx.ReadmeContent != "" {
		sb.WriteString("## README (truncated):\n```\n")
		sb.WriteString(ctx.ReadmeContent)
		sb.WriteString("\n```\n\n")
	}

	// Add CURRENT TECH STACK section (prioritized over git history)
	if len(ctx.CurrentDependencies) > 0 {
		sb.WriteString("## CURRENT TECH STACK (from HEAD - THIS IS THE SOURCE OF TRUTH):\n")
		sb.WriteString("These are the ACTUAL dependencies currently in use. Base your analysis on these:\n")
		for _, dep := range ctx.CurrentDependencies {
			sb.WriteString(fmt.Sprintf("- %s\n", dep))
		}
		sb.WriteString("\n")
	}

	for name, content := range ctx.PackageFiles {
		sb.WriteString(fmt.Sprintf("## %s:\n```\n%s\n```\n\n", name, content))
	}

	// Add documentation files (AGENTS.md, docs/*.md, etc.)
	if len(ctx.DocFiles) > 0 {
		sb.WriteString("## Project Documentation (HIGHLY RELEVANT - use this for architectural decisions):\n")
		for name, content := range ctx.DocFiles {
			sb.WriteString(fmt.Sprintf("### %s:\n```\n%s\n```\n\n", name, content))
		}
	}

	// Add deployment files (Dockerfile, docker-compose - shows actual deployment topology)
	if len(ctx.DeploymentInfo) > 0 {
		sb.WriteString("## Deployment Configuration (GROUND TRUTH for service topology):\n")
		sb.WriteString("These files show what's actually deployed as separate services vs embedded:\n")
		for name, content := range ctx.DeploymentInfo {
			sb.WriteString(fmt.Sprintf("### %s:\n```\n%s\n```\n\n", name, content))
		}
	}

	// Add entry points (main.go, cmd/*.go - shows initialization patterns)
	if len(ctx.EntryPoints) > 0 {
		sb.WriteString("## Entry Points (initialization code - what's actually started and how):\n")
		for name, content := range ctx.EntryPoints {
			sb.WriteString(fmt.Sprintf("### %s:\n```go\n%s\n```\n\n", name, content))
		}
	}

	// Add config files (.env.example, vite.config, Makefile - framework/external deps)
	if len(ctx.ConfigFiles) > 0 {
		sb.WriteString("## Config Files (framework choices, external dependencies, build patterns):\n")
		for name, content := range ctx.ConfigFiles {
			sb.WriteString(fmt.Sprintf("### %s:\n```\n%s\n```\n\n", name, content))
		}
	}

	// Add import graph (Go internal package relationships)
	if ctx.ImportGraph != "" {
		sb.WriteString("## Internal Package Dependencies (Go import analysis):\n")
		sb.WriteString("```\n")
		sb.WriteString(ctx.ImportGraph)
		sb.WriteString("```\n\n")
	}

	// Add dynamically discovered files (language-agnostic high-value files)
	if len(ctx.DiscoveredFiles) > 0 {
		sb.WriteString("## Discovered Project Files (auto-detected by pattern matching):\n")
		for name, content := range ctx.DiscoveredFiles {
			sb.WriteString(fmt.Sprintf("### %s:\n```\n%s\n```\n\n", name, content))
		}
	}

	if ctx.GitSummary != "" {
		sb.WriteString("## Git History (summary - may include OUTDATED technologies):\n```\n")
		sb.WriteString(ctx.GitSummary)
		sb.WriteString("\n```\n\n")
	}

	sb.WriteString("\n---\nAnalyze this project and respond with JSON only:")

	return sb.String()
}

// callLLMStreaming uses Eino ChatModel.Stream() for streaming responses
func (a *LLMAnalyzer) callLLMStreaming(ctx context.Context, prompt string) (*LLMResponse, error) {
	// Build message
	messages := []*schema.Message{
		schema.UserMessage(prompt),
	}

	// Stream response using Eino
	stream, err := a.ChatModel.Stream(ctx, messages)
	if err != nil {
		return nil, fmt.Errorf("stream: %w", err)
	}
	defer stream.Close()

	// Collect streamed content
	var content strings.Builder
	progressOut := os.Stderr
	chunkCount := 0
	startTime := time.Now()
	progressChars := []string{"░", "▒", "▓", "█"}

	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("recv stream: %w", err)
		}
		if msg.Content != "" {
			content.WriteString(msg.Content)
			chunkCount++
			if chunkCount%30 == 0 {
				// Animated progress indicator
				idx := (chunkCount / 30) % len(progressChars)
				fmt.Fprintf(progressOut, "\r   🧠 Analyzing with %s... %s %d tokens", a.Model, progressChars[idx], chunkCount)
			}
		}
	}
	elapsed := time.Since(startTime)
	fmt.Fprintf(progressOut, "\r   ✓ Analysis complete: %d tokens in %.1fs\n\n", chunkCount, elapsed.Seconds())

	result := content.String()
	if result == "" {
		return nil, fmt.Errorf("empty response from LLM")
	}

	// Clean up response (remove markdown code blocks if present)
	result = strings.TrimPrefix(result, "```json")
	result = strings.TrimPrefix(result, "```")
	result = strings.TrimSuffix(result, "```")
	result = strings.TrimSpace(result)

	var llmResp LLMResponse
	if err := json.Unmarshal([]byte(result), &llmResp); err != nil {
		// Show first 500 chars of content for debugging
		preview := result
		if len(preview) > 500 {
			preview = preview[:500] + "..."
		}
		return nil, fmt.Errorf("parse LLM JSON: %w\nContent preview: %s", err, preview)
	}

	return &llmResp, nil
}

// PrintAnalysis outputs the LLM analysis in human-readable format
func (r *LLMResponse) PrintAnalysis() {
	fmt.Println("\n🤖 LLM Architecture Analysis")
	fmt.Println("═══════════════════════════════════════════════════════")

	for i, f := range r.Features {
		fmt.Printf("\n## %d. %s\n", i+1, f.Name)
		fmt.Printf("   %s\n", f.Description)
		fmt.Printf("\n   **Why it exists:**\n   %s\n", f.Purpose)

		if len(f.Decisions) > 0 {
			fmt.Printf("\n   **Decisions:**\n")
			for _, d := range f.Decisions {
				fmt.Printf("   • %s [%s]\n", d.Title, d.Confidence)
				fmt.Printf("     Why: %s\n", d.Why)
				if d.Tradeoffs != "" {
					fmt.Printf("     Trade-offs: %s\n", d.Tradeoffs)
				}
			}
		}

		// Print relationships
		if len(f.DependsOn) > 0 {
			fmt.Printf("\n   **Depends On:** %s\n", strings.Join(f.DependsOn, ", "))
		}
		if len(f.Extends) > 0 {
			fmt.Printf("   **Extends:** %s\n", strings.Join(f.Extends, ", "))
		}
		if len(f.RelatedTo) > 0 {
			fmt.Printf("   **Related To:** %s\n", strings.Join(f.RelatedTo, ", "))
		}
	}
	fmt.Println("\n═══════════════════════════════════════════════════════")
}

// parsePackageJSONDeps extracts dependency names from package.json content
func parsePackageJSONDeps(content string) []string {
	var pkg struct {
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	}
	if err := json.Unmarshal([]byte(content), &pkg); err != nil {
		return nil
	}
	var deps []string
	// Only include production dependencies for clarity (devDeps are often noise)
	for name := range pkg.Dependencies {
		deps = append(deps, name)
	}
	// Sort for consistent output
	sort.Strings(deps)
	return deps
}

// parseGoModDeps extracts module names from go.mod content
func parseGoModDeps(content string) []string {
	var deps []string
	lines := strings.Split(content, "\n")
	inRequire := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "require (" {
			inRequire = true
			continue
		}
		if inRequire {
			if line == ")" {
				break
			}
			// Skip indirect dependencies
			if strings.Contains(line, "// indirect") {
				continue
			}
			parts := strings.Fields(line)
			if len(parts) >= 1 {
				// Shorten common paths for readability
				name := parts[0]
				if strings.Contains(name, "github.com/") {
					// Keep just the package name part
					segments := strings.Split(name, "/")
					if len(segments) >= 3 {
						name = strings.Join(segments[1:], "/")
					}
				}
				deps = append(deps, name)
			}
		}
	}
	return deps
}

// parseGoImports extracts internal package import relationships from Go files
func (a *LLMAnalyzer) parseGoImports() string {
	// Find go.mod to get module name
	goModPath := filepath.Join(a.BasePath, "go.mod")
	content, err := os.ReadFile(goModPath)
	if err != nil {
		// Check backend-go subdirectory
		goModPath = filepath.Join(a.BasePath, "backend-go", "go.mod")
		content, err = os.ReadFile(goModPath)
		if err != nil {
			return ""
		}
	}

	// Extract module name
	var moduleName string
	for _, line := range strings.Split(string(content), "\n") {
		if strings.HasPrefix(line, "module ") {
			moduleName = strings.TrimPrefix(line, "module ")
			moduleName = strings.TrimSpace(moduleName)
			break
		}
	}
	if moduleName == "" {
		return ""
	}

	// Map package -> imports (internal only)
	pkgImports := make(map[string][]string)
	basePath := a.BasePath
	if strings.Contains(goModPath, "backend-go") {
		basePath = filepath.Join(a.BasePath, "backend-go")
	}

	// Walk internal/ or pkg/ directories
	for _, dir := range []string{"internal", "pkg", "cmd"} {
		dirPath := filepath.Join(basePath, dir)
		filepath.WalkDir(dirPath, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			if !strings.HasSuffix(d.Name(), ".go") || strings.HasSuffix(d.Name(), "_test.go") {
				return nil
			}

			goContent, err := os.ReadFile(path)
			if err != nil {
				return nil
			}

			// Get relative package path
			relPath, _ := filepath.Rel(basePath, filepath.Dir(path))
			pkgPath := relPath

			// Parse imports
			inImportBlock := false
			for _, line := range strings.Split(string(goContent), "\n") {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "import (") {
					inImportBlock = true
					continue
				}
				if inImportBlock && line == ")" {
					inImportBlock = false
					continue
				}
				if inImportBlock || strings.HasPrefix(line, "import \"") {
					// Extract import path
					importPath := ""
					if strings.Contains(line, "\"") {
						parts := strings.Split(line, "\"")
						if len(parts) >= 2 {
							importPath = parts[1]
						}
					}
					// Only keep internal imports
					if strings.HasPrefix(importPath, moduleName+"/") {
						shortPath := strings.TrimPrefix(importPath, moduleName+"/")
						if shortPath != pkgPath {
							pkgImports[pkgPath] = append(pkgImports[pkgPath], shortPath)
						}
					}
				}
			}
			return nil
		})
	}

	if len(pkgImports) == 0 {
		return ""
	}

	// Format as readable summary
	var sb strings.Builder
	sb.WriteString("Internal package dependencies:\n")
	for pkg, imports := range pkgImports {
		if len(imports) > 0 {
			// Deduplicate
			seen := make(map[string]bool)
			unique := []string{}
			for _, imp := range imports {
				if !seen[imp] {
					seen[imp] = true
					unique = append(unique, imp)
				}
			}
			sb.WriteString(fmt.Sprintf("  %s → %s\n", pkg, strings.Join(unique, ", ")))
		}
	}
	return sb.String()
}
