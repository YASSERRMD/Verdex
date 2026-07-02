package vectorindex

import "errors"

// Sentinel errors that callers can test with errors.Is.
var (
	// ErrEmptyCaseID is returned when a case-scoped operation (BuildLeaves,
	// ReindexCase, Delete) is called with an empty case ID.
	ErrEmptyCaseID = errors.New("vectorindex: case id is required")

	// ErrNilGraphStore is returned when a function requiring a
	// graph.GraphStore is called with a nil store.
	ErrNilGraphStore = errors.New("vectorindex: graph store must not be nil")

	// ErrNilEmbeddingService is returned when a function requiring an
	// embedding.EmbeddingService is called with a nil service.
	ErrNilEmbeddingService = errors.New("vectorindex: embedding service must not be nil")

	// ErrNilVectorStore is returned when a function requiring a VectorStore
	// is called with a nil store.
	ErrNilVectorStore = errors.New("vectorindex: vector store must not be nil")

	// ErrEmptyVector is returned by Upsert or Query when given a
	// zero-length embedding vector.
	ErrEmptyVector = errors.New("vectorindex: vector must not be empty")

	// ErrDimensionMismatch is returned by Upsert or Query when a vector's
	// dimensionality does not match the dimensionality already established
	// by the store's existing entries.
	ErrDimensionMismatch = errors.New("vectorindex: vector dimension mismatch")

	// ErrRecordNotFound is returned by Delete when no record matches the
	// requested id.
	ErrRecordNotFound = errors.New("vectorindex: record not found")

	// ErrEmptyRecordID is returned when a record-id-scoped operation is
	// called with an empty id.
	ErrEmptyRecordID = errors.New("vectorindex: record id is required")

	// ErrInvalidTopK is returned by Query when TopK is less than 1.
	ErrInvalidTopK = errors.New("vectorindex: top-k must be at least 1")
)
