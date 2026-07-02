package hybridretrieval

import "errors"

// Sentinel errors that callers can test with errors.Is.
var (
	// ErrNilVectorStore is returned when a function requiring a
	// vectorindex.VectorStore is called with a nil store.
	ErrNilVectorStore = errors.New("hybridretrieval: vector store must not be nil")

	// ErrNilGraphStore is returned when a function requiring a
	// graph.GraphStore is called with a nil store.
	ErrNilGraphStore = errors.New("hybridretrieval: graph store must not be nil")

	// ErrEmptyVector is returned when a HybridQuery is executed with no
	// query vector and no anchor node id — there is nothing to search
	// from.
	ErrEmptyVector = errors.New("hybridretrieval: query vector must not be empty when no anchor node id is set")

	// ErrEmptyCaseID is returned when a HybridQuery is executed with no
	// case id set.
	ErrEmptyCaseID = errors.New("hybridretrieval: query case id is required")

	// ErrInvalidTopK is returned when a HybridQuery's TopK is negative.
	ErrInvalidTopK = errors.New("hybridretrieval: top-k must not be negative")

	// ErrInvalidMaxPerAnchor is returned when a HybridQuery's
	// MaxPerAnchor is negative.
	ErrInvalidMaxPerAnchor = errors.New("hybridretrieval: max results per anchor must not be negative")

	// errUnrecognizedExpansionHop is an internal error wrapped with the
	// offending ExpansionHop value when a HybridQuery.ExpansionHops entry
	// is not one of the recognized ExpansionHop constants. Not exported:
	// ExpansionHop.IsValid lets a caller check this proactively, and
	// Retriever.Retrieve wraps this into its own returned error with
	// context, so there is no reason for a caller to test for this exact
	// sentinel with errors.Is.
	errUnrecognizedExpansionHop = errors.New("unrecognized expansion hop")
)
