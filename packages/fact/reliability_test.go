package fact_test

import (
	"testing"

	"github.com/YASSERRMD/verdex/packages/fact"
)

func TestReliabilityScore_BoundsAndInputs(t *testing.T) {
	tests := []struct {
		name  string
		input fact.ReliabilityInput
	}{
		{name: "zero everything", input: fact.ReliabilityInput{}},
		{name: "max confidence, high corroboration, undisputed", input: fact.ReliabilityInput{ClassificationConfidence: 1, CorroborationCount: 10, DisputeStatus: fact.Undisputed}},
		{name: "disputed with high confidence", input: fact.ReliabilityInput{ClassificationConfidence: 1, DisputeStatus: fact.Disputed}},
		{name: "out of range confidence", input: fact.ReliabilityInput{ClassificationConfidence: 5, DisputeStatus: fact.Unknown}},
		{name: "negative confidence", input: fact.ReliabilityInput{ClassificationConfidence: -5, DisputeStatus: fact.Unknown}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := fact.ReliabilityScore(tt.input)
			if score < 0 || score > 1 {
				t.Fatalf("expected score in [0, 1], got %v", score)
			}
		})
	}
}

func TestReliabilityScore_MonotonicInCorroboration(t *testing.T) {
	base := fact.ReliabilityInput{ClassificationConfidence: 0.5, DisputeStatus: fact.Undisputed, CorroborationCount: 0}
	more := base
	more.CorroborationCount = 1
	most := base
	most.CorroborationCount = 5

	scoreBase := fact.ReliabilityScore(base)
	scoreMore := fact.ReliabilityScore(more)
	scoreMost := fact.ReliabilityScore(most)

	if scoreMore < scoreBase {
		t.Errorf("expected score to not decrease with more corroboration: base=%v more=%v", scoreBase, scoreMore)
	}
	if scoreMost < scoreMore {
		t.Errorf("expected score to not decrease with even more corroboration: more=%v most=%v", scoreMore, scoreMost)
	}
}

func TestReliabilityScore_DisputedScoresLowerThanUndisputed(t *testing.T) {
	undisputed := fact.ReliabilityInput{ClassificationConfidence: 0.6, CorroborationCount: 2, DisputeStatus: fact.Undisputed}
	disputed := undisputed
	disputed.DisputeStatus = fact.Disputed
	unknown := undisputed
	unknown.DisputeStatus = fact.Unknown

	scoreUndisputed := fact.ReliabilityScore(undisputed)
	scoreDisputed := fact.ReliabilityScore(disputed)
	scoreUnknown := fact.ReliabilityScore(unknown)

	if scoreDisputed >= scoreUndisputed {
		t.Errorf("expected disputed score (%v) to be lower than undisputed score (%v)", scoreDisputed, scoreUndisputed)
	}
	if scoreDisputed >= scoreUnknown {
		t.Errorf("expected disputed score (%v) to be lower than unknown score (%v)", scoreDisputed, scoreUnknown)
	}
	if scoreUnknown >= scoreUndisputed {
		t.Errorf("expected unknown score (%v) to be lower than undisputed score (%v)", scoreUnknown, scoreUndisputed)
	}
}
