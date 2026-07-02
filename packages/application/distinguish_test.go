package application_test

import (
	"errors"
	"testing"
	"time"

	"github.com/YASSERRMD/verdex/packages/application"
)

func TestNewDistinguishingFact_Success(t *testing.T) {
	rule := application.OriginatedRule{
		Rule:   testRule(t, "rule-1", "precedent text", "US-CA", "common_law"),
		Origin: application.OriginPrecedent,
	}
	fact := testFact(t, "fact-1", "the agreement was oral, not written")

	df, err := application.NewDistinguishingFact(fact, rule, "the precedent involved a written contract", time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if df.Fact.ID != "fact-1" {
		t.Fatalf("expected fact-1, got %s", df.Fact.ID)
	}
	if df.Rationale == "" {
		t.Fatal("expected non-empty rationale")
	}
}

func TestNewDistinguishingFact_RejectsNonPrecedentOrigin(t *testing.T) {
	rule := application.OriginatedRule{
		Rule:   testRule(t, "rule-1", "statute text", "US-CA", "civil_law"),
		Origin: application.OriginStatute,
	}
	fact := testFact(t, "fact-1", "some fact")

	_, err := application.NewDistinguishingFact(fact, rule, "rationale", time.Now())
	if err == nil {
		t.Fatal("expected error for statute-origin rule")
	}
}

func TestNewDistinguishingFact_RejectsEmptyRationale(t *testing.T) {
	rule := application.OriginatedRule{
		Rule:   testRule(t, "rule-1", "precedent text", "US-CA", "common_law"),
		Origin: application.OriginPrecedent,
	}
	fact := testFact(t, "fact-1", "some fact")

	_, err := application.NewDistinguishingFact(fact, rule, "", time.Now())
	if !errors.Is(err, application.ErrEmptyInput) {
		t.Fatalf("expected ErrEmptyInput, got %v", err)
	}
}
