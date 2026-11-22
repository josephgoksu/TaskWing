package engine

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

func (r *runner) join(ctx context.Context, cards []*cardRun, branch string) error {
	wt := filepath.Join(r.dir, "join")
	if _, err := git(ctx, r.root, "worktree", "add", "-q", "-b", branch, wt, r.base); err != nil {
		return err
	}
	var joined, conflicts []string
	for _, cr := range cards {
		if cr.state != stDone {
			continue
		}
		msg := fmt.Sprintf("Join %02d: %s", cr.n, cr.card.Title)
		if _, err := git(ctx, wt, "merge", "--no-ff", "--no-edit", "-m", msg, cr.ref); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			// A conflict, or commits git refuses to merge (an unrelated
			// history, a path this machine cannot check out): either way this
			// card is left out and the others go on. The reason is logged.
			unmerged, _ := git(ctx, wt, "diff", "--name-only", "--diff-filter=U")
			_, _ = git(ctx, wt, "merge", "--abort")
			_, _ = git(ctx, wt, "reset", "-q", "--hard", "HEAD")
			why := "conflicts with the others"
			if unmerged == "" {
				why = "could not be merged: " + truncate(err.Error(), 120)
			}
			conflicts = append(conflicts, fmt.Sprintf("%02d %s %s", cr.n, cr.card.Title, why))
			r.ev.log("join.conflict", map[string]any{"card": cr.card.ID, "why": why})
			continue
		}
		cr.joined = true
		joined = append(joined, fmt.Sprintf("%02d", cr.n))
	}
	r.ui.say("  Joined %d %s on branch %s", len(joined), plural(len(joined), "piece", "pieces"), branch)
	for _, c := range conflicts {
		r.ui.say("  %s %s and was left out", r.ui.red("✗"), c)
	}
	for _, cr := range cards {
		if len(cr.outside) > 0 {
			names := make([]string, len(cr.outside))
			for i, f := range cr.outside {
				names[i] = safeName(f)
			}
			r.ui.say("  %s %02d changed files outside its area: %s", r.ui.amber("!"), cr.n, truncate(strings.Join(names, ", "), 100))
		}
	}
	if len(joined) == 0 {
		return errors.New("nothing could be joined")
	}
	return nil
}
