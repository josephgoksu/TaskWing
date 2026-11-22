package engine

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

// The repository field of run.start is a folder on this machine or a GitHub
// owner/name. A GitHub repository is cloned once into ~/.taskwing/repos and
// brought up to date before each run.
// The owner may not start with "-" or ".", so the name never reads as a flag
// to gh or git.
var ghRepoRe = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_.-]*/[A-Za-z0-9_.-]+$`)

// A clone in ~/.taskwing/repos is reset before each run, so only one run may
// use it at a time.
var clones = struct {
	sync.Mutex
	busy map[string]bool
}{busy: map[string]bool{}}

// ReleaseRepo frees a clone ResolveRepo handed out. Other folders are ignored.
func ReleaseRepo(dir string) {
	clones.Lock()
	delete(clones.busy, dir)
	clones.Unlock()
}

// ResolveRepo returns the folder a run works in. For a GitHub repository the
// clone stays claimed until ReleaseRepo.
func ResolveRepo(ctx context.Context, home, repo string) (string, error) {
	repo = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(repo), "@"))
	if repo == "" {
		return "", errors.New("no repository given")
	}
	if strings.HasPrefix(repo, "~/") {
		if h, err := os.UserHomeDir(); err == nil {
			repo = filepath.Join(h, repo[2:])
		}
	}
	if filepath.IsAbs(repo) {
		st, err := os.Stat(repo)
		if err != nil || !st.IsDir() {
			return "", fmt.Errorf("no folder at %s", repo)
		}
		return repo, nil
	}
	repo = strings.TrimSuffix(strings.TrimPrefix(repo, "https://github.com/"), ".git")
	if !ghRepoRe.MatchString(repo) || strings.Contains(repo, "..") || strings.HasSuffix(repo, "/.") {
		return "", fmt.Errorf("%q is not a folder path or a GitHub owner/name", repo)
	}
	dir := filepath.Join(home, "repos", filepath.FromSlash(repo))
	clones.Lock()
	if clones.busy[dir] {
		clones.Unlock()
		return "", fmt.Errorf("another run is using %s; wait for it to finish, or give the path of a folder of your own", repo)
	}
	clones.busy[dir] = true
	clones.Unlock()
	dir, err := syncClone(ctx, repo, dir)
	if err != nil {
		ReleaseRepo(filepath.Join(home, "repos", filepath.FromSlash(repo)))
		return "", err
	}
	return dir, nil
}

// syncClone clones repo into dir, or brings an earlier clone up to date.
func syncClone(ctx context.Context, repo, dir string) (string, error) {
	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
			return "", err
		}
		out, err := exec.CommandContext(ctx, "gh", "repo", "clone", repo, dir, "--", "-q").CombinedOutput()
		if err != nil {
			return "", fmt.Errorf("could not clone %s with the GitHub CLI: %s", repo, lastLines(strings.TrimSpace(string(out)), 2))
		}
		return dir, nil
	}
	// Our own clone: bring its default branch up to date. The user's work
	// never lives here, so a reset is safe.
	if _, err := git(ctx, dir, "fetch", "-q", "origin"); err != nil {
		return "", fmt.Errorf("could not update %s: %v", repo, err)
	}
	head, err := git(ctx, dir, "symbolic-ref", "--short", "refs/remotes/origin/HEAD")
	if err != nil {
		_, _ = git(ctx, dir, "remote", "set-head", "origin", "--auto")
		if head, err = git(ctx, dir, "symbolic-ref", "--short", "refs/remotes/origin/HEAD"); err != nil {
			return "", fmt.Errorf("could not find the default branch of %s", repo)
		}
	}
	branch := strings.TrimPrefix(head, "origin/")
	if _, err := git(ctx, dir, "checkout", "-q", "-B", branch, head); err != nil {
		return "", fmt.Errorf("could not check out %s in %s: %v", branch, dir, err)
	}
	if _, err := git(ctx, dir, "reset", "-q", "--hard", head); err != nil {
		return "", err
	}
	if _, err := git(ctx, dir, "clean", "-qfd"); err != nil {
		return "", err
	}
	return dir, nil
}
