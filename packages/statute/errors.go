package statute

import "errors"

// Sentinel errors that callers can test with errors.Is.
var (
	// ErrMalformedCorpus is returned when a Loader cannot recognize the
	// shape of the supplied corpus source (empty input, invalid JSON, or
	// text with no recognizable act header).
	ErrMalformedCorpus = errors.New("statute: malformed statute corpus")

	// ErrUnresolvedCrossReference is returned by strict cross-reference
	// resolution when a detected reference (e.g. "see Section 12") does
	// not resolve to any rule node ID within the same loaded corpus.
	ErrUnresolvedCrossReference = errors.New("statute: unresolved cross-reference")

	// ErrEmptyInput is returned when an operation is given empty (or
	// whitespace-only, after trimming) required text input.
	ErrEmptyInput = errors.New("statute: input is empty")

	// ErrRuleNotFound is returned when a lookup by rule node ID finds no
	// matching record.
	ErrRuleNotFound = errors.New("statute: rule not found")

	// ErrPersistFailed is returned when persistence of one or more
	// irac.RuleNode values via graph.GraphStore fails.
	ErrPersistFailed = errors.New("statute: failed to persist rule node")
)
