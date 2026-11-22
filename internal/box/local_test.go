//go:build !windows

package box

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// alive reports whether pid is a live process (a zombie counts as gone).
func alive(pid int) bool {
	if syscall.Kill(pid, 0) != nil {
		return false
	}
	stat, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return true // no /proc (macOS): trust kill
	}
	// The state follows the parenthesised command name.
	s := string(stat)
	if i := strings.LastIndexByte(s, ')'); i >= 0 && len(s) > i+2 {
		return s[i+2] != 'Z'
	}
	return true
}

// waitPid reads the pid the script wrote to path.
func waitPid(t *testing.T, path string) int {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		data, _ := os.ReadFile(path)
		if pid, err := strconv.Atoi(strings.TrimSpace(string(data))); err == nil {
			return pid
		}
	}
	t.Fatal("the script never wrote its child pid")
	return 0
}

func waitGone(t *testing.T, pid int) {
	t.Helper()
	for deadline := time.Now().Add(2 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		if !alive(pid) {
			return
		}
	}
	_ = syscall.Kill(pid, syscall.SIGKILL)
	t.Fatalf("child %d still running", pid)
}

func TestLocalStopKillsProcessTree(t *testing.T) {
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "pid")
	l := &Local{Root: dir, AgentCmd: "sleep 30 & echo $! > " + pidFile + "; wait"}
	b, err := l.Create(context.Background(), "t")
	if err != nil {
		t.Fatal(err)
	}
	b.RepoDir = b.dir
	a, err := l.StartAgent(context.Background(), b, "go", filepath.Join(dir, "log"))
	if err != nil {
		t.Fatal(err)
	}
	pid := waitPid(t, pidFile)
	a.Stop()
	waitGone(t, pid)
	if a.Status().Running {
		t.Fatal("agent still running after Stop")
	}
}

func TestLocalStartAgentStopsWithContext(t *testing.T) {
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "pid")
	l := &Local{Root: dir, AgentCmd: "sleep 30 & echo $! > " + pidFile + "; wait"}
	b, _ := l.Create(context.Background(), "t")
	b.RepoDir = b.dir
	ctx, cancel := context.WithCancel(context.Background())
	if _, err := l.StartAgent(ctx, b, "go", filepath.Join(dir, "log")); err != nil {
		t.Fatal(err)
	}
	pid := waitPid(t, pidFile)
	if !alive(pid) {
		t.Fatal("agent died when StartAgent returned")
	}
	cancel()
	waitGone(t, pid)
}

func TestLocalExecTimeoutKillsTree(t *testing.T) {
	dir := t.TempDir()
	l := &Local{Root: dir}
	b, _ := l.Create(context.Background(), "t")
	pidFile := filepath.Join(dir, "pid")
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	start := time.Now()
	rc, err := l.Exec(ctx, b, "sleep 30 & echo $! > "+pidFile+"; wait", nil, nil)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("got rc=%d err=%v, want a deadline error", rc, err)
	}
	if took := time.Since(start); took > 3*time.Second {
		t.Fatalf("Exec took %s after the timeout", took)
	}
	waitGone(t, waitPid(t, pidFile))
}

func TestLocalExecExitCode(t *testing.T) {
	l := &Local{Root: t.TempDir()}
	b, _ := l.Create(context.Background(), "t")
	if rc, err := l.Exec(context.Background(), b, "exit 7", nil, nil); rc != 7 || err != nil {
		t.Fatalf("got %d, %v", rc, err)
	}
}
