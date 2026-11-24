// Command taskwing runs one task with several coding agents in sandboxes and
// opens one draft pull request.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/josephgoksu/TaskWing/v2/internal/box"
	"github.com/josephgoksu/TaskWing/v2/internal/engine"
)

// version is set by GoReleaser (-X main.version=vX.Y.Z). Without it, a build
// from `go install ...@vX.Y.Z` reports its module version instead.
var version = ""

func versionString() string {
	if version != "" {
		return version
	}
	if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
		return bi.Main.Version
	}
	return "dev"
}

const (
	defaultLead  = `claude -p --output-format text --allowedTools "Read,Glob,Grep,Bash(git log:*),Bash(git ls-files:*)"`
	defaultExam  = `claude -p --output-format text --allowedTools "Read,Glob,Grep,Write,Edit,Bash"`
	defaultLocal = `claude -p --dangerously-skip-permissions`
)

// Exit codes: 0 done, 1 error or stopped, 2 usage, 3 the exam ran and some
// checks failed (so scripts can tell "it ran, the work is not right" apart
// from "it did not run").
const (
	exitError  = 1
	exitUsage  = 2
	exitFailed = 3
)

// errChecksFailed ends a run whose exam had failures. main turns it into
// exit 3 after every deferred cleanup has run.
var errChecksFailed = errors.New("some checks failed")

// errUsage marks a mistake in the command line (exit 2).
type errUsage struct{ msg string }

func (e errUsage) Error() string { return e.msg }

func main() { os.Exit(run(os.Args[1:])) }

func run(args []string) int {
	if len(args) == 0 {
		usage(os.Stderr)
		return exitUsage
	}
	var err error
	switch args[0] {
	case "run":
		err = cmdRun(args[1:], false)
	case "hunt":
		err = cmdRun(args[1:], true)
	case "config":
		err = cmdConfig(args[1:])
	case "doctor":
		err = cmdDoctor(args[1:])
	case "engine":
		err = cmdEngine(args[1:])
	case "version", "--version", "-v":
		fmt.Println("taskwing", versionString())
	case "help", "--help", "-h":
		usage(os.Stdout)
	default:
		fmt.Fprintf(os.Stderr, "taskwing: unknown command %q (taskwing help lists them)\n", args[0])
		return exitUsage
	}
	var u errUsage
	switch {
	case err == nil:
		return 0
	case errors.Is(err, flag.ErrHelp):
		usage(os.Stdout)
		return 0
	case errors.Is(err, errChecksFailed):
		return exitFailed
	case errors.As(err, &u):
		fmt.Fprintln(os.Stderr, "taskwing:", err)
		return exitUsage
	default:
		fmt.Fprintln(os.Stderr, "taskwing:", err)
		return exitError
	}
}

