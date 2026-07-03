package dataresidency

import (
	"errors"
	"fmt"
)

// Sentinel errors that callers can test with errors.Is.
var (
	// ErrNilPolicy is returned when a nil *ResidencyPolicy is passed to
	// a function that requires one.
	ErrNilPolicy = errors.New("dataresidency: policy must not be nil")

	// ErrEmptyDeploymentID is returned when an operation is called
	// with a zero deployment ID.
	ErrEmptyDeploymentID = errors.New("dataresidency: deployment id is required")

	// ErrEmptyRegion is returned when a region string is required but
	// empty -- an undeclared region can never be proven compliant, so
	// this package fails closed rather than treating "" as a wildcard.
	ErrEmptyRegion = errors.New("dataresidency: region is required")

	// ErrRegionNotAllowed is returned by CheckTransfer and the
	// provider-locality guard when the destination/provider region is
	// not present in the policy's AllowedRegions.
	ErrRegionNotAllowed = errors.New("dataresidency: destination region is not in the allowed list")

	// ErrStrictModeViolation is returned when StrictMode is set and an
	// operation would cross any region boundary at all, even one that
	// would otherwise be permitted by AllowedRegions.
	ErrStrictModeViolation = errors.New("dataresidency: strict mode forbids any cross-region operation")

	// ErrAirGappedRequiresLocalFlag is returned when the air-gapped
	// preset is applied but the caller has not also set the
	// corresponding router.RoutingPolicy.AirGappedOnly flag -- the two
	// must be composed together, not applied independently.
	ErrAirGappedRequiresLocalFlag = errors.New("dataresidency: air-gapped preset requires router's AirGappedOnly flag to be set")

	// ErrStorageRegionMismatch is returned by RegionPin.Validate (and
	// surfaced by Verify) when the configured database host does not
	// resolve to the pinned region.
	ErrStorageRegionMismatch = errors.New("dataresidency: configured storage host does not match the pinned region")

	// ErrEmptyDSN is returned when RegionPin.Validate is called with an
	// empty database DSN.
	ErrEmptyDSN = errors.New("dataresidency: database DSN is required")

	// ErrNilStore is returned by constructors that require a non-nil
	// audit store.
	ErrNilStore = errors.New("dataresidency: audit store must not be nil")

	// ErrUnauthenticated is returned when an operation requiring an
	// actor is called with a context carrying no authenticated
	// identity.User.
	ErrUnauthenticated = errors.New("dataresidency: unauthenticated request")

	// ErrForbidden is returned when the authenticated actor lacks the
	// identity.Permission a residency operation requires.
	ErrForbidden = errors.New("dataresidency: actor lacks required permission")

	// ErrNilVerifier is returned by Verify's dependencies when a
	// required lookup function is nil.
	ErrNilVerifier = errors.New("dataresidency: verifier dependency must not be nil")

	// errEmptyDataClass is an internal validation error for a
	// DataClassRule with a blank DataClass name.
	errEmptyDataClass = errors.New("data class rule has an empty data class name")

	// errNoHostPatterns is an internal validation error for a RegionPin
	// with no host patterns to match against.
	errNoHostPatterns = errors.New("region pin has no host patterns")

	// errNoHostInDSN is an internal error for a DSN that parses
	// successfully but carries no host component.
	errNoHostInDSN = errors.New("dsn has no host component")
)

// wrapf mirrors the fmt.Errorf("pkg: fn: %w", err) convention used
// throughout this repository's packages.
func wrapf(fn string, err error) error {
	return fmt.Errorf("dataresidency: %s: %w", fn, err)
}
