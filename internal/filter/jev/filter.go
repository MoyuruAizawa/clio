package jev

import (
	"bytes"
	"clio/internal/filter"
	"context"
	"errors"
	"fmt"
	"math"
	"regexp"
	"sort"
)

var ansi = regexp.MustCompile(`\x1b(?:\[[0-?]*[ -/]*[@-~]|\][^\x07\x1b]*(?:\x07|\x1b\\)|[@-_])`)

type JEVFilter struct{ Client Client }

// Normalize keeps a byte-to-raw offset map, including replaced progress lines.
func normalize(raw []byte) ([]byte, []int) {
	spans := ansi.FindAllIndex(raw, -1)
	skip := map[int]int{}
	for _, s := range spans {
		skip[s[0]] = s[1]
	}
	var out []byte
	var offsets []int
	// Assign removed escape bytes to the following visible byte.
	pending := 0
	for i := 0; i < len(raw); {
		if end, ok := skip[i]; ok {
			i = end
			continue
		}
		b := raw[i]
		if b == '\r' {
			b = '\n'
			if i+1 < len(raw) && raw[i+1] == '\n' {
				i++
				b = '\n'
			}
		}
		out = append(out, b)
		offsets = append(offsets, pending)
		i++
		pending = i
	}
	return out, offsets
}
func chunks(raw []byte, stream string) []Chunk {
	norm, mapping := normalize(raw)
	var result []Chunk
	for start := 0; start < len(norm); {
		end := min(start+4096, len(norm))
		if end < len(norm) {
			if n := bytes.LastIndexByte(norm[start:end], '\n'); n >= 0 {
				end = start + n + 1
			}
		}
		rs := mapping[start]
		if start == 0 {
			rs = 0
		}
		re := len(raw)
		if end < len(norm) {
			re = mapping[end]
		}
		result = append(result, Chunk{ID: fmt.Sprintf("%s_%d", stream, len(result)), Stream: stream, Text: string(norm[start:end]), start: rs, end: re, normStart: start, normEnd: end})
		start = end
	}
	// Give Jev bounded previews of the neighboring chunk edges for its context judgments.
	for i := range result {
		ch := &result[i]
		if i > 0 {
			prev := result[i-1]
			from := lastLinesStart(norm, prev.normStart, prev.normEnd, 10)
			ch.Before = string(norm[from:prev.normEnd])
			ch.beforeStart, ch.beforeEnd = rawRange(mapping, from, prev.normEnd, len(raw))
		}
		if i+1 < len(result) {
			next := result[i+1]
			to := firstLinesEnd(norm, next.normStart, next.normEnd, 10)
			ch.After = string(norm[next.normStart:to])
			ch.afterStart, ch.afterEnd = rawRange(mapping, next.normStart, to, len(raw))
		}
	}
	return result
}

func lastLinesStart(data []byte, start, end, count int) int {
	pos := end
	if pos > start && data[pos-1] == '\n' {
		pos--
	}
	for n := 0; n < count && pos > start; n++ {
		i := bytes.LastIndexByte(data[start:pos], '\n')
		if i < 0 {
			return start
		}
		pos = start + i + 1
	}
	return pos
}

func firstLinesEnd(data []byte, start, end, count int) int {
	pos := start
	for n := 0; n < count && pos < end; n++ {
		i := bytes.IndexByte(data[pos:end], '\n')
		if i < 0 {
			return end
		}
		pos += i + 1
	}
	return pos
}

func rawRange(mapping []int, start, end, rawLen int) (int, int) {
	rawStart := rawLen
	if start < len(mapping) {
		rawStart = mapping[start]
	}
	rawEnd := rawLen
	if end < len(mapping) {
		rawEnd = mapping[end]
	}
	return rawStart, rawEnd
}

func beforeQuestion(id string) string { return id + "_include_before" }
func afterQuestion(id string) string  { return id + "_include_after" }
func (f *JEVFilter) Apply(ctx context.Context, in filter.FilterInput) (filter.FilterOutput, error) {
	var output filter.FilterOutput
	all := append(chunks(in.Stdout, "stdout"), chunks(in.Stderr, "stderr")...)
	if len(all) == 0 {
		if len(in.Stdout)+len(in.Stderr) > 0 {
			return output, errors.New("no selectable log content")
		}
		return output, nil
	}
	scores := map[string]float64{}
	for i := 0; i < len(all); i += 8 {
		batch := all[i:min(i+8, len(all))]
		s, e := f.Client.Evaluate(ctx, Request{Command: in.Command, AgentContext: in.AgentContext, ExitCode: in.ExitCode, Chunks: batch})
		if e != nil {
			return output, e
		}
		if len(s) != len(batch)*3 {
			return output, errors.New("invalid selection")
		}
		for _, ch := range batch {
			for _, key := range []string{ch.ID, beforeQuestion(ch.ID), afterQuestion(ch.ID)} {
				v, ok := s[key]
				if !ok || math.IsNaN(v) || math.IsInf(v, 0) || v < 0 || v > 1 {
					return output, errors.New("invalid selection")
				}
				scores[key] = v
			}
		}
	}
	sort.SliceStable(all, func(i, j int) bool { return scores[all[i].ID] > scores[all[j].ID] })
	raws := map[string][]byte{"stdout": in.Stdout, "stderr": in.Stderr}
	masks := map[string][]bool{"stdout": make([]bool, len(in.Stdout)), "stderr": make([]bool, len(in.Stderr))}
	remaining := 16384
	chosen := 0
	budgetRejected := false
	for _, ch := range all {
		if scores[ch.ID] < 0.5 {
			continue
		}
		mask := masks[ch.Stream]
		// Keep a selected chunk whole; Jev independently decides whether either edge is needed.
		spans := [][2]int{{ch.start, ch.end}}
		if scores[beforeQuestion(ch.ID)] >= 0.5 && ch.beforeEnd > ch.beforeStart {
			spans = append(spans, [2]int{ch.beforeStart, ch.beforeEnd})
		}
		if scores[afterQuestion(ch.ID)] >= 0.5 && ch.afterEnd > ch.afterStart {
			spans = append(spans, [2]int{ch.afterStart, ch.afterEnd})
		}
		// Admit the core plus selected edge excerpts together or not at all.
		cost := 0
		for _, span := range spans {
			for j := span[0]; j < span[1]; j++ {
				if !mask[j] {
					cost++
				}
			}
		}
		if cost > remaining {
			budgetRejected = true
			continue
		}
		for _, span := range spans {
			for j := span[0]; j < span[1]; j++ {
				mask[j] = true
			}
		}
		remaining -= cost
		chosen += cost
	}
	if chosen == 0 {
		if budgetRejected {
			return output, errors.New("no complete log ranges fit the 16 KiB output limit")
		}
		return output, errors.New("no log ranges selected")
	}
	var b bytes.Buffer
	for _, stream := range []string{"stdout", "stderr"} {
		raw := raws[stream]
		mask := masks[stream]
		if len(raw) == 0 {
			continue
		}
		fmt.Fprintf(&b, "[%s]\n", stream)
		omitting := false
		for i, c := range raw {
			if mask[i] {
				if omitting {
					b.WriteString("\n[clio: output omitted]\n")
					omitting = false
				}
				b.WriteByte(c)
			} else {
				omitting = true
			}
		}
		if omitting {
			b.WriteString("\n[clio: output omitted]\n")
		}
		b.WriteByte('\n')
	}
	output.Content = b.Bytes()
	return output, nil
}
