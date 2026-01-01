/*
Package core provides BootstrapReport for visibility into what was analyzed.
*/
package core

import "time"

// BootstrapReport provides visibility into what was analyzed during bootstrap.
// This enables users to understand coverage and debug missing context.
type BootstrapReport struct {
	Timestamp     time.Time              `json:"timestamp"`
	ProjectPath   string                 `json:"project_path"`
	Duration      time.Duration          `json:"duration"`
	Coverage      CoverageStats          `json:"coverage"`
	AgentReports  map[string]AgentReport `json:"agent_reports"`
	FindingCounts map[string]int         `json:"finding_counts"` // By type
	TotalFindings int                    `json:"total_findings"`
}

// CoverageStats tracks what was and wasn't analyzed.
type CoverageStats struct {
	FilesAnalyzed   int           `json:"files_analyzed"`
	FilesSkipped    int           `json:"files_skipped"`
	TotalFiles      int           `json:"total_files"`
	CoveragePercent float64       `json:"coverage_percent"`
	CharactersRead  int           `json:"characters_read"`
	FilesRead       []FileRead    `json:"files_read"`
	FilesSkippedLog []SkippedFile `json:"files_skipped_log"`
}

// FileRead records a file that was analyzed.
type FileRead struct {
	Path       string `json:"path"`
	Characters int    `json:"characters"`
	Lines      int    `json:"lines"`
	Truncated  bool   `json:"truncated"`
}

// SkippedFile records a file that was skipped with reason.
type SkippedFile struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

// AgentReport captures per-agent analysis details.
type AgentReport struct {
	Name         string        `json:"name"`
	Duration     time.Duration `json:"duration"`
	TokensUsed   int           `json:"tokens_used"`
	FindingCount int           `json:"finding_count"`
	Coverage     CoverageStats `json:"coverage"`
	Error        string        `json:"error,omitempty"`
}

// NewBootstrapReport creates a new report initialized with timestamp.
func NewBootstrapReport(projectPath string) *BootstrapReport {
	return &BootstrapReport{
		Timestamp:     time.Now(),
		ProjectPath:   projectPath,
		AgentReports:  make(map[string]AgentReport),
		FindingCounts: make(map[string]int),
	}
}

// AddAgentReport adds an agent's report to the bootstrap report.
func (r *BootstrapReport) AddAgentReport(name string, report AgentReport) {
	r.AgentReports[name] = report
}

// Finalize calculates aggregate stats after all agents complete.
func (r *BootstrapReport) Finalize(findings []Finding, duration time.Duration) {
	r.Duration = duration
	r.TotalFindings = len(findings)

	// Count findings by type
	for _, f := range findings {
		r.FindingCounts[string(f.Type)]++
	}

	// Aggregate coverage from all agents
	seenFiles := make(map[string]bool)
	for _, ar := range r.AgentReports {
		for _, fr := range ar.Coverage.FilesRead {
			if !seenFiles[fr.Path] {
				seenFiles[fr.Path] = true
				r.Coverage.FilesRead = append(r.Coverage.FilesRead, fr)
				r.Coverage.CharactersRead += fr.Characters
			}
		}
		r.Coverage.FilesSkippedLog = append(r.Coverage.FilesSkippedLog, ar.Coverage.FilesSkippedLog...)
	}
	r.Coverage.FilesAnalyzed = len(r.Coverage.FilesRead)
	r.Coverage.FilesSkipped = len(r.Coverage.FilesSkippedLog)
	r.Coverage.TotalFiles = r.Coverage.FilesAnalyzed + r.Coverage.FilesSkipped
	if r.Coverage.TotalFiles > 0 {
		r.Coverage.CoveragePercent = float64(r.Coverage.FilesAnalyzed) / float64(r.Coverage.TotalFiles) * 100
	}
}

// Summary returns a human-readable summary for CLI output.
func (r *BootstrapReport) Summary() string {
	return ""
}
