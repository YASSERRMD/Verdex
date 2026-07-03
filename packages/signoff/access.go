package signoff

import (
	"context"

	"github.com/YASSERRMD/verdex/packages/identity"
)

// RequireSignoffPermission checks that ctx carries an authenticated
// identity.User who holds identity.PermSignOff, mirroring
// packages/caselifecycle.RequireEditPermission. Approve and Reject
// call this before performing any write.
//
// Returns ErrUnauthenticated if no user is present on ctx, or
// ErrForbidden if the user lacks the permission.
func RequireSignoffPermission(ctx context.Context) error {
	user, ok := identity.UserFromContext(ctx)
	if !ok {
		return ErrUnauthenticated
	}
	if !user.HasPermission(identity.PermSignOff) {
		return ErrForbidden
	}
	return nil
}

// RequireViewPermission checks that ctx carries an authenticated
// identity.User who holds identity.PermViewCase. Read-only operations
// (Get, History) should call this before returning sign-off data.
//
// Returns ErrUnauthenticated if no user is present on ctx, or
// ErrForbidden if the user lacks the permission.
func RequireViewPermission(ctx context.Context) error {
	user, ok := identity.UserFromContext(ctx)
	if !ok {
		return ErrUnauthenticated
	}
	if !user.HasPermission(identity.PermViewCase) {
		return ErrForbidden
	}
	return nil
}
