package jev

import (
	"bytes"
	"clio/internal/filter"
	"context"
	"math"
	"strings"
	"testing"
)

type fakeClient struct {
	requests []Request
	score    func(Chunk) float64
}

func (c *fakeClient) Evaluate(ctx context.Context, r Request) (map[string]float64, error) {
	c.requests = append(c.requests, r)
	scores := map[string]float64{}
	for _, ch := range r.Chunks {
		v := c.score(ch)
		scores[ch.ID] = v
		scores[beforeQuestion(ch.ID)] = v
		scores[afterQuestion(ch.ID)] = v
	}
	return scores, nil
}
func TestNormalizationRawAndContext(t *testing.T) {
	raw := []byte("\x1b[31mprogress\rfailed\x1b[0m\r\nnext\n")
	norm, _ := normalize(raw)
	if string(norm) != "progress\nfailed\nnext\n" {
		t.Fatalf("%q", norm)
	}
	c := &fakeClient{score: func(Chunk) float64 { return 0.5 }}
	f := JEVFilter{c}
	out, e := f.Apply(context.Background(), filter.FilterInput{Command: []string{"go", "test"}, ExitCode: 1, Stdout: raw})
	if e != nil || !bytes.Contains(out.Content, raw) || c.requests[0].Command[0] != "go" || c.requests[0].ExitCode != 1 {
		t.Fatal(string(out.Content), e)
	}
}
func TestChunksAndBatching(t *testing.T) {
	raw := []byte(strings.Repeat("long", 20000))
	cs := chunks(raw, "stdout")
	var reconstructed []byte
	for _, ch := range cs {
		if len(ch.Text) > 4096 {
			t.Fatal("oversized chunk")
		}
		reconstructed = append(reconstructed, raw[ch.start:ch.end]...)
	}
	if !bytes.Equal(reconstructed, raw) {
		t.Fatal("raw mapping")
	}
	c := &fakeClient{score: func(Chunk) float64 { return 1 }}
	out, e := (&JEVFilter{c}).Apply(context.Background(), filter.FilterInput{Command: []string{"build", "debug"}, AgentContext: "Verify recent changes compile", ExitCode: 1, Stdout: raw})
	if e != nil || bytes.Count(out.Content, []byte("long"))*4 != 16384 || !bytes.Contains(out.Content, []byte("omitted")) {
		t.Fatal(len(out.Content), e)
	}
	if len(c.requests) < 2 {
		t.Fatal("not batched")
	}
	for _, r := range c.requests {
		if r.AgentContext != "Verify recent changes compile" || r.ExitCode != 1 || len(r.Command) != 2 || r.Command[0] != "build" {
			t.Fatal("context or command lost between batches", r)
		}
		if len(r.Chunks) > 8 {
			t.Fatal("batch too large")
		}
	}
}
func TestSelectionOrderOverlapAndLimit(t *testing.T) {
	raw := []byte(strings.Repeat("a\n", 12000))
	c := &fakeClient{score: func(ch Chunk) float64 {
		if ch.ID == "stdout_5" {
			return 0.99
		}
		return 0.6
	}}
	out, e := (&JEVFilter{c}).Apply(context.Background(), filter.FilterInput{Stdout: raw, Stderr: []byte("error\n")})
	if e != nil || bytes.Count(out.Content, []byte("a")) > 8192 || !bytes.Contains(out.Content, []byte("omitted")) {
		t.Fatal(e, len(out.Content))
	}
	cs := chunks(raw, "stdout")
	if len(cs) < 2 {
		t.Fatal("fixture")
	}
	c.score = func(ch Chunk) float64 {
		if ch.ID == "stdout_0" || ch.ID == "stdout_1" {
			return 1
		}
		return 0
	}
	out, e = (&JEVFilter{c}).Apply(context.Background(), filter.FilterInput{Stdout: raw})
	end := cs[2].end
	if e != nil || bytes.Count(out.Content, []byte("a\n")) != end/2 {
		t.Fatal(e, bytes.Count(out.Content, []byte("a\n")), end/2)
	}
}
func TestInvalidAndEmptySelection(t *testing.T) {
	for _, v := range []float64{0, math.NaN(), 2} {
		c := &fakeClient{score: func(Chunk) float64 { return v }}
		if _, e := (&JEVFilter{c}).Apply(context.Background(), filter.FilterInput{Stdout: []byte("error")}); e == nil {
			t.Fatal(v)
		}
	}
}
