/*
Copyright © 2025 Joseph Goksu josephgoksu@gmail.com
*/
package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/josephgoksu/taskwing.app/models"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

// TaskPattern data structures for the pattern library
type TaskPattern struct {
	PatternID          string                 `json:"pattern_id"`
	Name               string                 `json:"name"`
	Category           string                 `json:"category"`
	Description        string                 `json:"description"`
	WhenToUse          []string               `json:"when_to_use"`
	TaskBreakdown      []PatternPhase         `json:"task_breakdown"`
	SuccessFactors     []string               `json:"success_factors"`
	CommonPitfalls     []string               `json:"common_pitfalls"`
	Examples           []PatternExample       `json:"examples"`
	RecommendedTools   []string               `json:"recommended_tools"`
	AIGuidance         PatternAIGuidance      `json:"ai_guidance"`
	Metrics            PatternMetrics         `json:"metrics"`
	Tags               []string               `json:"tags"`
	LastUpdated        time.Time              `json:"last_updated"`
}

type PatternPhase struct {
	Phase                string   `json:"phase"`
	Tasks                []string `json:"tasks"`
	TypicalDurationHours float64  `json:"typical_duration_hours"`
	Priority             string   `json:"priority"`
}

type PatternExample struct {
	Project    string                 `json:"project"`
	Date       string                 `json:"date"`
	Outcome    string                 `json:"outcome"`
	ArchiveRef string                 `json:"archive_ref"`
	Metrics    map[string]interface{} `json:"metrics"`
}

type PatternAIGuidance struct {
	TaskGenerationHints  []string            `json:"task_generation_hints"`
	PrioritySuggestions  map[string]string   `json:"priority_suggestions"`
	DependencyPatterns   []string            `json:"dependency_patterns"`
}

type PatternMetrics struct {
	UsageCount               int     `json:"usage_count"`
	SuccessRate             float64 `json:"success_rate"`
	AverageDurationHours    float64 `json:"average_duration_hours"`
	AverageFileReduction    float64 `json:"average_file_reduction,omitempty"`
	ReworkRate              float64 `json:"rework_rate"`
	UserSatisfaction        string  `json:"user_satisfaction"`
}

type PatternLibrary struct {
	Version     string                        `json:"version"`
	Patterns    []TaskPattern                 `json:"patterns"`
	Categories  map[string]PatternCategory    `json:"categories"`
	Statistics  PatternLibraryStatistics      `json:"statistics"`
}

type PatternCategory struct {
	Description  string `json:"description"`
	PatternCount int    `json:"pattern_count"`
}

type PatternLibraryStatistics struct {
	TotalPatterns       int     `json:"total_patterns"`
	MostUsedPattern     string  `json:"most_used_pattern"`
	AverageSuccessRate  float64 `json:"average_success_rate"`
	TotalUsageCount     int     `json:"total_usage_count"`
}

// patternsCmd represents the patterns command
var patternsCmd = &cobra.Command{
	Use:   "patterns",
	Short: "Manage task patterns library",
	Long: `Manage the TaskWing patterns library which contains common task patterns
extracted from archived projects. Use this to:

- Extract patterns from archived data
- Match patterns for new tasks  
- View pattern statistics and recommendations
- Update pattern library with new insights

This enables AI-assisted task generation based on proven successful patterns.`,
}

var extractPatternsCmd = &cobra.Command{
	Use:   "extract",
	Short: "Extract patterns from archived projects",
	Long: `Analyze archived projects to automatically extract task patterns.
This processes archived JSON data to identify common task structures,
timing patterns, and success factors that can be reused for future projects.`,
	Run: runExtractPatterns,
}

var listPatternsCmd = &cobra.Command{
	Use:   "list",
	Short: "List available patterns",
	Long:  `Display all patterns in the library with their success rates and usage statistics.`,
	Run:   runListPatterns,
}

var matchPatternsCmd = &cobra.Command{
	Use:   "match <description>",
	Short: "Find patterns matching a task description",
	Long: `Analyze a task description and suggest matching patterns from the library.
This helps identify established workflows that can be applied to new work.`,
	Args: cobra.ExactArgs(1),
	Run:  runMatchPatterns,
}

