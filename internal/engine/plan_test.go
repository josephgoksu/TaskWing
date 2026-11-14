package engine

import (
	"strings"
	"testing"
)

func TestMatchOwn(t *testing.T) {
	cases := []struct {
		pattern, path string
		want          bool
	}{
		{"billing/retry/**", "billing/retry/schedule.go", true},
		{"billing/retry/**", "billing/retry/a/b.go", true},
		{"billing/retry/**", "billing/other.go", false},
		{"api/webhook.go", "api/webhook.go", true},
		{"api/webhook.go", "api/webhook_test.go", false},
		{"api/", "api/x/y.go", true},
		{"api", "api/x.go", true},
		{"api/*.go", "api/x.go", true},
		{"api/*.go", "api/sub/x.go", false},
		{"**/*_test.go", "a/b/c_test.go", true},
		{"**/*_test.go", "c_test.go", true},
		{"./web/**", "web/app.ts", true},
		{"web/?.ts", "web/a.ts", true},
	}
	for _, c := range cases {
		if got := MatchOwn(c.pattern, c.path); got != c.want {
			t.Errorf("MatchOwn(%q, %q) = %v, want %v", c.pattern, c.path, got, c.want)
		}
	}
}

func TestPatternsOverlap(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"billing/**", "api/**", false},
		{"billing/**", "billing/retry.go", true},
		{"billing/", "billing/retry/**", true},
		{"api/webhook.go", "api/webhook_test.go", false},
		{"api/webhook.go", "api/webhook.go", true},
		{"api", "api/x.go", true},
		{"**/*_test.go", "api/x.go", false},
		{"**/*_test.go", "api/**", true},
		{"web/*.ts", "web/app.ts", true},
	}
	for _, c := range cases {
		if got := patternsOverlap(c.a, c.b); got != c.want {
			t.Errorf("patternsOverlap(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
		if got := patternsOverlap(c.b, c.a); got != c.want {
			t.Errorf("patternsOverlap(%q, %q) = %v, want %v (reversed)", c.b, c.a, got, c.want)
		}
	}
}

func TestValidate(t *testing.T) {
	good := &Plan{Cards: []Card{
		{ID: "c1", Title: "A", Owns: []string{"a/**"}, Prompt: "x"},
		{ID: "c2", Title: "B", Owns: []string{"b/**"}, Prompt: "y"},
	}, Seams: []Seam{{Between: []string{"c1", "c2"}, Owner: "c1", Contract: "z"}}}
	if err := good.Validate(4); err != nil {
		t.Fatalf("good plan refused: %v", err)
	}
	bad := &Plan{Cards: []Card{
		{ID: "c1", Owns: []string{"a/**"}, Prompt: "x"},
		{ID: "c1", Owns: []string{"a/x.go"}, Prompt: ""},
		{ID: "c3", Prompt: "z"},
	}, Seams: []Seam{{Contract: "nobody owns me"}}}
	err := bad.Validate(2)
	if err == nil {
		t.Fatal("bad plan accepted")
	}
	for _, want := range []string{"3 cards", "used twice", "no prompt", "owns no files", "both own", "no owner"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
	if err := (&Plan{}).Validate(4); err == nil {
		t.Error("empty plan accepted")
	}
}

func TestParsePlanInFences(t *testing.T) {
	p, err := parsePlan("Sure.\n```json\n{\"task\":\"t\",\"cards\":[{\"id\":\"c1\",\"owns\":[\"a\"],\"prompt\":\"p\"}]}\n```\n")
	if err != nil || len(p.Cards) != 1 || p.Cards[0].ID != "c1" {
		t.Fatalf("parsePlan: %v %+v", err, p)
	}
	if _, err := parsePlan("no json here"); err == nil {
		t.Error("expected an error")
	}
}

func TestOutside(t *testing.T) {
	c := Card{Owns: []string{"a/**", "b.go"}}
	got := c.Outside([]string{"a/x.go", "b.go", "c.go"})
	if len(got) != 1 || got[0] != "c.go" {
		t.Fatalf("Outside = %v", got)
	}
}

func TestProofLine(t *testing.T) {
	cases := map[string]string{
		"done\nPROOF: 12 tests pass":              "12 tests pass",
		"**PROOF:** retries land at +24h":         "retries land at +24h",
		"PROOF: first\nmore\n  PROOF: second one": "second one",
		"no proof here":                           "",
	}
	for in, want := range cases {
		if got := proofLine(in); got != want {
			t.Errorf("proofLine(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFindSecrets(t *testing.T) {
	text := "PATH=/bin\nGH_TOKEN=ghp_abcdefghijklmnopqrstuvwxyz0123\nANTHROPIC_API_KEY=sk-ant-api03-abcdefghijklmnop\n"
	got := strings.Join(findSecrets(text), ",")
	for _, want := range []string{"GitHub token", "Anthropic API key", "GitHub token variable"} {
		if !strings.Contains(got, want) {
			t.Errorf("findSecrets missed %q (got %q)", want, got)
		}
	}
	if f := findSecrets("PATH=/bin\nGH_TOKEN=\nHOME=/root"); len(f) != 0 {
		t.Errorf("false positive: %v", f)
	}
}

func TestScanBoxAgentLogin(t *testing.T) {
	leaks, login := scanBox("PATH=/bin\nCLAUDE_CODE_OAUTH_TOKEN=sk-ant-oat01-abcdefghijklmnopqrstuv\n")
	if len(leaks) != 0 || !login {
		t.Fatalf("agent login reported as a leak: %v %v", leaks, login)
	}
	leaks, _ = scanBox("CLAUDE_CODE_OAUTH_TOKEN=sk-ant-oat01-abcdefghijklmnopqrstuv\nANTHROPIC_API_KEY=sk-ant-api03-abcdefghijklmnop\n")
	if len(leaks) != 1 {
		t.Fatalf("a real API key was missed: %v", leaks)
	}
}
