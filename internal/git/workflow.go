package git

import (
	"fmt"
	"regexp"
	"strings"
)

// WorkflowResult contains the outcome of a workflow operation.
type WorkflowResult struct {
	// BranchName is the name of the created feature branch
	BranchName string
	// DefaultBranch is the detected default branch (main/master)
	DefaultBranch string
	// WasStashed indicates if changes were auto-stashed
	WasStashed bool
	// PreviousBranch is the branch we were on before switching
	PreviousBranch string
}

// UnpushedCommitsError is returned when unpushed commits exist and user confirmation is needed.
type UnpushedCommitsError struct {
	Branch      string
	CommitCount string
}

func (e *UnpushedCommitsError) Error() string {
	return fmt.Sprintf("unpushed commits exist on branch %q", e.Branch)
}

// IsUnpushedCommitsError checks if an error is an UnpushedCommitsError.
func IsUnpushedCommitsError(err error) bool {
	_, ok := err.(*UnpushedCommitsError)
	return ok
}

// StartPlanWorkflow orchestrates the git context switch for starting a new plan.
// It handles:
// 1. Auto-stashing dirty changes
// 2. Checking for unpushed commits (returns error for CLI to prompt)
// 3. Switching to default branch and pulling latest
// 4. Creating and checking out the feature branch
//
// If skipUnpushedCheck is true, the workflow proceeds even with unpushed commits.
func (c *Client) StartPlanWorkflow(planID, planTitle string, skipUnpushedCheck bool) (*WorkflowResult, error) {
	result := &WorkflowResult{}

	// Validate inputs
	if planID == "" {
		return nil, fmt.Errorf("plan ID is required")
	}

	// Check if git is available
	if !c.IsGitInstalled() {
		return nil, ErrGitNotInstalled
	}

	// Check if we're in a git repository
	if !c.IsRepository() {
		return nil, ErrNotGitRepository
	}

	// Get current branch before any changes
	currentBranch, err := c.CurrentBranch()
	if err != nil {
		return nil, fmt.Errorf("get current branch: %w", err)
	}
	result.PreviousBranch = currentBranch

	// Step 1: Check for dirty state and auto-stash
	dirty, err := c.IsDirty()
	if err != nil {
		return nil, fmt.Errorf("check dirty state: %w", err)
	}
	if dirty {
		stashMsg := fmt.Sprintf("Auto-stash for plan %s", planID)
		if err := c.Stash(stashMsg); err != nil {
			return nil, fmt.Errorf("auto-stash: %w", err)
		}
		result.WasStashed = true
	}

	// Step 2: Check for unpushed commits (unless skipped)
	if !skipUnpushedCheck {
		hasUnpushed, err := c.HasUnpushedCommits()
		if err != nil {
			// If we stashed, try to restore before returning error
			if result.WasStashed {
				_ = c.StashPop()
			}
			return nil, fmt.Errorf("check unpushed commits: %w", err)
		}
		if hasUnpushed {
			// Restore stash before returning - let caller decide what to do
			if result.WasStashed {
				_ = c.StashPop()
				result.WasStashed = false
			}
			return nil, &UnpushedCommitsError{Branch: currentBranch}
		}
	}

	// Step 3: Detect and checkout default branch
	defaultBranch, err := c.DefaultBranch()
	if err != nil {
		if result.WasStashed {
			_ = c.StashPop()
		}
		return nil, fmt.Errorf("detect default branch: %w", err)
	}
	result.DefaultBranch = defaultBranch

	// Only checkout if not already on default branch
	if currentBranch != defaultBranch {
		if err := c.Checkout(defaultBranch); err != nil {
			if result.WasStashed {
				_ = c.StashPop()
			}
			return nil, fmt.Errorf("checkout %s: %w", defaultBranch, err)
		}
	}

	// Step 4: Pull latest from remote
	// Pull might fail if there are conflicts or no remote tracking - we continue anyway
	// since the branch creation should still work from the local state
	if c.HasRemote("origin") {
		_ = c.Pull("origin", defaultBranch)
	}

	// Step 5: Generate branch name and create branch
	branchName := GenerateBranchName(planID, planTitle)
	result.BranchName = branchName

	if err := c.CreateBranch(branchName); err != nil {
		// If branch already exists, try to check it out instead
		if err == ErrBranchExists {
			if checkoutErr := c.Checkout(branchName); checkoutErr != nil {
				return nil, fmt.Errorf("branch %s exists but checkout failed: %w", branchName, checkoutErr)
			}
			// Successfully checked out existing branch
			return result, nil
		}
		return nil, fmt.Errorf("create branch %s: %w", branchName, err)
	}

	return result, nil
}

// GenerateBranchName creates a sanitized branch name from plan ID and title.
// Format: feat/plan-{id}-{slug}
func GenerateBranchName(planID, planTitle string) string {
	slug := Slugify(planTitle)

	// Truncate slug if too long (git has limits, keep it reasonable)
	const maxSlugLen = 50
	if len(slug) > maxSlugLen {
		slug = slug[:maxSlugLen]
		// Don't end with a hyphen
		slug = strings.TrimSuffix(slug, "-")
	}

	// Extract short ID (last 8 chars if it's a longer ID)
	shortID := planID
	if len(planID) > 8 {
		shortID = planID[len(planID)-8:]
	}

	if slug == "" {
		return fmt.Sprintf("feat/plan-%s", shortID)
	}
	return fmt.Sprintf("feat/plan-%s-%s", shortID, slug)
}