var updatePatternsCmd = &cobra.Command{
	Use:   "update",
	Short: "Update pattern library from latest archives",
	Long: `Update existing patterns with new data from recent archives.
This keeps patterns current and improves accuracy of metrics and recommendations.`,
	Run: runUpdatePatterns,
}

func init() {
	rootCmd.AddCommand(patternsCmd)
	patternsCmd.AddCommand(extractPatternsCmd)
	patternsCmd.AddCommand(listPatternsCmd)
	patternsCmd.AddCommand(matchPatternsCmd)
	patternsCmd.AddCommand(updatePatternsCmd)
}

func runExtractPatterns(cmd *cobra.Command, args []string) {
	cfg := GetConfig()
	
	fmt.Printf("🔍 Extracting patterns from archived projects...\n\n")
	
	// Load existing pattern library
	library, err := loadPatternLibrary(cfg)
	if err != nil {
		fmt.Printf("Creating new pattern library...\n")
		library = &PatternLibrary{
			Version:    "1.0",
			Patterns:   []TaskPattern{},
			Categories: make(map[string]PatternCategory),
			Statistics: PatternLibraryStatistics{},
		}
	}
	
	// Get all archive files
	archiveDir := filepath.Join(cfg.Project.RootDir, "archive")
	archives, err := findArchiveFiles(archiveDir)
	if err != nil {
		fmt.Printf("Error finding archive files: %v\n", err)
		return
	}
	
	if len(archives) == 0 {
		fmt.Println("No archive files found. Use 'taskwing archive' to create archives first.")
		return
	}
	
	fmt.Printf("Found %d archive files to analyze\n", len(archives))
	
	// Extract patterns from each archive
	newPatterns := 0
	updatedPatterns := 0
	
	for _, archivePath := range archives {
		archive, err := loadArchiveFile(archivePath)
		if err != nil {
			fmt.Printf("Warning: Could not load %s: %v\n", archivePath, err)
			continue
		}
		
		// Extract pattern from this archive
		pattern, isNew := extractPatternFromArchive(archive, library)
		if pattern != nil {
			if isNew {
				library.Patterns = append(library.Patterns, *pattern)
				newPatterns++
			} else {
				updateExistingPattern(library, *pattern)
				updatedPatterns++
			}
		}
	}
	
	// Update library statistics
	updateLibraryStatistics(library)
	
	// Save updated library
	if err := savePatternLibrary(library, cfg); err != nil {
		fmt.Printf("Error saving pattern library: %v\n", err)
		return
	}
	
	fmt.Printf("\n✅ Pattern extraction complete!\n")
	fmt.Printf("📊 New patterns: %d\n", newPatterns)
	fmt.Printf("🔄 Updated patterns: %d\n", updatedPatterns)
	fmt.Printf("📚 Total patterns: %d\n", len(library.Patterns))
	
	// Show most promising patterns
	if len(library.Patterns) > 0 {
		fmt.Printf("\n🏆 Top patterns by success rate:\n")
		sort.Slice(library.Patterns, func(i, j int) bool {
			return library.Patterns[i].Metrics.SuccessRate > library.Patterns[j].Metrics.SuccessRate
		})
		
		for i, pattern := range library.Patterns {
			if i >= 3 { break } // Show top 3
			fmt.Printf("  %d. %s (%.0f%% success, %d uses)\n", 
				i+1, pattern.Name, pattern.Metrics.SuccessRate, pattern.Metrics.UsageCount)
		}
	}
}

