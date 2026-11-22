package main

import (
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestParseAnywhere(t *testing.T) {
	for _, tc := range []struct {
		args  []string
		words []string
		yes   bool
		err   string
	}{
		{args: []string{"add", "retries", "--yes"}, words: []string{"add", "retries"}, yes: true},
		{args: []string{"--yes", "add", "retries"}, words: []string{"add", "retries"}, yes: true},
		{args: []string{"add", "--", "--yes", "-1"}, words: []string{"add", "--yes", "-1"}},
		{args: []string{"add", "--bogus"}, err: "unknown flag"},
	} {
		fs := flag.NewFlagSet("t", flag.ContinueOnError)
		yes := fs.Bool("yes", false, "")
		words, err := parseAnywhere(fs, tc.args)
		if tc.err != "" {
			if err == nil || !strings.Contains(err.Error(), tc.err) {
				t.Errorf("%v: err = %v, want %q", tc.args, err, tc.err)
			}
			continue
		}
		if err != nil || !reflect.DeepEqual(words, tc.words) || *yes != tc.yes {
			t.Errorf("%v: got %v yes=%v err=%v, want %v yes=%v", tc.args, words, *yes, err, tc.words, tc.yes)
		}
	}
}

func TestExitCodes(t *testing.T) {
	t.Setenv("TASKWING_HOME", t.TempDir())
	devnull, _ := os.Open(os.DevNull)
	defer devnull.Close()
	for _, tc := range []struct {
		args []string
		want int
	}{
		{nil, exitUsage},
		{[]string{"bogus"}, exitUsage},
		{[]string{"help"}, 0},
		{[]string{"run", "--help"}, 0},
		{[]string{"run", "--bogus", "x"}, exitUsage},
		{[]string{"run"}, exitUsage},
		{[]string{"run", "x", "--provider", "nope"}, exitUsage},
		{[]string{"engine"}, exitError},
		{[]string{"version"}, 0},
	} {
		stdout, stderr := os.Stdout, os.Stderr
		os.Stdout, os.Stderr = devnull, devnull
		got := run(tc.args)
		os.Stdout, os.Stderr = stdout, stderr
		if got != tc.want {
			t.Errorf("taskwing %v exited %d, want %d", tc.args, got, tc.want)
		}
	}
}

func TestBuildProvider(t *testing.T) {
	for name, want := range map[string]string{"boat": "boat", "local": "local"} {
		fs := flag.NewFlagSet("t", flag.ContinueOnError)
		pf := addProviderFlags(fs)
		*pf.provider = name
		p, err := pf.build()
		if err != nil || p.Name() != want {
			t.Errorf("%s: got %v, %v", name, p, err)
		}
	}
}

func TestHomeAndConfigFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TASKWING_HOME", dir)
	if home() != dir {
		t.Fatalf("home() = %s, want %s", home(), dir)
	}
	devnull, _ := os.Open(os.DevNull)
	defer devnull.Close()
	stdout := os.Stdout
	os.Stdout = devnull
	err := cmdConfig([]string{"boat-base", " bx_123 "})
	os.Stdout = stdout
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, "config.json"))
	if err != nil || !strings.Contains(string(b), `"boat_base": "bx_123"`) {
		t.Fatalf("config.json = %s, %v", b, err)
	}
	if st, _ := os.Stat(filepath.Join(dir, "config.json")); st.Mode().Perm() != 0o600 {
		t.Errorf("config.json mode = %v, want 0600", st.Mode().Perm())
	}
}
