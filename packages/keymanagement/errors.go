package keymanagement

import (
	"errors"
	"fmt"
)

// Sentinel errors that callers can test with errors.Is.
var (
	// ErrNilKeyMetadata is returned when a nil *KeyMetadata is passed to
	// Validate or a Repository/Provider method that requires one.
	ErrNilKeyMetadata = errors.New("keymanagement: key metadata must not be nil")

	// ErrEmptyTenantID is returned when an operation is called with, or
	// an entity carries, a zero tenant ID.
	ErrEmptyTenantID = errors.New("keymanagement: tenant id is required")

	// ErrEmptyKeyID is returned when a KeyMetadata is persisted or
	// looked up with an empty ID.
	ErrEmptyKeyID = errors.New("keymanagement: key id is required")

	// ErrInvalidKeyState is returned when a KeyState is not one of the
	// recognized KeyState constants.
	ErrInvalidKeyState = errors.New("keymanagement: invalid key state")

	// ErrNilRepository is returned by constructors that require a
	// non-nil Repository.
	ErrNilRepository = errors.New("keymanagement: repository must not be nil")

	// ErrNilProvider is returned by constructors/adapters that require
	// a non-nil Provider.
	ErrNilProvider = errors.New("keymanagement: provider must not be nil")

	// ErrNotFound is returned by Repository.Get and Provider methods
	// when no key matches the requested ID (or the tenant scope hides
	// it).
	ErrNotFound = errors.New("keymanagement: key not found")

	// ErrNoActiveKey is returned by CurrentKey when a tenant has no key
	// in the Active state — a fail-closed condition, never silently
	// falling back to some other tenant's key or a hardcoded default.
	ErrNoActiveKey = errors.New("keymanagement: tenant has no active key")

	// ErrCrossTenantAccess is returned by Repository/Provider methods
	// when asked to operate on a KeyMetadata (or key material) whose
	// TenantID does not match the scope's tenantID, mirroring
	// packages/caseversioning.ErrCrossTenantAccess and
	// packages/notifications.ErrCrossTenantAccess exactly.
	ErrCrossTenantAccess = errors.New("keymanagement: cross-tenant access denied")

	// ErrUnauthenticated is returned when an operation requiring an
	// actor is called with a context carrying no authenticated
	// identity.User.
	ErrUnauthenticated = errors.New("keymanagement: unauthenticated request")

	// ErrForbidden is returned when the authenticated actor lacks the
	// identity.Permission a key operation requires.
	ErrForbidden = errors.New("keymanagement: actor lacks required permission")

	// ErrJustificationRequired is returned by the break-glass path when
	// no (or a blank) justification string is supplied — every
	// emergency access must be explicitly and legibly justified.
	ErrJustificationRequired = errors.New("keymanagement: break-glass access requires a justification")

	// ErrBreakGlassExpired is returned when a break-glass grant is used
	// after its time-bound window has elapsed.
	ErrBreakGlassExpired = errors.New("keymanagement: break-glass grant has expired")

	// ErrBreakGlassNotFound is returned when a break-glass grant ID does
	// not resolve to any recorded grant for the tenant.
	ErrBreakGlassNotFound = errors.New("keymanagement: break-glass grant not found")

	// ErrInvalidKeyMaterial is returned when key material read from a
	// Provider backend is missing or the wrong length for AES-256 (see
	// encryption.KeyBytes).
	ErrInvalidKeyMaterial = errors.New("keymanagement: invalid key material")

	// ErrOfflineStoreNotFound is returned by FileProvider when its
	// backing directory or an expected key file does not exist.
	ErrOfflineStoreNotFound = errors.New("keymanagement: offline key store path not found")
)

// wrapf mirrors the fmt.Errorf("pkg: fn: %w", err) convention used
// throughout this repository's packages.
func wrapf(fn string, err error) error {
	return fmt.Errorf("keymanagement: %s: %w", fn, err)
}
