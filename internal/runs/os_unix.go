//go:build !windows

package runs

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

// setGroup starts the CLI in its own process group, so stopping it also
// stops the tools it started.
func setGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func interrupt(p *os.Process) { syscall.Kill(-p.Pid, syscall.SIGINT) }

func kill(p *os.Process) { syscall.Kill(-p.Pid, syscall.SIGKILL) }

func candidates(path string) []string { return []string{path} }

func isExecutable(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir() && st.Mode()&0o111 != 0
}

const pathMark = "__AH_PATH__="

// loginPath asks the user's shell for PATH, as an interactive login shell
// (zsh and bash users set it up in .zshrc / .bashrc: nvm, ~/.local/bin).
func loginPath() string {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, shell, "-l", "-i", "-c", `printf '\n`+pathMark+`%s\n' "$PATH"`).Output()
	if err != nil && len(out) == 0 {
		return ""
	}
	s := string(out)
	i := strings.LastIndex(s, pathMark)
	if i < 0 {
		return ""
	}
	path, _, _ := strings.Cut(s[i+len(pathMark):], "\n")
	return strings.TrimSpace(path)
}
