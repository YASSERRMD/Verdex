package treeindex

import "errors"

// Sentinel errors that callers can test with errors.Is.
var (
	// ErrEmptyCaseID is returned when a case-scoped operation (RebuildCase,
	// LookupPaths) is called with an empty case ID.
	ErrEmptyCaseID = errors.New("treeindex: case id is required")

	// ErrNilGraphStore is returned when a function requiring a
	// graph.GraphStore is called with a nil store.
	ErrNilGraphStore = errors.New("treeindex: graph store must not be nil")

	// ErrEmptyNodeID is returned when a node-scoped operation is called
	// with an empty node id.
	ErrEmptyNodeID = errors.New("treeindex: node id is required")

	// ErrInvalidEdgeType is returned when a lookup is given an
	// irac.EdgeType that is not one of irac.AllEdgeTypes.
	ErrInvalidEdgeType = errors.New("treeindex: edge type is invalid")

	// ErrInvalidCacheCapacity is returned when NewLRUCache is called with
	// a non-positive capacity.
	ErrInvalidCacheCapacity = errors.New("treeindex: cache capacity must be positive")

	// ErrCaseNotIndexed is returned by LookupPaths when the requested
	// case has never been built via RebuildCase (or ReindexOnRevision).
	ErrCaseNotIndexed = errors.New("treeindex: case has not been indexed")
)
