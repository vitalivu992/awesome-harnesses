//go:build !windows

package shell

import (
	"os"
	"syscall"

	"github.com/aymanbagabas/go-pty"
)

// userShell is $SHELL (else /bin/sh) as a login shell, so the profile sets
// PATH as in a desktop terminal.
func userShell() (string, []string) {
	name := os.Getenv("SHELL")
	if name == "" {
		name = "/bin/sh"
	}
	return name, []string{"-l"}
}

// afterStart drops the server's copy of the terminal's slave end: once the
// shell and its children close theirs, reading the master ends.
func afterStart(p pty.Pty) {
	if u, ok := p.(pty.UnixPty); ok {
		u.Slave().Close()
	}
}

// hangUp sends SIGHUP to the shell's process group, as closing a terminal
// window does.
func hangUp(proc *os.Process) {
	syscall.Kill(-proc.Pid, syscall.SIGHUP)
}
