package application_test

import (
	"testing"

	"github.com/YASSERRMD/verdex/packages/application"
)

func TestWeightByLegalFamily_FlipsBetweenCommonAndCivilLaw(t *testing.T) {
	statuteRule := application.OriginatedRule{
		Rule:   testRule(t, "rule-statute", "text", "US-CA", "common_law"),
		Origin: application.OriginStatute,
	}
	precedentRule := application.OriginatedRule{
		Rule:   testRule(t, "rule-precedent", "text", "US-CA", "common_law"),
		Origin: application.OriginPrecedent,
	}

	commonStatute := application.WeightByLegalFamily(statuteRule, "common_law")
	commonPrecedent := application.WeightByLegalFamily(precedentRule, "common_law")
	if commonPrecedent <= commonStatute {
		t.Fatalf("expected precedent weight (%f) > statute weight (%f) under common_law", commonPrecedent, commonStatute)
	}

	civilStatute := application.WeightByLegalFamily(statuteRule, "civil_law")
	civilPrecedent := application.WeightByLegalFamily(precedentRule, "civil_law")
	if civilStatute <= civilPrecedent {
		t.Fatalf("expected statute weight (%f) > precedent weight (%f) under civil_law", civilStatute, civilPrecedent)
	}
}

func TestWeightByLegalFamily_NeutralForUnknownFamily(t *testing.T) {
	statuteRule := application.OriginatedRule{Origin: application.OriginStatute}
	precedentRule := application.OriginatedRule{Origin: application.OriginPrecedent}

	statuteWeight := application.WeightByLegalFamily(statuteRule, "unknown_family")
	precedentWeight := application.WeightByLegalFamily(precedentRule, "unknown_family")

	if statuteWeight != precedentWeight {
		t.Fatalf("expected equal neutral weights, got statute=%f precedent=%f", statuteWeight, precedentWeight)
	}
}
