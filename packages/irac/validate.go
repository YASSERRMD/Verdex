package irac

import "fmt"

// ValidationIssue is a single structured problem found while validating a
// tree's nodes and edges. ValidateTree collects every issue it finds
// rather than failing fast, so callers can see everything wrong with a
// tree at once instead of fixing problems one error at a time.
type ValidationIssue struct {
	// Err is the sentinel error classifying this issue (e.g.
	// ErrDanglingEdge, ErrIllegalEdgeTriple, ErrSelfLoop,
	// ErrMissingGuardrailLabel, ErrUnknownNodeType, ErrUnknownEdgeType).
	Err error

	// Message is a human-readable description of this specific
	// occurrence, including the offending IDs/types.
	Message string

	// EdgeIndex is the index into the edges slice passed to ValidateTree
	// that this issue concerns, or -1 if the issue concerns a node rather
	// than an edge.
	EdgeIndex int

	// NodeID is the ID of the node this issue concerns, or empty if the
	// issue concerns an edge whose endpoint node could not be resolved.
	NodeID string
}

// Error implements the error interface so a ValidationIssue can be used
// directly wherever a single error is expected (e.g. wrapped in a
// multi-error), while ValidateTree's []ValidationIssue return type keeps
// every issue visible to callers that want the full picture.
func (v ValidationIssue) Error() string {
	return v.Message
}

// ValidateTree checks a candidate set of nodes and edges for IRAC tree
// integrity:
//
//   - every edge's FromID and ToID reference a node present in nodes
//     (else ErrDanglingEdge);
//   - every edge's (FromNodeType, EdgeType, ToNodeType) triple is present
//     in the legal-constraint table from edge.go (else
//     ErrIllegalEdgeTriple);
//   - no edge is a self-loop, i.e. FromID == ToID (else ErrSelfLoop).
//
// It does not fail fast: every problem found across every edge is
// collected and returned, so callers can see everything wrong with a
// tree in one pass. Returns an empty (non-nil) slice when no issues are
// found.
func ValidateTree(nodes []NodeLike, edges []Edge) []ValidationIssue {
	issues := make([]ValidationIssue, 0)

	byID := make(map[string]NodeLike, len(nodes))
	for _, n := range nodes {
		byID[n.GetID()] = n
	}

	for i, e := range edges {
		if e.FromID == e.ToID {
			issues = append(issues, ValidationIssue{
				Err:       ErrSelfLoop,
				Message:   fmt.Sprintf("edge %d: self-loop on node %q", i, e.FromID),
				EdgeIndex: i,
				NodeID:    e.FromID,
			})
		}

		fromNode, fromOK := byID[e.FromID]
		if !fromOK {
			issues = append(issues, ValidationIssue{
				Err:       ErrDanglingEdge,
				Message:   fmt.Sprintf("edge %d: from-node %q not found", i, e.FromID),
				EdgeIndex: i,
				NodeID:    e.FromID,
			})
		}

		toNode, toOK := byID[e.ToID]
		if !toOK {
			issues = append(issues, ValidationIssue{
				Err:       ErrDanglingEdge,
				Message:   fmt.Sprintf("edge %d: to-node %q not found", i, e.ToID),
				EdgeIndex: i,
				NodeID:    e.ToID,
			})
		}

		if !e.Type.IsValid() {
			issues = append(issues, ValidationIssue{
				Err:       ErrUnknownEdgeType,
				Message:   fmt.Sprintf("edge %d: unknown edge type %q", i, e.Type),
				EdgeIndex: i,
			})
			continue
		}

		// Only check the triple when both endpoints resolved; a dangling
		// reference has already been reported above and checking the
		// triple against a missing node would be meaningless.
		if fromOK && toOK {
			if !IsLegalEdgeTriple(fromNode.GetType(), e.Type, toNode.GetType()) {
				issues = append(issues, ValidationIssue{
					Err: ErrIllegalEdgeTriple,
					Message: fmt.Sprintf(
						"edge %d: illegal triple (%s --%s--> %s)",
						i, fromNode.GetType(), e.Type, toNode.GetType(),
					),
					EdgeIndex: i,
				})
			}
		}
	}

	for _, n := range nodes {
		if !n.GetType().IsValid() {
			issues = append(issues, ValidationIssue{
				Err:       ErrUnknownNodeType,
				Message:   fmt.Sprintf("node %q: unknown node type %q", n.GetID(), n.GetType()),
				EdgeIndex: -1,
				NodeID:    n.GetID(),
			})
			continue
		}
		if c, ok := n.(ConclusionNode); ok {
			if !c.HasGuardrailLabel() {
				issues = append(issues, ValidationIssue{
					Err:       ErrMissingGuardrailLabel,
					Message:   fmt.Sprintf("node %q: conclusion missing draft_analysis guardrail label", n.GetID()),
					EdgeIndex: -1,
					NodeID:    n.GetID(),
				})
			}
		}
	}

	return issues
}
