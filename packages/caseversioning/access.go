package caseversioning

import (
	"context"

	"github.com/YASSERRMD/verdex/packages/identity"
)

// viewPermission is the identity.Permission required to read version
// history, mirroring packages/annotations's viewPermission exactly:
// snapshots are case-scoped content, so reading them requires the same
// permission as reading the case itself.
const viewPermission = identity.PermViewCase

// writePermission is the identity.Permission required to record a
// snapshot or restore a prior version.
const writePermission = identity.PermEditCase

// authorizeView checks that ctx carries an authenticated identity.User
// who holds viewPermission. Returns ErrUnauthenticated if no user is
// present on ctx, or ErrForbidden if the user lacks the permission.
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

// authorizeWrite checks that ctx carries an authenticated identity.User
// who holds writePermission. Returns ErrUnauthenticated if no user is
// present on ctx, or ErrForbidden if the user lacks the permission.
func authorizeWrite(ctx context.Context) (*identity.User, error) {
	user, ok := identity.UserFromContext(ctx)
	if !ok {
		return nil, ErrUnauthenticated
	}
	if !user.HasPermission(writePermission) {
		return nil, ErrForbidden
	}
	return user, nil
}
