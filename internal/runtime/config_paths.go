package runtime

import "path/filepath"

// DefaultConfigPaths returns candidate manifest paths given a working directory.
func DefaultConfigPaths(workdir string) []string {
	if workdir == "" {
		return nil
	}
	return []string{
		filepath.Join(workdir, "plugins.json"),
		filepath.Join(workdir, ".crush", "plugins.json"),
		filepath.Join(workdir, "plugins.manifest.json"),
	}
}
