package e2e_test

import (
	"testing"

	"github.com/YASSERRMD/verdex/packages/category"
	"github.com/YASSERRMD/verdex/packages/e2e"
)

func TestCivilSetupToOpinionScenario(t *testing.T) {
	sc := requireScenario(t, e2e.NewCivilSetupToOpinionScenario)
	if sc.Name() != "civil/setup-to-opinion" {
		t.Fatalf("Name() = %q, want %q", sc.Name(), "civil/setup-to-opinion")
	}
	if sc.CaseCategory() != category.CodeCivil {
		t.Fatalf("CaseCategory() = %q, want %q", sc.CaseCategory(), category.CodeCivil)
	}

	result := runAndRequirePassed(t, sc)
	if result.CompletedReasoningStages != 8 {
		t.Fatalf("CompletedReasoningStages = %d, want 8", result.CompletedReasoningStages)
	}
	if result.IngestionStage != "complete" {
		t.Fatalf("IngestionStage = %q, want complete", result.IngestionStage)
	}
	if result.ReasoningTermination != "complete" {
		t.Fatalf("ReasoningTermination = %q, want complete", result.ReasoningTermination)
	}
	if result.GuardrailApproved {
		t.Fatalf("GuardrailApproved = true, want false (no sign-off recorded)")
	}
	if result.CaseID == "" {
		t.Fatalf("CaseID is empty")
	}
	if result.Duration() < 0 {
		t.Fatalf("Duration() = %v, want non-negative", result.Duration())
	}
}

func TestCriminalSetupToOpinionScenario(t *testing.T) {
	sc := requireScenario(t, e2e.NewCriminalSetupToOpinionScenario)
	if sc.CaseCategory() != category.CodeCriminal {
		t.Fatalf("CaseCategory() = %q, want %q", sc.CaseCategory(), category.CodeCriminal)
	}
	runAndRequirePassed(t, sc)
}

func TestDomesticViolenceSetupToOpinionScenario(t *testing.T) {
	sc := requireScenario(t, e2e.NewDomesticViolenceSetupToOpinionScenario)
	if sc.CaseCategory() != category.CodeDomesticViolence {
		t.Fatalf("CaseCategory() = %q, want %q", sc.CaseCategory(), category.CodeDomesticViolence)
	}
	runAndRequirePassed(t, sc)
}

func TestConsumerSetupToOpinionScenario(t *testing.T) {
	sc := requireScenario(t, e2e.NewConsumerSetupToOpinionScenario)
	if sc.CaseCategory() != category.CodeConsumer {
		t.Fatalf("CaseCategory() = %q, want %q", sc.CaseCategory(), category.CodeConsumer)
	}
	runAndRequirePassed(t, sc)
}

func TestMultilingualIngestionScenario(t *testing.T) {
	sc := requireScenario(t, e2e.NewMultilingualIngestionScenario)
	runAndRequirePassed(t, sc)
}

func TestDiscardGuaranteeScenario(t *testing.T) {
	sc := requireScenario(t, e2e.NewDiscardGuaranteeScenario)
	result := runAndRequirePassed(t, sc)
	if result.CaseID == "" {
		t.Fatalf("expected a non-empty CaseID (the intake ID) in the result")
	}
}

func TestSignoffEnforcementScenario(t *testing.T) {
	sc := requireScenario(t, e2e.NewSignoffEnforcementScenario)
	result := runAndRequirePassed(t, sc)
	if !result.GuardrailApproved {
		t.Fatalf("GuardrailApproved = false after a real Approve decision, want true")
	}
}

func TestDataIsolationScenario(t *testing.T) {
	sc := requireScenario(t, e2e.NewDataIsolationScenario)
	runAndRequirePassed(t, sc)
}

// TestScenario_NamesAreUnique guards against two scenario constructors
// accidentally sharing a Name -- Suite.RunOne and the CI job both key
// on Name, so a collision would silently make one scenario
// unreachable.
func TestScenario_NamesAreUnique(t *testing.T) {
	scenarios := allRegisteredScenarios(t)

	seen := make(map[string]bool, len(scenarios))
	for _, sc := range scenarios {
		if seen[sc.Name()] {
			t.Fatalf("duplicate scenario name: %q", sc.Name())
		}
		seen[sc.Name()] = true
	}
}

// allRegisteredScenarios builds every scenario this package ships,
// failing the test immediately if any constructor errors. Shared by
// TestScenario_NamesAreUnique and the suite-level tests in
// suite_test.go.
func allRegisteredScenarios(t *testing.T) []e2e.Scenario {
	t.Helper()

	civil := requireScenario(t, e2e.NewCivilSetupToOpinionScenario)
	criminal := requireScenario(t, e2e.NewCriminalSetupToOpinionScenario)
	dv := requireScenario(t, e2e.NewDomesticViolenceSetupToOpinionScenario)
	consumer := requireScenario(t, e2e.NewConsumerSetupToOpinionScenario)
	multilingual := requireScenario(t, e2e.NewMultilingualIngestionScenario)
	discard := requireScenario(t, e2e.NewDiscardGuaranteeScenario)
	signoffSc := requireScenario(t, e2e.NewSignoffEnforcementScenario)
	isolation := requireScenario(t, e2e.NewDataIsolationScenario)

	usJurisdiction, err := e2e.NewMultiJurisdictionScenario("US")
	if err != nil {
		t.Fatalf("NewMultiJurisdictionScenario(US): %v", err)
	}
	saJurisdiction, err := e2e.NewMultiJurisdictionScenario("SA")
	if err != nil {
		t.Fatalf("NewMultiJurisdictionScenario(SA): %v", err)
	}

	return []e2e.Scenario{
		civil, criminal, dv, consumer, multilingual, discard, signoffSc, isolation,
		usJurisdiction, saJurisdiction,
	}
}
