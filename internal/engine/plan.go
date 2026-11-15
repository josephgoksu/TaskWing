package engine

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// Plan is what the lead agent writes: the task split into cards that own
// separate files.
type Plan struct {
	Task  string `json:"task"`
	Cards []Card `json:"cards"`
	Seams []Seam `json:"seams,omitempty"`
	// Interface pins the public names the task implies (routes, JSON fields,
	// env vars, flags, exported functions). Builders and the exam both use it.
	Interface string `json:"interface,omitempty"`
}

// Card is one piece of the work, built by one agent in one box.
type Card struct {
	ID     string   `json:"id"`
	Title  string   `json:"title"`
	Owns   []string `json:"owns"`
	Prompt string   `json:"prompt"`
	Where  string   `json:"where,omitempty"` // hunt: where the bug is
	Why    string   `json:"why,omitempty"`   // hunt: what goes wrong, in plain words
}

// Seam is a contract between two cards, owned by one of them.
type Seam struct {
	Between  []string `json:"between"`
	Owner    string   `json:"owner"`
	Contract string   `json:"contract"`
}

// Validate refuses a plan the engine cannot run safely: no cards, too many,
// two cards that own the same path, or a seam without an owner.
func (p *Plan) Validate(maxCards int) error {
	var errs []string
	if len(p.Cards) == 0 {
		errs = append(errs, "the plan has no cards")
	}
	if maxCards > 0 && len(p.Cards) > maxCards {
		errs = append(errs, fmt.Sprintf("the plan has %d cards; the most allowed is %d", len(p.Cards), maxCards))
	}
	ids := map[string]bool{}
	for i, c := range p.Cards {
		if c.ID == "" {
			errs = append(errs, fmt.Sprintf("card %d has no id", i+1))
		}
		if ids[c.ID] {
			errs = append(errs, fmt.Sprintf("card id %q is used twice", c.ID))
		}
		ids[c.ID] = true
		if strings.TrimSpace(c.Prompt) == "" {
			errs = append(errs, fmt.Sprintf("card %s has no prompt", c.ID))
		}
		if len(c.Owns) == 0 {
			errs = append(errs, fmt.Sprintf("card %s owns no files", c.ID))
		}
	}
	for i := 0; i < len(p.Cards); i++ {
		for j := i + 1; j < len(p.Cards); j++ {
			for _, a := range p.Cards[i].Owns {
				for _, b := range p.Cards[j].Owns {
					if patternsOverlap(a, b) {
						errs = append(errs, fmt.Sprintf("cards %s and %s both own %q / %q", p.Cards[i].ID, p.Cards[j].ID, a, b))
					}
				}
			}
		}
	}
	for _, s := range p.Seams {
		if s.Owner == "" || !ids[s.Owner] {
			errs = append(errs, fmt.Sprintf("seam %q has no owner card", s.Contract))
		}
	}
	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

// parsePlan takes the lead's output and finds the JSON object in it.
func parsePlan(out string) (*Plan, error) {
	js, err := extractJSON(out)
	if err != nil {
		return nil, err
	}
	var p Plan
	if err := json.Unmarshal([]byte(js), &p); err != nil {
		return nil, fmt.Errorf("the plan is not valid JSON: %v", err)
	}
	return &p, nil
}

// extractJSON returns the outermost {...} block in text, skipping code fences.
func extractJSON(text string) (string, error) {
	start := strings.IndexByte(text, '{')
	end := strings.LastIndexByte(text, '}')
	if start < 0 || end <= start {
		return "", errors.New("no JSON object in the output")
	}
	return text[start : end+1], nil
}

// MatchOwn reports whether path matches an ownership pattern. Patterns use
// * (one path segment), ** (any depth) and ?. A pattern ending in / owns the folder.
func MatchOwn(pattern, path string) bool {
	pattern = strings.TrimPrefix(strings.TrimSpace(pattern), "./")
	path = strings.TrimPrefix(path, "./")
	if strings.HasSuffix(pattern, "/") {
		pattern += "**"
	}
	if !hasWild(pattern) {
		return pattern == path || strings.HasPrefix(path, pattern+"/")
	}
	return globRegexp(pattern).MatchString(path)
}

func hasWild(p string) bool { return strings.ContainsAny(p, "*?[") }

func globRegexp(p string) *regexp.Regexp {
	var b strings.Builder
	b.WriteString("^")
	for i := 0; i < len(p); i++ {
		c := p[i]
		switch {
		case c == '*' && i+1 < len(p) && p[i+1] == '*':
			i++
			if i+1 < len(p) && p[i+1] == '/' {
				i++
				b.WriteString("(?:.*/)?")
			} else {
				b.WriteString(".*")
			}
		case c == '*':
			b.WriteString("[^/]*")
		case c == '?':
			b.WriteString("[^/]")
		default:
			b.WriteString(regexp.QuoteMeta(string(c)))
		}
	}
	b.WriteString("$")
	return regexp.MustCompile(b.String())
}

// staticPrefix is the part of a pattern before its first wildcard.
func staticPrefix(p string) string {
	if i := strings.IndexAny(p, "*?["); i >= 0 {
		return p[:i]
	}
	return p
}

// patternsOverlap is conservative: when in doubt it says yes, and the lead
// gets the plan back to fix.
func patternsOverlap(a, b string) bool {
	a = strings.TrimPrefix(strings.TrimSpace(a), "./")
	b = strings.TrimPrefix(strings.TrimSpace(b), "./")
	if strings.HasSuffix(a, "/") {
		a += "**"
	}
	if strings.HasSuffix(b, "/") {
		b += "**"
	}
	switch {
	case !hasWild(a) && !hasWild(b):
		return a == b || strings.HasPrefix(a, b+"/") || strings.HasPrefix(b, a+"/")
	case !hasWild(a):
		return MatchOwn(b, a) || strings.HasPrefix(staticPrefix(b), a+"/")
	case !hasWild(b):
		return MatchOwn(a, b) || strings.HasPrefix(staticPrefix(a), b+"/")
	}
	pa, pb := staticPrefix(a), staticPrefix(b)
	return strings.HasPrefix(pa, pb) || strings.HasPrefix(pb, pa)
}

// Outside returns the changed files that no pattern of the card owns.
func (c Card) Outside(files []string) []string {
	var out []string
	for _, f := range files {
		ok := false
		for _, p := range c.Owns {
			if MatchOwn(p, f) {
				ok = true
				break
			}
		}
		if !ok {
			out = append(out, f)
		}
	}
	return out
}
