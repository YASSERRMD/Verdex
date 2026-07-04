package securitytesting

// SeedHarness builds a Harness pre-registered with every production
// adversarial suite this phase ships: the automated security
// regression suite (task 1), the prompt-injection adversarial suite
// (task 3), the data-leakage suite (task 4), the authz-bypass suite
// (task 5), and the abuse-case suite (task 6). Task 2's pentest-scope
// harness contract (the Scenario interface itself) is what every one
// of these suites implements, not a separate registration step here.
//
// Deliberately excluded: VulnerableFixtureScenario/FixedFixtureScenario
// (selftest.go) are the harness's own self-test pair, proving the
// harness mechanics work at all -- they are not a real defense this
// platform ships, and including them in the default seeded Harness
// would mean every production suite run always contains one
// deliberately-failing scenario, which is not useful noise to carry
// into a real CI gate. Callers that specifically want to exercise the
// self-test pair should call NewHarness(VulnerableFixtureScenario(),
// FixedFixtureScenario()) directly, exactly as selftest_test.go does.
func SeedHarness() *Harness {
	h := NewHarness()
	for _, s := range NewRegressionSuite() {
		h.Add(s)
	}
	for _, s := range NewPromptInjectionSuite() {
		h.Add(s)
	}
	for _, s := range NewDataLeakageSuite() {
		h.Add(s)
	}
	for _, s := range NewAuthzBypassSuite() {
		h.Add(s)
	}
	for _, s := range NewAbuseCaseSuite() {
		h.Add(s)
	}
	return h
}
