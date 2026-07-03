package reportexport_test

import (
	"testing"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/citation"
	"github.com/YASSERRMD/verdex/packages/reportexport"
)

func TestAssemble_PullsRealDataFromCaseAndOpinion(t *testing.T) {
	tenantID := uuid.New()
	c := newTestCase(tenantID)
	opinion := newTestOpinion(c.ID, "The evidence favors the first party on the breach issue.")

	report := newAssembledReport(t, c, opinion)

	if report.CaseID != c.ID {
		t.Errorf("CaseID = %v, want %v", report.CaseID, c.ID)
	}
	if report.TenantID != c.TenantID {
		t.Errorf("TenantID = %v, want %v", report.TenantID, c.TenantID)
	}
	if report.CaseTitle != c.Title {
		t.Errorf("CaseTitle = %q, want %q", report.CaseTitle, c.Title)
	}
	if report.CaseReference != c.Reference {
		t.Errorf("CaseReference = %q, want %q", report.CaseReference, c.Reference)
	}
	if len(report.Issues) != 1 {
		t.Fatalf("len(Issues) = %d, want 1", len(report.Issues))
	}

	issue := report.Issues[0]
	want := opinion.Conclusions[0]
	if issue.IssueNodeID != want.IssueNodeID {
		t.Errorf("IssueNodeID = %q, want %q", issue.IssueNodeID, want.IssueNodeID)
	}
	if issue.Analysis != want.Text {
		t.Errorf("Analysis = %q, want %q", issue.Analysis, want.Text)
	}
	if issue.FavoredParty != want.FavoredParty {
		t.Errorf("FavoredParty = %q, want %q", issue.FavoredParty, want.FavoredParty)
	}
	if issue.Confidence != want.Confidence {
		t.Errorf("Confidence = %v, want %v", issue.Confidence, want.Confidence)
	}
	if issue.WeakestLink != want.WeakestLink {
		t.Errorf("WeakestLink = %q, want %q", issue.WeakestLink, want.WeakestLink)
	}
	if len(issue.SupportingFactIDs) != len(want.SupportingFactIDs) {
		t.Errorf("len(SupportingFactIDs) = %d, want %d", len(issue.SupportingFactIDs), len(want.SupportingFactIDs))
	}

	if len(report.SkippedIssueNodeIDs) != 1 || report.SkippedIssueNodeIDs[0] != "issue-2" {
		t.Errorf("SkippedIssueNodeIDs = %v, want [issue-2]", report.SkippedIssueNodeIDs)
	}
}

func TestAssemble_FormatsCitationsThroughCitationPackage(t *testing.T) {
	tenantID := uuid.New()
	c := newTestCase(tenantID)
	opinion := newTestOpinion(c.ID, "Analysis text.")

	report := newAssembledReport(t, c, opinion)

	if len(report.Issues[0].Citations) != 1 {
		t.Fatalf("len(Citations) = %d, want 1", len(report.Issues[0].Citations))
	}
	got := report.Issues[0].Citations[0]

	// citation.CommonLawFormatter renders a statute as "<Act>, s.<Section>".
	want := citation.CommonLawFormatter.Format(citation.FormatInput{
		Origin:  citation.OriginStatute,
		Act:     "Contracts Act",
		Section: "12",
	})
	if got.Text != want {
		t.Errorf("Citations[0].Text = %q, want %q (from citation.CommonLawFormatter, not hand-assembled)", got.Text, want)
	}
	if !got.Resolved || !got.Verified {
		t.Errorf("Citations[0] = %+v, want Resolved=true Verified=true", got)
	}
}

func TestAssemble_NilCaseOrOpinion(t *testing.T) {
	tenantID := uuid.New()
	c := newTestCase(tenantID)
	opinion := newTestOpinion(c.ID, "text")

	if _, err := reportexport.Assemble(nil, opinion, reportexport.AssembleInput{}); err != reportexport.ErrNilCase {
		t.Errorf("Assemble(nil case) err = %v, want ErrNilCase", err)
	}
	if _, err := reportexport.Assemble(c, nil, reportexport.AssembleInput{}); err != reportexport.ErrNilOpinion {
		t.Errorf("Assemble(nil opinion) err = %v, want ErrNilOpinion", err)
	}
}

func TestAssemble_UnknownJurisdictionKeyFallsBackToRawCitation(t *testing.T) {
	tenantID := uuid.New()
	c := newTestCase(tenantID)
	opinion := newTestOpinion(c.ID, "text")

	input := reportexport.AssembleInput{
		JurisdictionKey: "unknown_legal_family",
		Citations:       citation.NewDefaultRegistry(),
		AuthorityTrailsByIssue: map[string][]reportexport.AuthorityCitationInput{
			"issue-1": {
				{
					RuleID: "rule-1",
					FormatInput: citation.FormatInput{
						Origin:      citation.OriginPrecedent,
						RawCitation: "[2020] UKSC 1",
					},
				},
			},
		},
	}

	report, err := reportexport.Assemble(c, opinion, input)
	if err != nil {
		t.Fatalf("Assemble: %v", err)
	}
	if got := report.Issues[0].Citations[0].Text; got != "[2020] UKSC 1" {
		t.Errorf("Citations[0].Text = %q, want raw fallback %q", got, "[2020] UKSC 1")
	}
}
