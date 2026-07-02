package knowledgeapi

import (
	"context"

	"github.com/YASSERRMD/verdex/packages/identity"
)

// requiredPermission is the identity.Permission every KnowledgeAPI read
// method requires. Every method exposed by this package reads case
// knowledge (tree data, retrieval results, citations, validation status),
// so every method is gated on the same permission identity already uses
// for reading case materials, rather than this package inventing a new
// role/permission vocabulary. See doc/knowledge-api.md for why
// PermViewCase is the correct gate for a read-only knowledge facade.
const requiredPermission = identity.PermViewCase

// authorize checks that ctx carries an authenticated identity.User who
// holds requiredPermission, per this package's RBAC model. It returns
// ErrUnauthenticated if no user is present on ctx, or ErrForbidden if the
// user lacks the permission.
//
// authorize is a complementary, independent gate from
// knowledgeisolation's case/tenant boundary checks: this function decides
// "should this actor see any case knowledge at all," while every
// downstream call into a knowledgeisolation-wrapped store separately
// decides "can this specific data cross a case boundary." A caller must
// clear both.
//
// authorize deliberately returns only an error, not the authenticated
// identity.User: every current KnowledgeAPI method only needs the
// pass/fail outcome. A future method that needs the actor itself (e.g.
// for per-actor audit logging) should call identity.UserFromContext(ctx)
// directly after authorize succeeds, rather than this helper's signature
// growing an unused return value for callers that do not need it.
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
