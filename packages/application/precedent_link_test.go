package application_test

import (
	"errors"
	"testing"
	"time"

	"github.com/YASSERRMD/verdex/packages/application"
)

func TestNewPrecedentIssueLink_Success(t *testing.T) {
	rule := application.OriginatedRule{
		Rule:   testRule(t, "rule-1", "precedent text", "US-CA", "common_law"),
		Origin: application.OriginPrecedent,
	}

	link, err := application.NewPrecedentIssueLink("issue-1", rule, "same contested element", time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if link.IssueID != "issue-1" {
		t.Fatalf("expected issue-1, got %s", link.IssueID)
	}
	if link.Rationale != "same contested element" {
		t.Fatalf("unexpected rationale: %s", link.Rationale)
	}
	if link.Rule.Rule.ID != "rule-1" {
		t.Fatalf("expected rule-1, got %s", link.Rule.Rule.ID)
	}
}

func TestNewPrecedentIssueLink_RejectsNonPrecedentOrigin(t *testing.T) {
	rule := application.OriginatedRule{
		Rule:   testRule(t, "rule-1", "statute text", "US-CA", "civil_law"),
		Origin: application.OriginStatute,
	}

	_, err := application.NewPrecedentIssueLink("issue-1", rule, "rationale", time.Now())
	if err == nil {
		t.Fatal("expected error for statute-origin rule")
	}
}

func TestNewPrecedentIssueLink_RejectsEmptyInput(t *testing.T) {
	rule := application.OriginatedRule{
		Rule:   testRule(t, "rule-1", "precedent text", "US-CA", "common_law"),
		Origin: application.OriginPrecedent,
	}

	if _, err := application.NewPrecedentIssueLink("", rule, "rationale", time.Now()); !errors.Is(err, application.ErrEmptyInput) {
		t.Fatalf("expected ErrEmptyInput for blank issue ID, got %v", err)
	}
	if _, err := application.NewPrecedentIssueLink("issue-1", rule, "", time.Now()); !errors.Is(err, application.ErrEmptyInput) {
		t.Fatalf("expected ErrEmptyInput for blank rationale, got %v", err)
	}
}
