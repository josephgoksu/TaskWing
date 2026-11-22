package engine

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"regexp"
	"strings"
	"sync"
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

// UnmarshalJSON accepts "interface" as a string, or as a list or object, which
// leads often write for "one per line"; anything but a string is flattened.
func (p *Plan) UnmarshalJSON(data []byte) error {
	type plain Plan
	var raw struct {
		plain
		Interface json.RawMessage `json:"interface,omitempty"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*p = Plan(raw.plain)
	p.Interface = flattenJSON(raw.Interface)
	return nil
}

// flattenJSON turns a JSON string, list or object into plain lines.
func flattenJSON(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var list []json.RawMessage
	if json.Unmarshal(raw, &list) == nil {
		lines := make([]string, 0, len(list))
		for _, item := range list {
			lines = append(lines, flattenJSON(item))
		}
		return strings.Join(lines, "\n")
	}
	var buf bytes.Buffer
	if json.Indent(&buf, raw, "", "  ") == nil {
		return buf.String()
	}
	return string(raw)
}

var cardIDRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*$`)

// validCardID reports whether id can name a git branch, a ref and a log file.
func validCardID(id string) bool { return cardIDRe.MatchString(id) }

// Seam is a contract between two cards, owned by one of them.
type Seam struct {
	Between  []string `json:"between"`
	Owner    string   `json:"owner"`
	Contract string   `json:"contract"`
}

// Validate refuses a plan the engine cannot run safely: no cards, too many,
// two cards that own the same path, or a seam without an owner. It stores each
// card's owns patterns back in normal form, so MatchOwn and Outside see exactly
// what was checked here.
func (p *Plan) Validate(maxCards int) error {
	var errs []string
	if len(p.Cards) == 0 {
		errs = append(errs, "the plan has no cards")
	}
	if maxCards > 0 && len(p.Cards) > maxCards {
		errs = append(errs, fmt.Sprintf("the plan has %d cards; the most allowed is %d", len(p.Cards), maxCards))
	}
	ids := map[string]bool{}
	// Ids name git refs: on macOS and Windows "API" and "api" are the same
	// loose ref file, so two cards would overwrite each other's commits.
	folded := map[string]string{}
	for i := range p.Cards {
		c := &p.Cards[i]
		if c.ID == "" {
			errs = append(errs, fmt.Sprintf("card %d has no id", i+1))
		} else if !validCardID(c.ID) {
			errs = append(errs, fmt.Sprintf("card id %q may use only letters, digits, - and _", c.ID))
		}
		if ids[c.ID] {
			errs = append(errs, fmt.Sprintf("card id %q is used twice", c.ID))
		} else if prev, ok := folded[strings.ToLower(c.ID)]; ok {
			errs = append(errs, fmt.Sprintf("card ids %q and %q differ only in case", prev, c.ID))
		}
		ids[c.ID] = true
		folded[strings.ToLower(c.ID)] = c.ID
		if strings.TrimSpace(c.Prompt) == "" {
			errs = append(errs, fmt.Sprintf("card %s has no prompt", c.ID))
		}
		if len(c.Owns) == 0 {
			errs = append(errs, fmt.Sprintf("card %s owns no files", c.ID))
		}
		owns := make([]string, 0, len(c.Owns))
		for _, o := range c.Owns {
			n, err := normOwn(o)
			if err != nil {
				errs = append(errs, fmt.Sprintf("card %s owns %q: %v", c.ID, o, err))
				continue
			}
			owns = append(owns, n)
		}
		c.Owns = owns
	}
	for i := 0; i < len(p.Cards); i++ {
		for j := i + 1; j < len(p.Cards); j++ {
			if a, b, ok := p.Cards[i].overlaps(p.Cards[j]); ok {
				errs = append(errs, fmt.Sprintf("cards %s and %s both own %q / %q", p.Cards[i].ID, p.Cards[j].ID, a, b))
			}
		}
	}
	for _, s := range p.Seams {
		if s.Owner == "" || !ids[s.Owner] {
			errs = append(errs, fmt.Sprintf("seam %q has no owner card", s.Contract))
			continue
		}
		// A seam names the two cards that share a contract; workers only see
		// seams they are in, so a typo or a lone id hides the contract from one side.
		if len(s.Between) != 2 || s.Between[0] == s.Between[1] || !ids[s.Between[0]] || !ids[s.Between[1]] {
			errs = append(errs, fmt.Sprintf("seam %q must be between two different cards of the plan", s.Contract))
		} else if s.Owner != s.Between[0] && s.Owner != s.Between[1] {
			errs = append(errs, fmt.Sprintf("seam %q is owned by %s, which is not one of its two cards", s.Contract, s.Owner))
		}
	}
	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

// normOwn puts an owns pattern in one form: forward slashes, no leading ./,
// cleaned, a trailing / turned into /**. It refuses patterns that escape the
// repository, that own all of it (the split then protects nothing), or that
// reach the exam's reserved paths (a worker file there makes the hidden tests
// fail to apply in the exam box).
func normOwn(p string) (string, error) {
	s := strings.ReplaceAll(strings.TrimSpace(p), `\`, "/")
	switch {
	case s == "":
		return "", errors.New("the pattern is empty")
	case strings.HasPrefix(s, "/") || (len(s) >= 2 && s[1] == ':'):
		return "", errors.New("the pattern must be relative to the repository root")
	}
	for _, seg := range strings.Split(s, "/") {
		if seg == ".." {
			return "", errors.New("the pattern may not use ..")
		}
	}
	dir := strings.HasSuffix(s, "/")
	s = path.Clean(s)
	if dir && s != "." {
		s += "/**"
	}
	if s == "." || strings.Trim(s, "*/") == "" {
		return "", errors.New("the pattern owns the whole repository")
	}
	if strings.Contains(strings.ToLower(s), examMarker) {
		return "", fmt.Errorf("paths containing %q are reserved for the exam", examMarker)
	}
	return s, nil
}

// overlaps reports the first pair of patterns that c and o both own.
func (c Card) overlaps(o Card) (mine, theirs string, ok bool) {
	for _, a := range c.Owns {
		for _, b := range o.Owns {
			if patternsOverlap(a, b) {
				return a, b, true
			}
		}
	}
	return "", "", false
}

// parsePlan takes the lead's output and finds the JSON object in it.
func parsePlan(out string) (*Plan, error) {
	js, err := extractJSON(out)
	if err != nil {
		return nil, err
	}
	var p Plan
	if err := json.Unmarshal([]byte(js), &p); err != nil {
		return nil, fmt.Errorf("the plan is not valid JSON: %w", err)
	}
	return &p, nil
}

// extractJSON returns the first JSON object in text. Taking the first { to
// the last } breaks on prose around it ("routes like {id}", a closing "}" in
// a note), so it decodes from each { in turn and keeps the first that parses.
func extractJSON(text string) (string, error) {
	for i := 0; i < len(text); i++ {
		j := strings.IndexByte(text[i:], '{')
		if j < 0 {
			break
		}
		i += j
		var raw json.RawMessage
		if json.NewDecoder(strings.NewReader(text[i:])).Decode(&raw) != nil {
			continue
		}
		var obj map[string]json.RawMessage
		if json.Unmarshal(raw, &obj) == nil {
			return string(raw), nil
		}
	}
	return "", errors.New("no JSON object in the output")
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

// globCache holds compiled patterns: Validate and hunt compare every pattern
// with every other one, and recompiling each time made large plans slow.
var globCache = struct {
	sync.Mutex
	m map[string]*regexp.Regexp
}{m: map[string]*regexp.Regexp{}}

func globRegexp(p string) *regexp.Regexp {
	globCache.Lock()
	defer globCache.Unlock()
	if re, ok := globCache.m[p]; ok {
		return re
	}
	re := compileGlob(p)
	globCache.m[p] = re
	return re
}

func compileGlob(p string) *regexp.Regexp {
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
// gets the plan back to fix. It ignores case: on macOS and Windows "API/x.go"
// and "api/x.go" are one file, and two cards writing it conflict at join.
func patternsOverlap(a, b string) bool {
	a = strings.ToLower(strings.TrimPrefix(strings.TrimSpace(a), "./"))
	b = strings.ToLower(strings.TrimPrefix(strings.TrimSpace(b), "./"))
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
		return MatchOwn(b, a) || strings.HasPrefix(staticPrefix(b), a+"/") || globReachesInside(b, a)
	case !hasWild(b):
		return MatchOwn(a, b) || strings.HasPrefix(staticPrefix(a), b+"/") || globReachesInside(a, b)
	}
	pa, pb := staticPrefix(a), staticPrefix(b)
	return strings.HasPrefix(pa, pb) || strings.HasPrefix(pb, pa)
}

// globReachesInside reports whether glob g can match a path inside folder dir
// (a plain pattern owns everything below it): "src/*/x.go" reaches into "src/api".
// The leading segments of g must match dir's segments; a ** reaches anywhere
// below, so "**/*_test.go" reaches into "api". A plain pattern whose last
// segment has an extension ("api/x.go") is taken for a file, or a leading **
// would collide with every file another card names.
func globReachesInside(g, dir string) bool {
	gs, ds := strings.Split(g, "/"), strings.Split(dir, "/")
	for i, seg := range ds {
		if i >= len(gs) {
			return false
		}
		if gs[i] == "**" {
			return i > 0 || !strings.Contains(path.Base(dir)[1:], ".")
		}
		if !globRegexp(gs[i]).MatchString(seg) {
			return false
		}
	}
	return len(gs) > len(ds)
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
