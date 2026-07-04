package alerting

import (
	"context"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/identity"
)

// managePermission is the identity.Permission required to register or
// update an AlertRule, set an EscalationPolicy, or run a
// SyntheticCheck on demand.
const managePermission = identity.PermManageAlerting

// viewPermission is the identity.Permission required for read-only
// access to the alert-rule catalogue, alert-event history, escalation
// policies, and dashboards.
const viewPermission = identity.PermViewAlerting

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
// can manage alert rules can also read them.
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
// packages/compliance.requireMatchingUserTenant exactly -- an actor's
// role-level permission never lets them reach past their own tenant.
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
