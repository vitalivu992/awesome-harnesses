package catalog

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var nonAlnumRun = regexp.MustCompile(`[^A-Za-z0-9]+`)

// Slugs: Claude Code names a project folder after its path with every
// non-alphanumeric character turned into "-" (/home/a/my_app → -home-a-my-app);
// Cursor does the same without the leading dash and collapses runs
// (/home/a/.cursor → home-a-cursor).

// looseSlug collapses separator runs and trims them, so both styles compare equal.
func looseSlug(s string) string {
	return strings.Trim(nonAlnumRun.ReplaceAllString(s, "-"), "-")
}

// resolveSlug finds the existing directory whose path the slug was made
// from, walking the filesystem from "/" one folder name at a time (a "-" in
// the slug may stand for "/", "-", "_", "." or a space). Returns "" when no
// directory matches, e.g. the project was deleted or moved.
func resolveSlug(slug string) string {
	return walkSlug("/", looseSlug(slug))
}

func walkSlug(dir, rest string) string {
	if rest == "" {
		return dir
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if !e.IsDir() && e.Type()&os.ModeSymlink == 0 {
			continue
		}
		name := looseSlug(e.Name())
		if name == "" {
			continue
		}
		next := filepath.Join(dir, e.Name())
		if rest == name {
			if st, err := os.Stat(next); err == nil && st.IsDir() {
				return next
			}
			continue
		}
		if strings.HasPrefix(rest, name+"-") {
			if found := walkSlug(next, rest[len(name)+1:]); found != "" {
				return found
			}
		}
	}
	return ""
}

// guessSlug is the best path for a slug whose directory is gone: the deepest
// existing directory it starts with, plus the rest of the slug as one name
// (home-a-tmp-pr-42 with only /home/a left → /home/a/tmp-pr-42).
func guessSlug(slug string) string {
	dir, rest := "/", looseSlug(slug)
	for rest != "" {
		next, remaining := stepSlug(dir, rest)
		if next == "" {
			break
		}
		dir, rest = next, remaining
	}
	if rest == "" {
		return dir
	}
	return filepath.Join(dir, rest)
}

// stepSlug descends one existing folder that rest starts with (the longest
// such name, so "my-app" wins over "my"), returning it and what's left.
func stepSlug(dir, rest string) (string, string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", rest
	}
	best, bestRest := "", rest
	for _, e := range entries {
		if !e.IsDir() && e.Type()&os.ModeSymlink == 0 {
			continue
		}
		name := looseSlug(e.Name())
		if name == "" || len(name) <= len(best)-len(dir)-1 {
			continue
		}
		if !e.IsDir() { // a symlink (macOS /var, /tmp): only if it points at a folder
			if st, err := os.Stat(filepath.Join(dir, e.Name())); err != nil || !st.IsDir() {
				continue
			}
		}
		switch {
		case rest == name:
			best, bestRest = filepath.Join(dir, e.Name()), ""
		case strings.HasPrefix(rest, name+"-"):
			best, bestRest = filepath.Join(dir, e.Name()), rest[len(name)+1:]
		}
	}
	return best, bestRest
}
