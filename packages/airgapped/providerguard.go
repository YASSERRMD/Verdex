package airgapped

import (
	"strings"

	"github.com/YASSERRMD/verdex/packages/provider"
)

// localProviderPrefix is the provider-ID prefix packages/router's
// LocalOnlyEnforcer already uses to recognize a local/self-hosted
// provider (see packages/router/local_mode.go). This package reuses
// the exact same convention rather than inventing a second one, so a
// provider ID that satisfies router's air-gapped filter also satisfies
// this package's registration guard.
const localProviderPrefix = "local:"

// IsLocalProviderID reports whether id carries the "local:" prefix
// packages/adapters/local's LocalAdapter.ID() produces (e.g.
// "local:llama3:8b") and packages/router.LocalOnlyEnforcer requires
// under RoutingPolicy.AirGappedOnly.
func IsLocalProviderID(id string) bool {
	return strings.HasPrefix(id, localProviderPrefix)
}

// GuardRegister rejects registering p under id in an air-gapped
// Profile unless id carries the "local:" prefix -- task 2's "a real
// check that rejects any non-local adapter registration attempt under
// an air-gapped Profile". Callers wire this in front of
// provider.Registry.Register (or call it before constructing any
// non-local adapter) instead of calling Register directly once a
// Profile is active. It does not wrap or replace provider.Registry;
// registration itself remains that package's job.
func GuardRegister(profile *Profile, id string, p provider.LLMProvider) error {
	if profile == nil {
		return ErrNilProfile
	}
	if !IsLocalProviderID(id) {
		return wrapf("GuardRegister", ErrNonLocalProvider)
	}
	_ = p // p is accepted for signature symmetry with provider.Registry.Register; no further inspection is needed once the ID prefix is confirmed.
	return nil
}

// GuardRegistry registers p under id into reg only if id passes
// GuardRegister for profile, composing with provider.Registry.Register
// rather than duplicating its bookkeeping.
func GuardRegistry(profile *Profile, reg *provider.Registry, id string, p provider.LLMProvider) error {
	if reg == nil {
		return ErrNilRegistry
	}
	if err := GuardRegister(profile, id, p); err != nil {
		return err
	}
	return reg.Register(id, p)
}

// AuditRegistry inspects every provider ID already present in reg and
// returns the subset that violate the air-gapped local-only
// constraint (i.e. do not carry the "local:" prefix). An empty result
// means the registry is fully compliant. This is the read-only
// counterpart to GuardRegistry, useful for a startup/conformance check
// against a registry that may have been populated by code this package
// does not control.
func AuditRegistry(reg *provider.Registry) []string {
	if reg == nil {
		return nil
	}
	var violations []string
	for _, id := range reg.List() {
		if !IsLocalProviderID(id) {
			violations = append(violations, id)
		}
	}
	return violations
}
