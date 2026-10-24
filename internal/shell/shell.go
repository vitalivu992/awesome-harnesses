// Package shell runs interactive shells in pseudo-terminals for the browser's
// terminal panel. Each shell is a login shell of the server's user, started in
// a folder; its output is kept (Scrollback) so a browser that reloads or
// attaches later sees what the terminal shows.
//
//	GET    /api/shells              every shell, by tab number
//	POST   /api/shells              {"cwd", "run"} → 201 the new shell
//	DELETE /api/shells/{id}         ends it → 204
//	GET    /api/shells/{id}/attach  WebSocket: the scrollback, then the live output
//
// On the WebSocket, binary messages are terminal bytes both ways; text
// messages are JSON: {"type":"resize","cols","rows"} from the browser,
// {"type":"status", shell} and {"type":"exit", shell} from the server.
package shell

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/aymanbagabas/go-pty"
	"github.com/coder/websocket"
)

// Scrollback is how many bytes of output a shell keeps for a browser that attaches.
const Scrollback = 512 << 10

// pollEvery is how often the foreground command and folder are read.
const pollEvery = time.Second

// Info is what the browser shows of a shell: its tab number, the folder and
// the command in the foreground ("" at the prompt).
type Info struct {
	ID        string    `json:"id"`
	N         int       `json:"n"`
	Shell     string    `json:"shell"`
	Cwd       string    `json:"cwd"`
	Command   string    `json:"command"`
	Exited    bool      `json:"exited"`
	ExitCode  int       `json:"exitCode"`
	CreatedAt time.Time `json:"createdAt"`
}

// Manager owns the running shells.
type Manager struct {
	// Env is the shells' environment (TERM and COLORTERM are added). Nil: the server's.
	Env []string

	mu     sync.Mutex
	shells map[string]*session
	last   int // the newest shell's tab number
}

type message struct {
	text bool
	data []byte
}

type session struct {
	pty       pty.Pty
	cmd       *pty.Cmd
	done      chan struct{} // closed once the shell exited and its output was read
	closeOnce sync.Once

	mu     sync.Mutex
	info   Info
	scroll []byte
	subs   map[chan message]struct{}
}

// Register adds the routes to mux.
func (m *Manager) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/shells", m.list)
	mux.HandleFunc("POST /api/shells", m.create)
	mux.HandleFunc("DELETE /api/shells/{id}", m.remove)
	mux.HandleFunc("GET /api/shells/{id}/attach", m.attach)
}

// List returns every shell, by tab number.
func (m *Manager) List() []Info {
	m.mu.Lock()
	out := make([]Info, 0, len(m.shells))
	for _, s := range m.shells {
		out = append(out, s.snapshot())
	}
	m.mu.Unlock()
	sort.Slice(out, func(i, j int) bool { return out[i].N < out[j].N })
	return out
}

// Start runs a login shell in dir. A non-empty run is typed at the first
// prompt, followed by Enter.
func (m *Manager) Start(dir, run string) (Info, error) {
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		return Info{}, fmt.Errorf("%q is not a folder", dir)
	}
	p, err := pty.New()
	if err != nil {
		return Info{}, err
	}
	name, args := userShell()
	cmd := p.Command(name, args...)
	cmd.Dir = dir
	env := m.Env
	if env == nil {
		env = os.Environ()
	}
	cmd.Env = append(env[:len(env):len(env)], "TERM=xterm-256color", "COLORTERM=truecolor")
	if err := p.Resize(120, 30); err != nil {
		log.Printf("shell: resize: %v", err)
	}
	if err := cmd.Start(); err != nil {
		p.Close()
		return Info{}, err
	}
	afterStart(p)

	s := &session{pty: p, cmd: cmd, done: make(chan struct{}), subs: map[chan message]struct{}{}}
	s.info = Info{ID: newID(), Shell: filepath.Base(name), Cwd: dir, CreatedAt: time.Now().UTC()}

	m.mu.Lock()
	if m.shells == nil {
		m.shells = map[string]*session{}
	}
	m.last++
	s.info.N = m.last
	m.shells[s.info.ID] = s
	m.mu.Unlock()

	go s.run(run)
	log.Printf("shell %d: %s in %s (pid %d)", s.info.N, name, dir, cmd.Process.Pid)
	return s.snapshot(), nil
}

