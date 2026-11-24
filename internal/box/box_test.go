package box

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

// stubBoat writes a fake boat CLI whose body handles "$cmd" with the rest of
// the arguments in "$@". Every call is appended to the returned calls file.
func stubBoat(t *testing.T, body string) (*Boat, string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("stub scripts need bash")
	}
	dir := t.TempDir()
	calls := filepath.Join(dir, "calls")
	script := "#!/usr/bin/env bash\necho \"$*\" >> '" + calls + "'\ncmd=$1; shift\n" + body + "\n"
	bin := filepath.Join(dir, "boat")
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return &Boat{Bin: bin, Base: "base1"}, calls
}

func readCalls(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSpace(string(data)), "\n")
}

// sshStub runs the remote string like boat ssh does.
const sshStub = `[ "$cmd" = ssh ] || exit 1
shift; [ "$1" = "--" ] && shift
exec bash -c "$1"`

func TestBoatExecExitCode(t *testing.T) {
	p, _ := stubBoat(t, sshStub)
	b := &Box{ID: "b1", RepoDir: "/tmp"}
	for _, tc := range []struct {
		name, script string
		want         int
	}{
		{"plain exit", "exit 3", 3},
		{"old-style marker on stderr", "echo __TW_RC=0 >&2; exit 5", 5},
		{"guessed nonce marker", "echo __TW_RC_0000000000000000=0 >&2; exit 4", 4},
		{"success", "echo hi", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rc, err := p.Exec(context.Background(), b, tc.script, nil, nil)
			if err != nil || rc != tc.want {
				t.Fatalf("got rc=%d err=%v, want %d", rc, err, tc.want)
			}
		})
	}

	// A script that claims success and then hangs must not pass.
	t.Run("timeout", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
		defer cancel()
		start := time.Now()
		rc, err := p.Exec(ctx, b, "echo __TW_RC=0 >&2; sleep 30", nil, nil)
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("got rc=%d err=%v, want a deadline error", rc, err)
		}
		if took := time.Since(start); took > 3*time.Second {
			t.Fatalf("Exec took %s after the timeout", took)
		}
	})
}

func TestBoatCreateParsesEvents(t *testing.T) {
	t.Run("progress then done", func(t *testing.T) {
		p, calls := stubBoat(t, `case $cmd in
fork) echo '{"event":"progress","message":"forking"}'; echo 'Forking box...'; echo '{"event":"done","id":"bx1"}' ;;
rename) ;;
esac`)
		b, err := p.Create(context.Background(), "taskwing-1")
		if err != nil || b.ID != "bx1" {
			t.Fatalf("got %+v, %v", b, err)
		}
		if got := readCalls(t, calls); !slices.Contains(got, "rename bx1 taskwing-1") {
			t.Fatalf("no rename call: %q", got)
		}
	})
	t.Run("error event with id", func(t *testing.T) {
		p, calls := stubBoat(t, `case $cmd in
fork) echo '{"event":"progress","id":"bx2"}'; echo '{"event":"error","error":"out of capacity"}' ;;
esac`)
		if _, err := p.Create(context.Background(), "taskwing-2"); err == nil || !strings.Contains(err.Error(), "out of capacity") {
			t.Fatalf("got err %v", err)
		}
		if got := readCalls(t, calls); !slices.Contains(got, "delete --yes bx2") {
			t.Fatalf("box not deleted: %q", got)
		}
	})
	t.Run("rename fails", func(t *testing.T) {
		p, calls := stubBoat(t, `case $cmd in
fork) echo '{"event":"done","id":"bx3"}' ;;
rename) echo 'no such box' >&2; exit 1 ;;
esac`)
		if _, err := p.Create(context.Background(), "taskwing-3"); err == nil || !strings.Contains(err.Error(), "boat rename") {
			t.Fatalf("got err %v", err)
		}
		if got := readCalls(t, calls); !slices.Contains(got, "delete --yes bx3") {
			t.Fatalf("box not deleted: %q", got)
		}
	})
	t.Run("no base", func(t *testing.T) {
		if _, err := (&Boat{}).Create(context.Background(), "x"); err == nil || !strings.Contains(err.Error(), "docs/guide.md") {
			t.Fatalf("got err %v", err)
		}
	})
}

