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
			// Only unmerged paths make a conflict. Any other failure (no git
			// identity, a broken ref) is an error, not a card to leave out.
			unmerged, _ := git(ctx, wt, "diff", "--name-only", "--diff-filter=U")
			_, _ = git(ctx, wt, "merge", "--abort")
			if unmerged == "" {
				return fmt.Errorf("joining %02d failed: %w", cr.n, err)
			}
			conflicts = append(conflicts, fmt.Sprintf("%02d %s", cr.n, cr.card.Title))
			r.ev.log("join.conflict", map[string]any{"card": cr.card.ID})
			continue
		}
		cr.joined = true
		joined = append(joined, fmt.Sprintf("%02d", cr.n))
	}
	r.ui.say("  Joined %d %s on branch %s", len(joined), plural(len(joined), "piece", "pieces"), branch)
	for _, c := range conflicts {
		r.ui.say("  %s %s conflicts with the others and was left out", r.ui.red("✗"), c)
	}
	for _, cr := range cards {
		if len(cr.outside) > 0 {
			r.ui.say("  %s %02d changed files outside its area: %s", r.ui.amber("!"), cr.n, truncate(strings.Join(cr.outside, ", "), 100))
		}
	}
	if len(joined) == 0 {
		return errors.New("nothing could be joined")
	}
	return nil
}
