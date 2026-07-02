package reasoningtrace

import (
	"github.com/YASSERRMD/verdex/packages/lawapplication"
	"github.com/YASSERRMD/verdex/packages/synthesisagent"
)

// buildAuthorityTrails returns one AuthorityTrail per conclusion in
// opinion, each populated with that conclusion's verified supporting
// facts and, when the law-application stage's result contains an
// IssueApplication for the same IssueNodeID, that issue's controlling
// rules and their citation/verification status.
func buildAuthorityTrails(opinion synthesisagent.Opinion, law lawapplication.Result) []AuthorityTrail {
	if len(opinion.Conclusions) == 0 {
		return nil
	}

	applicationsByIssue := make(map[string]lawapplication.IssueApplication, len(law.IssueApplications))
	for _, app := range law.IssueApplications {
		applicationsByIssue[app.IssueNodeID] = app
	}

	trails := make([]AuthorityTrail, 0, len(opinion.Conclusions))
	for _, conclusion := range opinion.Conclusions {
		trail := AuthorityTrail{
			IssueNodeID:       conclusion.IssueNodeID,
			SupportingFactIDs: conclusion.SupportingFactIDs,
		}

		if app, ok := applicationsByIssue[conclusion.IssueNodeID]; ok {
			citationsByRule := make(map[string]lawapplication.AppliedCitation, len(app.Citations))
			for _, c := range app.Citations {
				citationsByRule[c.RuleID] = c
			}
			for _, ruleID := range app.ControllingRuleIDs {
				c := citationsByRule[ruleID]
				trail.Citations = append(trail.Citations, CitationTrail{
					RuleID:   ruleID,
					Citation: c.Citation,
					Verified: c.Verified,
					Resolved: c.Resolved,
				})
			}
		}

		trails = append(trails, trail)
	}
	return trails
}
