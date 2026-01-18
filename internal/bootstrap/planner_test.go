package bootstrap

import (
	"os"
	"path/filepath"
	"testing"
)

// TestValidateFlags tests flag validation rules
func TestValidateFlags(t *testing.T) {
	tests := []struct {
		name    string
		flags   Flags
		wantErr bool
		errMsg  string
	}{
		{
			name:    "empty flags valid",
			flags:   Flags{},
			wantErr: false,
		},
		{
			name:    "skip-index alone valid",
			flags:   Flags{SkipIndex: true},
			wantErr: false,
		},
		{
			name:    "force-index alone valid",
			flags:   Flags{Force: true},
			wantErr: false,
		},
		{
			name:    "skip-index and force-index conflict",
			flags:   Flags{SkipIndex: true, Force: true},
			wantErr: true,
			errMsg:  "cannot skip indexing and force indexing",
		},
		{
			name:    "skip-analyze flag valid",
			flags:   Flags{SkipAnalyze: true},
			wantErr: false,
		},
		{
			name:    "preview flag valid",
			flags:   Flags{Preview: true},
			wantErr: false,
		},
		{
			name:    "all non-conflicting flags valid",
			flags:   Flags{Preview: true, SkipAnalyze: true, Verbose: true, Trace: true},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateFlags(tt.flags)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidateFlags() expected error, got nil")
				} else if tt.errMsg != "" && !containsString(err.Error(), tt.errMsg) {
					t.Errorf("ValidateFlags() error = %q, want to contain %q", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateFlags() unexpected error: %v", err)
				}
			}
		})
	}
}

// TestDecidePlan_Modes tests mode detection logic
func TestDecidePlan_Modes(t *testing.T) {
	tests := []struct {
		name         string
		snapshot     *Snapshot
		flags        Flags
		expectedMode BootstrapMode
		wantError    bool
	}{
		{
			name: "first time - nothing exists",
			snapshot: &Snapshot{
				Project:         ProjectHealth{Status: HealthMissing},
				AIHealth:        map[string]AIHealth{},
				HasAnyLocalAI:   false,
				HasAnyGlobalMCP: false,
			},
			flags:        Flags{},
			expectedMode: ModeFirstTime,
		},
		{
			name: "first time - global MCP exists, no local",
			snapshot: &Snapshot{
				Project:         ProjectHealth{Status: HealthMissing},
				AIHealth:        map[string]AIHealth{},
				HasAnyLocalAI:   false,
				HasAnyGlobalMCP: true,
				GlobalMCPAIs:    []string{"claude"},
			},
			flags:        Flags{},
			expectedMode: ModeFirstTime,
		},
		{
			name: "repair - partial project",
			snapshot: &Snapshot{
				Project: ProjectHealth{
					Status:    HealthPartial,
					DirExists: true,
					Reason:    "missing subdirectories",
				},
				AIHealth:        map[string]AIHealth{},
				HasAnyLocalAI:   false,
				HasAnyGlobalMCP: false,
			},
			flags:        Flags{},
			expectedMode: ModeRepair,
		},
		{
			name: "repair - AI configs need repair",
			snapshot: &Snapshot{
				Project: ProjectHealth{Status: HealthOK, DirExists: true, MemoryDirExists: true, PlansDirExists: true},
				AIHealth: map[string]AIHealth{
					"claude": {
						Name:              "claude",
						Status:            HealthPartial,
						CommandsDirExists: true, // User started setup but didn't finish
						GlobalMCPExists:   true,
						Reason:            "only 3/7 command files present",
					},
				},
				HasAnyLocalAI:   true,
				HasAnyGlobalMCP: true,
				ExistingLocalAI: []string{"claude"},
				GlobalMCPAIs:    []string{"claude"},
			},
			flags:        Flags{},
			expectedMode: ModeRepair,
		},
		{
			name: "reconfigure - project OK but no AI configs",
			snapshot: &Snapshot{
				Project: ProjectHealth{Status: HealthOK, DirExists: true, MemoryDirExists: true, PlansDirExists: true},
				AIHealth: map[string]AIHealth{
					"claude": {Name: "claude", Status: HealthMissing},
				},
				HasAnyLocalAI:   false,
				HasAnyGlobalMCP: false,
			},
			flags:        Flags{},
			expectedMode: ModeReconfigure,
		},
		{
			name: "run - everything healthy",
			snapshot: &Snapshot{
				Project: ProjectHealth{Status: HealthOK, DirExists: true, MemoryDirExists: true, PlansDirExists: true},
				AIHealth: map[string]AIHealth{
					"claude": {Name: "claude", Status: HealthOK, CommandsDirExists: true, CommandFilesCount: 7},
				},
				HasAnyLocalAI:   true,
				ExistingLocalAI: []string{"claude"},
			},
			flags:        Flags{},
			expectedMode: ModeRun,
		},
		{
			name: "error - skip-init but project missing",
			snapshot: &Snapshot{
				Project:         ProjectHealth{Status: HealthMissing},
				AIHealth:        map[string]AIHealth{},
				HasAnyLocalAI:   false,
				HasAnyGlobalMCP: false,
			},
			flags:        Flags{SkipInit: true},
			expectedMode: ModeError,
			wantError:    true,
		},
		{
			name: "error - conflicting flags",
			snapshot: &Snapshot{
				Project:  ProjectHealth{Status: HealthOK, DirExists: true, MemoryDirExists: true, PlansDirExists: true},
				AIHealth: map[string]AIHealth{},
			},
			flags:        Flags{SkipIndex: true, Force: true},
			expectedMode: ModeError,
			wantError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan := DecidePlan(tt.snapshot, tt.flags)

			if plan.Mode != tt.expectedMode {
				t.Errorf("DecidePlan() mode = %v, want %v", plan.Mode, tt.expectedMode)
			}

			if tt.wantError && plan.Error == nil {
				t.Errorf("DecidePlan() expected error, got nil")
			}

			if !tt.wantError && plan.Error != nil {
				t.Errorf("DecidePlan() unexpected error: %v", plan.Error)
			}
		})
	}
}

