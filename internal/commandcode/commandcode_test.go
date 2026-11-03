package commandcode

import (
	"strings"
	"testing"

	"awesome-harness/internal/conv"
)

func TestParseFile(t *testing.T) {
	c, err := ParseFile("testdata/cc-1.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	// The meta file's title wins over session_info's name.
	if c.ID != "cc-1" || c.Harness != conv.CommandCode || c.Cwd != "/work/app" || c.Title != "Readme and tests" || !c.Resumable {
		t.Fatalf("summary = %+v", c.Summary)
	}
	// u0 was rewound (not on the branch ending at the last entry).
	var kinds []string
	for _, e := range c.Entries {
		kinds = append(kinds, e.Kind+":"+e.Status)
	}
	if got := strings.Join(kinds, " "); got != "turn:error turn:done event: turn:aborted turn:incomplete" {
		t.Fatalf("entries = %s", got)
	}
	if e := c.Entries[0]; e.Prompt != "read the readme" || e.Error != "Insufficient credits" || len(e.Steps) != 0 {
		t.Fatalf("credits turn = %+v", e)
	}
	done := c.Entries[1]
	if done.Final != "It's called **App**." || done.Model != "z-ai/glm-5.3-flash" || len(done.Steps) != 3 {
		t.Fatalf("done turn = %+v", done)
	}
	if s := done.Steps[2]; s.Tool != "read_file" || s.Output != "# App" || s.OutputAt != "2026-11-02T10:02:02.000Z" || s.IsError {
		t.Fatalf("tool step = %+v", s)
	}
	if ev := c.Entries[2]; ev.Text != "Compacted" || ev.Detail != "The user asked about the readme." {
		t.Fatalf("compaction = %+v", ev)
	}
	aborted := c.Entries[3]
	if aborted.Model != "claude-sonnet-5-5" || !aborted.Steps[0].IsError || aborted.Steps[0].Output != "FAIL" {
		t.Fatalf("aborted turn = %+v", aborted)
	}
	if last := c.Entries[4]; last.Prompt != "try again" {
		t.Fatalf("last turn = %+v (the stop hook message is not a prompt)", last)
	}
	if c.Turns != 4 || c.ToolCalls != 3 || c.Status != conv.StatusIncomplete || strings.Join(c.Models, ",") != "z-ai/glm-5.3-flash,claude-sonnet-5-5" {
		t.Fatalf("counts = %+v", c.Summary)
	}
	if c.StartedAt != "2026-11-02T10:00:00.000Z" || c.UpdatedAt != "2026-11-02T10:05:01.000Z" {
		t.Fatalf("started/updated = %s/%s", c.StartedAt, c.UpdatedAt)
	}
}

func TestReadHeader(t *testing.T) {
	h, err := ReadHeader("testdata/cc-1.jsonl")
	if err != nil || h.ID != "cc-1" || h.Cwd != "/work/app" {
		t.Fatalf("header = %+v, %v", h, err)
	}
}
