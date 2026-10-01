package execution

import (
	"clio/internal/executor"
	"clio/internal/filter"
	"clio/internal/render"
	"context"
	"fmt"
	"io"
	"time"
)

func ValidMode(mode string) bool { return mode == "status" || mode == "full" || mode == "on-error" }
func Tail(b []byte) []byte {
	if len(b) > 4096 {
		return b[len(b)-4096:]
	}
	return b
}
func Execute(ctx context.Context, args []string, mode, agentContext string, stdin io.Reader, out, errout io.Writer, f filter.Filter) (int, error) {
	r, err := executor.Run(ctx, args, stdin)
	if err != nil {
		return 1, err
	}
	if mode == "full" {
		if _, err = out.Write(r.Stdout); err != nil {
			return 1, err
		}
		if _, err = errout.Write(r.Stderr); err != nil {
			return 1, err
		}
	}
	if mode == "on-error" && r.ExitCode != 0 {
		fc, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		result, fe := f.Apply(fc, filter.FilterInput{Command: args, AgentContext: agentContext, ExitCode: r.ExitCode, Stdout: r.Stdout, Stderr: r.Stderr})
		cancel()
		if fe == nil && len(result.Content) == 0 && len(r.Stdout)+len(r.Stderr) > 0 {
			fe = fmt.Errorf("no log ranges selected")
		}
		if fe != nil {
			if _, err = fmt.Fprintf(errout, "clio: filtering failed: %s\n", fe); err != nil {
				return 1, err
			}
			if _, err = out.Write(Tail(r.Stdout)); err != nil {
				return 1, err
			}
			if _, err = errout.Write(Tail(r.Stderr)); err != nil {
				return 1, err
			}
		} else if _, err = out.Write(result.Content); err != nil {
			return 1, err
		}
	}
	if err = render.Metadata(errout, r); err != nil {
		return 1, err
	}
	return r.ExitCode, nil
}
