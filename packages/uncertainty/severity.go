package uncertainty

import "github.com/YASSERRMD/verdex/packages/evidenceweighing"

// contradictionSeverity is the fixed [0,1] severity assigned to every
// evidenceweighing.Contradiction finding. A contradiction is a structural
// defect (a fact cited by both parties for mutually exclusive claims) and
// is treated as more severe than an ordinary low-weight fact, but not
// maximal — a case can still be soundly decided around one contradicted
// fact if enough other evidence supports the same conclusion.
const contradictionSeverity = 0.7

// gapSeverity is the fixed [0,1] severity assigned to every
// evidenceweighing.Gap finding, whether a missing-fact citation or an
// uncited issue. Weighted slightly below contradictionSeverity: a gap
// means the record is silent, which is concerning but not evidence of an
// active factual dispute.
const gapSeverity = 0.6

// conflictingAuthoritySeverity is the fixed [0,1] severity assigned to
// every lawapplication.ConflictingAuthority finding. Unsettled or
// conflicting controlling law is treated as highly severe: it means the
// legal question the issue turns on has no single settled answer on this
// record, regardless of how confident the rest of the analysis is.
const conflictingAuthoritySeverity = 0.8

// evidenceSeverity scores a single evidenceweighing.FactWeight's own
// severity as thin/disputed evidence: a Contradicted fact is always
// treated as severely as contradictionSeverity itself (a FactWeight and
// its Contradiction finding both trace to the same underlying defect, so
// their severities intentionally agree); otherwise severity scales
// inversely with Weight, so a near-zero-weight fact approaches maximal
// severity and a merely-below-threshold fact stays mild.
func evidenceSeverity(fw evidenceweighing.FactWeight) float64 {
	if fw.Contradicted {
		return contradictionSeverity
	}
	return clamp01(1 - fw.Weight)
}

// clamp01 clamps v into [0, 1].
func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
