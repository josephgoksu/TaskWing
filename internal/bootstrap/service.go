package bootstrap

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/josephgoksu/TaskWing/internal/agents/core"
	"github.com/josephgoksu/TaskWing/internal/config"
	"github.com/josephgoksu/TaskWing/internal/knowledge"
	"github.com/josephgoksu/TaskWing/internal/llm"
	"github.com/josephgoksu/TaskWing/internal/memory"
	"github.com/josephgoksu/TaskWing/internal/project"
	"github.com/josephgoksu/TaskWing/internal/ui"
)

// Service handles the bootstrapping process of extracting architectural knowledge.
// It orchestrates analysis agents, result aggregation, and storage ingestion.
type Service struct {
	basePath    string       // project root (for file scanning)
	storePath   string       // global store (~/.taskwing/projects/<slug>/)
	llmCfg      llm.Config
	initializer *Initializer
}

// BootstrapResult contains the outcome of a bootstrap operation including warnings.
type BootstrapResult struct {
	FindingsCount int      `json:"findings_count"`
	Warnings      []string `json:"warnings,omitempty"` // Non-fatal issues encountered
	Errors        []string `json:"errors,omitempty"`   // Fatal errors (if any)
}

// NewService creates a new bootstrap service.
func NewService(basePath, storePath string, llmCfg llm.Config) *Service {
	return &Service{
		basePath:    basePath,
		storePath:   storePath,
		llmCfg:      llmCfg,
		initializer: NewInitializer(basePath, storePath),
	}
}

// SetVersion sets the CLI version on the underlying initializer so that
// createStructure() stamps .taskwing/version for post-upgrade migration detection.
func (s *Service) SetVersion(v string) {
	s.initializer.Version = v
}

// InitializeProject sets up the .taskwing directory structure and integrations.
func (s *Service) InitializeProject(verbose bool, selectedAIs []string) error {
	return s.initializer.Run(verbose, selectedAIs)
}

// RegenerateAIConfigs regenerates AI slash commands and hooks for specified AIs.
// This is used in repair mode when the project structure is healthy but AI configs need repair.
func (s *Service) RegenerateAIConfigs(verbose bool, targetAIs []string) error {
	return s.initializer.RegenerateConfigs(verbose, targetAIs)
}

// ProgressFunc is called during multi-repo analysis with the service name and status.
type ProgressFunc func(serviceName string, status string)

// RunMultiRepoAnalysis executes analysis for all services in a workspace.
// Each service's findings are tagged with the service name as workspace.
// If onProgress is non-nil, it is called before and after each service analysis.
// NOTE: Not safe for concurrent use. Swaps global project context per-service.
func (s *Service) RunMultiRepoAnalysis(ctx context.Context, ws *project.WorkspaceInfo, onProgress ProgressFunc) ([]core.Finding, []core.Relationship, []string, error) {
	var allFindings []core.Finding
	var allRelationships []core.Relationship
	var serviceErrors []string

	// Save the workspace-level project context to restore after each service
	workspaceCtx := config.GetProjectContext()

	for i, serviceName := range ws.Services {
		servicePath := ws.GetServicePath(serviceName)

		if onProgress != nil {
			onProgress(serviceName, fmt.Sprintf("[%d/%d] analyzing...", i+1, len(ws.Services)))
		}

		// Set per-service project context so git agents get the correct scopePath
		if svcCtx, detectErr := project.Detect(servicePath); detectErr == nil {
			_ = config.SetProjectContext(svcCtx)
		}

		runner := NewRunner(s.llmCfg, servicePath)

		// Incremental mode: check if we can skip or limit analysis
		opts := RunOptions{Workspace: serviceName}
		stateKey := "bootstrap-sha-" + serviceName
		dbPath := s.storePath
		if store, storeErr := memory.NewSQLiteStore(dbPath); storeErr == nil {
			if state, stateErr := store.GetBootstrapState(stateKey); stateErr == nil && state != nil && state.Checksum != "" {
				headSHA := getGitHEAD(servicePath)
				if headSHA != "" && headSHA == state.Checksum {
					if onProgress != nil {
						onProgress(serviceName, fmt.Sprintf("[%d/%d] no changes", i+1, len(ws.Services)))
					}
					_ = store.Close()
					runner.Close()
					continue
				}
				if headSHA != "" {
					changedFiles := getChangedFilesSince(servicePath, state.Checksum)
					if changedFiles != nil && len(changedFiles) == 0 {
						if onProgress != nil {
							onProgress(serviceName, fmt.Sprintf("[%d/%d] no changes", i+1, len(ws.Services)))
						}
						_ = store.Close()
						runner.Close()
						continue
					}
					if changedFiles != nil && len(changedFiles) < 50 {
						opts.ChangedFiles = changedFiles
					}
				}
			}
			_ = store.Close()
		}

		// Pass workspace (service name) to the runner so agents can tag their findings
		results, err := runner.RunWithOptions(ctx, servicePath, opts)
		// Close runner immediately after use - NOT deferred in loop!
		runner.Close()

		// Restore workspace context after each service
		if workspaceCtx != nil {
			_ = config.SetProjectContext(workspaceCtx)
		}

		if err != nil {
			serviceErrors = append(serviceErrors, fmt.Sprintf("%s: %s", serviceName, err.Error()))
			continue
		}

		// Aggregate findings - workspace tagging happens at agent level via Input.Workspace
		// We still set metadata["service"] for backward compatibility with ingestion
		findings := core.AggregateFindings(results)

		// Make evidence paths workspace-relative so verification resolves correctly.
		// Evidence paths from agents are relative to servicePath, but verification
		// uses s.basePath (workspace root). Prefixing with the service directory
		// makes filepath.Join(workspaceRoot, "serviceDir/path") resolve correctly.
		serviceRelPath, relErr := filepath.Rel(s.basePath, servicePath)
		if relErr != nil {
			serviceErrors = append(serviceErrors, fmt.Sprintf("%s: compute relative path: %s", serviceName, relErr.Error()))
			continue
		}
		for i := range findings {
			for j := range findings[i].Evidence {
				ev := &findings[i].Evidence[j]
				if ev.FilePath != "" && !filepath.IsAbs(ev.FilePath) {
					ev.FilePath = filepath.Join(serviceRelPath, ev.FilePath)
				}
			}
		}

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

		// Save git SHA for incremental mode on next run
		if headSHA := getGitHEAD(servicePath); headSHA != "" {
			if store, storeErr := memory.NewSQLiteStore(dbPath); storeErr == nil {
				_ = store.SetBootstrapState(&memory.BootstrapState{
					Component: stateKey,
					Status:    "completed",
					Checksum:  headSHA,
				})
				_ = store.Close()
			}
		}

		if onProgress != nil {
			onProgress(serviceName, fmt.Sprintf("[%d/%d] done (%d findings)", i+1, len(ws.Services), len(findings)))
		}
	}

	return allFindings, allRelationships, serviceErrors, nil
}

