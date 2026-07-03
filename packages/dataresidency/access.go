package dataresidency

import (
	"context"

	"github.com/YASSERRMD/verdex/packages/identity"
)

// managePermission is the identity.Permission required to define or
// change a deployment's ResidencyPolicy/RegionPin, and to invoke
// Verify. There is no dedicated "residency:manage" permission in
// identity's matrix -- residency policy is deployment/tenant
// configuration, the same category packages/keymanagement's access.go
// and packages/config's deployment-profile settings already gate on
// identity.PermManageSettings, so this package reuses it rather than
// adding a parallel permission for the same concept.
const managePermission = identity.PermManageSettings

// authorizeManage checks that ctx carries an authenticated
// identity.User who holds managePermission. Returns ErrUnauthenticated
// if no user is present on ctx, or ErrForbidden if the user lacks the
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
