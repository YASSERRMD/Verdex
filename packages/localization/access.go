package localization

import (
	"context"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/identity"
)

// authorizeActor resolves the authenticated identity.User from ctx,
// returning ErrUnauthenticated if none is present. Mirrors
// packages/compliance.authorizeActor and packages/privacy's own
// equivalent exactly.
func authorizeActor(ctx context.Context) (*identity.User, error) {
	user, ok := identity.UserFromContext(ctx)
	if !ok {
		return nil, ErrUnauthenticated
	}
	return user, nil
}

// requireMatchingUserTenant returns ErrCrossTenantAccess if user's
// TenantID does not match tenantID, mirroring
// packages/compliance.requireMatchingUserTenant and
// packages/privacy.requireMatchingUserTenant exactly -- a user may
// only ever set or read their own tenant's data, regardless of any
// permission they hold.
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

// requireSelfOrManage returns ErrForbidden unless actor is the same
// user as targetUserID or holds identity.PermManageUsers.
//
// This package deliberately does not add a new identity.Permission
// constant (see doc.go): a locale preference is a self-service user
// setting an authenticated actor sets for themselves -- the natural
// authorization rule is "you may always set your own preference", not
// "you need a dedicated capability to set anyone's preference". An
// actor managing another user's account (identity.PermManageUsers,
// already used for role/status changes -- see
// packages/identity/permission.go) may also set that user's
// preference, e.g. for an administrator provisioning a new user's
// default locale.
func requireSelfOrManage(actor *identity.User, targetUserID uuid.UUID) error {
	if actor == nil {
		return ErrUnauthenticated
	}
	if actor.ID == targetUserID {
		return nil
	}
	if actor.HasPermission(identity.PermManageUsers) {
		return nil
	}
	return ErrForbidden
}
