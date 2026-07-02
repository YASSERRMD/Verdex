package citation

import "errors"

// Sentinel errors that callers can test with errors.Is.
var (
	// ErrEmptyNodeID is returned when a function requiring a node ID is
	// called with an empty string.
	ErrEmptyNodeID = errors.New("citation: node id must not be empty")

	// ErrEmptyCaseID is returned when a function requiring a case ID is
	// called with an empty string.
	ErrEmptyCaseID = errors.New("citation: case id must not be empty")

	// ErrNilGraphStore is returned when a function requiring a
	// graph.GraphStore is called with a nil store.
	ErrNilGraphStore = errors.New("citation: graph store must not be nil")

	// ErrNilResolver is returned when a function requiring a Resolver is
	// called with a nil resolver.
	ErrNilResolver = errors.New("citation: resolver must not be nil")

	// ErrUnresolvedCitation is returned when a Resolver cannot produce any
	// citation text for a node (e.g. the node carries no formatted
	// citation and no fallback heuristic applies).
	ErrUnresolvedCitation = errors.New("citation: unable to resolve citation for node")

	// ErrCitationNotFound is returned by a Repository lookup when no
	// CitedUnit is stored under the requested key.
	ErrCitationNotFound = errors.New("citation: citation not found")

	// ErrUnknownFormatter is returned when Registry.Format is called with a
	// jurisdiction/legal-family key that has no registered Formatter and
	// no default formatter was configured.
	ErrUnknownFormatter = errors.New("citation: no formatter registered for key")
)
