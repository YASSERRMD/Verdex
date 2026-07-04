package accessgovernance

import (
	"context"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/identity"
)

// managePermission is the identity.Permission required to author or
// activate a Policy, and to create/revoke a CaseGrant. There is no
// dedicated "access_governance:manage" permission in identity's
// matrix, so this package reuses identity.PermManageSettings --
// exactly the precedent packages/dataresidency.authorizeManage
// establishes for deployment-level configuration that doesn't fit any
// more specific existing permission.
const managePermission = identity.PermManageSettings

// reviewPermission is the identity.Permission required to Attest a
// Review. Access review/attestation is an audit-adjacent
// administrative act, so this package gates it on PermAuditRead
// (read access to the trail an attestation itself becomes part of)
// combined with managePermission at the call site -- see Attest in
// review.go for the exact combination.
const reviewPermission = identity.PermAuditRead

// authorizeActor resolves the authenticated identity.User from ctx,
// returning ErrUnauthenticated if none is present.
func authorizeActor(ctx context.Context) (*identity.User, error) {
	user, ok := identity.UserFromContext(ctx)
	if !ok {
		return nil, ErrUnauthenticated
	}
	return user, nil
}

// authorizeManage checks that ctx carries an authenticated
// identity.User who holds managePermission.
func authorizeManage(ctx context.Context) (*identity.User, error) {
	user, err := authorizeActor(ctx)
	if err != nil {
		return nil, err
	}
	if !user.HasPermission(managePermission) {
		return nil, ErrForbidden
	}
	return user, nil
}

// requireMatchingUserTenant returns ErrCrossTenantAccess if user's
// TenantID does not match tenantID, mirroring
// packages/keymanagement.requireMatchingUserTenant exactly -- an
// actor's role-level permission never lets them reach past their own
// tenant.
func requireMatchingUserTenant(user *identity.User, tenantID uuid.UUID) error {
	if user == nil {
		return ErrUnauthenticated
	}
	if user.TenantID != tenantID {
		return ErrCrossTenantAccess
	}
	return nil
}

// requireMatchingTenant returns ErrCrossTenantAccess if want != got,
// used by store implementations to validate a record's TenantID
// against the tenantID scope it was addressed with.
func requireMatchingTenant(want, got uuid.UUID) error {
	if got != uuid.Nil && got != want {
		return ErrCrossTenantAccess
	}
	return nil
}

// actorLabel formats a best-effort actor string for audit recording,
// mirroring packages/keymanagement's actorLabel/currentActor
// convention.
func actorLabel(id uuid.UUID, ok bool) string {
	if !ok || id == uuid.Nil {
		return "unknown"
	}
	return id.String()
}