// TestDecidePlan_Actions tests action selection logic
func TestDecidePlan_Actions(t *testing.T) {
	tests := []struct {
		name          string
		snapshot      *Snapshot
		flags         Flags
		expectActions []Action
		expectSkipped bool // Check that skipped actions are populated
	}{
		{
			name: "first time - all init actions including LLM",
			snapshot: &Snapshot{
				Project:         ProjectHealth{Status: HealthMissing},
				AIHealth:        map[string]AIHealth{},
				HasAnyLocalAI:   false,
				HasAnyGlobalMCP: false,
			},
			flags: Flags{},
			expectActions: []Action{
				ActionInitProject,
				ActionGenerateAIConfigs,
				ActionInstallMCP,
				ActionIndexCode,
				ActionExtractMetadata,
				ActionLLMAnalyze,
			},
		},
		{
			name: "run mode - index, extract, and LLM (default)",
			snapshot: &Snapshot{
				Project:         ProjectHealth{Status: HealthOK, DirExists: true, MemoryDirExists: true, PlansDirExists: true},
				AIHealth:        map[string]AIHealth{"claude": {Status: HealthOK}},
				HasAnyLocalAI:   true,
				ExistingLocalAI: []string{"claude"},
			},
			flags: Flags{},
			expectActions: []Action{
				ActionIndexCode,
				ActionExtractMetadata,
				ActionLLMAnalyze,
			},
		},
		{
			name: "skip-index removes indexing but keeps LLM",
			snapshot: &Snapshot{
				Project:         ProjectHealth{Status: HealthOK, DirExists: true, MemoryDirExists: true, PlansDirExists: true},
				AIHealth:        map[string]AIHealth{"claude": {Status: HealthOK}},
				HasAnyLocalAI:   true,
				ExistingLocalAI: []string{"claude"},
			},
			flags: Flags{SkipIndex: true},
			expectActions: []Action{
				ActionExtractMetadata,
				ActionLLMAnalyze,
			},
			expectSkipped: true,
		},
		{
			name: "skip-analyze removes LLM action",
			snapshot: &Snapshot{
				Project:         ProjectHealth{Status: HealthOK, DirExists: true, MemoryDirExists: true, PlansDirExists: true},
				AIHealth:        map[string]AIHealth{"claude": {Status: HealthOK}},
				HasAnyLocalAI:   true,
				ExistingLocalAI: []string{"claude"},
			},
			flags: Flags{SkipAnalyze: true},
			expectActions: []Action{
				ActionIndexCode,
				ActionExtractMetadata,
			},
			expectSkipped: true,
		},
		{
			name: "large project without force skips indexing but keeps LLM",
			snapshot: &Snapshot{
				Project:         ProjectHealth{Status: HealthOK, DirExists: true, MemoryDirExists: true, PlansDirExists: true},
				AIHealth:        map[string]AIHealth{"claude": {Status: HealthOK}},
				HasAnyLocalAI:   true,
				ExistingLocalAI: []string{"claude"},
				FileCount:       6000,
				IsLargeProject:  true,
			},
			flags: Flags{},
			expectActions: []Action{
				ActionExtractMetadata,
				ActionLLMAnalyze,
			},
			expectSkipped: true,
		},
		{
			name: "large project with force includes indexing and LLM",
			snapshot: &Snapshot{
				Project:         ProjectHealth{Status: HealthOK, DirExists: true, MemoryDirExists: true, PlansDirExists: true},
				AIHealth:        map[string]AIHealth{"claude": {Status: HealthOK}},
				HasAnyLocalAI:   true,
				ExistingLocalAI: []string{"claude"},
				FileCount:       6000,
				IsLargeProject:  true,
			},
			flags: Flags{Force: true},
			expectActions: []Action{
				ActionIndexCode,
				ActionExtractMetadata,
				ActionLLMAnalyze,
			},
		},
		{
			name: "preview skips extract but keeps LLM",
			snapshot: &Snapshot{
				Project:         ProjectHealth{Status: HealthOK, DirExists: true, MemoryDirExists: true, PlansDirExists: true},
				AIHealth:        map[string]AIHealth{"claude": {Status: HealthOK}},
				HasAnyLocalAI:   true,
				ExistingLocalAI: []string{"claude"},
			},
			flags: Flags{Preview: true},
			expectActions: []Action{
				ActionIndexCode,
				ActionLLMAnalyze,
			},
			expectSkipped: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan := DecidePlan(tt.snapshot, tt.flags)

			if plan.Mode == ModeError {
				t.Fatalf("DecidePlan() returned error mode unexpectedly: %v", plan.Error)
			}

			// Check expected actions are present
			for _, expected := range tt.expectActions {
				found := false
				for _, actual := range plan.Actions {
					if actual == expected {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("DecidePlan() missing expected action %v, got %v", expected, plan.Actions)
				}
			}

			// Check no unexpected actions
			for _, actual := range plan.Actions {
				found := false
				for _, expected := range tt.expectActions {
					if actual == expected {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("DecidePlan() has unexpected action %v", actual)
				}
			}

			// Check skipped actions populated when expected
			if tt.expectSkipped && len(plan.SkippedActions) == 0 {
				t.Errorf("DecidePlan() expected skipped actions to be populated")
			}
		})
	}
}

// TestDecidePlan_RequiresLLMConfig tests LLM config requirement
func TestDecidePlan_RequiresLLMConfig(t *testing.T) {
	snapshot := &Snapshot{
		Project:         ProjectHealth{Status: HealthOK, DirExists: true, MemoryDirExists: true, PlansDirExists: true},
		AIHealth:        map[string]AIHealth{"claude": {Status: HealthOK}},
		HasAnyLocalAI:   true,
		ExistingLocalAI: []string{"claude"},
	}

	tests := []struct {
		name    string
		flags   Flags
		wantLLM bool
	}{
		{
			name:    "default - LLM required (analyze is default)",
			flags:   Flags{},
			wantLLM: true,
		},
		{
			name:    "skip-analyze - no LLM required",
			flags:   Flags{SkipAnalyze: true},
			wantLLM: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan := DecidePlan(snapshot, tt.flags)
			if plan.RequiresLLMConfig != tt.wantLLM {
				t.Errorf("DecidePlan() RequiresLLMConfig = %v, want %v", plan.RequiresLLMConfig, tt.wantLLM)
			}
		})
	}
}

