package notifications

import (
	"context"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/identity"
)

// authorizeSelf checks that ctx carries an authenticated identity.User
// and that the requested userID matches that user's own ID — a
// notification inbox is always the caller's own, never another user's
// (there is no "view teammate's inbox" permission in this system), so
// this is a simpler check than packages/caseversioning's
// permission-matrix-based authorizeView/authorizeWrite. Adapters
// (adapters.go) that write notifications on behalf of an upstream
// event bypass this check entirely — see Service.Notify, which is
// deliberately not actor-gated, since it is called by trusted
// server-side event hooks, not directly by an end user.
func authorizeSelf(ctx context.Context, userID uuid.UUID) (*identity.User, error) {
	user, ok := identity.UserFromContext(ctx)
	if !ok {
		return nil, ErrUnauthenticated
	}
	if user.ID != userID {
		return nil, ErrForbidden
	}
	return user, nil
}
