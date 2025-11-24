package runtime

import (
	"encoding/json"
	"os"

	"github.com/charmbracelet/crush/internal/plugin"
)

// LoadPluginsManifest reads plugin descriptors from disk.
func LoadPluginsManifest(path string) ([]plugin.Descriptor, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var descs []plugin.Descriptor
	if err := json.Unmarshal(data, &descs); err != nil {
		return nil, err
	}
	return descs, nil
}
