// report.go implements task 5: localizing a generated report.
//
// This package deliberately does not import packages/reportexport.
// packages/reportexport (Phase 073) already pulls in
// packages/citation, packages/synthesisagent, packages/caselifecycle,
// and packages/reasoningtrace to assemble and render a Report; a hard
// import here would drag that entire dependency graph into what should
// stay a light, UI-adjacent localization package (see doc.go's
// composition table). Instead:
//
//   - ReportLike is a minimal structural interface describing exactly
//     the handful of fields LocalizeReport needs from a report-shaped
//     value: a jurisdiction key (to pick a citation.Formatter) and a
//     list of section-like entries with a label and body.
//   - LocalizeReport is a documented adapter: given a ReportLike, a
//     Catalog, a citation.Registry, and a target Locale, it produces a
//     LocalizedReport whose section labels are translated (falling
//     back to the original label when no translation exists) and
//     whose embedded dates/numbers/citations are re-rendered through
//     this package's own locale-aware formatting.
//
// report_test.go exercises this against a local stand-in struct shaped
// like packages/reportexport.Report (CaseTitle, JurisdictionKey,
// Issues[].IssueNodeID/Analysis/Citations), not the real
// reportexport.Report -- exactly the "documented adapter function
// signature plus a unit test using a local stand-in struct" option
// this phase's brief calls out. A future phase wiring this directly
// against packages/reportexport only needs reportexport.Report (and
// reportexport.ReportIssue) to satisfy ReportLike (they already
// structurally do: CaseTitle, JurisdictionKey, and an Issues slice with
// IssueNodeID/Analysis/Citations are already present on those exact
// types per packages/reportexport/types.go) -- no change to this
// package would be required.
package localization

import (
	"time"

	"github.com/YASSERRMD/verdex/packages/citation"
)

// ReportIssueLike describes the minimal shape LocalizeReport needs from
// one report section/issue entry. Any type with these fields --
// including packages/reportexport.ReportIssue -- can be adapted to
// satisfy it with a one-line field copy, without this package
// importing packages/reportexport.
type ReportIssueLike struct {
	// ID identifies this issue/section (e.g. reportexport.ReportIssue's
	// IssueNodeID), used only for stable ordering/keys, never
	// rendered.
	ID string

	// Analysis is this issue's non-binding draft analysis text.
	Analysis string

	// Citations is every citation.FormatInput supporting this issue's
	// analysis, alongside the RuleID it belongs to -- mirroring
	// reportexport.AuthorityCitationInput's shape closely enough that
	// building a []ReportIssueLike from a []reportexport.ReportIssue is
	// a one-line field copy (see report_test.go).
	Citations []ReportCitationInput
}

// ReportCitationInput is the per-citation input LocalizeReport needs:
// enough of a citation.FormatInput to re-render the citation text
// through LocalizeCitation, plus the RuleID it supports.
type ReportCitationInput struct {
	RuleID      string
	FormatInput citation.FormatInput
}

// ReportLike is the minimal structural shape LocalizeReport needs from
// a report-shaped value. packages/reportexport.Report already has a
// CaseTitle, JurisdictionKey, and Issues field of a structurally
// compatible shape (see this file's doc comment) -- LocalizeReport
// takes a ReportLike built from any such value rather than the
// concrete reportexport.Report type, so this package never needs to
// import it.
type ReportLike struct {
	// CaseTitle is the report's case title.
	CaseTitle string

	// JurisdictionKey selects a citation.Formatter from the supplied
	// citation.Registry, exactly as reportexport.Report.JurisdictionKey
	// does.
	JurisdictionKey string

	// GeneratedAt is when the underlying analysis was generated
	// (reportexport.Report.OpinionGeneratedAt), localized into
	// LocalizedReport.GeneratedAtText via FormatDateTime.
	GeneratedAt time.Time

	// Issues is one entry per report section/issue.
	Issues []ReportIssueLike
}

// LocalizedSection is one section of a LocalizedReport: a translated
// label, the section's (untranslated -- see doc/localization.md for why
// full analysis-text machine translation is out of scope for this
// phase) analysis body, and its citations re-rendered through
// LocalizeCitation.
type LocalizedSection struct {
	ID        string
	Label     string
	Analysis  string
	Citations []string
}