// Stop ends a shell and forgets it.
func (m *Manager) Stop(id string) bool {
	m.mu.Lock()
	s, ok := m.shells[id]
	delete(m.shells, id)
	m.mu.Unlock()
	if ok {
		s.kill()
	}
	return ok
}

// StopAll ends every shell (on logout).
func (m *Manager) StopAll() {
	for _, info := range m.List() {
		m.Stop(info.ID)
	}
}

func (m *Manager) get(id string) *session {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.shells[id]
}

// run copies the output to the scrollback and the attached browsers, polls
// the foreground command and folder, and records the exit.
func (s *session) run(typeAhead string) {
	output := make(chan []byte, 64)
	go func() {
		defer close(output)
		buf := make([]byte, 32<<10)
		for {
			n, err := s.pty.Read(buf)
			if n > 0 {
				output <- append([]byte(nil), buf[:n]...)
			}
			if err != nil {
				return
			}
		}
	}()

	exited := make(chan struct{})
	go func() {
		err := s.cmd.Wait()
		code := 0
		if s.cmd.ProcessState != nil {
			code = s.cmd.ProcessState.ExitCode()
		} else if err != nil {
			code = -1
		}
		s.mu.Lock()
		s.info.Exited, s.info.ExitCode, s.info.Command = true, code, ""
		s.mu.Unlock()
		close(exited)
	}()

	// The typed command waits for the prompt: the first output, then a
	// quiet moment (prompt themes print in several bursts).
	var typeTimer, drain <-chan time.Time
	tick := time.NewTicker(pollEvery)
	defer tick.Stop()
	ticks := tick.C
	for output != nil || exited != nil {
		select {
		case b, ok := <-output:
			if !ok {
				output = nil
				continue
			}
			s.write(b)
			if typeAhead != "" {
				typeTimer = time.After(250 * time.Millisecond)
			}
		case <-typeTimer:
			if _, err := s.pty.Write([]byte(typeAhead + "\r")); err != nil {
				log.Printf("shell %d: type command: %v", s.info.N, err)
			}
			typeAhead, typeTimer = "", nil
		case <-ticks:
			s.poll()
		case <-exited:
			// Read what the shell printed last, then close the terminal (a
			// background job may still hold it open).
			exited, ticks = nil, nil
			drain = time.After(300 * time.Millisecond)
		case <-drain:
			s.closePty()
			drain = nil
		}
	}
	s.finish()
}

func (s *session) closePty() {
	s.closeOnce.Do(func() { s.pty.Close() })
}

// write appends output to the scrollback and sends it to the attached browsers.
func (s *session) write(b []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.scroll = append(s.scroll, b...)
	if over := len(s.scroll) - Scrollback; over > 0 {
		s.scroll = append(s.scroll[:0:0], s.scroll[over:]...)
	}
	s.broadcast(message{data: b})
}

// broadcast sends msg to every attached browser; one that can't keep up is
// dropped (its WebSocket closes and the browser reattaches). s.mu is held.
func (s *session) broadcast(msg message) {
	for ch := range s.subs {
		select {
		case ch <- msg:
		default:
			delete(s.subs, ch)
			close(ch)
		}
	}
}

func (s *session) poll() {
	s.mu.Lock()
	pid, cwd := s.cmd.Process.Pid, s.info.Cwd
	s.mu.Unlock()
	cwd, command := foreground(s.pty, pid, cwd)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.info.Exited || (cwd == s.info.Cwd && command == s.info.Command) {
		return
	}
	s.info.Cwd, s.info.Command = cwd, command
	s.broadcast(statusMessage("status", s.info))
}

// finish runs once the shell exited and its output is read: tell the
// browsers and let them go.
func (s *session) finish() {
	s.closePty()
	s.mu.Lock()
	s.broadcast(statusMessage("exit", s.info))
	for ch := range s.subs {
		delete(s.subs, ch)
		close(ch)
	}
	s.mu.Unlock()
	close(s.done)
	log.Printf("shell %d: exited (%d)", s.info.N, s.info.ExitCode)
}

