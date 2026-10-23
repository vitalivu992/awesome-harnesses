package cursor

import (
	"crypto/md5"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"time"

	_ "modernc.org/sqlite"
)

// ChatInfo is what the cursor-agent CLI keeps about a chat in
// ~/.cursor/chats/<md5(workspace)>/<id>/store.db. Only CLI chats have one, so
// it also tells whether `cursor-agent --resume <id>` can pick the chat up.
type ChatInfo struct {
	CreatedAt time.Time
	Name      string
	Model     string
}

// ChatStorePath is where cursor-agent keeps chat `id` of `workspace`.
func ChatStorePath(chatsDir, workspace, id string) string {
	sum := md5.Sum([]byte(workspace))
	return filepath.Join(chatsDir, hex.EncodeToString(sum[:]), id, "store.db")
}

var modelNameRe = regexp.MustCompile(`"modelName":"([^"]+)"`)

func openReadOnly(path string) (*sql.DB, error) {
	// immutable: never take locks or touch the -wal/-shm files of a store
	// Cursor may be writing; a slightly stale read is fine for browsing.
	u := url.URL{Scheme: "file", Path: path, RawQuery: "mode=ro&immutable=1"}
	db, err := sql.Open("sqlite", u.String())
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	return db, nil
}

// ReadChatInfo reads a CLI chat's metadata (meta key "0": hex-encoded JSON)
// and the model named in its newest blob that records one.
func ReadChatInfo(storePath string) (*ChatInfo, error) {
	if _, err := os.Stat(storePath); err != nil {
		return nil, err
	}
	db, err := openReadOnly(storePath)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	info := &ChatInfo{}
	var metaHex string
	if err := db.QueryRow(`SELECT value FROM meta WHERE key = '0'`).Scan(&metaHex); err == nil {
		var meta struct {
			Name      string `json:"name"`
			CreatedAt int64  `json:"createdAt"`
		}
		if b, err := hex.DecodeString(metaHex); err == nil && json.Unmarshal(b, &meta) == nil {
			info.Name = meta.Name
			if meta.CreatedAt > 0 {
				info.CreatedAt = time.UnixMilli(meta.CreatedAt)
			}
		}
	}
	rows, err := db.Query(`SELECT data FROM blobs ORDER BY rowid DESC`)
	if err != nil {
		return info, nil
	}
	defer rows.Close()
	for rows.Next() {
		var data []byte
		if rows.Scan(&data) != nil {
			continue
		}
		if m := modelNameRe.FindSubmatch(data); m != nil {
			info.Model = string(m[1])
			break
		}
	}
	return info, nil
}

// ReadTrackedModels maps conversation id → model from Cursor's AI code
// tracking database (~/.cursor/ai-tracking/ai-code-tracking.db), which
// records the model of every AI edit; the newest edit of a conversation wins.
// Conversations that never edited a file are absent.
func ReadTrackedModels(dbPath string) (map[string]string, error) {
	if _, err := os.Stat(dbPath); err != nil {
		return nil, err
	}
	db, err := openReadOnly(dbPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	rows, err := db.Query(`SELECT conversationId, model, MAX(timestamp) FROM ai_code_hashes
		WHERE conversationId IS NOT NULL AND model IS NOT NULL AND model != '' GROUP BY conversationId`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	models := map[string]string{}
	for rows.Next() {
		var id, model string
		var ts sql.NullInt64
		if rows.Scan(&id, &model, &ts) == nil {
			models[id] = model
		}
	}
	return models, rows.Err()
}
