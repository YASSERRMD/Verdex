package segmentation

import "errors"

// Sentinel errors that callers can test with errors.Is.
var (
	// ErrEmptyInput is returned when a segmentation operation is given
	// empty (or whitespace-only, after cleanup) text.
	ErrEmptyInput = errors.New("segmentation: input is empty")

	// ErrSpanOverlap is returned when two segments' SourceSpans overlap in
	// a way that violates the non-overlapping, gap-free coverage invariant
	// segmentation must preserve over the original source text.
	ErrSpanOverlap = errors.New("segmentation: source spans overlap")

	// ErrInvalidRequest is returned when a request contains invalid or
	// missing fields.
	ErrInvalidRequest = errors.New("segmentation: invalid request")
)
