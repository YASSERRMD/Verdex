package treevalidation

import "errors"

// Sentinel errors that callers can test with errors.Is.
var (
	// ErrCriticalFindings is returned by CanFinalize (and, transitively,
	// by TreeValidationService.Validate) when a Report contains at least
	// one SeverityCritical Finding — the tree must not be used downstream
	// until every critical Finding is resolved.
	ErrCriticalFindings = errors.New("treevalidation: tree has critical validation findings")

	// ErrEmptyTree is returned when Validate is called with a tree that
	// has no nodes at all — there is nothing to validate.
	ErrEmptyTree = errors.New("treevalidation: tree has no nodes")
)
