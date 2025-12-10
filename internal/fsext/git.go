package fsext

import (
	"os"
	"path/filepath"
)

// FindGitRoot walks up from dir until it finds a .git directory or file.
// Returns the path to the directory containing .git, or empty string if none.
func FindGitRoot(dir string) string {
	if dir == "" {
		return ""
	}
	dir, err := filepath.Abs(dir)
	if err != nil {
		return ""
	}

	cursor := dir
	for {
		if _, err := os.Stat(filepath.Join(cursor, ".git")); err == nil {
			return cursor
		}
		parent := filepath.Dir(cursor)
		if parent == cursor {
			return ""
		}
		cursor = parent
	}
}
