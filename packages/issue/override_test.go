package issue_test

import (
	"errors"
	"testing"
	"time"

	"github.com/YASSERRMD/verdex/packages/issue"
)

func TestApplyOverride_TakesPrecedenceButRetainsOriginal(t *testing.T) {
	original := issue.CandidateIssue{
		ID:         "issue-1",
		Text:       "whether the contract was breched", // typo, to be corrected
		Confidence: 0.4,
	}
	override := issue.ManualOverride{
		IssueID:    "issue-1",
		Text:       "whether the contract was breached",
		Material:   true,
		ReviewedBy: "reviewer-1",
	}

	got, err := issue.ApplyOverride(original, override)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.Text != override.Text {
		t.Errorf("expected corrected text %q, got %q", override.Text, got.Text)
	}
	if got.Confidence != 1.0 {
		t.Errorf("expected overridden confidence 1.0, got %v", got.Confidence)
	}
	if got.Override.Previous == nil {
		t.Fatalf("expected original candidate to be retained on Override.Previous")
	}
	if got.Override.Previous.Text != original.Text {
		t.Errorf("expected preserved original text %q, got %q", original.Text, got.Override.Previous.Text)
	}
}

func TestApplyOverride_MismatchedIssueID(t *testing.T) {
	original := issue.CandidateIssue{ID: "issue-1", Text: "whether x"}
	override := issue.ManualOverride{IssueID: "issue-2", Text: "whether y"}

	_, err := issue.ApplyOverride(original, override)
	if !errors.Is(err, issue.ErrInvalidOverride) {
		t.Fatalf("expected ErrInvalidOverride for mismatched IssueID, got %v", err)
	}
}

func TestApplyOverride_EmptyText(t *testing.T) {
	original := issue.CandidateIssue{ID: "issue-1", Text: "whether x"}
	override := issue.ManualOverride{IssueID: "issue-1", Text: "   "}

	_, err := issue.ApplyOverride(original, override)
	if !errors.Is(err, issue.ErrInvalidOverride) {
		t.Fatalf("expected ErrInvalidOverride for blank text, got %v", err)
	}
}

func TestApplyOverride_DefaultsReviewedAt(t *testing.T) {
	original := issue.CandidateIssue{ID: "issue-1", Text: "whether x"}
	override := issue.ManualOverride{IssueID: "issue-1", Text: "whether y"}

	before := time.Now()
	got, err := issue.ApplyOverride(original, override)
	after := time.Now()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Override.ReviewedAt.Before(before) || got.Override.ReviewedAt.After(after) {
		t.Errorf("expected ReviewedAt to default to now, got %v (window %v - %v)", got.Override.ReviewedAt, before, after)
	}
}
