package privacy_test

import (
	"context"
	"strings"
	"testing"

	"github.com/YASSERRMD/verdex/packages/pii"
	"github.com/YASSERRMD/verdex/packages/privacy"
)

// TestAnonymizeForAnalytics_RedactsContact proves AnonymizeForAnalytics
// actually detects and redacts personal data via packages/pii's own
// detection pipeline, rather than passing text through unchanged.
func TestAnonymizeForAnalytics_RedactsContact(t *testing.T) {
	t.Parallel()

	records := []privacy.AnalyticsRecord{
		{FieldName: "notes", Text: "Contact Jane Doe at jane.doe@example.com for follow-up."},
	}

	result, err := privacy.AnonymizeForAnalytics(context.Background(), "subject-1", privacy.CategoryContact, records, "")
	if err != nil {
		t.Fatalf("AnonymizeForAnalytics: %v", err)
	}

	if len(result.Fields) != 1 {
		t.Fatalf("len(result.Fields) = %d, want 1", len(result.Fields))
	}
	field := result.Fields[0]
	if strings.Contains(field.Text, "jane.doe@example.com") {
		t.Fatalf("field.Text = %q, still contains the raw email address", field.Text)
	}
	if field.MatchCount == 0 {
		t.Fatal("field.MatchCount = 0, want at least 1 (the email address)")
	}
	if result.TotalMatches != field.MatchCount {
		t.Fatalf("result.TotalMatches = %d, want %d", result.TotalMatches, field.MatchCount)
	}
	if result.SubjectID != "subject-1" {
		t.Fatalf("result.SubjectID = %q, want %q", result.SubjectID, "subject-1")
	}
}

// TestAnonymizeForAnalytics_NoPII proves a field with no detectable
// PII passes through with MatchCount == 0 and unchanged text.
func TestAnonymizeForAnalytics_NoPII(t *testing.T) {
	t.Parallel()

	records := []privacy.AnalyticsRecord{
		{FieldName: "summary", Text: "The hearing was rescheduled to next week."},
	}

	result, err := privacy.AnonymizeForAnalytics(context.Background(), "subject-2", privacy.CategoryBehavioral, records, "")
	if err != nil {
		t.Fatalf("AnonymizeForAnalytics: %v", err)
	}
	if result.TotalMatches != 0 {
		t.Fatalf("result.TotalMatches = %d, want 0", result.TotalMatches)
	}
	if result.Fields[0].Text != "The hearing was rescheduled to next week." {
		t.Fatalf("field.Text = %q, want unchanged (no PII detected)", result.Fields[0].Text)
	}
}

// TestAnonymizeForAnalytics_MultipleFields proves every input
// AnalyticsRecord produces exactly one corresponding AnonymizedField,
// in the same order.
func TestAnonymizeForAnalytics_MultipleFields(t *testing.T) {
	t.Parallel()

	records := []privacy.AnalyticsRecord{
		{FieldName: "party_name", Text: "John Smith"},
		{FieldName: "party_email", Text: "john.smith@example.com"},
		{FieldName: "summary", Text: "No personal data here."},
	}

	result, err := privacy.AnonymizeForAnalytics(context.Background(), "subject-3", privacy.CategoryCaseParty, records, "")
	if err != nil {
		t.Fatalf("AnonymizeForAnalytics: %v", err)
	}
	if len(result.Fields) != 3 {
		t.Fatalf("len(result.Fields) = %d, want 3", len(result.Fields))
	}
	for i, want := range []string{"party_name", "party_email", "summary"} {
		if result.Fields[i].FieldName != want {
			t.Fatalf("result.Fields[%d].FieldName = %q, want %q", i, result.Fields[i].FieldName, want)
		}
	}
}

// TestAnonymizeForAnalytics_PseudonymizeMode proves a caller can
// request pii.ModePseudonymize instead of the default irreversible
// redaction, still delegating entirely to packages/pii's Redactor.
func TestAnonymizeForAnalytics_PseudonymizeMode(t *testing.T) {
	t.Parallel()

	records := []privacy.AnalyticsRecord{
		{FieldName: "notes", Text: "Reach Jane Doe at jane.doe@example.com."},
	}

	result, err := privacy.AnonymizeForAnalytics(context.Background(), "subject-4", privacy.CategoryContact, records, pii.ModePseudonymize)
	if err != nil {
		t.Fatalf("AnonymizeForAnalytics: %v", err)
	}
	if strings.Contains(result.Fields[0].Text, "jane.doe@example.com") {
		t.Fatalf("field.Text = %q, still contains the raw email address under pseudonymize mode", result.Fields[0].Text)
	}
}