func usage(w io.Writer) {
	fmt.Fprint(w, `TaskWing: give it one task; a team of agents builds it in sandboxes, and you get back one tested pull request.

New here? In a git repository, try it on this machine first:
  taskwing run "add a /health endpoint" --provider local --no-pr

Commands
  taskwing run "<task>" [flags]    plan · you approve the exam · agents build · exam · draft PR
  taskwing hunt ["<area>"] [flags] find bugs, prove each with a failing test, fix them
  taskwing config [boat-base <id>] show or save settings (~/.taskwing/config.json)
  taskwing doctor [--provider boat|local] [--clean]
                                   find (and delete) leftover sandboxes and agent refs
  taskwing engine --json [--max-cards n] [--keep]
                                   the same runs for scripts: JSON lines on stdin and stdout
  taskwing version

Run flags
  --provider boat|local   where agents run (default boat; local = folders on this machine, not a sandbox)
  --boat-base <id>        boat box to fork (or taskwing config boat-base <id>)
  --boat-env <name>       boat environment for the forks
  --agent <name>          coding agent in each boat box: `+strings.Join(box.Agents, ", ")+` (default `+box.DefaultAgent+`)
  --model <name>          agent model in boat (default `+box.DefaultModel+` for claude, else the boat dashboard's)
  --effort <level>        agent effort in boat (default `+box.DefaultEffort+` for claude, else the boat dashboard's)
  --max-cards <n>         most agents the lead may plan, or bugs a hunt keeps (default 4)
  --yes                   do not ask before starting or before the PR
  --no-pr                 stop after the exam; keep the joined branch locally
  --keep                  keep the sandboxes after the run

At the plan: y starts, n stops, r asks for a new plan, any other sentence adds a check of your own.
During the build: e + Enter runs the exam now with what the agents have so far.

Exit codes: 0 done, 1 error or stopped, 2 usage, 3 some exam checks failed.
Every environment variable (TASKWING_*) and the agent commands: docs/guide.md
`)
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func home() string {
	if h := os.Getenv("TASKWING_HOME"); h != "" {
		return h
	}
	h, err := os.UserHomeDir()
	if err != nil {
		return ".taskwing"
	}
	return filepath.Join(h, ".taskwing")
}

type providerFlags struct {
	provider, base, boatEnv, agent, model, effort *string
}

func addProviderFlags(fs *flag.FlagSet) providerFlags {
	return providerFlags{
		provider: fs.String("provider", env("TASKWING_PROVIDER", "boat"), "boat or local"),
		base:     fs.String("boat-base", env("TASKWING_BOAT_BASE", loadConfig().BoatBase), "boat box to fork"),
		boatEnv:  fs.String("boat-env", os.Getenv("TASKWING_BOAT_ENV"), "boat environment"),
		agent:    fs.String("agent", env("TASKWING_AGENT", box.DefaultAgent), "coding agent in each box"),
		model:    fs.String("model", os.Getenv("TASKWING_MODEL"), "agent model"),
		effort:   fs.String("effort", os.Getenv("TASKWING_EFFORT"), "agent effort"),
	}
}

func (p providerFlags) build() (box.Provider, error) {
	agent := strings.ToLower(strings.TrimSpace(*p.agent))
	if !box.KnownAgent(agent) {
		return nil, errUsage{fmt.Sprintf("unknown agent %q (use one of: %s)", *p.agent, strings.Join(box.Agents, ", "))}
	}
	switch *p.provider {
	case "boat":
		return &box.Boat{Bin: env("TASKWING_BOAT_BIN", "boat"), Base: *p.base, Environment: *p.boatEnv, Agent: agent, Model: *p.model, Effort: *p.effort}, nil
	case "local":
		// Local agents are a shell command on this machine, not a boat agent.
		if agent != box.DefaultAgent && os.Getenv("TASKWING_LOCAL_AGENT") == "" {
			return nil, errUsage{fmt.Sprintf("agent %s runs only in boat; for local folders set TASKWING_LOCAL_AGENT to the %s command (prompt on stdin, see docs/guide.md)", agent, agent)}
		}
		return &box.Local{Root: filepath.Join(home(), "boxes"), AgentCmd: env("TASKWING_LOCAL_AGENT", defaultLocal)}, nil
	}
	return nil, errUsage{fmt.Sprintf("unknown provider %q (use boat or local)", *p.provider)}
}

// parseAnywhere lets flags come before or after the positional words.
// Everything after "--" is words, even if it looks like a flag.
func parseAnywhere(fs *flag.FlagSet, args []string) ([]string, error) {
	fs.SetOutput(io.Discard) // errors are printed once, by main
	var rest []string
	for i, a := range args {
		if a == "--" {
			args, rest = args[:i], args[i+1:]
			break
		}
	}
	var words []string
	for {
		if err := fs.Parse(args); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return nil, err
			}
			if name, ok := strings.CutPrefix(err.Error(), "flag provided but not defined: -"); ok {
				return nil, errUsage{"unknown flag --" + strings.TrimPrefix(name, "-") + " (taskwing help lists them)"}
			}
			return nil, errUsage{err.Error()}
		}
		if fs.NArg() == 0 {
			return append(words, rest...), nil
		}
		words = append(words, fs.Arg(0))
		args = fs.Args()[1:]
	}
}

func signalContext() (context.Context, func()) {
	ctx, cancel := context.WithCancel(context.Background())
	sig := make(chan os.Signal, 1)
	// SIGHUP too: agents run in process groups of their own, so a closed
	// terminal no longer reaches them; only this cleanup stops them.
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	done := make(chan struct{})
	go func() {
		select {
		case <-sig:
			fmt.Fprintln(os.Stderr, "\nStopping: cleaning up sandboxes. Press Ctrl-C again to quit at once.")
			signal.Stop(sig) // a second Ctrl-C kills the process
			cancel()
		case <-done:
		}
	}()
	return ctx, func() { close(done); signal.Stop(sig); cancel() }
}

var errStopped = errors.New("stopped; every sandbox was cleaned up")

