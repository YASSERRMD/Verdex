package reportexport

import (
	"errors"
	"fmt"

	"github.com/google/uuid"
)

// Sentinel errors that callers can test with errors.Is.
var (
	// ErrNilCase is returned when Assemble is called with a nil Case.
	ErrNilCase = errors.New("reportexport: case must not be nil")

	// ErrNilOpinion is returned when Assemble is called with a nil
	// Opinion.
	ErrNilOpinion = errors.New("reportexport: opinion must not be nil")

	// ErrEmptyTenantID is returned when an operation is called with a
	// zero tenant ID.
	ErrEmptyTenantID = errors.New("reportexport: tenant id is required")

	// ErrInvalidFormat is returned when Format is not one of the
	// recognized Format constants.
	ErrInvalidFormat = errors.New("reportexport: invalid export format")

	// ErrUnauthenticated is returned when an operation requiring an
	// actor is called with a context carrying no authenticated
	// identity.User.
	ErrUnauthenticated = errors.New("reportexport: unauthenticated request")

	// ErrForbidden is returned when the authenticated actor lacks the
	// permission required to export a case's report.
	ErrForbidden = errors.New("reportexport: actor lacks permission to export this case")

	// ErrCrossTenantAccess is returned when a caller attempts to
	// export or query audit records across tenant boundaries.
	ErrCrossTenantAccess = errors.New("reportexport: cross-tenant access denied")

	// ErrNilRepository is returned by constructors that require a
	// non-nil AuditRepository.
	ErrNilRepository = errors.New("reportexport: audit repository must not be nil")

	// ErrNilRecord is returned when a nil *AuditRecord is passed to a
	// repository method that requires one.
	ErrNilRecord = errors.New("reportexport: audit record must not be nil")
)

// wrapf mirrors the fmt.Errorf("pkg: fn: %w", err) convention used
// throughout this repository's packages.
func wrapf(fn string, err error) error {
	return fmt.Errorf("reportexport: %s: %w", fn, err)
}

// requireMatchingTenant returns ErrCrossTenantAccess if entityTenantID
// is set and does not equal scopeTenantID, mirroring
// packages/notifications's unexported helper of the same name and
// behavior. A nil entityTenantID (the zero uuid.UUID) is treated as
// "not yet assigned" and is not an error here.
func requireMatchingTenant(scopeTenantID, entityTenantID uuid.UUID) error {
	if entityTenantID != uuid.Nil && entityTenantID != scopeTenantID {
		return ErrCrossTenantAccess
	}
	return nil
}
