package bootstrap

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// DetectedFeature represents a feature found during scanning
type DetectedFeature struct {
	Name      string
	Path      string
	Source    string // "directory", "git"
	OneLiner  string
	FileCount int
}

// DetectedDecision represents a decision found during scanning
type DetectedDecision struct {
	Feature   string
	Title     string
	Reasoning string
	Source    string // "git_commit", "adr"
	Date      time.Time
	CommitSHA string
}

// ScanResult holds all detected features and decisions
type ScanResult struct {
	Features  []DetectedFeature
	Decisions []DetectedDecision
	Errors    []string
}

// Scanner scans a repository for features and decisions
type Scanner struct {
	RootPath       string
	IgnorePatterns []string
}

// NewScanner creates a new bootstrap scanner
func NewScanner(rootPath string) *Scanner {
	return &Scanner{
		RootPath: rootPath,
		IgnorePatterns: []string{
			"node_modules", "vendor", ".git", ".taskwing",
			"dist", "build", "__pycache__", ".cache",
			"coverage", ".nyc_output", "tmp", "temp",
		},
	}
}

// Scan performs full repository scan
func (s *Scanner) Scan() (*ScanResult, error) {
	result := &ScanResult{}

	// 1. Scan directory structure for features
	features, err := s.scanDirectories()
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("directory scan: %v", err))
	}
	result.Features = features

	// 2. Scan git history for decisions
	decisions, err := s.scanGitHistory()
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("git scan: %v", err))
	}
	result.Decisions = decisions

	return result, nil
}

// scanDirectories detects features from folder structure
func (s *Scanner) scanDirectories() ([]DetectedFeature, error) {
	var features []DetectedFeature

	// Common feature-containing directories
	featureDirs := []string{
		"src", "lib", "pkg", "internal", "cmd",
		"app", "modules", "components", "services",
		"features", "domains",
	}

	for _, dir := range featureDirs {
		dirPath := filepath.Join(s.RootPath, dir)
		if _, err := os.Stat(dirPath); os.IsNotExist(err) {
			continue
		}

		entries, err := os.ReadDir(dirPath)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			if s.shouldIgnore(entry.Name()) {
				continue
			}

			featurePath := filepath.Join(dirPath, entry.Name())
			fileCount := s.countFiles(featurePath)

			if fileCount > 0 {
				features = append(features, DetectedFeature{
					Name:      s.formatName(entry.Name()),
					Path:      featurePath,
					Source:    "directory",
					OneLiner:  s.inferDescription(entry.Name()),
					FileCount: fileCount,
				})
			}
		}
	}

	return features, nil
}

// scanGitHistory extracts decisions from conventional commits
func (s *Scanner) scanGitHistory() ([]DetectedDecision, error) {
	var decisions []DetectedDecision

	// Check if git repo exists
	gitDir := filepath.Join(s.RootPath, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("not a git repository")
	}

	// Get git log with conventional commit format
	// Format: <sha>|<date>|<subject>
	cmd := exec.Command("git", "log", "--oneline", "--format=%H|%ai|%s", "-n", "200")
	cmd.Dir = s.RootPath
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git log: %w", err)
	}

	// Parse conventional commits
	// Pattern 1: feat(scope): description
	// Pattern 2: feat: description (no scope)
	conventionalWithScope := regexp.MustCompile(`^(feat|fix|refactor|perf|docs|style|test|chore|build|ci)\(([^)]+)\):\s*(.+)$`)
	conventionalNoScope := regexp.MustCompile(`^(feat|fix|refactor|perf):\s*(.+)$`)

	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, "|", 3)
		if len(parts) != 3 {
			continue
		}

		sha := parts[0][:8] // Short SHA
		dateStr := parts[1]
		subject := parts[2]

		var commitType, scope, description string

		// Try with scope first
		matches := conventionalWithScope.FindStringSubmatch(subject)
		if matches != nil {
			commitType = matches[1]
			scope = matches[2]
			description = matches[3]
		} else {
			// Try without scope
			matches = conventionalNoScope.FindStringSubmatch(subject)
			if matches != nil {
				commitType = matches[1]
				scope = "General"
				description = matches[2]
			}
		}

		if commitType == "" {
			continue
		}

		// Only extract meaningful decisions (feat, fix with reasoning)
		if commitType != "feat" && commitType != "fix" && commitType != "refactor" && commitType != "perf" {
			continue
		}

		date, _ := time.Parse("2006-01-02 15:04:05 -0700", dateStr)

		decisions = append(decisions, DetectedDecision{
			Feature:   s.formatName(scope),
			Title:     s.formatDecisionTitle(commitType, description),
			Reasoning: fmt.Sprintf("Commit %s: %s", sha, subject),
			Source:    "git_commit",
			Date:      date,
			CommitSHA: sha,
		})
	}

	return decisions, nil
}

