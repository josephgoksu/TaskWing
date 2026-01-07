package bootstrap

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/josephgoksu/TaskWing/internal/agents/core"
	"github.com/josephgoksu/TaskWing/internal/config"
	"github.com/josephgoksu/TaskWing/internal/knowledge"
	"github.com/josephgoksu/TaskWing/internal/llm"
	"github.com/josephgoksu/TaskWing/internal/memory"
	"github.com/josephgoksu/TaskWing/internal/workspace"
)

// Service handles the bootstrapping process of extracting architectural knowledge.
// It orchestrates analysis agents, result aggregation, and storage ingestion.
type Service struct {
	basePath    string
	llmCfg      llm.Config
	initializer *Initializer
}

// NewService creates a new bootstrap service.
func NewService(basePath string, llmCfg llm.Config) *Service {
	return &Service{
		basePath:    basePath,
		llmCfg:      llmCfg,
		initializer: NewInitializer(basePath),
	}
}

// InitializeProject sets up the .taskwing directory structure and integrations.
func (s *Service) InitializeProject(verbose bool, selectedAIs []string) error {
	return s.initializer.Run(verbose, selectedAIs)
}

// RunAgentAnalysis executes the analysis agents for a single project/service.
// It returns raw results which can be used for UI streaming or batch processing.
// Note: UI orchestration (TUI) remains in the CLI layer, but this method supports headless execution.
func (s *Service) RunAgentAnalysis(ctx context.Context, projectPath string) ([]core.Output, error) {
	runner := NewRunner(s.llmCfg, projectPath)
	defer runner.Close()

	return runner.Run(ctx, projectPath)
}

// RunMultiRepoAnalysis executes analysis for all services in a workspace.
func (s *Service) RunMultiRepoAnalysis(ctx context.Context, ws *workspace.Info) ([]core.Finding, []core.Relationship, []string, error) {
	var allFindings []core.Finding
	var allRelationships []core.Relationship
	var serviceErrors []string

	for _, serviceName := range ws.Services {
		servicePath := ws.GetServicePath(serviceName)
		runner := NewRunner(s.llmCfg, servicePath)
		defer runner.Close()

		results, err := runner.Run(ctx, servicePath)
		if err != nil {
			serviceErrors = append(serviceErrors, fmt.Sprintf("%s: %s", serviceName, err.Error()))
			continue
		}

		// Prefix findings with service name for disambiguation
		findings := core.AggregateFindings(results)
		for i := range findings {
			findings[i].Title = fmt.Sprintf("[%s] %s", serviceName, findings[i].Title)
			if findings[i].Metadata == nil {
				findings[i].Metadata = make(map[string]any)
			}
			findings[i].Metadata["service"] = serviceName
		}

		relationships := core.AggregateRelationships(results)
		for i := range relationships {
			relationships[i].From = fmt.Sprintf("[%s] %s", serviceName, relationships[i].From)
			relationships[i].To = fmt.Sprintf("[%s] %s", serviceName, relationships[i].To)
		}

		allFindings = append(allFindings, findings...)
		allRelationships = append(allRelationships, relationships...)
	}

	return allFindings, allRelationships, serviceErrors, nil
}

// ProcessAndSaveResults aggregates, reports, and ingests findings into the knowledge system.
func (s *Service) ProcessAndSaveResults(ctx context.Context, results []core.Output, findings []core.Finding, relationships []core.Relationship, isPreview, isQuiet bool) error {
	// 1. Generate and save report
	report := generateReport(s.basePath, results, findings)
	reportPath := filepath.Join(s.basePath, ".taskwing", "last-bootstrap-report.json")
	if err := saveReport(reportPath, report); err != nil {
		// Non-fatal warning
		fmt.Fprintf(os.Stderr, "⚠️  Failed to save bootstrap report: %v\n", err)
	}

	// 2. Print summary (could serve as return value if we want pure separation, but fine here for CLI svc)
	printCoverageSummary(report)

	if isPreview {
		fmt.Println("\n💡 This was a preview. Run 'taskwing bootstrap' to save to memory.")
		return nil
	}

	// 3. Ingest into Knowledge System
	return s.ingestToMemory(ctx, findings, relationships, isQuiet)
}

// IngestDirectly ingests pre-aggregated findings (used by Multi-repo mode).
func (s *Service) IngestDirectly(ctx context.Context, findings []core.Finding, relationships []core.Relationship, isQuiet bool) error {
	return s.ingestToMemory(ctx, findings, relationships, isQuiet)
}

