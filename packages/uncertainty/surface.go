package uncertainty

import "time"

// nowFunc is overridden in tests for deterministic GeneratedAt values.
var nowFunc = time.Now

// Surface runs the full uncertainty-surfacing pass for req: it identifies
// low-confidence reasoning steps (identifyLowConfidence), thin or
// disputed evidence (identifyThinEvidence), and unsettled or conflicting
// law (identifyConflictingLaw); ranks every finding by its
// materiality-weighted impact on the case's outcome (rankUncertainties);
// scans every TentativeConclusion's Text for over-confident phrasing
// (identifyOverconfidence); and returns the combined Report.
//
// Returns ErrEmptyCaseID if req.CaseID is empty, ErrNoFramedIssues if
// req.Issues.Issues is empty, and ErrCaseIDMismatch if any of
// req.Evidence.CaseID, req.Law.CaseID, or req.Opinion.CaseID is non-empty
// and differs from req.CaseID.
func Surface(req Request) (Report, error) {
	if req.CaseID == "" {
		return Report{}, ErrEmptyCaseID
	}
	if len(req.Issues.Issues) == 0 {
		return Report{}, ErrNoFramedIssues
	}
	if err := checkCaseIDMismatch(req); err != nil {
		return Report{}, err
	}

	var findings []Uncertainty
	findings = append(findings, identifyLowConfidence(req)...)
	findings = append(findings, identifyThinEvidence(req)...)
	findings = append(findings, identifyConflictingLaw(req)...)

	ranked := rankUncertainties(req, findings)

	return Report{
		CaseID:              req.CaseID,
		Uncertainties:       ranked,
		OverconfidenceFlags: identifyOverconfidence(req),
		GeneratedAt:         nowFunc(),
	}, nil
}

// Analyze is an alias for Surface, kept so a caller reading this
// package's exported API by either the plan's "surface uncertainty" or
// "analyze the opinion for uncertainty" framing finds the entry point
// under either name.
func Analyze(req Request) (Report, error) {
	return Surface(req)
}

// checkCaseIDMismatch returns ErrCaseIDMismatch if any non-empty CaseID
// carried by req's upstream results differs from req.CaseID.
func checkCaseIDMismatch(req Request) error {
	if req.Evidence.CaseID != "" && req.Evidence.CaseID != req.CaseID {
		return ErrCaseIDMismatch
	}
	if req.Law.CaseID != "" && req.Law.CaseID != req.CaseID {
		return ErrCaseIDMismatch
	}
	if req.Opinion.CaseID != "" && req.Opinion.CaseID != req.CaseID {
		return ErrCaseIDMismatch
	}
	if req.Issues.CaseID != "" && req.Issues.CaseID != req.CaseID {
		return ErrCaseIDMismatch
	}
	return nil
}
