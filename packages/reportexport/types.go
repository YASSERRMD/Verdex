package reportexport

import (
	"time"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/citation"
)

// Format identifies a rendered report's output encoding.
type Format string

const (
	// FormatPDF renders the report as a PDF document (see pdf.go).
	FormatPDF Format = "pdf"

	// FormatDOCX renders the report as an Office Open XML (.docx)
	// document (see docx.go).
	FormatDOCX Format = "docx"

	// FormatMarkdown renders the report as a standalone Markdown
	// document (see markdown.go).
	FormatMarkdown Format = "markdown"

	// FormatText renders the report as plain text (see markdown.go).
	FormatText Format = "text"
)

// IsValid reports whether f is one of the recognized Format constants.
func (f Format) IsValid() bool {
	switch f {
	case FormatPDF, FormatDOCX, FormatMarkdown, FormatText:
		return true
	default:
		return false
	}
}

// ReportCitation is one jurisdiction-formatted citation supporting a
// ReportIssue's analysis, pairing the rendered citation text (via
// packages/citation's Formatter) with its resolution/verification
// status carried over from the underlying reasoning trace.
type ReportCitation struct {
	// RuleID is the irac.RuleNode.ID this citation supports.
	RuleID string

	// Text is the jurisdiction-formatted citation string, produced by
	// a citation.Formatter — never hand-assembled by this package.
	Text string

	// Resolved is true if a citation lookup was attempted without
	// error.
	Resolved bool

	// Verified is true if the underlying rule node was independently
	// confirmed to exist in the case's tree.
	Verified bool
}

// ReportIssue is one issue's structured analysis: the draft,
// non-binding conclusion, the party it currently favors (if any), the
// weakest link threatening its reliability, and every citation
// supporting it — assembled from one
// synthesisagent.TentativeConclusion plus its matching
// reasoningtrace.AuthorityTrail (if a Trace was supplied to Assemble).
type ReportIssue struct {
	// IssueNodeID is the irac.IssueNode.ID this entry resolves.
	IssueNodeID string

	// Analysis is the reasoned, non-binding draft analysis text for
	// this issue (synthesisagent.TentativeConclusion.Text, verbatim).
	Analysis string

	// FavoredParty is the party this draft analysis currently favors,
	// or empty if genuinely unresolved on the record.
	FavoredParty string

	// Confidence is this conclusion's own [0,1] confidence.
	Confidence float64

	// WeakestLink names the single supporting element most
	// threatening this conclusion's reliability.
	WeakestLink string

	// SupportingFactIDs are the irac.FactNode IDs this conclusion
	// traces back to (the "Facts" a reader can cross-reference for
	// this issue).
	SupportingFactIDs []string

	// Citations are this issue's controlling rules, jurisdiction-
	// formatted. Empty if no reasoningtrace.Trace was supplied to
	// Assemble.
	Citations []ReportCitation
}

// Report is the fully assembled, structured export payload for one
// case: facts, issues, analysis, and citations drawn from a
// synthesisagent.Opinion and a caselifecycle.Case, plus an optional
// reasoning-trace appendix. Report holds no rendering logic itself —
// pdf.go, docx.go, and markdown.go each render a Report into their own
// byte format.
type Report struct {
	// CaseID is the case this report was assembled for.
	CaseID uuid.UUID

	// TenantID is the tenant the source case belongs to.
	TenantID uuid.UUID

	// CaseTitle is the case's human-readable title
	// (caselifecycle.Case.Title).
	CaseTitle string

	// CaseReference is the case's external/docket reference, if any.
	CaseReference string

	// JurisdictionKey is the opaque jurisdiction/legal-family key
	// (e.g. "common_law", "civil_law") used to select a
	// citation.Formatter from the citation.Registry supplied to
	// Assemble — mirroring citation.Registry.Format's own key
	// parameter, so this package never hard-codes a jurisdiction
	// taxonomy.
	JurisdictionKey string

	// Issues is one ReportIssue per issue the source Opinion
	// addressed, in Opinion.Conclusions order.
	Issues []ReportIssue

	// SkippedIssueNodeIDs mirrors
	// synthesisagent.Opinion.SkippedIssueNodeIDs: issues for which no
	// grounded conclusion survived synthesis.
	SkippedIssueNodeIDs []string

	// TraceAppendix is the rendered reasoning-trace appendix text
	// (Markdown, via reasoningtrace.ExportMarkdown), or empty if no
	// Trace was supplied to Assemble.
	TraceAppendix string

	// OpinionGeneratedAt is when the source Opinion was synthesized.
	OpinionGeneratedAt time.Time

	// AssembledAt is when this Report was assembled.
	AssembledAt time.Time
}

// AssembleInput bundles the source data Assemble combines into a
// Report.
type AssembleInput struct {
	// CaseID/TenantID/CaseTitle/CaseReference come from the source
	// caselifecycle.Case. Required: CaseID, TenantID.
	CaseID        uuid.UUID
	TenantID      uuid.UUID
	CaseTitle     string
	CaseReference string

	// JurisdictionKey selects a citation.Formatter from Citations, and
	// is copied verbatim onto the resulting Report.
	JurisdictionKey string

	// Citations resolves JurisdictionKey to a citation.Formatter. If
	// nil, citation.NewDefaultRegistry() is used.
	Citations *citation.Registry

	// AuthorityTrailsByIssue optionally supplies each issue's
	// controlling-rule citation data (typically drawn from a
	// reasoningtrace.Trace's AuthorityTrails, keyed by
	// AuthorityTrail.IssueNodeID), letting Assemble render
	// ReportIssue.Citations without importing reasoningtrace types
	// directly. May be nil.
	AuthorityTrailsByIssue map[string][]AuthorityCitationInput

	// TraceAppendix is the pre-rendered reasoning-trace appendix
	// (typically reasoningtrace.ExportMarkdown's output). Empty
	// disables the appendix section.
	TraceAppendix string
}

// AuthorityCitationInput is the per-rule input Assemble needs to
// render one ReportIssue's Citations entry: enough of a
// citation.FormatInput to format the citation text, plus the
// resolution/verification flags carried over from a
// reasoningtrace.CitationTrail.
type AuthorityCitationInput struct {
	RuleID      string
	FormatInput citation.FormatInput
	Resolved    bool
	Verified    bool
}
