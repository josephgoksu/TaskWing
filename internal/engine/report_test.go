package engine

import (
	"strings"
	"testing"
	"time"
)

func reportFixture() (*Plan, []*cardRun, []Check, *Result) {
	plan := &Plan{Task: "Add retries, ping @octocat"}
	cards := []*cardRun{
		{n: 1, card: Card{Title: "Retry"}, state: stDone, joined: true, proof: "ok"},
		{n: 2, card: Card{Title: "Hooks"}, state: stDone, joined: true, early: true, outside: []string{"a`b.go", "c.go"}},
		{n: 3, card: Card{Title: "Docs"}, state: stDone, joined: true, outside: []string{"d.go"}},
	}
	checks := []Check{
		{ID: "k1", Text: "Retries <b>work</b>", Command: "go test ./...", Output: "```\n# injected heading\n````\n"},
		{ID: "k2", Text: "Fine", Passed: true},
	}
	return plan, cards, checks, &Result{RunID: "r1"}
}

func TestPRBodyFencesHostileOutput(t *testing.T) {
	plan, cards, checks, res := reportFixture()
	body := prBody(plan, cards, checks, res, time.Minute)
	if !strings.Contains(body, "\n`````\n$ go test ./...\n") {
		t.Fatalf("output fence is not longer than the output's backtick runs:\n%s", body)
	}
	if !strings.Contains(body, "<summary>✗ Retries &lt;b&gt;work&lt;/b&gt;</summary>") {
		t.Errorf("summary text not HTML-escaped:\n%s", body)
	}
	if strings.Contains(body, "@octocat") {
		t.Errorf("an @mention in the task survived:\n%s", body)
	}
}

func TestPRBodyMarksAndNotes(t *testing.T) {
	plan, cards, checks, res := reportFixture()
	body := prBody(plan, cards, checks, res, time.Minute)
	if !strings.Contains(body, "| ◐ | 02 Hooks |") || !strings.Contains(body, "| ✓ | 01 Retry |") {
		t.Errorf("early card not marked ◐:\n%s", body)
	}
	if !strings.Contains(body, "- Piece 02 changed files outside its area: ``a`b.go``, `c.go`\n- Piece 03") {
		t.Errorf("outside notes not a list of escaped names:\n%s", body)
	}
	if strings.Contains(body, "> Piece") {
		t.Errorf("outside notes still quote lines:\n%s", body)
	}
	if !strings.Contains(body, "Written before the build by a separate session") {
		t.Errorf("exam header missing:\n%s", body)
	}
	checks = append(checks, Check{ID: "u1", Text: "Mine", User: true, Passed: true})
	body = prBody(plan, cards, checks, res, time.Minute)
	if strings.Contains(body, "Written before the build by a separate session.") {
		t.Errorf("exam header claims user checks were written before the build:\n%s", body)
	}
}

func TestPRBodySecretScanLine(t *testing.T) {
	plan, cards, checks, res := reportFixture()
	if body := prBody(plan, cards, checks, res, 0); !strings.Contains(body, "Secret scan: not run (local mode).") {
		t.Errorf("unscanned run claims a scan:\n%s", body)
	}
	res.Scanned = true
	if body := prBody(plan, cards, checks, res, 0); !strings.Contains(body, "Secret scan: nothing found") {
		t.Errorf("clean scan line missing:\n%s", body)
	}
	res.Secrets = []string{"GitHub token in box 2"}
	if body := prBody(plan, cards, checks, res, 0); !strings.Contains(body, "Secret scan found: GitHub token in box 2") {
		t.Errorf("findings missing:\n%s", body)
	}
}

func TestFenceAndCode(t *testing.T) {
	cases := map[string]string{"plain": "```", "a ``` b": "````", "`````": "``````"}
	for in, want := range cases {
		if got := fence(in); got != want {
			t.Errorf("fence(%q) = %q, want %q", in, got, want)
		}
	}
	if got := code("`x"); got != "`` `x ``" {
		t.Errorf("code = %q", got)
	}
}

// git -z hands file names over raw: a newline in one must not end the list
// item and start markdown of its own.
func TestPRBodyQuotesControlCharsInNames(t *testing.T) {
	plan, cards, checks, res := reportFixture()
	cards[2].outside = []string{"x\n\n# All checks passed"}
	body := prBody(plan, cards, checks, res, 0)
	if strings.Contains(body, "\n# All checks passed") {
		t.Fatalf("a newline in a file name injected markdown:\n%s", body)
	}
	if !strings.Contains(body, `"x\n\n# All checks passed"`) {
		t.Errorf("the name is not shown quoted:\n%s", body)
	}
}
