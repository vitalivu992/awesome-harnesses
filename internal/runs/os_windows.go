package runs

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func setGroup(cmd *exec.Cmd) {}

// interrupt: Windows has no SIGINT for another console's process; the run is killed.
func interrupt(p *os.Process) { p.Kill() }

func kill(p *os.Process) { p.Kill() }

// candidates: the name with each PATHEXT extension (npm installs claude.cmd).
func candidates(path string) []string {
	if filepath.Ext(path) != "" {
		return []string{path}
	}
	exts := strings.Split(strings.ToLower(os.Getenv("PATHEXT")), ";")
	if len(exts) == 1 && exts[0] == "" {
		exts = []string{".com", ".exe", ".bat", ".cmd"}
	}
	out := make([]string, 0, len(exts))
	for _, e := range exts {
		if e != "" {
			out = append(out, path+e)
		}
	}
	return out
}

func isExecutable(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}

// loginPath: Windows services see the user's PATH already.
func loginPath() string { return "" }
