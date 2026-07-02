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
func authorize(ctx context.Context) (*identity.User, error) {
	user, ok := identity.UserFromContext(ctx)
	if !ok {
		return nil, ErrUnauthenticated
	}
	if !user.HasPermission(requiredPermission) {
		return nil, ErrForbidden
	}
	return user, nil
}
