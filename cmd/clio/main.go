package main

import (
	"clio/internal/credentials"
	"clio/internal/execution"
	"clio/internal/filter"
	"clio/internal/filter/jev"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
)

type lazyFilter struct{}

func (lazyFilter) Apply(ctx context.Context, in filter.FilterInput) (filter.FilterOutput, error) {
	s, e := credentials.Default()
	if e != nil {
		return filter.FilterOutput{}, errors.New("cannot locate credentials")
	}
	token, e := s.Read()
	if e != nil {
		return filter.FilterOutput{}, e
	}
	f := jev.JEVFilter{Client: &jev.HTTPClient{Token: token, Model: os.Getenv("TYPESAFE_MODEL")}}
	return f.Apply(ctx, in)
}
func run(ctx context.Context, args []string, stdin *os.File, out, errout io.Writer, f filter.Filter) int {
	if len(args) == 2 && args[0] == "auth" && args[1] == "login" {
		s, e := credentials.Default()
		if e == nil {
			e = credentials.Login(s, stdin, errout)
		}
		if e != nil {
			fmt.Fprintln(errout, "clio:", e)
			return 1
		}
		fmt.Fprintln(errout, "clio: credentials saved")
		return 0
	}
	if len(args) == 0 || args[0] != "exec" {
		fmt.Fprintln(errout, "usage: clio auth login | clio exec [--mode status|on-error|full] [--timeout duration] [--context purpose] -- command [args...]")
		return 2
	}
	boundary := -1
	for i := 1; i < len(args); i++ {
		if args[i] == "--" {
			boundary = i
			break
		}
	}
	if boundary < 0 || boundary == len(args)-1 {
		fmt.Fprintln(errout, "clio: command must follow --")
		return 2
	}
	flags := flag.NewFlagSet("exec", flag.ContinueOnError)
	flags.SetOutput(errout)
	mode := flags.String("mode", "on-error", "output mode")
	timeout := flags.Duration("timeout", 0, "child timeout")
	agentContext := flags.String("context", "", "agent's purpose for running the command")
	if flags.Parse(args[1:boundary]) != nil {
		return 2
	}
	if flags.NArg() != 0 || !execution.ValidMode(*mode) || *timeout < 0 {
		fmt.Fprintln(errout, "clio: invalid mode, timeout, or arguments")
		return 2
	}
	if *timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, *timeout)
		defer cancel()
	}
	code, e := execution.Execute(ctx, args[boundary+1:], *mode, *agentContext, stdin, out, errout, f)
	if e != nil {
		fmt.Fprintln(errout, "clio:", e)
		return 1
	}
	return code
}
func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	os.Exit(run(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr, lazyFilter{}))
}
