package engine

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

// A plan-only run (--plan-only) stops after the plan and the exam and leaves
// them in its run folder with planned.json. `taskwing approve <run id>` then
// builds that plan with that exam, without planning again. This lets a
// coding agent show the plan to its user and start the run only after the
// user said yes, since an agent's shell cannot answer the run's prompt.

const (
	plannedFile = "planned.json"
	usedFile    = "planned.used.json" // planned.json after approve: a plan is built once
)

type planned struct {
	Task    string    `json:"task"`
	Hunt    bool      `json:"hunt,omitempty"`
	Repo    string    `json:"repo"`
	Base    string    `json:"base"`
	Created time.Time `json:"created"`
}

var runIDPattern = regexp.MustCompile(`^tw-\d{8}-\d{6}-[0-9a-f]{4}$`)

func readPlanned(home, id string) (*planned, error) {
	if !runIDPattern.MatchString(id) {
		return nil, fmt.Errorf("%q is not a run id (they look like tw-20261122-143005-a1b2)", id)
	}
	dir := filepath.Join(home, "runs", id)
	b, err := os.ReadFile(filepath.Join(dir, plannedFile))
	if errors.Is(err, os.ErrNotExist) {
		if _, uerr := os.Stat(filepath.Join(dir, usedFile)); uerr == nil {
			return nil, fmt.Errorf("run %s was already approved; plan again for a new run", id)
		}
		return nil, fmt.Errorf("no saved plan for run %s: start one with taskwing run \"<task>\" --plan-only", id)
	}
	if err != nil {
		return nil, err
	}
	var p planned
	if err := json.Unmarshal(b, &p); err != nil {
		return nil, fmt.Errorf("run %s: %s is damaged: %w", id, plannedFile, err)
	}
	if p.Task == "" && !p.Hunt {
		return nil, fmt.Errorf("run %s: %s has no task", id, plannedFile)
	}
	return &p, nil
}

func (r *runner) savePlanned() error {
	b, err := json.MarshalIndent(planned{Task: r.o.Task, Hunt: r.o.Hunt, Repo: r.root, Base: r.base, Created: time.Now().UTC()}, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(r.dir, plannedFile), b, filePerm); err != nil {
		return fmt.Errorf("could not save the plan for later: %w", err)
	}
	return nil
}

// loadPlanned reads the saved plan and exam back, after checking the user is
// in the same repository at the same commit: the exam's tests were written
// against that commit, and the agents start from it.
func (r *runner) loadPlanned() (*Plan, []Check, []byte, error) {
	p, err := readPlanned(r.o.Home, r.id)
	if err != nil {
		return nil, nil, nil, err
	}
	if p.Repo != r.root {
		return nil, nil, nil, fmt.Errorf("run %s was planned in %s; run taskwing approve there", r.id, p.Repo)
	}
	if p.Base != r.base {
		return nil, nil, nil, fmt.Errorf("the repository moved since the plan (planned at %.8s, now at %.8s): plan again", p.Base, r.base)
	}
	var plan Plan
	if err := readJSON(filepath.Join(r.dir, "plan.json"), &plan); err != nil {
		return nil, nil, nil, err
	}
	if err := plan.Validate(0); err != nil {
		return nil, nil, nil, fmt.Errorf("run %s: the saved plan is unsafe: %w", r.id, err)
	}
	var checks []Check
	if err := readJSON(filepath.Join(r.dir, "exam", "checks.json"), &checks); err != nil {
		return nil, nil, nil, err
	}
	if len(checks) == 0 {
		return nil, nil, nil, fmt.Errorf("run %s: the saved exam has no checks", r.id)
	}
	patch, err := os.ReadFile(filepath.Join(r.dir, "exam", "exam.patch"))
	if err != nil {
		return nil, nil, nil, fmt.Errorf("run %s: the saved exam is missing: %w", r.id, err)
	}
	// Used once: a second approve of the same plan would build it again
	// into the same run folder and branch.
	if err := os.Rename(filepath.Join(r.dir, plannedFile), filepath.Join(r.dir, usedFile)); err != nil {
		return nil, nil, nil, err
	}
	return &plan, checks, patch, nil
}

func readJSON(path string, v any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("could not read the saved %s: %w", filepath.Base(path), err)
	}
	if err := json.Unmarshal(b, v); err != nil {
		return fmt.Errorf("the saved %s is damaged: %w", filepath.Base(path), err)
	}
	return nil
}