// Slugify converts a string to a URL/branch-safe slug.
func Slugify(s string) string {
	// Convert to lowercase
	s = strings.ToLower(s)

	// Replace spaces and underscores with hyphens
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.ReplaceAll(s, "_", "-")

	// Remove any character that isn't alphanumeric or hyphen
	reg := regexp.MustCompile(`[^a-z0-9-]`)
	s = reg.ReplaceAllString(s, "")

	// Replace multiple consecutive hyphens with single hyphen
	reg = regexp.MustCompile(`-+`)
	s = reg.ReplaceAllString(s, "-")

	// Trim leading and trailing hyphens
	s = strings.Trim(s, "-")

	return s
}

// CommitTaskProgress stages all changes and commits with a conventional commit message.
// This is called after each task completion.
func (c *Client) CommitTaskProgress(taskTitle string, taskType string) error {
	// Stage all changes
	if err := c.AddAll(); err != nil {
		return fmt.Errorf("stage changes: %w", err)
	}

	// Check if there's anything to commit
	dirty, err := c.IsDirty()
	if err != nil {
		return fmt.Errorf("check staged changes: %w", err)
	}
	if !dirty {
		// Nothing to commit - this is fine, task may not have made file changes
		return nil
	}

	// Generate commit message with conventional commit format
	commitType := determineCommitType(taskTitle, taskType)
	message := fmt.Sprintf("%s: %s", commitType, taskTitle)

	if err := c.Commit(message); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	return nil
}

// PushTaskProgress pushes the current branch to origin.
func (c *Client) PushTaskProgress(branchName string) error {
	// Check if this is the first push (no upstream)
	hasUnpushed, _ := c.HasUnpushedCommits()

	if hasUnpushed {
		// Branch might not have upstream yet, use -u
		return c.PushWithUpstream("origin", branchName)
	}

	return c.Push("origin", branchName)
}

// determineCommitType infers the conventional commit type from task title/type.
func determineCommitType(taskTitle, taskType string) string {
	titleLower := strings.ToLower(taskTitle)
	typeLower := strings.ToLower(taskType)

	// Check task type first
	switch typeLower {
	case "bug", "bugfix", "fix":
		return "fix"
	case "feature", "feat":
		return "feat"
	case "refactor":
		return "refactor"
	case "test", "testing":
		return "test"
	case "docs", "documentation":
		return "docs"
	case "chore":
		return "chore"
	}

	// Infer from title keywords
	switch {
	case strings.Contains(titleLower, "fix"):
		return "fix"
	case strings.Contains(titleLower, "bug"):
		return "fix"
	case strings.Contains(titleLower, "test"):
		return "test"
	case strings.Contains(titleLower, "refactor"):
		return "refactor"
	case strings.Contains(titleLower, "doc"):
		return "docs"
	case strings.Contains(titleLower, "chore"):
		return "chore"
	default:
		return "feat"
	}
}

// PRInfo contains information about a created pull request.
type PRInfo struct {
	URL    string
	Title  string
	Branch string
	Base   string
}

// TaskInfo represents minimal task information for PR body generation.
// This avoids coupling the git package to the task package.
type TaskInfo struct {
	Title   string
	Summary string
}

// GeneratePRBody creates a formatted PR body from plan goal and completed tasks.
func GeneratePRBody(planGoal string, tasks []TaskInfo) string {
	var sb strings.Builder

	sb.WriteString("## Summary\n\n")
	sb.WriteString(planGoal)
	sb.WriteString("\n\n")

	if len(tasks) > 0 {
		sb.WriteString("## Completed Tasks\n\n")
		for _, t := range tasks {
			sb.WriteString("- [x] ")
			sb.WriteString(t.Title)
			if t.Summary != "" {
				sb.WriteString("\n  - ")
				sb.WriteString(t.Summary)
			}
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	sb.WriteString("---\n")
	sb.WriteString("*Generated by TaskWing*\n")

	return sb.String()
}

// CreatePlanPR creates a pull request for a completed plan.
// It ensures all changes are pushed before creating the PR.
func (c *Client) CreatePlanPR(planGoal string, tasks []TaskInfo, baseBranch string) (*PRInfo, error) {
	// Check if gh is installed
	if !c.IsGhInstalled() {
		return nil, ErrGhNotInstalled
	}

	// Get current branch
	currentBranch, err := c.CurrentBranch()
	if err != nil {
		return nil, fmt.Errorf("get current branch: %w", err)
	}

	// If baseBranch not specified, try to detect it
	if baseBranch == "" {
		baseBranch, err = c.DefaultBranch()
		if err != nil {
			baseBranch = "main" // Fallback
		}
	}

	// Ensure all changes are pushed
	// Ignore push errors - PR might still work if branch exists on remote
	_ = c.PushTaskProgress(currentBranch)

	// Generate PR body
	body := GeneratePRBody(planGoal, tasks)

	// Create PR title from plan goal (truncate if too long)
	title := planGoal
	const maxTitleLen = 72
	if len(title) > maxTitleLen {
		title = title[:maxTitleLen-3] + "..."
	}

	// Create the PR
	url, err := c.CreatePR(title, body, baseBranch, currentBranch)
	if err != nil {
		return nil, fmt.Errorf("create PR: %w", err)
	}

	return &PRInfo{
		URL:    url,
		Title:  title,
		Branch: currentBranch,
		Base:   baseBranch,
	}, nil
}
