package lawapplication_test

import (
	"testing"

	"github.com/YASSERRMD/verdex/packages/lawapplication"
)

func TestInferOrigin_OriginHintWins(t *testing.T) {
	rule := lawapplication.RuleRef{
		Text:       "the court held that...",
		OriginHint: lawapplication.OriginStatute,
	}
	if got := lawapplication.InferOrigin(rule); got != lawapplication.OriginStatute {
		t.Errorf("InferOrigin = %v, want OriginStatute (hint should win over text heuristic)", got)
	}
}

func TestInferOrigin_StatuteKeywords(t *testing.T) {
	rule := lawapplication.RuleRef{Text: "Under 42 U.S.C. § 1983, a plaintiff may recover damages."}
	if got := lawapplication.InferOrigin(rule); got != lawapplication.OriginStatute {
		t.Errorf("InferOrigin = %v, want OriginStatute", got)
	}
}

func TestInferOrigin_PrecedentKeywords(t *testing.T) {
	rule := lawapplication.RuleRef{Text: "In Smith v. Jones, the court held that notice must be reasonable."}
	if got := lawapplication.InferOrigin(rule); got != lawapplication.OriginPrecedent {
		t.Errorf("InferOrigin = %v, want OriginPrecedent", got)
	}
}

func TestInferOrigin_StatuteWinsOnOverlap(t *testing.T) {
	rule := lawapplication.RuleRef{Text: "Section 5 of the Act of 1990, as construed in Smith v. Jones."}
	if got := lawapplication.InferOrigin(rule); got != lawapplication.OriginStatute {
		t.Errorf("InferOrigin = %v, want OriginStatute (statute should win on overlap)", got)
	}
}

func TestInferOrigin_UnknownWhenNoSignal(t *testing.T) {
	rule := lawapplication.RuleRef{Text: "Reasonable notice is required before termination."}
	if got := lawapplication.InferOrigin(rule); got != lawapplication.OriginUnknown {
		t.Errorf("InferOrigin = %v, want OriginUnknown", got)
	}
}

func TestInferOrigin_EmptyText(t *testing.T) {
	if got := lawapplication.InferOrigin(lawapplication.RuleRef{}); got != lawapplication.OriginUnknown {
		t.Errorf("InferOrigin = %v, want OriginUnknown", got)
	}
}

func TestOrigin_IsValid(t *testing.T) {
	valid := []lawapplication.Origin{lawapplication.OriginUnknown, lawapplication.OriginStatute, lawapplication.OriginPrecedent}
	for _, o := range valid {
		if !o.IsValid() {
			t.Errorf("Origin(%q).IsValid() = false, want true", o)
		}
	}
	if lawapplication.Origin("bogus").IsValid() {
		t.Errorf("Origin(bogus).IsValid() = true, want false")
	}
}
