package cursor

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"awesome-harness/internal/conv"
)

// LogTurn is one agent turn of a Cursor app chat, as the agent host extension
// logs it: when it started, with which model, and when it ended. The app keeps
// its chat metadata on the machine that runs the editor (over Remote-SSH, not
// this one), so these logs are the only local record of an app chat's model.
type LogTurn struct {
	Start  time.Time
	End    time.Time
	Model  string
	Action string // userMessageAction, backgroundTaskCompletionAction, executePlanAction…
}

// LogFiles are the agent host logs under a Cursor server logs folder
// (~/.cursor-server/data/logs/<session>/exthost<n>/anysphere.cursor-agent-host/*.log).
func LogFiles(logsDir string) []string {
	files, _ := filepath.Glob(filepath.Join(logsDir, "*", "exthost*", "anysphere.cursor-agent-host", "*.log"))
	return files
}

const logTimeLayout = "2006-01-02 15:04:05.000"

// ReadLogTurns maps conversation id → turns in start order. A turn starts at
// "Selected Agent Host turn runtime" ({conversationId, modelId, actionCase})
// and ends at the next "Turn terminal blob reads" ({sessionId}). Log times
// are local times without a zone.
func ReadLogTurns(files []string) map[string][]LogTurn {
	type end struct {
		id string
		at time.Time
	}
	turns := map[string][]LogTurn{}
	var ends []end
	for _, path := range files {
		f, err := os.Open(path)
		if err != nil {
			continue
		}
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 64*1024), 4*1024*1024)
		for sc.Scan() {
			line := sc.Text()
			start := strings.Contains(line, "Selected Agent Host turn runtime")
			if !start && !strings.Contains(line, "Turn terminal blob reads") {
				continue
			}
			brace := strings.IndexByte(line, '{')
			if len(line) < len(logTimeLayout) || brace < 0 {
				continue
			}
			at, err := time.ParseInLocation(logTimeLayout, line[:len(logTimeLayout)], time.Local)
			if err != nil {
				continue
			}
			var v struct {
				ConversationID string `json:"conversationId"`
				SessionID      string `json:"sessionId"`
				ModelID        string `json:"modelId"`
				ActionCase     string `json:"actionCase"`
			}
			if json.Unmarshal([]byte(line[brace:]), &v) != nil {
				continue
			}
			if start && v.ConversationID != "" {
				turns[v.ConversationID] = append(turns[v.ConversationID], LogTurn{Start: at, Model: v.ModelID, Action: v.ActionCase})
			} else if !start && v.SessionID != "" {
				ends = append(ends, end{v.SessionID, at})
			}
		}
		f.Close()
	}
	for _, ts := range turns {
		sort.Slice(ts, func(i, j int) bool { return ts[i].Start.Before(ts[j].Start) })
	}
	// An end belongs to the conversation's latest turn that started before it.
	for _, e := range ends {
		ts := turns[e.id]
		i := sort.Search(len(ts), func(i int) bool { return ts[i].Start.After(e.at) }) - 1
		if i >= 0 && ts[i].End.IsZero() {
			ts[i].End = e.at
		}
	}
	return turns
}

// LogModels lists the models of turns in order of first use.
func LogModels(turns []LogTurn) []string {
	var models []string
	seen := map[string]bool{}
	for _, t := range turns {
		if t.Model != "" && !seen[t.Model] {
			seen[t.Model] = true
			models = append(models, t.Model)
		}
	}
	return models
}

// matchWindow: prompt <timestamp>s are to the minute, log times to the millisecond.
const matchWindow = 90 * time.Second

// AnnotateTurns sets the model, start and end time of each turn whose prompt
// <timestamp> is within a minute and a half of a logged prompt turn, matching
// both in order. Turns without a <timestamp> are left as they are.
func AnnotateTurns(entries []conv.Entry, turns []LogTurn) {
	next := 0
	for i := range entries {
		e := &entries[i]
		if e.Kind != conv.EntryTurn || e.PromptAt == "" {
			continue
		}
		at, err := time.Parse(time.RFC3339Nano, e.PromptAt)
		if err != nil {
			continue
		}
		for j := next; j < len(turns); j++ {
			t := turns[j]
			if t.Action == "backgroundTaskCompletionAction" || t.Start.Before(at.Add(-matchWindow)) {
				continue
			}
			if t.Start.After(at.Add(matchWindow)) {
				break
			}
			e.PromptAt = t.Start.UTC().Format(time.RFC3339Nano)
			if e.Model == "" {
				e.Model = t.Model
			}
			if e.EndedAt == "" && !t.End.IsZero() {
				e.EndedAt = t.End.UTC().Format(time.RFC3339Nano)
			}
			next = j + 1
			break
		}
	}
}
