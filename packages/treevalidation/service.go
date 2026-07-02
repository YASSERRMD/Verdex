package treevalidation

import (
	"fmt"

	"github.com/YASSERRMD/verdex/packages/irac"
	"github.com/YASSERRMD/verdex/packages/treeassembly"
)

// CodeStructuralIssue translates an irac.ValidationIssue (see
// packages/irac/validate.go) into a Finding's Code field.
const CodeStructuralIssue = "structural_issue"

// CodeSemanticGap translates a treeassembly.Gap (see
// packages/treeassembly/gap.go) into a Finding's Code field.
const CodeSemanticGap = "semantic_gap"

// DefaultConfidenceThreshold is the confidence threshold
// TreeValidationService uses for FlagUnsupportedClaims when
// ConfidenceThreshold is left at its zero value.
const DefaultConfidenceThreshold = 0.5

// TreeValidationService orchestrates the full capstone integrity-gate
// pipeline over an assembled treeassembly.Tree:
//
//  1. structural validation, via irac.ValidateTree (composed through
//     treeassembly.ValidateIntegrity — not reimplemented here);
//  2. semantic gap detection, via treeassembly.DetectGaps (not
//     reimplemented here);
//  3. this package's own six checks: conclusion traceability, orphan
//     detection, full-graph cycle detection, unsupported-claim flagging,
//     confidence propagation, and jurisdiction consistency;
//  4. aggregation of every Finding from 1-3 into a Report;
//  5. the CanFinalize hard gate.
//
// This mirrors packages/treeassembly's own TreeAssemblyService
// orchestration pattern: a single entry point wires together this
// package's otherwise independent, individually testable checks.
type TreeValidationService struct {
	// CaseJurisdictionCode is the case's declared jurisdiction, checked
	// against every irac.RuleNode in the tree by
	// CheckJurisdictionConsistency. Leave blank to skip the jurisdiction-
	// consistency check entirely.
	CaseJurisdictionCode string

	// AllowedJurisdictionOverrides lists jurisdiction codes permitted to
	// differ from CaseJurisdictionCode (e.g. persuasive foreign
	// authority explicitly cited by the case).
	AllowedJurisdictionOverrides []string

	// ConfidenceThreshold is the minimum acceptable node confidence
	// passed to FlagUnsupportedClaims. If zero, DefaultConfidenceThreshold
	// is used.
	ConfidenceThreshold float64
}

// Validate runs the full pipeline described in the TreeValidationService
// doc comment over tree, returning the aggregated *Report alongside any
// error. Validate returns ErrEmptyTree (with a non-nil Report describing
// nothing else was checked) if tree has no nodes. Otherwise, Validate
// always returns a fully populated Report; the returned error is nil
// unless CanFinalize's gate rejects the Report (ErrCriticalFindings), in
// which case the Report is still returned so callers can inspect what
// went wrong.
func (s TreeValidationService) Validate(tree treeassembly.Tree) (*Report, error) {
	caseID := caseIDOf(tree)
	report := &Report{CaseID: caseID, Findings: make([]Finding, 0)}

	if len(tree.Nodes) == 0 {
		return report, ErrEmptyTree
	}

	// 1. structural validation, composed through treeassembly's own
	// wrapper rather than calling irac.ValidateTree directly.
	structuralIssues := treeassembly.ValidateIntegrity(&tree)
	for _, issue := range structuralIssues {
		report.Findings = append(report.Findings, Finding{
			Severity: SeverityCritical,
			Code:     CodeStructuralIssue,
			Message:  issue.Message,
			NodeID:   issue.NodeID,
		})
	}

	// 2. semantic gap detection, composed through treeassembly's own
	// DetectGaps rather than reimplementing gap analysis here.
	gaps := treeassembly.DetectGaps(&tree)
	for _, gap := range gaps {
		report.Findings = append(report.Findings, Finding{
			Severity: SeverityWarning,
			Code:     CodeSemanticGap,
			Message:  gap.Description,
			NodeID:   gap.NodeID,
		})
	}

	// 3. this package's own checks.
	report.Findings = append(report.Findings, CheckConclusionTraceability(tree)...)
	report.Findings = append(report.Findings, DetectOrphans(tree)...)
	report.Findings = append(report.Findings, DetectCycles(tree)...)

	threshold := s.ConfidenceThreshold
	if threshold == 0 {
		threshold = DefaultConfidenceThreshold
	}
	report.Findings = append(report.Findings, FlagUnsupportedClaims(tree, threshold)...)

	report.Findings = append(report.Findings, CheckConfidencePropagation(tree)...)

	if s.CaseJurisdictionCode != "" {
		report.Findings = append(
			report.Findings,
			CheckJurisdictionConsistency(tree, s.CaseJurisdictionCode, s.AllowedJurisdictionOverrides...)...,
		)
	}

	// 4-5. gate.
	if ok, err := CanFinalize(*report); !ok {
		return report, err
	}

	return report, nil
}

// caseIDOf returns the CaseID shared by tree's nodes, inspecting the
// first node that carries one. Returns "" if tree has no nodes or no
// node exposes a CaseID (irac.NodeLike itself does not — this type-
// switches over the concrete wrapper types, mirroring unsupported.go's
// nodeSpans/nodeConfidence helpers).
func caseIDOf(tree treeassembly.Tree) string {
	for _, n := range tree.Nodes {
		switch v := n.(type) {
		case irac.IssueNode:
			return v.CaseID
		case irac.RuleNode:
			return v.CaseID
		case irac.FactNode:
			return v.CaseID
		case irac.ApplicationNode:
			return v.CaseID
		case irac.ConclusionNode:
			return v.CaseID
		default:
			continue
		}
	}
	return ""
}

// String implements fmt.Stringer for Report, delegating to Summary for a
// concise representation.
func (r Report) String() string {
	return fmt.Sprintf("Report{CaseID: %q, %s}", r.CaseID, r.Summary())
}
