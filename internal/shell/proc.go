package shell

import (
	"path/filepath"
	"strings"
)

// commandLine is how a tab shows argv: the program's base name and its
// arguments, at most 120 characters.
func commandLine(argv []string) string {
	if len(argv) == 0 || argv[0] == "" {
		return ""
	}
	parts := append([]string{filepath.Base(argv[0])}, argv[1:]...)
	s := strings.Join(strings.Fields(strings.Join(parts, " ")), " ")
	if r := []rune(s); len(r) > 120 {
		s = string(r[:119]) + "…"
	}
	return s
}
