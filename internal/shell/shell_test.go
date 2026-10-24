//go:build !windows

package shell

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func TestCommandLine(t *testing.T) {
	cases := map[string][]string{
		"":                             nil,
		"vim main.go":                  {"/usr/bin/vim", "main.go"},
		"claude --resume 1 -p a b":     {"claude", "--resume", "1", "-p", "a  b"},
		strings.Repeat("x", 119) + "…": {strings.Repeat("x", 200)},
	}
	for want, argv := range cases {
		if got := commandLine(argv); got != want {
			t.Errorf("commandLine(%q) = %q, want %q", argv, got, want)
		}
	}
}

// term is a browser attached to a shell over the WebSocket.
type term struct {
	t      *testing.T
	conn   *websocket.Conn
	out    strings.Builder
	status []Info
	exit   *Info
}

func attach(t *testing.T, srv *httptest.Server, id string) *term {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(srv.URL, "http")+"/api/shells/"+id+"/attach", nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.CloseNow() })
	return &term{t: t, conn: conn}
}

func (c *term) send(s string) {
	c.t.Helper()
	if err := c.conn.Write(context.Background(), websocket.MessageBinary, []byte(s)); err != nil {
		c.t.Fatal(err)
	}
}

// waitFor reads messages until ok holds, failing after 5 s.
func (c *term) waitFor(what string, ok func() bool) {
	c.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for !ok() {
		typ, data, err := c.conn.Read(ctx)
		if err != nil {
			c.t.Fatalf("waiting for %s: %v\noutput: %q", what, err, c.out.String())
		}
		if typ == websocket.MessageBinary {
			c.out.Write(data)
			continue
		}
		var msg struct {
			Type  string
			Shell Info
		}
		if err := json.Unmarshal(data, &msg); err != nil {
			c.t.Fatal(err)
		}
		if msg.Type == "exit" {
			c.exit = &msg.Shell
		}
		c.status = append(c.status, msg.Shell)
	}
}

func (c *term) last() Info {
	if len(c.status) == 0 {
		return Info{}
	}
	return c.status[len(c.status)-1]
}

