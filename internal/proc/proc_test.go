//go:build !windows

package proc

import (
	"context"
	"os/exec"
	"testing"
	"time"
)

// A pipe keeps bash's child alive; without the group kill, Run would block
// until sleep ends.
func TestGroupStopsTheWholeTree(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	cmd := exec.CommandContext(ctx, "bash", "-c", "sleep 20 | cat")
	var out []byte
	cmd.Stdout = &sink{b: &out}
	Group(cmd)
	start := time.Now()
	_ = cmd.Run()
	if took := time.Since(start); took > 3*time.Second {
		t.Fatalf("Run took %s after the timeout; the child kept the pipe open", took)
	}
}

func TestKillBeforeStart(t *testing.T) {
	if err := Kill(exec.Command("true")); err != nil {
		t.Fatal(err)
	}
}

type sink struct{ b *[]byte }

func (s *sink) Write(p []byte) (int, error) { *s.b = append(*s.b, p...); return len(p), nil }
