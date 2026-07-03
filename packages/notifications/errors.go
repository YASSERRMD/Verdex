package notifications

import (
	"errors"
	"fmt"

	"github.com/google/uuid"
)

// Sentinel errors that callers can test with errors.Is.
var (
	// ErrNilNotification is returned when a nil *Notification is passed
	// to Validate or a Repository/Service method that requires one.
	ErrNilNotification = errors.New("notifications: notification must not be nil")

	// ErrNilPreference is returned when a nil *Preference is passed to
	// Validate or a Repository/Service method that requires one.
	ErrNilPreference = errors.New("notifications: preference must not be nil")

	// ErrEmptyTenantID is returned when an operation is called with, or
	// an entity carries, a zero tenant ID.
	ErrEmptyTenantID = errors.New("notifications: tenant id is required")

	// ErrEmptyRecipientID is returned when a Notification/Preference is
	// persisted without a recipient/user actor.
	ErrEmptyRecipientID = errors.New("notifications: recipient id is required")

	// ErrInvalidKind is returned when a Kind is not one of the
	// recognized Kind constants.
	ErrInvalidKind = errors.New("notifications: invalid notification kind")

	// ErrInvalidChannel is returned when a Channel is not one of the
	// recognized Channel constants.
	ErrInvalidChannel = errors.New("notifications: invalid delivery channel")

	// ErrNilRepository is returned by constructors that require a
	// non-nil Repository.
	ErrNilRepository = errors.New("notifications: repository must not be nil")

	// ErrNilPreferenceRepository is returned by constructors that
	// require a non-nil PreferenceRepository.
	ErrNilPreferenceRepository = errors.New("notifications: preference repository must not be nil")

	// ErrUnauthenticated is returned when an operation requiring an
	// actor is called with a context carrying no authenticated
	// identity.User.
	ErrUnauthenticated = errors.New("notifications: unauthenticated request")

	// ErrForbidden is returned when the authenticated actor requests
	// another user's notifications or preferences.
	ErrForbidden = errors.New("notifications: actor may only access their own notifications")

	// ErrNotFound is returned by Repository.Get and Service methods
	// when no notification matches the requested ID (or the tenant/
	// recipient scope hides it).
	ErrNotFound = errors.New("notifications: notification not found")

	// ErrCrossTenantAccess is returned by Repository methods when asked
	// to operate on a Notification/Preference whose TenantID does not
	// match the scope's tenantID, mirroring
	// packages/caseversioning.ErrCrossTenantAccess exactly.
	ErrCrossTenantAccess = errors.New("notifications: cross-tenant access denied")
)

// wrapf mirrors the fmt.Errorf("pkg: fn: %w", err) convention used
// throughout this repository's packages.
func wrapf(fn string, err error) error {
	return fmt.Errorf("notifications: %s: %w", fn, err)
}

// requireMatchingTenant returns ErrCrossTenantAccess if entityTenantID
// is set and does not equal scopeTenantID, mirroring
// packages/caseversioning's unexported helper of the same name and
// behavior. A nil entityTenantID (the zero uuid.UUID) is treated as
// "not yet assigned" and is not an error here.
func requireMatchingTenant(scopeTenantID, entityTenantID uuid.UUID) error {
	if entityTenantID != uuid.Nil && entityTenantID != scopeTenantID {
		return ErrCrossTenantAccess
	}
	return nil
}
