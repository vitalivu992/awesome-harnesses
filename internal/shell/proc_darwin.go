//go:build darwin && cgo

package shell

/*
#include <libproc.h>
#include <sys/param.h>
#include <string.h>
#include <sys/proc_info.h>

// Writes pid's working directory into buf; 0 on success.
static int ah_cwd(int pid, char *buf, int size) {
	struct proc_vnodepathinfo info;
	if (proc_pidinfo(pid, PROC_PIDVNODEPATHINFO, 0, &info, sizeof info) <= 0) return -1;
	strlcpy(buf, info.pvi_cdir.vip_path, size);
	return 0;
}
*/
import "C"

import (
	"unsafe"

	"github.com/aymanbagabas/go-pty"
	"golang.org/x/sys/unix"
)

// foreground returns the folder and command name of the terminal's
// foreground process group ("" when the shell itself is at the prompt):
// libproc for the folder, sysctl kern.proc.pid for the name (macOS keeps no
// argv there, so only the program name is shown). cwd is returned when the
// folder can't be read.
func foreground(p pty.Pty, shellPid int, cwd string) (string, string) {
	pid := shellPid
	if pgid, ok := foregroundGroup(p); ok && pgid > 0 {
		pid = pgid
	}
	buf := make([]byte, C.MAXPATHLEN)
	ptr := (*C.char)(unsafe.Pointer(&buf[0]))
	if C.ah_cwd(C.int(pid), ptr, C.int(len(buf))) == 0 || C.ah_cwd(C.int(shellPid), ptr, C.int(len(buf))) == 0 {
		cwd = C.GoString(ptr)
	}
	if pid == shellPid {
		return cwd, ""
	}
	kp, err := unix.SysctlKinfoProc("kern.proc.pid", pid)
	if err != nil {
		return cwd, ""
	}
	return cwd, commandLine([]string{unix.ByteSliceToString(kp.Proc.P_comm[:])})
}

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
