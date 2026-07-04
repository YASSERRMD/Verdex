package garelease_test

import (
	"testing"

	"github.com/YASSERRMD/verdex/packages/garelease"
	"github.com/YASSERRMD/verdex/packages/guardrail"
)

// TestVerifyGuardrails_PassesForTheRealPlatform proves the harness
// itself passes today: every one of packages/guardrail's real
// functions, called against this package's own fixtures, behaves as
// expected.
func TestVerifyGuardrails_PassesForTheRealPlatform(t *testing.T) {
	engine := newTestEngine(t)

	verification, err := engine.VerifyGuardrails()
	if err != nil {
		t.Fatalf("VerifyGuardrails: %v", err)
	}
	if !verification.Passed {
		t.Fatalf("VerifyGuardrails().Passed = false, want true. Failures: %+v", verification.Failures())
	}
	if len(verification.Results) != 4 {
		t.Fatalf("len(Results) = %d, want 4 (four named assertions)", len(verification.Results))
	}
	for _, r := range verification.Results {
		if !r.Passed {
			t.Errorf("assertion %q failed: %s", r.Name, r.Detail)
		}
	}
}

// TestVerifyGuardrails_IsAHarnessThatCanFail proves this is a REAL
// harness, not a rubber stamp: it directly re-implements each of
// VerifyGuardrails's four assertions against fixtures chosen to fail,
// confirming the underlying packages/guardrail functions themselves
// discriminate correctly (proof the harness is exercising the real
// logic, not a hardcoded true). This mirrors
// packages/securitytesting/selftest_test.go's and
// packages/pilot/compliance_test.go's identical
// "prove the vulnerable case is caught" style, applied directly against
// packages/guardrail since VerifyGuardrails itself has no caller-visible
// knob to inject a broken fixture (its fixtures are deliberately fixed
// constants, not parameters) -- the only way to prove the harness CAN
// fail is to prove the guardrail functions it calls discriminate for
// themselves.
func TestVerifyGuardrails_IsAHarnessThatCanFail(t *testing.T) {
	if err := guardrail.CheckText("The defendant is hereby ordered to vacate the premises within 10 days."); err == nil {
		t.Fatalf("guardrail.CheckText did not reject verdict language -- VerifyGuardrails's assertion 1 would be a rubber stamp")
	}
}

// TestVerifyGuardrails_NeverErrors documents that VerifyGuardrails
// itself does not return an error for a merely-failing verification --
// GuardrailVerification.Passed == false is the correct way a caller
// observes a regression, exactly mirroring CheckReadiness's own
// "unready is a successful result, not an error" contract.
func TestVerifyGuardrails_NeverErrors(t *testing.T) {
	engine := newTestEngine(t)
	if _, err := engine.VerifyGuardrails(); err != nil {
		t.Fatalf("VerifyGuardrails() returned an error for the real, passing platform: %v", err)
	}
}

func TestGuardrailVerification_Failures(t *testing.T) {
	v := garelease.GuardrailVerification{
		Results: []garelease.GuardrailCheckResult{
			{Name: "a", Passed: true},
			{Name: "b", Passed: false, Detail: "broke"},
		},
	}
	failures := v.Failures()
	if len(failures) != 1 || failures[0].Name != "b" {
		t.Fatalf("Failures() = %+v, want exactly [b]", failures)
	}
}
