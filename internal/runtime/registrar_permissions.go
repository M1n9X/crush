package runtime

import (
	"context"

	"github.com/charmbracelet/crush/internal/permission"
)

// PermissionsRegistrar seeds permission stores (e.g., clear on request or preload).
type PermissionsRegistrar struct {
	ClearPersistent bool
	Bootstrap       []permission.PermissionRequest
}

func (r PermissionsRegistrar) Register(ctx context.Context, deps Services) error {
	svc, ok := deps.Permissions.(permission.Service)
	if !ok || svc == nil {
		return nil
	}
	if r.ClearPersistent {
		_ = svc.ClearPersistent()
	}
	for _, p := range r.Bootstrap {
		svc.GrantPersistent(p)
	}
	return nil
}
