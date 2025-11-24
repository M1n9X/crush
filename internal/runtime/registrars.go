package runtime

import (
	"context"
)

// Registrars contains grouped registrar slices to allow fine-grained overrides.
type Registrars struct {
	Permissions []Registrar
	Telemetry   []Registrar
	Tools       []Registrar
	Plugins     []Registrar
	Other       []Registrar
}

// Flatten returns all registrars in a deterministic order.
func (r Registrars) Flatten() []Registrar {
	var all []Registrar
	all = append(all, r.Permissions...)
	all = append(all, r.Telemetry...)
	all = append(all, r.Tools...)
	all = append(all, r.Plugins...)
	all = append(all, r.Other...)
	return all
}

// DefaultRegistrarsV2 returns a structured set with sensible defaults.
func DefaultRegistrarsV2() Registrars {
	return Registrars{
		Permissions: []Registrar{
			PermissionsRegistrar{ClearPersistent: false},
		},
		Telemetry: []Registrar{
			TelemetryLoggerRegistrar{},
		},
		Tools: []Registrar{
			ToolsManifestRegistrar{Path: ""},
			ToolsManifestRegistrar{Path: ".crush/tools.json"},
		},
		Plugins: []Registrar{
			PluginRegistryRegistrar{},
		},
		Other: []Registrar{
			RegistrarFunc(func(ctx context.Context, deps Services) error {
				if deps.MCPInit != nil {
					go deps.MCPInit(ctx)
				}
				return nil
			}),
		},
	}
}
