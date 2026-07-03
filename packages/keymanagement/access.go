package keymanagement

import (
	"context"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/identity"
)

// viewPermission is the identity.Permission required to read key
// metadata (ListForTenant/Get-style operations, and the audit trail
// itself). Mirrors packages/caseversioning's viewPermission
// convention.
const viewPermission = identity.PermViewKeys

// managePermission is the identity.Permission required to rotate or
// revoke a key.
const managePermission = identity.PermManageKeys

// breakGlassPermission is the identity.Permission required to invoke
// the emergency break-glass procedure (breakglass.go). Deliberately
// distinct from managePermission — see identity.PermBreakGlassKeys's
// doc comment.
const breakGlassPermission = identity.PermBreakGlassKeys

// authorizeView checks that ctx carries an authenticated
// identity.User who holds viewPermission. Returns ErrUnauthenticated
// if no user is present on ctx, or ErrForbidden if the user lacks the
// permission.
func authorizeView(ctx context.Context) (*identity.User, error) {
	user, ok := identity.UserFromContext(ctx)
	if !ok {
		return nil, ErrUnauthenticated
	}
	if !user.HasPermission(viewPermission) {
		return nil, ErrForbidden
	}
	return user, nil
}

// authorizeManage checks that ctx carries an authenticated
// identity.User who holds managePermission (task 5: "role-gated key
// operations — who can rotate/revoke"). Returns ErrUnauthenticated if
// no user is present on ctx, or ErrForbidden if the user lacks the
// permission.
func authorizeManage(ctx context.Context) (*identity.User, error) {
	user, ok := identity.UserFromContext(ctx)
	if !ok {
		return nil, ErrUnauthenticated
	}
	if !user.HasPermission(managePermission) {
		return nil, ErrForbidden
	}
	return user, nil
}

// authorizeBreakGlass checks that ctx carries an authenticated
// identity.User who holds breakGlassPermission. Returns
// ErrUnauthenticated if no user is present on ctx, or ErrForbidden if
// the user lacks the permission.
func authorizeBreakGlass(ctx context.Context) (*identity.User, error) {
	user, ok := identity.UserFromContext(ctx)
	if !ok {
		return nil, ErrUnauthenticated
	}
	if !user.HasPermission(breakGlassPermission) {
		return nil, ErrForbidden
	}
	return user, nil
}

// requireMatchingUserTenant returns ErrCrossTenantAccess if user's
// TenantID does not match tenantID — a key operation always scoped to
// the actor's own tenant, never another tenant's, even for an admin.
func requireMatchingUserTenant(user *identity.User, tenantID uuid.UUID) error {
	if user.TenantID != tenantID {
		return ErrCrossTenantAccess
	}
	return nil
}
