package engine

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

func (r *runner) planAndExam(ctx context.Context) (*Plan, []Check, []byte, error) {
	var (
		wg      sync.WaitGroup
		plan    *Plan
		planErr error
		checks  []Check
		patch   []byte
		examErr error
	)
	t0 := time.Now()
	views := []*cardRun{
		{n: 1, card: Card{Title: "Lead: split the task"}, state: stWorking, thought: "reading the repository", started: t0},
		{n: 2, card: Card{Title: "Exam writer"}, state: stWaiting, thought: "waiting for the lead's public names", started: t0},
	}
	finish := func(cr *cardRun, err error, proof string) {
		cr.set(func(c *cardRun) {
			c.ended = time.Now()
			if err != nil {
				c.state, c.err = stFailed, truncate(err.Error(), 70)
			} else {
				c.state, c.proof = stDone, proof
			}
		})
	}
	// The lead goes first and pins the public names (routes, fields, env vars).
	// The exam is written from the task and those names only, never the pieces,
	// so it tests behaviour instead of guessing names the builders chose.
	wg.Add(1)
	go func() {
		defer wg.Done()
		plan, planErr = r.lead(ctx)
		if planErr == nil {
			finish(views[0], nil, fmt.Sprintf("%d %s", len(plan.Cards), plural(len(plan.Cards), "piece", "pieces")))
		} else {
			finish(views[0], planErr, "")
			examErr = errors.New("no plan, so no exam")
			finish(views[1], examErr, "")
			return
		}
		views[1].set(func(c *cardRun) { c.state, c.thought, c.started = stWorking, "writing the hidden tests", time.Now() })
		checks, patch, examErr = r.writeExam(ctx, plan.Interface)
		if examErr == nil {
			finish(views[1], nil, fmt.Sprintf("%d checks", len(checks)))
		} else {
			finish(views[1], examErr, "")
		}
	}()
	r.watch("Plan", t0, views, &wg)
	if planErr != nil {
		return nil, nil, nil, fmt.Errorf("plan: %w", planErr)
	}
	if examErr != nil {
		return nil, nil, nil, fmt.Errorf("exam: %w", examErr)
	}
	return plan, checks, patch, nil
}

func (r *runner) lead(ctx context.Context) (*Plan, error) {
	feedback := ""
	for attempt := 1; attempt <= 2; attempt++ {
		out, err := hostAgent(ctx, r.o.LeadCmd, r.root, leadPrompt(r.o.Task, r.o.MaxCards, feedback), filepath.Join(r.dir, "logs", fmt.Sprintf("lead-%d.log", attempt)))
		if err != nil {
			return nil, fmt.Errorf("the lead agent failed: %v", err)
		}
		plan, err := parsePlan(out)
		if err == nil {
			err = plan.Validate(r.o.MaxCards)
		}
		if err == nil {
			if plan.Task == "" {
				plan.Task = r.o.Task
			}
			r.saveJSON("plan.json", plan)
			r.ev.log("plan.ready", map[string]any{"cards": len(plan.Cards)})
			return plan, nil
		}
		r.ev.log("plan.refused", map[string]any{"reason": err.Error()})
		r.ui.say("  %s Plan refused, sent back to the lead: %s", r.ui.amber("!"), truncate(err.Error(), 120))
		feedback = err.Error()
	}
	return nil, fmt.Errorf("the lead's plan was refused twice: %s", feedback)
}

func (r *runner) showPlan(p *Plan, checks []Check) {
	r.ui.say("\n  %s", r.ui.bold("The plan"))
	for i, c := range p.Cards {
		r.ui.say("  %02d %s  %s", i+1, c.Title, r.ui.dim(strings.Join(c.Owns, ", ")))
	}
	for _, s := range p.Seams {
		r.ui.say("     %s %s (owner %s)", r.ui.dim("seam:"), s.Contract, s.Owner)
	}
	r.ui.say("\n  %s", r.ui.bold("The exam")+r.ui.dim("  (written by a separate session; the agents never see it)"))
	for i, c := range checks {
		r.ui.say("  %d. %s", i+1, c.Text)
	}
}