// ProcessAndSaveResults aggregates, reports, and ingests findings into the knowledge system.
func (s *Service) ProcessAndSaveResults(ctx context.Context, results []core.Output, findings []core.Finding, relationships []core.Relationship, isPreview, isQuiet bool) error {
	// 1. Generate and save report
	report := generateReport(s.basePath, results, findings)
	reportPath := filepath.Join(s.storePath, "last-bootstrap-report.json")
	if err := saveReport(reportPath, report); err != nil {
		// Non-fatal warning
		fmt.Fprintf(os.Stderr, "⚠️  Failed to save bootstrap report: %v\n", err)
	}

	// 2. Print summary using consistent UI renderer
	ui.RenderBootstrapResults(report)

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
	memoryPath, err := config.GetMemoryBasePath()
	if err != nil {
		return fmt.Errorf("get memory path: %w", err)
	}

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
	if err := s.generateOverviewIfNeeded(ctx, repo, !isQuiet); err != nil {
		return err // Non-fatal? Maybe, but consistent with other errors
	}

	// Generate ARCHITECTURE.md
	projectName := filepath.Base(s.basePath)
	if err := repo.GenerateArchitectureMD(projectName); err != nil {
		// Log warning but don't fail bootstrap
		fmt.Fprintf(os.Stderr, "⚠️  Failed to generate ARCHITECTURE.md: %v\n", err)
	} else if !isQuiet {
		fmt.Println("   ✓ Generated .taskwing/ARCHITECTURE.md")
	}

	return nil
}

