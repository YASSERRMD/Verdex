package uncertainty

import "fmt"

// identifyLowConfidence scans every low-confidence reasoning step across
// the three sources that carry a Confidence score — framed issues, law
// applications, and tentative conclusions — and returns one Uncertainty
// per step at or below req's effective LowConfidenceThreshold.
//
// This is deliberately three separate scans rather than one generic
// pass: each source's Confidence measures a different thing (how much to
// trust the issue's framing, the law application's controlling-rule
// selection, or the conclusion's own synthesis), so each gets its own
// Source tag and its own Caveat wording (see caveat.go).
func identifyLowConfidence(req Request) []Uncertainty {
	threshold := req.lowConfidenceThreshold()
	var out []Uncertainty

	for _, fi := range req.Issues.Issues {
		if fi.Confidence > threshold {
			continue
		}
		out = append(out, Uncertainty{
			IssueNodeID: fi.SourceIssueNodeID,
			Source:      SourceIssueFraming,
			Severity:    1 - fi.Confidence,
			Caveat:      lowConfidenceCaveat(SourceIssueFraming, fi.Confidence),
		})
	}

	for _, ia := range req.Law.IssueApplications {
		if ia.Confidence > threshold {
			continue
		}
		out = append(out, Uncertainty{
			IssueNodeID: ia.IssueNodeID,
			Source:      SourceLawApplication,
			Severity:    1 - ia.Confidence,
			Caveat:      lowConfidenceCaveat(SourceLawApplication, ia.Confidence),
		})
	}

	for _, tc := range req.Opinion.Conclusions {
		if tc.Confidence > threshold {
			continue
		}
		out = append(out, Uncertainty{
			IssueNodeID: tc.IssueNodeID,
			Source:      SourceConclusion,
			Severity:    1 - tc.Confidence,
			Caveat:      lowConfidenceCaveat(SourceConclusion, tc.Confidence),
		})
	}

	return out
}

// evidenceIssueLookup maps a FactNodeID to the issue(s) it is relevant
// to, so a fact-level finding (which evidenceweighing.FactWeight does not
// itself tag with an IssueNodeID) can still be attached to the right
// conclusion. It is built from the law application's ElementFactMap,
// which is the only upstream structure that already links a FactNodeID
// to an IssueNodeID (via its owning IssueApplication).
func evidenceIssueLookup(req Request) map[string][]string {
	out := make(map[string][]string)
	seen := make(map[string]map[string]bool)
	for _, ia := range req.Law.IssueApplications {
		for _, entry := range ia.ElementFactMap {
			if seen[entry.FactNodeID] == nil {
				seen[entry.FactNodeID] = make(map[string]bool)
			}
			if seen[entry.FactNodeID][ia.IssueNodeID] {
				continue
			}
			seen[entry.FactNodeID][ia.IssueNodeID] = true
			out[entry.FactNodeID] = append(out[entry.FactNodeID], ia.IssueNodeID)
		}
	}
	return out
}

// identifyThinEvidence surfaces every evidenceweighing.FactWeight that is
// Contradicted or falls at or below req's effective
// ThinEvidenceWeightThreshold, every evidenceweighing.Contradiction, and
// every evidenceweighing.Gap, each attached to the issue(s) it concerns.
//
// A contradicted or low-weight FactWeight is attached to every issue its
// owning arguments cited it for (via evidenceIssueLookup); a
// Contradiction and a Gap already carry their own IssueNodeID (Gap only
// when known — see evidenceweighing.GapKindMissingFact's doc comment).
func identifyThinEvidence(req Request) []Uncertainty {
	var out []Uncertainty
	threshold := req.thinEvidenceWeightThreshold()
	byFact := evidenceIssueLookup(req)

	for _, fw := range req.Evidence.FactWeights {
		if !fw.Contradicted && fw.Weight > threshold {
			continue
		}
		issues := byFact[fw.FactNodeID]
		if len(issues) == 0 {
			issues = []string{""}
		}
		for _, issueID := range issues {
			out = append(out, Uncertainty{
				IssueNodeID: issueID,
				Source:      SourceEvidence,
				Severity:    evidenceSeverity(fw),
				Caveat:      thinEvidenceCaveat(fw),
				Detail:      fw.FactNodeID,
			})
		}
	}

	for _, c := range req.Evidence.Contradictions {
		out = append(out, Uncertainty{
			IssueNodeID: c.IssueNodeID,
			Source:      SourceEvidence,
			Severity:    contradictionSeverity,
			Caveat:      contradictionCaveat(c),
			Detail:      c.FactNodeID,
		})
	}

	for _, g := range req.Evidence.Gaps {
		out = append(out, Uncertainty{
			IssueNodeID: g.IssueNodeID,
			Source:      SourceEvidence,
			Severity:    gapSeverity,
			Caveat:      gapCaveat(g),
			Detail:      g.FactNodeID,
		})
	}

	return out
}

// identifyConflictingLaw surfaces every
// lawapplication.ConflictingAuthority detected among an issue's
// controlling rules: unsettled or conflicting law is treated as a
// fixed-severity structural finding, distinct from (and in addition to)
// that issue's own low-confidence IssueApplication.Confidence finding, if
// any.
func identifyConflictingLaw(req Request) []Uncertainty {
	var out []Uncertainty
	for _, ia := range req.Law.IssueApplications {
		for _, conflict := range ia.Conflicts {
			out = append(out, Uncertainty{
				IssueNodeID: conflict.IssueNodeID,
				Source:      SourceLawApplication,
				Severity:    conflictingAuthoritySeverity,
				Caveat:      conflictingAuthorityCaveat(conflict),
				Detail:      fmt.Sprintf("%s vs %s", conflict.FirstRuleID, conflict.SecondRuleID),
			})
		}
	}
	return out
}