func runListPatterns(cmd *cobra.Command, args []string) {
	cfg := GetConfig()
	
	library, err := loadPatternLibrary(cfg)
	if err != nil {
		fmt.Printf("No pattern library found. Run 'taskwing patterns extract' first.\n")
		return
	}
	
	if len(library.Patterns) == 0 {
		fmt.Printf("No patterns found in library.\n")
		return
	}
	
	fmt.Printf("📚 TaskWing Pattern Library\n")
	fmt.Printf("Version: %s | Total: %d patterns\n\n", library.Version, len(library.Patterns))
	
	// Group by category
	categorized := make(map[string][]TaskPattern)
	for _, pattern := range library.Patterns {
		categorized[pattern.Category] = append(categorized[pattern.Category], pattern)
	}
	
	for category, patterns := range categorized {
		fmt.Printf("## %s\n", strings.Title(category))
		
		for _, pattern := range patterns {
			fmt.Printf("### %s (%s)\n", pattern.Name, pattern.PatternID)
			fmt.Printf("📝 %s\n", pattern.Description)
			fmt.Printf("📊 Success: %.0f%% | Uses: %d | Avg Duration: %.1fh\n", 
				pattern.Metrics.SuccessRate, pattern.Metrics.UsageCount, pattern.Metrics.AverageDurationHours)
			
			if len(pattern.WhenToUse) > 0 {
				fmt.Printf("🎯 When to use:\n")
				for _, condition := range pattern.WhenToUse {
					fmt.Printf("  • %s\n", condition)
				}
			}
			
			if len(pattern.TaskBreakdown) > 0 {
				fmt.Printf("⚡ Typical breakdown: ")
				phaseNames := []string{}
				for _, phase := range pattern.TaskBreakdown {
					phaseNames = append(phaseNames, phase.Phase)
				}
				fmt.Printf("%s\n", strings.Join(phaseNames, " → "))
			}
			
			fmt.Printf("🏷️  Tags: %s\n\n", strings.Join(pattern.Tags, ", "))
		}
	}
	
	// Show statistics
	fmt.Printf("📊 Library Statistics\n")
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	fmt.Printf("Most Used Pattern: %s\n", library.Statistics.MostUsedPattern)
	fmt.Printf("Average Success Rate: %.1f%%\n", library.Statistics.AverageSuccessRate)
	fmt.Printf("Total Usage Count: %d\n", library.Statistics.TotalUsageCount)
}

func runMatchPatterns(cmd *cobra.Command, args []string) {
	cfg := GetConfig()
	description := args[0]
	
	library, err := loadPatternLibrary(cfg)
	if err != nil {
		fmt.Printf("No pattern library found. Run 'taskwing patterns extract' first.\n")
		return
	}
	
	if len(library.Patterns) == 0 {
		fmt.Printf("No patterns found in library.\n")
		return
	}
	
	fmt.Printf("🔍 Finding patterns matching: \"%s\"\n\n", description)
	
	// Simple keyword matching for now - could be enhanced with ML
	matches := findMatchingPatterns(description, library.Patterns)
	
	if len(matches) == 0 {
		fmt.Printf("No matching patterns found.\n")
		fmt.Printf("\n💡 Consider these general approaches:\n")
		
		// Show top patterns by success rate as fallback
		sort.Slice(library.Patterns, func(i, j int) bool {
			return library.Patterns[i].Metrics.SuccessRate > library.Patterns[j].Metrics.SuccessRate
		})
		
		for i, pattern := range library.Patterns {
			if i >= 2 { break }
			fmt.Printf("  • %s (%.0f%% success rate)\n", pattern.Name, pattern.Metrics.SuccessRate)
		}
		return
	}
	
	fmt.Printf("Found %d matching patterns:\n\n", len(matches))
	
	for i, match := range matches {
		pattern := match.Pattern
		score := match.Score
		
		fmt.Printf("%d. %s (%.0f%% match)\n", i+1, pattern.Name, score*100)
		fmt.Printf("   📝 %s\n", pattern.Description)
		fmt.Printf("   📊 Success: %.0f%% | Duration: %.1fh | Uses: %d\n", 
			pattern.Metrics.SuccessRate, pattern.Metrics.AverageDurationHours, pattern.Metrics.UsageCount)
		
		if len(pattern.TaskBreakdown) > 0 {
			fmt.Printf("   ⚡ Suggested breakdown:\n")
			for _, phase := range pattern.TaskBreakdown {
				fmt.Printf("     • %s (%.1fh, %s priority)\n", 
					phase.Phase, phase.TypicalDurationHours, phase.Priority)
			}
		}
		
		if len(pattern.SuccessFactors) > 0 && i == 0 { // Show success factors for best match
			fmt.Printf("   🎯 Success factors:\n")
			for _, factor := range pattern.SuccessFactors {
				fmt.Printf("     • %s\n", factor)
			}
		}
		
		fmt.Printf("\n")
	}
	
	// Ask if user wants to apply the best pattern
	if len(matches) > 0 {
		bestPattern := matches[0].Pattern
		
		prompt := promptui.Prompt{
			Label:     fmt.Sprintf("Apply '%s' pattern to create tasks? (y/N)", bestPattern.Name),
			Default:   "N",
		}
		
		result, err := prompt.Run()
		if err == nil && strings.ToLower(result) == "y" {
			fmt.Printf("\n🔧 Creating tasks based on '%s' pattern...\n", bestPattern.Name)
			if err := applyPatternToCreateTasks(bestPattern, description); err != nil {
				fmt.Printf("Error creating tasks: %v\n", err)
			} else {
				fmt.Printf("✅ Tasks created successfully!\n")
			}
		}
	}
}

