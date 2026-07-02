package application_test

import (
	"errors"
	"testing"

	"github.com/YASSERRMD/verdex/packages/application"
)

func TestRuleChain_Validate_AcceptsAcyclicChain(t *testing.T) {
	chain := application.RuleChain{
		Rules: []application.OriginatedRule{
			{Rule: testRule(t, "rule-1", "section 1", "US-CA", "civil_law")},
			{Rule: testRule(t, "rule-2", "section 2 cross-references section 1", "US-CA", "civil_law")},
		},
	}

	if err := chain.Validate(); err != nil {
		t.Fatalf("unexpected error for acyclic chain: %v", err)
	}
	if chain.Len() != 2 {
		t.Fatalf("expected length 2, got %d", chain.Len())
	}
}

func TestRuleChain_Validate_RejectsCycle(t *testing.T) {
	repeated := testRule(t, "rule-1", "section 1", "US-CA", "civil_law")
	chain := application.RuleChain{
		Rules: []application.OriginatedRule{
			{Rule: repeated},
			{Rule: testRule(t, "rule-2", "section 2", "US-CA", "civil_law")},
			{Rule: repeated},
		},
	}

	err := chain.Validate()
	if !errors.Is(err, application.ErrCyclicChain) {
		t.Fatalf("expected ErrCyclicChain, got %v", err)
	}
}

func TestRuleChain_Validate_RejectsEmptyChain(t *testing.T) {
	chain := application.RuleChain{}

	err := chain.Validate()
	if !errors.Is(err, application.ErrEmptyInput) {
		t.Fatalf("expected ErrEmptyInput, got %v", err)
	}
}
