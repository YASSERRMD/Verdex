package auditlog

import (
	"context"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/identity"
)

// readPermission is the identity.Permission required to Query, Export,
// or VerifyTenantChain the audit trail (task 8). identity.PermAuditRead
// is already granted to RoleJudge, RoleAdmin, and RoleAuditor by
// packages/identity's PermissionMatrix — this package does not
// introduce a new permission, since PermAuditRead's doc comment
// already describes exactly this capability ("reading the immutable
// audit trail, system event logs, and aggregate compliance reports").
const readPermission = identity.PermAuditRead

// authorizeAuditRead checks that ctx carries an authenticated
// identity.User who holds readPermission. Returns ErrUnauthenticated if
// no user is present on ctx, or ErrForbidden if the user lacks the
// permission.
func authorizeAuditRead(ctx context.Context) (*identity.User, error) {
	user, ok := identity.UserFromContext(ctx)
	if !ok {
		return nil, ErrUnauthenticated
	}
	if !user.HasPermission(readPermission) {
		return nil, ErrForbidden
	}
	return user, nil
}

// requireMatchingUserTenant returns ErrCrossTenantAccess if user's
// TenantID does not match tenantID — an audit query is always scoped
// to the actor's own tenant, never another tenant's, even for an
// auditor or admin.
func requireMatchingUserTenant(user *identity.User, tenantID uuid.UUID) error {
	if user.TenantID != tenantID {
		return ErrCrossTenantAccess
	}
	return nil
}
