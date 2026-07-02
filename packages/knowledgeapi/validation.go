package knowledgeapi

import (
	"context"
	"errors"

	"github.com/YASSERRMD/verdex/packages/graph"
	"github.com/YASSERRMD/verdex/packages/irac"
	"github.com/YASSERRMD/verdex/packages/treeassembly"
	"github.com/YASSERRMD/verdex/packages/treevalidation"
)

// ValidationStatus returns the current integrity/validation status of
// this KnowledgeAPI's case, composing treevalidation.TreeValidationService.
// Validate and treevalidation.CanFinalize rather than re-deriving any of
// treevalidation's six checks here. The tree is assembled from a full
// case-scoped Traverse plus this package's own edge-loading helper
// (tree.go), converting each irac.Node into the typed irac.NodeLike
// wrapper treeassembly.Tree requires.
//
// CanFinalize in the response is false whenever the aggregated Report
// contains at least one SeverityCritical Finding, per treevalidation's
// hard gate — this mirrors, for tree reads exposed through this facade,
// the same guarantee Phase 040's treevalidation established for any
// future downstream consumer of an assembled tree. CanFinalize is also
// forced false when the case has no nodes at all (treevalidation.
// ErrEmptyTree): CanFinalize's own vacuous-true behaviour for a Report
// with zero Findings is correct for "no critical problems found", but a
// tree with literally nothing in it is not a tree a caller should ever
// treat as finalizable, so this endpoint does not let that ambiguity
// leak through its CanFinalize field.
func (api *KnowledgeAPI) ValidationStatus(ctx context.Context, req ValidationStatusRequest) (ValidationStatusResponse, error) {
	if err := authorize(ctx); err != nil {
		return ValidationStatusResponse{}, err
	}
	if req.CaseID == "" || req.CaseID != api.caseID {
		return ValidationStatusResponse{}, ErrEmptyCaseID
	}

	nodes, err := api.store.Traverse(ctx, graph.TraversalQuery{CaseID: api.caseID})
	if err != nil {
		return ValidationStatusResponse{}, err
	}

	edges, err := loadCaseEdges(ctx, api.store, api.caseID)
	if err != nil {
		return ValidationStatusResponse{}, err
	}

	tree := treeassembly.Tree{
		Nodes: nodeLikesFromNodes(nodes),
		Edges: edges,
	}

	service := treevalidation.TreeValidationService{
		CaseJurisdictionCode:         api.validation.caseJurisdictionCode,
		AllowedJurisdictionOverrides: api.validation.allowedJurisdictionOverrides,
		ConfidenceThreshold:          api.validation.confidenceThreshold,
	}

	// service.Validate's own returned error (ErrEmptyTree or
	// ErrCriticalFindings) is intentionally not treated as a hard failure
	// here: this endpoint's contract is to report validation STATUS, not
	// to fail the request when a tree happens to be unfinalizable. The
	// Report it always returns (even on error) is what CanFinalize below
	// re-derives the boolean/summary from.
	report, validateErr := service.Validate(tree)
	if report == nil {
		report = &treevalidation.Report{CaseID: api.caseID}
	}

	canFinalize, _ := treevalidation.CanFinalize(*report)
	if errors.Is(validateErr, treevalidation.ErrEmptyTree) {
		canFinalize = false
	}

	findings := make([]FindingDTO, 0, len(report.Findings))
	for _, f := range report.Findings {
		findings = append(findings, FindingDTO{
			Severity: string(f.Severity),
			Code:     f.Code,
			Message:  f.Message,
			NodeID:   f.NodeID,
		})
	}

	return ValidationStatusResponse{
		Version:     APIVersionV1,
		CaseID:      api.caseID,
		CanFinalize: canFinalize,
		Summary:     report.Summary(),
		Findings:    findings,
	}, nil
}

// nodeLikesFromNodes converts every irac.Node into the typed irac.NodeLike
// wrapper treeassembly.Tree requires, dispatching on irac.NodeType. Nodes
// of an unrecognized type are skipped rather than causing the whole
// conversion to fail, since treevalidation's own checks only inspect the
// five known node types.
func nodeLikesFromNodes(nodes []irac.Node) []irac.NodeLike {
	out := make([]irac.NodeLike, 0, len(nodes))
	for _, n := range nodes {
		switch n.Type {
		case irac.NodeIssue:
			out = append(out, irac.IssueNode{Node: n})
		case irac.NodeRule:
			out = append(out, irac.RuleNode{Node: n})
		case irac.NodeFact:
			out = append(out, irac.FactNode{Node: n})
		case irac.NodeApplication:
			out = append(out, irac.ApplicationNode{Node: n})
		case irac.NodeConclusion:
			out = append(out, irac.ConclusionNode{Node: n, Label: irac.DraftAnalysisLabel})
		}
	}
	return out
}
