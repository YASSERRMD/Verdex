package graph

import "errors"

// Sentinel errors that callers can test with errors.Is.
var (
	// ErrNodeNotFound is returned by GetNode (and other lookups) when no
	// node matches the requested id.
	ErrNodeNotFound = errors.New("graph: node not found")

	// ErrCrossTenantAccess is returned by TenantScopedStore when a
	// caller attempts to read or write a node/edge belonging to a
	// different tenant than the one the store is scoped to.
	ErrCrossTenantAccess = errors.New("graph: cross-tenant access rejected")

	// ErrTransactionFailed is returned by WithTransaction when the
	// wrapped store's writes could not be committed or rolled back
	// cleanly.
	ErrTransactionFailed = errors.New("graph: transaction failed")

	// ErrEmptyCaseID is returned when a caseID-scoped operation
	// (Traverse, DeleteTree, Export) is called with an empty caseID.
	ErrEmptyCaseID = errors.New("graph: case id is required")

	// ErrEmptyNodeID is returned when a node-id-scoped operation is
	// called with an empty id.
	ErrEmptyNodeID = errors.New("graph: node id is required")
)
