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
		{"src/api", "src/*/handlers.go", true},
		{"src/api", "src/**/handlers.go", true},
		{"api/*.go", "api/sub", false},
		{"api", "**/*_test.go", true},
		{"billing", "**/handlers/*.go", true},
		{"API/x.go", "api/x.go", true},
		{"Billing/**", "billing/retry.go", true},
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
	p, err = parsePlan(`{"task":"t","cards":[{"id":"c1","owns":["a"],"prompt":"p"}],"interface":["GET /a","POST /b"]}`)
	if err != nil || p.Interface != "GET /a\nPOST /b" || len(p.Cards) != 1 {
		t.Fatalf("interface as a list: %v %+v", err, p)
	}
}

func TestValidateRefusesUnsafeCardIDs(t *testing.T) {
	p := &Plan{Cards: []Card{{ID: "auth api", Title: "A", Owns: []string{"a/**"}, Prompt: "x"}}}
	if err := p.Validate(4); err == nil || !strings.Contains(err.Error(), "may use only") {
		t.Fatalf("an id with a space was accepted: %v", err)
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
	leaks, login := scanBox("PATH=/bin\nCLAUDE_CODE_OAUTH_TOKEN=sk-ant-oat01-abcdefghijklmnopqrstuv\n", "")
	if len(leaks) != 0 || !login {
		t.Fatalf("agent login reported as a leak: %v %v", leaks, login)
	}
	leaks, _ = scanBox("CLAUDE_CODE_OAUTH_TOKEN=sk-ant-oat01-abcdefghijklmnopqrstuv\nANTHROPIC_API_KEY=sk-ant-api03-abcdefghijklmnop\n", "")
	if len(leaks) != 1 {
		t.Fatalf("a real API key was missed: %v", leaks)
	}
	// Codex runs on an OpenAI key: expected for codex, a leak for claude.
	const openai = "OPENAI_API_KEY=sk-proj-abcdefghijklmnopqrstuvwxyz0123456789\n"
	if leaks, login := scanBox(openai, "codex"); len(leaks) != 0 || !login {
		t.Errorf("codex: its own login reported as a leak: %v %v", leaks, login)
	}
	if leaks, _ := scanBox(openai, "claude"); len(leaks) != 1 {
		t.Errorf("claude: an OpenAI key was not reported: %v", leaks)
	}
}

func TestInterfaceReachesExamAndBuilders(t *testing.T) {
	iface := "POST /evaluations/{id}/disposition body {\"decision\": ...}"
	if !strings.Contains(examPrompt("task", iface), iface) {
		t.Error("the exam prompt lacks the pinned interface")
	}
	p := &Plan{Interface: iface, Cards: []Card{{ID: "c1", Title: "A", Owns: []string{"a/**"}, Prompt: "x"}}}
	if !strings.Contains(workerPrompt(p, p.Cards[0], 1, "~/tw/repo"), iface) {
		t.Error("the builder prompt lacks the pinned interface")
	}
	if strings.Contains(examPrompt("task", ""), "The public interface is pinned") {
		t.Error("an empty interface should add nothing")
	}
}

func TestValidateNormalisesAndRefusesOwns(t *testing.T) {
	p := &Plan{Cards: []Card{
		{ID: "c1", Owns: []string{"./a/b/", `web\app.ts`, "x//y/./z.go"}, Prompt: "x"},
	}}
	if err := p.Validate(4); err != nil {
		t.Fatalf("refused: %v", err)
	}
	if got := strings.Join(p.Cards[0].Owns, ","); got != "a/b/**,web/app.ts,x/y/z.go" {
		t.Fatalf("owns not normalised: %s", got)
	}
	bad := map[string]string{
		"":                        "empty",
		"  ":                      "empty",
		"/etc/passwd":             "relative",
		"C:/x":                    "relative",
		"../x":                    "..",
		"a/../../x":               "..",
		".":                       "whole repository",
		"./":                      "whole repository",
		"*":                       "whole repository",
		"**":                      "whole repository",
		"**/*":                    "whole repository",
		"a/taskwing_exam_test.go": "reserved",
		"a/TaskWing_Exam/**":      "reserved",
	}
	for own, want := range bad {
		p := &Plan{Cards: []Card{{ID: "c1", Owns: []string{own}, Prompt: "x"}}}
		err := p.Validate(4)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("owns %q: got %v, want an error with %q", own, err, want)
		}
	}
}

func TestValidateOverlapIgnoresCase(t *testing.T) {
	p := &Plan{Cards: []Card{
		{ID: "c1", Owns: []string{"API/handler.go"}, Prompt: "x"},
		{ID: "c2", Owns: []string{"./api/handler.go"}, Prompt: "y"},
	}}
	if err := p.Validate(4); err == nil || !strings.Contains(err.Error(), "both own") {
		t.Fatalf("case-different paths accepted: %v", err)
	}
}

