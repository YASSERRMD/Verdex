package reportexport

import (
	"github.com/YASSERRMD/verdex/packages/citation"
	"github.com/YASSERRMD/verdex/packages/reasoningtrace"
)

// WithTrace populates an AssembleInput's TraceAppendix and
// AuthorityTrailsByIssue fields from an already-assembled
// reasoningtrace.Trace, so callers never need to hand-build either
// field themselves. It returns input unchanged (as a convenience for
// chaining into Assemble) after mutating it in place.
//
// TraceAppendix is rendered via reasoningtrace.ExportMarkdown — this
// package never re-derives trace narrative itself, only embeds
// Trace's own rendering. AuthorityTrailsByIssue is derived from
// trace.AuthorityTrails, converting each CitationTrail's raw citation
// string into a citation.FormatInput (Origin: OriginPrecedent,
// RawCitation: the trail's Citation text) so ReportIssue.Citations can
// still be re-formatted through a jurisdiction-specific
// citation.Formatter rather than trusting the trail's citation string
// as already-correctly-formatted for the report's target
// jurisdiction.
func WithTrace(input AssembleInput, trace reasoningtrace.Trace) (AssembleInput, error) {
	appendix, err := reasoningtrace.ExportMarkdown(trace)
	if err != nil {
		return input, wrapf("WithTrace", err)
	}
	input.TraceAppendix = appendix

	if len(trace.AuthorityTrails) > 0 {
		byIssue := make(map[string][]AuthorityCitationInput, len(trace.AuthorityTrails))
		for _, trail := range trace.AuthorityTrails {
			cites := make([]AuthorityCitationInput, 0, len(trail.Citations))
			for _, c := range trail.Citations {
				cites = append(cites, AuthorityCitationInput{
					RuleID: c.RuleID,
					FormatInput: citation.FormatInput{
						Origin:      citation.OriginPrecedent,
						RawCitation: c.Citation,
					},
					Resolved: c.Resolved,
					Verified: c.Verified,
				})
			}
			if len(cites) > 0 {
				byIssue[trail.IssueNodeID] = cites
			}
		}
		input.AuthorityTrailsByIssue = byIssue
	}

	return input, nil
}
