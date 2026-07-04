package localization_test

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/citation"
	"github.com/YASSERRMD/verdex/packages/localization"
)

// stubReport is a local stand-in shaped like
// packages/reportexport.Report (CaseTitle, JurisdictionKey,
// Issues[].IssueNodeID/Analysis/Citations), exactly the "documented
// adapter function signature plus a unit test using a local stand-in
// struct" option this phase's brief calls out (see report.go's doc
// comment) -- this package does not import packages/reportexport.
type stubReport struct {
	CaseTitle       string
	JurisdictionKey string
	GeneratedAt     time.Time
	Issues          []stubReportIssue
}

// stubReportIssue mirrors packages/reportexport.ReportIssue's shape
// closely enough (IssueNodeID, Analysis, and a citation list) that
// adapting it into a []localization.ReportIssueLike is a one-line
// field copy per entry, as toReportLike below demonstrates.
type stubReportIssue struct {
	IssueNodeID string
	Analysis    string
	Citations   []stubReportCitation
}

type stubReportCitation struct {
	RuleID      string
	FormatInput citation.FormatInput
}

// toReportLike adapts a stubReport into a localization.ReportLike --
// the one-line-per-field copy a real integration against
// packages/reportexport.Report would perform identically.
func toReportLike(r stubReport) localization.ReportLike {
	issues := make([]localization.ReportIssueLike, 0, len(r.Issues))
	for _, issue := range r.Issues {
		cites := make([]localization.ReportCitationInput, 0, len(issue.Citations))
		for _, c := range issue.Citations {
			cites = append(cites, localization.ReportCitationInput{
				RuleID:      c.RuleID,
				FormatInput: c.FormatInput,
			})
		}
		issues = append(issues, localization.ReportIssueLike{
			ID:        issue.IssueNodeID,
			Analysis:  issue.Analysis,
			Citations: cites,
		})
	}
	return localization.ReportLike{
		CaseTitle:       r.CaseTitle,
		JurisdictionKey: r.JurisdictionKey,
		GeneratedAt:     r.GeneratedAt,
		Issues:          issues,
	}
}

func TestLocalizeReportBasicStructure(t *testing.T) {
	stub := stubReport{
		CaseTitle:       "State v. Example",
		JurisdictionKey: "common_law",
		GeneratedAt:     time.Date(2026, time.July, 4, 9, 0, 0, 0, time.UTC),
		Issues: []stubReportIssue{
			{
				IssueNodeID: uuid.NewString(),
				Analysis:    "The draft, non-binding analysis text for issue one.",
				Citations: []stubReportCitation{
					{
						RuleID: uuid.NewString(),
						FormatInput: citation.FormatInput{
							Origin:      citation.OriginPrecedent,
							CaseName:    "Smith v Jones",
							RawCitation: "[2020] UKSC 1",
						},
					},
				},
			},
		},
	}

	cat := localization.NewSeededCatalog()
	registry := citation.NewDefaultRegistry()

	localized := localization.LocalizeReport(cat, registry, localization.LocaleArabic, toReportLike(stub))

	if localized.Locale != localization.LocaleArabic {
		t.Errorf("Locale = %q, want ar", localized.Locale)
	}
	if localized.Direction != localization.DirectionRTL {
		t.Errorf("Direction = %q, want rtl", localized.Direction)
	}
	if localized.CaseTitle != stub.CaseTitle {
		t.Errorf("CaseTitle = %q, want %q", localized.CaseTitle, stub.CaseTitle)
	}
	if len(localized.Sections) != 1 {
		t.Fatalf("len(Sections) = %d, want 1", len(localized.Sections))
	}

	section := localized.Sections[0]
	if section.Label != "المسألة" {
		t.Errorf("Sections[0].Label = %q, want المسألة (Arabic for Issue)", section.Label)
	}
	if section.Analysis != stub.Issues[0].Analysis {
		t.Errorf("Sections[0].Analysis = %q, want unchanged analysis text", section.Analysis)
	}
	if len(section.Citations) != 1 {
		t.Fatalf("len(Sections[0].Citations) = %d, want 1", len(section.Citations))
	}
	// Contains, not HasPrefix: the Arabic locale wraps the citation in
	// bidi embedding controls (see citation.go's doc comment), so the
	// case name is no longer the literal first character.
	if !strings.Contains(section.Citations[0], "Smith v Jones") {
		t.Errorf("Sections[0].Citations[0] = %q, want it to contain the case name", section.Citations[0])
	}

	if !strings.Contains(localized.GeneratedAtText, "يوليو") {
		t.Errorf("GeneratedAtText = %q, want it to contain the Arabic month name", localized.GeneratedAtText)
	}

	if localized.DisclaimerText == "" {
		t.Errorf("DisclaimerText is empty, want the localized non-binding disclaimer")
	}
	if !strings.Contains(localized.DisclaimerText, "غير ملزم") {
		t.Errorf("DisclaimerText = %q, want it to contain the Arabic non-binding wording", localized.DisclaimerText)
	}
}

