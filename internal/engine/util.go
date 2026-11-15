package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// git runs git in dir and returns trimmed stdout.
func git(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		return strings.TrimSpace(out.String()), fmt.Errorf("git %s: %v: %s", strings.Join(args, " "), err, strings.TrimSpace(errb.String()))
	}
	return strings.TrimSpace(out.String()), nil
}

// hostAgent runs a coding agent on this machine (the lead or the exam
// session). The command is a shell command; the prompt arrives on stdin.
func hostAgent(ctx context.Context, command, dir, prompt, logPath string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "bash", "-c", command)
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(prompt)
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	err := cmd.Run()
	if logPath != "" {
		_ = os.WriteFile(logPath, []byte("$ "+command+"\n--- prompt\n"+prompt+"\n--- stdout\n"+out.String()+"\n--- stderr\n"+errb.String()), 0o644)
	}
	if err != nil {
		all := errb.String() + out.String()
		if strings.Contains(all, "Not logged in") || strings.Contains(all, "/login") {
			return out.String(), fmt.Errorf("the claude CLI on this machine is not logged in: run claude, type /login, then try again")
		}
		return out.String(), fmt.Errorf("%v: %s", err, lastLines(all, 5))
	}
	return out.String(), nil
}

// events is the run's append-only log (~/.taskwing/runs/<id>/events.jsonl).
type events struct {
	mu sync.Mutex
	f  *os.File
}

func openEvents(dir string) (*events, error) {
	f, err := os.OpenFile(filepath.Join(dir, "events.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
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

var proofRe = regexp.MustCompile(`(?m)^\s*\**PROOF:?\**:?\s*(.+)$`)

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
}

const secretScanScript = `env 2>/dev/null; cat ~/.git-credentials ~/.config/gh/hosts.yml ~/.netrc 2>/dev/null; true`

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
