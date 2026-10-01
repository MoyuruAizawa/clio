package filter

import "context"

type Filter interface {
	Apply(context.Context, FilterInput) (FilterOutput, error)
}
type FilterInput struct {
	Command        []string
	AgentContext   string
	ExitCode       int
	Stdout, Stderr []byte
}
type FilterOutput struct{ Content []byte }
