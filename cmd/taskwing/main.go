// Command taskwing runs one task with several coding agents in sandboxes and
// opens one draft pull request.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/josephgoksu/TaskWing/internal/box"
	"github.com/josephgoksu/TaskWing/internal/engine"
)

var version = "v2-poc"

const (
	defaultLead  = `claude -p --output-format text --allowedTools "Read,Glob,Grep,Bash(git log:*),Bash(git ls-files:*)"`
	defaultExam  = `claude -p --output-format text --allowedTools "Read,Glob,Grep,Write,Edit,Bash"`
	defaultLocal = `claude -p --dangerously-skip-permissions`
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "run":
		err = cmdRun(os.Args[2:])
	case "doctor":
		err = cmdDoctor(os.Args[2:])
	case "version", "--version", "-v":
		fmt.Println("taskwing", version)
	case "help", "--help", "-h":
		usage()
	default:
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "taskwing:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `taskwing runs one task with several agents in sandboxes and opens one draft pull request.

  taskwing run "<task>" [flags]    plan, exam, build, join, exam, pull request
  taskwing doctor [--clean]        find (and delete) leftover sandboxes and refs
  taskwing version

Run flags:
  --provider boat|local   where agents run (TASKWING_PROVIDER, default boat)
  --boat-base <id>        prepared boat box to fork (TASKWING_BOAT_BASE)
  --boat-env <name>       boat environment for the forks (TASKWING_BOAT_ENV)
  --model <name>          agent model in boat (TASKWING_MODEL, default opus-5-5)
  --effort <level>        agent effort in boat (TASKWING_EFFORT, default medium)
  --max-cards <n>         most pieces the lead may make (default 4)
  --yes                   do not ask before starting or before the PR
  --no-pr                 stop after the exam; keep the branch locally
  --keep                  keep the sandboxes after the run

Commands (prompt on stdin):
  TASKWING_LEAD_CMD    the lead agent      default: `+defaultLead+`
  TASKWING_EXAM_CMD    the exam writer     default: `+defaultExam+`
  TASKWING_LOCAL_AGENT agents in --provider local  default: `+defaultLocal+`
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
	provider, base, boatEnv, model, effort *string
}

func addProviderFlags(fs *flag.FlagSet) providerFlags {
	return providerFlags{
		provider: fs.String("provider", env("TASKWING_PROVIDER", "boat"), "boat or local"),
		base:     fs.String("boat-base", os.Getenv("TASKWING_BOAT_BASE"), "boat box to fork"),
		boatEnv:  fs.String("boat-env", os.Getenv("TASKWING_BOAT_ENV"), "boat environment"),
		model:    fs.String("model", env("TASKWING_MODEL", "opus-5-5"), "agent model"),
		effort:   fs.String("effort", env("TASKWING_EFFORT", "medium"), "agent effort"),
	}
}

func (p providerFlags) build() (box.Provider, error) {
	switch *p.provider {
	case "boat":
		return &box.Boat{Bin: env("TASKWING_BOAT_BIN", "boat"), Base: *p.base, Environment: *p.boatEnv, Model: *p.model, Effort: *p.effort}, nil
	case "local":
		return &box.Local{Root: filepath.Join(home(), "boxes"), AgentCmd: env("TASKWING_LOCAL_AGENT", defaultLocal)}, nil
	}
	return nil, fmt.Errorf("unknown provider %q (use boat or local)", *p.provider)
}

// parseAnywhere lets flags come before or after the positional words.
func parseAnywhere(fs *flag.FlagSet, args []string) ([]string, error) {
	var words []string
	for {
		if err := fs.Parse(args); err != nil {
			return nil, err
		}
		if fs.NArg() == 0 {
			return words, nil
		}
		words = append(words, fs.Arg(0))
		args = fs.Args()[1:]
	}
}

func signalContext() (context.Context, func()) {
	ctx, cancel := context.WithCancel(context.Background())
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
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

func cmdRun(args []string) error {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
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
	if task == "" {
		return fmt.Errorf(`give the task in quotes: taskwing run "add a retry schedule to failed payments"`)
	}
	prov, err := pf.build()
	if err != nil {
		return err
	}
	cwd, _ := os.Getwd()
	ctx, stop := signalContext()
	defer stop()
	res, err := engine.Run(ctx, engine.Options{
		Task:     task,
		Dir:      cwd,
		Provider: prov,
		Home:     home(),
		LeadCmd:  env("TASKWING_LEAD_CMD", defaultLead),
		ExamCmd:  env("TASKWING_EXAM_CMD", defaultExam),
		MaxCards: *maxCards,
		Yes:      *yes,
		NoPR:     *noPR,
		Keep:     *keep,
		Out:      os.Stdout,
		In:       os.Stdin,
	})
	if errors.Is(err, context.Canceled) {
		err = errors.New("stopped; every sandbox was cleaned up")
	}
	if err != nil {
		if res != nil && res.RunDir != "" {
			return fmt.Errorf("%v\n  run log: %s", err, res.RunDir)
		}
		return err
	}
	if res.Total > 0 && res.Passed < res.Total {
		os.Exit(3)
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
	prov, err := pf.build()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	all, err := prov.List(ctx, engine.RunBoxPrefix)
	var boxes []box.Box
	for _, b := range all {
		if b.ID != os.Getenv("TASKWING_BOAT_BASE") && b.ID != *pf.base { // never the base box
			boxes = append(boxes, b)
		}
	}
	if err != nil {
		fmt.Printf("%s sandboxes: could not list (%v)\n", prov.Name(), err)
	} else if len(boxes) == 0 {
		fmt.Printf("%s sandboxes: none left over\n", prov.Name())
	} else {
		fmt.Printf("%s sandboxes left over: %d\n", prov.Name(), len(boxes))
		for _, b := range boxes {
			b := b
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

	if out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output(); err == nil {
		root := strings.TrimSpace(string(out))
		_ = exec.Command("git", "-C", root, "worktree", "prune").Run()
		refs, _ := exec.Command("git", "-C", root, "for-each-ref", "--format=%(refname)", "refs/taskwing/").Output()
		branches, _ := exec.Command("git", "-C", root, "for-each-ref", "--format=%(refname:short)", "refs/heads/taskwing/").Output()
		list := strings.Fields(string(refs))
		fmt.Printf("agent refs in this repository: %d\n", len(list))
		for _, r := range list {
			line := "  " + r
			if *clean {
				if err := exec.Command("git", "-C", root, "update-ref", "-d", r).Run(); err == nil {
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
