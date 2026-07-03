package caselifecycle

import (
	"context"

	"github.com/YASSERRMD/verdex/packages/identity"
)

// RequireViewPermission checks that ctx carries an authenticated
// identity.User who holds identity.PermViewCase, mirroring
// packages/grounding.RequireCheckPermission. Read-only operations
// (Get, List, History) should call this before returning case data.
//
// Returns ErrUnauthenticated if no user is present on ctx, or
// ErrForbidden if the user lacks the permission.
func RequireViewPermission(ctx context.Context) error {
	user, ok := identity.UserFromContext(ctx)
	if !ok {
		return ErrUnauthenticated
	}
	if !user.HasPermission(identity.PermViewCase) {
		return ErrForbidden
	}
	return nil
}

// RequireEditPermission checks that ctx carries an authenticated
// identity.User who holds identity.PermEditCase. Mutating operations
// (Transition, Reopen, Archive, SetMetadata, MergeMetadata, and their
// bulk equivalents) should call this before performing any write.
//
// Returns ErrUnauthenticated if no user is present on ctx, or
// ErrForbidden if the user lacks the permission.
//
// Note: Transition, Reopen, Archive, SetMetadata, and MergeMetadata do
// not call this automatically — they only require ctx to carry *some*
// authenticated user (to attribute the TransitionRecord's Actor).
// Callers building an API layer on top of this package (e.g. an HTTP
// handler) are expected to call RequireEditPermission explicitly
// before invoking those functions, mirroring how
// packages/grounding.Check leaves the RequireCheckPermission call to
// its own caller rather than calling it internally.
func RequireEditPermission(ctx context.Context) error {
	user, ok := identity.UserFromContext(ctx)
	if !ok {
		return ErrUnauthenticated
	}
	if !user.HasPermission(identity.PermEditCase) {
		return ErrForbidden
	}
	return nil
}

// RequireDeletePermission checks that ctx carries an authenticated
// identity.User who holds identity.PermDeleteCase. Intended for a
// future hard-delete operation on Repository (not added by this
// phase); provided now so downstream packages/API layers gating
// destructive case operations have a single stable helper to call
// rather than re-deriving the permission check.
func RequireDeletePermission(ctx context.Context) error {
	user, ok := identity.UserFromContext(ctx)
	if !ok {
		return ErrUnauthenticated
	}
	if !user.HasPermission(identity.PermDeleteCase) {
		return ErrForbidden
	}
	return nil
}
