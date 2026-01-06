// Package git provides shell-based wrappers for git and gh CLI commands.
// It uses os/exec instead of go-git to ensure compatibility with user's
// SSH keys, GPG signing config, and other shell environment settings.
package git

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// Common errors returned by git operations.
var (
	ErrGitNotInstalled    = errors.New("git is not installed or not in PATH")
	ErrGhNotInstalled     = errors.New("gh CLI is not installed or not in PATH")
	ErrNotGitRepository   = errors.New("not a git repository")
	ErrUnpushedCommits    = errors.New("unpushed commits exist on current branch")
	ErrDirtyWorkingTree   = errors.New("working tree has uncommitted changes")
	ErrBranchExists       = errors.New("branch already exists")
	ErrBranchNotFound     = errors.New("branch not found")
	ErrNoRemoteConfigured = errors.New("no remote configured for this repository")
)

// Commander is an interface for executing commands.
// This allows mocking in tests.
type Commander interface {
	Run(name string, args ...string) (string, error)
	RunInDir(dir, name string, args ...string) (string, error)
}

// ShellCommander executes real shell commands.
type ShellCommander struct{}

// Run executes a command in the current directory.
func (c *ShellCommander) Run(name string, args ...string) (string, error) {
	return c.RunInDir("", name, args...)
}

// RunInDir executes a command in the specified directory.
func (c *ShellCommander) RunInDir(dir, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		// Include stderr in error for debugging
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg != "" {
			return "", fmt.Errorf("%w: %s", err, errMsg)
		}
		return "", err
	}
	return strings.TrimSpace(stdout.String()), nil
}

// Client wraps git and gh CLI operations.
type Client struct {
	commander Commander
	workDir   string
}

// NewClient creates a new git client for the given directory.
func NewClient(workDir string) *Client {
	return &Client{
		commander: &ShellCommander{},
		workDir:   workDir,
	}
}

// NewClientWithCommander creates a client with a custom commander (for testing).
func NewClientWithCommander(workDir string, commander Commander) *Client {
	return &Client{
		commander: commander,
		workDir:   workDir,
	}
}

// IsGitInstalled checks if git binary is available in PATH.
func (c *Client) IsGitInstalled() bool {
	_, err := c.commander.Run("git", "--version")
	return err == nil
}

// IsGhInstalled checks if gh CLI binary is available in PATH.
func (c *Client) IsGhInstalled() bool {
	_, err := c.commander.Run("gh", "--version")
	return err == nil
}

// IsRepository checks if the working directory is a git repository.
func (c *Client) IsRepository() bool {
	_, err := c.commander.RunInDir(c.workDir, "git", "rev-parse", "--is-inside-work-tree")
	return err == nil
}

// IsDirty checks if the working directory has uncommitted changes.
func (c *Client) IsDirty() (bool, error) {
	output, err := c.commander.RunInDir(c.workDir, "git", "status", "--porcelain")
	if err != nil {
		return false, fmt.Errorf("check dirty state: %w", err)
	}
	return output != "", nil
}

// HasUnpushedCommits checks if the current branch has commits not pushed to remote.
func (c *Client) HasUnpushedCommits() (bool, error) {
	branch, err := c.CurrentBranch()
	if err != nil {
		return false, err
	}

	// Check if branch has an upstream configured
	_, err = c.commander.RunInDir(c.workDir, "git", "rev-parse", "--abbrev-ref", branch+"@{upstream}")
	if err != nil {
		// No upstream configured - could be a new branch, not an error
		return false, nil
	}

	// Count commits ahead of upstream
	output, err := c.commander.RunInDir(c.workDir, "git", "rev-list", "--count", branch+"@{upstream}..HEAD")
	if err != nil {
		return false, fmt.Errorf("check unpushed commits: %w", err)
	}

	return output != "0", nil
}

// CurrentBranch returns the name of the current branch.
func (c *Client) CurrentBranch() (string, error) {
	output, err := c.commander.RunInDir(c.workDir, "git", "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", fmt.Errorf("get current branch: %w", err)
	}
	return output, nil
}

// DefaultBranch returns the default branch name (main or master).
func (c *Client) DefaultBranch() (string, error) {
	// Try to get from remote HEAD reference
	output, err := c.commander.RunInDir(c.workDir, "git", "symbolic-ref", "refs/remotes/origin/HEAD")
	if err == nil {
		// Output is like "refs/remotes/origin/main"
		parts := strings.Split(output, "/")
		if len(parts) > 0 {
			return parts[len(parts)-1], nil
		}
	}

	// Fallback: check if main or master exists
	for _, branch := range []string{"main", "master"} {
		_, err := c.commander.RunInDir(c.workDir, "git", "rev-parse", "--verify", branch)
		if err == nil {
			return branch, nil
		}
	}

	return "", fmt.Errorf("could not determine default branch")
}

