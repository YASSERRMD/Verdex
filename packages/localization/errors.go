package localization

import (
	"errors"
	"fmt"
)

// Sentinel errors that callers can test with errors.Is.
var (
	// ErrUnauthenticated is returned when an operation requiring an
	// actor is called with a context carrying no authenticated
	// identity.User.
	ErrUnauthenticated = errors.New("localization: unauthenticated request")

	// ErrForbidden is returned when the authenticated actor lacks the
	// identity.Permission a localization operation requires.
	ErrForbidden = errors.New("localization: actor lacks required permission")

	// ErrCrossTenantAccess is returned when an operation would read or
	// write a record whose TenantID does not match the caller's scope,
	// mirroring packages/privacy.ErrCrossTenantAccess and
	// packages/compliance.ErrCrossTenantAccess.
	ErrCrossTenantAccess = errors.New("localization: cross-tenant access denied")

	// ErrEmptyTenantID is returned when an operation is called with a
	// zero tenant ID.
	ErrEmptyTenantID = errors.New("localization: tenant id is required")

	// ErrNilStore is returned by constructors that require a non-nil
	// backing store/repository.
	ErrNilStore = errors.New("localization: store must not be nil")

	// ErrNilAuditSink is returned by constructors that require a
	// non-nil audit sink composing with packages/auditlog.Store.
	ErrNilAuditSink = errors.New("localization: audit sink must not be nil")

	// ErrInvalidLocale is returned when a Locale value fails structural
	// validation or is not one of the seeded, recognized locales.
	ErrInvalidLocale = errors.New("localization: invalid or unrecognized locale")

	// ErrUnknownKey is returned by MustTranslate when a translation key
	// has no entry in any locale, including the fallback English
	// locale -- an authoring bug (see catalog.go), unlike an
	// individual-locale gap, which Translate resolves silently via
	// fallback.
	ErrUnknownKey = errors.New("localization: translation key not found in fallback locale")

	// ErrNilFormatter is returned when LocalizeCitation is called with
	// a nil citation.Formatter.
	ErrNilFormatter = errors.New("localization: citation formatter must not be nil")

	// ErrNilCatalog is returned when a function requiring a *Catalog is
	// called with nil.
	ErrNilCatalog = errors.New("localization: catalog must not be nil")

	// ErrInvalidPreference is returned when a Preference fails
	// structural validation.
	ErrInvalidPreference = errors.New("localization: invalid locale preference")

	// ErrPreferenceNotFound is returned when no Preference is on file
	// for a tenant/user pair.
	ErrPreferenceNotFound = errors.New("localization: locale preference not found")
)

// wrapf mirrors the fmt.Errorf("pkg: fn: %w", err) convention used
// throughout this repository's packages.
func wrapf(fn string, err error) error {
	return fmt.Errorf("localization: %s: %w", fn, err)
}
