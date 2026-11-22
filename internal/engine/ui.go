package engine

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

// Square states, drawn as the agent squares on the board.
const (
	stWaiting = "waiting"
	stWorking = "working"
	stDone    = "done"
	stFailed  = "stuck"
)

type ui struct {
	out   io.Writer
	in    *bufio.Reader
	tty   bool
	color bool
	mu    sync.Mutex
	drawn int // lines of the live board on screen
}

func newUI(out io.Writer, in io.Reader) *ui {
	u := &ui{out: out, in: bufio.NewReader(in)}
	if f, ok := out.(*os.File); ok {
		if st, err := f.Stat(); err == nil && st.Mode()&os.ModeCharDevice != 0 {
			u.tty = true
			u.color = os.Getenv("NO_COLOR") == ""
		}
	}
	return u
}

func (u *ui) c(code, s string) string {
	if !u.color {
		return s
	}
	return "\x1b[" + code + "m" + s + "\x1b[0m"
}

func (u *ui) bold(s string) string  { return u.c("1", s) }
func (u *ui) dim(s string) string   { return u.c("2", s) }
func (u *ui) green(s string) string { return u.c("32", s) }
func (u *ui) red(s string) string   { return u.c("31", s) }
func (u *ui) amber(s string) string { return u.c("33", s) }

func (u *ui) square(state string) string {
	switch state {
	case stDone:
		return u.c("90", "■")
	case stWorking:
		return u.c("34", "■")
	case stFailed:
		return u.c("31", "■")
	default:
		return "□"
	}
}

// say prints a line above the live board.
func (u *ui) say(format string, a ...any) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.clearBoard()
	fmt.Fprintf(u.out, format+"\n", a...)
}

func (u *ui) step(n int, title string) {
	u.say("\n%s %s", u.bold(fmt.Sprintf("%d/4", n)), u.bold(title))
}

func (u *ui) clearBoard() {
	if u.tty && u.drawn > 0 {
		fmt.Fprintf(u.out, "\x1b[%dA\x1b[J", u.drawn)
	}
	u.drawn = 0
}

func (u *ui) ask(q string) bool {
	u.mu.Lock()
	u.clearBoard()
	fmt.Fprintf(u.out, "%s [y/N] ", q)
	u.mu.Unlock()
	line, _ := u.in.ReadString('\n')
	a := strings.ToLower(strings.TrimSpace(line))
	return a == "y" || a == "yes"
}

type cardView struct {
	n       int
	title   string
	state   string
	thought string
	since   time.Time
	took    time.Duration
}

// board draws one square per agent, then one line per agent. On a terminal it
// redraws in place; otherwise it prints only when something changed.
func (u *ui) board(stage string, started time.Time, cards []cardView) {
	u.mu.Lock()
	defer u.mu.Unlock()
	counts := map[string]int{}
	var sq []string
	for _, c := range cards {
		counts[c.state]++
		sq = append(sq, u.square(c.state))
	}
	var parts []string
	for _, k := range []string{stWorking, stWaiting, stDone, stFailed} {
		if counts[k] > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", counts[k], k))
		}
	}
	lines := []string{fmt.Sprintf("  %s  %s · %s · %s", strings.Join(sq, " "), stage, strings.Join(parts, ", "), dur(time.Since(started)))}
	for _, c := range cards {
		tail := c.thought
		if c.state == stDone && c.took > 0 {
			tail = dur(c.took) + "  " + tail
		}
		lines = append(lines, fmt.Sprintf("  %02d %s %-8s %-26s %s", c.n, u.square(c.state), c.state, truncate(c.title, 26), u.dim(truncate(tail, 70))))
	}
	if !u.tty {
		return
	}
	u.clearBoard()
	// Autowrap off while drawing: a line wider than the terminal is cut at the
	// edge instead of taking two rows, so the next redraw clears all of it.
	fmt.Fprint(u.out, "\x1b[?7l")
	for _, l := range lines {
		fmt.Fprintln(u.out, l)
	}
	fmt.Fprint(u.out, "\x1b[?7h")
	u.drawn = len(lines)
}

// release leaves the last board on screen and stops redrawing it.
func (u *ui) release() {
	u.mu.Lock()
	u.drawn = 0
	u.mu.Unlock()
}