func runUpdatePatterns(cmd *cobra.Command, args []string) {
	fmt.Printf("🔄 Updating pattern library with latest data...\n\n")
	
	// This is essentially the same as extract but focuses on updating existing patterns
	runExtractPatterns(cmd, args)
}

// Helper functions

func loadPatternLibrary(cfg *AppConfig) (*PatternLibrary, error) {
	libPath := filepath.Join(cfg.Project.RootDir, "archive", "patterns.json")
	
	data, err := os.ReadFile(libPath)
	if err != nil {
		return nil, err
	}
	
	var library PatternLibrary
	if err := json.Unmarshal(data, &library); err != nil {
		return nil, err
	}
	
	return &library, nil
}

func savePatternLibrary(library *PatternLibrary, cfg *AppConfig) error {
	libPath := filepath.Join(cfg.Project.RootDir, "archive", "patterns.json")
	
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(libPath), 0755); err != nil {
		return err
	}
	
	data, err := json.MarshalIndent(library, "", "  ")
	if err != nil {
		return err
	}
	
	return os.WriteFile(libPath, data, 0644)
}

func findArchiveFiles(archiveDir string) ([]string, error) {
	var archives []string
	
	err := filepath.Walk(archiveDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		
		if strings.HasSuffix(path, ".json") && !strings.Contains(path, "patterns.json") && !strings.Contains(path, "index.json") {
			archives = append(archives, path)
		}
		
		return nil
	})
	
	return archives, err
}

type ArchiveData struct {
	Version     string                 `json:"version"`
	ArchivedAt  time.Time             `json:"archived_at"`
	Project     ProjectInfo           `json:"project"`
	Tasks       []models.Task         `json:"tasks"`
	Retrospective *Retrospective      `json:"retrospective,omitempty"`
	Patterns    []TaskPattern         `json:"patterns_identified,omitempty"`
}

func loadArchiveFile(path string) (*ArchiveData, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	
	var archive ArchiveData
	if err := json.Unmarshal(data, &archive); err != nil {
		return nil, err
	}
	
	return &archive, nil
}

func extractPatternFromArchive(archive *ArchiveData, library *PatternLibrary) (*TaskPattern, bool) {
	// Skip if not enough tasks to establish a pattern
	if len(archive.Tasks) < 3 {
		return nil, false
	}
	
	// Extract pattern based on task structure and retrospective data
	pattern := &TaskPattern{
		PatternID:   generatePatternID(archive.Project.Name),
		Name:        derivePatternName(archive.Project.Name, archive.Tasks),
		Category:    deriveCategory(archive.Tasks),
		Description: deriveDescription(archive.Project.Description, archive.Tasks),
		LastUpdated: time.Now(),
	}
	
	// Extract task breakdown phases
	pattern.TaskBreakdown = extractTaskBreakdown(archive.Tasks)
	
	// Extract success factors from retrospective
	if archive.Retrospective != nil {
		pattern.SuccessFactors = archive.Retrospective.WhatWentWell
		pattern.CommonPitfalls = archive.Retrospective.WhatWentWrong
	}
	
	// Calculate metrics
	pattern.Metrics = calculatePatternMetrics(archive)
	
	// Extract tags
	pattern.Tags = extractPatternTags(archive.Project.Name, archive.Tasks)
	
	// Extract when to use criteria
	pattern.WhenToUse = deriveWhenToUse(archive.Project.Description, archive.Tasks)
	
	// Check if this pattern already exists
	existingPattern := findExistingPattern(pattern, library)
	if existingPattern != nil {
		// Update existing pattern
		mergePatternData(existingPattern, pattern)
		return existingPattern, false
	}
	
	return pattern, true
}