// Stash saves uncommitted changes to the stash.
func (c *Client) Stash(message string) error {
	args := []string{"stash", "push"}
	if message != "" {
		args = append(args, "-m", message)
	}
	_, err := c.commander.RunInDir(c.workDir, "git", args...)
	if err != nil {
		return fmt.Errorf("stash changes: %w", err)
	}
	return nil
}

// StashPop restores the most recent stash.
func (c *Client) StashPop() error {
	_, err := c.commander.RunInDir(c.workDir, "git", "stash", "pop")
	if err != nil {
		return fmt.Errorf("pop stash: %w", err)
	}
	return nil
}

// Checkout switches to the specified branch.
func (c *Client) Checkout(branch string) error {
	_, err := c.commander.RunInDir(c.workDir, "git", "checkout", branch)
	if err != nil {
		return fmt.Errorf("checkout %s: %w", branch, err)
	}
	return nil
}

// CreateBranch creates and checks out a new branch.
func (c *Client) CreateBranch(name string) error {
	_, err := c.commander.RunInDir(c.workDir, "git", "checkout", "-b", name)
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			return ErrBranchExists
		}
		return fmt.Errorf("create branch %s: %w", name, err)
	}
	return nil
}

// Pull fetches and merges changes from the remote.
func (c *Client) Pull(remote, branch string) error {
	_, err := c.commander.RunInDir(c.workDir, "git", "pull", remote, branch)
	if err != nil {
		return fmt.Errorf("pull %s/%s: %w", remote, branch, err)
	}
	return nil
}

// Fetch fetches updates from the remote without merging.
func (c *Client) Fetch(remote string) error {
	_, err := c.commander.RunInDir(c.workDir, "git", "fetch", remote)
	if err != nil {
		return fmt.Errorf("fetch %s: %w", remote, err)
	}
	return nil
}

// Add stages files for commit.
func (c *Client) Add(paths ...string) error {
	args := append([]string{"add"}, paths...)
	_, err := c.commander.RunInDir(c.workDir, "git", args...)
	if err != nil {
		return fmt.Errorf("add files: %w", err)
	}
	return nil
}

// AddAll stages all changes.
func (c *Client) AddAll() error {
	return c.Add(".")
}

// Commit creates a commit with the given message.
func (c *Client) Commit(message string) error {
	_, err := c.commander.RunInDir(c.workDir, "git", "commit", "-m", message)
	if err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

// Push pushes the current branch to the remote.
func (c *Client) Push(remote, branch string) error {
	_, err := c.commander.RunInDir(c.workDir, "git", "push", remote, branch)
	if err != nil {
		return fmt.Errorf("push %s/%s: %w", remote, branch, err)
	}
	return nil
}

// PushWithUpstream pushes and sets upstream tracking.
func (c *Client) PushWithUpstream(remote, branch string) error {
	_, err := c.commander.RunInDir(c.workDir, "git", "push", "-u", remote, branch)
	if err != nil {
		return fmt.Errorf("push with upstream %s/%s: %w", remote, branch, err)
	}
	return nil
}

// BranchExists checks if a branch exists locally.
func (c *Client) BranchExists(name string) bool {
	_, err := c.commander.RunInDir(c.workDir, "git", "rev-parse", "--verify", name)
	return err == nil
}

// RemoteURL returns the URL of the specified remote.
func (c *Client) RemoteURL(remote string) (string, error) {
	output, err := c.commander.RunInDir(c.workDir, "git", "remote", "get-url", remote)
	if err != nil {
		return "", fmt.Errorf("get remote URL: %w", err)
	}
	return output, nil
}

// HasRemote checks if a remote is configured.
func (c *Client) HasRemote(name string) bool {
	_, err := c.commander.RunInDir(c.workDir, "git", "remote", "get-url", name)
	return err == nil
}

// CreatePR creates a pull request using gh CLI.
func (c *Client) CreatePR(title, body, base, head string) (string, error) {
	if !c.IsGhInstalled() {
		return "", ErrGhNotInstalled
	}

	output, err := c.commander.RunInDir(c.workDir, "gh", "pr", "create",
		"--title", title,
		"--body", body,
		"--base", base,
		"--head", head,
	)
	if err != nil {
		return "", fmt.Errorf("create PR: %w", err)
	}
	return output, nil
}
