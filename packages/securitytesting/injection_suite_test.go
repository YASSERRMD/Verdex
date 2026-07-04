package securitytesting_test

import (
	"testing"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/securitytesting"
)

func TestNewPromptInjectionSuite_EveryScenarioPasses(t *testing.T) {
	t.Parallel()

	scenarios := securitytesting.NewPromptInjectionSuite()
	if len(scenarios) == 0 {
		t.Fatal("NewPromptInjectionSuite() returned zero scenarios")
	}

	h := securitytesting.NewHarness(scenarios...)
	records := h.RunAll(t.Context(), uuid.Nil, uuid.Nil)
	if len(records) != len(scenarios) {
		t.Fatalf("RunAll produced %d records, want %d (one per scenario)", len(records), len(scenarios))
	}
	for _, r := range records {
		if r.Result.Outcome != securitytesting.OutcomePassed {
			t.Errorf("scenario %s outcome = %v, detail = %q, want OutcomePassed -- packages/threatmodel.DetectInjectionAttempt should catch every corpus entry and never false-positive on legitimate text", r.ScenarioName, r.Result.Outcome, r.Result.Detail)
		}
		if r.ScenarioCategory != securitytesting.CategoryPromptInjection {
			t.Errorf("scenario %s category = %v, want CategoryPromptInjection", r.ScenarioName, r.ScenarioCategory)
		}
	}
}

func TestNewPromptInjectionSuite_CoversEveryFindingKind(t *testing.T) {
	t.Parallel()

	// A coarse sanity check that this suite's corpus actually spans
	// the adversarial techniques the brief calls for (role-override,
	// delimiter-breaking, instruction-smuggling) rather than just one
	// narrow slice of it -- checked by scenario-name prefix, since the
	// corpus entries themselves are unexported.
	scenarios := securitytesting.NewPromptInjectionSuite()
	prefixes := map[string]bool{
		"prompt-injection/role-override":               false,
		"prompt-injection/instruction-smuggling":       false,
		"prompt-injection/delimiter-breaking":          false,
		"prompt-injection/data-exfiltration":           false,
		"prompt-injection/legitimate-text-not-flagged": false,
	}
	for _, s := range scenarios {
		for prefix := range prefixes {
			if len(s.Name()) >= len(prefix) && s.Name()[:len(prefix)] == prefix {
				prefixes[prefix] = true
			}
		}
	}
	for prefix, found := range prefixes {
		if !found {
			t.Errorf("NewPromptInjectionSuite() has no scenario with name prefix %q", prefix)
		}
	}
}
