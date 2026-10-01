package render

import (
	"clio/internal/executor"
	"fmt"
	"io"
)

func Metadata(w io.Writer, r executor.Result) error {
	_, err := fmt.Fprintf(w, "clio: exit=%d duration=%s", r.ExitCode, r.Duration)
	if err != nil {
		return err
	}
	if r.Signal != "" {
		if _, err = fmt.Fprintf(w, " signal=%s", r.Signal); err != nil {
			return err
		}
	}
	_, err = fmt.Fprintf(w, " timeout=%t cancelled=%t\n", r.TimedOut, r.Cancelled)
	return err
}
