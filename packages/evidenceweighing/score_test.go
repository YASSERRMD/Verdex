package evidenceweighing_test

import (
	"strings"
	"testing"

	"github.com/YASSERRMD/verdex/packages/evidenceweighing"
)

func TestScoreFact_MonotonicInCorroborationAndStrength(t *testing.T) {
	rubric := evidenceweighing.DefaultRubric()
	fact := evidenceweighing.FactRef{ID: "fact-1", Confidence: 0.6}

	low := evidenceweighing.ScoreFact(rubric, fact, 0, false, 0.0)
	higherCorroboration := evidenceweighing.ScoreFact(rubric, fact, 3, false, 0.0)
	higherStrength := evidenceweighing.ScoreFact(rubric, fact, 0, false, 1.0)

	if higherCorroboration.Weight <= low.Weight {
		t.Errorf("more corroboration should not lower weight: low=%.4f higher=%.4f", low.Weight, higherCorroboration.Weight)
	}
	if higherStrength.Weight <= low.Weight {
		t.Errorf("higher citing strength should not lower weight: low=%.4f higher=%.4f", low.Weight, higherStrength.Weight)
	}
}

func TestScoreFact_ContradictionPenalty(t *testing.T) {
	rubric := evidenceweighing.DefaultRubric()
	fact := evidenceweighing.FactRef{ID: "fact-1", Confidence: 0.8}

	uncontradicted := evidenceweighing.ScoreFact(rubric, fact, 2, false, 0.5)
	contradicted := evidenceweighing.ScoreFact(rubric, fact, 2, true, 0.5)

	if contradicted.Weight >= uncontradicted.Weight {
		t.Errorf("contradicted fact should score lower: contradicted=%.4f uncontradicted=%.4f", contradicted.Weight, uncontradicted.Weight)
	}
	if !contradicted.Contradicted {
		t.Errorf("Contradicted flag should be true")
	}
	if !strings.Contains(contradicted.Rationale, "contradiction penalty") {
		t.Errorf("rationale should mention the contradiction penalty, got %q", contradicted.Rationale)
	}
}

func TestScoreFact_WeightAlwaysInUnitInterval(t *testing.T) {
	rubric := evidenceweighing.DefaultRubric()
	extreme := evidenceweighing.FactRef{ID: "fact-1", Confidence: 5.0} // out-of-range input

	fw := evidenceweighing.ScoreFact(rubric, extreme, 100, true, 5.0)

	if fw.Weight < 0 || fw.Weight > 1 {
		t.Errorf("Weight = %.4f, want in [0, 1]", fw.Weight)
	}
}

func TestScoreFact_RationaleNonEmpty(t *testing.T) {
	rubric := evidenceweighing.DefaultRubric()
	fact := evidenceweighing.FactRef{ID: "fact-1", Confidence: 0.5}

	fw := evidenceweighing.ScoreFact(rubric, fact, 1, false, 0.3)

	if fw.Rationale == "" {
		t.Errorf("expected a non-empty rationale")
	}
}

func TestScoreFacts_UncitedFactStillScored(t *testing.T) {
	rubric := evidenceweighing.DefaultRubric()
	facts := []evidenceweighing.FactRef{{ID: "fact-1", Confidence: 0.7}}

	weights := evidenceweighing.ScoreFacts(rubric, facts, nil)

	if len(weights) != 1 {
		t.Fatalf("len(weights) = %d, want 1", len(weights))
	}
	if weights[0].CorroborationCount != 0 || weights[0].Contradicted {
		t.Errorf("uncited fact should have zero corroboration and not be contradicted: %+v", weights[0])
	}
}
