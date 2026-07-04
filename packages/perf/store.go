package perf

import (
	"context"
	"sort"
	"sync"
)

// Store is the persistence contract for historical BenchmarkRun records,
// mirroring packages/reasoningeval.Store's shape (SaveScore/ListScores)
// applied to this package's own BenchmarkRun record instead of
// QualityScore. This package does not import packages/reasoningeval; the
// shape is followed by reference (see doc.go).
//
// Implementations must be safe for concurrent use from multiple
// goroutines.
type Store interface {
	// SaveRun persists run.
	SaveRun(ctx context.Context, run BenchmarkRun) error

	// ListRuns returns every persisted BenchmarkRun, most recent first. If
	// operation is non-empty, only runs with that Operation are returned.
	// Pass "" to skip the filter.
	ListRuns(ctx context.Context, operation OperationName) ([]BenchmarkRun, error)
}

// InMemoryStore is a thread-safe in-process implementation of Store. All
// data is lost when the process exits, mirroring
// packages/reasoningeval.InMemoryStore's convention. This is the only Store
// implementation this phase provides -- see doc.go's go.mod discussion for
// why no Postgres-backed implementation or migration is added.
type InMemoryStore struct {
	mu   sync.RWMutex
	runs []BenchmarkRun
}

// NewInMemoryStore returns a ready-to-use InMemoryStore.
func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{}
}

// SaveRun implements Store.
func (s *InMemoryStore) SaveRun(_ context.Context, run BenchmarkRun) error {
	if err := run.validate(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.runs = append(s.runs, run)
	return nil
}

// ListRuns implements Store.
func (s *InMemoryStore) ListRuns(_ context.Context, operation OperationName) ([]BenchmarkRun, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]BenchmarkRun, 0, len(s.runs))
	for _, r := range s.runs {
		if operation != "" && r.Operation != operation {
			continue
		}
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].RecordedAt.After(out[j].RecordedAt) })
	return out, nil
}
