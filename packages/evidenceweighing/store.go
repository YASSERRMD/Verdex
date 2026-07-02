package evidenceweighing

import (
	"context"
	"sync"
)

// Repository persists an EvidenceWeighingResult per case, so a caller
// (e.g. Phase 054's law-application module or Phase 055's synthesis
// agent) can retrieve a case's evidence weights without recomputing them.
// Implementations:
//
//   - InMemoryRepository (this file): a fully in-memory implementation
//     backed by a map, mirroring graph.InMemoryGraphStore,
//     vectorindex.InMemoryVectorStore, and citation.InMemoryRepository's
//     shared convention of being the default, always-available
//     implementation used by tests and by any deployment that does not
//     yet need a durable backend.
type Repository interface {
	// Save persists result, keyed by result.CaseID. Calling Save again
	// with the same CaseID overwrites the previously stored result
	// (idempotent upsert), mirroring citation.Repository.Save's
	// overwrite convention.
	Save(ctx context.Context, result EvidenceWeighingResult) error

	// Get returns the EvidenceWeighingResult stored for caseID, or
	// ErrResultNotFound if none was ever saved.
	Get(ctx context.Context, caseID string) (EvidenceWeighingResult, error)

	// DeleteByCase removes the EvidenceWeighingResult saved for caseID.
	// Not an error to delete a case with none saved.
	DeleteByCase(ctx context.Context, caseID string) error
}

// InMemoryRepository is a fully in-memory Repository implementation
// backed by a map. It is safe for concurrent use: all access to its
// internal map is serialized by mu.
type InMemoryRepository struct {
	mu sync.RWMutex

	// results maps case id -> EvidenceWeighingResult.
	results map[string]EvidenceWeighingResult
}

// NewInMemoryRepository constructs an empty InMemoryRepository.
func NewInMemoryRepository() *InMemoryRepository {
	return &InMemoryRepository{results: make(map[string]EvidenceWeighingResult)}
}

// Save implements Repository.
func (r *InMemoryRepository) Save(_ context.Context, result EvidenceWeighingResult) error {
	if result.CaseID == "" {
		return ErrEmptyCaseID
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.results[result.CaseID] = result
	return nil
}

// Get implements Repository.
func (r *InMemoryRepository) Get(_ context.Context, caseID string) (EvidenceWeighingResult, error) {
	if caseID == "" {
		return EvidenceWeighingResult{}, ErrEmptyCaseID
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	result, ok := r.results[caseID]
	if !ok {
		return EvidenceWeighingResult{}, ErrResultNotFound
	}
	return result, nil
}

// DeleteByCase implements Repository.
func (r *InMemoryRepository) DeleteByCase(_ context.Context, caseID string) error {
	if caseID == "" {
		return ErrEmptyCaseID
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.results, caseID)
	return nil
}

// Len returns the number of cases currently stored. Useful for tests
// asserting on repository state.
func (r *InMemoryRepository) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return len(r.results)
}
