// Package opencodetest builds OpenCode-schema databases for tests.
package opencodetest

import (
	"database/sql"
	"fmt"
)

// Schema is the subset of OpenCode's tables internal/opencode reads, for tests
// (here and in internal/catalog) that build a database.
const Schema = `
CREATE TABLE session (id TEXT PRIMARY KEY, parent_id TEXT, directory TEXT NOT NULL, title TEXT NOT NULL,
	time_created INTEGER NOT NULL, time_updated INTEGER NOT NULL, time_archived INTEGER);
CREATE TABLE message (id TEXT PRIMARY KEY, session_id TEXT NOT NULL, time_created INTEGER NOT NULL, data TEXT NOT NULL);
CREATE TABLE part (id TEXT PRIMARY KEY, message_id TEXT NOT NULL, session_id TEXT NOT NULL, time_created INTEGER NOT NULL, data TEXT NOT NULL);`

// Seed creates the schema and one session: a done turn (reasoning, a
// completed and a failed tool call, final text) and an aborted one, plus a
// subagent and an archived session that List must skip.
func Seed(db *sql.DB, dir string) error {
	stmts := []string{Schema,
		fmt.Sprintf(`INSERT INTO session VALUES ('ses_1', NULL, %q, 'Fix the build', 1000, 9000, NULL)`, dir),
		fmt.Sprintf(`INSERT INTO session VALUES ('ses_sub', 'ses_1', %q, 'Child session - x', 2000, 3000, NULL)`, dir),
		fmt.Sprintf(`INSERT INTO session VALUES ('ses_old', NULL, %q, 'New session - 2026-01-01T00:00:00Z', 100, 200, 300)`, dir),
		`INSERT INTO message VALUES ('msg_1', 'ses_1', 1000, '{"role":"user","time":{"created":1000}}')`,
		`INSERT INTO part VALUES ('prt_1', 'msg_1', 'ses_1', 1000, '{"type":"text","text":"why does the build fail?"}')`,
		`INSERT INTO part VALUES ('prt_2', 'msg_1', 'ses_1', 1001, '{"type":"text","text":"<file contents>","synthetic":true}')`,
		`INSERT INTO message VALUES ('msg_2', 'ses_1', 2000, '{"role":"assistant","modelID":"glm-5.3","providerID":"zai","finish":"tool-calls","time":{"created":2000,"completed":4000}}')`,
		`INSERT INTO part VALUES ('prt_3', 'msg_2', 'ses_1', 2001, '{"type":"reasoning","text":"Check the log.","time":{"start":2000,"end":2500}}')`,
		`INSERT INTO part VALUES ('prt_4', 'msg_2', 'ses_1', 2600, '{"type":"tool","tool":"bash","state":{"status":"completed","input":{"command":"make"},"output":"error: x","time":{"start":2600,"end":3600}}}')`,
		`INSERT INTO part VALUES ('prt_5', 'msg_2', 'ses_1', 3700, '{"type":"tool","tool":"read","state":{"status":"error","input":{"filePath":"/x"},"error":"no such file","time":{"start":3700,"end":3800}}}')`,
		`INSERT INTO message VALUES ('msg_3', 'ses_1', 5000, '{"role":"assistant","modelID":"glm-5.3","finish":"stop","time":{"created":5000,"completed":6000}}')`,
		`INSERT INTO part VALUES ('prt_6', 'msg_3', 'ses_1', 5000, '{"type":"text","text":"The Makefile uses a missing target.","time":{"start":5000,"end":5900}}')`,
		`INSERT INTO message VALUES ('msg_4', 'ses_1', 7000, '{"role":"user","time":{"created":7000}}')`,
		`INSERT INTO part VALUES ('prt_7', 'msg_4', 'ses_1', 7000, '{"type":"text","text":"fix it"}')`,
		`INSERT INTO message VALUES ('msg_5', 'ses_1', 8000, '{"role":"assistant","modelID":"glm-5.3","time":{"created":8000},"error":{"name":"MessageAbortedError","data":{"message":"Aborted"}}}')`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			return fmt.Errorf("%s: %w", s, err)
		}
	}
	return nil
}
