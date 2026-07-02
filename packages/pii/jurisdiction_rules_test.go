package pii_test

import (
	"testing"

	"github.com/YASSERRMD/verdex/packages/pii"
)

func TestJurisdictionPIIRules_RuleFor_FallsBackToDefault(t *testing.T) {
	rules := pii.NewJurisdictionPIIRules(pii.CategoryRule{Sensitivity: 0.5})

	got := rules.RuleFor("AE", pii.CategoryFinancial)
	if got.Sensitivity != 0.5 {
		t.Errorf("RuleFor() = %+v, want default rule", got)
	}
}

func TestJurisdictionPIIRules_SetRule_Override(t *testing.T) {
	rules := pii.NewJurisdictionPIIRules(pii.CategoryRule{Sensitivity: 0.3})
	rules.SetRule("AE", pii.CategoryFinancial, pii.CategoryRule{
		Sensitivity:  0.95,
		RequiredMode: pii.ModeIrreversibleRedact,
	})

	got := rules.RuleFor("AE", pii.CategoryFinancial)
	if got.Sensitivity != 0.95 || got.RequiredMode != pii.ModeIrreversibleRedact {
		t.Errorf("RuleFor(AE, financial) = %+v, want overridden rule", got)
	}

	// A different jurisdiction with no override still gets the default.
	other := rules.RuleFor("PK", pii.CategoryFinancial)
	if other.Sensitivity != 0.3 {
		t.Errorf("RuleFor(PK, financial) = %+v, want default rule", other)
	}

	// A different category within the overridden jurisdiction still gets
	// the default.
	otherCategory := rules.RuleFor("AE", pii.CategoryName)
	if otherCategory.Sensitivity != 0.3 {
		t.Errorf("RuleFor(AE, name) = %+v, want default rule", otherCategory)
	}
}

func TestJurisdictionPIIRules_CodeNormalization(t *testing.T) {
	rules := pii.NewJurisdictionPIIRules(pii.CategoryRule{})
	rules.SetRule("ae", pii.CategoryIdentifier, pii.CategoryRule{Sensitivity: 0.9})

	got := rules.RuleFor(" AE ", pii.CategoryIdentifier)
	if got.Sensitivity != 0.9 {
		t.Errorf("RuleFor() with differing case/whitespace = %+v, want normalized match", got)
	}
}

func TestJurisdictionPIIRules_ApplyToMatches(t *testing.T) {
	rules := pii.NewJurisdictionPIIRules(pii.CategoryRule{})
	rules.SetRule("AE", pii.CategoryIdentifier, pii.CategoryRule{RequiredMode: pii.ModeIrreversibleRedact})

	matches := []pii.PIIMatch{
		{Category: pii.CategoryIdentifier},
		{Category: pii.CategoryName}, // no override configured
	}

	overrides := rules.ApplyToMatches("AE", matches)
	if overrides[pii.CategoryIdentifier] != pii.ModeIrreversibleRedact {
		t.Errorf("overrides[identifier] = %v, want ModeIrreversibleRedact", overrides[pii.CategoryIdentifier])
	}
	if _, ok := overrides[pii.CategoryName]; ok {
		t.Errorf("overrides[name] should be absent (no RequiredMode configured), got %v", overrides[pii.CategoryName])
	}
}
