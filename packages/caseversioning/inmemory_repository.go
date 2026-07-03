package caseversioning

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

// InMemoryRepository is an in-process Repository implementation for
// tests and other packages' test fixtures, mirroring
// packages/annotations.InMemoryRepository's convention exactly. It is
// safe for concurrent use.
type InMemoryRepository struct {
	mu      sync.RWMutex
	records map[uuid.UUID]*Snapshot
}

// NewInMemoryRepository builds an empty InMemoryRepository.
func NewInMemoryRepository() *InMemoryRepository {
	return &InMemoryRepository{records: make(map[uuid.UUID]*Snapshot)}
}

func cloneSnapshot(s *Snapshot) *Snapshot {
	if s == nil {
		return nil
	}
	cp := *s
	if s.RestoredFromID != nil {
		id := *s.RestoredFromID
		cp.RestoredFromID = &id
	}
	return &cp
}

// Create implements Repository.
func (r *InMemoryRepository) Create(_ context.Context, tenantID uuid.UUID, s *Snapshot) error {
	if s == nil {
		return wrapf("InMemoryRepository.Create", ErrNilSnapshot)
	}
	if s.TenantID == uuid.Nil {
		s.TenantID = tenantID
	}
	if err := requireMatchingTenant(tenantID, s.TenantID); err != nil {
		return err
	}
	if err := s.Validate(); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	if s.CreatedAt.IsZero() {
		s.CreatedAt = time.Now().UTC()
	}

	r.records[s.ID] = cloneSnapshot(s)
	return nil
}

// Get implements Repository.
func (r *InMemoryRepository) Get(_ context.Context, tenantID, id uuid.UUID) (*Snapshot, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	s, ok := r.records[id]
	if !ok || s.TenantID != tenantID {
		return nil, ErrNotFound
	}
	return cloneSnapshot(s), nil
}

// ListByCase implements Repository.
func (r *InMemoryRepository) ListByCase(_ context.Context, tenantID, caseID uuid.UUID, filter SnapshotFilter) ([]*Snapshot, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]*Snapshot, 0)
	for _, s := range r.records {
		if s.TenantID != tenantID || s.CaseID != caseID {
			continue
		}
		if filter.Kind != "" && s.ArtifactKind != filter.Kind {
			continue
		}
		out = append(out, cloneSnapshot(s))
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out, nil
}

// Latest implements Repository.
func (r *InMemoryRepository) Latest(_ context.Context, tenantID, caseID uuid.UUID, kind ArtifactKind) (*Snapshot, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var latest *Snapshot
	for _, s := range r.records {
		if s.TenantID != tenantID || s.CaseID != caseID || s.ArtifactKind != kind {
			continue
		}
		if latest == nil || s.CreatedAt.After(latest.CreatedAt) {
			latest = s
		}
	}
	if latest == nil {
		return nil, ErrNotFound
	}
	return cloneSnapshot(latest), nil
}

var _ Repository = (*InMemoryRepository)(nil)
