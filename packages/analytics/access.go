package analytics

import (
	"context"

	"github.com/YASSERRMD/verdex/packages/identity"
)

// viewPermission is the identity.Permission required to read caseload,
// category, jurisdiction, and reasoning-quality-trend views. These are
// all read-only aggregates over case-scoped data, so this package
// reuses identity.PermViewCase — the same permission
// packages/caselifecycle, packages/casesearch, and
// packages/reasoningeval already gate case reads on — rather than
// inventing a new one.
const viewPermission = identity.PermViewCase

// costPermission is the identity.Permission required to read the
// usage/cost view (UsageView). Token spend is financial/operational
// data, not case content, so it is gated more narrowly than
// viewPermission: per identity.PermissionMatrix, only RoleJudge,
// RoleAdmin, and RoleAuditor hold identity.PermAuditRead, while
// RoleAdvocate and RoleClerk do not — matching this phase's
// requirement that "only certain roles see cost/usage views".
const costPermission = identity.PermAuditRead

// RequireViewPermission checks that ctx carries an authenticated
// identity.User who holds viewPermission, mirroring
// packages/caselifecycle.RequireViewPermission and
// packages/reasoningeval.RequireViewPermission exactly.
//
// Returns ErrUnauthenticated if no user is present on ctx, or
// ErrForbidden if the user lacks the permission.
func RequireViewPermission(ctx context.Context) error {
	user, ok := identity.UserFromContext(ctx)
	if !ok {
		return ErrUnauthenticated
	}
	if !user.HasPermission(viewPermission) {
		return ErrForbidden
	}
	return nil
}

// RequireCostPermission checks that ctx carries an authenticated
// identity.User who holds costPermission.
//
// Returns ErrUnauthenticated if no user is present on ctx, or
// ErrForbidden if the user lacks the permission.
func RequireCostPermission(ctx context.Context) error {
	user, ok := identity.UserFromContext(ctx)
	if !ok {
		return ErrUnauthenticated
	}
	if !user.HasPermission(costPermission) {
		return ErrForbidden
	}
	return nil
}