func TestValidateRefusesIDsDifferingInCase(t *testing.T) {
	p := &Plan{Cards: []Card{
		{ID: "api", Owns: []string{"a/**"}, Prompt: "x"},
		{ID: "API", Owns: []string{"b/**"}, Prompt: "y"},
	}}
	if err := p.Validate(4); err == nil || !strings.Contains(err.Error(), "differ only in case") {
		t.Fatalf("ids differing in case accepted: %v", err)
	}
}

func TestValidateSeams(t *testing.T) {
	cards := []Card{
		{ID: "c1", Owns: []string{"a/**"}, Prompt: "x"},
		{ID: "c2", Owns: []string{"b/**"}, Prompt: "y"},
		{ID: "c3", Owns: []string{"c/**"}, Prompt: "z"},
	}
	cases := []struct {
		seam Seam
		want string // "" means accepted
	}{
		{Seam{Between: []string{"c1", "c2"}, Owner: "c2"}, ""},
		{Seam{Between: []string{"c1"}, Owner: "c1"}, "two different cards"},
		{Seam{Between: []string{"c1", "c1"}, Owner: "c1"}, "two different cards"},
		{Seam{Between: []string{"c1", "c2", "c3"}, Owner: "c1"}, "two different cards"},
		{Seam{Between: []string{"c1", "nope"}, Owner: "c1"}, "two different cards"},
		{Seam{Between: []string{"c1", "c2"}, Owner: "c3"}, "not one of its two cards"},
		{Seam{Between: []string{"c1", "c2"}, Owner: "zz"}, "no owner"},
	}
	for _, c := range cases {
		p := &Plan{Cards: cards, Seams: []Seam{c.seam}}
		err := p.Validate(4)
		switch {
		case c.want == "" && err != nil:
			t.Errorf("%+v refused: %v", c.seam, err)
		case c.want != "" && (err == nil || !strings.Contains(err.Error(), c.want)):
			t.Errorf("%+v: got %v, want %q", c.seam, err, c.want)
		}
	}
}

func TestExtractJSONSkipsProse(t *testing.T) {
	plan := `{"task":"t","cards":[{"id":"c1","owns":["a"],"prompt":"use {id} routes"}]}`
	for _, in := range []string{
		"Routes like {id} stay.\n" + plan + "\nDone {really}.",
		"Here:\n```json\n" + plan + "\n```\nNote: closing } here",
		"{not json} " + plan,
		plan,
	} {
		got, err := extractJSON(in)
		if err != nil || got != plan {
			t.Errorf("extractJSON(%q) = %q, %v", in, got, err)
		}
	}
	for _, in := range []string{"", "no braces", "{id} and {name}", "[1, 2]"} {
		if got, err := extractJSON(in); err == nil {
			t.Errorf("extractJSON(%q) = %q, want an error", in, got)
		}
	}
}

func TestCardOverlaps(t *testing.T) {
	a := Card{Owns: []string{"a/**", "shared.go"}}
	b := Card{Owns: []string{"b/**", "shared.go"}}
	if mine, theirs, ok := a.overlaps(b); !ok || mine != "shared.go" || theirs != "shared.go" {
		t.Fatalf("overlaps = %q %q %v", mine, theirs, ok)
	}
	if _, _, ok := a.overlaps(Card{Owns: []string{"c/**"}}); ok {
		t.Fatal("disjoint cards overlap")
	}
}

func TestLeadPromptCardCount(t *testing.T) {
	if p := leadPrompt("t", 1, ""); strings.Contains(p, "2 to 1") || !strings.Contains(p, "Exactly 1 card") {
		t.Error("max 1 reads wrong")
	}
	if p := leadPrompt("t", 6, ""); !strings.Contains(p, "At most 6 cards") {
		t.Error("max 6 not stated")
	}
	p := &Plan{Cards: []Card{{ID: "c1", Owns: []string{"a/**"}, Prompt: "x"}}}
	if !strings.Contains(workerPrompt(p, p.Cards[0], 1, "~/r"), examMarker) {
		t.Error("workers are not told the exam paths are reserved")
	}
}

// A small object in the prose must not hide the answer that holds the key.
func TestExtractJSONPrefersWantedKey(t *testing.T) {
	plan := `{"cards":[{"id":"c1"}]}`
	got, err := extractJSON(`Example: {"id": 1} and {}.`+"\n"+plan, "cards")
	if err != nil || got != plan {
		t.Errorf("extractJSON = %q, %v; want the plan", got, err)
	}
	if got, err := extractJSON(`{"id": 1}`, "cards"); err != nil || got != `{"id": 1}` {
		t.Errorf("without the key the first object is the fallback, got %q, %v", got, err)
	}
}
