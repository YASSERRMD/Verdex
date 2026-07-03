package casesearch

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

// InMemorySavedSearchRepository is an in-process SavedSearchRepository
// implementation for tests and other packages' test fixtures, mirroring
// packages/signoff.InMemoryRepository's convention exactly. It is safe
// for concurrent use.
type InMemorySavedSearchRepository struct {
	mu      sync.RWMutex
	records map[uuid.UUID]*SavedSearch
}

// NewInMemorySavedSearchRepository builds an empty
// InMemorySavedSearchRepository.
func NewInMemorySavedSearchRepository() *InMemorySavedSearchRepository {
	return &InMemorySavedSearchRepository{records: make(map[uuid.UUID]*SavedSearch)}
}

func cloneSavedSearch(s *SavedSearch) *SavedSearch {
	if s == nil {
		return nil
	}
	cp := *s
	return &cp
}

// Create implements SavedSearchRepository.
func (r *InMemorySavedSearchRepository) Create(_ context.Context, tenantID uuid.UUID, s *SavedSearch) error {
	if s == nil {
		return wrapf("InMemorySavedSearchRepository.Create", ErrNilRepository)
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
	now := time.Now().UTC()
	if s.CreatedAt.IsZero() {
		s.CreatedAt = now
	}
	s.UpdatedAt = now

	r.records[s.ID] = cloneSavedSearch(s)
	return nil
}

// Get implements SavedSearchRepository.
func (r *InMemorySavedSearchRepository) Get(_ context.Context, tenantID, id uuid.UUID) (*SavedSearch, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	s, ok := r.records[id]
	if !ok || s.TenantID != tenantID {
		return nil, ErrNotFound
	}
	return cloneSavedSearch(s), nil
}

// ListByOwner implements SavedSearchRepository.
func (r *InMemorySavedSearchRepository) ListByOwner(_ context.Context, tenantID, ownerID uuid.UUID) ([]*SavedSearch, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]*SavedSearch, 0)
	for _, s := range r.records {
		if s.TenantID != tenantID || s.OwnerID != ownerID {
			continue
		}
		out = append(out, cloneSavedSearch(s))
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	return out, nil
}

// Delete implements SavedSearchRepository.
func (r *InMemorySavedSearchRepository) Delete(_ context.Context, tenantID, id uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	s, ok := r.records[id]
	if !ok || s.TenantID != tenantID {
		return ErrNotFound
	}
	delete(r.records, id)
	return nil
}

var _ SavedSearchRepository = (*InMemorySavedSearchRepository)(nil)
