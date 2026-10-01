package executor

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestHelper(t *testing.T) {
	if os.Getenv("CLIO_HELPER") != "1" {
		return
	}
	args := os.Args
	for len(args) > 0 && args[0] != "--" {
		args = args[1:]
	}
	args = args[1:]
	switch args[0] {
	case "large":
		var wg sync.WaitGroup
		for _, w := range []io.Writer{os.Stdout, os.Stderr} {
			wg.Go(func() { fmt.Fprint(w, strings.Repeat("x", 1<<20)) })
		}
		wg.Wait()
	case "args":
		fmt.Printf("%q", args[1:])
	case "input":
		io.Copy(os.Stdout, os.Stdin)
	case "env":
		fmt.Print(os.Getenv("TYPESAFE_API_KEY"))
	case "sleep":
		time.Sleep(30 * time.Second)
	case "fail":
		fmt.Fprint(os.Stdout, "out")
		fmt.Fprint(os.Stderr, "err")
		os.Exit(7)
	}
	os.Exit(0)
}
func helper(t *testing.T, args ...string) []string {
	t.Helper()
	t.Setenv("CLIO_HELPER", "1")
	return append([]string{os.Args[0], "-test.run=^TestHelper$", "--"}, args...)
}
func TestCapture(t *testing.T) {
	r, e := Run(context.Background(), helper(t, "large"), nil)
	if e != nil || r.ExitCode != 0 || len(r.Stdout) != 1<<20 || len(r.Stderr) != 1<<20 {
		t.Fatalf("capture: %v %d %d %d", e, r.ExitCode, len(r.Stdout), len(r.Stderr))
	}
}
func TestArgumentsAndInput(t *testing.T) {
	r, e := Run(context.Background(), helper(t, "args", "a b", "", "$(echo nope)", "--"), nil)
	if e != nil || string(r.Stdout) != `["a b" "" "$(echo nope)" "--"]` {
		t.Fatalf("%q %v", r.Stdout, e)
	}
	r, e = Run(context.Background(), helper(t, "input"), strings.NewReader("input"))
	if e != nil || string(r.Stdout) != "input" {
		t.Fatal(e, string(r.Stdout))
	}
}
func TestExitAndEnvironment(t *testing.T) {
	r, e := Run(context.Background(), helper(t, "fail"), nil)
	if e != nil || r.ExitCode != 7 || string(r.Stdout) != "out" || string(r.Stderr) != "err" {
		t.Fatal(r, e)
	}
	t.Setenv("TYPESAFE_API_KEY", "secret")
	r, e = Run(context.Background(), helper(t, "env"), nil)
	if e != nil || len(r.Stdout) != 0 {
		t.Fatal(r, e)
	}
}
func TestTimeoutCancellation(t *testing.T) {
	for _, cancelled := range []bool{false, true} {
		args := helper(t, "sleep")
		ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
		if cancelled {
			go func() { time.Sleep(50 * time.Millisecond); cancel() }()
		}
		r, e := Run(ctx, args, nil)
		cancel()
		if e != nil || r.ExitCode != 137 || r.Signal == "" || r.Cancelled != cancelled || r.TimedOut == cancelled {
			t.Fatal(r, e)
		}
	}
}
func TestStartFailure(t *testing.T) {
	_, e := Run(context.Background(), []string{"/nonexistent-clio-command"}, nil)
	if e == nil {
		t.Fatal("expected failure")
	}
}
