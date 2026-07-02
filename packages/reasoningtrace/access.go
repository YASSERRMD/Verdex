package reasoningtrace

import (
	"context"

	"github.com/YASSERRMD/verdex/packages/identity"
)

// requiredPermission is the identity.Permission required to read a
// case's reasoning trace. A Trace exposes every model call, tool call,
// citation, and draft conclusion the reasoning pipeline produced for a
// case, so it is gated on the same permission knowledgeapi already uses
// for reading case knowledge, rather than this package inventing a new
// role/permission vocabulary.
const requiredPermission = identity.PermViewCase

// RequireViewPermission checks that ctx carries an authenticated
// identity.User who holds identity.PermViewCase, mirroring
// packages/knowledgeapi's own authorize-then-proceed pattern. Build calls
// this before reading a single Checkpoint or assembling any part of a
// Trace, so an unauthorized caller never even causes a CheckpointStore
// read.
//
// Returns ErrUnauthenticated if no user is present on ctx, or
// ErrForbidden if the user lacks the permission.
func RequireViewPermission(ctx context.Context) error {
	user, ok := identity.UserFromContext(ctx)
	if !ok {
		return ErrUnauthenticated
	}
	if !user.HasPermission(requiredPermission) {
		return ErrForbidden
	}
	return nil
}
