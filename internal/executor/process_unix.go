//go:build darwin || linux

package executor

import (
	"os"
	"os/exec"
	"syscall"
)

func configure(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if err == syscall.ESRCH {
			return os.ErrProcessDone
		}
		return err
	}
}
func termination(p *os.ProcessState, code int) (string, int) {
	if s, ok := p.Sys().(syscall.WaitStatus); ok && s.Signaled() {
		return s.Signal().String(), 128 + int(s.Signal())
	}
	return "", code
}
