package bootstrap

import (
	"fmt"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"
)

// GitStats contains deterministic git statistics extracted without LLM.
type GitStats struct {
	TotalCommits     int               `json:"total_commits"`
	FirstCommitDate  time.Time         `json:"first_commit_date"`
	LastCommitDate   time.Time         `json:"last_commit_date"`
	ProjectAgeMonths int               `json:"project_age_months"`
	Contributors     []Contributor     `json:"contributors"`
	CommitsByType    map[string]int    `json:"commits_by_type"`
	MostChangedFiles []FileChangeCount `json:"most_changed_files"`
	RecentActivity   []MonthlyActivity `json:"recent_activity"`
}

// Contributor represents a git contributor with commit count.
type Contributor struct {
	Name    string `json:"name"`
	Commits int    `json:"commits"`
}

// FileChangeCount represents how often a file was changed.
type FileChangeCount struct {
	Path    string `json:"path"`
	Changes int    `json:"changes"`
}

// MonthlyActivity represents commit activity for a month.
type MonthlyActivity struct {
	Month   string `json:"month"` // YYYY-MM format
	Commits int    `json:"commits"`
}

// GitStatParser extracts deterministic git statistics without using LLM.
type GitStatParser struct {
	basePath string
}

// NewGitStatParser creates a new git stats parser.
func NewGitStatParser(basePath string) *GitStatParser {
	return &GitStatParser{basePath: basePath}
}

// Parse extracts git statistics from the repository.
func (p *GitStatParser) Parse() (*GitStats, error) {
	stats := &GitStats{
		CommitsByType: make(map[string]int),
	}

	// Check if this is a git repo
	if !p.isGitRepo() {
		return nil, fmt.Errorf("not a git repository")
	}

	// Get total commit count
	if count, err := p.getCommitCount(); err == nil {
		stats.TotalCommits = count
	}

	// Get first and last commit dates
	if first, last, err := p.getCommitDateRange(); err == nil {
		stats.FirstCommitDate = first
		stats.LastCommitDate = last
		stats.ProjectAgeMonths = int(last.Sub(first).Hours() / 24 / 30)
	}

	// Get contributors
	if contributors, err := p.getContributors(); err == nil {
		stats.Contributors = contributors
	}

	// Get commit type distribution (conventional commits)
	if types, err := p.getCommitTypes(); err == nil {
		stats.CommitsByType = types
	}

	// Get most changed files
	if files, err := p.getMostChangedFiles(); err == nil {
		stats.MostChangedFiles = files
	}

	// Get recent monthly activity
	if activity, err := p.getMonthlyActivity(); err == nil {
		stats.RecentActivity = activity
	}

	return stats, nil
}

func (p *GitStatParser) isGitRepo() bool {
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	cmd.Dir = p.basePath
	out, err := cmd.Output()
	return err == nil && strings.TrimSpace(string(out)) == "true"
}

func (p *GitStatParser) getCommitCount() (int, error) {
	cmd := exec.Command("git", "rev-list", "--count", "HEAD")
	cmd.Dir = p.basePath
	out, err := cmd.Output()
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(out)))
}

func (p *GitStatParser) getCommitDateRange() (time.Time, time.Time, error) {
	var first, last time.Time

	// First commit
	cmd := exec.Command("git", "log", "--reverse", "--format=%aI", "-1")
	cmd.Dir = p.basePath
	out, err := cmd.Output()
	if err != nil {
		return first, last, err
	}
	first, err = time.Parse(time.RFC3339, strings.TrimSpace(string(out)))
	if err != nil {
		return first, last, err
	}

	// Last commit
	cmd = exec.Command("git", "log", "--format=%aI", "-1")
	cmd.Dir = p.basePath
	out, err = cmd.Output()
	if err != nil {
		return first, last, err
	}
	last, err = time.Parse(time.RFC3339, strings.TrimSpace(string(out)))
	if err != nil {
		return first, last, err
	}

	return first, last, nil
}

func (p *GitStatParser) getContributors() ([]Contributor, error) {
	cmd := exec.Command("git", "shortlog", "-sn", "--all", "--no-merges")
	cmd.Dir = p.basePath
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var contributors []Contributor
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Format: "  123\tAuthor Name"
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) != 2 {
			continue
		}
		count, err := strconv.Atoi(strings.TrimSpace(parts[0]))
		if err != nil {
			continue
		}
		contributors = append(contributors, Contributor{
			Name:    strings.TrimSpace(parts[1]),
			Commits: count,
		})
		// Limit to top 10
		if len(contributors) >= 10 {
			break
		}
	}
	return contributors, nil
}

func (p *GitStatParser) getCommitTypes() (map[string]int, error) {
	// Get recent commit messages to classify by conventional commit type
	cmd := exec.Command("git", "log", "--format=%s", "-500")
	cmd.Dir = p.basePath
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	types := make(map[string]int)
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		commitType := classifyCommitMessage(line)
		types[commitType]++
	}
	return types, nil
}

