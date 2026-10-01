package main

import (
	"bytes"
	"clio/internal/filter"
	"context"
	"os"
	"testing"
)

type neverFilter struct{ t *testing.T }

func (f neverFilter) Apply(context.Context, filter.FilterInput) (filter.FilterOutput, error) {
	f.t.Fatal("unexpected filter")
	return filter.FilterOutput{}, nil
}
func TestArguments(t *testing.T) {
	for _, args := range [][]string{nil, {"exec"}, {"exec", "--"}, {"exec", "--context"}, {"exec", "--context", "--", "true"}, {"exec", "--mode", "bad", "--", "true"}, {"exec", "--timeout", "-1s", "--", "true"}, {"exec", "--timeout", "nonsense", "--", "true"}, {"exec", "extra", "--", "true"}, {"auth"}} {
		var out, errout bytes.Buffer
		if code := run(context.Background(), args, os.Stdin, &out, &errout, neverFilter{t}); code != 2 {
			t.Fatal(args, code)
		}
	}
}
func TestCLIStatusAndFull(t *testing.T) {
	for _, mode := range []string{"status", "full", "on-error"} {
		var out, errout bytes.Buffer
		code := run(context.Background(), []string{"exec", "--mode", mode, "--context", "Verify recent changes", "--", "/bin/echo", "a b"}, os.Stdin, &out, &errout, neverFilter{t})
		if code != 0 {
			t.Fatal(code, errout.String())
		}
		if mode == "full" && out.String() != "a b\n" {
			t.Fatal(out.String())
		}
		if mode != "full" && out.Len() != 0 {
			t.Fatal(out.String())
		}
	}
}
