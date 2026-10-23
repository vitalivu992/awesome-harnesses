//go:build !linux

package cursor

import (
	"os"
	"time"
)

// birthTime falls back to the modification time where statx isn't available.
func birthTime(_ string, st os.FileInfo) time.Time { return st.ModTime() }
