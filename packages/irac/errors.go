package irac

import "errors"

// Sentinel errors that callers can test with errors.Is.
var (
	// ErrDanglingEdge is returned when an Edge references a FromID or ToID
	// that does not correspond to any node in the tree being validated.
	ErrDanglingEdge = errors.New("irac: edge references unknown node id")

	// ErrIllegalEdgeTriple is returned when an Edge's (FromNodeType,
	// EdgeType, ToNodeType) triple is not present in the legal-constraint
	// table (see edge.go).
	ErrIllegalEdgeTriple = errors.New("irac: illegal edge triple for node types")

	// ErrMissingGuardrailLabel is returned when a ConclusionNode is found
	// (e.g. during unmarshal or validation) without the mandatory
	// draft_analysis label attached.
	ErrMissingGuardrailLabel = errors.New("irac: conclusion node missing draft_analysis guardrail label")

	// ErrSelfLoop is returned when an Edge's FromID equals its ToID.
	ErrSelfLoop = errors.New("irac: edge is a self-loop")

	// ErrEmptyCaseID is returned when a node or revision is constructed or
	// validated with an empty CaseID.
	ErrEmptyCaseID = errors.New("irac: case id is required")

	// ErrUnknownNodeType is returned when a Node's Type is not one of the
	// recognized NodeType constants.
	ErrUnknownNodeType = errors.New("irac: unknown node type")

	// ErrUnknownEdgeType is returned when an Edge's Type is not one of the
	// recognized EdgeType constants.
	ErrUnknownEdgeType = errors.New("irac: unknown edge type")
)