// Pattern matching logic

type PatternMatch struct {
	Pattern TaskPattern
	Score   float64
}

func findMatchingPatterns(description string, patterns []TaskPattern) []PatternMatch {
	var matches []PatternMatch
	
	descLower := strings.ToLower(description)
	words := strings.Fields(descLower)
	
	for _, pattern := range patterns {
		score := calculateMatchScore(descLower, words, pattern)
		if score > 0.1 { // Minimum threshold
			matches = append(matches, PatternMatch{
				Pattern: pattern,
				Score:   score,
			})
		}
	}
	
	// Sort by score descending
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].Score > matches[j].Score
	})
	
	return matches
}

func calculateMatchScore(description string, words []string, pattern TaskPattern) float64 {
	score := 0.0
	maxScore := 0.0
	
	// Check name match
	maxScore += 1.0
	if strings.Contains(strings.ToLower(pattern.Name), description) {
		score += 1.0
	}
	
	// Check description match
	maxScore += 1.0
	if strings.Contains(strings.ToLower(pattern.Description), description) {
		score += 1.0
	}
	
	// Check tags
	maxScore += 1.0
	for _, tag := range pattern.Tags {
		if strings.Contains(description, strings.ToLower(tag)) {
			score += 1.0
			break
		}
	}
	
	// Check when to use criteria
	maxScore += 1.0
	for _, criteria := range pattern.WhenToUse {
		if containsAnyWord(strings.ToLower(criteria), words) {
			score += 1.0
			break
		}
	}
	
	if maxScore == 0 {
		return 0
	}
	
	return score / maxScore
}

func containsAnyWord(text string, words []string) bool {
	for _, word := range words {
		if len(word) > 3 && strings.Contains(text, word) {
			return true
		}
	}
	return false
}

// Implementation helper functions - these would be expanded in a full implementation

func generatePatternID(projectName string) string {
	// Simple ID generation - could be enhanced
	cleaned := strings.ToLower(strings.ReplaceAll(projectName, " ", "-"))
	return fmt.Sprintf("pattern-%s-%d", cleaned, time.Now().Unix()%10000)
}

func derivePatternName(projectName string, tasks []models.Task) string {
	// Analyze project name and task titles to derive a pattern name
	if strings.Contains(strings.ToLower(projectName), "documentation") {
		return "Documentation Consolidation"
	}
	if strings.Contains(strings.ToLower(projectName), "archive") || strings.Contains(strings.ToLower(projectName), "system") {
		return "System Implementation"
	}
	return fmt.Sprintf("%s Pattern", projectName)
}

func deriveCategory(tasks []models.Task) string {
	// Analyze task patterns to determine category
	hasRefactor := false
	hasDevelopment := false
	
	for _, task := range tasks {
		taskLower := strings.ToLower(task.Title + " " + task.Description)
		if strings.Contains(taskLower, "refactor") || strings.Contains(taskLower, "consolidate") || strings.Contains(taskLower, "cleanup") {
			hasRefactor = true
		}
		if strings.Contains(taskLower, "implement") || strings.Contains(taskLower, "create") || strings.Contains(taskLower, "build") {
			hasDevelopment = true
		}
	}
	
	if hasRefactor {
		return "refactoring"
	}
	if hasDevelopment {
		return "development"
	}
	return "general"
}

func deriveDescription(projectDesc string, tasks []models.Task) string {
	if projectDesc != "" {
		return projectDesc
	}
	return fmt.Sprintf("Pattern derived from %d tasks", len(tasks))
}

