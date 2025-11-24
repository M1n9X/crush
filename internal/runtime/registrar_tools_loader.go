package runtime

import (
	"context"
	"path/filepath"

	"github.com/charmbracelet/crush/internal/agent/capability"
)

// ToolsConfig is a minimal manifest for declaring tool descriptors.
type ToolsConfig struct {
	Tools []capability.Descriptor `json:"tools"`
}

// ToolsManifestRegistrar loads tool descriptors from a manifest file (JSON).
type ToolsManifestRegistrar struct {
	Path string
}

func (r ToolsManifestRegistrar) Register(ctx context.Context, deps Services) error {
	if r.Path == "" {
		return nil
	}
	reg, ok := deps.ToolsRegistry.(*capability.Registry)
	if !ok || reg == nil {
		return nil
	}
	cfg, err := LoadToolsManifest(filepath.Clean(r.Path))
	if err != nil {
		return nil
	}
	for _, d := range cfg.Tools {
		reg.Register(d)
	}
	return nil
}