func classifyCommitMessage(msg string) string {
	msg = strings.ToLower(msg)

	// Conventional commit prefixes
	prefixes := []struct {
		prefix string
		label  string
	}{
		{"feat:", "feature"},
		{"feat(", "feature"},
		{"fix:", "bugfix"},
		{"fix(", "bugfix"},
		{"docs:", "docs"},
		{"docs(", "docs"},
		{"test:", "test"},
		{"test(", "test"},
		{"refactor:", "refactor"},
		{"refactor(", "refactor"},
		{"chore:", "chore"},
		{"chore(", "chore"},
		{"ci:", "ci"},
		{"ci(", "ci"},
		{"build:", "build"},
		{"build(", "build"},
		{"perf:", "performance"},
		{"perf(", "performance"},
		{"style:", "style"},
		{"style(", "style"},
	}

	for _, p := range prefixes {
		if strings.HasPrefix(msg, p.prefix) {
			return p.label
		}
	}

	// Fallback heuristics
	if strings.Contains(msg, "merge") {
		return "merge"
	}
	if strings.Contains(msg, "fix") || strings.Contains(msg, "bug") {
		return "bugfix"
	}
	if strings.Contains(msg, "add") || strings.Contains(msg, "implement") {
		return "feature"
	}
	if strings.Contains(msg, "update") || strings.Contains(msg, "change") {
		return "update"
	}
	if strings.Contains(msg, "remove") || strings.Contains(msg, "delete") {
		return "removal"
	}
	if strings.Contains(msg, "refactor") || strings.Contains(msg, "clean") {
		return "refactor"
	}

	return "other"
}

func (p *GitStatParser) getMostChangedFiles() ([]FileChangeCount, error) {
	// Get files sorted by number of commits that touched them
	cmd := exec.Command("git", "log", "--format=", "--name-only", "-500")
	cmd.Dir = p.basePath
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	fileCounts := make(map[string]int)
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fileCounts[line]++
	}

	// Sort by count
	type kv struct {
		Key   string
		Value int
	}
	var sorted []kv
	for k, v := range fileCounts {
		sorted = append(sorted, kv{k, v})
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Value > sorted[j].Value
	})

	// Return top 20
	var result []FileChangeCount
	for i, kv := range sorted {
		if i >= 20 {
			break
		}
		result = append(result, FileChangeCount{
			Path:    kv.Key,
			Changes: kv.Value,
		})
	}
	return result, nil
}

func (p *GitStatParser) getMonthlyActivity() ([]MonthlyActivity, error) {
	// Get commit dates for the last 12 months
	cmd := exec.Command("git", "log", "--format=%aI", "--since=12 months ago")
	cmd.Dir = p.basePath
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	monthCounts := make(map[string]int)
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		t, err := time.Parse(time.RFC3339, line)
		if err != nil {
			continue
		}
		month := t.Format("2006-01")
		monthCounts[month]++
	}

	// Sort months
	var months []string
	for m := range monthCounts {
		months = append(months, m)
	}
	sort.Strings(months)

	var result []MonthlyActivity
	for _, m := range months {
		result = append(result, MonthlyActivity{
			Month:   m,
			Commits: monthCounts[m],
		})
	}
	return result, nil
}

// ToMarkdown converts git stats to a markdown summary for storage.
func (s *GitStats) ToMarkdown() string {
	var sb strings.Builder

	sb.WriteString("# Git Statistics\n\n")
	sb.WriteString(fmt.Sprintf("- **Total Commits**: %d\n", s.TotalCommits))
	sb.WriteString(fmt.Sprintf("- **Project Age**: %d months\n", s.ProjectAgeMonths))

	if !s.FirstCommitDate.IsZero() {
		sb.WriteString(fmt.Sprintf("- **First Commit**: %s\n", s.FirstCommitDate.Format("2006-01-02")))
	}
	if !s.LastCommitDate.IsZero() {
		sb.WriteString(fmt.Sprintf("- **Last Commit**: %s\n", s.LastCommitDate.Format("2006-01-02")))
	}

	if len(s.Contributors) > 0 {
		sb.WriteString("\n## Top Contributors\n\n")
		for _, c := range s.Contributors {
			sb.WriteString(fmt.Sprintf("- %s (%d commits)\n", c.Name, c.Commits))
		}
	}

	if len(s.CommitsByType) > 0 {
		sb.WriteString("\n## Commit Types\n\n")
		for t, count := range s.CommitsByType {
			sb.WriteString(fmt.Sprintf("- %s: %d\n", t, count))
		}
	}

	if len(s.MostChangedFiles) > 0 {
		sb.WriteString("\n## Most Changed Files\n\n")
		for _, f := range s.MostChangedFiles[:min(10, len(s.MostChangedFiles))] {
			sb.WriteString(fmt.Sprintf("- %s (%d changes)\n", f.Path, f.Changes))
		}
	}

	return sb.String()
}