func (s *Service) ingestToMemory(ctx context.Context, findings []core.Finding, relationships []core.Relationship, isQuiet bool) error {
	memoryPath := config.GetMemoryBasePath()

	// Create Repo
	// Note: We're creating a new connection here. Ideally, connection pooling or shared instances would be better.
	repo, err := memory.NewDefaultRepository(memoryPath)
	if err != nil {
		return fmt.Errorf("open memory repo: %w", err)
	}
	defer func() { _ = repo.Close() }()

	// Create Knowledge Service
	ks := knowledge.NewService(repo, s.llmCfg)
	ks.SetBasePath(s.basePath)

	// Ingest
	if err := ks.IngestFindingsWithRelationships(ctx, findings, relationships, nil, !isQuiet); err != nil {
		return err
	}

	// Generate Project Overview if needed
	return s.generateOverviewIfNeeded(ctx, repo, !isQuiet)
}

// generateOverviewIfNeeded checks existence and generates overview
func (s *Service) generateOverviewIfNeeded(ctx context.Context, repo *memory.Repository, verbose bool) error {
	existing, err := repo.GetProjectOverview()
	if err != nil {
		return fmt.Errorf("check overview: %w", err)
	}
	if existing != nil {
		if verbose {
			fmt.Println("\n📋 Project overview already exists (use 'tw overview regenerate' to update)")
		}
		return nil
	}

	if verbose {
		fmt.Println("\n📋 Generating project overview...")
	}

	analyzer := NewOverviewAnalyzer(s.llmCfg, s.basePath)
	overview, err := analyzer.Analyze(ctx)
	if err != nil {
		return fmt.Errorf("analyze project: %w", err)
	}

	if err := repo.SaveProjectOverview(overview); err != nil {
		return fmt.Errorf("save overview: %w", err)
	}

	if verbose {
		fmt.Println("   ✓ Project overview generated")
		fmt.Printf("   \"%s\"\n", overview.ShortDescription)
	}
	return nil
}

// --- Internal Helper Functions ---

func generateReport(projectPath string, results []core.Output, findings []core.Finding) *core.BootstrapReport {
	report := core.NewBootstrapReport(projectPath)

	var totalDuration time.Duration
	for _, result := range results {
		agentReport := core.AgentReport{
			Name:         result.AgentName,
			Duration:     result.Duration,
			TokensUsed:   result.TokensUsed,
			FindingCount: len(result.Findings),
			Coverage:     result.Coverage,
		}
		if result.Error != nil {
			agentReport.Error = result.Error.Error()
		}
		report.AddAgentReport(result.AgentName, agentReport)
		totalDuration += result.Duration
	}

	report.Finalize(findings, totalDuration)
	return report
}

func saveReport(path string, report *core.BootstrapReport) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create report directory: %w", err)
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal report: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

func printCoverageSummary(report *core.BootstrapReport) {
	fmt.Println()
	fmt.Println("📊 Bootstrap Coverage Report")
	fmt.Println("────────────────────────────")
	fmt.Printf("   Files analyzed: %d\n", report.Coverage.FilesAnalyzed)
	fmt.Printf("   Files skipped:  %d\n", report.Coverage.FilesSkipped)
	fmt.Printf("   Coverage:       %.1f%%\n", report.Coverage.CoveragePercent)
	fmt.Printf("   Total findings: %d\n", report.TotalFindings)

	if len(report.FindingCounts) > 0 {
		fmt.Println()
		fmt.Println("   Findings by type:")
		for fType, count := range report.FindingCounts {
			fmt.Printf("     • %s: %d\n", fType, count)
		}
	}

	fmt.Println()
	fmt.Println("   Per-agent coverage:")
	for name, ar := range report.AgentReports {
		status := "✓"
		if ar.Error != "" {
			status = "✗"
		}
		fileWord := "files"
		if ar.Coverage.FilesAnalyzed == 1 {
			fileWord = "file"
		}
		findingWord := "findings"
		if ar.FindingCount == 1 {
			findingWord = "finding"
		}
		fmt.Printf("     %s %s: %d %s, %d %s\n", status, name, ar.Coverage.FilesAnalyzed, fileWord, ar.FindingCount, findingWord)
	}

	fmt.Println()
	fmt.Printf("📄 Full report: .taskwing/last-bootstrap-report.json\n")
}
