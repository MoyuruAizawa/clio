//go:build darwin || linux

package credentials

import (
	"os"
	"os/exec"
	"testing"
)

func TestTerminalLoginHelper(t *testing.T) {
	if os.Getenv("CLIO_TERMINAL_HELPER") != "1" {
		return
	}
	if e := Login(Store{os.Getenv("CLIO_TERMINAL_DIR")}, os.Stdin, os.Stdout); e != nil {
		os.Exit(1)
	}
	os.Exit(0)
}

func TestTerminalLoginNoEcho(t *testing.T) {
	python, e := exec.LookPath("python3")
	if e != nil {
		t.Skip("python3 required for PTY verification")
	}
	// A real PTY verifies ReadPassword disables echo, not just prompt wording.
	const script = `import os, pty, select, subprocess, sys, termios, time
master, slave = pty.openpty()
env = dict(os.environ, CLIO_TERMINAL_HELPER="1", CLIO_TERMINAL_DIR=sys.argv[2])
p = subprocess.Popen([sys.argv[1], "-test.run=^TestTerminalLoginHelper$"], stdin=slave, stdout=slave, stderr=slave, env=env)
try:
    deadline = time.monotonic() + 10
    output = b""
    while b"TypeSafe API token: " not in output:
        assert time.monotonic() < deadline, "prompt timed out"
        if select.select([master], [], [], .1)[0]: output += os.read(master, 4096)
    while termios.tcgetattr(slave)[3] & termios.ECHO:
        assert time.monotonic() < deadline, "terminal echo remained enabled"
        time.sleep(.01)
    os.write(master, b"fake-terminal-secret\n")
    assert p.wait(timeout=10) == 0, "login failed"
    while select.select([master], [], [], .1)[0]: output += os.read(master, 4096)
    assert b"fake-terminal-secret" not in output, "token was echoed"
finally:
    if p.poll() is None: p.kill(); p.wait()
    os.close(master); os.close(slave)
`
	dir := t.TempDir() + "/clio"
	c := exec.Command(python, "-c", script, os.Args[0], dir)
	if output, e := c.CombinedOutput(); e != nil {
		t.Fatalf("terminal login: %v\n%s", e, output)
	}
	token, e := (Store{dir}).Read()
	if e != nil || token != "fake-terminal-secret" {
		t.Fatal("terminal credentials not saved", e)
	}
}
