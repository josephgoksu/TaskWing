package git

import (
	"errors"
	"strings"
	"testing"
)

// MockCommander is a test double for Commander that records calls and returns configured responses.
type MockCommander struct {
	// Calls records all commands that were executed
	Calls []MockCall
	// Responses maps command strings to their outputs/errors
	Responses map[string]MockResponse
}

// MockCall records a single command invocation.
type MockCall struct {
	Dir  string
	Name string
	Args []string
}

// MockResponse holds the output and error for a mocked command.
type MockResponse struct {
	Output string
	Error  error
}

// NewMockCommander creates a mock commander with pre-configured responses.
func NewMockCommander() *MockCommander {
	return &MockCommander{
		Responses: make(map[string]MockResponse),
	}
}

// Run implements Commander.Run
func (m *MockCommander) Run(name string, args ...string) (string, error) {
	return m.RunInDir("", name, args...)
}

// RunInDir implements Commander.RunInDir
func (m *MockCommander) RunInDir(dir, name string, args ...string) (string, error) {
	m.Calls = append(m.Calls, MockCall{Dir: dir, Name: name, Args: args})

	// Build key for lookup
	key := name + " " + strings.Join(args, " ")
	if resp, ok := m.Responses[key]; ok {
		return resp.Output, resp.Error
	}
	// Default: command succeeds with empty output
	return "", nil
}

// SetResponse configures the response for a command.
func (m *MockCommander) SetResponse(cmd string, output string, err error) {
	m.Responses[cmd] = MockResponse{Output: output, Error: err}
}

// LastCall returns the most recent command call.
func (m *MockCommander) LastCall() *MockCall {
	if len(m.Calls) == 0 {
		return nil
	}
	return &m.Calls[len(m.Calls)-1]
}

