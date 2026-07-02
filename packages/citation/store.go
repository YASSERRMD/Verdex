package citation

import (
	"context"
	"sync"
)

// Repository persists resolved CitedUnits per case, so a later audit or
// reporting pass can inspect exactly what was cited without re-running
// resolution and verification. Implementations:
//
//   - InMemoryRepository (this file): a fully in-memory implementation
//     backed by maps, mirroring graph.InMemoryGraphStore and
//     vectorindex.InMemoryVectorStore's convention of being the default,
//     always-available implementation used by tests and by any
//     deployment that does not yet need a durable backend.
type Repository interface {
	// Save persists unit, keyed by (unit.CaseID, unit.NodeID). Calling
	// Save again with the same case/node pair overwrites the previously
	// stored CitedUnit (idempotent upsert), mirroring
	// graph.GraphStore.CreateNode's overwrite convention.
	Save(ctx context.Context, unit CitedUnit) error

	// Get returns the CitedUnit stored for (caseID, nodeID), or
	// ErrCitationNotFound if none was ever saved.
	Get(ctx context.Context, caseID, nodeID string) (CitedUnit, error)

	// ListByCase returns every CitedUnit saved for caseID, in no
	// particular order. Empty (not nil) if none were saved.
	ListByCase(ctx context.Context, caseID string) ([]CitedUnit, error)

	// DeleteByCase removes every CitedUnit saved for caseID. Not an error
	// to delete a case with none saved.
	DeleteByCase(ctx context.Context, caseID string) error
}

// InMemoryRepository is a fully in-memory Repository implementation
// backed by maps. It is safe for concurrent use: all access to its
// internal maps is serialized by mu.
type InMemoryRepository struct {
	mu sync.RWMutex

	// units maps case id -> node id -> CitedUnit.
	units map[string]map[string]CitedUnit
}

// NewInMemoryRepository constructs an empty InMemoryRepository.
func NewInMemoryRepository() *InMemoryRepository {
	return &InMemoryRepository{units: make(map[string]map[string]CitedUnit)}
}

// Save implements Repository.
func (r *InMemoryRepository) Save(_ context.Context, unit CitedUnit) error {
	if unit.CaseID == "" {
		return ErrEmptyCaseID
	}
	if unit.NodeID == "" {
		return ErrEmptyNodeID
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	byNode, ok := r.units[unit.CaseID]
	if !ok {
		byNode = make(map[string]CitedUnit)
		r.units[unit.CaseID] = byNode
	}
	byNode[unit.NodeID] = unit
	return nil
}

// SaveAll saves every unit in units via Save, stopping at the first
// error.
func (r *InMemoryRepository) SaveAll(ctx context.Context, units []CitedUnit) error {
	for _, u := range units {
		if err := r.Save(ctx, u); err != nil {
			return err
		}
	}
	return nil
}

// Get implements Repository.
func (r *InMemoryRepository) Get(_ context.Context, caseID, nodeID string) (CitedUnit, error) {
	if caseID == "" {
		return CitedUnit{}, ErrEmptyCaseID
	}
	if nodeID == "" {
		return CitedUnit{}, ErrEmptyNodeID
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	byNode, ok := r.units[caseID]
	if !ok {
		return CitedUnit{}, ErrCitationNotFound
	}
	unit, ok := byNode[nodeID]
	if !ok {
		return CitedUnit{}, ErrCitationNotFound
	}
	return unit, nil
}

// ListByCase implements Repository.
func (r *InMemoryRepository) ListByCase(_ context.Context, caseID string) ([]CitedUnit, error) {
	if caseID == "" {
		return nil, ErrEmptyCaseID
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	byNode := r.units[caseID]
	out := make([]CitedUnit, 0, len(byNode))
	for _, unit := range byNode {
		out = append(out, unit)
	}
	return out, nil
}

// DeleteByCase implements Repository.
func (r *InMemoryRepository) DeleteByCase(_ context.Context, caseID string) error {
	if caseID == "" {
		return ErrEmptyCaseID
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.units, caseID)
	return nil
}

// Len returns the total number of CitedUnits currently stored across
// every case. Useful for tests asserting on repository state.
func (r *InMemoryRepository) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	count := 0
	for _, byNode := range r.units {
		count += len(byNode)
	}
	return count
}
