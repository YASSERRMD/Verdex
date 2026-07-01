package tenancy

import (
	"errors"

	"github.com/google/uuid"
)

// ErrCrossTenantAccess is returned by tenant-scoped repository
// wrappers (e.g. TenantScopedDeploymentRepository) when asked to
// operate on an entity whose TenantID does not match the scope's
// tenantID. This check runs before any database access, so a caller
// that accidentally mixes up a tenant ID and an entity from a
// different tenant fails fast with a clear, checkable sentinel error
// rather than relying solely on the database's Row-Level Security to
// catch the mistake. See scope.go and README.md for the full
// defense-in-depth rationale.
var ErrCrossTenantAccess = errors.New("tenancy: cross-tenant access denied")

// requireMatchingTenant returns ErrCrossTenantAccess if entityTenantID
// is set and does not equal scopeTenantID. A nil entityTenantID (the
// zero uuid.UUID) is treated as "not yet assigned" and is not an error
// here; callers that must require an assigned tenant should check that
// separately.
func requireMatchingTenant(scopeTenantID, entityTenantID uuid.UUID) error {
	if entityTenantID != uuid.Nil && entityTenantID != scopeTenantID {
		return ErrCrossTenantAccess
	}
	return nil
}
