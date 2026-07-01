package tenancy

import (
	"context"
	"errors"
	"fmt"

	"github.com/YASSERRMD/verdex/packages/persistence"
)

// SandboxTenantSlug is the well-known slug of the default sandbox
// tenant SeedSandboxTenant ensures exists. Phase 008's setup wizard
// and early manual testing use this as the default tenant before any
// real tenant onboarding flow exists.
const SandboxTenantSlug = "sandbox"

// sandboxTenantName is the display name given to the sandbox tenant
// the first time SeedSandboxTenant creates it. It is not enforced on
// subsequent calls: if an operator has since renamed the sandbox
// tenant, SeedSandboxTenant leaves that rename alone rather than
// stomping it back to this default.
const sandboxTenantName = "Sandbox"

// SeedSandboxTenant ensures a well-known tenant with slug
// SandboxTenantSlug exists, creating it if absent. It is idempotent:
// running it again against a database that already has the sandbox
// tenant is a no-op that returns the existing tenant, so it is safe to
// call unconditionally from a bootstrap path on every startup.
//
// This is a seed *routine*, not a migration: it inserts data, not
// schema, so it belongs in application code that runs after
// migrations have been applied, not in packages/persistence/migrations.
func SeedSandboxTenant(ctx context.Context, exec persistence.Executor) (*persistence.Tenant, error) {
	tenants := persistence.NewPostgresTenantRepository()

	existing, err := tenants.GetBySlug(ctx, exec, SandboxTenantSlug)
	if err == nil {
		return existing, nil
	}
	if !errors.Is(err, persistence.ErrNotFound) {
		return nil, fmt.Errorf("tenancy: SeedSandboxTenant: lookup: %w", err)
	}

	tenant := &persistence.Tenant{Name: sandboxTenantName, Slug: SandboxTenantSlug}
	if err := tenants.Create(ctx, exec, tenant); err != nil {
		// A concurrent SeedSandboxTenant call (e.g. two service
		// instances starting simultaneously) could race between the
		// GetBySlug miss above and this Create; the tenants_slug_unique
		// constraint (migrations/000001_create_tenants.up.sql) makes
		// the loser's Create fail rather than create a duplicate. Treat
		// that race as success by re-fetching, so callers never see a
		// spurious failure from an otherwise-idempotent seed routine.
		if second, getErr := tenants.GetBySlug(ctx, exec, SandboxTenantSlug); getErr == nil {
			return second, nil
		}
		return nil, fmt.Errorf("tenancy: SeedSandboxTenant: create: %w", err)
	}
	return tenant, nil
}
