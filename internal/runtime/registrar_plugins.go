package runtime

import (
	"context"

	"github.com/charmbracelet/crush/internal/agent/capability"
	"github.com/charmbracelet/crush/internal/plugin"
)

// PluginRegistryRegistrar seeds the plugin host from provided descriptors (if any).
type PluginRegistryRegistrar struct {
	Descriptors []plugin.Descriptor
	Path        string
}

func (r PluginRegistryRegistrar) Register(ctx context.Context, deps Services) error {
	host, ok := deps.PluginHost.(*plugin.Host)
	if !ok || host == nil {
		return nil
	}
	if r.Path != "" {
		if descs, err := LoadPluginsManifest(r.Path); err == nil {
			r.Descriptors = append(r.Descriptors, descs...)
		}
	}
	// If no descriptors were found and config paths are provided, attempt to locate manifests.
	if len(r.Descriptors) == 0 && len(deps.ConfigPaths) > 0 {
		for _, p := range deps.ConfigPaths {
			if descs, err := LoadPluginsManifest(p); err == nil {
				r.Descriptors = append(r.Descriptors, descs...)
			}
		}
	}
	// Optionally register plugin descriptors into capability registry for visibility.
	if reg, ok := deps.ToolsRegistry.(*capability.Registry); ok && reg != nil {
		for _, d := range r.Descriptors {
			reg.Register(capability.Descriptor{
				ID:          "plugin:" + d.Name,
				Description: "Plugin " + d.Name,
				Category:    "plugin",
				ReadOnly:    true,
				Tags:        []string{"plugin", d.Sandbox},
			})
		}
	}
	for _, d := range r.Descriptors {
		host.Register(d)
	}
	return nil
}
