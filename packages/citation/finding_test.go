package citation_test

import (
	"testing"

	"github.com/YASSERRMD/verdex/packages/citation"
	"github.com/YASSERRMD/verdex/packages/irac"
)

func TestFindingsFromVerification(t *testing.T) {
	cases := []struct {
		name     string
		result   citation.VerificationResult
		wantLen  int
		wantSev  citation.Severity
		wantCode string
	}{
		{
			name:    "verified produces no finding",
			result:  citation.VerificationResult{Status: citation.StatusVerified},
			wantLen: 0,
		},
		{
			name:     "hallucinated is critical",
			result:   citation.VerificationResult{Status: citation.StatusHallucinated, Unit: citation.CitedUnit{NodeID: "n", CaseID: "c"}},
			wantLen:  1,
			wantSev:  citation.SeverityCritical,
			wantCode: citation.CodeHallucinated,
		},
		{
			name:     "wrong case is critical",
			result:   citation.VerificationResult{Status: citation.StatusWrongCase, Unit: citation.CitedUnit{NodeID: "n", CaseID: "c"}, ActualCaseID: "other"},
			wantLen:  1,
			wantSev:  citation.SeverityCritical,
			wantCode: citation.CodeWrongCase,
		},
		{
			name:     "broken is critical",
			result:   citation.VerificationResult{Status: citation.StatusBroken, Unit: citation.CitedUnit{NodeID: "n", CaseID: "c"}},
			wantLen:  1,
			wantSev:  citation.SeverityCritical,
			wantCode: citation.CodeBrokenDeleted,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			findings := citation.FindingsFromVerification(tc.result)
			if len(findings) != tc.wantLen {
				t.Fatalf("len(findings) = %d, want %d", len(findings), tc.wantLen)
			}
			if tc.wantLen == 0 {
				return
			}
			if findings[0].Severity != tc.wantSev {
				t.Errorf("Severity = %q, want %q", findings[0].Severity, tc.wantSev)
			}
			if findings[0].Code != tc.wantCode {
				t.Errorf("Code = %q, want %q", findings[0].Code, tc.wantCode)
			}
		})
	}
}

func TestFindingsFromUnitMissingCitationAndSpans(t *testing.T) {
	unit := citation.CitedUnit{NodeID: "n", CaseID: "c"}
	findings := citation.FindingsFromUnit(unit)
	if len(findings) != 2 {
		t.Fatalf("len(findings) = %d, want 2", len(findings))
	}
	for _, f := range findings {
		if f.Severity != citation.SeverityWarning {
			t.Errorf("Severity = %q, want %q", f.Severity, citation.SeverityWarning)
		}
	}
}

func TestFindingsFromUnitFullyPopulated(t *testing.T) {
	unit := citation.CitedUnit{
		NodeID:   "n",
		CaseID:   "c",
		Citation: "Act 1, s.1",
		Spans:    irac.Spans{{Start: 0, End: 5}},
	}

	findings := citation.FindingsFromUnit(unit)
	if len(findings) != 0 {
		t.Errorf("len(findings) = %d, want 0 for fully populated unit", len(findings))
	}
}

func TestReportSummaryAndHasCritical(t *testing.T) {
	report := citation.Report{
		CaseID: "case-1",
		Findings: []citation.Finding{
			{Severity: citation.SeverityCritical},
			{Severity: citation.SeverityWarning},
			{Severity: citation.SeverityWarning},
			{Severity: citation.SeverityInfo},
		},
	}

	want := "1 critical, 2 warning, 1 info (4 total)"
	if got := report.Summary(); got != want {
		t.Errorf("Summary() = %q, want %q", got, want)
	}
	if !report.HasCritical() {
		t.Error("HasCritical() = false, want true")
	}

	clean := citation.Report{Findings: []citation.Finding{{Severity: citation.SeverityInfo}}}
	if clean.HasCritical() {
		t.Error("HasCritical() = true, want false")
	}
}
