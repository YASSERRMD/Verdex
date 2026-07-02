package lawapplication_test

import (
	"errors"
	"testing"

	"github.com/YASSERRMD/verdex/packages/lawapplication"
)

func TestAttachCitations_NilLookupMarksUnresolved(t *testing.T) {
	rules := []lawapplication.RuleRef{{ID: "rule-1", Text: "Section 5 of the Act."}}
	got := lawapplication.AttachCitations([]string{"rule-1"}, rules, nil)
	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1", len(got))
	}
	if got[0].Resolved {
		t.Errorf("Resolved = true, want false with nil lookup")
	}
	if got[0].Origin != lawapplication.OriginStatute {
		t.Errorf("Origin = %v, want OriginStatute (inferred from text even without lookup)", got[0].Origin)
	}
}

func TestAttachCitations_SuccessfulLookup(t *testing.T) {
	rules := []lawapplication.RuleRef{{ID: "rule-1"}}
	lookup := func(ruleID string) (string, lawapplication.Origin, bool, string, error) {
		return "Fake Reporter " + ruleID, lawapplication.OriginPrecedent, true, "verified", nil
	}

	got := lawapplication.AttachCitations([]string{"rule-1"}, rules, lookup)
	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1", len(got))
	}
	c := got[0]
	if !c.Resolved || !c.Verified {
		t.Errorf("c = %+v, want resolved and verified", c)
	}
	if c.Citation != "Fake Reporter rule-1" {
		t.Errorf("Citation = %q, want %q", c.Citation, "Fake Reporter rule-1")
	}
	if c.Origin != lawapplication.OriginPrecedent {
		t.Errorf("Origin = %v, want OriginPrecedent (from lookup)", c.Origin)
	}
}

func TestAttachCitations_LookupErrorMarksUnresolved(t *testing.T) {
	rules := []lawapplication.RuleRef{{ID: "rule-1"}}
	lookup := func(_ string) (string, lawapplication.Origin, bool, string, error) {
		return "", lawapplication.OriginUnknown, false, "", errors.New("not found")
	}

	got := lawapplication.AttachCitations([]string{"rule-1"}, rules, lookup)
	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1", len(got))
	}
	if got[0].Resolved {
		t.Errorf("Resolved = true, want false on lookup error")
	}
}

func TestAttachCitations_OneEntryPerControllingRule(t *testing.T) {
	rules := []lawapplication.RuleRef{{ID: "rule-1"}, {ID: "rule-2"}}
	got := lawapplication.AttachCitations([]string{"rule-1", "rule-2"}, rules, nil)
	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2", len(got))
	}
}
