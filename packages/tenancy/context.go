package tenancy

import (
	"context"

	"github.com/YASSERRMD/verdex/packages/persistence"
)

// tenantContextKey is an unexported context key type so values stored
// by this package can never collide with keys set by other packages,
// mirroring the pattern packages/observability/context.go uses for
// loggerKey.
type tenantContextKey struct{}

// TenantContext carries the tenant resolved for the current request or
// operation. It wraps *persistence.Tenant rather than aliasing it
// directly so this package can grow request-scoped fields (e.g.
// resolution metadata) later without changing persistence.Tenant's
// shape.
type TenantContext struct {
	Tenant *persistence.Tenant
}

// WithTenant returns a copy of ctx carrying tenant as the active
// request-scoped tenant. A subsequent TenantFromContext(ctx) call
// returns tenant.
func WithTenant(ctx context.Context, tenant *persistence.Tenant) context.Context {
	return context.WithValue(ctx, tenantContextKey{}, &TenantContext{Tenant: tenant})
}

// TenantFromContext returns the *persistence.Tenant attached to ctx by
// WithTenant, and whether one was present. It returns (nil, false) if
// no tenant has been resolved onto ctx yet.
func TenantFromContext(ctx context.Context) (*persistence.Tenant, bool) {
	tc, ok := ctx.Value(tenantContextKey{}).(*TenantContext)
	if !ok || tc == nil || tc.Tenant == nil {
		return nil, false
	}
	return tc.Tenant, true
}
