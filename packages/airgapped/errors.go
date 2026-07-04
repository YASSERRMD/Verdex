package airgapped

import (
	"errors"
	"fmt"
)

// Sentinel errors that callers can test with errors.Is.
var (
	// ErrNilProfile is returned when a nil *Profile is passed to a
	// function that requires one.
	ErrNilProfile = errors.New("airgapped: profile must not be nil")

	// ErrResidencyNotAirGapped is returned when a Profile's
	// dataresidency.ResidencyPolicy does not match the shape produced
	// by dataresidency.AirGappedPreset (StrictMode true, no allowed
	// regions).
	ErrResidencyNotAirGapped = errors.New("airgapped: residency policy is not configured for air-gapped operation")

	// ErrRoutingNotAirGapped is returned when a Profile's
	// router.RoutingPolicy does not have AirGappedOnly set.
	ErrRoutingNotAirGapped = errors.New("airgapped: routing policy does not set AirGappedOnly")

	// ErrKeyProviderRequired is returned when a Profile has no
	// keymanagement.FileProvider configured as its key source.
	ErrKeyProviderRequired = errors.New("airgapped: a keymanagement.FileProvider is required as the mandated key source")

	// ErrDeploymentProfileRequired is returned when a Profile has no
	// underlying config.Profile name set.
	ErrDeploymentProfileRequired = errors.New("airgapped: a config deployment profile name is required")

	// ErrNonLocalProvider is returned when a provider registration
	// attempt (or an already-registered provider ID) does not carry
	// the "local:" prefix mandated for an air-gapped Profile.
	ErrNonLocalProvider = errors.New("airgapped: only local:-prefixed providers may be registered under an air-gapped profile")

	// ErrNilRegistry is returned by GuardRegistry when given a nil
	// provider.Registry.
	ErrNilRegistry = errors.New("airgapped: registry must not be nil")

	// ErrDisallowedAddress is returned by the NetworkPolicy guard when
	// an address is neither loopback nor explicitly allow-listed.
	ErrDisallowedAddress = errors.New("airgapped: address is not loopback and is not on the network policy allow-list")

	// ErrEmptyAddress is returned when the network guard is called
	// with an empty address string.
	ErrEmptyAddress = errors.New("airgapped: address must not be empty")

	// ErrEmptyBundlePath is returned when a bundle-consuming function
	// (ProvisionCorpus, ApplyUpdateBundle) is called with an empty
	// path.
	ErrEmptyBundlePath = errors.New("airgapped: bundle path must not be empty")

	// ErrBundleNotFound is returned when a bundle path does not exist
	// on disk.
	ErrBundleNotFound = errors.New("airgapped: bundle path does not exist")

	// ErrEmptyCorpusBundle is returned when a corpus bundle directory
	// contains no recognizable statute or precedent files.
	ErrEmptyCorpusBundle = errors.New("airgapped: corpus bundle contains no statute or precedent files")

	// ErrInvalidManifest is returned when an update bundle's manifest
	// file is missing or malformed.
	ErrInvalidManifest = errors.New("airgapped: update bundle manifest is missing or malformed")

	// ErrChecksumMismatch is returned when a bundle file's SHA-256
	// checksum does not match the manifest's declared checksum.
	ErrChecksumMismatch = errors.New("airgapped: bundle file checksum does not match the manifest")

	// ErrSignatureInvalid is returned when a bundle's, license's, or
	// activation's signature fails verification.
	ErrSignatureInvalid = errors.New("airgapped: signature verification failed")

	// ErrNilSigner is returned by constructors that require a non-nil
	// provenance.Signer.
	ErrNilSigner = errors.New("airgapped: signer must not be nil")

	// ErrLicenseExpired is returned when a LicenseKey's ExpiresAt has
	// already passed.
	ErrLicenseExpired = errors.New("airgapped: license key has expired")

	// ErrLicenseRevoked is returned when an Activation references a
	// LicenseKey marked revoked.
	ErrLicenseRevoked = errors.New("airgapped: license key has been revoked")
)

// wrapf mirrors the fmt.Errorf("pkg: fn: %w", err) convention used
// throughout this repository's packages.
func wrapf(fn string, err error) error {
	return fmt.Errorf("airgapped: %s: %w", fn, err)
}
