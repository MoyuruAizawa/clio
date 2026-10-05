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
	for _, args := range [][]string{nil, {"exec"}, {"exec", "--"}, {"exec", "--context"}, {"exec", "--context", "--", "true"}, {"exec", "--mode", "on-error", "--", "true"}, {"exec", "--mode", "status", "--", "true"}, {"exec", "--mode", "full", "--", "true"}, {"exec", "--timeout", "-1s", "--", "true"}, {"exec", "--timeout", "nonsense", "--", "true"}, {"exec", "extra", "--", "true"}, {"auth"}} {
		var out, errout bytes.Buffer
		if code := run(context.Background(), args, os.Stdin, &out, &errout, neverFilter{t}); code != 2 {
			t.Fatal(args, code)
		}
	}
}

func TestCLIAlwaysOnErrorAndPreservesChildModeArgs(t *testing.T) {
	var out, errout bytes.Buffer
	code := run(context.Background(), []string{"exec", "--context", "Verify recent changes", "--", "/bin/sh", "-c", `test "$1" = --mode && test "$2" = full`, "sh", "--mode", "full"}, os.Stdin, &out, &errout, neverFilter{t})
	if code != 0 {
		t.Fatal(code, errout.String())
	}
	if out.Len() != 0 {
		t.Fatalf("successful child output should be suppressed, got %q", out.String())
	}
	if !bytes.Contains([]byte(errout.String()), []byte("exit=0")) {
		t.Fatal(errout.String())
	}
}