func extractTaskBreakdown(tasks []models.Task) []PatternPhase {
	// Group tasks by common phases based on title keywords
	phases := make(map[string][]models.Task)
	
	for _, task := range tasks {
		phase := identifyPhase(task.Title)
		phases[phase] = append(phases[phase], task)
	}
	
	var breakdown []PatternPhase
	phaseOrder := []string{"Audit", "Design", "Implementation", "Cleanup", "Testing"}
	
	for _, phaseName := range phaseOrder {
		if tasks, exists := phases[phaseName]; exists {
			phase := PatternPhase{
				Phase:                phaseName,
				Priority:             determinePriority(tasks),
				TypicalDurationHours: calculateAveragePhaseHours(tasks),
			}
			
			for _, task := range tasks {
				phase.Tasks = append(phase.Tasks, task.Title)
			}
			
			breakdown = append(breakdown, phase)
		}
	}
	
	return breakdown
}

func identifyPhase(title string) string {
	titleLower := strings.ToLower(title)
	
	if strings.Contains(titleLower, "audit") || strings.Contains(titleLower, "analysis") || strings.Contains(titleLower, "inventory") {
		return "Audit"
	}
	if strings.Contains(titleLower, "design") || strings.Contains(titleLower, "structure") || strings.Contains(titleLower, "plan") {
		return "Design"
	}
	if strings.Contains(titleLower, "cleanup") || strings.Contains(titleLower, "remove") || strings.Contains(titleLower, "delete") {
		return "Cleanup"
	}
	if strings.Contains(titleLower, "test") || strings.Contains(titleLower, "verify") || strings.Contains(titleLower, "validate") {
		return "Testing"
	}
	return "Implementation"
}

func determinePriority(tasks []models.Task) string {
	highCount := 0
	for _, task := range tasks {
		if task.Priority == "high" {
			highCount++
		}
	}
	
	if float64(highCount)/float64(len(tasks)) > 0.5 {
		return "high"
	}
	return "medium"
}

func calculateAveragePhaseHours(tasks []models.Task) float64 {
	totalHours := 0.0
	count := 0
	
	for _, task := range tasks {
		if task.CompletedAt != nil {
			hours := task.CompletedAt.Sub(task.CreatedAt).Hours()
			totalHours += hours
			count++
		}
	}
	
	if count == 0 {
		return 0.5 // Default estimate
	}
	
	return totalHours / float64(count)
}

func calculatePatternMetrics(archive *ArchiveData) PatternMetrics {
	totalHours := 0.0
	completedTasks := 0
	
	for _, task := range archive.Tasks {
		if task.Status == "completed" && task.CompletedAt != nil {
			hours := task.CompletedAt.Sub(task.CreatedAt).Hours()
			totalHours += hours
			completedTasks++
		}
	}
	
	successRate := float64(completedTasks) / float64(len(archive.Tasks)) * 100
	
	return PatternMetrics{
		UsageCount:           1,
		SuccessRate:         successRate,
		AverageDurationHours: totalHours,
		ReworkRate:          0, // Would need more analysis to determine
		UserSatisfaction:    "high", // Could be derived from retrospective
	}
}

func extractPatternTags(projectName string, tasks []models.Task) []string {
	tags := []string{}
	
	// Add project-based tags
	projectLower := strings.ToLower(projectName)
	if strings.Contains(projectLower, "documentation") {
		tags = append(tags, "documentation")
	}
	if strings.Contains(projectLower, "archive") {
		tags = append(tags, "archival")
	}
	if strings.Contains(projectLower, "system") {
		tags = append(tags, "system")
	}
	
	// Add task-based tags
	hasRefactor := false
	hasImplementation := false
	
	for _, task := range tasks {
		taskLower := strings.ToLower(task.Title + " " + task.Description)
		if strings.Contains(taskLower, "refactor") || strings.Contains(taskLower, "consolidate") {
			hasRefactor = true
		}
		if strings.Contains(taskLower, "implement") || strings.Contains(taskLower, "create") {
			hasImplementation = true
		}
	}
	
	if hasRefactor {
		tags = append(tags, "refactoring")
	}
	if hasImplementation {
		tags = append(tags, "implementation")
	}
	
	return tags
}

func deriveWhenToUse(projectDesc string, tasks []models.Task) []string {
	criteria := []string{}
	
	// Analyze project and tasks to derive when to use this pattern
	if strings.Contains(strings.ToLower(projectDesc), "multiple") {
		criteria = append(criteria, "Multiple files or components need consolidation")
	}
	if strings.Contains(strings.ToLower(projectDesc), "unify") || strings.Contains(strings.ToLower(projectDesc), "simplify") {
		criteria = append(criteria, "Existing structure needs simplification")
	}
	
	return criteria
}