// TestDecidePlan_Warnings tests warning generation
func TestDecidePlan_Warnings(t *testing.T) {
	tests := []struct {
		name           string
		snapshot       *Snapshot
		flags          Flags
		expectWarnings bool
		warnContains   string
	}{
		{
			name: "large project generates warning",
			snapshot: &Snapshot{
				Project:         ProjectHealth{Status: HealthOK, DirExists: true, MemoryDirExists: true, PlansDirExists: true},
				AIHealth:        map[string]AIHealth{"claude": {Status: HealthOK}},
				HasAnyLocalAI:   true,
				ExistingLocalAI: []string{"claude"},
				FileCount:       6000,
				IsLargeProject:  true,
			},
			flags:          Flags{},
			expectWarnings: true,
			warnContains:   "Large codebase",
		},
		{
			name: "preview mode generates warning",
			snapshot: &Snapshot{
				Project:         ProjectHealth{Status: HealthOK, DirExists: true, MemoryDirExists: true, PlansDirExists: true},
				AIHealth:        map[string]AIHealth{"claude": {Status: HealthOK}},
				HasAnyLocalAI:   true,
				ExistingLocalAI: []string{"claude"},
			},
			flags:          Flags{Preview: true},
			expectWarnings: true,
			warnContains:   "Preview mode",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan := DecidePlan(tt.snapshot, tt.flags)

			if tt.expectWarnings {
				if len(plan.Warnings) == 0 {
					t.Errorf("DecidePlan() expected warnings, got none")
				}
				found := false
				for _, w := range plan.Warnings {
					if containsString(w, tt.warnContains) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("DecidePlan() warnings should contain %q, got %v", tt.warnContains, plan.Warnings)
				}
			}
		})
	}
}