func cmdRun(args []string, hunt bool) error {
	name := "run"
	if hunt {
		name = "hunt"
	}
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	pf := addProviderFlags(fs)
	maxCards := fs.Int("max-cards", 4, "most pieces")
	yes := fs.Bool("yes", false, "do not ask")
	noPR := fs.Bool("no-pr", false, "no pull request")
	keep := fs.Bool("keep", false, "keep sandboxes")
	words, err := parseAnywhere(fs, args)
	if err != nil {
		return err
	}
	task := strings.TrimSpace(strings.Join(words, " "))
	if task == "" && !hunt {
		return errUsage{`give the task in quotes: taskwing run "add a retry schedule to failed payments"`}
	}
	prov, err := pf.build()
	if err != nil {
		return err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	ctx, stop := signalContext()
	defer stop()
	res, err := engine.Run(ctx, engine.Options{
		Task:     task,
		Dir:      cwd,
		Provider: prov,
		Home:     home(),
		LeadCmd:  env("TASKWING_LEAD_CMD", defaultLead),
		ExamCmd:  env("TASKWING_EXAM_CMD", defaultExam),
		HuntCmd:  env("TASKWING_HUNT_CMD", env("TASKWING_EXAM_CMD", defaultExam)),
		Hunt:     hunt,
		MaxCards: *maxCards,
		Yes:      *yes,
		NoPR:     *noPR,
		Keep:     *keep,
		Out:      os.Stdout,
		In:       os.Stdin,
	})
	if errors.Is(err, context.Canceled) {
		err = errStopped
	}
	if err != nil {
		if res != nil && res.RunDir != "" {
			return fmt.Errorf("%v\n  run log: %s", err, res.RunDir)
		}
		return err
	}
	if res.Total > 0 && res.Passed < res.Total {
		return errChecksFailed
	}
	return nil
}

// cmdEngine serves the JSON lines protocol (docs/engine-protocol.md). The
// client picks the repository, the sandbox and the models for each run; the
// commands and boat settings come from the same environment and config as `run`.
func cmdEngine(args []string) error {
	fs := flag.NewFlagSet("engine", flag.ContinueOnError)
	jsonOut := fs.Bool("json", false, "JSON lines on stdin and stdout")
	maxCards := fs.Int("max-cards", 4, "most pieces")
	keep := fs.Bool("keep", false, "keep sandboxes")
	if _, err := parseAnywhere(fs, args); err != nil {
		return err
	}
	if !*jsonOut {
		return errors.New("the engine only speaks JSON lines for now: taskwing engine --json")
	}
	// The client closes stdin to stop; Ctrl-C works too when run by hand.
	ctx, stop := signalContext()
	defer stop()
	h := home()
	start := func(ctx context.Context, c engine.Command) (engine.Options, error) {
		if strings.TrimSpace(c.Task) == "" {
			return engine.Options{}, errors.New("no task given")
		}
		// Canonical values are lower case; the desktop app's labels
		// ("Claude", "Local folders") are still accepted.
		// models names the coding agent in each box (boat --provider).
		agent := strings.ToLower(strings.TrimSpace(c.Models))
		if agent == "" {
			agent = strings.ToLower(env("TASKWING_AGENT", box.DefaultAgent))
		}
		if !box.KnownAgent(agent) {
			return engine.Options{}, fmt.Errorf("%s is not ready yet: use one agent for every box, one of %s", c.Models, strings.Join(box.Agents, ", "))
		}
		provider := "boat"
		switch strings.ToLower(c.Sandbox) {
		case "", "boat":
		case "local", "local folders":
			provider = "local"
		default:
			return engine.Options{}, fmt.Errorf("unknown sandbox %q (use boat or local)", c.Sandbox)
		}
		// The boat settings `run` would use, from the environment and config.
		pf := addProviderFlags(flag.NewFlagSet("engine run", flag.ContinueOnError))
		*pf.provider = provider
		*pf.agent = agent
		prov, err := pf.build()
		if err != nil {
			return engine.Options{}, err
		}
		dir, err := engine.ResolveRepo(ctx, h, c.Repo)
		if err != nil {
			return engine.Options{}, err
		}
		return engine.Options{
			Task:     strings.TrimSpace(c.Task),
			Dir:      dir,
			Provider: prov,
			Home:     h,
			LeadCmd:  env("TASKWING_LEAD_CMD", defaultLead),
			ExamCmd:  env("TASKWING_EXAM_CMD", defaultExam),
			MaxCards: *maxCards,
			Keep:     *keep,
			Out:      os.Stderr,
		}, nil
	}
	if err := engine.Serve(ctx, os.Stdin, os.Stdout, versionString(), start); err != nil {
		return err
	}
	if ctx.Err() != nil {
		return errStopped // Ctrl-C, not the client closing stdin
	}
	return nil
}

func cmdDoctor(args []string) error {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	pf := addProviderFlags(fs)
	clean := fs.Bool("clean", false, "delete what is found")
	if _, err := parseAnywhere(fs, args); err != nil {
		return err
	}
	// Doctor starts no agent: an agent setting it cannot use (a typo in
	// TASKWING_AGENT, or codex with --provider local) must not stop the cleanup.
	*pf.agent = box.DefaultAgent
	prov, err := pf.build()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	all, err := prov.List(ctx, engine.RunBoxPrefix)
	var boxes []box.Box
	for _, b := range all {
		// Never a base box: the flag's, the environment's or the saved one,
		// since --boat-base can name a different box than the others.
		if b.ID != *pf.base && b.ID != os.Getenv("TASKWING_BOAT_BASE") && b.ID != loadConfig().BoatBase {
			boxes = append(boxes, b)
		}
	}
	var missing *exec.Error
	if errors.As(err, &missing) {
		fmt.Printf("%s: not installed, nothing to check\n", prov.Name())
	} else if err != nil {
		fmt.Printf("%s sandboxes: could not list (%v)\n", prov.Name(), err)
	} else if len(boxes) == 0 {
		fmt.Printf("%s sandboxes: none left over\n", prov.Name())
	} else {
		fmt.Printf("%s sandboxes left over: %d\n", prov.Name(), len(boxes))
		for _, b := range boxes {
			line := "  " + b.ID + "  " + b.Name
			if *clean {
				if err := prov.Destroy(ctx, &b); err != nil {
					line += "  (delete failed: " + err.Error() + ")"
				} else {
					line += "  deleted"
				}
			}
			fmt.Println(line)
		}
	}

	if out, err := exec.CommandContext(ctx, "git", "rev-parse", "--show-toplevel").Output(); err == nil {
		root := strings.TrimSpace(string(out))
		_ = exec.CommandContext(ctx, "git", "-C", root, "worktree", "prune").Run()
		refs, _ := exec.CommandContext(ctx, "git", "-C", root, "for-each-ref", "--format=%(refname)", engine.AgentRefPrefix).Output()
		branches, _ := exec.CommandContext(ctx, "git", "-C", root, "for-each-ref", "--format=%(refname:short)", "refs/heads/"+engine.BranchPrefix).Output()
		list := strings.Fields(string(refs))
		fmt.Printf("agent refs in this repository: %d\n", len(list))
		for _, r := range list {
			line := "  " + r
			if *clean {
				if err := exec.CommandContext(ctx, "git", "-C", root, "update-ref", "-d", r).Run(); err == nil {
					line += "  deleted"
				}
			}
			fmt.Println(line)
		}
		if bs := strings.Fields(string(branches)); len(bs) > 0 {
			fmt.Printf("joined branches (kept; delete with git branch -D): %s\n", strings.Join(bs, ", "))
		}
	}
	if !*clean {
		fmt.Println("Run with --clean to delete the sandboxes and agent refs above.")
	}
	return nil
}

// config is ~/.taskwing/config.json: settings the user would otherwise export.
type config struct {
	BoatBase string `json:"boat_base,omitempty"`
}

func configPath() string { return filepath.Join(home(), "config.json") }

// loadConfig reads the config file once per process. A broken file is
// reported, not silently read as empty (the boat base would vanish).
var loadConfig = sync.OnceValue(func() config {
	var c config
	if b, err := os.ReadFile(configPath()); err == nil {
		if err := json.Unmarshal(b, &c); err != nil {
			fmt.Fprintf(os.Stderr, "taskwing: ignoring %s: %v\n", configPath(), err)
		}
	}
	return c
})

func cmdConfig(args []string) error {
	c := loadConfig()
	switch {
	case len(args) == 0:
		fmt.Printf("boat-base  %s\n  (%s)\n", c.BoatBase, configPath())
		return nil
	case len(args) == 2 && args[0] == "boat-base":
		c.BoatBase = strings.TrimSpace(args[1])
	default:
		return errUsage{"usage: taskwing config [boat-base <id>]"}
	}
	if err := os.MkdirAll(home(), 0o700); err != nil {
		return err
	}
	b, _ := json.MarshalIndent(c, "", "  ")
	if err := os.WriteFile(configPath(), append(b, '\n'), 0o600); err != nil {
		return err
	}
	fmt.Println("saved:", configPath())
	return nil
}
