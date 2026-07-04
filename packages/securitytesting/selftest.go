package securitytesting

import "context"

// This file is the harness's own proof-of-life: a "harness that can't
// fail is worthless" self-test pair. VulnerableFixtureScenario models a
// component that has NOT actually implemented the isolation guard it
// claims to (a naive, unscoped in-memory map with zero tenant checks
// at all) -- the Scenario correctly reports OutcomeFailed against it.
// FixedFixtureScenario models the same shape with the guard correctly
// wired in, and the identical Scenario reports OutcomePassed. Both
// scenarios probe the exact same behavior through the exact same
// assertion logic; only the fixture underneath differs -- which is
// what proves the assertion logic itself, not the fixture, is doing
// the real work. Neither Scenario is included in any of this package's
// production suites (NewRegressionSuite, NewPromptInjectionSuite,
// NewDataLeakageSuite, NewAuthzBypassSuite, NewAbuseCaseSuite) --
// selftest_test.go exercises them directly.

// naiveUnscopedStore is the deliberately-vulnerable fixture: a plain
// map with no tenant check whatsoever on Get. This is what a
// regression in (for instance) InMemoryFindingRepository's
// requireMatchingTenant call would look like if that guard were ever
// accidentally deleted.
type naiveUnscopedStore struct {
	values map[string]string
}

func newNaiveUnscopedStore() *naiveUnscopedStore {
	return &naiveUnscopedStore{values: map[string]string{
		"tenant-a-secret": "tenant A's confidential case note", // #nosec G101 -- fixture map key/value, not a credential
	}}
}

// get returns the value for key with NO tenant scoping at all --
// intentionally vulnerable: any caller can read any key regardless of
// which tenant they claim to be scoped to.
func (s *naiveUnscopedStore) get(_ tenantScope, key string) (string, bool) {
	v, ok := s.values[key]
	return v, ok
}

// scopedStore is the fixed fixture: identical data, but get refuses to
// return a value unless the caller's tenantScope matches the key's
// owning tenant -- the correctly-implemented version of the same
// guard.
type scopedStore struct {
	values map[string]scopedValue
}

type scopedValue struct {
	owner tenantScope
	data  string
}

type tenantScope string

func newScopedStore() *scopedStore {
	return &scopedStore{values: map[string]scopedValue{
		"tenant-a-secret": {owner: "tenant-a", data: "tenant A's confidential case note"},
	}}
}

// get returns the value for key only if scope matches the key's owner
// -- the fixed, correctly-scoped equivalent of naiveUnscopedStore.get.
func (s *scopedStore) get(scope tenantScope, key string) (string, bool) {
	v, ok := s.values[key]
	if !ok || v.owner != scope {
		return "", false
	}
	return v.data, true
}

// probeAttackerCannotReadTenantASecret is the shared assertion both
// self-test scenarios run: an attacker claiming to be scoped to
// "tenant-b" must never be able to read the "tenant-a-secret" key.
// getFn abstracts over which fixture (vulnerable or fixed) is under
// test, so this one function is the single place the actual security
// property is asserted.
func probeAttackerCannotReadTenantASecret(getFn func(scope tenantScope, key string) (string, bool)) Result {
	value, ok := getFn("tenant-b", "tenant-a-secret")
	if ok {
		return Result{
			Outcome: OutcomeFailed,
			Detail:  "attacker scoped to tenant-b successfully read a key owned by tenant-a -- isolation guard did not hold",
			Evidence: map[string]string{
				"leaked_value": value,
			},
		}
	}
	return Result{Outcome: OutcomePassed, Detail: "attacker scoped to tenant-b correctly could not read a key owned by tenant-a"}
}

// VulnerableFixtureScenario runs probeAttackerCannotReadTenantASecret
// against naiveUnscopedStore, which has no tenant check at all --
// correctly-functioning harness logic must report OutcomeFailed here.
func VulnerableFixtureScenario() Scenario {
	return NewScenarioFunc(
		"selftest/vulnerable-fixture-correctly-flagged",
		CategoryRegression,
		func(_ context.Context) (Result, error) {
			store := newNaiveUnscopedStore()
			return probeAttackerCannotReadTenantASecret(store.get), nil
		},
	)
}

// FixedFixtureScenario runs the identical probe against scopedStore,
// which DOES check tenant ownership -- correctly-functioning harness
// logic must report OutcomePassed here, proving the harness is not
// simply hardcoded to always fail (or always pass) regardless of what
// it is actually probing.
func FixedFixtureScenario() Scenario {
	return NewScenarioFunc(
		"selftest/fixed-fixture-correctly-passed",
		CategoryRegression,
		func(_ context.Context) (Result, error) {
			store := newScopedStore()
			return probeAttackerCannotReadTenantASecret(store.get), nil
		},
	)
}
