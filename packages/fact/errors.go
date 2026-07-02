package fact

import "errors"

// Sentinel errors that callers can test with errors.Is.
var (
	// ErrClassificationInvalid is returned when a
	// evidence.Classification cannot be turned into a candidate fact
	// (e.g. empty SegmentID, or no segment text supplied for it).
	ErrClassificationInvalid = errors.New("fact: invalid classification")

	// ErrEmptyInput is returned when an operation is given empty (or
	// whitespace-only, after trimming) required text input.
	ErrEmptyInput = errors.New("fact: input is empty")

	// ErrFactNotFound is returned when a lookup by fact node ID finds no
	// matching record.
	ErrFactNotFound = errors.New("fact: fact not found")

	// ErrPersistFailed is returned when persistence of one or more
	// irac.FactNode values (or their supporting edges) via
	// graph.GraphStore fails.
	ErrPersistFailed = errors.New("fact: failed to persist fact node")
)
