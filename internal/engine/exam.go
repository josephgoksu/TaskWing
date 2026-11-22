package engine

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
)

func (r *runner) exam(ctx context.Context, checks []Check, patch, userPatch []byte) error {
	wt := filepath.Join(r.dir, "join")
	bundle := filepath.Join(r.dir, "joined.bundle")
	if _, err := git(ctx, wt, "bundle", "create", "-q", bundle, "HEAD"); err != nil {
		if _, err2 := git(ctx, wt, "bundle", "create", bundle, "HEAD"); err2 != nil {
			return err
		}
	}
	r.ui.say("  Starting a clean sandbox for the exam...")
	b, err := r.o.Provider.Create(ctx, fmt.Sprintf("%s · %s · exam", BoxPrefix, r.id))
	if err != nil {
		return fmt.Errorf("no exam sandbox: %w", err)
	}
	r.track(b)
	defer r.destroy(b)
	if err := r.upload(ctx, b, bundle, "taskwing/exam"); err != nil {
		return err
	}
	if len(patch) > 0 {
		var out strings.Builder
		rc, err := r.o.Provider.Exec(ctx, b, `cd "$TW_REPO" && git apply --index --whitespace=nowarn`, strings.NewReader(string(patch)), &out)
		if err != nil || rc != 0 {
			return fmt.Errorf("the exam's tests did not apply on the joined work (exit %d): %v %s", rc, err, lastLines(out.String(), 3))
		}
	}
	// The user's checks have their own tests. If those do not apply, only
	// the user's checks fail.
	userBroken := ""
	if len(userPatch) > 0 {
		var out strings.Builder
		rc, err := r.o.Provider.Exec(ctx, b, `cd "$TW_REPO" && git apply --index --whitespace=nowarn`, strings.NewReader(string(userPatch)), &out)
		if err != nil || rc != 0 {
			userBroken = fmt.Sprintf("the tests for your checks did not apply (exit %d): %v %s", rc, err, lastLines(out.String(), 3))
		}
	}
	for i := range checks {
		// After Ctrl-C every remaining check would "fail"; stop instead.
		if err := ctx.Err(); err != nil {
			return err
		}
		c := &checks[i]
		if c.User && userBroken != "" {
			c.Output = userBroken
		}
		if c.Command == "" || (c.User && userBroken != "") {
			c.Passed = false
			r.ui.say("  %s %s %s", r.ui.red("✗"), c.Text, r.ui.dim("(no test)"))
			r.ev.log("exam.check", map[string]any{"n": i + 1, "passed": false})
			continue
		}
		cctx, cancel := context.WithTimeout(ctx, r.o.CheckTimeout)
		var out strings.Builder
		// The command sits on its own lines, so a trailing "# comment" in it
		// cannot swallow the closing parenthesis.
		script := fmt.Sprintf("cd \"$TW_REPO\" || exit 99\n(\n%s\n) > \"$TW_REPO.check.log\" 2>&1 </dev/null\nrc=$?\ntail -n 40 \"$TW_REPO.check.log\"\nexit $rc\n", c.Command)
		rc, err := r.o.Provider.Exec(cctx, b, script, nil, &out)
		cancel()
		c.Output = lastLines(out.String(), 40)
		c.Passed = err == nil && rc == 0
		mark := r.ui.green("✓")
		if !c.Passed {
			mark = r.ui.red("✗")
		}
		r.ui.say("  %s %s", mark, c.Text)
		r.ev.log("exam.check", map[string]any{"n": i + 1, "passed": c.Passed})
	}
	return ctx.Err()
}
