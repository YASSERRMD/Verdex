package treevalidation

import (
	"fmt"
	"math"

	"github.com/YASSERRMD/verdex/packages/irac"
	"github.com/YASSERRMD/verdex/packages/treeassembly"
)

// CodeConfidenceExceedsChain flags a ConclusionNode whose Confidence
// exceeds the minimum confidence found along its supporting chain (the
// Application it concludes from, and every Fact/Rule that Application
// applies).
const CodeConfidenceExceedsChain = "confidence_exceeds_chain"

// CheckConfidencePropagation enforces that a ConclusionNode's confidence
// never overstates what its supporting chain justifies: a conclusion is
// only as reliable as the weakest link feeding it (Conclusion
// --concludes_from--> Application --applies_to--> Fact/Rule). A
// conclusion reporting higher confidence than the minimum confidence
// found across that chain is flagged, since it would be overstating the
// strength of its own reasoning relative to its inputs.
//
// Conclusions with no resolvable supporting chain are skipped here —
// that defect is CheckConclusionTraceability's responsibility to flag,
// not this check's (composition, not duplication).
//
// A nil tree yields no findings. Order is deterministic: input order of
// tree.Nodes, restricted to ConclusionNodes.
func CheckConfidencePropagation(tree treeassembly.Tree) []Finding {
	findings := make([]Finding, 0)

	nodeByID := make(map[string]irac.NodeLike, len(tree.Nodes))
	for _, n := range tree.Nodes {
		nodeByID[n.GetID()] = n
	}

	applicationsFromConclusion := make(map[string]map[string]struct{})
	for _, e := range tree.Edges {
		if e.Type != irac.EdgeConcludesFrom {
			continue
		}
		if applicationsFromConclusion[e.FromID] == nil {
			applicationsFromConclusion[e.FromID] = make(map[string]struct{})
		}
		applicationsFromConclusion[e.FromID][e.ToID] = struct{}{}
	}

	appliesFromApplication := make(map[string]map[string]struct{})
	for _, e := range tree.Edges {
		if e.Type != irac.EdgeAppliesTo {
			continue
		}
		if appliesFromApplication[e.FromID] == nil {
			appliesFromApplication[e.FromID] = make(map[string]struct{})
		}
		appliesFromApplication[e.FromID][e.ToID] = struct{}{}
	}

	for _, n := range tree.Nodes {
		conclusion, ok := n.(irac.ConclusionNode)
		if !ok {
			continue
		}

		minChainConfidence := math.Inf(1)
		chainFound := false

		for appID := range applicationsFromConclusion[conclusion.ID] {
			if appNode, ok := nodeByID[appID]; ok {
				if c, ok := nodeConfidence(appNode); ok {
					chainFound = true
					if c < minChainConfidence {
						minChainConfidence = c
					}
				}
			}
			for targetID := range appliesFromApplication[appID] {
				targetNode, ok := nodeByID[targetID]
				if !ok {
					continue
				}
				c, ok := nodeConfidence(targetNode)
				if !ok {
					continue
				}
				chainFound = true
				if c < minChainConfidence {
					minChainConfidence = c
				}
			}
		}

		if !chainFound {
			continue
		}

		if conclusion.Confidence > minChainConfidence {
			findings = append(findings, Finding{
				Severity: SeverityCritical,
				Code:     CodeConfidenceExceedsChain,
				Message: fmt.Sprintf(
					"conclusion %q has confidence %.4f exceeding its supporting chain's minimum confidence %.4f",
					conclusion.ID, conclusion.Confidence, minChainConfidence,
				),
				NodeID: conclusion.ID,
			})
		}
	}

	return findings
}
