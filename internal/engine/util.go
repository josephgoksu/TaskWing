package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/josephgoksu/TaskWing/v2/internal/proc"
)

// git runs git in dir and returns trimmed stdout.
func git(ctx context.Context, dir string, args ...string) (string, error) {
	out, err := gitRaw(ctx, dir, args...)
	return strings.TrimSpace(out), err
}

// hostGit is put in front of every git command on this machine. The engine
// merges and checks out commits that agents wrote, and agents are untrusted
// (PRINCIPLES.md #5): without these, a worker that edits a hook in a repo
// with core.hooksPath set (husky does this) runs code on the host at join,
// and an fsmonitor or ext:: remote could do the same.
var hostGit = []string{"-c", "core.hooksPath=/dev/null", "-c", "core.fsmonitor=false", "-c", "protocol.ext.allow=never"}

// gitRaw runs git in dir and returns stdout untouched. Patches need it: a
// binary patch ends with a blank line that git apply requires.
func gitRaw(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", append(append([]string(nil), hostGit...), args...)...)
	cmd.Dir = dir
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return out.String(), ctx.Err()
		}
		return out.String(), withDetail(fmt.Errorf("git %s: %w", strings.Join(args, " "), err), errb.String())
	}
	return out.String(), nil
}

// withDetail adds a command's own words to its error, and nothing when it
// printed none (an error ending in ": " reads as cut off).
func withDetail(err error, detail string) error {
	if detail = strings.TrimSpace(detail); detail == "" {
		return err
	}
	return fmt.Errorf("%w: %s", err, detail)
}

// Run folders hold prompts, agent output and bundles of the user's code:
// readable by the user only.
const (
	dirPerm  = 0o700
	filePerm = 0o600
)

// hostAgentTimeout caps one lead, exam or hunter session.
const hostAgentTimeout = 30 * time.Minute

// hostAgent runs a coding agent on this machine (the lead or the exam
// session). The command is a shell command; the prompt arrives on stdin.
func hostAgent(parent context.Context, command, dir, prompt, logPath string) (string, error) {
	ctx := parent
	ctx, cancel := context.WithTimeout(ctx, hostAgentTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "bash", "-c", command)
	proc.Group(cmd)
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(prompt)
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	err := cmd.Run()
	// Exit 0 with a leftover child holding stdout open: the session is done.
	// The leftovers are stopped, so they do not outlive it on this machine.
	if errors.Is(err, exec.ErrWaitDelay) {
		err = nil
	}
	_ = proc.Kill(cmd)
	if logPath != "" {
		_ = os.WriteFile(logPath, []byte("$ "+command+"\n--- prompt\n"+prompt+"\n--- stdout\n"+out.String()+"\n--- stderr\n"+errb.String()), filePerm)
	}
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) && parent.Err() == nil {
			return out.String(), fmt.Errorf("ran over %s and was stopped", dur(hostAgentTimeout))
		}
		if parent.Err() != nil {
			return out.String(), parent.Err()
		}
		// Only the CLI's own words: a plan that mentions a /login route must
		// not read as a logged-out CLI.
		if strings.Contains(errb.String(), "Not logged in") || strings.Contains(errb.String(), "Please run /login") {
			return out.String(), fmt.Errorf("the claude CLI on this machine is not logged in: run claude, type /login, then try again")
		}
		return out.String(), withDetail(err, lastLines(errb.String()+out.String(), 5))
	}
	return out.String(), nil
}

// events is the run's append-only log (~/.taskwing/runs/<id>/events.jsonl).
type events struct {
	mu sync.Mutex
	f  *os.File
}

