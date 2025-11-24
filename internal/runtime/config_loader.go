package runtime

import (
	"os"
	"path/filepath"
)

// ResolveToolsManifest returns the manifest path if present under config directory.
func ResolveToolsManifest(configDir string) string {
	if configDir == "" {
		return ""
	}
	candidates := []string{
		filepath.Join(configDir, "tools.json"),
		filepath.Join(configDir, "tools.manifest.json"),
		filepath.Join(configDir, ".crush", "tools.json"),
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return ""
}

// ResolvePluginsManifest returns the plugins manifest path if present.
func ResolvePluginsManifest(configDir string) string {
	if configDir == "" {
		return ""
	}
	candidates := []string{
		filepath.Join(configDir, "plugins.json"),
		filepath.Join(configDir, "plugins.manifest.json"),
		filepath.Join(configDir, ".crush", "plugins.json"),
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return ""
}
