package shell

import (
	"os"
	"os/exec"

	"github.com/aymanbagabas/go-pty"
)

// userShell is PowerShell, else %ComSpec% (cmd.exe).
func userShell() (string, []string) {
	if p, err := exec.LookPath("powershell.exe"); err == nil {
		return p, []string{"-NoLogo"}
	}
	if c := os.Getenv("ComSpec"); c != "" {
		return c, nil
	}
	return "cmd.exe", nil
}

func afterStart(pty.Pty) {}

// hangUp has no Windows counterpart: closing the ConPTY ends the console's processes.
func hangUp(*os.Process) {}