// LocalizedReport is LocalizeReport's output: a Locale-tagged,
// Direction-aware rendering of a ReportLike's structure, with the
// mandatory non-binding disclaimer already appended in the target
// locale (see DisclaimerText) -- mirroring
// packages/guardrail.RequireDisclaimer's "always attach it" discipline,
// applied here in translated form rather than skipped for non-English
// output.
type LocalizedReport struct {
	Locale          Locale
	Direction       Direction
	CaseTitle       string
	GeneratedAtText string
	Sections        []LocalizedSection
	DisclaimerText  string
}

// LocalizeReport adapts report into a LocalizedReport for locale (task
// 5): translates each section's label via cat (falling back through
// Translate's normal fallback-to-English rule when locale lacks a
// section-label translation), formats GeneratedAt via FormatDateTime,
// re-renders every citation via LocalizeCitation using a
// citation.Formatter resolved from registry by report.JurisdictionKey
// (degrading to citation.CommonLawFormatter if JurisdictionKey is
// unregistered and registry has no fallback configured -- report
// localization should never drop every citation from the output just
// because a jurisdiction key was blank or unrecognized), and appends
// the translated non-binding disclaimer.
//
// If registry is nil, citation.NewDefaultRegistry() is used, mirroring
// reportexport.AssembleInput.Citations's own "nil means default
// registry" convention. If cat is nil, NewSeededCatalog() is used.
func LocalizeReport(cat *Catalog, registry *citation.Registry, locale Locale, report ReportLike) LocalizedReport {
	if cat == nil {
		cat = NewSeededCatalog()
	}
	if registry == nil {
		registry = citation.NewDefaultRegistry()
	}
	formatter := resolveFormatter(registry, report.JurisdictionKey)

	sections := make([]LocalizedSection, 0, len(report.Issues))
	for _, issue := range report.Issues {
		sections = append(sections, localizeSection(cat, formatter, locale, issue))
	}

	return LocalizedReport{
		Locale:          locale,
		Direction:       DirectionFor(locale),
		CaseTitle:       report.CaseTitle,
		GeneratedAtText: FormatDateTime(cat, locale, report.GeneratedAt),
		Sections:        sections,
		DisclaimerText:  DisclaimerText(cat, locale),
	}
}

// resolveFormatter adapts registry's key-based Format method into a
// plain citation.Formatter bound to jurisdictionKey, so callers here
// can treat "resolve once per report" and "format once per citation"
// as separate steps. Falls back to citation.CommonLawFormatter if
// registry has no entry (and no configured fallback) for
// jurisdictionKey.
func resolveFormatter(registry *citation.Registry, jurisdictionKey string) citation.Formatter {
	if registry.Has(jurisdictionKey) {
		key := jurisdictionKey
		return citation.FormatterFunc(func(in citation.FormatInput) string {
			text, err := registry.Format(key, in)
			if err != nil {
				return citation.CommonLawFormatter.Format(in)
			}
			return text
		})
	}
	return citation.CommonLawFormatter
}

// localizeSection localizes one ReportIssueLike into a
// LocalizedSection.
func localizeSection(cat *Catalog, formatter citation.Formatter, locale Locale, issue ReportIssueLike) LocalizedSection {
	citationTexts := make([]string, 0, len(issue.Citations))
	for _, c := range issue.Citations {
		text, err := LocalizeCitation(formatter, locale, c.FormatInput)
		if err != nil {
			continue
		}
		citationTexts = append(citationTexts, text)
	}

	return LocalizedSection{
		ID:        issue.ID,
		Label:     Translate(cat, locale, normalizeKey("report_section", "issue")),
		Analysis:  issue.Analysis,
		Citations: citationTexts,
	}
}

// DisclaimerText returns the localized non-binding disclaimer (title +
// body, joined by ": ") for locale, via cat's
// "disclaimer.non_binding_title"/"disclaimer.non_binding_body" seeded
// keys (see seed.go).
func DisclaimerText(cat *Catalog, locale Locale) string {
	title := Translate(cat, locale, "disclaimer.non_binding_title")
	body := Translate(cat, locale, "disclaimer.non_binding_body")
	return title + ": " + body
}
