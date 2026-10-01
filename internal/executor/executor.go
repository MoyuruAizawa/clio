package executor

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
)

type Result struct {
	Stdout, Stderr      []byte
	ExitCode            int
	Duration            time.Duration
	Signal              string
	TimedOut, Cancelled bool
}

func Environment() []string {
	var env []string
	for _, e := range os.Environ() {
		if !strings.HasPrefix(e, "TYPESAFE_API_KEY=") {
			env = append(env, e)
		}
	}
	return env
}
func Run(ctx context.Context, args []string, stdin io.Reader) (Result, error) {
	var r Result
	if len(args) == 0 {
		return r, errors.New("missing command")
	}
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Env = Environment()
	cmd.Stdin = stdin
	var out, errout bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errout
	configure(cmd)
	cmd.WaitDelay = 200 * time.Millisecond
	start := time.Now()
	if err := cmd.Start(); err != nil {
		return r, errors.New("could not start command")
	}
	err := cmd.Wait()
	r.Duration = time.Since(start)
	r.Stdout = out.Bytes()
	r.Stderr = errout.Bytes()
	r.ExitCode = cmd.ProcessState.ExitCode()
	r.Signal, r.ExitCode = termination(cmd.ProcessState, r.ExitCode)
	r.TimedOut = errors.Is(ctx.Err(), context.DeadlineExceeded)
	r.Cancelled = errors.Is(ctx.Err(), context.Canceled)
	var exit *exec.ExitError
	if err != nil && !errors.As(err, &exit) && !errors.Is(err, exec.ErrWaitDelay) {
		return r, errors.New("command capture failed")
	}
	return r, nil
}
