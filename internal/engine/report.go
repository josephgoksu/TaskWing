package engine

import (
	"bytes"
	"context"
	"fmt"
	"html"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// openPR pushes the joined branch from this machine and opens a draft pull
// request. GitHub credentials never enter a box.
func (r *runner) openPR(ctx context.Context, plan *Plan, cards []*cardRun, checks []Check, res *Result) (string, error) {
	r.ui.say("  Pushing %s from this machine...", res.Branch)
	if _, err := git(ctx, r.root, "push", "-q", "-u", "origin", res.Branch); err != nil {
		return "", fmt.Errorf("push failed: %w", err)
	}
	body := prBody(plan, cards, checks, res, time.Since(r.start))
	bodyPath := filepath.Join(r.dir, "pr-body.md")
	if err := os.WriteFile(bodyPath, []byte(body), 0o600); err != nil {
		return "", fmt.Errorf("could not write the pull request text: %w", err)
	}
	title := noMention(truncate(plan.Task, 70))
	cmd := exec.CommandContext(ctx, "gh", "pr", "create", "--draft", "--base", r.branch, "--head", res.Branch, "--title", title, "--body-file", bodyPath)
	cmd.Dir = r.root
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("gh pr create failed: %w: %s", err, lastLines(errb.String(), 3))
	}
	return strings.TrimSpace(lastLines(out.String(), 1)), nil
}

// prBody is the pull request text: what was asked, who built what, and the exam.
// The task, titles and check output come from agents and worker code, so
// everything is escaped: unescaped they can break the layout, hide text in
// HTML, or @mention people from our PR.
func prBody(plan *Plan, cards []*cardRun, checks []Check, res *Result, took time.Duration) string {
	var b strings.Builder
	passed := 0
	for _, c := range checks {
		if c.Passed {
			passed++
		}
	}
	fmt.Fprintf(&b, "## Task\n\n%s\n\n", noMention(plan.Task))
	fmt.Fprintf(&b, "**Exam: %d of %d checks passed** · %d %s · %s\n\n", passed, len(checks), len(cards), plural(len(cards), "piece", "pieces"), dur(took))

	var bugs []*cardRun
	for _, cr := range cards {
		if cr.card.Why != "" {
			bugs = append(bugs, cr)
		}
	}
	if len(bugs) > 0 {
		b.WriteString("## Bugs\n\nEach one was proven by a test that failed on the old code. The fixers never saw those tests.\n\n| | Where | What went wrong |\n|---|---|---|\n")
		for _, cr := range bugs {
			fmt.Fprintf(&b, "| %02d | %s | %s |\n", cr.n, code(cell(cr.card.Where)), cell(cr.card.Why))
		}
		b.WriteString("\n")
	}
	b.WriteString("## Pieces\n\n| | Piece | Files | Proof |\n|---|---|---|---|\n")
	for _, cr := range cards {
		mark := "✓"
		proof := cr.proof
		switch {
		case cr.state != stDone:
			mark, proof = "✗", "stuck: "+cr.err
		case !cr.joined:
			mark, proof = "✗", "conflicted with the other pieces; left out"
		case cr.early:
			mark = "◐" // stopped before it said it was done
		}
		fmt.Fprintf(&b, "| %s | %02d %s | %d | %s |\n", mark, cr.n, cell(cr.card.Title), len(cr.files), cell(proof))
	}
	b.WriteString("\n")
	// A list, not "> " lines: consecutive quote lines merge into one paragraph.
	outside := false
	for _, cr := range cards {
		if len(cr.outside) == 0 {
			continue
		}
		files := make([]string, len(cr.outside))
		for i, f := range cr.outside {
			files[i] = code(noMention(f))
		}
		fmt.Fprintf(&b, "- Piece %02d changed files outside its area: %s\n", cr.n, strings.Join(files, ", "))
		outside = true
	}
	if outside {
		b.WriteString("\n")
	}

	user := false
	for _, c := range checks {
		user = user || c.User
	}
	if user {
		b.WriteString("## Exam\n\nWritten by separate sessions: most checks before the build, the ones you added during it. The builders never saw any of it.\n\n| | Check |\n|---|---|\n")
	} else {
		b.WriteString("## Exam\n\nWritten before the build by a separate session. The builders never saw it.\n\n| | Check |\n|---|---|\n")
	}
	for _, c := range checks {
		mark := "✓"
		if !c.Passed {
			mark = "✗"
		}
		fmt.Fprintf(&b, "| %s | %s |\n", mark, cell(c.Text))
	}
	for _, c := range checks {
		if !c.Passed {
			out := "$ " + c.Command + "\n" + c.Output
			f := fence(out)
			fmt.Fprintf(&b, "\n<details><summary>✗ %s</summary>\n\n%s\n%s\n%s\n</details>\n", html.EscapeString(noMention(c.Text)), f, out, f)
		}
	}
	switch {
	case !res.Scanned:
		b.WriteString("\nSecret scan: not run (local mode).\n")
	case len(res.Secrets) == 0:
		b.WriteString("\nSecret scan: nothing found in any sandbox.\n")
	default:
		fmt.Fprintf(&b, "\nSecret scan found: %s\n", strings.Join(res.Secrets, "; "))
	}
	fmt.Fprintf(&b, "\nRun %s\n", res.RunID)
	return b.String()
}

func cell(s string) string {
	s = strings.NewReplacer("|", "\\|", "\r", " ", "\n", " ").Replace(truncate(s, 160))
	return noMention(s)
}

// noMention stops @name in agent text from pinging people: a zero-width
// joiner after the @ keeps the text readable but breaks the mention.
func noMention(s string) string {
	return strings.ReplaceAll(s, "@", "@\u200d")
}

// longestRun is the longest run of backticks in s.
func longestRun(s string) int {
	most, n := 0, 0
	for i := 0; i < len(s); i++ {
		if s[i] == '`' {
			n++
			most = max(most, n)
		} else {
			n = 0
		}
	}
	return most
}

// fence returns a code fence longer than any backtick run in s, so output
// containing ``` cannot close the block and turn the rest into markdown.
func fence(s string) string {
	return strings.Repeat("`", max(3, longestRun(s)+1))
}

// code wraps s in an inline code span that its own backticks cannot end.
func code(s string) string {
	t := strings.Repeat("`", longestRun(s)+1)
	if strings.HasPrefix(s, "`") || strings.HasSuffix(s, "`") {
		s = " " + s + " "
	}
	return t + s + t
}
