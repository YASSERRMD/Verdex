package reportexport_test

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/reportexport"
)

func TestRedact_MasksPIIInAnalysisAndWeakestLink(t *testing.T) {
	c := newTestCase(uuid.New())
	opinion := newTestOpinion(c.ID, "Contact the witness at jane.doe@example.com for corroboration.")
	opinion.Conclusions[0].WeakestLink = "Reach the paralegal at jane.doe@example.com to confirm."
	report := newAssembledReport(t, c, opinion)

	redacted, err := reportexport.Redact(context.Background(), report, reportexport.RedactionOptions{})
	if err != nil {
		t.Fatalf("Redact: %v", err)
	}

	if strings.Contains(redacted.Issues[0].Analysis, "jane.doe@example.com") {
		t.Errorf("Redact did not remove the email from Analysis: %q", redacted.Issues[0].Analysis)
	}
	if !strings.Contains(redacted.Issues[0].Analysis, "[REDACTED:") {
		t.Errorf("Redact did not insert a redaction placeholder in Analysis: %q", redacted.Issues[0].Analysis)
	}
	if strings.Contains(redacted.Issues[0].WeakestLink, "jane.doe@example.com") {
		t.Errorf("Redact did not remove the email from WeakestLink: %q", redacted.Issues[0].WeakestLink)
	}
}

func TestRedact_LeavesStructuralFieldsUntouched(t *testing.T) {
	c := newTestCase(uuid.New())
	opinion := newTestOpinion(c.ID, "Contact jane.doe@example.com for details.")
	report := newAssembledReport(t, c, opinion)

	redacted, err := reportexport.Redact(context.Background(), report, reportexport.RedactionOptions{})
	if err != nil {
		t.Fatalf("Redact: %v", err)
	}

	if redacted.Issues[0].IssueNodeID != report.Issues[0].IssueNodeID {
		t.Errorf("Redact changed IssueNodeID")
	}
	if redacted.Issues[0].FavoredParty != report.Issues[0].FavoredParty {
		t.Errorf("Redact changed FavoredParty")
	}
	if redacted.Issues[0].Confidence != report.Issues[0].Confidence {
		t.Errorf("Redact changed Confidence")
	}
	if len(redacted.Issues[0].Citations) != len(report.Issues[0].Citations) {
		t.Fatalf("Redact changed citation count")
	}
	if redacted.Issues[0].Citations[0].Text != report.Issues[0].Citations[0].Text {
		t.Errorf("Redact changed citation text, want it untouched")
	}
}

func TestRedact_DoesNotMutateOriginalReport(t *testing.T) {
	c := newTestCase(uuid.New())
	opinion := newTestOpinion(c.ID, "Contact jane.doe@example.com for details.")
	report := newAssembledReport(t, c, opinion)
	original := report.Issues[0].Analysis

	if _, err := reportexport.Redact(context.Background(), report, reportexport.RedactionOptions{}); err != nil {
		t.Fatalf("Redact: %v", err)
	}

	if report.Issues[0].Analysis != original {
		t.Errorf("Redact mutated the original report's Analysis field")
	}
}

func TestRedact_RenderedOutputExcludesPII(t *testing.T) {
	c := newTestCase(uuid.New())
	opinion := newTestOpinion(c.ID, "The witness at jane.doe@example.com confirmed the timeline.")
	report := newAssembledReport(t, c, opinion)

	redacted, err := reportexport.Redact(context.Background(), report, reportexport.RedactionOptions{})
	if err != nil {
		t.Fatalf("Redact: %v", err)
	}

	md, err := reportexport.RenderMarkdown(redacted)
	if err != nil {
		t.Fatalf("RenderMarkdown: %v", err)
	}
	if strings.Contains(md, "jane.doe@example.com") {
		t.Errorf("Rendered Markdown of a redacted report still contains PII: %q", md)
	}

	pdfBytes, err := reportexport.RenderPDF(redacted)
	if err != nil {
		t.Fatalf("RenderPDF: %v", err)
	}
	if strings.Contains(string(pdfBytes), "jane.doe@example.com") {
		t.Errorf("Rendered PDF of a redacted report still contains PII")
	}
}

func TestRedact_NilReport(t *testing.T) {
	if _, err := reportexport.Redact(context.Background(), nil, reportexport.RedactionOptions{}); err != reportexport.ErrNilCase {
		t.Errorf("Redact(nil) err = %v, want ErrNilCase", err)
	}
}
