package execution

import (
	"bytes"
	"clio/internal/filter"
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

type fakeFilter struct {
	calls   int
	in      filter.FilterInput
	err     error
	content []byte
	t       *testing.T
}

func TestChildDeadlineDoesNotCancelFiltering(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	f := &fakeFilter{t: t, content: []byte("selected after timeout")}
	var out, errout bytes.Buffer
	code, e := Execute(ctx, []string{"/bin/sleep", "2"}, "", nil, &out, &errout, f)
	if e != nil || code != 137 || f.calls != 1 || out.String() != "selected after timeout" || !strings.Contains(errout.String(), "timeout=true") {
		t.Fatal(code, e, f.calls, out.String(), errout.String())
	}
}

func (f *fakeFilter) Apply(ctx context.Context, in filter.FilterInput) (filter.FilterOutput, error) {
	f.calls++
	f.in = in
	if ctx.Err() != nil {
		f.t.Fatal("child deadline leaked")
	}
	return filter.FilterOutput{Content: f.content}, f.err
}
func TestHelper(t *testing.T) {
	if os.Getenv("CLIO_EXEC_HELPER") != "1" {
		return
	}
	n, _ := strconv.Atoi(os.Getenv("CLIO_EXEC_BYTES"))
	fmt.Fprint(os.Stdout, strings.Repeat("o", n))
	fmt.Fprint(os.Stderr, strings.Repeat("e", n))
	code, _ := strconv.Atoi(os.Getenv("CLIO_EXEC_CODE"))
	os.Exit(code)
}
func TestAlwaysOnError(t *testing.T) {
	for _, code := range []int{0, 9} {
		t.Run(strconv.Itoa(code), func(t *testing.T) {
			t.Setenv("CLIO_EXEC_HELPER", "1")
			t.Setenv("CLIO_EXEC_BYTES", "10")
			t.Setenv("CLIO_EXEC_CODE", strconv.Itoa(code))
			var out, errout bytes.Buffer
			f := &fakeFilter{content: []byte("selected"), t: t}
			args := []string{os.Args[0], "-test.run=^TestHelper$"}
			actual, e := Execute(context.Background(), args, "Verify the recent changes", nil, &out, &errout, f)
			if e != nil || actual != code || !strings.Contains(errout.String(), fmt.Sprintf("exit=%d", code)) {
				t.Fatal(actual, e, errout.String())
			}
			want := ""
			calls := 0
			if code != 0 {
				want = "selected"
				calls = 1
			}
			if out.String() != want || f.calls != calls {
				t.Fatal(out.String(), f.calls)
			}
			if calls > 0 && (f.in.AgentContext != "Verify the recent changes" || f.in.ExitCode != code || len(f.in.Stdout) != 10 || len(f.in.Stderr) != 10 || f.in.Command[0] != args[0]) {
				t.Fatal(f.in)
			}
		})
	}
}
func TestFallback(t *testing.T) {
	for _, empty := range []bool{false, true} {
		t.Setenv("CLIO_EXEC_HELPER", "1")
		t.Setenv("CLIO_EXEC_BYTES", "20000")
		t.Setenv("CLIO_EXEC_CODE", "7")
		f := &fakeFilter{t: t}
		if !empty {
			f.err = errors.New("unavailable")
		}
		var out, errout bytes.Buffer
		code, e := Execute(context.Background(), []string{os.Args[0], "-test.run=^TestHelper$"}, "", nil, &out, &errout, f)
		if e != nil || code != 7 || out.Len() != 4096 || strings.Count(errout.String(), "e") < 4096 || !strings.Contains(errout.String(), "filtering failed") {
			t.Fatal(code, e, out.Len(), errout.Len())
		}
	}
}
