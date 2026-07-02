package treevalidation

import "fmt"

// Severity classifies how serious a Finding is.
type Severity string

const (
	// SeverityCritical marks a Finding that must block downstream use of
	// the tree (see gate.go's CanFinalize).
	SeverityCritical Severity = "critical"

	// SeverityWarning marks a Finding worth surfacing to a reviewer but
	// that does not, on its own, block downstream use.
	SeverityWarning Severity = "warning"

	// SeverityInfo marks a purely informational Finding.
	SeverityInfo Severity = "info"
)

// Finding is a single structured problem (or informational note) surfaced
// by one of this package's checks.
type Finding struct {
	// Severity classifies how serious this Finding is.
	Severity Severity

	// Code is a short, stable machine-readable identifier for the kind of
	// problem this Finding represents (e.g. CodeConclusionNotTraceable).
	Code string

	// Message is a human-readable description of this specific
	// occurrence.
	Message string

	// NodeID is the ID of the node this Finding concerns, or empty if the
	// Finding does not concern a single node.
	NodeID string
}

// Report aggregates every Finding collected across every check this
// package runs against one treeassembly.Tree (both this package's own
// checks — traceability, orphans, cycles, unsupported claims, confidence
// propagation, jurisdiction consistency — and the structural/semantic
// findings translated from irac.ValidateTree and treeassembly.DetectGaps
// by service.go). A Report is the single artifact downstream phases
// inspect (via gate.go's CanFinalize) before trusting a tree.
type Report struct {
	// CaseID identifies the case whose tree this Report was computed for.
	CaseID string

	// Findings is every Finding collected across every check, in the
	// order the checks were run.
	Findings []Finding
}

// Summary returns a short human-readable count of this Report's Findings
// by severity, e.g. "3 critical, 2 warning, 0 info (5 total)".
func (r Report) Summary() string {
	var critical, warning, info int
	for _, f := range r.Findings {
		switch f.Severity {
		case SeverityCritical:
			critical++
		case SeverityWarning:
			warning++
		case SeverityInfo:
			info++
		}
	}
	return fmt.Sprintf(
		"%d critical, %d warning, %d info (%d total)",
		critical, warning, info, len(r.Findings),
	)
}

// HasCritical reports whether r contains at least one SeverityCritical
// Finding.
func (r Report) HasCritical() bool {
	for _, f := range r.Findings {
		if f.Severity == SeverityCritical {
			return true
		}
	}
	return false
}

// CountBySeverity returns the number of Findings in r with the given
// severity.
func (r Report) CountBySeverity(s Severity) int {
	count := 0
	for _, f := range r.Findings {
		if f.Severity == s {
			count++
		}
	}
	return count
}
