package reasoningeval_test

import (
	"testing"

	"github.com/YASSERRMD/verdex/packages/reasoningeval"
)

func TestAggregateByJurisdiction_GroupsCorrectly(t *testing.T) {
	scores := []reasoningeval.QualityScore{
		{JurisdictionCode: "AE-DXB", LegalFamily: "civil_law", Overall: 0.9},
		{JurisdictionCode: "AE-DXB", LegalFamily: "civil_law", Overall: 0.8},
		{JurisdictionCode: "US-NY", LegalFamily: "common_law", Overall: 0.6},
	}

	summaries := reasoningeval.AggregateByJurisdiction(scores)

	if len(summaries) != 2 {
		t.Fatalf("len(summaries) = %d, want 2", len(summaries))
	}

	dxb, ok := summaries["AE-DXB"]
	if !ok {
		t.Fatal("summaries missing AE-DXB")
	}
	if dxb.Count != 2 {
		t.Errorf("AE-DXB Count = %d, want 2", dxb.Count)
	}
	if want := 0.85; !floatNear(dxb.AvgOverall, want) {
		t.Errorf("AE-DXB AvgOverall = %.4f, want %.4f", dxb.AvgOverall, want)
	}
	if dxb.LegalFamily != "civil_law" {
		t.Errorf("AE-DXB LegalFamily = %q, want civil_law", dxb.LegalFamily)
	}

	ny, ok := summaries["US-NY"]
	if !ok {
		t.Fatal("summaries missing US-NY")
	}
	if ny.Count != 1 || ny.AvgOverall != 0.6 {
		t.Errorf("US-NY summary = %+v, want Count=1 AvgOverall=0.6", ny)
	}
}

func TestAggregateByJurisdiction_KeepsUnknownJurisdictionScores(t *testing.T) {
	scores := []reasoningeval.QualityScore{
		{JurisdictionCode: "", Overall: 0.5},
	}
	summaries := reasoningeval.AggregateByJurisdiction(scores)
	unknown, ok := summaries[""]
	if !ok {
		t.Fatal("summaries missing the empty-jurisdiction group")
	}
	if unknown.Count != 1 {
		t.Errorf("Count = %d, want 1", unknown.Count)
	}
}

func TestAggregateByLegalFamily_GroupsAcrossJurisdictions(t *testing.T) {
	scores := []reasoningeval.QualityScore{
		{JurisdictionCode: "AE-DXB", LegalFamily: "civil_law", Overall: 0.9},
		{JurisdictionCode: "FR-PAR", LegalFamily: "civil_law", Overall: 0.7},
		{JurisdictionCode: "US-NY", LegalFamily: "common_law", Overall: 0.6},
		{JurisdictionCode: "unknown", LegalFamily: "", Overall: 0.1},
	}

	summaries := reasoningeval.AggregateByLegalFamily(scores)

	if len(summaries) != 2 {
		t.Fatalf("len(summaries) = %d, want 2 (empty family excluded)", len(summaries))
	}

	civil, ok := summaries["civil_law"]
	if !ok {
		t.Fatal("summaries missing civil_law")
	}
	if civil.Count != 2 {
		t.Errorf("civil_law Count = %d, want 2", civil.Count)
	}
	if want := 0.8; !floatNear(civil.AvgOverall, want) {
		t.Errorf("civil_law AvgOverall = %.4f, want %.4f", civil.AvgOverall, want)
	}
}
