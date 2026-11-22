package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// writeExam runs the exam session in a throwaway worktree, then keeps its new
// files as a patch on this machine. The patch only enters the exam box.
func (r *runner) writeExam(ctx context.Context, iface string) ([]Check, []byte, error) {
	checks, patch, err := r.examSession(ctx, examPrompt(r.o.Task, iface), "exam", "taskwing_exam")
	for i := range checks {
		checks[i].ID = fmt.Sprintf("k%d", i+1)
	}
	return checks, patch, err
}

type userExamResult struct {
	checks []Check
	patch  []byte
}

// startUserExam turns the checks the user added in the client into tests, in its
// own session, while the agents build. Like the main exam, the builders never
// see it. A check no test could be written for fails in the exam.
func (r *runner) startUserExam(ctx context.Context, iface string, approved []WireCheck) chan userExamResult {
	const most = 10
	var mine, extra []WireCheck
	for _, c := range approved {
		if c.AddedBy != "user" || strings.TrimSpace(c.Text) == "" || c.ID == "" {
			continue
		}
		if len(mine) < most {
			mine = append(mine, c)
		} else {
			extra = append(extra, c)
		}
	}
	if len(mine) == 0 {
		return nil
	}
	texts := make([]string, len(mine))
	for i, c := range mine {
		texts[i] = truncate(strings.TrimSpace(c.Text), 300)
	}
	r.ev.log("exam.user", map[string]any{"checks": len(mine)})
	done := make(chan userExamResult, 1)
	go func() {
		checks, patch, err := r.examSession(ctx, userExamPrompt(r.o.Task, iface, texts), "user-exam", userExamMarker)
		why := ""
		switch {
		case err != nil:
			why = err.Error()
		case len(checks) != len(mine):
			why = fmt.Sprintf("the session wrote %d checks for %d sentences", len(checks), len(mine))
		}
		out := make([]Check, len(mine))
		for i, c := range mine {
			out[i] = Check{ID: c.ID, Text: texts[i], User: true}
			if why == "" {
				out[i].Command = checks[i].Command
			} else {
				out[i].Output = "no test could be written for this check: " + why
			}
		}
		// A check past the limit fails instead of vanishing from the exam.
		for _, c := range extra {
			out = append(out, Check{ID: c.ID, Text: truncate(strings.TrimSpace(c.Text), 300), User: true,
				Output: fmt.Sprintf("no test was written: only the first %d checks you add get tests", most)})
		}
		if why != "" {
			patch = nil
			r.ev.log("exam.user.failed", map[string]any{"why": why})
		}
		done <- userExamResult{checks: out, patch: patch}
	}()
	return done
}

// examSession runs one exam-writing session (name is "exam" or "user-exam").
// Only new files whose path contains marker are kept.
func (r *runner) examSession(ctx context.Context, prompt, name, marker string) ([]Check, []byte, error) {
	wt := filepath.Join(r.dir, name+"-worktree")
	if _, err := git(ctx, r.root, "worktree", "add", "-q", "--detach", wt, r.base); err != nil {
		return nil, nil, err
	}
	defer func() {
		_, _ = git(context.Background(), r.root, "worktree", "remove", "--force", wt)
	}()
	out, err := hostAgent(ctx, r.o.ExamCmd, wt, prompt, filepath.Join(r.dir, "logs", name+"-writer.log"))
	if err != nil {
		return nil, nil, fmt.Errorf("the exam session failed: %v", err)
	}
	js, err := extractJSON(out)
	if err != nil {
		return nil, nil, fmt.Errorf("the exam session gave no checks: %v", err)
	}
	var e struct {
		Checks []Check `json:"checks"`
	}
	if err := json.Unmarshal([]byte(js), &e); err != nil {
		return nil, nil, fmt.Errorf("the exam's checks are not valid JSON: %v", err)
	}
	var checks []Check
	for _, c := range e.Checks {
		if strings.TrimSpace(c.Text) != "" && strings.TrimSpace(c.Command) != "" {
			checks = append(checks, Check{Text: c.Text, Command: c.Command})
		}
	}
	if len(checks) == 0 {
		return nil, nil, errors.New("the exam has no checks")
	}
	// Keep only new files with the marker in the path. Anything else the
	// session touched (caches, lock files, edits to existing files) is dropped.
	if _, err := git(ctx, wt, "add", "-A"); err != nil {
		return nil, nil, err
	}
	status, _ := git(ctx, wt, "diff", "--cached", "--name-status", "--no-renames", r.base)
	var keep, dropped []string
	for _, line := range strings.Split(status, "\n") {
		f := strings.SplitN(line, "\t", 2)
		if len(f) != 2 {
			continue
		}
		if f[0] == "A" && strings.Contains(f[1], marker) {
			keep = append(keep, f[1])
		} else {
			dropped = append(dropped, f[1])
		}
	}
	if len(keep) == 0 {
		return nil, nil, fmt.Errorf("the exam session wrote no test files (their path must contain %q)", marker)
	}
	if len(dropped) > 0 {
		r.ev.log("exam.dropped", map[string]any{"files": dropped})
	}
	if _, err := git(ctx, wt, "reset", "-q"); err != nil {
		return nil, nil, err
	}
	if _, err := git(ctx, wt, append([]string{"add", "--"}, keep...)...); err != nil {
		return nil, nil, err
	}
	patch, err := gitRaw(ctx, wt, "diff", "--cached", "--binary", r.base)
	if err != nil {
		return nil, nil, err
	}
	examDir := filepath.Join(r.dir, name)
	_ = os.MkdirAll(examDir, 0o755)
	_ = os.WriteFile(filepath.Join(examDir, "exam.patch"), []byte(patch), 0o644)
	b, _ := json.MarshalIndent(checks, "", "  ")
	_ = os.WriteFile(filepath.Join(examDir, "checks.json"), b, 0o644)
	r.ev.log("exam.ready", map[string]any{"checks": len(checks), "files": keep})
	return checks, []byte(patch), nil
}
