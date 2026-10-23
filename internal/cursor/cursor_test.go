package cursor

import (
	"testing"

	"awesome-harness/internal/conv"
)

func TestParseFile(t *testing.T) {
	c, err := ParseFile("testdata/chat.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	if c.ID != "chat" || c.Harness != conv.Cursor || c.Title != "add a notes button" {
		t.Fatalf("id/harness/title = %q/%q/%q", c.ID, c.Harness, c.Title)
	}
	if c.StartedAt != "2026-10-23T06:46:00Z" {
		t.Fatalf("started = %q (from the first <timestamp>)", c.StartedAt)
	}
	if c.UpdatedAt == "" || c.Resumable {
		t.Fatalf("updated = %q resumable = %v (resumability comes from the CLI store)", c.UpdatedAt, c.Resumable)
	}
	if len(c.Entries) != 4 {
		t.Fatalf("entries = %d", len(c.Entries))
	}
	first := c.Entries[0]
	if first.Prompt != "add a notes button" || first.PromptAt != "2026-10-23T06:46:00Z" || first.Status != conv.StatusDone {
		t.Fatalf("first = %+v", first)
	}
	if first.Final != "Added the **Notes** button." {
		t.Fatalf("final = %q ([REDACTED] must be dropped)", first.Final)
	}
	if len(first.Steps) != 3 || first.Steps[0].Text != "Looking at the table." || first.Steps[1].Tool != "Grep" || first.Steps[2].Tool != "Shell" {
		t.Fatalf("steps = %+v", first.Steps)
	}
	if second := c.Entries[1]; second.Prompt != "shorter please" || second.Status != conv.StatusAborted {
		t.Fatalf("second = %+v (context blocks stripped, user abort is aborted)", second)
	}
	if third := c.Entries[2]; third.Status != conv.StatusDone || third.Final != "Done." {
		t.Fatalf("third = %+v (no marker but a final answer: done)", third)
	}
	if last := c.Entries[3]; last.Status != conv.StatusError || last.Error != "rate limited" {
		t.Fatalf("last = %+v", last)
	}
	if c.Turns != 4 || c.ToolCalls != 4 || c.Status != conv.StatusError {
		t.Fatalf("turns/tools/status = %d/%d/%s", c.Turns, c.ToolCalls, c.Status)
	}
}

func TestParseTimestamp(t *testing.T) {
	cases := map[string]string{
		"Friday, Oct 23, 2026, 1:46 PM (UTC+7)":    "2026-10-23T06:46:00Z",
		"Monday, Oct 19, 2026, 9:05 AM (UTC)":      "2026-10-19T09:05:00Z",
		"Monday, Oct 19, 2026, 9:05 AM (UTC-5:30)": "2026-10-19T14:35:00Z",
		"Wednesday, September 30, 2026, 4:09 PM":   "2026-09-30T16:09:00Z",
		"not a date":                               "",
	}
	for in, want := range cases {
		if got := ParseTimestamp(in); got != want {
			t.Errorf("ParseTimestamp(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestStripContext(t *testing.T) {
	in := "<external_links>\n<a>x</a>\n</external_links>\n<attached_files>f</attached_files>\nhello <b>there</b>"
	if got := stripContext(in); got != "hello <b>there</b>" {
		t.Fatalf("stripContext = %q", got)
	}
}
