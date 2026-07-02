package uncertainty

import "sort"

// materialityAmplification returns the multiplier applied to an
// Uncertainty's Severity for an issue ranked materialityRank among
// totalIssues (1 being most material). The most material issue gets the
// full 1.0 multiplier; amplification decays linearly to a floor of 0.5
// for the least material issue, so even a low-materiality issue's
// uncertainty is never zeroed out — an uncertainty is still worth
// surfacing even on a peripheral issue, just ranked lower than an
// equally severe one on a more material issue.
//
// An Uncertainty with no known issue (materialityRank 0, e.g. a
// FactWeight finding that could not be attached to any issue) or a
// Report computed against a single-issue case receives the full 1.0
// multiplier: there is no materiality context to discount by.
func materialityAmplification(materialityRank, totalIssues int) float64 {
	const floor = 0.5
	if materialityRank <= 0 || totalIssues <= 1 {
		return 1.0
	}
	// Linear decay from 1.0 (rank 1) to floor (rank totalIssues).
	step := (1.0 - floor) / float64(totalIssues-1)
	return 1.0 - step*float64(materialityRank-1)
}

// impactScore combines an Uncertainty's own Severity with its issue's
// materiality amplification into a single [0, ~1] score used to derive
// ImpactRank: uncertainties attached to issues the issue-agent ranked
// more material amplify their impact on the case's overall outcome,
// relative to an equally severe uncertainty on a less material issue.
func impactScore(severity float64, materialityRank, totalIssues int) float64 {
	return severity * materialityAmplification(materialityRank, totalIssues)
}

// rankUncertainties computes each Uncertainty's ImpactScore (via
// impactScore, using req's materiality context) and assigns ImpactRank
// 1..n by descending ImpactScore. Ties are broken deterministically by
// Source then IssueNodeID then Detail, so repeated runs against the same
// input always produce the same ordering.
func rankUncertainties(req Request, findings []Uncertainty) []Uncertainty {
	materiality := req.materialityRankByIssue()
	totalIssues := len(req.Issues.Issues)

	out := make([]Uncertainty, len(findings))
	copy(out, findings)
	for i := range out {
		rank := materiality[out[i].IssueNodeID]
		out[i].ImpactScore = impactScore(out[i].Severity, rank, totalIssues)
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].ImpactScore != out[j].ImpactScore {
			return out[i].ImpactScore > out[j].ImpactScore
		}
		if out[i].Source != out[j].Source {
			return out[i].Source < out[j].Source
		}
		if out[i].IssueNodeID != out[j].IssueNodeID {
			return out[i].IssueNodeID < out[j].IssueNodeID
		}
		return out[i].Detail < out[j].Detail
	})

	for i := range out {
		out[i].ImpactRank = i + 1
	}
	return out
}
