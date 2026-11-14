package engine

import (
	"bytes"
	"context"
	"fmt"
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
	if err := os.WriteFile(bodyPath, []byte(body), 0o644); err != nil {
		return "", err
	}
	title := truncate(plan.Task, 70)
	cmd := exec.CommandContext(ctx, "gh", "pr", "create", "--draft", "--base", r.branch, "--head", res.Branch, "--title", title, "--body-file", bodyPath)
	cmd.Dir = r.root
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("gh pr create failed: %v: %s", err, lastLines(errb.String(), 3))
	}
	return strings.TrimSpace(lastLines(out.String(), 1)), nil
}

// prBody is the pull request text: what was asked, who built what, and the exam.
func prBody(plan *Plan, cards []*cardRun, checks []Check, res *Result, took time.Duration) string {
	var b strings.Builder
	passed := 0
	for _, c := range checks {
		if c.Passed {
			passed++
		}
	}
	fmt.Fprintf(&b, "## Task\n\n%s\n\n", plan.Task)
	fmt.Fprintf(&b, "**Exam: %d of %d checks passed** · %d %s · %s\n\n", passed, len(checks), len(cards), plural(len(cards), "piece", "pieces"), dur(took))

	b.WriteString("## Pieces\n\n| | Piece | Files | Proof |\n|---|---|---|---|\n")
	for _, cr := range cards {
		mark := "✓"
		proof := cr.proof
		switch {
		case cr.state != stDone:
			mark, proof = "✗", "stuck: "+cr.err
		case !cr.joined:
			mark, proof = "✗", "conflicted with the other pieces; left out"
		}
		fmt.Fprintf(&b, "| %s | %02d %s | %d | %s |\n", mark, cr.n, cell(cr.card.Title), len(cr.files), cell(proof))
	}
	b.WriteString("\n")
	for _, cr := range cards {
		if len(cr.outside) > 0 {
			fmt.Fprintf(&b, "> Piece %02d changed files outside its area: %s\n", cr.n, strings.Join(cr.outside, ", "))
		}
	}

	b.WriteString("## Exam\n\nWritten before the build by a separate session. The builders never saw it.\n\n| | Check |\n|---|---|\n")
	for _, c := range checks {
		mark := "✓"
		if !c.Passed {
			mark = "✗"
		}
		fmt.Fprintf(&b, "| %s | %s |\n", mark, cell(c.Text))
	}
	for _, c := range checks {
		if !c.Passed {
			fmt.Fprintf(&b, "\n<details><summary>✗ %s</summary>\n\n```\n$ %s\n%s\n```\n</details>\n", c.Text, c.Command, c.Output)
		}
	}
	if len(res.Secrets) == 0 {
		b.WriteString("\nSecret scan: nothing found in any sandbox.\n")
	} else {
		fmt.Fprintf(&b, "\nSecret scan found: %s\n", strings.Join(res.Secrets, "; "))
	}
	fmt.Fprintf(&b, "\nRun %s\n", res.RunID)
	return b.String()
}

func cell(s string) string {
	return strings.ReplaceAll(truncate(s, 160), "|", "\\|")
}
