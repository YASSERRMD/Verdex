package annotations

import (
	"errors"
	"fmt"

	"github.com/google/uuid"
)

// Sentinel errors that callers can test with errors.Is.
var (
	// ErrNilAnnotation is returned when a nil *Annotation is passed to
	// Validate or a Repository method that requires one.
	ErrNilAnnotation = errors.New("annotations: annotation must not be nil")

	// ErrEmptyCaseID is returned when an Annotation carries a zero
	// CaseID.
	ErrEmptyCaseID = errors.New("annotations: case id is required")

	// ErrEmptyTenantID is returned when an operation is called with, or
	// an Annotation carries, a zero tenant ID.
	ErrEmptyTenantID = errors.New("annotations: tenant id is required")

	// ErrEmptyBody is returned when an Annotation's Body is blank.
	ErrEmptyBody = errors.New("annotations: body is required")

	// ErrInvalidAnchorType is returned when an Annotation's AnchorType
	// is not one of the recognized AnchorType constants.
	ErrInvalidAnchorType = errors.New("annotations: invalid anchor type")

	// ErrEmptyAnchorID is returned when AnchorTreeNode or
	// AnchorEvidenceSegment is set with a blank AnchorID.
	ErrEmptyAnchorID = errors.New("annotations: anchor id is required for this anchor type")

	// ErrUnexpectedAnchorID is returned when AnchorCase is set with a
	// non-blank AnchorID.
	ErrUnexpectedAnchorID = errors.New("annotations: anchor id must be empty for case-level annotations")

	// ErrNilRepository is returned by constructors and helpers that
	// require a non-nil Repository.
	ErrNilRepository = errors.New("annotations: repository must not be nil")

	// ErrNilCaseReader is returned by NewService when constructed
	// without a CaseAccessReader.
	ErrNilCaseReader = errors.New("annotations: case access reader must not be nil")

	// ErrUnauthenticated is returned when an operation requiring an
	// actor is called with a context carrying no authenticated
	// identity.User.
	ErrUnauthenticated = errors.New("annotations: unauthenticated request")

	// ErrForbidden is returned when the authenticated actor lacks the
	// permission required for the requested operation.
	ErrForbidden = errors.New("annotations: actor lacks required permission")

	// ErrNotFound is returned by Repository.Get/Update/Delete/Resolve
	// when no annotation matches the requested ID (or the tenant scope
	// hides it).
	ErrNotFound = errors.New("annotations: annotation not found")

	// ErrCrossTenantAccess is returned by Repository methods when asked
	// to operate on an Annotation whose TenantID does not match the
	// scope's tenantID, mirroring packages/casesearch's and
	// packages/caselifecycle's ErrCrossTenantAccess exactly.
	ErrCrossTenantAccess = errors.New("annotations: cross-tenant access denied")

	// ErrNotAuthor is returned by Update/Delete when the acting user is
	// neither the annotation's AuthorID nor holds an edit-override
	// permission.
	ErrNotAuthor = errors.New("annotations: only the author may modify this annotation")

	// ErrParentNotFound is returned by Create when ParentID is set but
	// no such annotation exists (in the same case and tenant).
	ErrParentNotFound = errors.New("annotations: parent annotation not found")

	// ErrParentIsReply is returned by Create when ParentID references
	// an annotation that is itself a reply — this package supports one
	// level of threading (root + replies), matching Thread's
	// documented contract.
	ErrParentIsReply = errors.New("annotations: cannot reply to a reply; thread is one level deep")

	// ErrAlreadyResolved is returned by Resolve when the annotation is
	// already resolved.
	ErrAlreadyResolved = errors.New("annotations: annotation is already resolved")

	// ErrNotResolved is returned by Reopen when the annotation is not
	// currently resolved.
	ErrNotResolved = errors.New("annotations: annotation is not resolved")
)

// wrapf mirrors the fmt.Errorf("pkg: fn: %w", err) convention used
// throughout this repository's packages.
func wrapf(fn string, err error) error {
	return fmt.Errorf("annotations: %s: %w", fn, err)
}

// requireMatchingTenant returns ErrCrossTenantAccess if entityTenantID
// is set and does not equal scopeTenantID, mirroring
// packages/casesearch's and packages/caselifecycle's unexported helper
// of the same name and behavior. A nil entityTenantID (the zero
// uuid.UUID) is treated as "not yet assigned" and is not an error
// here.
func requireMatchingTenant(scopeTenantID, entityTenantID uuid.UUID) error {
	if entityTenantID != uuid.Nil && entityTenantID != scopeTenantID {
		return ErrCrossTenantAccess
	}
	return nil
}