// TestProbeProjectHealth tests project health detection
func TestProbeProjectHealth(t *testing.T) {
	tests := []struct {
		name         string
		setup        func(dir string)
		expectStatus HealthStatus
	}{
		{
			name:         "missing - no directory",
			setup:        func(dir string) {}, // No setup
			expectStatus: HealthMissing,
		},
		{
			name: "partial - only taskwing dir",
			setup: func(dir string) {
				os.MkdirAll(filepath.Join(dir, ".taskwing"), 0755)
			},
			expectStatus: HealthPartial,
		},
		{
			name: "partial - missing plans",
			setup: func(dir string) {
				os.MkdirAll(filepath.Join(dir, ".taskwing", "memory"), 0755)
			},
			expectStatus: HealthPartial,
		},
		{
			name: "ok - all directories present",
			setup: func(dir string) {
				os.MkdirAll(filepath.Join(dir, ".taskwing", "memory"), 0755)
				os.MkdirAll(filepath.Join(dir, ".taskwing", "plans"), 0755)
			},
			expectStatus: HealthOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			tt.setup(tmpDir)

			health := probeProjectHealth(tmpDir)
			if health.Status != tt.expectStatus {
				t.Errorf("probeProjectHealth() status = %v, want %v (reason: %s)",
					health.Status, tt.expectStatus, health.Reason)
			}
		})
	}
}

