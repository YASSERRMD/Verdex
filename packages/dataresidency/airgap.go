package dataresidency

import (
	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/router"
)

// ComposeWithRouterAirGap validates that an air-gapped ResidencyPolicy
// (see AirGappedPreset) is paired with router's existing
// RoutingPolicy.AirGappedOnly flag (Phase 012's local/air-gapped-only
// mode). Task 6 asks for "a policy preset ... requiring router's
// existing air-gapped/local-only flag to be set" -- this function is
// that composition check: it does not set the flag itself (this
// package never constructs or mutates a router.RoutingPolicy; that
// remains router's own concern) and does not modify packages/router at
// all. It simply refuses to certify the pairing as consistent unless
// both sides agree, so a deployment cannot end up with a residency
// policy that claims air-gapped isolation while the router is still
// willing to dial an external provider.
//
// Returns ErrNilPolicy if policy is nil, ErrAirGappedRequiresLocalFlag
// if policy claims air-gapped semantics (StrictMode true and
// AllowedRegions empty) but routingPolicy.AirGappedOnly is false, and
// nil if the composition is consistent.
func ComposeWithRouterAirGap(policy *ResidencyPolicy, routingPolicy router.RoutingPolicy) error {
	if policy == nil {
		return ErrNilPolicy
	}
	if !IsAirGappedPolicy(policy) {
		// Not an air-gapped policy at all: nothing to compose, this is
		// not an error -- a regionally-scoped (non-air-gapped) policy
		// is free to run with or without router's AirGappedOnly flag.
		return nil
	}
	if !routingPolicy.AirGappedOnly {
		return wrapf("ComposeWithRouterAirGap", ErrAirGappedRequiresLocalFlag)
	}
	return nil
}

// IsAirGappedPolicy reports whether policy matches the shape produced
// by AirGappedPreset: StrictMode true and no allowed regions declared.
func IsAirGappedPolicy(policy *ResidencyPolicy) bool {
	if policy == nil {
		return false
	}
	return policy.StrictMode && len(policy.AllowedRegions) == 0
}

// AirGappedPresetFor is a small convenience alias kept alongside the
// composition helpers in this file so callers wiring up an air-gapped
// deployment can find both the preset constructor and the router
// composition check in one place; it defers entirely to
// AirGappedPreset in types.go.
func AirGappedPresetFor(deploymentID uuid.UUID) ResidencyPolicy {
	return AirGappedPreset(deploymentID)
}