func openEvents(dir string) (*events, error) {
	f, err := os.OpenFile(filepath.Join(dir, "events.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, filePerm)
	if err != nil {
		return nil, err
	}
	return &events{f: f}, nil
}

func (e *events) log(kind string, fields map[string]any) {
	if e == nil {
		return
	}
	m := map[string]any{"ts": time.Now().UTC().Format(time.RFC3339), "event": kind}
	for k, v := range fields {
		m[k] = v
	}
	b, _ := json.Marshal(m)
	e.mu.Lock()
	defer e.mu.Unlock()
	_, _ = e.f.Write(append(b, '\n'))
}

func (e *events) close() {
	if e != nil {
		e.f.Close()
	}
}

func lastLines(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

// The colon is required, so a line that starts with "PROOFREAD" is not a proof.
var proofRe = regexp.MustCompile(`(?m)^\s*\**PROOF\**\s*:\**\s*(.+)$`)

// proofLine pulls the agent's "PROOF: ..." line out of its final answer.
func proofLine(final string) string {
	m := proofRe.FindAllStringSubmatch(final, -1)
	if len(m) == 0 {
		return ""
	}
	return strings.TrimSpace(strings.Trim(m[len(m)-1][1], "*"))
}

// secretPatterns find credentials that must never be in a box. Values are never printed.
var secretPatterns = []struct {
	name string
	re   *regexp.Regexp
}{
	{"GitHub token", regexp.MustCompile(`\b(ghp|gho|ghu|ghs|ghr)_[A-Za-z0-9]{20,}`)},
	{"GitHub fine-grained token", regexp.MustCompile(`github_pat_[A-Za-z0-9_]{20,}`)},
	{"Anthropic API key", regexp.MustCompile(`sk-ant-[A-Za-z0-9_-]{16,}`)},
	{"OpenAI API key", regexp.MustCompile(`\bsk-(proj-)?[A-Za-z0-9]{32,}`)},
	{"AWS secret key", regexp.MustCompile(`AWS_SECRET_ACCESS_KEY=\S+`)},
	{"GitHub token variable", regexp.MustCompile(`(?m)^(GH_TOKEN|GITHUB_TOKEN)=\S+`)},
	{"gh login", regexp.MustCompile(`oauth_token:\s*\S+`)},
	// An SSH key lets an agent push over SSH without any token. (Not
	// SSH_AUTH_SOCK: the scan runs over boat ssh, whose own session may
	// forward the user's agent, which the agent process never gets.)
	{"SSH key", regexp.MustCompile(`(?m)^TW_SSH_KEY=\S+`)},
}

const secretScanScript = `env 2>/dev/null; cat ~/.git-credentials ~/.config/gh/hosts.yml ~/.netrc 2>/dev/null; for k in ~/.ssh/id_*; do case "$k" in *.pub) ;; *) [ -f "$k" ] && echo "TW_SSH_KEY=$k";; esac; done; true`

// scanBox splits a box's scan into leaks and the agent's own Claude login
// (CLAUDE_CODE_OAUTH_TOKEN), which agents need to run and is expected.
func scanBox(text string) (leaks []string, agentLogin bool) {
	var rest []string
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "CLAUDE_CODE_OAUTH_TOKEN=") {
			agentLogin = agentLogin || len(line) > len("CLAUDE_CODE_OAUTH_TOKEN=")
			continue
		}
		rest = append(rest, line)
	}
	return findSecrets(strings.Join(rest, "\n")), agentLogin
}

func findSecrets(text string) []string {
	var found []string
	for _, p := range secretPatterns {
		if p.re.MatchString(text) {
			found = append(found, p.name)
		}
	}
	return found
}

func dur(d time.Duration) string {
	d = d.Round(time.Second)
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm %02ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%dh %02dm", int(d.Hours()), int(d.Minutes())%60)
}

func truncate(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

// claudeLoggedIn fails early when a host command uses the claude CLI and the
// CLI is logged out (a logged-in desktop app does not log in the CLI).
func claudeLoggedIn(ctx context.Context, o Options) error {
	uses := false
	for _, c := range []string{o.LeadCmd, o.ExamCmd, o.HuntCmd} {
		if strings.HasPrefix(strings.TrimSpace(c), "claude ") {
			uses = true
		}
	}
	if !uses {
		return nil
	}
	out, err := exec.CommandContext(ctx, "claude", "auth", "status").CombinedOutput()
	if err != nil && len(out) == 0 {
		return nil // older CLI without auth status: find out on the first call
	}
	compact := strings.ReplaceAll(strings.ReplaceAll(string(out), " ", ""), "\"", "")
	if strings.Contains(compact, "loggedIn:false") {
		return fmt.Errorf("the claude CLI on this machine is not logged in (the desktop app login is separate): run claude, type /login, then try again")
	}
	return nil
}

// safeName shows a file name an agent chose. git -z hands names over raw, so
// a name may hold a newline (which restyles the pull request text) or an
// escape sequence (which drives the terminal); such a name is shown quoted.
func safeName(f string) string {
	if strings.IndexFunc(f, unicode.IsControl) >= 0 {
		return strconv.Quote(f)
	}
	return f
}

// splitNUL splits the output of a git command run with -z.
func splitNUL(s string) []string {
	var out []string
	for _, f := range strings.Split(s, "\x00") {
		if f != "" {
			out = append(out, f)
		}
	}
	return out
}