// TestProbeAIHealth tests AI health detection
func TestProbeAIHealth(t *testing.T) {
	tests := []struct {
		name         string
		aiName       string
		setup        func(dir string)
		expectStatus HealthStatus
	}{
		{
			name:         "missing - no directory",
			aiName:       "claude",
			setup:        func(dir string) {},
			expectStatus: HealthMissing,
		},
		{
			name:   "partial - directory exists but no files",
			aiName: "claude",
			setup: func(dir string) {
				os.MkdirAll(filepath.Join(dir, ".claude", "commands"), 0755)
			},
			expectStatus: HealthPartial,
		},
		{
			name:   "partial - some command files",
			aiName: "claude",
			setup: func(dir string) {
				cmdDir := filepath.Join(dir, ".claude", "commands")
				os.MkdirAll(cmdDir, 0755)
				os.WriteFile(filepath.Join(cmdDir, "taskwing.md"), []byte("test"), 0644)
				os.WriteFile(filepath.Join(cmdDir, "tw-next.md"), []byte("test"), 0644)
			},
			expectStatus: HealthPartial,
		},
		{
			name:   "partial - commands ok but hooks missing (claude)",
			aiName: "claude",
			setup: func(dir string) {
				cmdDir := filepath.Join(dir, ".claude", "commands")
				os.MkdirAll(cmdDir, 0755)
				// Create all 7 command files
				for _, name := range []string{"taskwing", "tw-next", "tw-done", "tw-context", "tw-status", "tw-block", "tw-plan"} {
					os.WriteFile(filepath.Join(cmdDir, name+".md"), []byte("test"), 0644)
				}
			},
			expectStatus: HealthPartial, // Hooks missing
		},
		{
			name:   "ok - all files present (claude)",
			aiName: "claude",
			setup: func(dir string) {
				cmdDir := filepath.Join(dir, ".claude", "commands")
				os.MkdirAll(cmdDir, 0755)
				for _, name := range []string{"taskwing", "tw-next", "tw-done", "tw-context", "tw-status", "tw-block", "tw-plan"} {
					os.WriteFile(filepath.Join(cmdDir, name+".md"), []byte("test"), 0644)
				}
				// Add settings.json with valid JSON
				os.MkdirAll(filepath.Join(dir, ".claude"), 0755)
				os.WriteFile(filepath.Join(dir, ".claude", "settings.json"), []byte(`{"hooks":{}}`), 0644)
			},
			expectStatus: HealthOK,
		},
		{
			name:   "invalid - malformed hooks JSON (claude)",
			aiName: "claude",
			setup: func(dir string) {
				cmdDir := filepath.Join(dir, ".claude", "commands")
				os.MkdirAll(cmdDir, 0755)
				for _, name := range []string{"taskwing", "tw-next", "tw-done", "tw-context", "tw-status", "tw-block", "tw-plan"} {
					os.WriteFile(filepath.Join(cmdDir, name+".md"), []byte("test"), 0644)
				}
				os.MkdirAll(filepath.Join(dir, ".claude"), 0755)
				os.WriteFile(filepath.Join(dir, ".claude", "settings.json"), []byte(`{invalid json`), 0644)
			},
			expectStatus: HealthInvalid,
		},
		{
			name:   "ok - gemini with toml files",
			aiName: "gemini",
			setup: func(dir string) {
				cmdDir := filepath.Join(dir, ".gemini", "commands")
				os.MkdirAll(cmdDir, 0755)
				for _, name := range []string{"taskwing", "tw-next", "tw-done", "tw-context", "tw-status", "tw-block", "tw-plan"} {
					os.WriteFile(filepath.Join(cmdDir, name+".toml"), []byte("test"), 0644)
				}
			},
			expectStatus: HealthOK, // Gemini doesn't need hooks
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			tt.setup(tmpDir)

			health := probeAIHealth(tmpDir, tt.aiName)
			if health.Status != tt.expectStatus {
				t.Errorf("probeAIHealth(%q) status = %v, want %v (reason: %s)",
					tt.aiName, health.Status, tt.expectStatus, health.Reason)
			}
		})
	}
}