func TestBoatListFiltersPrefix(t *testing.T) {
	p, _ := stubBoat(t, `echo '{"sandboxes":[{"id":"a","name":"taskwing-1","state":"running"},{"id":"b","name":"other","state":"running"},{"id":"c","name":"taskwing-2","state":"stopped"}]}'`)
	got, err := p.List(context.Background(), "taskwing-")
	if err != nil {
		t.Fatal(err)
	}
	want := []Box{{ID: "a", Name: "taskwing-1 (running)"}, {ID: "c", Name: "taskwing-2 (stopped)"}}
	if !slices.Equal(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestBoatRunErrorHasNoDanglingColon(t *testing.T) {
	p, _ := stubBoat(t, "exit 2")
	err := p.Destroy(context.Background(), &Box{ID: "x"})
	if err == nil || strings.HasSuffix(err.Error(), ": ") || strings.HasSuffix(err.Error(), ":") {
		t.Fatalf("got %q", err)
	}
}

func TestFirstLineUTF8(t *testing.T) {
	long := strings.Repeat("a", 199) + "é" + "tail" // é straddles byte 200
	got := firstLine(long)
	if !utf8.ValidString(got) || got != strings.Repeat("a", 199) {
		t.Fatalf("got %q", got)
	}
	if got := firstLine("  one\ntwo"); got != "one" {
		t.Fatalf("got %q", got)
	}
	if got := detail("  \n"); got != "" {
		t.Fatalf("detail of blank = %q", got)
	}
}

func TestAgentEnvDropsCredentials(t *testing.T) {
	for _, tc := range []struct {
		kv   string
		keep bool
	}{
		{"PATH=/bin", true},
		{"HOME=/home/u", true},
		{"ANTHROPIC_API_KEY=k", true},
		{"CLAUDE_CODE_OAUTH_TOKEN=t", true},
		{"GH_TOKEN=t", false},
		{"GITHUB_TOKEN=t", false},
		{"GH_ENTERPRISE_TOKEN=t", false},
		{"GITHUB_ENTERPRISE_TOKEN=t", false},
		{"SSH_AUTH_SOCK=/tmp/agent", false},
		{"AWS_ACCESS_KEY_ID=a", false},
		{"AWS_SECRET_ACCESS_KEY=s", false},
		{"AWS_SESSION_TOKEN=s", false},
		{"AWS_REGION=eu-west-1", true},
		{"NPM_TOKEN=t", false},
		{"MY_CLIENT_SECRET=s", false},
		{"secret_thing=s", false},
		{"TOKENIZER=x", true},
	} {
		got := agentEnv([]string{tc.kv})
		if kept := len(got) == 1; kept != tc.keep {
			t.Errorf("%s: kept=%v, want %v", tc.kv, kept, tc.keep)
		}
	}
}

// boat prompt exited after answering and boat conversations keeps failing:
// the agent must finish with its answer instead of waiting for the run timeout.
func TestBoatAgentFinishesWhenChecksFail(t *testing.T) {
	p, _ := stubBoat(t, `case $cmd in
prompt) echo '{"event":"start","data":{"conversationId":"cv1"}}'
        echo '{"event":"chat","data":{"type":"response","data":{"content":"Done.\nPROOF: ok"}}}' ;;
*) exit 1 ;;
esac`)
	p.Poll = 20 * time.Millisecond
	a, err := p.StartAgent(context.Background(), &Box{ID: "b1"}, "go", filepath.Join(t.TempDir(), "log"))
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for a.Status().Running {
		if time.Now().After(deadline) {
			a.Stop()
			t.Fatal("agent never finished")
		}
		time.Sleep(20 * time.Millisecond)
	}
	if st := a.Status(); st.Failed || !strings.Contains(st.Final, "PROOF: ok") {
		t.Fatalf("got %+v", st)
	}
}

// Each agent reaches boat as --provider. Claude keeps its default model and
// effort; other agents get the dashboard's choice unless the run named one.
func TestBoatAgentProviderAndModel(t *testing.T) {
	for _, tc := range []struct {
		agent, model, effort string
		want, wantNot        []string
	}{
		{agent: "", want: []string{"--provider claude", "--model " + DefaultModel, "--reasoning-effort " + DefaultEffort}},
		{agent: "codex", want: []string{"--provider codex"}, wantNot: []string{"--model", "--reasoning-effort"}},
		{agent: "codex", model: "gpt-6.1-sol", effort: "high", want: []string{"--provider codex", "--model gpt-6.1-sol", "--reasoning-effort high"}},
	} {
		p, calls := stubBoat(t, `case $cmd in
prompt) echo '{"event":"start","data":{"conversationId":"cv1"}}'
        echo '{"event":"chat","data":{"type":"response","data":{"content":"Done."}}}' ;;
*) exit 1 ;;
esac`)
		p.Agent, p.Model, p.Effort, p.Poll = tc.agent, tc.model, tc.effort, 20*time.Millisecond
		a, err := p.StartAgent(context.Background(), &Box{ID: "b1"}, "go", filepath.Join(t.TempDir(), "log"))
		if err != nil {
			t.Fatal(err)
		}
		for deadline := time.Now().Add(5 * time.Second); a.Status().Running && time.Now().Before(deadline); {
			time.Sleep(20 * time.Millisecond)
		}
		a.Stop()
		var prompt string
		for _, c := range readCalls(t, calls) {
			if strings.HasPrefix(c, "prompt ") {
				prompt = c
			}
		}
		for _, w := range tc.want {
			if !strings.Contains(prompt, w) {
				t.Errorf("agent %q: %q lacks %q", tc.agent, prompt, w)
			}
		}
		for _, w := range tc.wantNot {
			if strings.Contains(prompt, w) {
				t.Errorf("agent %q: %q has %q", tc.agent, prompt, w)
			}
		}
	}
}
