package cursor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"awesome-harness/internal/conv"
)

func TestReadLogTurnsAndAnnotate(t *testing.T) {
	dir := t.TempDir()
	host := filepath.Join(dir, "20261023T100000", "exthost2", "anysphere.cursor-agent-host")
	if err := os.MkdirAll(host, 0o755); err != nil {
		t.Fatal(err)
	}
	// Log times are local; prompts are in UTC.
	at := func(utc string) string {
		ts, _ := time.Parse(time.RFC3339, utc)
		return ts.Local().Format(logTimeLayout)
	}
	start := func(utc, id, model, action string) string {
		return at(utc) + ` [info] Selected Agent Host turn runtime {"runtime":"connect","conversationId":"` + id +
			`","actionCase":"` + action + `","modelId":"` + model + `"}`
	}
	end := func(utc, id string) string {
		return at(utc) + ` [info] Turn terminal blob reads {"component":"storage","sessionId":"` + id + `","count":0}`
	}
	host1 := strings.Join([]string{
		start("2026-10-23T15:44:55Z", "chat", "composer-2.5", "userMessageAction"),
		"2026-10-23 22:45:00.000 [info] unrelated line",
		start("2026-10-23T15:45:30Z", "chat", "composer-2.5", "backgroundTaskCompletionAction"),
		start("2026-10-23T15:46:54Z", "chat", "claude-opus-5-5", "userMessageAction"),
		start("2026-10-23T15:46:00Z", "other", "grok-4.7", "userMessageAction"),
	}, "\n")
	storage := strings.Join([]string{
		end("2026-10-23T15:45:20Z", "chat"),
		end("2026-10-23T15:47:04Z", "chat"),
	}, "\n")
	os.WriteFile(filepath.Join(host, "Cursor Agent Host.log"), []byte(host1), 0o644)
	os.WriteFile(filepath.Join(host, "Cursor Agent Host Storage.log"), []byte(storage), 0o644)

	files := LogFiles(dir)
	if len(files) != 2 {
		t.Fatalf("files = %v", files)
	}
	turns := ReadLogTurns(files)["chat"]
	if len(turns) != 3 || turns[0].End.IsZero() || !turns[1].End.IsZero() || turns[2].End.IsZero() {
		t.Fatalf("turns = %+v (each end goes to the latest turn started before it)", turns)
	}
	if got := LogModels(turns); strings.Join(got, ",") != "composer-2.5,claude-opus-5-5" {
		t.Fatalf("models = %v", got)
	}

	entries := []conv.Entry{
		{Kind: conv.EntryTurn, PromptAt: "2026-10-23T15:45:00Z"},
		{Kind: conv.EntryEvent},
		{Kind: conv.EntryTurn}, // no <timestamp>: left alone
		{Kind: conv.EntryTurn, PromptAt: "2026-10-23T15:47:00Z"},
		{Kind: conv.EntryTurn, PromptAt: "2026-10-23T18:00:00Z"}, // nothing logged near it
	}
	AnnotateTurns(entries, turns)
	if e := entries[0]; e.Model != "composer-2.5" || e.PromptAt != "2026-10-23T15:44:55Z" || e.EndedAt != "2026-10-23T15:45:20Z" {
		t.Fatalf("first = %+v", e)
	}
	if e := entries[2]; e.Model != "" || e.PromptAt != "" {
		t.Fatalf("untimed = %+v", e)
	}
	if e := entries[3]; e.Model != "claude-opus-5-5" || e.EndedAt != "2026-10-23T15:47:04Z" {
		t.Fatalf("second = %+v (background turns are skipped)", e)
	}
	if e := entries[4]; e.Model != "" || e.PromptAt != "2026-10-23T18:00:00Z" {
		t.Fatalf("unmatched = %+v", e)
	}
}
