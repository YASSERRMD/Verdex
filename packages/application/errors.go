package application

import "errors"

// Sentinel errors that callers can test with errors.Is.
var (
	// ErrNoMatchingRules is returned when MatchIssueToRules (or a
	// higher-level pipeline stage built on it) finds no candidate rule
	// that scores above zero against an issue.
	ErrNoMatchingRules = errors.New("application: no matching rules for issue")

	// ErrEmptyInput is returned when an operation is given empty (or
	// whitespace-only, after trimming) required text input.
	ErrEmptyInput = errors.New("application: input is empty")

	// ErrCyclicChain is returned when a RuleChain's Validate detects a
	// cycle among its OriginatedRules (an OriginatedRule's underlying
	// irac.RuleNode.ID appears more than once in the chain).
	ErrCyclicChain = errors.New("application: rule chain contains a cycle")

	// ErrIllegalEdge is returned when persisting an application subgraph
	// would require an irac.Edge whose (FromNodeType, EdgeType,
	// ToNodeType) triple is not present in packages/irac/edge.go's
	// legalEdgeTriples constraint table.
	ErrIllegalEdge = errors.New("application: edge is not legal per irac constraint table")

	// ErrApplicationNotFound is returned when a lookup by
	// irac.ApplicationNode ID finds no matching record.
	ErrApplicationNotFound = errors.New("application: application node not found")

	// ErrPersistFailed is returned when persistence of an application
	// subgraph (nodes and/or edges) via graph.GraphStore fails.
	ErrPersistFailed = errors.New("application: failed to persist application subgraph")
)