func findExistingPattern(pattern *TaskPattern, library *PatternLibrary) *TaskPattern {
	for i, existing := range library.Patterns {
		if strings.ToLower(existing.Name) == strings.ToLower(pattern.Name) {
			return &library.Patterns[i]
		}
	}
	return nil
}

func updateExistingPattern(library *PatternLibrary, newPattern TaskPattern) {
	for i, existing := range library.Patterns {
		if strings.ToLower(existing.Name) == strings.ToLower(newPattern.Name) {
			// Update metrics
			library.Patterns[i].Metrics.UsageCount++
			library.Patterns[i].LastUpdated = time.Now()
			
			// Update success rate (weighted average)
			oldWeight := float64(existing.Metrics.UsageCount - 1)
			newWeight := 1.0
			totalWeight := oldWeight + newWeight
			
			library.Patterns[i].Metrics.SuccessRate = 
				(existing.Metrics.SuccessRate*oldWeight + newPattern.Metrics.SuccessRate*newWeight) / totalWeight
			
			// Update average duration
			library.Patterns[i].Metrics.AverageDurationHours = 
				(existing.Metrics.AverageDurationHours*oldWeight + newPattern.Metrics.AverageDurationHours*newWeight) / totalWeight
			
			break
		}
	}
}

func mergePatternData(existing *TaskPattern, new *TaskPattern) {
	// Merge data from new pattern into existing
	// This is a simplified merge - could be more sophisticated
	existing.LastUpdated = time.Now()
	
	// Merge examples
	existing.Examples = append(existing.Examples, new.Examples...)
	
	// Merge tags (unique)
	tagMap := make(map[string]bool)
	for _, tag := range existing.Tags {
		tagMap[tag] = true
	}
	for _, tag := range new.Tags {
		if !tagMap[tag] {
			existing.Tags = append(existing.Tags, tag)
		}
	}
}

func updateLibraryStatistics(library *PatternLibrary) {
	library.Statistics.TotalPatterns = len(library.Patterns)
	
	if len(library.Patterns) == 0 {
		return
	}
	
	// Find most used pattern
	mostUsed := library.Patterns[0]
	totalUsage := 0
	totalSuccess := 0.0
	
	for _, pattern := range library.Patterns {
		totalUsage += pattern.Metrics.UsageCount
		totalSuccess += pattern.Metrics.SuccessRate
		
		if pattern.Metrics.UsageCount > mostUsed.Metrics.UsageCount {
			mostUsed = pattern
		}
	}
	
	library.Statistics.MostUsedPattern = mostUsed.Name
	library.Statistics.TotalUsageCount = totalUsage
	library.Statistics.AverageSuccessRate = totalSuccess / float64(len(library.Patterns))
	
	// Update categories
	categoryCount := make(map[string]int)
	for _, pattern := range library.Patterns {
		categoryCount[pattern.Category]++
	}
	
	library.Categories = make(map[string]PatternCategory)
	for cat, count := range categoryCount {
		library.Categories[cat] = PatternCategory{
			Description:  fmt.Sprintf("Patterns for %s work", cat),
			PatternCount: count,
		}
	}
}

func applyPatternToCreateTasks(pattern TaskPattern, description string) error {
	fmt.Printf("Creating tasks based on pattern breakdown:\n")
	
	for i, phase := range pattern.TaskBreakdown {
		fmt.Printf("%d. %s Phase (%.1fh, %s priority)\n", 
			i+1, phase.Phase, phase.TypicalDurationHours, phase.Priority)
		
		for _, taskTitle := range phase.Tasks {
			fmt.Printf("   • %s\n", taskTitle)
		}
	}
	
	// This would integrate with the task creation system
	// For now, just show what would be created
	fmt.Printf("\nNote: Automatic task creation will be implemented in future version.\n")
	fmt.Printf("Use these suggestions to manually create tasks with appropriate priorities.\n")
	
	return nil
}