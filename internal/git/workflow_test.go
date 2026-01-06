package git

import (
	"errors"
	"strings"
	"testing"
)

// TestSlugify verifies string slugification.
func TestSlugify(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "simple lowercase",
			input: "hello world",
			want:  "hello-world",
		},
		{
			name:  "mixed case",
			input: "Hello World",
			want:  "hello-world",
		},
		{
			name:  "special characters",
			input: "Add User Authentication!",
			want:  "add-user-authentication",
		},
		{
			name:  "underscores",
			input: "add_new_feature",
			want:  "add-new-feature",
		},
		{
			name:  "multiple spaces",
			input: "hello   world",
			want:  "hello-world",
		},
		{
			name:  "leading trailing spaces",
			input: "  hello world  ",
			want:  "hello-world",
		},
		{
			name:  "numbers",
			input: "version 2.0 release",
			want:  "version-20-release",
		},
		{
			name:  "unicode characters",
			input: "café résumé",
			want:  "caf-rsum",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "only special chars",
			input: "!!!???",
			want:  "",
		},
		{
			name:  "hyphens preserved",
			input: "pre-existing-slug",
			want:  "pre-existing-slug",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Slugify(tt.input)
			if got != tt.want {
				t.Errorf("Slugify(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// TestGenerateBranchName verifies branch name generation.
func TestGenerateBranchName(t *testing.T) {
	tests := []struct {
		name      string
		planID    string
		planTitle string
		want      string
	}{
		{
			name:      "short ID with title",
			planID:    "abc123",
			planTitle: "Add Authentication",
			want:      "feat/plan-abc123-add-authentication",
		},
		{
			name:      "long ID truncated",
			planID:    "plan-1234567890abcdef",
			planTitle: "New Feature",
			want:      "feat/plan-90abcdef-new-feature",
		},
		{
			name:      "empty title",
			planID:    "abc123",
			planTitle: "",
			want:      "feat/plan-abc123",
		},
		{
			name:      "special chars in title",
			planID:    "xyz789",
			planTitle: "Fix Bug #123!",
			want:      "feat/plan-xyz789-fix-bug-123",
		},
		{
			name:      "very long title truncated",
			planID:    "short",
			planTitle: "This is a very long title that should be truncated to keep the branch name reasonable in length",
			want:      "feat/plan-short-this-is-a-very-long",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GenerateBranchName(tt.planID, tt.planTitle)
			if got != tt.want {
				t.Errorf("GenerateBranchName(%q, %q) = %q, want %q", tt.planID, tt.planTitle, got, tt.want)
			}
		})
	}
}

// TestStartPlanWorkflow verifies the complete workflow.
func TestStartPlanWorkflow(t *testing.T) {
	tests := []struct {
		name              string
		planID            string
		planTitle         string
		skipUnpushedCheck bool
		setup             func(*MockCommander)
		wantResult        *WorkflowResult
		wantErr           bool
		wantErrType       error
	}{
		{
			name:      "successful workflow - clean state",
			planID:    "plan-abc",
			planTitle: "New Feature",
			setup: func(m *MockCommander) {
				m.SetResponse("git --version", "git version 2.40.0", nil)
				m.SetResponse("git rev-parse --is-inside-work-tree", "true", nil)
				m.SetResponse("git rev-parse --abbrev-ref HEAD", "main", nil)
				m.SetResponse("git status --porcelain", "", nil)
				m.SetResponse("git rev-parse --abbrev-ref main@{upstream}", "origin/main", nil)
				m.SetResponse("git rev-list --count main@{upstream}..HEAD", "0", nil)
				m.SetResponse("git symbolic-ref refs/remotes/origin/HEAD", "refs/remotes/origin/main", nil)
				m.SetResponse("git remote get-url origin", "https://github.com/user/repo.git", nil)
				m.SetResponse("git pull origin main", "", nil)
				m.SetResponse("git checkout -b feat/plan-plan-abc-new-feature", "", nil)
			},
			wantResult: &WorkflowResult{
				BranchName:     "feat/plan-plan-abc-new-feature",
				DefaultBranch:  "main",
				WasStashed:     false,
				PreviousBranch: "main",
			},
			wantErr: false,
		},
		{
			name:      "workflow with dirty state - auto stash",
			planID:    "plan-xyz",
			planTitle: "Bug Fix",
			setup: func(m *MockCommander) {
				m.SetResponse("git --version", "git version 2.40.0", nil)
				m.SetResponse("git rev-parse --is-inside-work-tree", "true", nil)
				m.SetResponse("git rev-parse --abbrev-ref HEAD", "feature-old", nil)
				m.SetResponse("git status --porcelain", " M file.go", nil)
				m.SetResponse("git stash push -m Auto-stash for plan plan-xyz", "", nil)
				m.SetResponse("git rev-parse --abbrev-ref feature-old@{upstream}", "origin/feature-old", nil)
				m.SetResponse("git rev-list --count feature-old@{upstream}..HEAD", "0", nil)
				m.SetResponse("git symbolic-ref refs/remotes/origin/HEAD", "refs/remotes/origin/main", nil)
				m.SetResponse("git checkout main", "", nil)
				m.SetResponse("git remote get-url origin", "https://github.com/user/repo.git", nil)
				m.SetResponse("git pull origin main", "", nil)
				m.SetResponse("git checkout -b feat/plan-plan-xyz-bug-fix", "", nil)
			},
			wantResult: &WorkflowResult{
				BranchName:     "feat/plan-plan-xyz-bug-fix",
				DefaultBranch:  "main",
				WasStashed:     true,
				PreviousBranch: "feature-old",
			},
			wantErr: false,
		},
		{
			name:      "workflow blocked by unpushed commits",
			planID:    "plan-123",
			planTitle: "Test",
			setup: func(m *MockCommander) {
				m.SetResponse("git --version", "git version 2.40.0", nil)
				m.SetResponse("git rev-parse --is-inside-work-tree", "true", nil)
				m.SetResponse("git rev-parse --abbrev-ref HEAD", "feature-wip", nil)
				m.SetResponse("git status --porcelain", "", nil)
				m.SetResponse("git rev-parse --abbrev-ref feature-wip@{upstream}", "origin/feature-wip", nil)
				m.SetResponse("git rev-list --count feature-wip@{upstream}..HEAD", "3", nil)
			},
			wantResult:  nil,
			wantErr:     true,
			wantErrType: &UnpushedCommitsError{},
		},
		{
			name:              "workflow proceeds with unpushed commits when skipped",
			planID:            "plan-456",
			planTitle:         "Proceed Anyway",
			skipUnpushedCheck: true,
			setup: func(m *MockCommander) {
				m.SetResponse("git --version", "git version 2.40.0", nil)
				m.SetResponse("git rev-parse --is-inside-work-tree", "true", nil)
				m.SetResponse("git rev-parse --abbrev-ref HEAD", "feature-wip", nil)
				m.SetResponse("git status --porcelain", "", nil)
				// HasUnpushedCommits is not called when skipUnpushedCheck is true
				m.SetResponse("git symbolic-ref refs/remotes/origin/HEAD", "refs/remotes/origin/main", nil)
				m.SetResponse("git checkout main", "", nil)
				m.SetResponse("git remote get-url origin", "https://github.com/user/repo.git", nil)
				m.SetResponse("git pull origin main", "", nil)
				m.SetResponse("git checkout -b feat/plan-plan-456-proceed-anyway", "", nil)
			},
			wantResult: &WorkflowResult{
				BranchName:     "feat/plan-plan-456-proceed-anyway",
				DefaultBranch:  "main",
				WasStashed:     false,
				PreviousBranch: "feature-wip",
			},
			wantErr: false,
		},
		{
			name:      "empty plan ID fails",
			planID:    "",
			planTitle: "Test",
			setup:     func(m *MockCommander) {},
			wantErr:   true,
		},
		{
			name:      "git not installed",
			planID:    "plan-123",
			planTitle: "Test",
			setup: func(m *MockCommander) {
				m.SetResponse("git --version", "", errors.New("not found"))
			},
			wantErr: true,
		},
		{
			name:      "not a git repository",
			planID:    "plan-123",
			planTitle: "Test",
			setup: func(m *MockCommander) {
				m.SetResponse("git --version", "git version 2.40.0", nil)
				m.SetResponse("git rev-parse --is-inside-work-tree", "", errors.New("not a git repository"))
			},
			wantErr: true,
		},
		{
			name:      "branch already exists - checkout instead",
			planID:    "plan-exist",
			planTitle: "Existing",
			setup: func(m *MockCommander) {
				m.SetResponse("git --version", "git version 2.40.0", nil)
				m.SetResponse("git rev-parse --is-inside-work-tree", "true", nil)
				m.SetResponse("git rev-parse --abbrev-ref HEAD", "main", nil)
				m.SetResponse("git status --porcelain", "", nil)
				m.SetResponse("git rev-parse --abbrev-ref main@{upstream}", "origin/main", nil)
				m.SetResponse("git rev-list --count main@{upstream}..HEAD", "0", nil)
				m.SetResponse("git symbolic-ref refs/remotes/origin/HEAD", "refs/remotes/origin/main", nil)
				m.SetResponse("git remote get-url origin", "https://github.com/user/repo.git", nil)
				m.SetResponse("git pull origin main", "", nil)
				m.SetResponse("git checkout -b feat/plan-an-exist-existing", "", errors.New("fatal: a branch named 'feat/plan-an-exist-existing' already exists"))
				m.SetResponse("git checkout feat/plan-an-exist-existing", "", nil)
			},
			wantResult: &WorkflowResult{
				BranchName:     "feat/plan-an-exist-existing",
				DefaultBranch:  "main",
				WasStashed:     false,
				PreviousBranch: "main",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockCommander()
			tt.setup(mock)
			client := NewClientWithCommander("/test/dir", mock)

			result, err := client.StartPlanWorkflow(tt.planID, tt.planTitle, tt.skipUnpushedCheck)

			if (err != nil) != tt.wantErr {
				t.Errorf("StartPlanWorkflow() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErrType != nil {
				if !IsUnpushedCommitsError(err) {
					t.Errorf("StartPlanWorkflow() error type = %T, want UnpushedCommitsError", err)
				}
				return
			}

			if tt.wantResult != nil {
				if result == nil {
					t.Fatal("StartPlanWorkflow() result is nil, want non-nil")
				}
				if result.BranchName != tt.wantResult.BranchName {
					t.Errorf("BranchName = %q, want %q", result.BranchName, tt.wantResult.BranchName)
				}
				if result.DefaultBranch != tt.wantResult.DefaultBranch {
					t.Errorf("DefaultBranch = %q, want %q", result.DefaultBranch, tt.wantResult.DefaultBranch)
				}
				if result.WasStashed != tt.wantResult.WasStashed {
					t.Errorf("WasStashed = %v, want %v", result.WasStashed, tt.wantResult.WasStashed)
				}
				if result.PreviousBranch != tt.wantResult.PreviousBranch {
					t.Errorf("PreviousBranch = %q, want %q", result.PreviousBranch, tt.wantResult.PreviousBranch)
				}
			}
		})
	}
}

// TestIsUnpushedCommitsError verifies error type checking.
func TestIsUnpushedCommitsError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "is UnpushedCommitsError",
			err:  &UnpushedCommitsError{Branch: "feature"},
			want: true,
		},
		{
			name: "is regular error",
			err:  errors.New("some error"),
			want: false,
		},
		{
			name: "is nil",
			err:  nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsUnpushedCommitsError(tt.err)
			if got != tt.want {
				t.Errorf("IsUnpushedCommitsError() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestDetermineCommitType verifies commit type inference.
func TestDetermineCommitType(t *testing.T) {
	tests := []struct {
		name      string
		taskTitle string
		taskType  string
		want      string
	}{
		{
			name:      "explicit fix type",
			taskTitle: "Update something",
			taskType:  "fix",
			want:      "fix",
		},
		{
			name:      "explicit feature type",
			taskTitle: "Something",
			taskType:  "feature",
			want:      "feat",
		},
		{
			name:      "infer fix from title",
			taskTitle: "Fix login bug",
			taskType:  "",
			want:      "fix",
		},
		{
			name:      "infer test from title",
			taskTitle: "Add unit tests",
			taskType:  "",
			want:      "test",
		},
		{
			name:      "infer refactor from title",
			taskTitle: "Refactor auth module",
			taskType:  "",
			want:      "refactor",
		},
		{
			name:      "default to feat",
			taskTitle: "Add new endpoint",
			taskType:  "",
			want:      "feat",
		},
		{
			name:      "docs type",
			taskTitle: "Update documentation",
			taskType:  "docs",
			want:      "docs",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := determineCommitType(tt.taskTitle, tt.taskType)
			if got != tt.want {
				t.Errorf("determineCommitType(%q, %q) = %q, want %q", tt.taskTitle, tt.taskType, got, tt.want)
			}
		})
	}
}

// TestGeneratePRBody verifies PR body generation.
func TestGeneratePRBody(t *testing.T) {
	tests := []struct {
		name         string
		planGoal     string
		tasks        []TaskInfo
		wantContains []string
	}{
		{
			name:     "with tasks and summaries",
			planGoal: "Add user authentication",
			tasks: []TaskInfo{
				{Title: "Create login form", Summary: "Implemented with validation"},
				{Title: "Add session management", Summary: "Using JWT tokens"},
			},
			wantContains: []string{
				"## Summary",
				"Add user authentication",
				"## Completed Tasks",
				"- [x] Create login form",
				"Implemented with validation",
				"- [x] Add session management",
				"Using JWT tokens",
				"*Generated by TaskWing*",
			},
		},
		{
			name:     "with tasks without summaries",
			planGoal: "Fix bugs",
			tasks: []TaskInfo{
				{Title: "Fix login bug"},
				{Title: "Fix logout bug"},
			},
			wantContains: []string{
				"Fix bugs",
				"- [x] Fix login bug",
				"- [x] Fix logout bug",
			},
		},
		{
			name:     "no tasks",
			planGoal: "Empty plan",
			tasks:    []TaskInfo{},
			wantContains: []string{
				"## Summary",
				"Empty plan",
				"*Generated by TaskWing*",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := GeneratePRBody(tt.planGoal, tt.tasks)
			for _, want := range tt.wantContains {
				if !strings.Contains(body, want) {
					t.Errorf("GeneratePRBody() missing %q in:\n%s", want, body)
				}
			}
		})
	}
}

// TestCreatePlanPR verifies PR creation workflow.
func TestCreatePlanPR(t *testing.T) {
	tests := []struct {
		name       string
		planGoal   string
		tasks      []TaskInfo
		baseBranch string
		setup      func(*MockCommander)
		wantURL    string
		wantErr    bool
	}{
		{
			name:       "successful PR creation",
			planGoal:   "Add new feature",
			tasks:      []TaskInfo{{Title: "Task 1"}},
			baseBranch: "main",
			setup: func(m *MockCommander) {
				m.SetResponse("gh --version", "gh version 2.40.0", nil)
				m.SetResponse("git rev-parse --abbrev-ref HEAD", "feat/plan-abc-feature", nil)
				m.SetResponse("git rev-parse --abbrev-ref feat/plan-abc-feature@{upstream}", "origin/feat/plan-abc-feature", nil)
				m.SetResponse("git rev-list --count feat/plan-abc-feature@{upstream}..HEAD", "0", nil)
				m.SetResponse("git push origin feat/plan-abc-feature", "", nil)
				// The PR body will be generated, we just need to match the command pattern
			},
			wantURL: "https://github.com/user/repo/pull/123",
			wantErr: false,
		},
		{
			name:     "gh not installed",
			planGoal: "Test",
			tasks:    []TaskInfo{},
			setup: func(m *MockCommander) {
				m.SetResponse("gh --version", "", errors.New("not found"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockCommander()
			tt.setup(mock)

			// For the successful case, set up the PR create response
			if tt.wantURL != "" {
				// We need to match the gh pr create command - the body will vary
				// So we add a catch-all for any gh pr create command
				for key := range mock.Responses {
					if strings.HasPrefix(key, "gh pr create") {
						mock.Responses[key] = MockResponse{Output: tt.wantURL, Error: nil}
					}
				}
				// Also set a generic response for the pr create
				mock.SetResponse("gh pr create --title Add new feature --body ## Summary\n\nAdd new feature\n\n## Completed Tasks\n\n- [x] Task 1\n\n---\n*Generated by TaskWing*\n --base main --head feat/plan-abc-feature",
					tt.wantURL, nil)
			}

			client := NewClientWithCommander("/test/dir", mock)

			prInfo, err := client.CreatePlanPR(tt.planGoal, tt.tasks, tt.baseBranch)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreatePlanPR() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && prInfo != nil {
				if prInfo.URL != tt.wantURL {
					t.Errorf("CreatePlanPR() URL = %q, want %q", prInfo.URL, tt.wantURL)
				}
			}
		})
	}
}

// TestCommitTaskProgress verifies task commit logic.
func TestCommitTaskProgress(t *testing.T) {
	tests := []struct {
		name      string
		taskTitle string
		taskType  string
		setup     func(*MockCommander)
		wantErr   bool
	}{
		{
			name:      "successful commit",
			taskTitle: "Add new feature",
			taskType:  "feat",
			setup: func(m *MockCommander) {
				m.SetResponse("git add .", "", nil)
				m.SetResponse("git status --porcelain", " M file.go", nil)
				m.SetResponse("git commit -m feat: Add new feature", "", nil)
			},
			wantErr: false,
		},
		{
			name:      "nothing to commit",
			taskTitle: "No changes",
			taskType:  "",
			setup: func(m *MockCommander) {
				m.SetResponse("git add .", "", nil)
				m.SetResponse("git status --porcelain", "", nil)
			},
			wantErr: false,
		},
		{
			name:      "add fails",
			taskTitle: "Task",
			taskType:  "",
			setup: func(m *MockCommander) {
				m.SetResponse("git add .", "", errors.New("permission denied"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockCommander()
			tt.setup(mock)
			client := NewClientWithCommander("/test/dir", mock)

			err := client.CommitTaskProgress(tt.taskTitle, tt.taskType)
			if (err != nil) != tt.wantErr {
				t.Errorf("CommitTaskProgress() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