// TestFormatPlanSummary tests output formatting
func TestFormatPlanSummary(t *testing.T) {
	plan := &Plan{
		Mode:          ModeFirstTime,
		Actions:       []Action{ActionInitProject, ActionGenerateAIConfigs},
		DetectedState: "New project",
		ActionSummary: []string{"Create .taskwing/", "Generate AI configs"},
		Warnings:      []string{"Test warning"},
		Reasons:       []string{"No existing config"},
	}

	// Test quiet mode (single line)
	quietOutput := FormatPlanSummary(plan, true)
	if !containsString(quietOutput, "mode=first_time") {
		t.Errorf("FormatPlanSummary(quiet) should contain mode, got: %s", quietOutput)
	}

	// Test verbose mode
	verboseOutput := FormatPlanSummary(plan, false)
	if !containsString(verboseOutput, "Detected:") {
		t.Errorf("FormatPlanSummary(verbose) should contain 'Detected:', got: %s", verboseOutput)
	}
	if !containsString(verboseOutput, "Actions:") {
		t.Errorf("FormatPlanSummary(verbose) should contain 'Actions:', got: %s", verboseOutput)
	}
	if !containsString(verboseOutput, "Warnings:") {
		t.Errorf("FormatPlanSummary(verbose) should contain 'Warnings:', got: %s", verboseOutput)
	}
}

// TestProbeEnvironment integration test
func TestProbeEnvironment(t *testing.T) {
	tmpDir := t.TempDir()

	// Setup a realistic project
	os.MkdirAll(filepath.Join(tmpDir, ".taskwing", "memory"), 0755)
	os.MkdirAll(filepath.Join(tmpDir, ".taskwing", "plans"), 0755)
	os.MkdirAll(filepath.Join(tmpDir, ".git"), 0755)

	cmdDir := filepath.Join(tmpDir, ".claude", "commands")
	os.MkdirAll(cmdDir, 0755)
	for _, name := range []string{"taskwing", "tw-next", "tw-done", "tw-context", "tw-status", "tw-block", "tw-plan"} {
		os.WriteFile(filepath.Join(cmdDir, name+".md"), []byte("test"), 0644)
	}
	os.WriteFile(filepath.Join(tmpDir, ".claude", "settings.json"), []byte(`{"hooks":{}}`), 0644)

	snap, err := ProbeEnvironment(tmpDir)
	if err != nil {
		t.Fatalf("ProbeEnvironment() error: %v", err)
	}

	// Verify snapshot
	if snap.Project.Status != HealthOK {
		t.Errorf("snapshot.Project.Status = %v, want %v", snap.Project.Status, HealthOK)
	}

	if !snap.IsGitRepo {
		t.Errorf("snapshot.IsGitRepo = false, want true")
	}

	claudeHealth := snap.AIHealth["claude"]
	if claudeHealth.Status != HealthOK {
		t.Errorf("snapshot.AIHealth[claude].Status = %v, want %v (reason: %s)",
			claudeHealth.Status, HealthOK, claudeHealth.Reason)
	}
}

// TestContainsAction tests the containsAction helper
func TestContainsAction(t *testing.T) {
	actions := []Action{ActionInitProject, ActionGenerateAIConfigs, ActionIndexCode}

	tests := []struct {
		target Action
		want   bool
	}{
		{ActionInitProject, true},
		{ActionIndexCode, true},
		{ActionLLMAnalyze, false},
		{ActionExtractMetadata, false},
	}

	for _, tt := range tests {
		got := containsAction(actions, tt.target)
		if got != tt.want {
			t.Errorf("containsAction(%v) = %v, want %v", tt.target, got, tt.want)
		}
	}
}

// TestDetectProjectRoot tests project root detection
func TestDetectProjectRoot(t *testing.T) {
	// Create a temp dir with .git
	tmpDir := t.TempDir()
	gitDir := filepath.Join(tmpDir, ".git")
	os.MkdirAll(gitDir, 0755)

	// Create a nested directory
	nestedDir := filepath.Join(tmpDir, "src", "pkg", "deep")
	os.MkdirAll(nestedDir, 0755)

	tests := []struct {
		name     string
		basePath string
		wantRoot string
	}{
		{
			name:     "at git root",
			basePath: tmpDir,
			wantRoot: tmpDir,
		},
		{
			name:     "nested in git repo",
			basePath: nestedDir,
			wantRoot: tmpDir,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectProjectRoot(tt.basePath)
			if got != tt.wantRoot {
				t.Errorf("detectProjectRoot(%s) = %s, want %s", tt.basePath, got, tt.wantRoot)
			}
		})
	}
}

