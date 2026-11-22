package engine

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Square states, drawn as the agent squares on the board.
const (
	stWaiting = "waiting"
	stWorking = "working"
	stDone    = "done"
	stStuck   = "stuck"
)

type ui struct {
	out   io.Writer
	in    io.Reader
	tty   bool // out is a terminal
	inTTY bool // in is a terminal (someone can type during the build)
	color bool
	mu    sync.Mutex
	drawn int    // lines of the live board on screen
	hint  string // a dim line under the live board

	linesOnce sync.Once
	lineCh    chan string
}

func newUI(out io.Writer, in io.Reader) *ui {
	u := &ui{out: out, in: in}
	if f, ok := in.(*os.File); ok {
		if st, err := f.Stat(); err == nil && st.Mode()&os.ModeCharDevice != 0 {
			u.inTTY = true
		}
	}
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

// square draws one agent. Each state has its own shape as well as its own
// colour, so the board still reads with NO_COLOR or on a colour-blind screen.
func (u *ui) square(state string) string {
	switch state {
	case stDone:
		return u.c("90", "■")
	case stWorking:
		return u.c("34", "▣")
	case stStuck:
		return u.c("31", "✗")
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

// lines reads the user's input one line at a time, on one goroutine for the
// whole run, so a question and the build's "e" never read stdin at once.
// The channel closes when the input ends.
func (u *ui) lines() <-chan string {
	u.linesOnce.Do(func() {
		u.lineCh = make(chan string)
		go func() {
			defer close(u.lineCh)
			if u.in == nil {
				return
			}
			br := bufio.NewReader(u.in)
			for {
				line, err := br.ReadString('\n')
				if line != "" || err == nil {
					u.lineCh <- strings.TrimSpace(line)
				}
				if err != nil {
					return
				}
			}
		}()
	})
	return u.lineCh
}

// prompt prints q and returns the next line the user types. Ctrl-C (ctx)
// ends the wait at once; the end of the input reads as an empty answer.
func (u *ui) prompt(ctx context.Context, q string) (string, error) {
	u.mu.Lock()
	u.clearBoard()
	fmt.Fprint(u.out, q)
	u.mu.Unlock()
	select {
	case line, ok := <-u.lines():
		if !ok {
			fmt.Fprintln(u.out)
		}
		return line, nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

// ask asks a yes/no question; anything but y or yes is no.
func (u *ui) ask(ctx context.Context, q string) (bool, error) {
	a, err := u.prompt(ctx, q+" [y/N] ")
	a = strings.ToLower(a)
	return a == "y" || a == "yes", err
}

// width is the terminal's width in columns, or 100 when it is unknown.
func (u *ui) width() int {
	if n, err := strconv.Atoi(os.Getenv("COLUMNS")); err == nil && n > 0 {
		return n
	}
	if f, ok := u.out.(*os.File); ok {
		if n := termWidth(f); n > 0 {
			return n
		}
	}
	return 100
}

type cardView struct {
	n       int
	title   string
	state   string
	thought string
	took    time.Duration
}

// board draws one square per agent, then one line per agent: its square, its
// title and its last action or proof. On a terminal it redraws in place;
// piped output gets one plain line per change instead (see watchUntil).
func (u *ui) board(started time.Time, cards []cardView) {
	u.mu.Lock()
	defer u.mu.Unlock()
	if !u.tty {
		return
	}
	counts := map[string]int{}
	var sq []string
	for _, c := range cards {
		counts[c.state]++
		sq = append(sq, u.square(c.state))
	}
	var parts []string
	for _, k := range []string{stWorking, stWaiting, stDone, stStuck} {
		if counts[k] > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", counts[k], k))
		}
	}
	lines := []string{fmt.Sprintf("  %s  %s · %s", strings.Join(sq, " "), strings.Join(parts, ", "), dur(time.Since(started)))}
	// "  01 ▣ " is 7 columns and the title 22 plus a space: the rest of the
	// line is the agent's own, so it stays one line on a narrow terminal.
	const titleW = 22
	tailW := u.width() - 7 - titleW - 2
	if tailW < 10 {
		tailW = 10
	}
	for _, c := range cards {
		tail := c.thought
		if c.state == stDone && c.took > 0 {
			tail = dur(c.took) + "  " + tail
		}
		lines = append(lines, fmt.Sprintf("  %02d %s %-*s %s", c.n, u.square(c.state), titleW, truncate(c.title, titleW), u.dim(truncate(tail, tailW))))
	}
	if u.hint != "" {
		lines = append(lines, "  "+u.dim(u.hint))
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
	u.hint = ""
	u.mu.Unlock()
}
