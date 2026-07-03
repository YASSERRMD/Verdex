package casesearch

import (
	"context"

	"github.com/YASSERRMD/verdex/packages/identity"
)

// requiredPermission is the identity.Permission every casesearch
// operation requires, mirroring packages/knowledgeapi's requiredPermission
// exactly: search is a read-only view over case knowledge, so it is
// gated on the same permission as any other case read.
const requiredPermission = identity.PermViewCase

// authorize checks that ctx carries an authenticated identity.User who
// holds requiredPermission. Returns ErrUnauthenticated if no user is
// present on ctx, or ErrForbidden if the user lacks the permission.
func authorize(ctx context.Context) error {
	user, ok := identity.UserFromContext(ctx)
	if !ok {
		return ErrUnauthenticated
	}
	if !user.HasPermission(requiredPermission) {
		return ErrForbidden
	}
	return nil
}
