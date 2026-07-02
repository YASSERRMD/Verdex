package treevalidation

import "fmt"

// CanFinalize is the hard gate blocking downstream use of a reasoning
// tree: it returns false and a descriptive error whenever report contains
// at least one SeverityCritical Finding. Future phases (e.g. Phase 055's
// synthesis) must call CanFinalize before treating a tree as usable —
// mirroring CONTRIBUTING.md's guardrail spirit that a reasoning artifact
// with critical integrity failures must not be usable for further
// reasoning.
//
// A Report with only SeverityWarning and/or SeverityInfo Findings (or no
// Findings at all) passes the gate: CanFinalize returns (true, nil).
func CanFinalize(report Report) (bool, error) {
	if !report.HasCritical() {
		return true, nil
	}
	return false, fmt.Errorf(
		"%w: case %q has %d critical finding(s) (%s)",
		ErrCriticalFindings, report.CaseID, report.CountBySeverity(SeverityCritical), report.Summary(),
	)
}
