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
	checks, patch, err := r.examSession(ctx, examPrompt(r.o.Task, iface), "exam", examMarker)
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
	// The worktree sits outside the run folder, so the session's own folder
	// does not lead it to the lead's plan and logs.
	tmp, err := os.MkdirTemp("", "taskwing-"+name+"-")
	if err != nil {
		return nil, nil, err
	}
	wt := filepath.Join(tmp, "repo")
	if _, err := git(ctx, r.root, "worktree", "add", "-q", "--detach", wt, r.base); err != nil {
		_ = os.RemoveAll(tmp)
		return nil, nil, err
	}
	defer func() {
		_, _ = git(context.Background(), r.root, "worktree", "remove", "--force", wt)
		_ = os.RemoveAll(tmp)
	}()
	out, err := hostAgent(ctx, r.o.ExamCmd, wt, prompt, filepath.Join(r.dir, "logs", name+"-writer.log"))
	if err != nil {
		return nil, nil, fmt.Errorf("the exam session failed: %w", err)
	}
	js, err := extractJSON(out, "checks")
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
	// -z keeps unusual file names as they are; it lists status, path, status, path...
	status, err := gitRaw(ctx, wt, "diff", "--cached", "--name-status", "--no-renames", "-z", r.base)
	if err != nil {
		return nil, nil, err
	}
	var keep, dropped []string
	fields := splitNUL(status)
	for i := 0; i+1 < len(fields); i += 2 {
		if st, f := fields[i], fields[i+1]; st == "A" && strings.Contains(f, marker) {
			keep = append(keep, f)
		} else {
			dropped = append(dropped, f)
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
	if err := os.MkdirAll(examDir, dirPerm); err != nil {
		return nil, nil, err
	}
	if err := os.WriteFile(filepath.Join(examDir, "exam.patch"), []byte(patch), filePerm); err != nil {
		return nil, nil, err
	}
	r.saveJSON(filepath.Join(name, "checks.json"), checks)
	r.ev.log("exam.ready", map[string]any{"checks": len(checks), "files": keep})
	return checks, []byte(patch), nil
}
