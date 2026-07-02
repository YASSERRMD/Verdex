package vectorindex

import (
	"context"
	"fmt"
)

// HealthCheck reports whether store is currently able to serve reads and
// writes. Mirrors packages/graph's HealthCheck free function: for
// InMemoryVectorStore, health is always nil (an in-memory, map-backed store
// has nothing external to fail against); any other VectorStore
// implementation is expected to answer its own Health method, which this
// function delegates to.
func HealthCheck(ctx context.Context, store VectorStore) error {
	if store == nil {
		return fmt.Errorf("vectorindex: HealthCheck: %w", ErrNilVectorStore)
	}
	return store.Health(ctx)
}
