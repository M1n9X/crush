package runtime

import (
	"encoding/json"
	"os"
)

// LoadToolsManifest reads a manifest JSON into ToolsConfig.
func LoadToolsManifest(path string) (ToolsConfig, error) {
	var cfg ToolsConfig
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	err = json.Unmarshal(data, &cfg)
	return cfg, err
}