// TestIsGitRepository tests git repository detection
func TestIsGitRepository(t *testing.T) {
	// Create temp dirs
	gitRepo := t.TempDir()
	os.MkdirAll(filepath.Join(gitRepo, ".git"), 0755)

	nonGitDir := t.TempDir()

	tests := []struct {
		name     string
		basePath string
		want     bool
	}{
		{"git repo", gitRepo, true},
		{"non-git dir", nonGitDir, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isGitRepository(tt.basePath)
			if got != tt.want {
				t.Errorf("isGitRepository(%s) = %v, want %v", tt.basePath, got, tt.want)
			}
		})
	}
}

// TestCountSourceFiles tests source file counting
func TestCountSourceFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create some source files
	os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "util.go"), []byte("package main"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("# README"), 0644) // Not a source file

	// Create nested source files
	srcDir := filepath.Join(tmpDir, "src")
	os.MkdirAll(srcDir, 0755)
	os.WriteFile(filepath.Join(srcDir, "app.ts"), []byte("export {}"), 0644)
	os.WriteFile(filepath.Join(srcDir, "index.js"), []byte("module.exports = {}"), 0644)

	// Create node_modules (should be skipped)
	nodeModules := filepath.Join(tmpDir, "node_modules", "pkg")
	os.MkdirAll(nodeModules, 0755)
	os.WriteFile(filepath.Join(nodeModules, "index.js"), []byte("module.exports = {}"), 0644)

	count := countSourceFiles(tmpDir)

	// Should count: main.go, util.go, app.ts, index.js = 4
	// Should NOT count: README.md, node_modules/pkg/index.js
	if count != 4 {
		t.Errorf("countSourceFiles() = %d, want 4", count)
	}
}

// TestProbeAIHealth_AllAIs tests AI health probing for all supported AIs
func TestProbeAIHealth_AllAIs(t *testing.T) {
	tests := []struct {
		name         string
		aiName       string
		setup        func(dir string)
		expectStatus HealthStatus
	}{
		{
			name:         "copilot - missing",
			aiName:       "copilot",
			setup:        func(dir string) {},
			expectStatus: HealthMissing,
		},
		{
			name:   "copilot - ok",
			aiName: "copilot",
			setup: func(dir string) {
				cmdDir := filepath.Join(dir, ".github", "copilot-instructions")
				os.MkdirAll(cmdDir, 0755)
				for _, name := range []string{"taskwing", "tw-next", "tw-done", "tw-context", "tw-status", "tw-block", "tw-plan"} {
					os.WriteFile(filepath.Join(cmdDir, name+".md"), []byte("test"), 0644)
				}
			},
			expectStatus: HealthOK,
		},
		{
			name:   "codex - partial (commands ok, hooks missing)",
			aiName: "codex",
			setup: func(dir string) {
				cmdDir := filepath.Join(dir, ".codex", "commands")
				os.MkdirAll(cmdDir, 0755)
				for _, name := range []string{"taskwing", "tw-next", "tw-done", "tw-context", "tw-status", "tw-block", "tw-plan"} {
					os.WriteFile(filepath.Join(cmdDir, name+".md"), []byte("test"), 0644)
				}
			},
			expectStatus: HealthPartial, // Hooks missing
		},
		{
			name:   "codex - ok",
			aiName: "codex",
			setup: func(dir string) {
				cmdDir := filepath.Join(dir, ".codex", "commands")
				os.MkdirAll(cmdDir, 0755)
				for _, name := range []string{"taskwing", "tw-next", "tw-done", "tw-context", "tw-status", "tw-block", "tw-plan"} {
					os.WriteFile(filepath.Join(cmdDir, name+".md"), []byte("test"), 0644)
				}
				os.WriteFile(filepath.Join(dir, ".codex", "settings.json"), []byte(`{"hooks":{}}`), 0644)
			},
			expectStatus: HealthOK,
		},
		{
			name:   "cursor - ok",
			aiName: "cursor",
			setup: func(dir string) {
				cmdDir := filepath.Join(dir, ".cursor", "rules")
				os.MkdirAll(cmdDir, 0755)
				for _, name := range []string{"taskwing", "tw-next", "tw-done", "tw-context", "tw-status", "tw-block", "tw-plan"} {
					os.WriteFile(filepath.Join(cmdDir, name+".md"), []byte("test"), 0644)
				}
			},
			expectStatus: HealthOK, // Cursor doesn't need hooks
		},
		{
			name:         "unknown - unsupported",
			aiName:       "unknown-ai",
			setup:        func(dir string) {},
			expectStatus: HealthUnsupported,
		},
	}

	for _, tt := range tests {
		name := tt.aiName
		if tt.name != "" {
			name = tt.name
		}
		t.Run(name, func(t *testing.T) {
			tmpDir := t.TempDir()
			tt.setup(tmpDir)

			health := probeAIHealth(tmpDir, tt.aiName)
			if health.Status != tt.expectStatus {
				t.Errorf("probeAIHealth(%q) status = %v, want %v (reason: %s)",
					tt.aiName, health.Status, tt.expectStatus, health.Reason)
			}
		})
	}
}

