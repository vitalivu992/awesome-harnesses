package codex

import (
	"encoding/json"
	"testing"

	"awesome-harness/internal/conv"
)

func TestParseFile(t *testing.T) {
	c, err := ParseFile("testdata/rollout.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	if c.ID != "01a0-codex" || c.Harness != conv.Codex || c.Cwd != "/work/app" || c.GitBranch != "main" || !c.Resumable {
		t.Fatalf("summary = %+v", c.Summary)
	}
	if c.Title != "list the files" || c.StartedAt != "2026-10-20T10:00:00.000Z" || c.UpdatedAt != "2026-10-20T10:02:01.000Z" {
		t.Fatalf("title/started/updated = %q/%s/%s", c.Title, c.StartedAt, c.UpdatedAt)
	}
	if len(c.Entries) != 3 {
		t.Fatalf("entries = %d, want 3 (injected context and the <turn_aborted> note are not prompts)", len(c.Entries))
	}
	first := c.Entries[0]
	if first.Prompt != "list the files" || first.Model != "gpt-5.5" || first.Status != conv.StatusDone || first.Final != "There is one file: `main.go`." {
		t.Fatalf("first = %+v", first)
	}
	if first.EndedAt != "2026-10-20T10:00:07.200Z" {
		t.Fatalf("endedAt = %s (task_complete time)", first.EndedAt)
	}
	if len(first.Steps) != 4 || first.Steps[0].Kind != conv.StepThinking || first.Steps[0].Text != "**Listing files**" {
		t.Fatalf("steps = %+v", first.Steps)
	}
	shell, patch, search := first.Steps[1], first.Steps[2], first.Steps[3]
	var in struct {
		Command []string `json:"command"`
	}
	if json.Unmarshal(shell.Input, &in) != nil || len(in.Command) != 3 || shell.Output != "main.go\n" || shell.IsError {
		t.Fatalf("shell = %+v (arguments decoded to an object, output unwrapped)", shell)
	}
	var patchText string
	if json.Unmarshal(patch.Input, &patchText) != nil || patch.Tool != "apply_patch" || patch.Output != "failed" || !patch.IsError {
		t.Fatalf("apply_patch = %+v (freeform input kept as a string, exit_code 1 is an error)", patch)
	}
	if search.Tool != "tool_search" || string(search.Input) != `{"query":"memos"}` {
		t.Fatalf("tool_search = %+v (object arguments)", search)
	}
	if e := c.Entries[1]; e.Prompt != "now delete it" || e.Status != conv.StatusAborted {
		t.Fatalf("aborted turn = %+v", e)
	}
	if e := c.Entries[2]; e.Status != conv.StatusError || e.Error != "Your access token could not be refreshed" {
		t.Fatalf("error turn = %+v", e)
	}
	if len(c.Models) != 1 || c.ToolCalls != 4 || c.Status != conv.StatusError {
		t.Fatalf("models/tools/status = %v/%d/%s", c.Models, c.ToolCalls, c.Status)
	}
}

func TestReadMeta(t *testing.T) {
	m, err := ReadMeta("testdata/rollout.jsonl")
	if err != nil || m.ID != "01a0-codex" || m.Cwd != "/work/app" || m.Subagent() {
		t.Fatalf("meta = %+v, %v", m, err)
	}
	sub := Meta{Source: json.RawMessage(`{"subagent":{"thread_spawn":{}}}`)}
	if !sub.Subagent() {
		t.Fatal("an object source is a subagent session")
	}
}