func newServer(t *testing.T) (*Manager, *httptest.Server) {
	t.Helper()
	t.Setenv("SHELL", "/bin/sh")
	m := &Manager{Env: append(os.Environ(), "PS1=$ ")}
	mux := http.NewServeMux()
	m.Register(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(func() {
		m.StopAll()
		srv.Close()
	})
	return m, srv
}

func TestShell(t *testing.T) {
	m, srv := newServer(t)
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	dir, _ = filepath.EvalSymlinks(dir)

	first, err := m.Start(dir, "echo typed-$((40+2))")
	if err != nil {
		t.Fatal(err)
	}
	second, err := m.Start(dir, "")
	if err != nil {
		t.Fatal(err)
	}
	if first.N != 1 || second.N != 2 || first.Shell != "sh" || first.Cwd != dir {
		t.Fatalf("shells = %+v, %+v", first, second)
	}

	c := attach(t, srv, first.ID)
	// The run command is typed at the prompt and runs.
	c.waitFor("typed command", func() bool { return strings.Contains(c.out.String(), "typed-42") })
	// Typed input runs and streams back.
	c.send("echo hello-$((6*7))\r")
	c.waitFor("echo", func() bool { return strings.Contains(c.out.String(), "hello-42") })

	if runtime.GOOS == "linux" {
		// The tab follows cd and shows the foreground command.
		c.send("cd sub\r")
		c.waitFor("cwd", func() bool { return c.last().Cwd == filepath.Join(dir, "sub") })
		c.send("sleep 30\r")
		c.waitFor("command", func() bool { return c.last().Command == "sleep 30" })
		c.send("\x03") // Ctrl+C
		c.waitFor("prompt", func() bool { return len(c.status) > 0 && c.last().Command == "" && c.last().Cwd != dir })
	}

	// A second browser gets the scrollback.
	c2 := attach(t, srv, first.ID)
	c2.waitFor("scrollback", func() bool { return strings.Contains(c2.out.String(), "hello-42") })

	// Exit: both browsers are told the code; the shell stays listed until closed.
	c.send("exit 3\r")
	c.waitFor("exit", func() bool { return c.exit != nil })
	if !c.exit.Exited || c.exit.ExitCode != 3 {
		t.Fatalf("exit = %+v", c.exit)
	}
	c2.waitFor("exit", func() bool { return c2.exit != nil })
	if got := m.List(); len(got) != 2 || !got[0].Exited || got[1].Exited {
		t.Fatalf("list = %+v", got)
	}

	// Closing a running shell ends it; numbers keep counting up.
	if !m.Stop(second.ID) || !m.Stop(first.ID) {
		t.Fatal("Stop = false")
	}
	if len(m.List()) != 0 {
		t.Fatalf("list after stop = %+v", m.List())
	}
	third, err := m.Start(dir, "")
	if err != nil {
		t.Fatal(err)
	}
	if third.N != 3 {
		t.Fatalf("number = %d, want 3", third.N)
	}
}

func TestStopEndsAShellThatIgnoresHangUp(t *testing.T) {
	m, srv := newServer(t)
	info, err := m.Start(t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	c := attach(t, srv, info.ID)
	c.send("trap '' HUP; echo trapped\r")
	c.waitFor("trap", func() bool { return strings.Count(c.out.String(), "trapped") >= 2 })
	start := time.Now()
	m.Stop(info.ID)
	if time.Since(start) > 4*time.Second {
		t.Fatalf("Stop took %s", time.Since(start))
	}
	c.waitFor("closed", func() bool { return c.exit != nil })
}

func TestRoutes(t *testing.T) {
	_, srv := newServer(t)
	post := func(contentType, body string) *http.Response {
		res, err := http.Post(srv.URL+"/api/shells", contentType, strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		return res
	}
	if res := post("text/plain", `{}`); res.StatusCode != http.StatusUnsupportedMediaType {
		t.Errorf("text/plain = %d", res.StatusCode)
	}
	if res := post("application/json", `{"cwd":"relative"}`); res.StatusCode != http.StatusBadRequest {
		t.Errorf("relative cwd = %d", res.StatusCode)
	}
	if res := post("application/json", `{"cwd":"/no/such/dir"}`); res.StatusCode != http.StatusBadRequest {
		t.Errorf("missing cwd = %d", res.StatusCode)
	}
	if res := post("application/json", `{"cwd":"`+t.TempDir()+`"}`); res.StatusCode != http.StatusCreated {
		t.Errorf("create = %d", res.StatusCode)
	}

	// Another origin can't attach.
	res, err := http.Get(srv.URL + "/api/shells")
	if err != nil {
		t.Fatal(err)
	}
	var list struct{ Shells []Info }
	json.NewDecoder(res.Body).Decode(&list)
	res.Body.Close()
	if len(list.Shells) != 1 {
		t.Fatalf("shells = %+v", list.Shells)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, resp, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(srv.URL, "http")+"/api/shells/"+list.Shells[0].ID+"/attach",
		&websocket.DialOptions{HTTPHeader: http.Header{"Origin": {"http://localhost:5555"}}})
	if err == nil || resp == nil || resp.StatusCode != http.StatusForbidden {
		t.Errorf("cross-origin attach: err %v, resp %v", err, resp)
	}

	req, _ := http.NewRequest(http.MethodDelete, srv.URL+"/api/shells/"+list.Shells[0].ID, nil)
	if res, err := http.DefaultClient.Do(req); err != nil || res.StatusCode != http.StatusNoContent {
		t.Errorf("delete: %v %v", err, res)
	}
	if res, err := http.DefaultClient.Do(req); err != nil || res.StatusCode != http.StatusNotFound {
		t.Errorf("delete again: %v %v", err, res)
	}
}