func (s *session) kill() {
	hangUp(s.cmd.Process)
	s.closePty()
	select {
	case <-s.done:
	case <-time.After(3 * time.Second):
		s.cmd.Process.Kill()
	}
}

func (s *session) snapshot() Info {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.info
}

// subscribe returns the scrollback and a channel of what follows; nil when
// the shell has exited (the scrollback is all there is).
func (s *session) subscribe() ([]byte, chan message) {
	s.mu.Lock()
	defer s.mu.Unlock()
	scroll := append([]byte(nil), s.scroll...)
	select {
	case <-s.done:
		return scroll, nil
	default:
	}
	ch := make(chan message, 256)
	s.subs[ch] = struct{}{}
	return scroll, ch
}

func (s *session) unsubscribe(ch chan message) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.subs[ch]; ok {
		delete(s.subs, ch)
		close(ch)
	}
}

func statusMessage(kind string, info Info) message {
	b, _ := json.Marshal(map[string]any{"type": kind, "shell": info})
	return message{text: true, data: b}
}

// ---------- HTTP ----------

func (m *Manager) list(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"shells": m.List()})
}

func (m *Manager) create(w http.ResponseWriter, r *http.Request) {
	// JSON only: a cross-site form can't post it (the cookie is SameSite too).
	if mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type")); err != nil || mt != "application/json" {
		writeJSON(w, http.StatusUnsupportedMediaType, map[string]string{"error": "expected application/json"})
		return
	}
	var body struct {
		Cwd string `json:"cwd"`
		Run string `json:"run"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad request"})
		return
	}
	dir := body.Cwd
	if dir == "" {
		dir, _ = os.UserHomeDir()
	}
	if !filepath.IsAbs(dir) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "cwd must be an absolute path"})
		return
	}
	info, err := m.Start(filepath.Clean(dir), body.Run)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, info)
}

func (m *Manager) remove(w http.ResponseWriter, r *http.Request) {
	if !m.Stop(r.PathValue("id")) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (m *Manager) attach(w http.ResponseWriter, r *http.Request) {
	s := m.get(r.PathValue("id"))
	if s == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	// Accept checks that Origin is this host: another site (or another port
	// on localhost) can't open a shell with the user's cookie.
	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	defer conn.CloseNow()
	conn.SetReadLimit(1 << 20)
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	scroll, ch := s.subscribe()
	if ch != nil {
		defer s.unsubscribe(ch)
	}
	if len(scroll) > 0 {
		if err := conn.Write(ctx, websocket.MessageBinary, scroll); err != nil {
			return
		}
	}
	info := s.snapshot()
	kind := "status"
	if ch == nil {
		kind = "exit"
	}
	if err := conn.Write(ctx, websocket.MessageText, statusMessage(kind, info).data); err != nil || ch == nil {
		conn.Close(websocket.StatusNormalClosure, "")
		return
	}

	go func() {
		defer cancel()
		for {
			typ, data, err := conn.Read(ctx)
			if err != nil {
				return
			}
			if typ == websocket.MessageBinary {
				if _, err := s.pty.Write(data); err != nil {
					return
				}
				continue
			}
			var msg struct {
				Type       string `json:"type"`
				Cols, Rows int
			}
			if json.Unmarshal(data, &msg) == nil && msg.Type == "resize" && msg.Cols > 0 && msg.Rows > 0 && msg.Cols < 1000 && msg.Rows < 1000 {
				if err := s.pty.Resize(msg.Cols, msg.Rows); err != nil && !errors.Is(err, os.ErrClosed) {
					log.Printf("shell %d: resize: %v", info.N, err)
				}
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				conn.Close(websocket.StatusNormalClosure, "")
				return
			}
			typ := websocket.MessageBinary
			if msg.text {
				typ = websocket.MessageText
			}
			if err := conn.Write(ctx, typ, msg.data); err != nil {
				return
			}
		}
	}
}

func newID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("shell: write response: %v", err)
	}
}