// TestIsGitInstalled verifies git installation check.
func TestIsGitInstalled(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(*MockCommander)
		expected bool
	}{
		{
			name: "git is installed",
			setup: func(m *MockCommander) {
				m.SetResponse("git --version", "git version 2.40.0", nil)
			},
			expected: true,
		},
		{
			name: "git is not installed",
			setup: func(m *MockCommander) {
				m.SetResponse("git --version", "", errors.New("executable not found"))
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockCommander()
			tt.setup(mock)
			client := NewClientWithCommander("/test/dir", mock)

			result := client.IsGitInstalled()
			if result != tt.expected {
				t.Errorf("IsGitInstalled() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// TestIsGhInstalled verifies gh CLI installation check.
func TestIsGhInstalled(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(*MockCommander)
		expected bool
	}{
		{
			name: "gh is installed",
			setup: func(m *MockCommander) {
				m.SetResponse("gh --version", "gh version 2.40.0", nil)
			},
			expected: true,
		},
		{
			name: "gh is not installed",
			setup: func(m *MockCommander) {
				m.SetResponse("gh --version", "", errors.New("executable not found"))
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockCommander()
			tt.setup(mock)
			client := NewClientWithCommander("/test/dir", mock)

			result := client.IsGhInstalled()
			if result != tt.expected {
				t.Errorf("IsGhInstalled() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// TestIsDirty verifies detection of uncommitted changes.
func TestIsDirty(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(*MockCommander)
		wantDirty   bool
		wantErr     bool
		errContains string
	}{
		{
			name: "clean working tree",
			setup: func(m *MockCommander) {
				m.SetResponse("git status --porcelain", "", nil)
			},
			wantDirty: false,
			wantErr:   false,
		},
		{
			name: "dirty with modified files",
			setup: func(m *MockCommander) {
				m.SetResponse("git status --porcelain", " M file.go\n?? new.txt", nil)
			},
			wantDirty: true,
			wantErr:   false,
		},
		{
			name: "git command fails",
			setup: func(m *MockCommander) {
				m.SetResponse("git status --porcelain", "", errors.New("not a git repository"))
			},
			wantDirty: false,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockCommander()
			tt.setup(mock)
			client := NewClientWithCommander("/test/dir", mock)

			dirty, err := client.IsDirty()
			if (err != nil) != tt.wantErr {
				t.Errorf("IsDirty() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if dirty != tt.wantDirty {
				t.Errorf("IsDirty() = %v, want %v", dirty, tt.wantDirty)
			}
		})
	}
}

// TestHasUnpushedCommits verifies detection of unpushed commits.
func TestHasUnpushedCommits(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(*MockCommander)
		wantAhead   bool
		wantErr     bool
		errContains string
	}{
		{
			name: "no unpushed commits",
			setup: func(m *MockCommander) {
				m.SetResponse("git rev-parse --abbrev-ref HEAD", "main", nil)
				m.SetResponse("git rev-parse --abbrev-ref main@{upstream}", "origin/main", nil)
				m.SetResponse("git rev-list --count main@{upstream}..HEAD", "0", nil)
			},
			wantAhead: false,
			wantErr:   false,
		},
		{
			name: "has unpushed commits",
			setup: func(m *MockCommander) {
				m.SetResponse("git rev-parse --abbrev-ref HEAD", "feature", nil)
				m.SetResponse("git rev-parse --abbrev-ref feature@{upstream}", "origin/feature", nil)
				m.SetResponse("git rev-list --count feature@{upstream}..HEAD", "3", nil)
			},
			wantAhead: true,
			wantErr:   false,
		},
		{
			name: "new branch without upstream",
			setup: func(m *MockCommander) {
				m.SetResponse("git rev-parse --abbrev-ref HEAD", "new-branch", nil)
				m.SetResponse("git rev-parse --abbrev-ref new-branch@{upstream}", "", errors.New("no upstream"))
			},
			wantAhead: false,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockCommander()
			tt.setup(mock)
			client := NewClientWithCommander("/test/dir", mock)

			ahead, err := client.HasUnpushedCommits()
			if (err != nil) != tt.wantErr {
				t.Errorf("HasUnpushedCommits() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if ahead != tt.wantAhead {
				t.Errorf("HasUnpushedCommits() = %v, want %v", ahead, tt.wantAhead)
			}
		})
	}
}

// TestCurrentBranch verifies getting current branch name.
func TestCurrentBranch(t *testing.T) {
	tests := []struct {
		name       string
		setup      func(*MockCommander)
		wantBranch string
		wantErr    bool
	}{
		{
			name: "on main branch",
			setup: func(m *MockCommander) {
				m.SetResponse("git rev-parse --abbrev-ref HEAD", "main", nil)
			},
			wantBranch: "main",
			wantErr:    false,
		},
		{
			name: "on feature branch",
			setup: func(m *MockCommander) {
				m.SetResponse("git rev-parse --abbrev-ref HEAD", "feat/plan-abc-feature", nil)
			},
			wantBranch: "feat/plan-abc-feature",
			wantErr:    false,
		},
		{
			name: "git command fails",
			setup: func(m *MockCommander) {
				m.SetResponse("git rev-parse --abbrev-ref HEAD", "", errors.New("not a git repo"))
			},
			wantBranch: "",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockCommander()
			tt.setup(mock)
			client := NewClientWithCommander("/test/dir", mock)

			branch, err := client.CurrentBranch()
			if (err != nil) != tt.wantErr {
				t.Errorf("CurrentBranch() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if branch != tt.wantBranch {
				t.Errorf("CurrentBranch() = %q, want %q", branch, tt.wantBranch)
			}
		})
	}
}

// TestDefaultBranch verifies detection of default branch.
func TestDefaultBranch(t *testing.T) {
	tests := []struct {
		name       string
		setup      func(*MockCommander)
		wantBranch string
		wantErr    bool
	}{
		{
			name: "default branch from remote HEAD",
			setup: func(m *MockCommander) {
				m.SetResponse("git symbolic-ref refs/remotes/origin/HEAD", "refs/remotes/origin/main", nil)
			},
			wantBranch: "main",
			wantErr:    false,
		},
		{
			name: "fallback to main when remote HEAD fails",
			setup: func(m *MockCommander) {
				m.SetResponse("git symbolic-ref refs/remotes/origin/HEAD", "", errors.New("not found"))
				m.SetResponse("git rev-parse --verify main", "abc123", nil)
			},
			wantBranch: "main",
			wantErr:    false,
		},
		{
			name: "fallback to master when main doesn't exist",
			setup: func(m *MockCommander) {
				m.SetResponse("git symbolic-ref refs/remotes/origin/HEAD", "", errors.New("not found"))
				m.SetResponse("git rev-parse --verify main", "", errors.New("not found"))
				m.SetResponse("git rev-parse --verify master", "abc123", nil)
			},
			wantBranch: "master",
			wantErr:    false,
		},
		{
			name: "error when no default branch found",
			setup: func(m *MockCommander) {
				m.SetResponse("git symbolic-ref refs/remotes/origin/HEAD", "", errors.New("not found"))
				m.SetResponse("git rev-parse --verify main", "", errors.New("not found"))
				m.SetResponse("git rev-parse --verify master", "", errors.New("not found"))
			},
			wantBranch: "",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockCommander()
			tt.setup(mock)
			client := NewClientWithCommander("/test/dir", mock)

			branch, err := client.DefaultBranch()
			if (err != nil) != tt.wantErr {
				t.Errorf("DefaultBranch() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if branch != tt.wantBranch {
				t.Errorf("DefaultBranch() = %q, want %q", branch, tt.wantBranch)
			}
		})
	}
}

// TestStash verifies stash operations.
func TestStash(t *testing.T) {
	tests := []struct {
		name    string
		message string
		setup   func(*MockCommander)
		wantErr bool
	}{
		{
			name:    "stash with message",
			message: "Auto-stash for Plan abc123",
			setup: func(m *MockCommander) {
				m.SetResponse("git stash push -m Auto-stash for Plan abc123", "", nil)
			},
			wantErr: false,
		},
		{
			name:    "stash without message",
			message: "",
			setup: func(m *MockCommander) {
				m.SetResponse("git stash push", "", nil)
			},
			wantErr: false,
		},
		{
			name:    "stash fails",
			message: "test",
			setup: func(m *MockCommander) {
				m.SetResponse("git stash push -m test", "", errors.New("stash failed"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockCommander()
			tt.setup(mock)
			client := NewClientWithCommander("/test/dir", mock)

			err := client.Stash(tt.message)
			if (err != nil) != tt.wantErr {
				t.Errorf("Stash() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestCheckout verifies branch checkout.
func TestCheckout(t *testing.T) {
	tests := []struct {
		name    string
		branch  string
		setup   func(*MockCommander)
		wantErr bool
	}{
		{
			name:   "successful checkout",
			branch: "main",
			setup: func(m *MockCommander) {
				m.SetResponse("git checkout main", "", nil)
			},
			wantErr: false,
		},
		{
			name:   "checkout fails",
			branch: "nonexistent",
			setup: func(m *MockCommander) {
				m.SetResponse("git checkout nonexistent", "", errors.New("branch not found"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockCommander()
			tt.setup(mock)
			client := NewClientWithCommander("/test/dir", mock)

			err := client.Checkout(tt.branch)
			if (err != nil) != tt.wantErr {
				t.Errorf("Checkout() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestCreateBranch verifies branch creation.
func TestCreateBranch(t *testing.T) {
	tests := []struct {
		name    string
		branch  string
		setup   func(*MockCommander)
		wantErr error
	}{
		{
			name:   "successful branch creation",
			branch: "feat/plan-abc-new-feature",
			setup: func(m *MockCommander) {
				m.SetResponse("git checkout -b feat/plan-abc-new-feature", "", nil)
			},
			wantErr: nil,
		},
		{
			name:   "branch already exists",
			branch: "existing-branch",
			setup: func(m *MockCommander) {
				m.SetResponse("git checkout -b existing-branch", "", errors.New("fatal: a branch named 'existing-branch' already exists"))
			},
			wantErr: ErrBranchExists,
		},
		{
			name:   "other error",
			branch: "bad-branch",
			setup: func(m *MockCommander) {
				m.SetResponse("git checkout -b bad-branch", "", errors.New("some other error"))
			},
			wantErr: errors.New("create branch"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockCommander()
			tt.setup(mock)
			client := NewClientWithCommander("/test/dir", mock)

			err := client.CreateBranch(tt.branch)
			if tt.wantErr == nil && err != nil {
				t.Errorf("CreateBranch() unexpected error = %v", err)
				return
			}
			if tt.wantErr != nil {
				if err == nil {
					t.Errorf("CreateBranch() expected error containing %v, got nil", tt.wantErr)
					return
				}
				if errors.Is(tt.wantErr, ErrBranchExists) {
					if !errors.Is(err, ErrBranchExists) {
						t.Errorf("CreateBranch() error = %v, want %v", err, ErrBranchExists)
					}
				} else if !strings.Contains(err.Error(), tt.wantErr.Error()) {
					t.Errorf("CreateBranch() error = %v, want containing %v", err, tt.wantErr)
				}
			}
		})
	}
}

// TestPull verifies pull operation.
func TestPull(t *testing.T) {
	tests := []struct {
		name    string
		remote  string
		branch  string
		setup   func(*MockCommander)
		wantErr bool
	}{
		{
			name:   "successful pull",
			remote: "origin",
			branch: "main",
			setup: func(m *MockCommander) {
				m.SetResponse("git pull origin main", "", nil)
			},
			wantErr: false,
		},
		{
			name:   "pull with conflicts",
			remote: "origin",
			branch: "main",
			setup: func(m *MockCommander) {
				m.SetResponse("git pull origin main", "", errors.New("merge conflict"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockCommander()
			tt.setup(mock)
			client := NewClientWithCommander("/test/dir", mock)

			err := client.Pull(tt.remote, tt.branch)
			if (err != nil) != tt.wantErr {
				t.Errorf("Pull() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestCommit verifies commit operation.
func TestCommit(t *testing.T) {
	tests := []struct {
		name    string
		message string
		setup   func(*MockCommander)
		wantErr bool
	}{
		{
			name:    "successful commit",
			message: "feat: add new feature",
			setup: func(m *MockCommander) {
				m.SetResponse("git commit -m feat: add new feature", "", nil)
			},
			wantErr: false,
		},
		{
			name:    "nothing to commit",
			message: "empty commit",
			setup: func(m *MockCommander) {
				m.SetResponse("git commit -m empty commit", "", errors.New("nothing to commit"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockCommander()
			tt.setup(mock)
			client := NewClientWithCommander("/test/dir", mock)

			err := client.Commit(tt.message)
			if (err != nil) != tt.wantErr {
				t.Errorf("Commit() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestPush verifies push operation.
func TestPush(t *testing.T) {
	tests := []struct {
		name    string
		remote  string
		branch  string
		setup   func(*MockCommander)
		wantErr bool
	}{
		{
			name:   "successful push",
			remote: "origin",
			branch: "feat/plan-abc-feature",
			setup: func(m *MockCommander) {
				m.SetResponse("git push origin feat/plan-abc-feature", "", nil)
			},
			wantErr: false,
		},
		{
			name:   "push rejected",
			remote: "origin",
			branch: "main",
			setup: func(m *MockCommander) {
				m.SetResponse("git push origin main", "", errors.New("rejected"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockCommander()
			tt.setup(mock)
			client := NewClientWithCommander("/test/dir", mock)

			err := client.Push(tt.remote, tt.branch)
			if (err != nil) != tt.wantErr {
				t.Errorf("Push() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestPushWithUpstream verifies push with -u flag.
func TestPushWithUpstream(t *testing.T) {
	mock := NewMockCommander()
	mock.SetResponse("git push -u origin feat/new-branch", "", nil)
	client := NewClientWithCommander("/test/dir", mock)

	err := client.PushWithUpstream("origin", "feat/new-branch")
	if err != nil {
		t.Errorf("PushWithUpstream() unexpected error = %v", err)
	}

	// Verify correct command was called
	call := mock.LastCall()
	if call == nil {
		t.Fatal("expected a command call")
	}
	if call.Name != "git" {
		t.Errorf("expected git command, got %s", call.Name)
	}
	expectedArgs := []string{"push", "-u", "origin", "feat/new-branch"}
	if len(call.Args) != len(expectedArgs) {
		t.Errorf("args = %v, want %v", call.Args, expectedArgs)
	}
}

// TestCreatePR verifies PR creation via gh CLI.
func TestCreatePR(t *testing.T) {
	tests := []struct {
		name    string
		title   string
		body    string
		base    string
		head    string
		setup   func(*MockCommander)
		wantURL string
		wantErr bool
	}{
		{
			name:  "successful PR creation",
			title: "Add new feature",
			body:  "## Summary\n- Task 1 completed",
			base:  "main",
			head:  "feat/plan-abc-feature",
			setup: func(m *MockCommander) {
				m.SetResponse("gh --version", "gh version 2.40.0", nil)
				m.SetResponse("gh pr create --title Add new feature --body ## Summary\n- Task 1 completed --base main --head feat/plan-abc-feature",
					"https://github.com/user/repo/pull/123", nil)
			},
			wantURL: "https://github.com/user/repo/pull/123",
			wantErr: false,
		},
		{
			name:  "gh not installed",
			title: "Test PR",
			body:  "body",
			base:  "main",
			head:  "feature",
			setup: func(m *MockCommander) {
				m.SetResponse("gh --version", "", errors.New("not found"))
			},
			wantURL: "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockCommander()
			tt.setup(mock)
			client := NewClientWithCommander("/test/dir", mock)

			url, err := client.CreatePR(tt.title, tt.body, tt.base, tt.head)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreatePR() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if url != tt.wantURL {
				t.Errorf("CreatePR() = %q, want %q", url, tt.wantURL)
			}
		})
	}
}

// TestBranchExists verifies branch existence check.
func TestBranchExists(t *testing.T) {
	tests := []struct {
		name   string
		branch string
		setup  func(*MockCommander)
		want   bool
	}{
		{
			name:   "branch exists",
			branch: "main",
			setup: func(m *MockCommander) {
				m.SetResponse("git rev-parse --verify main", "abc123", nil)
			},
			want: true,
		},
		{
			name:   "branch does not exist",
			branch: "nonexistent",
			setup: func(m *MockCommander) {
				m.SetResponse("git rev-parse --verify nonexistent", "", errors.New("not found"))
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockCommander()
			tt.setup(mock)
			client := NewClientWithCommander("/test/dir", mock)

			exists := client.BranchExists(tt.branch)
			if exists != tt.want {
				t.Errorf("BranchExists() = %v, want %v", exists, tt.want)
			}
		})
	}
}

// TestAddAll verifies staging all changes.
func TestAddAll(t *testing.T) {
	mock := NewMockCommander()
	mock.SetResponse("git add .", "", nil)
	client := NewClientWithCommander("/test/dir", mock)

	err := client.AddAll()
	if err != nil {
		t.Errorf("AddAll() unexpected error = %v", err)
	}

	call := mock.LastCall()
	if call == nil {
		t.Fatal("expected a command call")
	}
	if call.Dir != "/test/dir" {
		t.Errorf("command dir = %s, want /test/dir", call.Dir)
	}
}

// TestIsRepository verifies repository detection.
func TestIsRepository(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*MockCommander)
		want  bool
	}{
		{
			name: "is a git repository",
			setup: func(m *MockCommander) {
				m.SetResponse("git rev-parse --is-inside-work-tree", "true", nil)
			},
			want: true,
		},
		{
			name: "not a git repository",
			setup: func(m *MockCommander) {
				m.SetResponse("git rev-parse --is-inside-work-tree", "", errors.New("not a git repository"))
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockCommander()
			tt.setup(mock)
			client := NewClientWithCommander("/test/dir", mock)

			isRepo := client.IsRepository()
			if isRepo != tt.want {
				t.Errorf("IsRepository() = %v, want %v", isRepo, tt.want)
			}
		})
	}
}
