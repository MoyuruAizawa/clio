package jev

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

	"clio/internal/filter"
)

// Each distinguishable block is exactly one chunk, ending on a line boundary.
func blocks(letters string) []byte {
	var b bytes.Buffer
	for _, c := range []byte(letters) {
		b.Write(bytes.Repeat([]byte{c}, 4095))
		b.WriteByte('\n')
	}
	return b.Bytes()
}

func applyScores(in filter.FilterInput, scores map[string]float64) (filter.FilterOutput, error) {
	c := &fakeClient{score: func(ch Chunk) float64 { return scores[ch.ID] }}
	return (&JEVFilter{c}).Apply(context.Background(), in)
}

func assertBlocks(t *testing.T, content []byte, letters, selected string) {
	t.Helper()
	last := -1
	for _, c := range []byte(letters) {
		want := 0
		if strings.ContainsRune(selected, rune(c)) {
			want = 4095
		}
		if count := bytes.Count(content, []byte{c}); count != want {
			t.Fatalf("block %c: got %d bytes, want %d (chunk must be whole and unique)", c, count, want)
		}
		if want > 0 {
			pos := bytes.Index(content, bytes.Repeat([]byte{c}, 4095))
			if pos <= last {
				t.Fatal("raw order changed")
			}
			last = pos
		}
	}
}

func TestWholeNeighborsAtStreamEdges(t *testing.T) {
	for _, tc := range []struct {
		index int
		want  string
	}{{0, "AB"}, {2, "BCD"}, {4, "DE"}} {
		t.Run(fmt.Sprint(tc.index), func(t *testing.T) {
			out, e := applyScores(filter.FilterInput{Stdout: blocks("ABCDE"), Stderr: blocks("XYZ")}, map[string]float64{fmt.Sprintf("stdout_%d", tc.index): 0.5})
			if e != nil {
				t.Fatal(e)
			}
			assertBlocks(t, out.Content, "ABCDE", tc.want)
			assertBlocks(t, out.Content, "XYZ", "")
			if !bytes.Contains(out.Content, []byte("[clio: output omitted]")) {
				t.Fatal("missing omission marker")
			}
		})
	}
	// The first stderr chunk must never include the last stdout chunk as context.
	out, e := applyScores(filter.FilterInput{Stdout: blocks("ABCDE"), Stderr: blocks("XYZ")}, map[string]float64{"stderr_0": 1})
	if e != nil {
		t.Fatal(e)
	}
	assertBlocks(t, out.Content, "ABCDE", "")
	assertBlocks(t, out.Content, "XYZ", "XY")
}

func TestOverlappingNeighborsConsumeBudgetOnce(t *testing.T) {
	out, e := applyScores(filter.FilterInput{Stdout: blocks("ABCDE")}, map[string]float64{"stdout_1": 0.9, "stdout_2": 0.8})
	if e != nil {
		t.Fatal(e)
	}
	assertBlocks(t, out.Content, "ABCDE", "ABCD")
	if !bytes.Contains(out.Content, blocks("ABCD")) {
		t.Fatal("overlapping raw ranges not merged")
	}
}

func TestBudgetSkipsWholeGroupsAndKeepsConsideringCandidates(t *testing.T) {
	// EFG wins first. ABC cannot fit; DEF can fit because EF is already retained.
	out, e := applyScores(filter.FilterInput{Stdout: blocks("ABCDEFG")}, map[string]float64{"stdout_5": 0.99, "stdout_1": 0.9, "stdout_4": 0.8})
	if e != nil {
		t.Fatal(e)
	}
	assertBlocks(t, out.Content, "ABCDEFG", "DEFG")
	if !bytes.Contains(out.Content, blocks("DEFG")) {
		t.Fatal("selected groups are not intact")
	}
	// A high-ranked complete group leaves 4 KiB: do not return a partial lower group.
	out, e = applyScores(filter.FilterInput{Stdout: blocks("ABCDEFG")}, map[string]float64{"stdout_5": 0.99, "stdout_1": 0.9})
	if e != nil {
		t.Fatal(e)
	}
	assertBlocks(t, out.Content, "ABCDEFG", "EFG")
}

func TestRawBudgetIncludesRemovedANSIBytes(t *testing.T) {
	raw := []byte(strings.Repeat("\x1b[31m", 4000) + "compiler error\n")
	if _, e := applyScores(filter.FilterInput{Stdout: raw}, map[string]float64{"stdout_0": 1}); e == nil || !strings.Contains(e.Error(), "output limit") {
		t.Fatal("oversized raw range must fail without truncating", e)
	}
	out, e := applyScores(filter.FilterInput{Stdout: raw, Stderr: []byte("small error\n")}, map[string]float64{"stdout_0": 1, "stderr_0": 0.9})
	if e != nil || !bytes.Contains(out.Content, []byte("small error\n")) || bytes.Contains(out.Content, []byte("compiler error")) {
		t.Fatal("smaller range should still be considered", e)
	}
}

func TestGradleFailureIncludesWholeRoutineNeighbors(t *testing.T) {
	progressLine := "> Task :feature:example:compile UP-TO-DATE\n"
	routine := []byte(strings.Repeat(progressLine, 4096/len(progressLine)))
	failed := []byte("> Task :common:data:compileDevelopDebugKotlin FAILED\n")
	stdout := append(bytes.Repeat(routine, 3), failed...)
	stderr := []byte("e: FindPendantUseCase.kt:27:82 requires operator function 'component3()'.\nBUILD FAILED\n")
	cs := chunks(stdout, "stdout")
	important := cs[len(cs)-1]
	out, e := applyScores(filter.FilterInput{Command: []string{"./gradlew", "assembleDevelopDebug"}, ExitCode: 1, Stdout: stdout, Stderr: stderr}, map[string]float64{important.ID: 0.9, "stderr_0": 1})
	if e != nil {
		t.Fatal(e)
	}
	want := stdout[cs[len(cs)-2].start:]
	if !bytes.Contains(out.Content, want) || !bytes.Contains(out.Content, stderr) || !bytes.Contains(want, []byte("UP-TO-DATE")) {
		t.Fatal("complete failure chunk and routine neighbor must survive")
	}
}
