package airgapped

import (
	"time"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/dataresidency"
	"github.com/YASSERRMD/verdex/packages/keymanagement"
	"github.com/YASSERRMD/verdex/packages/router"
)

// ConfigProfileName is the packages/config deployment-profile name
// (Phase 002's "per-deployment config profiles" pattern -- see
// packages/config/profile.go) that every air-gapped deployment is
// expected to select, normally via the VERDEX_PROFILE environment
// variable or config.WithProfile. This package does not introduce a
// new config-loading mechanism; it validates that the name a caller
// declares matches this constant, so a deployment cannot claim to be
// "the air-gapped tier" while actually running under some other named
// profile.
const ConfigProfileName = "airgapped"

// Profile is the air-gapped deployment profile: it composes the
// residency, routing, and key-management pieces that already exist
// (Phases 012, 076, 078) into one value that Validate checks for
// internal consistency before Conformance certifies it end to end.
// Profile itself stores no secrets and opens no connections; it is a
// declaration of which already-built components a deployment has
// wired together, mirroring how dataresidency.ResidencyPolicy composes
// with (rather than duplicates) packages/tenancy's deployment concept.
type Profile struct {
	// DeploymentID identifies the deployment this profile governs.
	DeploymentID uuid.UUID `json:"deployment_id"`

	// ConfigProfile is the packages/config deployment-profile name in
	// effect for this deployment. Must equal ConfigProfileName.
	ConfigProfile string `json:"config_profile"`

	// Residency is the dataresidency.ResidencyPolicy for this
	// deployment. Must match the shape produced by
	// dataresidency.AirGappedPreset (StrictMode true, no allowed
	// regions) -- see dataresidency.IsAirGappedPolicy.
	Residency dataresidency.ResidencyPolicy `json:"residency"`

	// Routing is the router.RoutingPolicy for this deployment. Must
	// have AirGappedOnly set to true.
	Routing router.RoutingPolicy `json:"routing"`

	// KeyProvider is the mandated offline key source: a
	// keymanagement.FileProvider. Declared here as a marker
	// (KeyProviderConfigured) rather than the live *FileProvider value
	// itself, since Profile is a lightweight, JSON-serializable
	// declaration and the live provider is constructed and owned by
	// the deployment's startup code -- see NewProfile, which accepts
	// the live *keymanagement.FileProvider and records that it was
	// supplied.
	KeyProviderConfigured bool `json:"key_provider_configured"`

	// AllowedNetworkTargets lists additional non-loopback host:port (or
	// bare host) targets that NetworkPolicy permits beyond loopback,
	// for deployments that run a local model server on a private LAN
	// address rather than localhost (see NetworkPolicy in
	// networkpolicy.go). Empty means loopback-only.
	AllowedNetworkTargets []string `json:"allowed_network_targets,omitempty"`

	// CreatedAt and UpdatedAt are informational bookkeeping fields,
	// mirroring dataresidency.ResidencyPolicy's convention.
	CreatedAt time.Time `json:"created_at,omitempty"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
}

// NewProfile builds a Profile from its constituent, already-built
// pieces: an air-gapped dataresidency.ResidencyPolicy, an air-gapped
// router.RoutingPolicy, and a live *keymanagement.FileProvider
// (required non-nil -- this is the "mandated key source" task 1
// calls for). It does not call Validate itself; callers should call
// Validate (or Conformance) before trusting the result.
func NewProfile(deploymentID uuid.UUID, residency dataresidency.ResidencyPolicy, routing router.RoutingPolicy, keyProvider *keymanagement.FileProvider, allowedNetworkTargets []string) *Profile {
	now := time.Now().UTC()
	return &Profile{
		DeploymentID:          deploymentID,
		ConfigProfile:         ConfigProfileName,
		Residency:             residency,
		Routing:               routing,
		KeyProviderConfigured: keyProvider != nil,
		AllowedNetworkTargets: allowedNetworkTargets,
		CreatedAt:             now,
		UpdatedAt:             now,
	}
}

// Validate checks p for internal consistency, rejecting a Profile
// whose components are not actually configured for offline operation
// (task 1's "reject if any component isn't actually configured for
// offline operation"). It does not reach out to any live system; see
// Conformance for the end-to-end check that also exercises the
// provider registry and network/zero-egress guards.
func (p *Profile) Validate() error {
	if p == nil {
		return ErrNilProfile
	}
	if p.DeploymentID == uuid.Nil {
		return wrapf("Validate", dataresidency.ErrEmptyDeploymentID)
	}
	if p.ConfigProfile != ConfigProfileName {
		return wrapf("Validate", ErrDeploymentProfileRequired)
	}
	if err := p.Residency.Validate(); err != nil {
		return wrapf("Validate", err)
	}
	if !dataresidency.IsAirGappedPolicy(&p.Residency) {
		return wrapf("Validate", ErrResidencyNotAirGapped)
	}
	if !p.Routing.AirGappedOnly {
		return wrapf("Validate", ErrRoutingNotAirGapped)
	}
	if err := p.Routing.Validate(); err != nil {
		return wrapf("Validate", err)
	}
	if err := dataresidency.ComposeWithRouterAirGap(&p.Residency, p.Routing); err != nil {
		return wrapf("Validate", err)
	}
	if !p.KeyProviderConfigured {
		return wrapf("Validate", ErrKeyProviderRequired)
	}
	return nil
}
