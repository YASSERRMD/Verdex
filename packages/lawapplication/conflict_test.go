package lawapplication_test

import (
	"testing"

	"github.com/YASSERRMD/verdex/packages/lawapplication"
)

func TestDetectConflictingAuthority_FlagsOpposingParties(t *testing.T) {
	args := []lawapplication.ArgumentRef{
		{IssueNodeID: "issue-1", PartyID: "plaintiff", SupportingRuleIDs: []string{"rule-a"}},
		{IssueNodeID: "issue-1", PartyID: "defendant", SupportingRuleIDs: []string{"rule-b"}},
	}

	conflicts := lawapplication.DetectConflictingAuthority("issue-1", []string{"rule-a", "rule-b"}, args)
	if len(conflicts) != 1 {
		t.Fatalf("len(conflicts) = %d, want 1", len(conflicts))
	}
	c := conflicts[0]
	if c.FirstRuleID != "rule-a" || c.SecondRuleID != "rule-b" {
		t.Errorf("conflict rule ids = %s/%s, want rule-a/rule-b", c.FirstRuleID, c.SecondRuleID)
	}
	if c.IssueNodeID != "issue-1" {
		t.Errorf("IssueNodeID = %s, want issue-1", c.IssueNodeID)
	}
	if c.Rationale == "" {
		t.Errorf("Rationale should be non-empty")
	}
}

func TestDetectConflictingAuthority_NoConflictWhenSameParty(t *testing.T) {
	args := []lawapplication.ArgumentRef{
		{IssueNodeID: "issue-1", PartyID: "plaintiff", SupportingRuleIDs: []string{"rule-a"}},
		{IssueNodeID: "issue-1", PartyID: "plaintiff", SupportingRuleIDs: []string{"rule-b"}},
	}

	conflicts := lawapplication.DetectConflictingAuthority("issue-1", []string{"rule-a", "rule-b"}, args)
	if len(conflicts) != 0 {
		t.Errorf("conflicts = %v, want none (same party citing both rules)", conflicts)
	}
}

func TestDetectConflictingAuthority_NoConflictWhenOverlappingParties(t *testing.T) {
	args := []lawapplication.ArgumentRef{
		{IssueNodeID: "issue-1", PartyID: "plaintiff", SupportingRuleIDs: []string{"rule-a"}},
		{IssueNodeID: "issue-1", PartyID: "plaintiff", SupportingRuleIDs: []string{"rule-b"}},
		{IssueNodeID: "issue-1", PartyID: "defendant", SupportingRuleIDs: []string{"rule-b"}},
	}

	// rule-a: {plaintiff}; rule-b: {plaintiff, defendant} -> overlapping, not disjoint.
	conflicts := lawapplication.DetectConflictingAuthority("issue-1", []string{"rule-a", "rule-b"}, args)
	if len(conflicts) != 0 {
		t.Errorf("conflicts = %v, want none (overlapping party sets)", conflicts)
	}
}

func TestDetectConflictingAuthority_NoConflictWhenRuleUncited(t *testing.T) {
	// rule-c is "controlling" (e.g. via a governs edge) but no argument
	// cites it at all -- it has no party set, so it cannot conflict.
	args := []lawapplication.ArgumentRef{
		{IssueNodeID: "issue-1", PartyID: "plaintiff", SupportingRuleIDs: []string{"rule-a"}},
	}

	conflicts := lawapplication.DetectConflictingAuthority("issue-1", []string{"rule-a", "rule-c"}, args)
	if len(conflicts) != 0 {
		t.Errorf("conflicts = %v, want none (rule-c uncited)", conflicts)
	}
}

func TestDetectConflictingAuthority_SingleRuleNeverConflicts(t *testing.T) {
	args := []lawapplication.ArgumentRef{
		{IssueNodeID: "issue-1", PartyID: "plaintiff", SupportingRuleIDs: []string{"rule-a"}},
	}
	conflicts := lawapplication.DetectConflictingAuthority("issue-1", []string{"rule-a"}, args)
	if len(conflicts) != 0 {
		t.Errorf("conflicts = %v, want none (only one rule)", conflicts)
	}
}
