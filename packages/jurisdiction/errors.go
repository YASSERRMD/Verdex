package jurisdiction

import "errors"

// Sentinel errors returned by the jurisdiction package.
var (
	// ErrJurisdictionNotFound is returned when a requested jurisdiction does not
	// exist in the registry or repository.
	ErrJurisdictionNotFound = errors.New("jurisdiction not found")

	// ErrInvalidJurisdiction is returned when a Jurisdiction value fails
	// structural or business-rule validation.
	ErrInvalidJurisdiction = errors.New("invalid jurisdiction")

	// ErrDuplicateJurisdiction is returned when an attempt is made to create a
	// jurisdiction that already exists (same country code + court name combination).
	ErrDuplicateJurisdiction = errors.New("duplicate jurisdiction")

	// ErrCountryCodeInvalid is returned when the supplied ISO 3166-1 alpha-2
	// country code is not exactly two uppercase ASCII letters.
	ErrCountryCodeInvalid = errors.New("country code must be a 2-letter ISO 3166-1 alpha-2 code")
)
