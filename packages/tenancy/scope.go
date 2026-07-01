package tenancy

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/YASSERRMD/verdex/packages/persistence"
)

// WithTenantScope begins a real transaction on pool (via
// persistence.WithTx — transaction handling is not hand-rewritten
// here), issues `SET LOCAL app.current_tenant_id = '<tenantID>'` as
// the first statement inside that transaction, and only then invokes
// fn with the tx-scoped Executor. Every statement fn runs is therefore
// subject to the tenant_isolation Row-Level-Security policy defined in
// migrations/000003_enable_rls_deployments.up.sql, scoped to tenantID.
//
// # Why SET LOCAL, never plain SET
//
// SET LOCAL's effect is scoped to the current transaction: it is
// automatically and unconditionally undone when the transaction ends
// (commit or rollback), even if the underlying connection is returned
// to pgxpool's pool afterward for reuse by a different, unrelated
// request.
//
// Plain SET has session scope: it would persist on the physical
// connection after this transaction ends. Because pgxpool multiplexes
// many logical callers over a small set of pooled physical
// connections, a later request for a *different* tenant could acquire
// that same connection and inherit the previous tenant's
// app.current_tenant_id setting — a real cross-tenant data leak, not
// merely a style issue. Do not use plain SET anywhere in this package.
func WithTenantScope(
	ctx context.Context,
	pool *pgxpool.Pool,
	tenantID uuid.UUID,
	fn func(ctx context.Context, exec persistence.Executor) error,
) error {
	if pool == nil {
		return fmt.Errorf("tenancy: WithTenantScope: pool must not be nil")
	}
	if tenantID == uuid.Nil {
		return fmt.Errorf("tenancy: WithTenantScope: tenantID must not be nil")
	}
	if fn == nil {
		return fmt.Errorf("tenancy: WithTenantScope: fn must not be nil")
	}

	return persistence.WithTx(ctx, pool, func(ctx context.Context, exec persistence.Executor) error {
		// SET LOCAL cannot take a bound parameter (PostgreSQL does not
		// allow parameterized SET statements), so the tenant ID is
		// interpolated directly. uuid.UUID.String() always yields the
		// canonical "8-4-4-4-12" hex form, which cannot contain quotes
		// or SQL metacharacters, so this is not a SQL-injection vector
		// despite the string interpolation.
		setStmt := fmt.Sprintf("SET LOCAL app.current_tenant_id = '%s'", tenantID.String())
		if _, err := exec.Exec(ctx, setStmt); err != nil {
			return fmt.Errorf("tenancy: WithTenantScope: set app.current_tenant_id: %w", err)
		}

		return fn(ctx, exec)
	})
}
