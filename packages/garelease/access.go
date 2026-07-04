package garelease

import (
	"context"

	"github.com/YASSERRMD/verdex/packages/identity"
)

// managePermission is the identity.Permission required to freeze a
// ReleaseCandidate, cut a Release, or run the guardrail/audit
// verification and post-release checklist operations.
const managePermission = identity.PermManageRelease

// viewPermission is the identity.Permission required for read-only
// access to readiness snapshots, release candidates, and releases.
const viewPermission = identity.PermViewRelease

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
// can manage release readiness can also read it. Unlike
// authorizeManage/authorizeActor, authorizeView returns only an error,
// not the resolved *identity.User: every read-only operation this
// package gates with authorizeView (CheckReadiness,
// GetReleaseCandidate, ListReleaseCandidates, GetRelease, ListReleases)
// operates on platform-global data with no tenant to match against
// (see types.go's ReleaseCandidate/Release doc comments), so there is
// no further use for the actor's identity once the permission check
// itself has passed -- unlike packages/compliance's identically-named
// authorizeView, whose tenant-scoped callers need the returned user's
// TenantID for requireMatchingUserTenant.
func authorizeView(ctx context.Context) error {
	user, err := authorizeActor(ctx)
	if err != nil {
		return err
	}
	if !user.HasPermission(viewPermission) && !user.HasPermission(managePermission) {
		return ErrForbidden
	}
	return nil
}
