package opencode

import (
	"database/sql"
	"path/filepath"
	"testing"

	"awesome-harness/internal/conv"
	"awesome-harness/internal/opencode/opencodetest"
)

func seeded(t *testing.T) *sql.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "opencode.db")
	w, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if err := opencodetest.Seed(w, "/work/app"); err != nil {
		t.Fatal(err)
	}
	w.Close()
	db, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestList(t *testing.T) {
	sessions, err := List(seeded(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 1 || sessions[0].ID != "ses_1" || sessions[0].Directory != "/work/app" || sessions[0].Updated.UnixMilli() != 9000 {
		t.Fatalf("sessions = %+v (subagent and archived sessions are left out)", sessions)
	}
}

func TestRead(t *testing.T) {
	c, err := Read(seeded(t), conv.Kilo, "ses_1")
	if err != nil {
		t.Fatal(err)
	}
	if c.Harness != conv.Kilo || c.Title != "Fix the build" || c.Cwd != "/work/app" || !c.Resumable {
		t.Fatalf("summary = %+v", c.Summary)
	}
	if len(c.Entries) != 2 {
		t.Fatalf("entries = %d", len(c.Entries))
	}
	first := c.Entries[0]
	if first.Prompt != "why does the build fail?" || first.Status != conv.StatusDone || first.Model != "glm-5.3" {
		t.Fatalf("first = %+v (synthetic parts are not part of the prompt)", first)
	}
	if first.Final != "The Makefile uses a missing target." || first.EndedAt != "1970-01-01T00:00:06Z" {
		t.Fatalf("final = %q ended %s", first.Final, first.EndedAt)
	}
	if len(first.Steps) != 3 || first.Steps[0].DurationMs != 500 {
		t.Fatalf("steps = %+v", first.Steps)
	}
	bash, read := first.Steps[1], first.Steps[2]
	if bash.Output != "error: x" || bash.IsError || bash.At != "1970-01-01T00:00:02.6Z" || bash.OutputAt != "1970-01-01T00:00:03.6Z" {
		t.Fatalf("bash = %+v", bash)
	}
	if !read.IsError || read.Output != "no such file" {
		t.Fatalf("read = %+v (error state shows the error)", read)
	}
	if e := c.Entries[1]; e.Prompt != "fix it" || e.Status != conv.StatusAborted {
		t.Fatalf("second = %+v", e)
	}
	if _, err := Read(seeded(t), conv.OpenCode, "missing"); err == nil {
		t.Fatal("missing session: want an error")
	}
}
