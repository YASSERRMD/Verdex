package grounding

import (
	"context"

	"github.com/YASSERRMD/verdex/packages/identity"
)

// requiredPermission is the identity.Permission required to run a
// grounding check over a case's opinion. A Report exposes exactly the
// same case-scoped reasoning content (conclusion text, supporting node
// IDs, citations) that packages/reasoningtrace and packages/knowledgeapi
// already gate on identity.PermViewCase, so this package reuses that
// permission rather than inventing a new role/permission vocabulary.
const requiredPermission = identity.PermViewCase

// RequireCheckPermission checks that ctx carries an authenticated
// identity.User who holds identity.PermViewCase, mirroring
// packages/reasoningtrace.RequireViewPermission and
// packages/knowledgeapi's own authorize-then-proceed pattern. Check calls
// this before reading a single irac.Node or citation.Finding, so an
// unauthorized caller never even triggers a GraphStore read.
//
// Returns ErrUnauthenticated if no user is present on ctx, or
// ErrForbidden if the user lacks the permission.
func RequireCheckPermission(ctx context.Context) error {
	user, ok := identity.UserFromContext(ctx)
	if !ok {
		return ErrUnauthenticated
	}
	if !user.HasPermission(requiredPermission) {
		return ErrForbidden
	}
	return nil
}