// generateOverviewIfNeeded checks existence and generates overview
func (s *Service) generateOverviewIfNeeded(ctx context.Context, repo *memory.Repository, verbose bool) error {
	existing, err := repo.GetProjectOverview()
	if err != nil {
		return fmt.Errorf("check overview: %w", err)
	}
	if existing != nil {
		if verbose {
			fmt.Println("\n📋 Project overview already exists (re-run bootstrap with --force to refresh)")
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

// RunDeterministicBootstrap collects project metadata without LLM calls.
// This is the default bootstrap mode - fast, reliable, and always succeeds.
// It extracts: git statistics, documentation files, and stores them for RAG.
// Returns a BootstrapResult with warnings for any non-fatal issues encountered.
func (s *Service) RunDeterministicBootstrap(ctx context.Context, isQuiet bool) (*BootstrapResult, error) {
	result := &BootstrapResult{}

	memoryPath, err := config.GetMemoryBasePath()
	if err != nil {
		return nil, fmt.Errorf("get memory path: %w", err)
	}

	repo, err := memory.NewDefaultRepository(memoryPath)
	if err != nil {
		return nil, fmt.Errorf("open memory repo: %w", err)
	}
	defer func() { _ = repo.Close() }()

	if !isQuiet {
		fmt.Println()
		fmt.Println("📊 Extracting Project Metadata")
		fmt.Println("──────────────────────────────")
	}

	var findings []core.Finding
	startTime := time.Now()

	// 1. Extract Git Statistics (deterministic)
	if !isQuiet {
		fmt.Print("   📈 Analyzing git history...")
	}
	gitParser := NewGitStatParser(s.basePath)
	gitStats, err := gitParser.Parse()
	if err != nil {
		// Track warning instead of silently swallowing
		result.Warnings = append(result.Warnings, fmt.Sprintf("git stats: %v", err))
		if !isQuiet {
			if strings.Contains(err.Error(), "not a git repository") {
				fmt.Println(" skipped (not a git repository)")
			} else {
				fmt.Printf(" skipped (%v)\n", err)
			}
		}
	} else {
		if !isQuiet {
			fmt.Printf(" %d commits, %d contributors\n", gitStats.TotalCommits, len(gitStats.Contributors))
		}
		// Convert to finding for storage (deterministic bootstrap data)
		findings = append(findings, core.Finding{
			Type:        memory.NodeTypeMetadata,
			Title:       "Git Repository Statistics",
			Description: gitStats.ToMarkdown(),
			SourceAgent: "git-stats",
			Metadata: map[string]any{
				"total_commits":      gitStats.TotalCommits,
				"contributors":       len(gitStats.Contributors),
				"project_age_months": gitStats.ProjectAgeMonths,
			},
		})
	}

	// 2. Load Documentation Files (deterministic)
	// For multi-repo workspaces, also scan sub-repo directories
	if !isQuiet {
		fmt.Print("   📄 Loading documentation...")
	}
	docLoader := NewDocLoader(s.basePath)
	ws, wsErr := project.DetectWorkspace(s.basePath)
	var docs []DocFile
	if wsErr == nil && len(ws.Services) > 0 && !containsDot(ws.Services) {
		docs, err = docLoader.LoadForServices(ws.Services)
	} else {
		docs, err = docLoader.Load()
	}
	if err != nil {
		// Track warning instead of silently swallowing
		result.Warnings = append(result.Warnings, fmt.Sprintf("doc loader: %v", err))
		if !isQuiet {
			fmt.Printf(" failed (%v)\n", err)
		}
	} else {
		if !isQuiet {
			// Show category breakdown for better visibility
			categories := make(map[string]int)
			for _, doc := range docs {
				categories[doc.Category]++
			}
			fmt.Printf(" %d files", len(docs))
			if len(categories) > 0 {
				var parts []string
				for cat, count := range categories {
					parts = append(parts, fmt.Sprintf("%d %s", count, cat))
				}
				fmt.Printf(" (%s)", joinMax(parts, 3))
			}
			fmt.Println()
		}
		// Convert each doc to a finding for storage and RAG retrieval
		for _, doc := range docs {
			findings = append(findings, core.Finding{
				Type:        memory.NodeTypeDocumentation,
				Title:       fmt.Sprintf("Documentation: %s", doc.Name),
				Description: doc.Content,
				SourceAgent: "doc-loader",
				Metadata: map[string]any{
					"path":     doc.Path,
					"category": doc.Category,
					"size":     doc.Size,
				},
			})
		}
	}

	if len(findings) == 0 {
		if !isQuiet {
			fmt.Println("   ⚠️  No metadata extracted (not a git repo or no docs)")
		}
		result.Warnings = append(result.Warnings, "no metadata extracted (not a git repo or no docs)")
		return result, nil
	}

	// 3. Ingest findings to knowledge graph
	ks := knowledge.NewService(repo, s.llmCfg)
	ks.SetBasePath(s.basePath)

	if !isQuiet {
		fmt.Print("   💾 Storing to memory...")
	}

	if err := ks.IngestFindings(ctx, findings, nil, false); err != nil {
		if !isQuiet {
			fmt.Println(" failed")
		}
		return nil, fmt.Errorf("ingest metadata: %w", err)
	}

	elapsed := time.Since(startTime).Round(time.Millisecond)
	if !isQuiet {
		fmt.Printf(" done (%v)\n", elapsed)
		fmt.Printf("\n   ✅ Extracted %d items in %v\n", len(findings), elapsed)
	}

	result.FindingsCount = len(findings)
	return result, nil
}

// containsDot returns true if any element equals ".".
func containsDot(ss []string) bool {
	for _, s := range ss {
		if s == "." {
			return true
		}
	}
	return false
}

// joinMax joins up to n strings with commas.
func joinMax(parts []string, n int) string {
	if len(parts) <= n {
		result := ""
		for i, p := range parts {
			if i > 0 {
				result += ", "
			}
			result += p
		}
		return result
	}
	result := ""
	for i := 0; i < n; i++ {
		if i > 0 {
			result += ", "
		}
		result += parts[i]
	}
	return result + ", ..."
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

// getGitHEAD returns the current git HEAD SHA for a directory, or empty string if not a git repo.
func getGitHEAD(dir string) string {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// getChangedFilesSince returns files changed between oldSHA and HEAD in the given directory.
// Returns nil if git operations fail (triggers full bootstrap fallback).
func getChangedFilesSince(dir, oldSHA string) []string {
	if oldSHA == "" {
		return nil
	}
	cmd := exec.Command("git", "diff", "--name-only", oldSHA+"..HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	raw := strings.TrimSpace(string(out))
	if raw == "" {
		return []string{} // No changes - empty slice means "nothing changed"
	}
	return strings.Split(raw, "\n")
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
