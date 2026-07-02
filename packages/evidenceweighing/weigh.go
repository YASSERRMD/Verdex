package evidenceweighing

import (
	"time"

	"github.com/YASSERRMD/verdex/packages/firstpartyagent"
	"github.com/YASSERRMD/verdex/packages/secondpartyagent"
)

// WeighRequest bundles everything Weigh needs for one case: the case's
// FactNodes (as FactRefs), the full set of issue node IDs under
// consideration (for uncited-issue gap detection), both parties'
// ArgumentSets, the Rubric to apply, and the LegalFamily governing this
// case (used only to select/record the jurisdiction profile — see
// jurisdiction.go).
//
// Either ArgumentSet may be its zero value if that party produced no
// arguments (e.g. every issue was skipped upstream); Weigh still runs
// against whichever ArgumentSet is non-empty. ErrNoArgumentSets is
// returned only if both are empty.
type WeighRequest struct {
	// CaseID is the case being weighed.
	CaseID string

	// Facts are every FactRef known in the case's tree, typically loaded
	// via knowledgeapi.GetTree with NodeTypeFilter set to fact nodes and
	// converted by the caller into FactRefs.
	Facts []FactRef

	// IssueNodeIDs are every issue node ID under consideration, used by
	// DetectGaps to surface GapKindUncitedIssue findings.
	IssueNodeIDs []string

	// FirstParty is the first party's ArgumentSet
	// (packages/firstpartyagent, Phase 051).
	FirstParty firstpartyagent.ArgumentSet

	// SecondParty is the second party's ArgumentSet
	// (packages/secondpartyagent, Phase 052).
	SecondParty secondpartyagent.ArgumentSet

	// Rubric is the WeightFactors/JurisdictionProfile combination to
	// apply. If the zero value, DefaultRubric() is used.
	Rubric Rubric

	// LegalFamily selects the JurisdictionProfile applied by Rubric, via
	// ProfileForFamily, when Rubric.Profile is the zero value. If Rubric
	// already carries an explicit non-zero Profile, LegalFamily is
	// recorded on the result but does not override the caller's chosen
	// Profile — an explicitly supplied Rubric always wins.
	LegalFamily LegalFamily
}

// Weigh computes an EvidenceWeighingResult for req: per-fact weights (see
// ScoreFacts), contradictions (see DetectContradictions), and gaps (see
// DetectGaps).
//
// Returns ErrEmptyCaseID if req.CaseID is empty, ErrNoArgumentSets if both
// req.FirstParty and req.SecondParty carry zero arguments, and
// ErrCaseIDMismatch if either non-empty ArgumentSet's CaseID differs from
// req.CaseID.
func Weigh(req WeighRequest) (EvidenceWeighingResult, error) {
	if req.CaseID == "" {
		return EvidenceWeighingResult{}, ErrEmptyCaseID
	}

	firstArgs := req.FirstParty.Arguments
	secondArgs := req.SecondParty.Arguments
	if len(firstArgs) == 0 && len(secondArgs) == 0 {
		return EvidenceWeighingResult{}, ErrNoArgumentSets
	}

	if len(firstArgs) > 0 && req.FirstParty.CaseID != "" && req.FirstParty.CaseID != req.CaseID {
		return EvidenceWeighingResult{}, ErrCaseIDMismatch
	}
	if len(secondArgs) > 0 && req.SecondParty.CaseID != "" && req.SecondParty.CaseID != req.CaseID {
		return EvidenceWeighingResult{}, ErrCaseIDMismatch
	}

	rubric := req.Rubric
	if isZeroWeightFactors(rubric.Factors) {
		rubric.Factors = DefaultWeightFactors()
	}
	if isZeroProfile(rubric.Profile) {
		rubric.Profile = ProfileForFamily(req.LegalFamily)
	}

	arguments := make([]CitingArgument, 0, len(firstArgs)+len(secondArgs))
	arguments = append(arguments, citingArgumentsFromFirstParty(req.FirstParty)...)
	arguments = append(arguments, citingArgumentsFromSecondParty(req.SecondParty)...)

	factWeights := ScoreFacts(rubric, req.Facts, arguments)
	contradictions := DetectContradictions(arguments)
	gaps := DetectGaps(arguments, req.Facts, req.IssueNodeIDs)

	return EvidenceWeighingResult{
		CaseID:         req.CaseID,
		FactWeights:    factWeights,
		Contradictions: contradictions,
		Gaps:           gaps,
		LegalFamily:    req.LegalFamily,
		GeneratedAt:    time.Now(),
	}, nil
}

// isZeroWeightFactors reports whether f is the zero value.
func isZeroWeightFactors(f WeightFactors) bool {
	return f == WeightFactors{}
}

// isZeroProfile reports whether p is the zero value.
func isZeroProfile(p JurisdictionProfile) bool {
	return p == JurisdictionProfile{}
}
