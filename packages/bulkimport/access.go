package bulkimport

import (
	"context"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/identity"
)

// managePermission is the identity.Permission required to register an
// ImportJob, run a batch, pause/resume a job, or roll one back.
const managePermission = identity.PermManageBulkImport

// viewPermission is the identity.Permission required for read-only
// access to import job history, per-record outcomes, and progress.
const viewPermission = identity.PermViewBulkImport

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

// authorizeView checks that ctx carries an authenticated identity.User
// who holds either viewPermission or managePermission -- everyone who
// can run/roll back an import can also read its history.
func authorizeView(ctx context.Context) (*identity.User, error) {
	user, err := authorizeActor(ctx)
	if err != nil {
		return nil, err
	}
	if !user.HasPermission(viewPermission) && !user.HasPermission(managePermission) {
		return nil, ErrForbidden
	}
	return user, nil
}

// requireMatchingUserTenant returns ErrCrossTenantAccess if user's
// TenantID does not match tenantID, mirroring
// packages/privacy.requireMatchingUserTenant and
// packages/compliance's equivalent exactly -- an actor's role-level
// permission never lets them reach past their own tenant.
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

// actorFromCtx resolves the actor's user ID from ctx if present,
// falling back to uuid.Nil (which actorFor renders as systemActor)
// when ctx carries no authenticated user -- used by audit-on-failure
// paths, which must still record an event even when authorizeManage
// itself failed (e.g. ErrUnauthenticated).
func actorFromCtx(ctx context.Context) uuid.UUID {
	user, err := authorizeActor(ctx)
	if err != nil {
		return uuid.Nil
	}
	return user.ID
}
