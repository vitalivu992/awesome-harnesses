package shell

import (
	"os"
	"strconv"
	"strings"

	"github.com/aymanbagabas/go-pty"
	"golang.org/x/sys/unix"
)

// foreground returns the folder and command line of the terminal's foreground
// process group ("" when the shell itself is at the prompt), from /proc.
// cwd is returned when the folder can't be read.
func foreground(p pty.Pty, shellPid int, cwd string) (string, string) {
	pid := shellPid
	if pgid, ok := foregroundGroup(p); ok && pgid > 0 {
		pid = pgid
	}
	if dir, err := os.Readlink("/proc/" + strconv.Itoa(pid) + "/cwd"); err == nil {
		cwd = dir
	} else if dir, err := os.Readlink("/proc/" + strconv.Itoa(shellPid) + "/cwd"); err == nil {
		cwd = dir
	}
	if pid == shellPid {
		return cwd, ""
	}
	raw, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/cmdline")
	if err != nil {
		return cwd, ""
	}
	return cwd, commandLine(strings.Split(strings.TrimRight(string(raw), "\x00"), "\x00"))
}

// foregroundGroup is tcgetpgrp on the master side. It goes through Control,
// not Fd: Fd would switch the file to blocking mode, and Close could then no
// longer interrupt a pending Read.
func foregroundGroup(p pty.Pty) (int, bool) {
	u, ok := p.(pty.UnixPty)
	if !ok {
		return 0, false
	}
	pgid, ioErr := 0, error(nil)
	if err := u.Control(func(fd uintptr) { pgid, ioErr = unix.IoctlGetInt(int(fd), unix.TIOCGPGRP) }); err != nil || ioErr != nil {
		return 0, false
	}
	return pgid, true
}
