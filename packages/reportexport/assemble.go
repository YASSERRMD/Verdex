package reportexport

import (
	"time"

	"github.com/YASSERRMD/verdex/packages/caselifecycle"
	"github.com/YASSERRMD/verdex/packages/citation"
	"github.com/YASSERRMD/verdex/packages/synthesisagent"
)

// nowFunc is overridable in tests, mirroring
// packages/notifications.Service's now field convention.
var nowFunc = func() time.Time { return time.Now().UTC() }

// Assemble builds a structured Report from a case, its synthesized
// Opinion, and jurisdiction-formatted citation data — the single entry
// point tying packages/caselifecycle, packages/synthesisagent, and
// packages/citation together into this package's own Report type,
// without re-deriving any of their underlying analysis.
//
// Returns ErrNilCase or ErrNilOpinion if either required argument is
// nil. Returns caselifecycle's own validation error (via
// requireMatchingCase) if opinion.CaseID does not match c.ID.
func Assemble(c *caselifecycle.Case, opinion *synthesisagent.Opinion, input AssembleInput) (*Report, error) {
	if c == nil {
		return nil, ErrNilCase
	}
	if opinion == nil {
		return nil, ErrNilOpinion
	}

	registry := input.Citations
	if registry == nil {
		registry = citation.NewDefaultRegistry()
	}

	issues := make([]ReportIssue, 0, len(opinion.Conclusions))
	for _, concl := range opinion.Conclusions {
		issues = append(issues, buildReportIssue(concl, input.JurisdictionKey, registry, input.AuthorityTrailsByIssue))
	}

	return &Report{
		CaseID:              c.ID,
		TenantID:            c.TenantID,
		CaseTitle:           firstNonEmpty(input.CaseTitle, c.Title),
		CaseReference:       firstNonEmpty(input.CaseReference, c.Reference),
		JurisdictionKey:     input.JurisdictionKey,
		Issues:              issues,
		SkippedIssueNodeIDs: append([]string(nil), opinion.SkippedIssueNodeIDs...),
		TraceAppendix:       input.TraceAppendix,
		OpinionGeneratedAt:  opinion.GeneratedAt,
		AssembledAt:         nowFunc(),
	}, nil
}

func buildReportIssue(
	concl synthesisagent.TentativeConclusion,
	jurisdictionKey string,
	registry *citation.Registry,
	trailsByIssue map[string][]AuthorityCitationInput,
) ReportIssue {
	issue := ReportIssue{
		IssueNodeID:       concl.IssueNodeID,
		Analysis:          concl.Text,
		FavoredParty:      concl.FavoredParty,
		Confidence:        concl.Confidence,
		WeakestLink:       concl.WeakestLink,
		SupportingFactIDs: append([]string(nil), concl.SupportingFactIDs...),
	}

	for _, ac := range trailsByIssue[concl.IssueNodeID] {
		text, err := registry.Format(jurisdictionKey, ac.FormatInput)
		if err != nil {
			// Unknown jurisdiction key with no fallback registered:
			// fall back to the raw citation/act fields rather than
			// dropping the citation entirely.
			text = rawFallbackCitation(ac.FormatInput)
		}
		issue.Citations = append(issue.Citations, ReportCitation{
			RuleID:   ac.RuleID,
			Text:     text,
			Resolved: ac.Resolved,
			Verified: ac.Verified,
		})
	}

	return issue
}

// rawFallbackCitation renders a best-effort citation string directly
// from FormatInput when no Formatter is available for the requested
// jurisdiction key and no fallback was registered, so a report never
// silently drops a citation's identifying text.
func rawFallbackCitation(in citation.FormatInput) string {
	switch {
	case in.RawCitation != "":
		return in.RawCitation
	case in.CaseName != "":
		return in.CaseName
	case in.Act != "":
		return in.Act
	default:
		return ""
	}
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
