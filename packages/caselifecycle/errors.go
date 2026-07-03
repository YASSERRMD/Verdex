package caselifecycle

import (
	"errors"
	"fmt"

	"github.com/google/uuid"
)

// Sentinel errors that callers can test with errors.Is.
var (
	// ErrNotFound is returned by Repository Get/Transition/Metadata
	// methods when no case matches the requested id (or the tenant
	// scope hides it — see ErrCrossTenantAccess for the distinction
	// between "does not exist" and "exists but is not yours").
	ErrNotFound = errors.New("caselifecycle: case not found")

	// ErrCrossTenantAccess is returned by Repository methods when
	// asked to operate on a Case whose TenantID does not match the
	// scope's tenantID, mirroring packages/tenancy.ErrCrossTenantAccess.
	// This check runs before any database access.
	ErrCrossTenantAccess = errors.New("caselifecycle: cross-tenant access denied")

	// ErrInvalidCase is returned when a Case fails validation (missing
	// required fields such as TenantID, JurisdictionID, CategoryID, or
	// Title).
	ErrInvalidCase = errors.New("caselifecycle: invalid case")

	// ErrIllegalTransition is returned by Transition when the
	// requested from-state to to-state transition is not present in
	// allowedTransitions.
	ErrIllegalTransition = errors.New("caselifecycle: illegal state transition")

	// ErrUnauthenticated is returned when an operation requiring an
	// actor is called with a context carrying no authenticated
	// identity.User.
	ErrUnauthenticated = errors.New("caselifecycle: unauthenticated request")

	// ErrForbidden is returned when the authenticated actor lacks the
	// permission required for the requested operation.
	ErrForbidden = errors.New("caselifecycle: actor lacks required permission")

	// ErrReasonRequired is returned by Reopen when called with a blank
	// justification: reopening a closed case must always be
	// self-documenting in the audit log.
	ErrReasonRequired = errors.New("caselifecycle: a reason/justification is required")

	// ErrActionNotPermitted is returned by RequireAction (and may be
	// returned by callers using CanPerform directly) when an operation
	// is not permitted for a case's current State.
	ErrActionNotPermitted = errors.New("caselifecycle: action not permitted in current case state")

	// ErrInvalidMetadataKey is returned by SetMetadata/MergeMetadata
	// when passed a blank metadata key.
	ErrInvalidMetadataKey = errors.New("caselifecycle: metadata key must not be blank")

	// ErrMetadataVersionConflict is returned by SetMetadata/MergeMetadata
	// when the caller's expected version does not match the case's
	// current metadata version, signaling a lost-update race.
	ErrMetadataVersionConflict = errors.New("caselifecycle: metadata version conflict")

	// ErrNilRepository is returned by constructors/functions that
	// require a non-nil Repository.
	ErrNilRepository = errors.New("caselifecycle: repository must not be nil")
)

// requireMatchingTenant returns ErrCrossTenantAccess if entityTenantID
// is set and does not equal scopeTenantID, mirroring
// packages/tenancy's unexported helper of the same name/behavior. A
// nil entityTenantID (the zero uuid.UUID) is treated as "not yet
// assigned" and is not an error here.
func requireMatchingTenant(scopeTenantID, entityTenantID uuid.UUID) error {
	if entityTenantID != uuid.Nil && entityTenantID != scopeTenantID {
		return ErrCrossTenantAccess
	}
	return nil
}

// wrapf is a small helper mirroring the fmt.Errorf("pkg: fn: %w", err)
// convention used throughout this repository's packages.
func wrapf(fn string, err error) error {
	return fmt.Errorf("caselifecycle: %s: %w", fn, err)
}
