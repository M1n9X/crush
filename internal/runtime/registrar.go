package runtime

import (
	"context"
)

// Registrar allows composing boot steps (e.g., permissions, telemetry, MCP).
type Registrar interface {
	Register(ctx context.Context, deps Services) error
}

// RegistrarFunc is a functional adapter for Registrar.
type RegistrarFunc func(ctx context.Context, deps Services) error

// Register implements Registrar.
func (f RegistrarFunc) Register(ctx context.Context, deps Services) error {
	return f(ctx, deps)
}

// Services exposes the core services available during app bootstrap.
// Keeping this small avoids pulling the entire app to reduce coupling.
type Services struct {
	Config        any
	ConfigPaths   []string
	Permissions   any
	Telemetry     any
	Freshness     any
	MCPInit       func(context.Context)
	ToolsRegistry any
	PluginHost    any
	WorkingDir    string
	ConfigDir     string
}

// RunRegistrars executes registrars sequentially.
func RunRegistrars(ctx context.Context, registrars []Registrar, deps Services) error {
	for _, r := range registrars {
		if err := r.Register(ctx, deps); err != nil {
			return err
		}
	}
	return nil
}
