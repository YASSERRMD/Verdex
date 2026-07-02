package precedent

import "errors"

// Sentinel errors that callers can test with errors.Is.
var (
	// ErrMalformedCorpus is returned when a Loader cannot recognize the
	// shape of the supplied precedent corpus source (empty input, invalid
	// JSON, or text with no recognizable case header).
	ErrMalformedCorpus = errors.New("precedent: malformed precedent corpus")

	// ErrHoldingNotFound is returned when a deterministic extraction pass
	// cannot locate a holding section within a precedent's FullText (see
	// holding.go). This is not necessarily fatal to a whole-corpus
	// ingestion — callers may choose to proceed with an empty Holding
	// rather than fail the batch.
	ErrHoldingNotFound = errors.New("precedent: holding not found")

	// ErrEmptyInput is returned when an operation is given empty (or
	// whitespace-only, after trimming) required input.
	ErrEmptyInput = errors.New("precedent: input is empty")

	// ErrRuleNotFound is returned when a lookup by rule node ID finds no
	// matching record.
	ErrRuleNotFound = errors.New("precedent: rule not found")

	// ErrPersistFailed is returned when persistence of one or more
	// irac.RuleNode values via graph.GraphStore fails.
	ErrPersistFailed = errors.New("precedent: failed to persist rule node")
)
