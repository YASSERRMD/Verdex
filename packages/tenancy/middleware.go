package tenancy

import (
	"context"
	"net/http"

	"github.com/YASSERRMD/verdex/packages/persistence"
)

// resolvedTenantKey is an unexported context key used to pass a
// resolved *persistence.Tenant from a resolution step (commit 6,
// resolve.go, or any future Phase 006 identity/session-based
// resolver) into Middleware. It is intentionally distinct from the
// context key WithTenant/TenantFromContext use (context.go): callers
// downstream of Middleware always read the tenant back via
// TenantFromContext, never via this package-private key, so the
// "resolved but not yet validated/attached" state and the "attached,
// ready to use" state can never be confused.
type resolvedTenantKey struct{}

// WithResolvedTenant returns a copy of ctx carrying tenant as the
// pending resolution result for Middleware to attach. Tenant
// resolution logic (e.g. resolve.go's header-based resolver, or a
// future Phase 006 identity/session-based resolver) calls this after
// looking up a tenant, before Middleware runs.
func WithResolvedTenant(ctx context.Context, tenant *persistence.Tenant) context.Context {
	return context.WithValue(ctx, resolvedTenantKey{}, tenant)
}

// resolvedTenantFromContext returns the tenant a resolver attached to
// ctx via WithResolvedTenant, if any.
func resolvedTenantFromContext(ctx context.Context) (*persistence.Tenant, bool) {
	tenant, ok := ctx.Value(resolvedTenantKey{}).(*persistence.Tenant)
	return tenant, ok && tenant != nil
}

// Middleware returns net/http middleware that expects a tenant to
// already have been resolved upstream (via WithResolvedTenant) and
// attaches it to the request context as the active tenant, retrievable
// downstream via TenantFromContext. Middleware itself performs no
// lookup and no I/O; it is the seam between "a tenant was resolved"
// and "handlers can call TenantFromContext".
//
// If no tenant was resolved upstream, Middleware responds with 401
// Unauthorized and does not call next — every handler wrapped by
// Middleware can assume TenantFromContext always succeeds.
//
// # Ordering with observability.CorrelationMiddleware
//
// Compose Middleware *inside* (i.e. wrapped by)
// observability.CorrelationMiddleware:
//
//	handler = observability.CorrelationMiddleware(logger)(tenancy.Middleware(next))
//
// Correlation IDs must exist for every request, including ones that
// get rejected for having no resolvable tenant, so the 401 response
// above still carries a correlation ID for support/debugging. Placing
// CorrelationMiddleware outermost guarantees that. Tenant resolution,
// by contrast, is meaningless without a correlation ID already on the
// context (log lines emitted during resolution/rejection should still
// be traceable), so tenancy.Middleware must run after
// (i.e. be nested inside) CorrelationMiddleware, never before it.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenant, ok := resolvedTenantFromContext(r.Context())
		if !ok {
			http.Error(w, "tenant not resolved", http.StatusUnauthorized)
			return
		}

		ctx := WithTenant(r.Context(), tenant)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