// TestDecidePlan_RepairMode_LocalPartialWithoutGlobalMCP tests that partial local configs
// without global MCP are detected for repair
func TestDecidePlan_RepairMode_LocalPartialWithoutGlobalMCP(t *testing.T) {
	snapshot := &Snapshot{
		Project: ProjectHealth{Status: HealthOK, DirExists: true, MemoryDirExists: true, PlansDirExists: true},
		AIHealth: map[string]AIHealth{
			"claude": {
				Name:              "claude",
				Status:            HealthPartial,
				CommandsDirExists: true,  // Has local config
				CommandFilesCount: 3,     // But incomplete
				GlobalMCPExists:   false, // NO global MCP
				Reason:            "only 3/7 command files present",
			},
		},
		HasAnyLocalAI:   true,
		ExistingLocalAI: []string{"claude"},
		HasAnyGlobalMCP: false,
	}

	plan := DecidePlan(snapshot, Flags{})

	// Should be ModeRepair, not ModeRun
	if plan.Mode != ModeRepair {
		t.Errorf("DecidePlan() mode = %v, want %v (partial local config should trigger repair)",
			plan.Mode, ModeRepair)
	}

	// Should have claude in AIsNeedingRepair
	found := false
	for _, ai := range plan.AIsNeedingRepair {
		if ai == "claude" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("DecidePlan() AIsNeedingRepair = %v, should contain 'claude'", plan.AIsNeedingRepair)
	}
}

// TestProbeEnvironment_InvalidPath tests error handling for invalid paths
func TestProbeEnvironment_InvalidPath(t *testing.T) {
	// Non-existent path
	_, err := ProbeEnvironment("/non/existent/path/that/does/not/exist")
	if err == nil {
		t.Error("ProbeEnvironment() should return error for non-existent path")
	}

	// File instead of directory
	tmpFile := filepath.Join(t.TempDir(), "file.txt")
	os.WriteFile(tmpFile, []byte("test"), 0644)
	_, err = ProbeEnvironment(tmpFile)
	if err == nil {
		t.Error("ProbeEnvironment() should return error for file path")
	}
}

// TestGlobalMCPDetector_Injection tests the GlobalMCPDetector injection
func TestGlobalMCPDetector_Injection(t *testing.T) {
	// Reset after test
	originalDetector := GlobalMCPDetector
	defer func() { GlobalMCPDetector = originalDetector }()

	// Without injection
	GlobalMCPDetector = nil
	if checkGlobalMCPForAI("claude") {
		t.Error("checkGlobalMCPForAI() should return false when no detector is injected")
	}

	// With injection
	GlobalMCPDetector = func(aiName string) bool {
		return aiName == "claude"
	}
	if !checkGlobalMCPForAI("claude") {
		t.Error("checkGlobalMCPForAI('claude') should return true when detector returns true")
	}
	if checkGlobalMCPForAI("gemini") {
		t.Error("checkGlobalMCPForAI('gemini') should return false when detector returns false")
	}
}

// Helper
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && findSubstring(s, substr)
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
