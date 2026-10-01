//go:build !darwin && !linux

package executor

import (
	"os"
	"os/exec"
)

func configure(cmd *exec.Cmd)                                {}
func termination(p *os.ProcessState, code int) (string, int) { return "", code }
