//go:build !linux && !(darwin && cgo)

package shell

import "github.com/aymanbagabas/go-pty"

// foreground can't see into the terminal here (Windows ConPTY): the tab keeps
// the folder the shell started in and shows no command.
func foreground(_ pty.Pty, _ int, cwd string) (string, string) {
	return cwd, ""
}
