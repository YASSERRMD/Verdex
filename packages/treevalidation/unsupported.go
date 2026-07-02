package treevalidation

import (
	"fmt"

	"github.com/YASSERRMD/verdex/packages/irac"
	"github.com/YASSERRMD/verdex/packages/treeassembly"
)

// CodeMissingSourceSpans flags a node whose Spans (source-span backing)
// is empty.
const CodeMissingSourceSpans = "missing_source_spans"

// CodeLowConfidence flags a node whose Confidence is below the
// configured threshold.
const CodeLowConfidence = "low_confidence"

// nodeSpans returns the []irac.SourceSpan carried by n, and true if n is
// one of the concrete node types that carries Spans. irac.NodeLike itself
// exposes no Spans accessor (only GetID/GetType), so this type-switches
// over the concrete wrapper types declared in packages/irac/node.go.
func nodeSpans(n irac.NodeLike) ([]irac.SourceSpan, bool) {
	switch v := n.(type) {
	case irac.IssueNode:
		return v.Spans, true
	case irac.RuleNode:
		return v.Spans, true
	case irac.FactNode:
		return v.Spans, true
	case irac.ApplicationNode:
		return v.Spans, true
	case irac.ConclusionNode:
		return v.Spans, true
	default:
		return nil, false
	}
}

// nodeConfidence returns the Confidence carried by n, and true if n is one
// of the concrete node types this package recognizes.
func nodeConfidence(n irac.NodeLike) (float64, bool) {
	switch v := n.(type) {
	case irac.IssueNode:
		return v.Confidence, true
	case irac.RuleNode:
		return v.Confidence, true
	case irac.FactNode:
		return v.Confidence, true
	case irac.ApplicationNode:
		return v.Confidence, true
	case irac.ConclusionNode:
		return v.Confidence, true
	default:
		return 0, false
	}
}

// FlagUnsupportedClaims returns a Finding for every node in tree that is
// either:
//
//   - missing source-span backing (empty Spans), or
//   - carrying a Confidence strictly below confidenceThreshold.
//
// Both conditions indicate the node's claim is not adequately supported
// to be relied upon downstream. A nil tree yields no findings. Order is
// deterministic: input order of tree.Nodes.
func FlagUnsupportedClaims(tree treeassembly.Tree, confidenceThreshold float64) []Finding {
	findings := make([]Finding, 0)

	for _, n := range tree.Nodes {
		if spans, ok := nodeSpans(n); ok && len(spans) == 0 {
			findings = append(findings, Finding{
				Severity: SeverityWarning,
				Code:     CodeMissingSourceSpans,
				Message:  fmt.Sprintf("node %q (%s) has no source-span backing", n.GetID(), n.GetType()),
				NodeID:   n.GetID(),
			})
		}

		if confidence, ok := nodeConfidence(n); ok && confidence < confidenceThreshold {
			findings = append(findings, Finding{
				Severity: SeverityWarning,
				Code:     CodeLowConfidence,
				Message: fmt.Sprintf(
					"node %q (%s) has confidence %.4f below threshold %.4f",
					n.GetID(), n.GetType(), confidence, confidenceThreshold,
				),
				NodeID: n.GetID(),
			})
		}
	}

	return findings
}
