//go:build darwin || linux

package credentials

import (
	"os"
	"syscall"
	"testing"
)

type wrongOwner struct{ os.FileInfo }

func (wrongOwner) Sys() any { return &syscall.Stat_t{Uid: uint32(os.Getuid() + 1)} }
func TestWrongOwner(t *testing.T) {
	st, e := os.Stat(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	if secureInfo(wrongOwner{st}, true) == nil {
		t.Fatal("wrong owner accepted")
	}
}
