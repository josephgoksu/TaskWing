package engine

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/josephgoksu/TaskWing/v2/internal/box"
)

func (r *runner) exam(ctx context.Context, checks []Check, patch, userPatch []byte) error {
	wt := filepath.Join(r.dir, "join")
	joined := filepath.Join(r.dir, "joined.bundle")
	if err := bundle(ctx, wt, joined); err != nil {
		return err
	}
	r.ui.say("  Starting a clean sandbox for the exam...")
	b, err := r.o.Provider.Create(ctx, fmt.Sprintf("%s · %s · exam", BoxPrefix, r.id))
	if err != nil {
		return fmt.Errorf("no exam sandbox: %w", err)
	}
	r.track(b)
	defer r.destroy(b)
	if err := r.upload(ctx, b, joined, BranchPrefix+"exam"); err != nil {
		return err
	}
	if err := r.applyPatch(ctx, b, patch); err != nil {
		return fmt.Errorf("the exam's tests did not apply on the joined work: %w", err)
	}
	// The user's checks have their own tests. If those do not apply, only
	// the user's checks fail.
	userBroken := ""
	if err := r.applyPatch(ctx, b, userPatch); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		userBroken = "the tests for your checks did not apply: " + err.Error()
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
		timedOut := errors.Is(cctx.Err(), context.DeadlineExceeded)
		cancel()
		c.Output = lastLines(out.String(), 40)
		c.Passed = err == nil && rc == 0
		switch {
		case ctx.Err() != nil:
			return ctx.Err()
		case timedOut:
			c.Output = strings.TrimSpace(c.Output + "\n(ran over " + dur(r.o.CheckTimeout) + " and was stopped)")
		case err != nil:
			c.Output = strings.TrimSpace(c.Output + "\n(" + err.Error() + ")")
		}
		mark := r.ui.green("✓")
		if !c.Passed {
			mark = r.ui.red("✗")
		}
		r.ui.say("  %s %s", mark, c.Text)
		r.ev.log("exam.check", map[string]any{"n": i + 1, "passed": c.Passed})
	}
	return ctx.Err()
}

// applyPatch applies a patch of exam tests in the box. An empty patch is a no-op.
func (r *runner) applyPatch(ctx context.Context, b *box.Box, patch []byte) error {
	if len(patch) == 0 {
		return nil
	}
	var out strings.Builder
	rc, err := r.o.Provider.Exec(ctx, b, `cd "$TW_REPO" && git apply --index --whitespace=nowarn`, bytes.NewReader(patch), &out)
	if err != nil {
		return err
	}
	if rc != 0 {
		return withDetail(fmt.Errorf("git apply exited %d", rc), lastLines(out.String(), 3))
	}
	return nil
}
