package grounding

import "fmt"

// CanFinalize is the hard gate blocking downstream use of a synthesized
// opinion on the grounds of content, not structure or sign-off: it
// returns false and a descriptive error whenever report contains at
// least one SeverityCritical Finding (in either Report.Findings or any
// ConclusionResult.CitationFindings).
//
// CanFinalize mirrors packages/treevalidation.CanFinalize and
// packages/guardrail.CanFinalize's shape exactly — a pure function over
// already-computed check results — but is a distinct concern from both:
// treevalidation blocks on tree STRUCTURAL integrity (orphans, cycles,
// unsupported claims as detected against the tree shape itself);
// guardrail blocks on human SIGN-OFF state; this gate blocks on whether
// the synthesized opinion's own assertions are actually grounded in the
// case's facts and law. A caller that needs all three guarantees calls
// all three gates — this package does not import treevalidation or
// guardrail, and neither of them imports this package.
//
// Returns ErrNilReport if report is nil, distinguishing "no grounding
// check was ever run" (a caller error — Check should have been called
// first) from a legitimately clean Report with zero Findings.
func CanFinalize(report *Report) (bool, error) {
	if report == nil {
		return false, ErrNilReport
	}
	if !report.HasCritical() {
		return true, nil
	}
	return false, fmt.Errorf(
		"%w: case %q has %d critical finding(s) (%s)",
		ErrCriticalFindings, report.CaseID, report.CountBySeverity(SeverityCritical), report.Summary(),
	)
}
