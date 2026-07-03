package dataresidency

import (
	"context"

	"github.com/YASSERRMD/verdex/packages/provider"
)

// CheckProviderLocality guards provider selection (task 4): given a
// candidate provider's capability descriptor and a deployment's
// residency policy, it returns nil if the provider's declared Region is
// permitted, and a wrapped ErrRegionNotAllowed otherwise. An empty
// Capability.Region is always rejected -- an undeclared locality can
// never be proven compliant, so this guard fails closed exactly like
// CheckTransfer does for an empty destRegion.
//
// This is deliberately a plain function over provider.Capability, not
// a method on router.Router or a change to router's selection
// internals: packages/router (or any other caller that has already
// resolved a candidate provider's Capability) can call this before
// dispatching, composing with the existing LocalOnlyEnforcer selector
// rather than duplicating or replacing it. See doc/data-residency.md
// for the composition write-up and provider_check_test.go for a test
// proving this rejects a disallowed-region provider selection.
func CheckProviderLocality(_ context.Context, cap provider.Capability, policy *ResidencyPolicy) error {
	if policy == nil {
		return ErrNilPolicy
	}
	if cap.Region == "" {
		return wrapf("CheckProviderLocality", ErrRegionNotAllowed)
	}
	if policy.StrictMode {
		// Under strict mode only air-gapped/local capabilities may be
		// selected at all -- there is no "home region" for a fully
		// air-gapped policy (AllowedRegions is empty by construction,
		// see AirGappedPreset), so any declared Region fails closed
		// here too, forcing callers onto router's local-only path.
		return wrapf("CheckProviderLocality", ErrStrictModeViolation)
	}
	if !policy.AllowsRegion(cap.Region) {
		return wrapf("CheckProviderLocality", ErrRegionNotAllowed)
	}
	return nil
}

// FilterProvidersByLocality returns the subset of capabilities whose
// Region is permitted by policy, preserving order. It is a convenience
// wrapper a caller like router can use to narrow an already-resolved
// candidate list in one call instead of looping with
// CheckProviderLocality itself.
func FilterProvidersByLocality(ctx context.Context, capabilities []provider.Capability, policy *ResidencyPolicy) []provider.Capability {
	var out []provider.Capability
	for _, c := range capabilities {
		if CheckProviderLocality(ctx, c, policy) == nil {
			out = append(out, c)
		}
	}
	return out
}
