//go:build darwin || linux

package executor

import (
	"context"
	"testing"
	"time"
)

func TestSignalAndInheritedPipes(t *testing.T) {
	r, e := Run(context.Background(), []string{"/bin/sh", "-c", "kill -TERM $$"}, nil)
	if e != nil || r.ExitCode != 143 || r.Signal == "" {
		t.Fatal(r, e)
	}
	start := time.Now()
	r, e = Run(context.Background(), []string{"/bin/sh", "-c", "sleep 1 & printf done"}, nil)
	if e != nil || string(r.Stdout) != "done" || time.Since(start) > 800*time.Millisecond {
		t.Fatal(r, e)
	}
}
func TestGroupCancellation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	start := time.Now()
	r, e := Run(ctx, []string{"/bin/sh", "-c", "sleep 30 & wait"}, nil)
	if e != nil || !r.TimedOut || r.ExitCode != 137 || time.Since(start) > time.Second {
		t.Fatal(r, e)
	}
}