// TestLocalizeReportUnknownJurisdictionDegradesGracefully asserts an
// unregistered/blank JurisdictionKey does not cause LocalizeReport to
// drop every citation -- it falls back to CommonLawFormatter rather
// than erroring out or silently omitting the citation list.
func TestLocalizeReportUnknownJurisdictionDegradesGracefully(t *testing.T) {
	stub := stubReport{
		CaseTitle:       "Unknown Jurisdiction Case",
		JurisdictionKey: "some-unregistered-key",
		GeneratedAt:     time.Now(),
		Issues: []stubReportIssue{
			{
				IssueNodeID: uuid.NewString(),
				Analysis:    "Analysis text.",
				Citations: []stubReportCitation{
					{
						FormatInput: citation.FormatInput{
							Origin:  citation.OriginStatute,
							Act:     "Some Act",
							Section: "1",
						},
					},
				},
			},
		},
	}

	localized := localization.LocalizeReport(nil, nil, localization.LocaleEnglish, toReportLike(stub))
	if len(localized.Sections) != 1 || len(localized.Sections[0].Citations) != 1 {
		t.Fatalf("expected exactly one citation to survive graceful degradation, got %+v", localized.Sections)
	}
	if localized.Sections[0].Citations[0] != "Some Act, s.1" {
		t.Errorf("Citations[0] = %q, want %q (CommonLawFormatter fallback)", localized.Sections[0].Citations[0], "Some Act, s.1")
	}
}

// TestLocalizeReportEmptyIssuesProducesEmptySections covers the
// zero-issue edge case.
func TestLocalizeReportEmptyIssuesProducesEmptySections(t *testing.T) {
	stub := stubReport{CaseTitle: "Empty Case", GeneratedAt: time.Now()}
	localized := localization.LocalizeReport(nil, nil, localization.LocaleEnglish, toReportLike(stub))
	if len(localized.Sections) != 0 {
		t.Errorf("len(Sections) = %d, want 0", len(localized.Sections))
	}
}

func TestDisclaimerTextCoordinatesWithGuardrailSubstance(t *testing.T) {
	cat := localization.NewSeededCatalog()
	text := localization.DisclaimerText(cat, localization.LocaleEnglish)
	// Substance check mirroring packages/guardrail's outputDisclaimer
	// and apps/web's Disclaimer.tsx: must name both "non-binding" and
	// the judge sign-off requirement, never a bare pass-through of an
	// empty string.
	if !strings.Contains(strings.ToLower(text), "non-binding") {
		t.Errorf("DisclaimerText(en) = %q, want it to mention non-binding", text)
	}
	if !strings.Contains(text, "judge") {
		t.Errorf("DisclaimerText(en) = %q, want it to mention judge review/sign-off", text)
	}
}
