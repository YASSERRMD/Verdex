package citation

import "fmt"

// Severity classifies how serious a Finding is. This mirrors
// packages/treevalidation's Severity convention exactly, redeclared
// locally to keep this package free of a cross-package dependency on
// treevalidation for a three-value enum.
type Severity string

const (
	// SeverityCritical marks a Finding that must block downstream use of
	// a citation (e.g. a hallucinated or wrong-case citation).
	SeverityCritical Severity = "critical"

	// SeverityWarning marks a Finding worth surfacing to a reviewer but
	// that does not, on its own, block downstream use (e.g. a stale
	// citation whose span text has drifted from current node text).
	SeverityWarning Severity = "warning"

	// SeverityInfo marks a purely informational Finding.
	SeverityInfo Severity = "info"
)

// Finding codes: short, stable, machine-readable identifiers for the kind
// of problem a Finding represents.
const (
	// CodeHallucinated flags a citation whose target node does not exist
	// anywhere in the GraphStore.
	CodeHallucinated = "citation_hallucinated"

	// CodeWrongCase flags a citation whose target node exists, but under
	// a different case than claimed.
	CodeWrongCase = "citation_wrong_case"

	// CodeBrokenDeleted flags a citation whose target node previously
	// existed for the claimed case but has since been deleted or moved
	// (e.g. via GraphStore.DeleteTree).
	CodeBrokenDeleted = "citation_broken_deleted"

	// CodeBrokenStale flags a citation whose recorded source text no
	// longer matches the target node's current text.
	CodeBrokenStale = "citation_broken_stale"

	// CodeUnresolved flags a citation that carries no resolved Citation
	// text at all (Resolver returned CertaintyNone, or Resolve was never
	// run).
	CodeUnresolved = "citation_unresolved"

	// CodeNoSourceSpan flags a citation with no source spans, meaning
	// there is no traceable link back to ingested source text even though
	// the node itself may be verified.
	CodeNoSourceSpan = "citation_no_source_span"
)

// Finding is a single structured problem (or informational note) surfaced
// by this package's checks against a CitedUnit, mirroring
// packages/treevalidation's Finding/Report/Severity convention.
type Finding struct {
	// Severity classifies how serious this Finding is.
	Severity Severity

	// Code is a short, stable machine-readable identifier for the kind of
	// problem this Finding represents (see the Code* constants above).
	Code string

	// Message is a human-readable description of this specific
	// occurrence.
	Message string

	// NodeID is the ID of the node this Finding concerns.
	NodeID string

	// CaseID is the case this Finding concerns.
	CaseID string
}

// FindingsFromVerification translates a VerificationResult into zero or
// one Finding: StatusVerified produces no Finding (nothing to flag),
// StatusHallucinated and StatusWrongCase each produce a SeverityCritical
// Finding, and StatusBroken produces a Finding whose severity depends on
// whether the underlying cause was deletion (critical) or staleness
// (warning) — see DetectBroken, which is what actually assigns
// StatusBroken; a VerificationResult with StatusBroken but no further
// context defaults to SeverityCritical here since a broken-by-default
// citation cannot be positively confirmed as merely stale.
func FindingsFromVerification(result VerificationResult) []Finding {
	unit := result.Unit
	switch result.Status {
	case StatusVerified:
		return nil
	case StatusHallucinated:
		return []Finding{{
			Severity: SeverityCritical,
			Code:     CodeHallucinated,
			Message:  fmt.Sprintf("citation for node %q claims case %q but no such node exists", unit.NodeID, unit.CaseID),
			NodeID:   unit.NodeID,
			CaseID:   unit.CaseID,
		}}
	case StatusWrongCase:
		return []Finding{{
			Severity: SeverityCritical,
			Code:     CodeWrongCase,
			Message:  fmt.Sprintf("citation for node %q claims case %q but the node belongs to case %q", unit.NodeID, unit.CaseID, result.ActualCaseID),
			NodeID:   unit.NodeID,
			CaseID:   unit.CaseID,
		}}
	case StatusBroken:
		return []Finding{{
			Severity: SeverityCritical,
			Code:     CodeBrokenDeleted,
			Message:  fmt.Sprintf("citation for node %q in case %q is broken (target deleted or moved)", unit.NodeID, unit.CaseID),
			NodeID:   unit.NodeID,
			CaseID:   unit.CaseID,
		}}
	default:
		return nil
	}
}

// FindingsFromUnit surfaces content-level Findings about unit itself,
// independent of GraphStore verification: an unresolved citation (no
// Citation text) or a citation with no source spans. These complement
// FindingsFromVerification, which only speaks to whether the node
// exists.
func FindingsFromUnit(unit CitedUnit) []Finding {
	var findings []Finding
	if !unit.HasCitation() {
		findings = append(findings, Finding{
			Severity: SeverityWarning,
			Code:     CodeUnresolved,
			Message:  fmt.Sprintf("node %q has no resolved citation text", unit.NodeID),
			NodeID:   unit.NodeID,
			CaseID:   unit.CaseID,
		})
	}
	if !unit.HasSpans() {
		findings = append(findings, Finding{
			Severity: SeverityWarning,
			Code:     CodeNoSourceSpan,
			Message:  fmt.Sprintf("node %q has no source spans linking it to ingested text", unit.NodeID),
			NodeID:   unit.NodeID,
			CaseID:   unit.CaseID,
		})
	}
	return findings
}

// Report aggregates every Finding collected while auditing a batch of
// CitedUnits (typically every hybridretrieval.Item in a single
// hybridretrieval.Result), mirroring packages/treevalidation's Report.
type Report struct {
	// CaseID identifies the case whose citations this Report was computed
	// for.
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
