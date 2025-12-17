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
	DirectoryTree string
	PackageFiles  map[string]string // package.json, go.mod, etc.
	ReadmeContent string
	GitSummary    string
}

func (a *LLMAnalyzer) gatherContext() (*ProjectContext, error) {
	ctx := &ProjectContext{
		PackageFiles: make(map[string]string),
	}

	out := os.Stderr

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

	// 2. Read key config files (limit size)
	fmt.Fprint(out, "   📦 Reading package files...")
	configFiles := []string{
		"package.json", "go.mod", "Cargo.toml", "pyproject.toml",
	}
	foundConfigs := []string{}
	for _, f := range configFiles {
		path := filepath.Join(a.BasePath, f)
		if content, err := os.ReadFile(path); err == nil {
			// Truncate to 1000 chars
			if len(content) > 1000 {
				content = content[:1000]
			}
			ctx.PackageFiles[f] = string(content)
			foundConfigs = append(foundConfigs, f)
		}
	}
	if len(foundConfigs) > 0 {
		fmt.Fprintf(out, " %s\n", strings.Join(foundConfigs, ", "))
	} else {
		fmt.Fprintln(out, " none found")
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
		sb.WriteString("- Recent conventional commits (sample):\n")
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

	for name, content := range ctx.PackageFiles {
		sb.WriteString(fmt.Sprintf("## %s:\n```\n%s\n```\n\n", name, content))
	}

	if ctx.GitSummary != "" {
		sb.WriteString("## Git History (summary):\n```\n")
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
	fmt.Fprint(progressOut, "   Streaming: ")
	chunkCount := 0

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
			if chunkCount%50 == 0 {
				fmt.Fprint(progressOut, ".")
			}
		}
	}

	fmt.Fprintf(progressOut, " (%d chunks)\n\n", chunkCount)

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
