package tenancy

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/YASSERRMD/verdex/packages/persistence"
)

// TenantSlugHeader is the inbound HTTP header HeaderResolver reads to
// determine which tenant a request belongs to.
//
// # This is a placeholder resolution mechanism
//
// Phase 006 (Identity & RBAC) does not exist yet, so there is no
// authenticated session or token to resolve a tenant from. Until then,
// callers are expected to set this header directly (e.g. from an
// internal gateway, a test harness, or a trusted operator tool) -
// HeaderResolver performs no authentication and trusts the header
// value outright. Phase 006 is expected to replace HeaderResolver
// entirely with resolution derived from an authenticated
// identity/session (e.g. a claim or session record naming the
// tenant), at which point this header-based path should be removed
// rather than kept as a fallback, to avoid leaving two divergent
// resolution mechanisms in place. Do not build additional
// authentication/authorization logic on top of this header in the
// meantime; that is out of scope for this phase.
const TenantSlugHeader = "X-Tenant-Slug"

// ErrTenantSlugMissing is returned by HeaderResolver.Resolve when the
// inbound request has no TenantSlugHeader value (or it is blank).
var ErrTenantSlugMissing = errors.New("tenancy: request has no tenant slug header")

// HeaderResolver resolves a *persistence.Tenant from the
// TenantSlugHeader of an inbound request, looking it up via
// persistence.TenantRepository.GetBySlug. See TenantSlugHeader's
// documentation for why this is a placeholder, to be superseded by
// Phase 006.
type HeaderResolver struct {
	tenants persistence.TenantRepository
	exec    persistence.Executor
}

// NewHeaderResolver builds a HeaderResolver that looks tenants up via
// tenants, executing queries against exec (typically a *pgxpool.Pool;
// an Executor is accepted so a transaction-scoped call site can pass a
// pgx.Tx instead).
func NewHeaderResolver(tenants persistence.TenantRepository, exec persistence.Executor) *HeaderResolver {
	return &HeaderResolver{tenants: tenants, exec: exec}
}

// Resolve reads TenantSlugHeader from r and looks up the matching
// tenant. It returns ErrTenantSlugMissing if the header is absent or
// blank, and persistence.ErrNotFound (unwrapped, checkable via
// errors.Is) if no tenant has that slug.
func (h *HeaderResolver) Resolve(ctx context.Context, r *http.Request) (*persistence.Tenant, error) {
	slug := r.Header.Get(TenantSlugHeader)
	if slug == "" {
		return nil, ErrTenantSlugMissing
	}

	tenant, err := h.tenants.GetBySlug(ctx, h.exec, slug)
	if err != nil {
		if errors.Is(err, persistence.ErrNotFound) {
			return nil, err
		}
		return nil, fmt.Errorf("tenancy: HeaderResolver.Resolve: %w", err)
	}
	return tenant, nil
}

// ResolveMiddleware returns net/http middleware that runs resolver
// against each inbound request and attaches the result via
// WithResolvedTenant, so a subsequent Middleware call (middleware.go)
// can pick it up. Requests that fail resolution get a 401 with a
// generic message (the specific reason - missing header vs. unknown
// slug - is not exposed to the client, to avoid tenant-enumeration via
// error message differences); resolver errors are not logged here,
// since resolution runs before any logger is guaranteed to be bound to
// the request context in every deployment of this middleware.
//
// Compose it outside tenancy.Middleware and inside
// observability.CorrelationMiddleware:
//
//	handler = observability.CorrelationMiddleware(logger)(
//	    tenancy.ResolveMiddleware(resolver)(
//	        tenancy.Middleware(next)))
func ResolveMiddleware(resolver *HeaderResolver) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tenant, err := resolver.Resolve(r.Context(), r)
			if err != nil {
				http.Error(w, "tenant not resolved", http.StatusUnauthorized)
				return
			}

			ctx := WithResolvedTenant(r.Context(), tenant)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
