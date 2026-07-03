package reportexport

import (
	"context"

	"github.com/YASSERRMD/verdex/packages/identity"
)

// authorizeExport checks that ctx carries an authenticated
// identity.User holding identity.PermViewCase — the same permission
// that gates reading a case's details, filings, and attached documents
// in the first place (see packages/identity/permission.go). Exporting
// a report is a read of the case's analysis, not a mutation, so this
// package does not require a separate export-specific permission.
func authorizeExport(ctx context.Context) (*identity.User, error) {
	user, ok := identity.UserFromContext(ctx)
	if !ok {
		return nil, ErrUnauthenticated
	}
	if !user.HasPermission(identity.PermViewCase) {
		return nil, ErrForbidden
	}
	return user, nil
}
