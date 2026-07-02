package vectorindex

import "context"

// VectorStore persists and queries VectorRecords produced by embedding
// IndexableLeaf values. It mirrors packages/graph's GraphStore/
// InMemoryGraphStore split: this interface is the storage-agnostic
// contract, and InMemoryVectorStore (inmemory.go) is the brute-force
// reference implementation used by tests and small deployments today. A
// real ANN backend (e.g. pgvector, already an indirect dependency of this
// workspace via packages/graph) can implement this same interface later
// without any caller-visible change — see IndexConfig and
// doc/vector-index.md's "ANN extension point" section.
type VectorStore interface {
	// Upsert stores record, overwriting any existing record with the same
	// ID. Mirrors GraphStore.CreateNode's idempotent-upsert contract, so
	// re-indexing a leaf (e.g. after a tree revision) is always safe to
	// repeat. Returns ErrEmptyRecordID if record.ID is empty, or
	// ErrEmptyVector if record.Vector is empty.
	Upsert(ctx context.Context, record VectorRecord) error

	// Query returns the top-K VectorRecords most similar to req.Vector
	// under the store's configured Metric, restricted to records matching
	// req.Filter and (if set) req.CaseID. Results are sorted by descending
	// VectorScore. Returns ErrEmptyVector if req.Vector is empty, or
	// ErrInvalidTopK if req.TopK is negative.
	Query(ctx context.Context, req QueryRequest) ([]ScoredResult, error)

	// Delete removes the record with the given id. It is not an error to
	// delete an id that does not exist, mirroring GraphStore.DeleteTree's
	// "not an error to delete a case with no nodes" convention. Returns
	// ErrEmptyRecordID if id is empty.
	Delete(ctx context.Context, id string) error

	// DeleteCase removes every record belonging to caseID. It is not an
	// error to delete a case with no records. Returns ErrEmptyCaseID if
	// caseID is empty.
	DeleteCase(ctx context.Context, caseID string) error

	// Health reports whether the store is currently able to serve reads
	// and writes. Mirrors packages/graph's HealthCheck free function, but
	// exposed as a method here since (unlike GraphStore) every VectorStore
	// implementation this package defines is expected to answer its own
	// health directly rather than needing a type-switch dispatcher (see
	// HealthCheck in health.go for the dispatcher this package still
	// provides for consistency with packages/graph's pattern).
	Health(ctx context.Context) error
}
