package claude

import (
	"testing"

	"awesome-harness/internal/conv"
)

func TestParseFile(t *testing.T) {
	c, err := ParseFile("testdata/session.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	if c.ID != "session" || c.Harness != conv.Claude || c.Title != "Health endpoint" {
		t.Fatalf("id/harness/title = %q/%q/%q", c.ID, c.Harness, c.Title)
	}
	if c.Cwd != "/work/app" || c.GitBranch != "main" {
		t.Fatalf("cwd/branch = %q/%q", c.Cwd, c.GitBranch)
	}
	if c.StartedAt != "2026-10-20T10:00:00.000Z" || c.UpdatedAt != "2026-10-20T10:04:01.000Z" {
		t.Fatalf("started/updated = %s/%s", c.StartedAt, c.UpdatedAt)
	}
	if got := len(c.Entries); got != 5 {
		for _, e := range c.Entries {
			t.Logf("%s %q %q", e.Kind, e.Prompt, e.Text)
		}
		t.Fatalf("entries = %d, want 5 (turn, /model event, turn, compact event, turn)", got)
	}

	first := c.Entries[0]
	if first.Kind != conv.EntryTurn || first.Prompt != "add a health endpoint" || first.Status != conv.StatusDone {
		t.Fatalf("first turn = %+v", first)
	}
	if first.Final != "Added `GET /health`." || first.FinalAt != "2026-10-20T10:00:10.000Z" {
		t.Fatalf("final = %q at %s", first.Final, first.FinalAt)
	}
	if first.Model != "claude-opus-5-5" || first.EndedAt != "2026-10-20T10:00:10.000Z" {
		t.Fatalf("model/ended = %q/%q", first.Model, first.EndedAt)
	}
	kinds := ""
	for _, s := range first.Steps {
		kinds += s.Kind[:2] + " "
	}
	if kinds != "th te to to " {
		t.Fatalf("step kinds = %q (sidechain lines must be skipped)", kinds)
	}
	if first.Steps[0].DurationMs != 800 {
		t.Fatalf("thinking duration = %d", first.Steps[0].DurationMs)
	}
	read, bash := first.Steps[2], first.Steps[3]
	if read.Tool != "Read" || read.Output != "package main" || read.OutputAt != "2026-10-20T10:00:04.000Z" {
		t.Fatalf("read step = %+v", read)
	}
	if bash.Output != "FAIL" || !bash.IsError {
		t.Fatalf("bash step = %+v", bash)
	}

	if ev := c.Entries[1]; ev.Kind != conv.EntryEvent || ev.Text != "/model" || ev.Detail != "Set model to Sonnet 5.5" {
		t.Fatalf("command event = %+v", ev)
	}
	if tt := c.Entries[2]; tt.Prompt != "now add tests" || tt.Status != conv.StatusAborted || tt.Final != "" {
		t.Fatalf("interrupted turn = %+v", tt)
	}
	if ev := c.Entries[3]; ev.Text != "Conversation compacted" || ev.Detail == "" {
		t.Fatalf("compact event = %+v", ev)
	}
	if last := c.Entries[4]; last.Status != conv.StatusError || last.Error != "Not logged in · Please run /login" {
		t.Fatalf("error turn = %+v", last)
	}

	if c.Turns != 3 || c.ToolCalls != 3 || c.Status != conv.StatusError || !c.Resumable {
		t.Fatalf("summary turns/tools/status/resumable = %d/%d/%s/%v", c.Turns, c.ToolCalls, c.Status, c.Resumable)
	}
	if len(c.Models) != 2 || c.Models[0] != "claude-opus-5-5" || c.Models[1] != "claude-sonnet-5-5" {
		t.Fatalf("models = %v", c.Models)
	}
}

func TestReadCwd(t *testing.T) {
	if got := ReadCwd("testdata/session.jsonl"); got != "/work/app" {
		t.Fatalf("ReadCwd = %q", got)
	}
	if got := ReadCwd("testdata/missing.jsonl"); got != "" {
		t.Fatalf("ReadCwd(missing) = %q", got)
	}
}