// Helper methods

func (s *Scanner) shouldIgnore(name string) bool {
	for _, pattern := range s.IgnorePatterns {
		if strings.EqualFold(name, pattern) {
			return true
		}
		// Also ignore hidden directories
		if strings.HasPrefix(name, ".") {
			return true
		}
	}
	return false
}

func (s *Scanner) countFiles(dir string) int {
	count := 0
	filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() {
			count++
		}
		return nil
	})
	return count
}

func (s *Scanner) formatName(name string) string {
	// Convert kebab-case or snake_case to Title Case
	name = strings.ReplaceAll(name, "-", " ")
	name = strings.ReplaceAll(name, "_", " ")
	return strings.Title(strings.ToLower(name))
}

func (s *Scanner) inferDescription(name string) string {
	// Basic heuristics for common directory names
	lower := strings.ToLower(name)
	descriptions := map[string]string{
		"auth":       "Authentication and authorization",
		"user":       "User management",
		"users":      "User management",
		"api":        "API endpoints",
		"db":         "Database operations",
		"database":   "Database operations",
		"models":     "Data models",
		"utils":      "Utility functions",
		"helpers":    "Helper functions",
		"config":     "Configuration management",
		"middleware": "HTTP middleware",
		"routes":     "Route definitions",
		"handlers":   "Request handlers",
		"services":   "Business logic services",
		"core":       "Core functionality",
		"common":     "Shared utilities",
		"cmd":        "CLI commands",
		"pkg":        "Public packages",
		"internal":   "Internal packages",
		"memory":     "Memory management",
		"store":      "Data storage",
		"cache":      "Caching layer",
	}

	for key, desc := range descriptions {
		if strings.Contains(lower, key) {
			return desc
		}
	}

	return fmt.Sprintf("%s module", s.formatName(name))
}

func (s *Scanner) formatDecisionTitle(commitType, description string) string {
	switch commitType {
	case "feat":
		return fmt.Sprintf("Add: %s", description)
	case "fix":
		return fmt.Sprintf("Fix: %s", description)
	case "refactor":
		return fmt.Sprintf("Refactor: %s", description)
	case "perf":
		return fmt.Sprintf("Optimize: %s", description)
	default:
		return description
	}
}

// PrintPreview outputs a human-readable preview of the scan results
func (r *ScanResult) PrintPreview() {
	fmt.Println("\n🔍 Bootstrap Scan Preview")
	fmt.Println("═══════════════════════════════════════════════════════")

	if len(r.Features) > 0 {
		fmt.Printf("\n📁 Features Detected (%d)\n", len(r.Features))
		fmt.Println("─────────────────────────────────────────────────────")
		for _, f := range r.Features {
			fmt.Printf("  • %s\n", f.Name)
			fmt.Printf("    Path: %s\n", f.Path)
			fmt.Printf("    Files: %d\n", f.FileCount)
			fmt.Printf("    Description: %s\n", f.OneLiner)
			fmt.Println()
		}
	} else {
		fmt.Println("\n📁 No features detected from directory structure")
	}

	if len(r.Decisions) > 0 {
		fmt.Printf("\n📝 Decisions from Git History (%d)\n", len(r.Decisions))
		fmt.Println("─────────────────────────────────────────────────────")

		// Group by feature
		byFeature := make(map[string][]DetectedDecision)
		for _, d := range r.Decisions {
			byFeature[d.Feature] = append(byFeature[d.Feature], d)
		}

		for feature, decisions := range byFeature {
			fmt.Printf("\n  [%s]\n", feature)
			// Show max 5 per feature
			limit := 5
			if len(decisions) < limit {
				limit = len(decisions)
			}
			for i := 0; i < limit; i++ {
				d := decisions[i]
				fmt.Printf("    • %s\n", d.Title)
				fmt.Printf("      %s\n", d.Date.Format("2006-01-02"))
			}
			if len(decisions) > 5 {
				fmt.Printf("    ... and %d more\n", len(decisions)-5)
			}
		}
	} else {
		fmt.Println("\n📝 No decisions detected from git history")
		fmt.Println("    (Tip: Use conventional commits like 'feat(auth): add JWT support')")
	}

	if len(r.Errors) > 0 {
		fmt.Printf("\n⚠️  Warnings (%d)\n", len(r.Errors))
		for _, e := range r.Errors {
			fmt.Printf("  • %s\n", e)
		}
	}

	fmt.Println("\n═══════════════════════════════════════════════════════")
}
