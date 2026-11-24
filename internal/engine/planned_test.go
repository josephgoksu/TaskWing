package engine

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/josephgoksu/TaskWing/v2/internal/box"
)

// A coding agent plans with PlanOnly, shows the plan to its user, and builds
// it with Approve on a yes: the plan and the exam are not made again.
func TestPlanOnlyThenApprove(t *testing.T) {
	quietSecrets(t)
	repo := newRepo(t)
	home := t.TempDir()
	prov := &box.Local{Root: filepath.Join(home, "boxes"), AgentCmd: "bash " + testdata(t, "agent.sh")}
	var out bytes.Buffer
	res, err := Run(context.Background(), Options{
		Task:     "add a greeting and a farewell",
		Dir:      repo,
		Provider: prov,
		Home:     home,
		LeadCmd:  "bash " + testdata(t, "lead.sh"),
		ExamCmd:  "bash " + testdata(t, "exam.sh"),
		PlanOnly: true,
		NoPR:     true,
		Out:      &out,
		In:       strings.NewReader(""),
	})
	if err != nil || !res.Stopped {
		t.Fatalf("plan-only: %v %+v\n%s", err, res, out.String())
	}
	// With the flags the plan was made with: a bare approve would build in
	// boat and open a pull request.
	if !strings.Contains(out.String(), "taskwing approve "+res.RunID+" --provider local --no-pr") {
		t.Errorf("plan-only output does not say how to approve:\n%s", out.String())
	}
	if left, _ := os.ReadDir(filepath.Join(home, "boxes")); len(left) != 0 {
		t.Errorf("plan-only started %d boxes", len(left))
	}

	// No lead and no exam writer: approving must not plan again.
	out.Reset()
	res2, err := Run(context.Background(), Options{
		Approve:  res.RunID,
		Dir:      repo,
		Provider: prov,
		Home:     home,
		LeadCmd:  "false",
		ExamCmd:  "false",
		NoPR:     true,
		Out:      &out,
		In:       strings.NewReader(""),
	})
	t.Log("\n" + out.String())
	if err != nil {
		t.Fatal(err)
	}
	if res2.RunID != res.RunID {
		t.Errorf("approve made run %s, want %s", res2.RunID, res.RunID)
	}
	checkJoined(t, repo, res2)

	// A plan is built once.
	_, err = Run(context.Background(), Options{Approve: res.RunID, Dir: repo, Provider: prov, Home: home, NoPR: true, Out: &out, In: strings.NewReader("")})
	if err == nil || !strings.Contains(err.Error(), "already approved") {
		t.Errorf("second approve: %v", err)
	}
}

func TestApproveRefusesMovedRepo(t *testing.T) {
	quietSecrets(t)
	repo := newRepo(t)
	home := t.TempDir()
	prov := &box.Local{Root: filepath.Join(home, "boxes"), AgentCmd: "false"}
	var out bytes.Buffer
	res, err := Run(context.Background(), Options{
		Task: "add a greeting and a farewell", Dir: repo, Provider: prov, Home: home,
		LeadCmd: "bash " + testdata(t, "lead.sh"), ExamCmd: "bash " + testdata(t, "exam.sh"),
		PlanOnly: true, NoPR: true, Out: &out, In: strings.NewReader(""),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "more.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	sh(t, repo, "git", "add", "-A")
	sh(t, repo, "git", "commit", "-q", "-m", "more")
	_, err = Run(context.Background(), Options{Approve: res.RunID, Dir: repo, Provider: prov, Home: home, NoPR: true, Out: &out, In: strings.NewReader("")})
	if err == nil || !strings.Contains(err.Error(), "moved since the plan") {
		t.Errorf("approve after a new commit: %v", err)
	}
	if _, err := os.Stat(filepath.Join(res.RunDir, plannedFile)); err != nil {
		t.Errorf("a refused approve used up the plan: %v", err)
	}
}

func TestApproveRejectsBadRunID(t *testing.T) {
	for _, id := range []string{"../etc", "tw-1", "tw-20261122-143005-a1b2/../x"} {
		if _, err := readPlanned(t.TempDir(), id); err == nil || !strings.Contains(err.Error(), "not a run id") {
			t.Errorf("%q: %v", id, err)
		}
	}
}
