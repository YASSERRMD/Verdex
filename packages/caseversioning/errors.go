package caseversioning

import (
	"errors"
	"fmt"

	"github.com/google/uuid"
)

// Sentinel errors that callers can test with errors.Is.
var (
	// ErrNilSnapshot is returned when a nil *Snapshot is passed to
	// Validate or a Repository/Service method that requires one.
	ErrNilSnapshot = errors.New("caseversioning: snapshot must not be nil")

	// ErrEmptyCaseID is returned when a Snapshot carries a zero CaseID.
	ErrEmptyCaseID = errors.New("caseversioning: case id is required")

	// ErrEmptyTenantID is returned when an operation is called with, or
	// a Snapshot carries, a zero tenant ID.
	ErrEmptyTenantID = errors.New("caseversioning: tenant id is required")

	// ErrInvalidArtifactKind is returned when a Snapshot's ArtifactKind
	// is not one of the recognized ArtifactKind constants.
	ErrInvalidArtifactKind = errors.New("caseversioning: invalid artifact kind")

	// ErrEmptyCreatedBy is returned when a Snapshot is persisted without
	// a CreatedBy actor.
	ErrEmptyCreatedBy = errors.New("caseversioning: created_by is required")

	// ErrNilRepository is returned by constructors and helpers that
	// require a non-nil Repository.
	ErrNilRepository = errors.New("caseversioning: repository must not be nil")

	// ErrNilCaseRepository is returned by NewService when constructed
	// without a caselifecycle.Repository.
	ErrNilCaseRepository = errors.New("caseversioning: case repository must not be nil")

	// ErrUnauthenticated is returned when an operation requiring an
	// actor is called with a context carrying no authenticated
	// identity.User.
	ErrUnauthenticated = errors.New("caseversioning: unauthenticated request")

	// ErrForbidden is returned when the authenticated actor lacks the
	// permission required for the requested operation.
	ErrForbidden = errors.New("caseversioning: actor lacks required permission")

	// ErrNotFound is returned by Repository.Get and Service methods when
	// no snapshot matches the requested ID (or the tenant scope hides
	// it).
	ErrNotFound = errors.New("caseversioning: snapshot not found")

	// ErrCrossTenantAccess is returned by Repository methods when asked
	// to operate on a Snapshot whose TenantID does not match the
	// scope's tenantID, mirroring packages/annotations's and
	// packages/caselifecycle's ErrCrossTenantAccess exactly.
	ErrCrossTenantAccess = errors.New("caseversioning: cross-tenant access denied")

	// ErrMismatchedCase is returned by Diff when snapshotA and
	// snapshotB do not belong to the same CaseID.
	ErrMismatchedCase = errors.New("caseversioning: snapshots belong to different cases")

	// ErrMismatchedArtifactKind is returned by Diff when snapshotA and
	// snapshotB have different ArtifactKind values.
	ErrMismatchedArtifactKind = errors.New("caseversioning: snapshots have different artifact kinds")

	// ErrNotRestorable is returned by Restore when called against a
	// Snapshot whose ArtifactKind does not support restore (only
	// ArtifactCaseMetadata does today).
	ErrNotRestorable = errors.New("caseversioning: snapshot's artifact kind does not support restore")

	// ErrNoPayload is returned when a Snapshot required to carry a
	// payload (e.g. a case-metadata snapshot, for diff/restore) has a
	// nil Payload.
	ErrNoPayload = errors.New("caseversioning: snapshot has no payload")
)

// wrapf mirrors the fmt.Errorf("pkg: fn: %w", err) convention used
// throughout this repository's packages.
func wrapf(fn string, err error) error {
	return fmt.Errorf("caseversioning: %s: %w", fn, err)
}

// requireMatchingTenant returns ErrCrossTenantAccess if entityTenantID
// is set and does not equal scopeTenantID, mirroring
// packages/annotations's and packages/caselifecycle's unexported helper
// of the same name and behavior. A nil entityTenantID (the zero
// uuid.UUID) is treated as "not yet assigned" and is not an error here.
func requireMatchingTenant(scopeTenantID, entityTenantID uuid.UUID) error {
	if entityTenantID != uuid.Nil && entityTenantID != scopeTenantID {
		return ErrCrossTenantAccess
	}
	return nil
}
